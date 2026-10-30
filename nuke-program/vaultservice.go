package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/4rji/nuke-program/internal/vault"
)

const defaultUnlockWindow = 10 * time.Second

type VaultEvent struct {
	VaultID  string  `json:"vaultId"`
	Status   string  `json:"status"`
	Message  string  `json:"message"`
	Attempts int     `json:"attempts"`
	Deadline int64   `json:"deadline"`
	Phase    string  `json:"phase"`
	Percent  float64 `json:"percent"`
}

type UnlockState struct {
	VaultID  string      `json:"vaultId"`
	Status   string      `json:"status"`
	Message  string      `json:"message"`
	Attempts int         `json:"attempts"`
	Deadline int64       `json:"deadline"`
	Mount    MountResult `json:"mount"`
}

type runtimeVault struct {
	path     string
	name     string
	status   string
	attempts int
	deadline time.Time
	timer    *time.Timer
	session  *vault.Session
	mount    MountResult
}

type nukeJob struct {
	id            string
	reason        string
	containerPath string
	runtimeState  *runtimeVault
	session       *vault.Session
	mount         MountResult
}

type VaultService struct {
	mu                sync.Mutex
	nukeWG            sync.WaitGroup
	dataRoot          string
	vaultDir          string
	runtime           map[string]*runtimeVault
	mounter           platformMounter
	emit              func(string, any)
	challengeDuration time.Duration
	kdfParams         vault.KDFParams
}

func NewVaultService(dataRoot string) (*VaultService, error) {
	if dataRoot == "" {
		configRoot, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dataRoot = filepath.Join(configRoot, "NukeVault")
	}
	dataRoot, err := filepath.Abs(dataRoot)
	if err != nil {
		return nil, err
	}
	vaultDir := filepath.Join(dataRoot, "vaults")
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataRoot, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(vaultDir, 0o700); err != nil {
		return nil, err
	}
	return &VaultService{
		dataRoot:          dataRoot,
		vaultDir:          vaultDir,
		runtime:           make(map[string]*runtimeVault),
		mounter:           newPlatformMounter(),
		challengeDuration: defaultUnlockWindow,
		kdfParams:         vault.DefaultKDFParams,
	}, nil
}

func (s *VaultService) setEmitter(emit func(string, any)) {
	s.mu.Lock()
	s.emit = emit
	s.mu.Unlock()
}

func (s *VaultService) ListVaults() ([]vault.VaultInfo, error) {
	return vault.List(s.vaultDir)
}

func (s *VaultService) CreateVault(name, password, nukePassword string) (vault.VaultInfo, error) {
	return vault.Create(s.vaultDir, name, password, nukePassword, s.kdfParams)
}

func (s *VaultService) ArmVault(id string) (UnlockState, error) {
	containerPath, err := vault.FindPath(s.vaultDir, id)
	if err != nil {
		return UnlockState{}, err
	}
	info, err := vault.ReadInfo(containerPath)
	if err != nil {
		return UnlockState{}, err
	}
	s.mu.Lock()
	if existing := s.runtime[id]; existing != nil {
		state := stateFromRuntime(id, existing, "Challenge already active.")
		s.mu.Unlock()
		return state, nil
	}
	runtimeState := &runtimeVault{
		path:     containerPath,
		name:     info.Name,
		status:   "armed",
		deadline: time.Now().Add(s.challengeDuration),
	}
	runtimeState.timer = s.newChallengeTimer(id, runtimeState.deadline)
	s.runtime[id] = runtimeState
	state := stateFromRuntime(id, runtimeState, "Challenge armed. Enter a password within 10 seconds.")
	s.mu.Unlock()
	s.emitEvent(VaultEvent{VaultID: id, Status: "armed", Message: state.Message, Deadline: state.Deadline})
	return state, nil
}

