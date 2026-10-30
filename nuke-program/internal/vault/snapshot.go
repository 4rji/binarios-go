package vault

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

type Entry struct {
	Path    string
	Mode    uint32
	ModTime int64
	IsDir   bool
	Data    []byte
}

type Snapshot struct {
	Entries []Entry
}

func InitialSnapshot(name string) Snapshot {
	message := "NUKEVAULT LAB CONTAINER\n\n" +
		"Vault: " + name + "\n\n" +
		"This disposable vault is encrypted at rest. Its unlock challenge is intentionally destructive:\n" +
		"- 10 seconds without a password triggers Nuke.\n" +
		"- the dedicated Nuke password triggers immediately.\n" +
		"- three incorrect passwords trigger Nuke.\n\n" +
		"Use only disposable classroom evidence.\n"
	return Snapshot{Entries: []Entry{{
		Path:    "README-LAB.txt",
		Mode:    0o600,
		ModTime: time.Now().UTC().UnixNano(),
		Data:    []byte(message),
	}}}
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := validateSnapshot(snapshot); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	if err := gob.NewEncoder(&result).Encode(snapshot); err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	return result.Bytes(), nil
}

func decodeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", ErrInvalidVault)
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[string]struct{}, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if !validEntryPath(entry.Path) {
			return ErrInvalidVault
		}
		if _, exists := seen[entry.Path]; exists {
			return ErrInvalidVault
		}
		seen[entry.Path] = struct{}{}
		if entry.IsDir && len(entry.Data) != 0 {
			return ErrInvalidVault
		}
	}
	return nil
}

func validEntryPath(name string) bool {
	if name == "" || strings.ContainsRune(name, '\x00') || strings.HasPrefix(name, "/") {
		return false
	}
	clean := path.Clean(name)
	return clean == name && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func NewMemoryFS(snapshot Snapshot) (webdav.FileSystem, error) {
	if err := validateSnapshot(snapshot); err != nil {
		return nil, err
	}
	fs := webdav.NewMemFS()
	ctx := context.Background()
	entries := append([]Entry(nil), snapshot.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		leftDepth := strings.Count(entries[i].Path, "/")
		rightDepth := strings.Count(entries[j].Path, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return entries[i].Path < entries[j].Path
	})
	for _, entry := range entries {
		fullPath := "/" + entry.Path
		mode := os.FileMode(entry.Mode & 0o777)
		if entry.IsDir {
			if err := fs.Mkdir(ctx, fullPath, mode); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			continue
		}
		if err := ensureParentDirs(ctx, fs, path.Dir(fullPath)); err != nil {
			return nil, err
		}
		f, err := fs.OpenFile(ctx, fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return nil, err
		}
		_, writeErr := f.Write(entry.Data)
		closeErr := f.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	return fs, nil
}

func CaptureFS(ctx context.Context, fs webdav.FileSystem) (Snapshot, error) {
	result := Snapshot{}
	var walk func(string) error
	walk = func(dir string) error {
		f, err := fs.OpenFile(ctx, dir, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		children, readErr := f.Readdir(-1)
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
		for _, child := range children {
			childPath := path.Join(dir, child.Name())
			relative := strings.TrimPrefix(childPath, "/")
			entry := Entry{
				Path:    relative,
				Mode:    uint32(child.Mode().Perm()),
				ModTime: child.ModTime().UTC().UnixNano(),
				IsDir:   child.IsDir(),
			}
			if child.IsDir() {
				result.Entries = append(result.Entries, entry)
				if err := walk(childPath); err != nil {
					return err
				}
				continue
			}
			file, err := fs.OpenFile(ctx, childPath, os.O_RDONLY, 0)
			if err != nil {
				return err
			}
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			entry.Data = data
			result.Entries = append(result.Entries, entry)
		}
		return nil
	}
	if err := walk("/"); err != nil {
		return Snapshot{}, err
	}
	return result, validateSnapshot(result)
}

func ensureParentDirs(ctx context.Context, fs webdav.FileSystem, dir string) error {
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	if _, err := fs.Stat(ctx, dir); err == nil {
		return nil
	}
	if err := ensureParentDirs(ctx, fs, path.Dir(dir)); err != nil {
		return err
	}
	if err := fs.Mkdir(ctx, dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
