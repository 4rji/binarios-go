//go:build !darwin && !linux

package main

import (
	"context"
	"errors"
)

type nativeMounter struct{}

func newPlatformMounter() platformMounter { return nativeMounter{} }

func (nativeMounter) Mount(ctx context.Context, name, url, dataRoot string) (MountResult, error) {
	return MountResult{URL: url, Message: "No automatic WebDAV mounter is available on this platform."}, errors.New("unsupported platform")
}

func (nativeMounter) Unmount(ctx context.Context, mount MountResult) error { return nil }

func (nativeMounter) Reveal(ctx context.Context, mount MountResult) error {
	return errors.New("unsupported platform")
}
