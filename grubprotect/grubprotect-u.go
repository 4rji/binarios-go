package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	stateDir  = "/var/lib/ubuntu-update"
	filesList = stateDir + "/.state.list"
	manifest  = stateDir + "/.state.hash"
	meta      = stateDir + "/.state.meta"
	stateTar  = stateDir + "/.state.tgz"
)

func appName() string {
	return filepath.Base(os.Args[0])
}

func logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", appName(), msg)
}

func die(msg string) {
	logf("ERROR: %s", msg)
	os.Exit(1)
}

func needRoot() {
	if os.Geteuid() == 0 {
		return
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		cmd := exec.Command("sudo", os.Args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				os.Exit(ee.ExitCode())
			}
			die(err.Error())
		}
		os.Exit(0)
	}
	die("Run as root.")
}

func have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func detectRebuildCmd() []string {
	if have("update-grub") {
		return []string{"update-grub"}
	}
	if have("grub2-mkconfig") {
		if _, err := os.Stat("/boot/grub2/grub.cfg"); err == nil {
			return []string{"grub2-mkconfig", "-o", "/boot/grub2/grub.cfg"}
		}
		if _, err := os.Stat("/boot/grub/grub.cfg"); err == nil {
			return []string{"grub2-mkconfig", "-o", "/boot/grub/grub.cfg"}
		}
		return []string{"grub2-mkconfig", "-o", "/boot/grub2/grub.cfg"}
	}
	if have("grub-mkconfig") {
		if _, err := os.Stat("/boot/grub/grub.cfg"); err == nil {
			return []string{"grub-mkconfig", "-o", "/boot/grub/grub.cfg"}
		}
		return []string{"grub-mkconfig", "-o", "/boot/grub/grub.cfg"}
	}
	return nil
}

func detectFiles() []string {
	var paths []string
	paths = append(paths, "/etc/default/grub")
	if st, err := os.Stat("/etc/grub.d"); err == nil && st.IsDir() {
		paths = append(paths, "/etc/grub.d")
	}
	if _, err := os.Stat("/boot/grub/grub.cfg"); err == nil {
		paths = append(paths, "/boot/grub/grub.cfg")
	}
	if _, err := os.Stat("/boot/grub2/grub.cfg"); err == nil {
		paths = append(paths, "/boot/grub2/grub.cfg")
	}

	if st, err := os.Stat("/boot/efi/EFI"); err == nil && st.IsDir() {
		walkMaxDepth("/boot/efi/EFI", 3, func(p string, d fs.DirEntry) {
			if d.IsDir() {
				return
			}
			name := strings.ToLower(d.Name())
			if (strings.HasPrefix(name, "grub") && strings.HasSuffix(name, ".cfg")) ||
				strings.HasSuffix(name, ".efi") ||
				(strings.HasPrefix(name, "shim") && strings.HasSuffix(name, ".efi")) {
				paths = append(paths, p)
			}
		})
	}

	uniq := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			uniq[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(uniq))
	for p := range uniq {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func walkMaxDepth(root string, maxDepth int, fn func(string, fs.DirEntry)) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(os.PathSeparator)) + 1
		}
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		fn(p, d)
		return nil
	})
}

func mkStateDir() {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		die(err.Error())
	}
	_ = os.Chmod(stateDir, 0700)
	_ = os.Chown(stateDir, 0, 0)
}

func writeFilesList() {
	files := detectFiles()
	if len(files) == 0 {
		die("No GRUB-related files found to protect.")
	}
	f, err := os.Create(filesList)
	if err != nil {
		die(err.Error())
	}
	defer f.Close()
	for _, p := range files {
		_, _ = fmt.Fprintln(f, p)
	}
}

func listTargetFiles(target string) []string {
	var out []string
	info, err := os.Stat(target)
	if err != nil {
		return out
	}
	if info.IsDir() {
		_ = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			out = append(out, p)
			return nil
		})
	} else {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

func hashTargets() {
	f, err := os.Create(manifest)
	if err != nil {
		die(err.Error())
	}
	defer f.Close()

	for _, target := range readLines(filesList) {
		for _, p := range listTargetFiles(target) {
			h, ok := hashFile(p)
			if !ok {
				continue
			}
			_, _ = fmt.Fprintf(f, "%s  %s\n", h, p)
		}
	}
}

func hashFile(p string) (string, bool) {
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func metaSnapshot() {
	f, err := os.Create(meta)
	if err != nil {
		die(err.Error())
	}
	defer f.Close()

	for _, target := range readLines(filesList) {
		for _, p := range listTargetFiles(target) {
			line := metaLine(p)
			if line == "" {
				continue
			}
			_, _ = fmt.Fprintln(f, line)
		}
	}
}

func metaLine(p string) string {
	info, err := os.Stat(p)
	if err != nil {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	uid := strconv.Itoa(int(st.Uid))
	if u, err := user.LookupId(uid); err == nil {
		uid = u.Username
	}
	gid := strconv.Itoa(int(st.Gid))
	mode := strconv.FormatUint(uint64(info.Mode().Perm()), 8)
	mtime := strconv.FormatInt(info.ModTime().Unix(), 10)
	return fmt.Sprintf("%s|%s|%s|%s|%s", p, uid, gid, mode, mtime)
}

func makeStateTar() {
	f, err := os.Create(stateTar)
	if err != nil {
		die(err.Error())
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	seenDirs := make(map[string]struct{})
	for _, target := range readLines(filesList) {
		addPathToTar(tw, target, seenDirs)
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			_ = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				addPathToTar(tw, p, seenDirs)
				return nil
			})
		}
	}

	_ = os.Chown(stateTar, 0, 0)
	_ = os.Chmod(stateTar, 0600)
}

func addPathToTar(tw *tar.Writer, p string, seen map[string]struct{}) {
	info, err := os.Lstat(p)
	if err != nil {
		return
	}
	rel := strings.TrimPrefix(p, "/")
	rel = path.Clean(rel)
	if rel == "." || rel == "" {
		return
	}

	if info.IsDir() {
		if _, ok := seen[rel]; ok {
			return
		}
		seen[rel] = struct{}{}
	}

	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		if l, err := os.Readlink(p); err == nil {
			link = l
		}
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return
	}
	hdr.Name = rel
	if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return
	}
	if info.Mode().IsRegular() {
		f, err := os.Open(p)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = io.Copy(tw, f)
	}
}

