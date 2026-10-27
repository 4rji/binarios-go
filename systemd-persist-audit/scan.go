package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Severity int

const (
	SeverityInfo Severity = iota + 1
	SeverityReview
	SeveritySuspicious
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeveritySuspicious:
		return "SUSPICIOUS"
	case SeverityReview:
		return "REVIEW"
	default:
		return "INFO"
	}
}

func parseSeverity(value string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info", "low":
		return SeverityInfo, nil
	case "review", "medium":
		return SeverityReview, nil
	case "suspicious", "high":
		return SeveritySuspicious, nil
	case "critical", "crit":
		return SeverityCritical, nil
	default:
		return 0, fmt.Errorf("invalid severity %q (use info, review, suspicious, or critical)", value)
	}
}

type Finding struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Severity       Severity `json:"-"`
	SeverityLabel  string   `json:"severity"`
	Score          int      `json:"score"`
	Title          string   `json:"title"`
	Unit           string   `json:"unit,omitempty"`
	Path           string   `json:"path,omitempty"`
	Line           int      `json:"line,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type ScanResult struct {
	StartedAt    time.Time `json:"started_at"`
	Duration     string    `json:"duration"`
	Root         string    `json:"root"`
	FilesScanned int       `json:"files_scanned"`
	Systemctl    bool      `json:"systemctl_queried"`
	Warnings     []string  `json:"warnings,omitempty"`
	Findings     []Finding `json:"-"`
}

type scanOptions struct {
	Root         string
	UseSystemctl bool
}

type directive struct {
	Section string
	Key     string
	Value   string
	Line    int
	Raw     string
}

var (
	riskyPathRE = regexp.MustCompile(`(?i)(^|[[:space:]='" ])(/tmp/|/var/tmp/|/dev/shm/|/home/[^/]+/|/root/)`)
	networkRE   = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(curl|wget|nc|ncat|netcat|socat|telnet)([^[:alnum:]_]|$)`)
	shellRE     = regexp.MustCompile(`(?i)(^|[[:space:]])(/usr)?/bin/(ba|da|z|k)?sh[[:space:]]+(-[^[:space:]]*)?c([[:space:]]|$)`)
	encodedRE   = regexp.MustCompile(`(?i)(base64[[:space:]].*(-d|--decode)|python[0-9.]*[[:space:]]+-c|perl[[:space:]]+-e)`)
)

func normalizeRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("--root cannot be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("invalid --root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("cannot access --root %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--root %q is not a directory", abs)
	}
	return filepath.Clean(abs), nil
}

func scan(opts scanOptions) ScanResult {
	started := time.Now()
	result := ScanResult{StartedAt: started.UTC(), Root: opts.Root}
	dirs := activeSystemdDirs(opts.Root)
	seenPaths := make(map[string]bool)
	unitPaths := make(map[string]string)

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("cannot inspect %s: %v", displayPath(opts.Root, path), walkErr))
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !isUnitArtifact(path) || seenPaths[path] {
				return nil
			}
			seenPaths[path] = true
			result.FilesScanned++
			unit := unitName(path)
			if strings.HasSuffix(unit, ".service") || strings.HasSuffix(unit, ".timer") {
				if _, exists := unitPaths[unit]; !exists || strings.Contains(path, filepath.Join("etc", "systemd")) {
					unitPaths[unit] = path
				}
			}
			result.Findings = append(result.Findings, inspectArtifact(opts.Root, path, unit)...)
			return nil
		})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("cannot walk %s: %v", displayPath(opts.Root, dir), err))
		}
	}

	if len(dirs) == 0 {
		result.Warnings = append(result.Warnings, "no systemd unit directories were found")
	}

	if opts.UseSystemctl && opts.Root == string(filepath.Separator) {
		enabled, active, warnings, queried := querySystemd()
		result.Systemctl = queried
		result.Warnings = append(result.Warnings, warnings...)
		result.Findings = append(result.Findings, runtimeFindings(enabled, active, unitPaths, opts.Root)...)
	}

	result.Findings = deduplicate(result.Findings)
	for i := range result.Findings {
		result.Findings[i].SeverityLabel = result.Findings[i].Severity.String()
	}
	result.Duration = time.Since(started).Round(time.Millisecond).String()
	return result
}

func activeSystemdDirs(root string) []string {
	relative := []string{
		"etc/systemd/system",
		"run/systemd/system",
		"usr/local/lib/systemd/system",
		"usr/lib/systemd/system",
		"lib/systemd/system",
	}
	var dirs []string
	seen := make(map[string]bool)
	for _, rel := range relative {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			canonical = path
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		dirs = append(dirs, path)
	}
	return dirs
}

func isUnitArtifact(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".service") || strings.HasSuffix(base, ".timer") {
		return true
	}
	return strings.HasSuffix(base, ".conf") && strings.HasSuffix(filepath.Base(filepath.Dir(path)), ".d")
}

func unitName(path string) string {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".conf") {
		parent := filepath.Base(filepath.Dir(path))
		return strings.TrimSuffix(parent, ".d")
	}
	return base
}

func inspectArtifact(root, path, unit string) []Finding {
	shownPath := displayPath(root, path)
	var findings []Finding
	info, err := os.Lstat(path)
	if err != nil {
		return []Finding{newFinding("Scanner", SeverityReview, 45, "Unit metadata could not be read", unit, shownPath, 0, err.Error(), "Run as root and inspect the file manually.")}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(path)
		if readErr != nil {
			findings = append(findings, newFinding("Symlinks", SeverityReview, 45, "Unreadable unit symlink", unit, shownPath, 0, readErr.Error(), "Inspect the symlink and its parent directory."))
			return findings
		}
		if target == "/dev/null" {
			findings = append(findings, newFinding("Symlinks", SeverityInfo, 10, "Masked unit", unit, shownPath, 0, "symlink -> /dev/null", "Confirm that masking this unit is intentional."))
		} else if riskyPathRE.MatchString(" " + target) {
			findings = append(findings, newFinding("Symlinks", SeveritySuspicious, 85, "Unit symlink targets a risky location", unit, shownPath, 0, "symlink -> "+target, "Verify the target owner, contents, and provenance."))
		}
		return findings
	}

	mode := info.Mode().Perm()
	if mode&0002 != 0 {
		findings = append(findings, newFinding("Permissions", SeverityCritical, 100, "World-writable systemd artifact", unit, shownPath, 0, mode.String(), "Remove world-write access and investigate recent modifications."))
	} else if mode&0020 != 0 {
		findings = append(findings, newFinding("Permissions", SeveritySuspicious, 80, "Group-writable systemd artifact", unit, shownPath, 0, mode.String(), "Confirm the group membership and whether group write access is required."))
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Uid != 0 {
		findings = append(findings, newFinding("Permissions", SeveritySuspicious, 90, "System unit is not owned by root", unit, shownPath, 0, "owner uid="+strconv.FormatUint(uint64(stat.Uid), 10), "Restore the expected owner only after validating the file's provenance."))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		findings = append(findings, newFinding("Scanner", SeverityReview, 45, "Unit content could not be read", unit, shownPath, 0, err.Error(), "Run as root and inspect the file manually."))
		return findings
	}
	if len(content) > 4<<20 {
		findings = append(findings, newFinding("Scanner", SeverityReview, 50, "Unusually large systemd artifact", unit, shownPath, 0, fmt.Sprintf("size=%d bytes", len(content)), "Inspect the file for embedded or generated content."))
	}

	directives := parseDirectives(string(content))
	if strings.HasSuffix(filepath.Base(filepath.Dir(path)), ".d") {
		findings = append(findings, newFinding("Overrides", SeverityReview, 55, "Drop-in changes a unit's effective configuration", unit, shownPath, 0, "drop-in: "+filepath.Base(path), "Compare with the vendor unit using systemd-delta or systemctl cat."))
	}
	if strings.Contains(shownPath, "/etc/systemd/system/") && !strings.HasSuffix(filepath.Base(filepath.Dir(path)), ".d") {
		findings = append(findings, newFinding("Local units", SeverityReview, 35, "Locally managed systemd unit", unit, shownPath, 0, "unit is stored under /etc/systemd/system", "Validate it against the host baseline and change records."))
	}

	var timerParts []string
	for _, d := range directives {
		key := strings.ToLower(d.Key)
		value := strings.TrimSpace(d.Value)
		evidence := fmt.Sprintf("%s=%s", d.Key, value)
		switch {
		case strings.HasPrefix(key, "exec") || key == "environmentfile":
			if riskyPathRE.MatchString(" " + value) {
				findings = append(findings, newFinding("Execution", SeveritySuspicious, 92, "Command or environment uses a risky writable location", unit, shownPath, d.Line, evidence, "Inspect the referenced file, owner, hashes, and recent changes."))
			}
			if networkRE.MatchString(value) {
				findings = append(findings, newFinding("Execution", SeveritySuspicious, 88, "Unit invokes a network transfer tool", unit, shownPath, d.Line, evidence, "Confirm the destination and why network retrieval is needed at service start."))
			}
			if shellRE.MatchString(value) {
				findings = append(findings, newFinding("Execution", SeveritySuspicious, 84, "Unit launches a shell command", unit, shownPath, d.Line, evidence, "Review quoting, expansions, and every command executed by the shell."))
			}
			if encodedRE.MatchString(value) {
				findings = append(findings, newFinding("Execution", SeveritySuspicious, 90, "Unit executes encoded or inline code", unit, shownPath, d.Line, evidence, "Decode and review the payload without executing it."))
			}
		case key == "restart" && (strings.EqualFold(value, "always") || strings.EqualFold(value, "on-failure") || strings.EqualFold(value, "on-abort")):
			findings = append(findings, newFinding("Resilience", SeverityInfo, 20, "Service automatically restarts", unit, shownPath, d.Line, evidence, "Confirm the restart policy matches the service baseline."))
		case key == "remainafterexit" && strings.EqualFold(value, "yes"):
			findings = append(findings, newFinding("Resilience", SeverityInfo, 22, "Service remains active after its process exits", unit, shownPath, d.Line, evidence, "Confirm this oneshot-style behavior is intended."))
		case strings.HasSuffix(unit, ".timer") && (strings.HasPrefix(key, "oncalendar") || strings.HasPrefix(key, "onbootsec") || strings.HasPrefix(key, "onstartupsec") || strings.HasPrefix(key, "onunitactive") || strings.HasPrefix(key, "onunitinactive") || key == "persistent" || key == "unit"):
			timerParts = append(timerParts, evidence)
		}
	}
	if len(timerParts) > 0 {
		findings = append(findings, newFinding("Timers", SeverityReview, 42, "Timer provides scheduled execution", unit, shownPath, 0, strings.Join(timerParts, "; "), "Validate the schedule and inspect the service triggered by this timer."))
	}
	return findings
}

func parseDirectives(content string) []directive {
	scanner := bufio.NewScanner(strings.NewReader(content))
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 4<<20)
	section := ""
	lineNo := 0
	logicalStart := 0
	logical := ""
	var directives []directive
	flush := func() {
		line := strings.TrimSpace(logical)
		logical = ""
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			return
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			return
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return
		}
		directives = append(directives, directive{Section: section, Key: strings.TrimSpace(line[:idx]), Value: strings.TrimSpace(line[idx+1:]), Line: logicalStart, Raw: line})
	}
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if logical == "" {
			logicalStart = lineNo
		}
		trimmedRight := strings.TrimRight(line, " \t\r")
		if strings.HasSuffix(trimmedRight, "\\") {
			logical += strings.TrimSuffix(trimmedRight, "\\") + " "
			continue
		}
		logical += line
		flush()
	}
	if logical != "" {
		flush()
	}
	return directives
}

func querySystemd() (map[string]string, map[string]bool, []string, bool) {
	enabled := make(map[string]string)
	active := make(map[string]bool)
	if _, err := exec.LookPath("systemctl"); err != nil {
		return enabled, active, []string{"systemctl is unavailable; runtime state was not collected"}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "list-unit-files", "--type=service", "--type=timer", "--state=enabled", "--no-legend", "--no-pager", "--plain")
	cmd.Env = append(os.Environ(), "SYSTEMD_PAGER=cat", "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return enabled, active, []string{"systemctl could not query enabled units: " + conciseError(err)}, true
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			enabled[fields[0]] = fields[1]
		}
	}

	ctxActive, cancelActive := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancelActive()
	activeCmd := exec.CommandContext(ctxActive, "systemctl", "list-units", "--type=service", "--type=timer", "--all", "--no-legend", "--no-pager", "--plain")
	activeCmd.Env = append(os.Environ(), "SYSTEMD_PAGER=cat", "LC_ALL=C")
	activeOutput, activeErr := activeCmd.Output()
	var warnings []string
	if activeErr != nil {
		warnings = append(warnings, "systemctl could not query active units: "+conciseError(activeErr))
	} else {
		for _, line := range strings.Split(string(activeOutput), "\n") {
			fields := strings.Fields(strings.TrimLeft(line, "● "))
			if len(fields) >= 4 && fields[2] == "active" {
				active[fields[0]] = true
			}
		}
	}
	return enabled, active, warnings, true
}

func conciseError(err error) string {
	if exitErr, ok := err.(*exec.ExitError); ok {
		message := strings.TrimSpace(string(exitErr.Stderr))
		if message != "" {
			return message
		}
	}
	return err.Error()
}

func runtimeFindings(enabled map[string]string, active map[string]bool, paths map[string]string, root string) []Finding {
	units := make([]string, 0, len(enabled))
	for unit := range enabled {
		units = append(units, unit)
	}
	sort.Strings(units)
	findings := make([]Finding, 0, len(units))
	for _, unit := range units {
		category := "Services"
		score := 25
		title := "Service is enabled at boot"
		recommendation := "Validate this enabled service against the host baseline."
		if strings.HasSuffix(unit, ".timer") {
			category = "Timers"
			score = 48
			title = "Timer is enabled"
			recommendation = "Validate the timer schedule and the service it triggers."
		}
		evidence := "state=" + enabled[unit]
		if active[unit] {
			evidence += "; active=yes"
		} else {
			evidence += "; active=no"
		}
		path := ""
		if stored, ok := paths[unit]; ok {
			path = displayPath(root, stored)
		}
		findings = append(findings, newFinding(category, SeverityReview, score, title, unit, path, 0, evidence, recommendation))
	}
	return findings
}

func newFinding(category string, severity Severity, score int, title, unit, path string, line int, evidence, recommendation string) Finding {
	return Finding{Category: category, Severity: severity, SeverityLabel: severity.String(), Score: score, Title: title, Unit: unit, Path: path, Line: line, Evidence: evidence, Recommendation: recommendation}
}

func deduplicate(findings []Finding) []Finding {
	seen := make(map[string]bool)
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		key := strings.Join([]string{f.Category, f.Title, f.Unit, f.Path, strconv.Itoa(f.Line), f.Evidence}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func displayPath(root, path string) string {
	if root == string(filepath.Separator) {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return string(filepath.Separator) + filepath.ToSlash(rel)
}
