//go:build darwin && integration

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/4rji/nuke-program/internal/vault"
)

func TestDarwinWebDAVMount(t *testing.T) {
	root := t.TempDir()
	info, err := vault.Create(root, "Mount Probe", "correct horse", "burn after reading", vault.KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	containerPath, err := vault.FindPath(root, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	unlocked, err := vault.Authenticate(containerPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	session, err := vault.StartSession(containerPath, unlocked, func(err error) { t.Errorf("session error: %v", err) })
	clear(unlocked.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	dataRoot := t.TempDir()
	staleMount := filepath.Join(dataRoot, "mounts", safeMountName(info.Name))
	if err := os.MkdirAll(staleMount, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	mount, err := (nativeMounter{}).Mount(ctx, info.Name, session.URL(), dataRoot)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if !mount.Automatic || mount.MountPoint == "" {
		t.Fatalf("unexpected mount result: %#v", mount)
	}
	if mount.MountPoint == staleMount {
		t.Fatal("new session reused a stale mount point")
	}
	unmounted := false
	defer func() {
		if unmounted {
			return
		}
		unmountCtx, unmountCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unmountCancel()
		_ = (nativeMounter{}).Unmount(unmountCtx, mount)
	}()
	target := filepath.Join(mount.MountPoint, "PearsonVUE exam appointment.ics")
	if err := os.WriteFile(target, []byte("old appointment"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("new appointment"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new appointment" {
		t.Fatalf("mounted replacement returned %q", data)
	}
	localSource := filepath.Join(t.TempDir(), "Finder copy source.ics")
	if err := os.WriteFile(localSource, []byte("first Finder-style copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, attribute := range [][2]string{
		{"com.apple.quarantine", "0081;00000000;NukeVaultTest;00000000-0000-0000-0000-000000000000"},
		{"com.apple.metadata:kMDItemWhereFroms", "https://example.invalid/lab-evidence"},
	} {
		if output, err := exec.Command("/usr/bin/xattr", "-w", attribute[0], attribute[1], localSource).CombinedOutput(); err != nil {
			t.Fatalf("set test xattr %s: %v: %s", attribute[0], err, output)
		}
	}
	copyTarget := filepath.Join(mount.MountPoint, "Finder copied appointment.ics")
	if output, err := exec.Command("/bin/cp", "-f", localSource, copyTarget).CombinedOutput(); err != nil {
		t.Fatalf("first mounted cp failed: %v: %s", err, output)
	}
	if err := os.WriteFile(localSource, []byte("replacement Finder-style copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/bin/cp", "-f", localSource, copyTarget).CombinedOutput(); err != nil {
		t.Fatalf("replacement mounted cp failed: %v: %s", err, output)
	}
	data, err = os.ReadFile(copyTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "replacement Finder-style copy" {
		t.Fatalf("mounted cp replacement returned %q", data)
	}
	unmountCtx, unmountCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer unmountCancel()
	if err := (nativeMounter{}).Unmount(unmountCtx, mount); err != nil {
		t.Fatal(err)
	}
	unmounted = true
}
