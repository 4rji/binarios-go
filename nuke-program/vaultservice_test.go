package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/4rji/nuke-program/internal/vault"
)

type testMounter struct{}

func (testMounter) Mount(ctx context.Context, name, url, dataRoot string) (MountResult, error) {
	return MountResult{URL: url, MountPoint: "/lab/" + safeMountName(name), Automatic: true, Message: "test mount"}, nil
}

func (testMounter) Unmount(ctx context.Context, mount MountResult) error { return nil }
func (testMounter) Reveal(ctx context.Context, mount MountResult) error  { return nil }

func newTestService(t *testing.T) *VaultService {
	t.Helper()
	service, err := NewVaultService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service.kdfParams = vault.KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}
	service.challengeDuration = 100 * time.Millisecond
	service.mounter = testMounter{}
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	return service
}

func TestCorrectPasswordMountsAndPreservesVault(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ArmVault(info.ID); err != nil {
		t.Fatal(err)
	}
	state, err := service.SubmitPassword(info.ID, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "mounted" || !state.Mount.Automatic {
		t.Fatalf("unexpected state: %#v", state)
	}
	if err := service.LockVault(info.ID); err != nil {
		t.Fatal(err)
	}
	containerPath, err := vault.FindPath(service.vaultDir, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(containerPath); err != nil {
		t.Fatalf("normal lock removed vault: %v", err)
	}
}

func TestNukePasswordWipesImmediately(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	_, _ = service.ArmVault(info.ID)
	state, err := service.SubmitPassword(info.ID, "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "nuking" {
		t.Fatalf("unexpected state: %#v", state)
	}
	waitUntilRemoved(t, containerPath)
}

func TestShutdownCannotCancelAcceptedNuke(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	if _, err := service.ArmVault(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitPassword(info.ID, "burn after reading"); err != nil {
		t.Fatal(err)
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(containerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shutdown cancelled accepted Nuke: %v", err)
	}
}

func TestManualDestroyRequiresUnlockedVault(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	if _, err := service.DestroyVault(info.ID); err == nil {
		t.Fatal("manual destruction was allowed before unlocking")
	}
	if _, err := service.ArmVault(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitPassword(info.ID, "correct horse"); err != nil {
		t.Fatal(err)
	}
	state, err := service.DestroyVault(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "nuking" {
		t.Fatalf("unexpected state: %#v", state)
	}
	waitUntilRemoved(t, containerPath)
}

func TestThirdWrongPasswordWipes(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	_, _ = service.ArmVault(info.ID)
	for attempt := 1; attempt <= 3; attempt++ {
		state, err := service.SubmitPassword(info.ID, "definitely wrong")
		if err != nil {
			t.Fatal(err)
		}
		if attempt < 3 && state.Attempts != attempt {
			t.Fatalf("attempt %d produced %#v", attempt, state)
		}
	}
	waitUntilRemoved(t, containerPath)
}

func TestTimeoutWipes(t *testing.T) {
	service := newTestService(t)
	service.challengeDuration = 25 * time.Millisecond
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	if _, err := service.ArmVault(info.ID); err != nil {
		t.Fatal(err)
	}
	waitUntilRemoved(t, containerPath)
}

func TestSupersededDeadlineCannotWipeNewChallenge(t *testing.T) {
	service := newTestService(t)
	info, err := service.CreateVault("Evidence", "correct horse", "burn after reading")
	if err != nil {
		t.Fatal(err)
	}
	containerPath, _ := vault.FindPath(service.vaultDir, info.ID)
	if _, err := service.ArmVault(info.ID); err != nil {
		t.Fatal(err)
	}

	service.mu.Lock()
	oldDeadline := service.runtime[info.ID].deadline
	service.mu.Unlock()
	if _, err := service.SubmitPassword(info.ID, "definitely wrong"); err != nil {
		t.Fatal(err)
	}

	service.triggerNukeAtDeadline(info.ID, oldDeadline)
	if _, err := os.Stat(containerPath); err != nil {
		t.Fatalf("superseded deadline removed the vault: %v", err)
	}
	service.mu.Lock()
	state := service.runtime[info.ID]
	service.mu.Unlock()
	if state == nil || state.status != "armed" || state.attempts != 1 {
		t.Fatalf("unexpected challenge after stale deadline: %#v", state)
	}
}

func waitUntilRemoved(t *testing.T, target string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(target)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("target was not removed: %s", target)
}
