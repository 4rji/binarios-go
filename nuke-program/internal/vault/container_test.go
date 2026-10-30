package vault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var testKDF = KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}

func TestCreateAuthenticateAndSave(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "Evidence", "correct horse", "burn after reading", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	containerPath, err := FindPath(root, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(containerPath, "incorrect"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected wrong password, got %v", err)
	}
	if _, err := Authenticate(containerPath, "burn after reading"); !errors.Is(err, ErrNukePassword) {
		t.Fatalf("expected nuke password, got %v", err)
	}
	unlocked, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if len(unlocked.Snapshot.Entries) != 1 || unlocked.Snapshot.Entries[0].Path != "README-LAB.txt" {
		t.Fatalf("unexpected initial snapshot: %#v", unlocked.Snapshot)
	}
	unlocked.Snapshot.Entries = append(unlocked.Snapshot.Entries, Entry{Path: "evidence.txt", Mode: 0o600, Data: []byte("recover me")})
	if err := SaveSnapshot(containerPath, unlocked.Header, unlocked.MasterKey, unlocked.Snapshot); err != nil {
		t.Fatal(err)
	}
	clear(unlocked.MasterKey)
	reopened, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(reopened.MasterKey)
	if !snapshotContains(reopened.Snapshot, "evidence.txt", []byte("recover me")) {
		t.Fatal("saved evidence was not recovered")
	}
}

func TestWebDAVSessionPersistsMutations(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "WebDAV", "correct horse", "burn after reading", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := FindPath(root, info.ID)
	unlocked, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	session, err := StartSession(containerPath, unlocked, func(err error) { t.Errorf("session error: %v", err) })
	clear(unlocked.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPut, session.URL()+"notes.txt", bytes.NewBufferString("volatile evidence"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode >= 300 {
		t.Fatalf("PUT returned %s", response.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(reopened.MasterKey)
	if !snapshotContains(reopened.Snapshot, "notes.txt", []byte("volatile evidence")) {
		t.Fatal("WebDAV mutation was not persisted")
	}
}

func TestWebDAVReplacesExistingFile(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "WebDAV Replace", "correct horse", "burn after reading", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := FindPath(root, info.ID)
	unlocked, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	session, err := StartSession(containerPath, unlocked, func(err error) { t.Errorf("session error: %v", err) })
	clear(unlocked.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	name := "PearsonVUE exam appointment.ics"
	destination := session.URL() + url.PathEscape(name)
	putWebDAVFile(t, destination, "old appointment")
	temporary := session.URL() + ".PearsonVUE-replacement.tmp"
	putWebDAVFile(t, temporary, "new appointment")

	request, err := http.NewRequest("MOVE", temporary, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Destination", destination)
	request.Header.Set("Overwrite", "T")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode >= 300 {
		t.Fatalf("MOVE replacement returned %s", response.Status)
	}

	response, err = http.Get(destination)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK || string(data) != "new appointment" {
		t.Fatalf("replacement was not visible: status=%s data=%q", response.Status, data)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(reopened.MasterKey)
	if !snapshotContains(reopened.Snapshot, name, []byte("new appointment")) {
		t.Fatal("replacement was not persisted in the encrypted container")
	}
}

func TestWebDAVUploadDoesNotBlockParallelOptions(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "Concurrent WebDAV", "correct horse", "burn after reading", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := FindPath(root, info.ID)
	unlocked, err := Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	session, err := StartSession(containerPath, unlocked, func(err error) { t.Errorf("session error: %v", err) })
	clear(unlocked.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	body := &gatedReader{started: make(chan struct{}), release: make(chan struct{}), data: []byte("Finder upload")}
	putRequest, err := http.NewRequest(http.MethodPut, session.URL()+"external-file.pdf", body)
	if err != nil {
		t.Fatal(err)
	}
	putDone := make(chan error, 1)
	go func() {
		response, requestErr := http.DefaultClient.Do(putRequest)
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode >= 300 {
				requestErr = fmt.Errorf("PUT returned %s", response.Status)
			}
		}
		putDone <- requestErr
	}()

	select {
	case <-body.started:
	case <-time.After(2 * time.Second):
		t.Fatal("PUT body was never requested")
	}

	optionsCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	optionsRequest, err := http.NewRequestWithContext(optionsCtx, http.MethodOptions, session.URL(), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(optionsRequest)
	if err != nil {
		t.Fatalf("parallel OPTIONS was blocked by PUT: %v", err)
	}
	response.Body.Close()
	close(body.release)
	if err := <-putDone; err != nil {
		t.Fatal(err)
	}
}

func TestWipeOnlyManagedContainer(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "Disposable", "correct horse", "burn after reading", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := FindPath(root, info.ID)
	outside := filepath.Join(t.TempDir(), "outside.nukevol")
	if err := os.WriteFile(outside, []byte(fileMagic+"fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Wipe(outside, root, nil); err == nil {
		t.Fatal("wipe accepted a target outside the managed directory")
	}
	var phases []string
	if err := Wipe(containerPath, root, func(update WipeProgress) { phases = append(phases, update.Phase) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(containerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("container still exists after wipe: %v", err)
	}
	if len(phases) == 0 || phases[len(phases)-1] != "deleted" {
		t.Fatalf("unexpected wipe progress: %v", phases)
	}
}

func snapshotContains(snapshot Snapshot, name string, data []byte) bool {
	for _, entry := range snapshot.Entries {
		if entry.Path == name && bytes.Equal(entry.Data, data) {
			return true
		}
	}
	return false
}

func putWebDAVFile(t *testing.T, target, contents string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, target, bytes.NewBufferString(contents))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode >= 300 {
		t.Fatalf("PUT returned %s", response.Status)
	}
}

type gatedReader struct {
	started chan struct{}
	release chan struct{}
	data    []byte
	once    sync.Once
}

func (r *gatedReader) Read(buffer []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(buffer, r.data)
	r.data = r.data[n:]
	return n, nil
}
