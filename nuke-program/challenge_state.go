package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const challengeStateVersion = 1

type persistedChallenge struct {
	Version  int    `json:"version"`
	VaultID  string `json:"vaultId"`
	Attempts int    `json:"attempts"`
	Deadline int64  `json:"deadline"`
}

func (s *VaultService) loadChallengeState(id string) (persistedChallenge, bool, error) {
	path, err := s.challengeStatePath(id)
	if err != nil {
		return persistedChallenge{}, false, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedChallenge{}, false, nil
	}
	if err != nil {
		return persistedChallenge{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4096 {
		return persistedChallenge{}, false, errors.New("invalid persisted challenge state")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return persistedChallenge{}, false, err
	}
	var state persistedChallenge
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return persistedChallenge{}, false, errors.New("invalid persisted challenge state")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return persistedChallenge{}, false, err
	}
	if state.Version != challengeStateVersion || state.VaultID != id || state.Attempts < 0 || state.Attempts > 3 || state.Deadline <= 0 {
		return persistedChallenge{}, false, errors.New("invalid persisted challenge state")
	}
	return state, true, nil
}

func (s *VaultService) saveChallengeState(id string, attempts int, deadline time.Time) error {
	path, err := s.challengeStatePath(id)
	if err != nil {
		return err
	}
	if attempts < 0 || attempts > 3 || deadline.IsZero() {
		return errors.New("invalid challenge state")
	}
	state := persistedChallenge{
		Version:  challengeStateVersion,
		VaultID:  id,
		Attempts: attempts,
		Deadline: deadline.UnixMilli(),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.challengeDir, ".challenge-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		_ = temporary.Close()
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	cleanup = false
	return syncChallengeDir(s.challengeDir)
}

func (s *VaultService) clearChallengeState(id string) error {
	path, err := s.challengeStatePath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncChallengeDir(s.challengeDir)
}

func (s *VaultService) challengeStatePath(id string) (string, error) {
	if len(id) != 32 {
		return "", errors.New("invalid vault id")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", errors.New("invalid vault id")
	}
	return filepath.Join(s.challengeDir, id+".json"), nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("invalid persisted challenge state")
	}
	return nil
}

func syncChallengeDir(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open challenge state directory: %w", err)
	}
	defer directory.Close()
	return directory.Sync()
}
