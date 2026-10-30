//go:build linux

package main

import (
	"context"
	"fmt"
	"os/exec"
)

type nativeMounter struct{}

func newPlatformMounter() platformMounter { return nativeMounter{} }

func (nativeMounter) Mount(ctx context.Context, name, url, dataRoot string) (MountResult, error) {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return MountResult{
			URL:       url,
			Automatic: false,
			Message:   "gio is unavailable; use the local WebDAV URL from a compatible file manager.",
		}, err
	}
	command := exec.CommandContext(ctx, gio, "mount", url)
	if output, err := command.CombinedOutput(); err != nil {
		return MountResult{
			URL:       url,
			Automatic: false,
			Message:   "Automatic gio mounting failed; the local WebDAV URL remains available.",
		}, fmt.Errorf("gio mount: %w: %s", err, string(output))
	}
	return MountResult{
		URL:        url,
		MountPoint: url,
		Automatic:  true,
		Message:    "Mounted through gio WebDAV without FUSE.",
	}, nil
}

func (nativeMounter) Unmount(ctx context.Context, mount MountResult) error {
	gio, err := exec.LookPath("gio")
	if err != nil || mount.URL == "" {
		return err
	}
	return exec.CommandContext(ctx, gio, "mount", "-u", mount.URL).Run()
}

func (nativeMounter) Reveal(ctx context.Context, mount MountResult) error {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return err
	}
	target := mount.MountPoint
	if target == "" {
		target = mount.URL
	}
	return exec.CommandContext(ctx, gio, "open", target).Run()
}
