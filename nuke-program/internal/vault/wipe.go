package vault

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type WipeProgress struct {
	Phase   string  `json:"phase"`
	Percent float64 `json:"percent"`
}

func Wipe(containerPath, root string, progress func(WipeProgress)) error {
	validatedPath, err := validateWipeTarget(containerPath, root)
	if err != nil {
		return err
	}
	before, err := os.Lstat(validatedPath)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(validatedPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	after, err := f.Stat()
	if err != nil {
		return err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return errors.New("wipe target changed during validation")
	}
	size := after.Size()
	if size <= int64(len(fileMagic)+4) {
		return ErrInvalidVault
	}

	if progress != nil {
		progress(WipeProgress{Phase: "destroying-keys", Percent: 0})
	}
	headerBytes := min(size, 1<<20)
	if err := overwriteRange(f, 0, headerBytes, randomFill, nil); err != nil {
		return fmt.Errorf("destroy key material: %w", err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if progress != nil {
		progress(WipeProgress{Phase: "destroying-keys", Percent: 100})
	}

	if err := overwriteRange(f, 0, size, randomFill, func(written int64) {
		if progress != nil {
			progress(WipeProgress{Phase: "random-pass", Percent: percent(written, size)})
		}
	}); err != nil {
		return fmt.Errorf("random wipe pass: %w", err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := overwriteRange(f, 0, size, zeroFill, func(written int64) {
		if progress != nil {
			progress(WipeProgress{Phase: "zero-pass", Percent: percent(written, size)})
		}
	}); err != nil {
		return fmt.Errorf("zero wipe pass: %w", err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	if err := os.Remove(validatedPath); err != nil {
		return err
	}
	if err := syncDir(filepath.Dir(validatedPath)); err != nil {
		return err
	}
	if progress != nil {
		progress(WipeProgress{Phase: "deleted", Percent: 100})
	}
	return nil
}

func validateWipeTarget(containerPath, root string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve managed vault directory: %w", err)
	}
	targetAbs, err := filepath.Abs(containerPath)
	if err != nil {
		return "", err
	}
	if filepath.Ext(targetAbs) != ".nukevol" {
		return "", errors.New("refusing to wipe a non-.nukevol target")
	}
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(targetAbs))
	if err != nil {
		return "", fmt.Errorf("resolve target directory: %w", err)
	}
	relative, err := filepath.Rel(rootResolved, filepath.Join(parentResolved, filepath.Base(targetAbs)))
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", errors.New("refusing to wipe outside the managed vault directory")
	}
	info, err := os.Lstat(targetAbs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("refusing to wipe a symlink or non-regular file")
	}
	f, _, err := openRegular(targetAbs)
	if err != nil {
		return "", err
	}
	magic := make([]byte, len(fileMagic))
	_, readErr := io.ReadFull(f, magic)
	closeErr := f.Close()
	if readErr != nil || string(magic) != fileMagic {
		return "", ErrInvalidVault
	}
	if closeErr != nil {
		return "", closeErr
	}
	return targetAbs, nil
}

type fillFunc func([]byte) error

func randomFill(buffer []byte) error {
	_, err := io.ReadFull(rand.Reader, buffer)
	return err
}

func zeroFill(buffer []byte) error {
	clear(buffer)
	return nil
}

func overwriteRange(f *os.File, offset, size int64, fill fillFunc, update func(int64)) error {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	buffer := make([]byte, 1<<20)
	var written int64
	for written < size {
		chunk := min(int64(len(buffer)), size-written)
		current := buffer[:chunk]
		if err := fill(current); err != nil {
			return err
		}
		n, err := f.Write(current)
		written += int64(n)
		if update != nil {
			update(written)
		}
		if err != nil {
			return err
		}
		if n != len(current) {
			return io.ErrShortWrite
		}
	}
	return nil
}

func percent(value, total int64) float64 {
	if total <= 0 {
		return 100
	}
	return float64(value) * 100 / float64(total)
}