func restoreFromState() {
	f, err := os.Open(stateTar)
	if err != nil {
		die("No state archive found. Run: " + appName() + " init")
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		die(err.Error())
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			die(err.Error())
		}
		name := path.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") || strings.Contains(name, "../") {
			continue
		}
		dst := filepath.Join("/", filepath.FromSlash(name))
		if err := restoreEntry(dst, hdr, tr); err != nil {
			logf("Restore warning: %s", err)
		}
	}
}

func restoreEntry(dst string, hdr *tar.Header, tr *tar.Reader) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(dst, os.FileMode(hdr.Mode)); err != nil {
			return err
		}
		_ = os.Chtimes(dst, time.Unix(hdr.AccessTime.Unix(), 0), time.Unix(hdr.ModTime.Unix(), 0))
		_ = os.Chown(dst, hdr.Uid, hdr.Gid)
	case tar.TypeSymlink:
		_ = os.RemoveAll(dst)
		if err := os.Symlink(hdr.Linkname, dst); err != nil {
			return err
		}
		_ = os.Lchown(dst, hdr.Uid, hdr.Gid)
	default:
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
		_ = os.Chtimes(dst, time.Now(), time.Unix(hdr.ModTime.Unix(), 0))
		_ = os.Chown(dst, hdr.Uid, hdr.Gid)
	}
	return nil
}

func checkIntegrity() bool {
	if !fileExists(filesList) || !fileExists(manifest) || !fileExists(stateTar) {
		die("Not initialized. Run: " + appName() + " init")
	}
	cur := strings.Join(computeCurrentHashes(), "\n") + "\n"
	prev, err := os.ReadFile(manifest)
	if err != nil {
		return false
	}
	return string(prev) == cur
}

func computeCurrentHashes() []string {
	var lines []string
	for _, target := range readLines(filesList) {
		for _, p := range listTargetFiles(target) {
			h, ok := hashFile(p)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s  %s", h, p))
		}
	}
	return lines
}

func preventDeletionLock() {
	if !have("chattr") {
		logf("chattr not found; skipping lock.")
		return
	}
	_ = exec.Command("chattr", "-R", "+i", stateDir).Run()
	for _, p := range []string{filesList, manifest, meta, stateTar} {
		_ = exec.Command("chattr", "+i", p).Run()
	}
}

func preventDeletionUnlock() {
	if !have("chattr") {
		logf("chattr not found; nothing to unlock.")
		return
	}
	_ = exec.Command("chattr", "-R", "-i", stateDir).Run()
	for _, p := range []string{filesList, manifest, meta, stateTar} {
		_ = exec.Command("chattr", "-i", p).Run()
	}
}

func cmdInit() {
	needRoot()
	mkStateDir()
	writeFilesList()
	hashTargets()
	metaSnapshot()
	makeStateTar()
	logf("Initialized baseline. Files tracked: %d", len(readLines(filesList)))
}

func cmdStatus() {
	needRoot()
	if !fileExists(filesList) {
		die("Not initialized. Run: " + appName() + " init")
	}
	fmt.Printf("STATE_DIR: %s\n", stateDir)
	fmt.Println("Tracked targets:")
	for _, p := range readLines(filesList) {
		fmt.Printf("  - %s\n", p)
	}
}

func cmdCheck(noRebuild bool) {
	needRoot()
	if checkIntegrity() {
		logf("OK (no changes)")
		return
	}
	logf("CHANGE DETECTED -> restoring baseline")
	preventDeletionUnlock()
	restoreFromState()

	if !noRebuild {
		if rebuild := detectRebuildCmd(); len(rebuild) > 0 {
			logf("Rebuilding grub config: %s", strings.Join(rebuild, " "))
			cmd := exec.Command(rebuild[0], rebuild[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		} else {
			logf("No grub rebuild command found; warning only.")
		}
	}

	writeFilesList()
	hashTargets()
	metaSnapshot()
	makeStateTar()
	preventDeletionLock()
	logf("Restored baseline complete.")
}

func cmdRestore() {
	needRoot()
	preventDeletionUnlock()
	restoreFromState()
	logf("Restore done.")
}

func cmdLock() {
	needRoot()
	preventDeletionLock()
	logf("Lock attempted.")
}

func cmdUnlock() {
	needRoot()
	preventDeletionUnlock()
	logf("Unlock attempted.")
}

func usage() {
	fmt.Printf("Usage: %s <init|check|restore|status|lock|unlock> [--no-rebuild]\n\n", appName())
	fmt.Printf("Examples:\n")
	fmt.Printf("  %s init\n", appName())
	fmt.Printf("  %s check\n", appName())
	fmt.Printf("  %s check --no-rebuild\n", appName())
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "init":
		cmdInit()
	case "check":
		noRebuild := len(args) > 0 && args[0] == "--no-rebuild"
		cmdCheck(noRebuild)
	case "restore":
		cmdRestore()
	case "status":
		cmdStatus()
	case "lock":
		cmdLock()
	case "unlock":
		cmdUnlock()
	default:
		usage()
		os.Exit(1)
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readLines(p string) []string {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

