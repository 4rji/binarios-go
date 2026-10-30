package vault

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/webdav"
)

type Session struct {
	mu            sync.RWMutex
	containerPath string
	header        Header
	masterKey     []byte
	fs            webdav.FileSystem
	server        *http.Server
	listener      net.Listener
	url           string
	onError       func(error)
	closed        atomic.Bool
}

func StartSession(containerPath string, unlocked *Unlocked, onError func(error)) (*Session, error) {
	if unlocked == nil || len(unlocked.MasterKey) != 32 {
		return nil, errors.New("invalid unlocked vault")
	}
	fs, err := NewMemoryFS(unlocked.Snapshot)
	if err != nil {
		return nil, err
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local WebDAV listener: %w", err)
	}
	session := &Session{
		containerPath: containerPath,
		header:        unlocked.Header,
		masterKey:     append([]byte(nil), unlocked.MasterKey...),
		fs:            fs,
		listener:      listener,
		onError:       onError,
	}
	prefix := "/" + token
	davHandler := &webdav.Handler{
		Prefix:     prefix,
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}
	mux := http.NewServeMux()
	mux.Handle(prefix+"/", session.wrapDAV(davHandler))
	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	session.url = "http://" + listener.Addr().String() + prefix + "/"
	go func() {
		if err := session.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && onError != nil {
			onError(err)
		}
	}()
	return session, nil
}

func (s *Session) URL() string {
	return s.url
}

func (s *Session) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return errors.New("vault session closed")
	}
	return s.saveLocked()
}

func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	shutdownErr := s.server.Shutdown(ctx)
	cancel()
	var closeErr error
	if shutdownErr != nil {
		// Shutdown is graceful and can time out while a WebDAV client is in
		// the middle of an upload. Close the active connection so normal lock
		// cannot remain stuck behind that request forever.
		closeErr = s.server.Close()
	}

	s.mu.Lock()
	saveErr := s.saveLocked()
	zero(s.masterKey)
	s.masterKey = nil
	s.mu.Unlock()
	return errors.Join(shutdownErr, closeErr, saveErr)

}

// Destroy stops WebDAV immediately and discards the in-memory key without
// saving another encrypted snapshot. The caller can then wipe the existing
// managed container even when Finder has an unfinished upload.
func (s *Session) Destroy() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	closeErr := s.server.Close()
	s.mu.Lock()
	zero(s.masterKey)
	s.masterKey = nil
	s.mu.Unlock()
	return closeErr
}

func (s *Session) wrapDAV(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r.RemoteAddr) {
			http.Error(w, "loopback access only", http.StatusForbidden)
			return
		}
		if r.ContentLength > 64<<20 {
			http.Error(w, "individual files are limited to 64 MiB in lab mode", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mutating := isMutatingMethod(r.Method)
		if s.closed.Load() {
			http.Error(w, "vault session closed", http.StatusServiceUnavailable)
			return
		}

		// Do not hold the session lock while serving WebDAV. Finder may wait for
		// parallel OPTIONS, PROPFIND, or LOCK requests before it starts sending a
		// PUT body. Serializing the whole request here would deadlock that upload.
		// NewMemFS and NewMemLS provide their own concurrency protection.
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if mutating && recorder.status < http.StatusBadRequest {
			s.mu.Lock()
			var err error
			if !s.closed.Load() {
				err = s.saveLocked()
			}
			s.mu.Unlock()
			if err != nil && s.onError != nil {
				s.onError(err)
			}
		}
	})
}

func (s *Session) saveLocked() error {
	if len(s.masterKey) != 32 {
		return errors.New("vault key is no longer available")
	}
	snapshot, err := CaptureFS(context.Background(), s.fs)
	if err != nil {
		return err
	}
	return SaveSnapshot(s.containerPath, s.header, s.masterKey, snapshot)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "COPY", "PROPPATCH":
		return true
	default:
		return false
	}
}

func isLoopbackRequest(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}