func (s *VaultService) SubmitPassword(id, password string) (UnlockState, error) {
	s.mu.Lock()
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status != "armed" {
		s.mu.Unlock()
		return UnlockState{}, errors.New("vault challenge is not armed")
	}
	if runtimeState.timer != nil {
		runtimeState.timer.Stop()
	}
	runtimeState.status = "verifying"
	containerPath := runtimeState.path
	s.mu.Unlock()

	unlocked, err := vault.Authenticate(containerPath, password)
	password = ""
	if errors.Is(err, vault.ErrNukePassword) {
		if !s.queueNuke(id, "The dedicated Nuke password was entered.") {
			return UnlockState{}, errors.New("vault challenge ended")
		}
		return UnlockState{VaultID: id, Status: "nuking", Message: "Nuke password accepted. Key destruction started."}, nil
	}
	if errors.Is(err, vault.ErrWrongPassword) {
		s.mu.Lock()
		runtimeState = s.runtime[id]
		if runtimeState == nil || runtimeState.status != "verifying" {
			s.mu.Unlock()
			return UnlockState{}, errors.New("vault challenge ended")
		}
		runtimeState.attempts++
		attempts := runtimeState.attempts
		if attempts >= 3 {
			job := s.claimNukeLocked(id, "Three incorrect passwords were entered.")
			s.mu.Unlock()
			if job == nil {
				return UnlockState{}, errors.New("vault challenge ended")
			}
			go s.runNuke(job)
			return UnlockState{VaultID: id, Status: "nuking", Attempts: attempts, Message: "Third failure. Nuke started."}, nil
		}
		runtimeState.status = "armed"
		runtimeState.deadline = time.Now().Add(s.challengeDuration)
		runtimeState.timer = s.newChallengeTimer(id, runtimeState.deadline)
		state := stateFromRuntime(id, runtimeState, fmt.Sprintf("Incorrect password. Attempt %d of 3.", attempts))
		s.mu.Unlock()
		s.emitEvent(VaultEvent{VaultID: id, Status: "armed", Message: state.Message, Attempts: attempts, Deadline: state.Deadline})
		return state, nil
	}
	if err != nil {
		s.mu.Lock()
		delete(s.runtime, id)
		s.mu.Unlock()
		return UnlockState{}, err
	}

	s.mu.Lock()
	runtimeState = s.runtime[id]
	if runtimeState == nil || runtimeState.status != "verifying" {
		s.mu.Unlock()
		clear(unlocked.MasterKey)
		return UnlockState{}, errors.New("vault challenge ended")
	}
	s.mu.Unlock()

	session, err := vault.StartSession(containerPath, unlocked, func(sessionErr error) {
		s.emitEvent(VaultEvent{VaultID: id, Status: "error", Message: sessionErr.Error()})
	})
	clear(unlocked.MasterKey)
	if err != nil {
		s.mu.Lock()
		delete(s.runtime, id)
		s.mu.Unlock()
		return UnlockState{}, err
	}

	mountCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	mount, mountErr := s.mounter.Mount(mountCtx, unlocked.Header.Name, session.URL(), s.dataRoot)
	cancel()
	if mount.URL == "" {
		mount.URL = session.URL()
	}
	message := mount.Message
	if mountErr != nil && message == "" {
		message = mountErr.Error()
	}
	s.mu.Lock()
	runtimeState = s.runtime[id]
	if runtimeState == nil {
		s.mu.Unlock()
		_ = session.Close()
		return UnlockState{}, errors.New("vault challenge ended")
	}
	runtimeState.status = "mounted"
	runtimeState.session = session
	runtimeState.mount = mount
	state := stateFromRuntime(id, runtimeState, message)
	s.mu.Unlock()
	s.emitEvent(VaultEvent{VaultID: id, Status: "mounted", Message: message})
	return state, nil
}

func (s *VaultService) newChallengeTimer(id string, deadline time.Time) *time.Timer {
	delay := time.Until(deadline)
	if delay < 0 {
		delay = 0
	}
	return time.AfterFunc(delay, func() {
		s.triggerNukeAtDeadline(id, deadline)
	})
}

func (s *VaultService) triggerNukeAtDeadline(id string, deadline time.Time) {
	s.mu.Lock()
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status != "armed" || !runtimeState.deadline.Equal(deadline) {
		s.mu.Unlock()
		return
	}
	job := s.claimNukeLocked(id, "No password was entered within 10 seconds.")
	s.mu.Unlock()
	if job != nil {
		go s.runNuke(job)
	}
}

func (s *VaultService) LockVault(id string) error {
	s.mu.Lock()
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status != "mounted" {
		s.mu.Unlock()
		return errors.New("vault is not mounted")
	}
	session := runtimeState.session
	mount := runtimeState.mount
	delete(s.runtime, id)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	unmountErr := s.mounter.Unmount(ctx, mount)
	cancel()
	closeErr := session.Close()
	s.emitEvent(VaultEvent{VaultID: id, Status: "locked", Message: "Vault locked."})
	return errors.Join(unmountErr, closeErr)
}

