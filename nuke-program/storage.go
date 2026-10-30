package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/4rji/nuke-program/internal/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const storageSettingsVersion = 1

type StorageLocation struct {
	Path      string `json:"path"`
	IsDefault bool   `json:"isDefault"`
	IsCloud   bool   `json:"isCloud"`
}

type storageSettings struct {
	Version  int    `json:"version"`
	VaultDir string `json:"vaultDir"`
}

func loadVaultDirectory(dataRoot string) (string, error) {
	defaultDirectory := filepath.Join(dataRoot, "vaults")
	settingsPath := filepath.Join(dataRoot, "storage.json")
	info, err := os.Lstat(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return defaultDirectory, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4096 {
		return "", errors.New("invalid storage settings")
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", err
	}
	var settings storageSettings
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		return "", errors.New("invalid storage settings")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("invalid storage settings")
	}
	if settings.Version != storageSettingsVersion || settings.VaultDir == "" || !filepath.IsAbs(settings.VaultDir) {
		return "", errors.New("invalid storage settings")
	}
	resolved, err := filepath.EvalSymlinks(settings.VaultDir)
	if err != nil {
		return "", fmt.Errorf("configured vault location is unavailable: %w", err)
	}
	if !sameCleanPath(resolved, defaultDirectory) && !strings.EqualFold(filepath.Base(resolved), "NukeVault") {
		return "", errors.New("configured custom vault location must be a dedicated NukeVault directory")
	}
	info, err = os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("configured vault location is not a directory")
	}
	return resolved, nil
}

func (s *VaultService) GetStorageLocation() StorageLocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storageLocationLocked()
}

func (s *VaultService) ChooseStorageLocation() (StorageLocation, error) {
	current := s.GetStorageLocation()
	initialDirectory := filepath.Dir(current.Path)
	if current.IsDefault {
		if home, err := os.UserHomeDir(); err == nil {
			documents := filepath.Join(home, "Documents")
			if info, statErr := os.Stat(documents); statErr == nil && info.IsDir() {
				initialDirectory = documents
			} else {
				initialDirectory = home
			}
		}
	}
	selected, err := application.Get().Dialog.OpenFile().
		SetTitle("Choose NukeVault Storage Folder").
		SetMessage("NukeVault will create a dedicated NukeVault folder inside this location.").
		SetDirectory(initialDirectory).
		SetButtonText("Use This Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		ResolvesAliases(true).
		PromptForSingleSelection()
	if err != nil {
		return current, err
	}
	if selected == "" {
		return current, nil
	}
	return s.setStorageLocation(selected)
}

func (s *VaultService) ImportVault() (vault.VaultInfo, error) {
	s.mu.Lock()
	if len(s.runtime) != 0 {
		s.mu.Unlock()
		return vault.VaultInfo{}, errors.New("finish the active vault challenge before importing another vault")
	}
	vaultDir := s.vaultDir
	s.mu.Unlock()

	selected, err := application.Get().Dialog.OpenFile().
		SetTitle("Open NukeVault Container").
		SetMessage("Select a .nukevol file to copy into the managed vault directory.").
		SetDirectory(filepath.Dir(vaultDir)).
		SetButtonText("Open Vault").
		CanChooseDirectories(false).
		CanChooseFiles(true).
		AllowsOtherFileTypes(false).
		ResolvesAliases(true).
		AddFilter("NukeVault containers", "*.nukevol").
		PromptForSingleSelection()
	if err != nil {
		return vault.VaultInfo{}, err
	}
	if selected == "" {
		return vault.VaultInfo{}, nil
	}
	return s.importVault(selected)
}

func (s *VaultService) importVault(sourcePath string) (vault.VaultInfo, error) {
	s.mu.Lock()
	if len(s.runtime) != 0 {
		s.mu.Unlock()
		return vault.VaultInfo{}, errors.New("finish the active vault challenge before importing another vault")
	}
	vaultDir := s.vaultDir
	s.mu.Unlock()
	return vault.Import(vaultDir, sourcePath)
}

func (s *VaultService) setStorageLocation(selected string) (StorageLocation, error) {
	selected, err := filepath.Abs(selected)
	if err != nil {
		return StorageLocation{}, err
	}
	selected, err = filepath.EvalSymlinks(selected)
	if err != nil {
		return StorageLocation{}, fmt.Errorf("resolve selected folder: %w", err)
	}
	info, err := os.Stat(selected)
	if err != nil || !info.IsDir() {
		return StorageLocation{}, errors.New("selected storage location is not a directory")
	}
	vaultDir := selected
	if !strings.EqualFold(filepath.Base(vaultDir), "NukeVault") {
		vaultDir = filepath.Join(vaultDir, "NukeVault")
	}
	if pathInside(vaultDir, filepath.Join(s.dataRoot, "mounts")) || pathInside(vaultDir, s.challengeDir) {
		return StorageLocation{}, errors.New("the temporary mount or challenge-state directory cannot be used for vault storage")
	}
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		return StorageLocation{}, fmt.Errorf("create selected vault directory: %w", err)
	}
	if err := os.Chmod(vaultDir, 0o700); err != nil {
		return StorageLocation{}, fmt.Errorf("protect selected vault directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(vaultDir)
	if err != nil {
		return StorageLocation{}, err
	}
	if pathInside(resolved, filepath.Join(s.dataRoot, "mounts")) || pathInside(resolved, s.challengeDir) {
		return StorageLocation{}, errors.New("the temporary mount or challenge-state directory cannot be used for vault storage")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.runtime) != 0 {
		return StorageLocation{}, errors.New("lock or destroy the active vault before changing storage location")
	}
	if err := s.persistStorageLocationLocked(resolved); err != nil {
		return StorageLocation{}, err
	}
	s.vaultDir = resolved
	return s.storageLocationLocked(), nil
}

func (s *VaultService) persistStorageLocationLocked(vaultDir string) error {
	settings := storageSettings{Version: storageSettingsVersion, VaultDir: vaultDir}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.dataRoot, ".storage-*.tmp")
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
	if err := os.Rename(temporaryPath, filepath.Join(s.dataRoot, "storage.json")); err != nil {
		return err
	}
	cleanup = false
	return syncChallengeDir(s.dataRoot)
}

func (s *VaultService) storageLocationLocked() StorageLocation {
	defaultDirectory := filepath.Join(s.dataRoot, "vaults")
	return StorageLocation{
		Path:      s.vaultDir,
		IsDefault: sameCleanPath(s.vaultDir, defaultDirectory),
		IsCloud:   likelyCloudPath(s.vaultDir),
	}
}

func likelyCloudPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, marker := range []string{"/mobile documents/", "icloud", "onedrive", "dropbox", "nextcloud"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sameCleanPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func pathInside(candidate, root string) bool {
	candidate, candidateErr := filepath.Abs(candidate)
	root, rootErr := filepath.Abs(root)
	if candidateErr != nil || rootErr != nil {
		return false
	}
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
