//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type nativeMounter struct{}

func newPlatformMounter() platformMounter { return nativeMounter{} }

func (nativeMounter) Mount(ctx context.Context, name, url, dataRoot string) (MountResult, error) {
	mountRoot := filepath.Join(dataRoot, "mounts")
	if err := os.MkdirAll(mountRoot, 0o700); err != nil {
		return MountResult{URL: url, Message: "WebDAV is available, but the mount directory could not be created."}, err
	}
	mountName := safeMountName(name) + "-" + sessionMountSuffix(url)
	mountPoint := filepath.Join(mountRoot, mountName)
	if err := os.Mkdir(mountPoint, 0o700); err != nil && !os.IsExist(err) {
		return MountResult{URL: url, Message: "WebDAV is available, but the mount point could not be created."}, err
	}
	command := exec.CommandContext(ctx, "/sbin/mount_webdav", "-S", "-v", mountName, url, mountPoint)
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.Remove(mountPoint)
		return MountResult{
			URL:       url,
			Automatic: false,
			Message:   "The profile or macOS blocked automatic WebDAV mounting. The local URL remains available.",
		}, fmt.Errorf("mount WebDAV: %w: %s", err, string(output))
	}
	return MountResult{
		URL:        url,
		MountPoint: mountPoint,
		Automatic:  true,
		Message:    "Mounted with macOS WebDAV without FUSE.",
	}, nil
}

func (nativeMounter) Unmount(ctx context.Context, mount MountResult) error {
	if mount.MountPoint == "" {
		return nil
	}
	command := exec.CommandContext(ctx, "/sbin/umount", mount.MountPoint)
	output, commandErr := command.CombinedOutput()
	removeErr := os.Remove(mount.MountPoint)
	if commandErr != nil {
		commandErr = fmt.Errorf("unmount WebDAV: %w: %s", commandErr, string(output))
	}
	return joinMountErrors(commandErr, removeErr)
}

func (nativeMounter) Reveal(ctx context.Context, mount MountResult) error {
	target := mount.MountPoint
	if target == "" {
		target = mount.URL
	}
	return exec.CommandContext(ctx, "/usr/bin/open", target).Run()
}