func (s *VaultService) DestroyVault(id string) (UnlockState, error) {
	s.mu.Lock()
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status != "mounted" {
		s.mu.Unlock()
		return UnlockState{}, errors.New("vault must be unlocked before manual destruction")
	}
	job := s.claimNukeLocked(id, "Manual destruction was confirmed after unlocking the vault.")
	state := stateFromRuntime(id, runtimeState, "Manual destruction confirmed. Nuke started.")
	state.Status = "nuking"
	s.mu.Unlock()
	if job == nil {
		return UnlockState{}, errors.New("vault destruction could not be started")
	}
	go s.runNuke(job)
	return state, nil
}

func (s *VaultService) RevealVault(id string) error {
	s.mu.Lock()
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status != "mounted" {
		s.mu.Unlock()
		return errors.New("vault is not mounted")
	}
	mount := runtimeState.mount
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.mounter.Reveal(ctx, mount)
}

func (s *VaultService) ServiceShutdown() error {
	s.mu.Lock()
	states := make([]*runtimeVault, 0, len(s.runtime))
	for id, runtimeState := range s.runtime {
		if runtimeState.timer != nil {
			runtimeState.timer.Stop()
		}
		if runtimeState.status != "wiping" {
			states = append(states, runtimeState)
			delete(s.runtime, id)
		}
	}
	s.mu.Unlock()
	var result error
	for _, runtimeState := range states {
		if runtimeState.session == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		unmountErr := s.mounter.Unmount(ctx, runtimeState.mount)
		cancel()
		result = errors.Join(result, unmountErr, runtimeState.session.Close())
	}
	s.nukeWG.Wait()
	return result
}

func (s *VaultService) queueNuke(id, reason string) bool {
	s.mu.Lock()
	job := s.claimNukeLocked(id, reason)
	s.mu.Unlock()
	if job == nil {
		return false
	}
	go s.runNuke(job)
	return true
}

func (s *VaultService) claimNukeLocked(id, reason string) *nukeJob {
	runtimeState := s.runtime[id]
	if runtimeState == nil || runtimeState.status == "wiping" {
		return nil
	}
	if runtimeState.timer != nil {
		runtimeState.timer.Stop()
	}
	runtimeState.status = "wiping"
	s.nukeWG.Add(1)
	return &nukeJob{
		id:            id,
		reason:        reason,
		containerPath: runtimeState.path,
		runtimeState:  runtimeState,
		session:       runtimeState.session,
		mount:         runtimeState.mount,
	}
}

func (s *VaultService) runNuke(job *nukeJob) {
	defer s.nukeWG.Done()
	s.emitEvent(VaultEvent{VaultID: job.id, Status: "wiping", Message: job.reason, Phase: "unmounting"})
	if job.session != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.mounter.Unmount(ctx, job.mount)
		cancel()
		_ = job.session.Close()
	}
	err := vault.Wipe(job.containerPath, s.vaultDir, func(update vault.WipeProgress) {
		s.emitEvent(VaultEvent{VaultID: job.id, Status: "wiping", Message: job.reason, Phase: update.Phase, Percent: update.Percent})
	})
	s.mu.Lock()
	if s.runtime[job.id] == job.runtimeState {
		delete(s.runtime, job.id)
	}
	s.mu.Unlock()
	if err != nil {
		s.emitEvent(VaultEvent{VaultID: job.id, Status: "error", Message: err.Error()})
		return
	}
	s.emitEvent(VaultEvent{VaultID: job.id, Status: "destroyed", Message: "The container was cryptographically nuked, overwritten, and deleted.", Phase: "deleted", Percent: 100})
}

func (s *VaultService) emitEvent(event VaultEvent) {
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		emit("vault:state", event)
	}
}

func stateFromRuntime(id string, runtimeState *runtimeVault, message string) UnlockState {
	return UnlockState{
		VaultID:  id,
		Status:   runtimeState.status,
		Message:  message,
		Attempts: runtimeState.attempts,
		Deadline: runtimeState.deadline.UnixMilli(),
		Mount:    runtimeState.mount,
	}
}
