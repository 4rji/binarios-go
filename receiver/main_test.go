package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseReceiveArgsSupportsPositionalsAndFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want receiverOptions
	}{
		{
			name: "defaults",
			args: nil,
			want: receiverOptions{Port: "1235", Prefix: "file", OutputDir: "."},
		},
		{
			name: "positional port and prefix",
			args: []string{"9000", "loot-"},
			want: receiverOptions{Port: "9000", Prefix: "loot-", OutputDir: "."},
		},
		{
			name: "explicit flags",
			args: []string{"--port", "9001", "--prefix", "drop-", "--output-dir", "incoming"},
			want: receiverOptions{Port: "9001", Prefix: "drop-", OutputDir: "incoming"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReceiveArgs(tt.args)
			if err != nil {
				t.Fatalf("parseReceiveArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseReceiveArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseReceiveArgsRejectsAmbiguousPort(t *testing.T) {
	_, err := parseReceiveArgs([]string{"--port", "9000", "9001"})
	if err == nil {
		t.Fatal("parseReceiveArgs() expected an error when --port and positional port are mixed")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 0, want: "0 B"},
		{bytes: 999, want: "999 B"},
		{bytes: 1024, want: "1.0 KiB"},
		{bytes: 1536, want: "1.5 KiB"},
		{bytes: 2 * 1024 * 1024, want: "2.0 MiB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatBytes(tt.bytes); got != tt.want {
				t.Fatalf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestRenderStartupMenuUsesFriendlyAddressAndConcreteSendTarget(t *testing.T) {
	menu := renderStartupMenu("1235", "/tmp/incoming", []ifaceAddress{
		{Name: "en0", IP: "10.0.4.180"},
	})

	wants := []string{
		"0.0.0.0:1235 (all interfaces)",
		"/tmp/incoming",
		"nc -q 0 10.0.4.180 1235 < file_send",
		"nc      10.0.4.180 1235 < file_send",
		"en0  10.0.4.180",
	}
	for _, want := range wants {
		if !strings.Contains(menu, want) {
			t.Fatalf("renderStartupMenu() missing %q in:\n%s", want, menu)
		}
	}

	if strings.Contains(menu, "[::]") {
		t.Fatalf("renderStartupMenu() should not show raw dual-stack listener address:\n%s", menu)
	}
	if strings.Contains(menu, "\033[92m") {
		t.Fatalf("renderStartupMenu() should not use the old bright green command color:\n%s", menu)
	}
	if !strings.HasPrefix(menu, "\033[31m╭─ Receiver ready ") {
		t.Fatalf("renderStartupMenu() should render the summary box in dark red:\n%s", menu)
	}
}

func TestCreateNextAvailableFileSkipsExistingWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "file")
	existing := prefix + "1"
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	counter := 1
	name, file, err := createNextAvailableFile(prefix, &counter)
	if err != nil {
		t.Fatalf("createNextAvailableFile() error = %v", err)
	}
	if name != prefix+"2" {
		t.Fatalf("createNextAvailableFile() name = %q, want %q", name, prefix+"2")
	}
	if counter != 3 {
		t.Fatalf("counter = %d, want 3", counter)
	}
	if _, err := file.WriteString("new"); err != nil {
		t.Fatalf("write reserved file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close reserved file: %v", err)
	}

	gotExisting, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(gotExisting) != "keep" {
		t.Fatalf("existing file was overwritten: got %q", gotExisting)
	}

	gotNew, err := os.ReadFile(prefix + "2")
	if err != nil {
		t.Fatalf("read new file: %v", err)
	}
	if string(gotNew) != "new" {
		t.Fatalf("new file = %q, want %q", gotNew, "new")
	}
}

func TestServeFileReceiverAcceptsConcurrentConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	opts := receiverOptions{Port: "1235", Prefix: "file", OutputDir: dir}
	ln := mustListenLocal(t)

	done := make(chan error, 1)
	go func() {
		done <- serveFileReceiver(ctx, ln, opts)
	}()

	first := mustDial(t, ln.Addr().String())
	defer first.Close()
	if _, err := first.Write([]byte("first-still-open")); err != nil {
		t.Fatalf("write first connection: %v", err)
	}
	waitForFile(t, filepath.Join(dir, "file1"))

	second := mustDial(t, ln.Addr().String())
	if _, err := second.Write([]byte("second-complete")); err != nil {
		t.Fatalf("write second connection: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second connection: %v", err)
	}

	waitForFileContent(t, filepath.Join(dir, "file2"), "second-complete")

	if err := first.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}
	cancel()
	waitForServeToStop(t, done)
}

func TestServeFileReceiverClosesActiveConnectionsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	opts := receiverOptions{Port: "1235", Prefix: "file", OutputDir: dir}
	ln := mustListenLocal(t)

	done := make(chan error, 1)
	go func() {
		done <- serveFileReceiver(ctx, ln, opts)
	}()

	conn := mustDial(t, ln.Addr().String())
	defer conn.Close()
	if _, err := conn.Write([]byte("partial")); err != nil {
		t.Fatalf("write connection: %v", err)
	}
	waitForFile(t, filepath.Join(dir, "file1"))

	cancel()
	waitForServeToStop(t, done)
}

func mustListenLocal(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func mustDial(t *testing.T, addr string) net.Conn {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("dial %s: %v", addr, lastErr)
	return nil
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file %s was not created", path)
}

func waitForFileContent(t *testing.T, path, want string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := os.ReadFile(path)
		if err == nil && string(got) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	t.Fatalf("file %s = %q, want %q", path, got, want)
}

func waitForServeToStop(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveFileReceiver() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveFileReceiver() did not stop after shutdown")
	}
}
