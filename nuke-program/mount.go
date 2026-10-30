package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

type MountResult struct {
	URL        string `json:"url"`
	MountPoint string `json:"mountPoint"`
	Automatic  bool   `json:"automatic"`
	Message    string `json:"message"`
}

var unsafeMountName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeMountName(name string) string {
	name = unsafeMountName.ReplaceAllString(strings.TrimSpace(name), "-")
	name = strings.Trim(name, ".-")
	if name == "" {
		return "NukeVault"
	}
	if len(name) > 48 {
		return name[:48]
	}
	return name
}

func sessionMountSuffix(sessionURL string) string {
	digest := sha256.Sum256([]byte(sessionURL))
	return hex.EncodeToString(digest[:4])
}

func joinMountErrors(primary, secondary error) error {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	return errors.Join(primary, secondary)
}

type platformMounter interface {
	Mount(context.Context, string, string, string) (MountResult, error)
	Unmount(context.Context, MountResult) error
	ForceUnmount(context.Context, MountResult) error
	Reveal(context.Context, MountResult) error
}
