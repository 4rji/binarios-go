package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiCyan    = "\x1b[1;36m"
	ansiGreen   = "\x1b[1;32m"
	ansiYellow  = "\x1b[1;33m"
	ansiRed     = "\x1b[1;31m"
	ansiMagenta = "\x1b[1;35m"
)

var (
	errFZFUnavailable = errors.New("fzf is unavailable")
	errFZFCancelled   = errors.New("fzf selection cancelled")
	errMenuQuit       = errors.New("menu closed")
)

type jsonOutput struct {
	Tool         string     `json:"tool"`
	Version      string     `json:"version"`
	StartedAt    string     `json:"started_at"`
	Duration     string     `json:"duration"`
	Root         string     `json:"root"`
	FilesScanned int        `json:"files_scanned"`
	Systemctl    bool       `json:"systemctl_queried"`
	Warnings     []string   `json:"warnings,omitempty"`
	Findings     []*Finding `json:"findings"`
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func colorize(enabled bool, code, value string) string {
	if !enabled {
		return value
	}
	return code + value + ansiReset
}

func severityColor(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return ansiMagenta
	case SeveritySuspicious:
		return ansiRed
	case SeverityReview:
		return ansiYellow
	default:
		return ansiGreen
	}
}

func severityTag(f *Finding, color bool) string {
	return colorize(color, severityColor(f.Severity), fmt.Sprintf("%-10s", f.Severity.String()))
}

func printSummary(out io.Writer, result ScanResult, findings []*Finding, limit int, color bool) {
	fmt.Fprintf(out, "%s  %s\n", colorize(color, ansiCyan, "SPAUDIT"), colorize(color, ansiDim, "systemd persistence triage"))
	fmt.Fprintf(out, "Scanned %d unit artifacts in %s", result.FilesScanned, result.Duration)
	if result.Systemctl {
		fmt.Fprint(out, "; runtime state included")
	}
	fmt.Fprintln(out, ".")

	counts := severityCounts(findings)
	fmt.Fprintf(out, "Risk  %s %d   %s %d   %s %d   %s %d\n\n",
		colorize(color, ansiMagenta, "critical"), counts[SeverityCritical],
		colorize(color, ansiRed, "suspicious"), counts[SeveritySuspicious],
		colorize(color, ansiYellow, "review"), counts[SeverityReview],
		colorize(color, ansiGreen, "info"), counts[SeverityInfo])

	groups := groupFindings(findings)
	if len(groups) == 0 {
		fmt.Fprintln(out, colorize(color, ansiGreen, "No findings matched the selected filters."))
	} else {
		fmt.Fprintln(out, colorize(color, ansiBold, "Categories"))
		for _, group := range groups {
			fmt.Fprintf(out, "  %-14s %4d  highest %-10s score %3d\n", group.Name, len(group.Findings), group.Findings[0].Severity.String(), group.Findings[0].Score)
		}
	}

	if limit > len(findings) {
		limit = len(findings)
	}
	if limit > 0 {
		fmt.Fprintf(out, "\n%s\n", colorize(color, ansiBold, "Highest-risk findings"))
		for _, finding := range findings[:limit] {
			location := finding.Unit
			if location == "" {
				location = finding.Path
			}
			fmt.Fprintf(out, "  %s %3d  %-12s  %-28s  %s\n", severityTag(finding, color), finding.Score, truncate(finding.Category, 12), truncate(location, 28), finding.Title)
		}
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(out, "\n%s %s\n", colorize(color, ansiYellow, "Warning:"), warning)
	}
	if isTerminal(os.Stdout) {
		fmt.Fprintln(out, "\nUse --view fzf to search and preview, --view menu for the built-in browser, or --view full for every detail.")
	}
	fmt.Fprintln(out, colorize(color, ansiDim, "Read-only scan. Findings are triage signals, not proof of compromise."))
}

func printFull(out io.Writer, result ScanResult, findings []*Finding, color bool) {
	printSummary(out, result, findings, 0, color)
	groups := groupFindings(findings)
	for _, group := range groups {
		fmt.Fprintf(out, "\n%s\n", colorize(color, ansiCyan, "== "+group.Name+" =="))
		for _, finding := range group.Findings {
			printFinding(out, finding, color)
		}
	}
}

func printFinding(out io.Writer, finding *Finding, color bool) {
	fmt.Fprintf(out, "\n%s score=%d  %s\n", severityTag(finding, color), finding.Score, colorize(color, ansiBold, finding.Title))
	if finding.Unit != "" {
		fmt.Fprintf(out, "  Unit:     %s\n", finding.Unit)
	}
	if finding.Path != "" {
		location := finding.Path
		if finding.Line > 0 {
			location += ":" + strconv.Itoa(finding.Line)
		}
		fmt.Fprintf(out, "  Path:     %s\n", location)
	}
	if finding.Evidence != "" {
		fmt.Fprintf(out, "  Evidence: %s\n", finding.Evidence)
	}
	if finding.Recommendation != "" {
		fmt.Fprintf(out, "  Next:     %s\n", finding.Recommendation)
	}
}

func printJSON(out io.Writer, result ScanResult, findings []*Finding) error {
	payload := jsonOutput{
		Tool: "spaudit", Version: version, StartedAt: result.StartedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		Duration: result.Duration, Root: result.Root, FilesScanned: result.FilesScanned,
		Systemctl: result.Systemctl, Warnings: result.Warnings, Findings: findings,
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

type findingGroup struct {
	Name     string
	Findings []*Finding
}

func groupFindings(findings []*Finding) []findingGroup {
	byName := make(map[string][]*Finding)
	for _, finding := range findings {
		byName[finding.Category] = append(byName[finding.Category], finding)
	}
	groups := make([]findingGroup, 0, len(byName))
	for name, items := range byName {
		sortFindings(items)
		groups = append(groups, findingGroup{Name: name, Findings: items})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Findings[0].Score != groups[j].Findings[0].Score {
			return groups[i].Findings[0].Score > groups[j].Findings[0].Score
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}

func severityCounts(findings []*Finding) map[Severity]int {
	counts := make(map[Severity]int)
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	return counts
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}

func runFZF(findings []*Finding, color bool) error {
	fzf, err := exec.LookPath("fzf")
	if err != nil {
		return errFZFUnavailable
	}
	if len(findings) == 0 {
		fmt.Println("No findings matched the selected filters.")
		return nil
	}
	tempDir, err := os.MkdirTemp("", "spaudit-fzf-")
	if err != nil {
		return fmt.Errorf("create fzf preview directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var rows strings.Builder
	for _, finding := range findings {
		previewPath := filepath.Join(tempDir, finding.ID)
		var preview strings.Builder
		fmt.Fprintf(&preview, "%s\n", colorize(color, severityColor(finding.Severity), finding.Severity.String()+"  score="+strconv.Itoa(finding.Score)))
		fmt.Fprintf(&preview, "%s\n\n", colorize(color, ansiBold, finding.Title))
		fmt.Fprintf(&preview, "Category: %s\n", finding.Category)
		if finding.Unit != "" {
			fmt.Fprintf(&preview, "Unit:     %s\n", finding.Unit)
		}
		if finding.Path != "" {
			fmt.Fprintf(&preview, "Path:     %s", finding.Path)
			if finding.Line > 0 {
				fmt.Fprintf(&preview, ":%d", finding.Line)
			}
			fmt.Fprintln(&preview)
		}
		if finding.Evidence != "" {
			fmt.Fprintf(&preview, "\nEvidence\n%s\n", finding.Evidence)
		}
		if finding.Recommendation != "" {
			fmt.Fprintf(&preview, "\nRecommended next step\n%s\n", finding.Recommendation)
		}
		if err := os.WriteFile(previewPath, []byte(preview.String()), 0600); err != nil {
			return fmt.Errorf("write fzf preview: %w", err)
		}
		location := finding.Unit
		if location == "" {
			location = finding.Path
		}
		fmt.Fprintf(&rows, "%s\t%s\t%03d\t%-12s\t%-28s\t%s\n",
			finding.ID, colorize(color, severityColor(finding.Severity), fmt.Sprintf("%-10s", finding.Severity.String())),
			finding.Score, finding.Category, truncate(location, 28), finding.Title)
	}

	previewCommand := "cat -- " + shellQuote(tempDir) + "/{1}"
	cmd := exec.Command(fzf,
		"--ansi", "--no-sort", "--layout=reverse", "--border", "--multi",
		"--prompt=spaudit> ", "--pointer=▶", "--marker=✓",
		"--header=Risk-sorted. Type to filter • Ctrl-S toggles match sorting • Tab selects • Enter opens",
		"--delimiter=\t", "--with-nth=2..", "--preview="+previewCommand,
		"--preview-window=right,60%,wrap", "--bind=ctrl-s:toggle-sort",
	)
	cmd.Stdin = strings.NewReader(rows.String())
	cmd.Stderr = os.Stderr
	selected, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return errFZFCancelled
		}
		return fmt.Errorf("fzf failed: %w", err)
	}
	selectedIDs := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(selected)), "\n") {
		if fields := strings.SplitN(line, "\t", 2); len(fields) > 0 {
			selectedIDs[fields[0]] = true
		}
	}
	if len(selectedIDs) > 0 {
		fmt.Println()
		for _, finding := range findings {
			if selectedIDs[finding.ID] {
				printFinding(os.Stdout, finding, color)
			}
		}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func runMenu(result ScanResult, findings []*Finding, color bool) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		clearScreen(os.Stdout)
		printSummary(os.Stdout, result, findings, 0, color)
		groups := groupFindings(findings)
		if len(groups) == 0 {
			return nil
		}
		fmt.Fprintln(os.Stdout, "\nSelect a category:")
		for i, group := range groups {
			fmt.Fprintf(os.Stdout, "  %2d) %-14s %4d findings  top score %d\n", i+1, group.Name, len(group.Findings), group.Findings[0].Score)
		}
		fmt.Fprint(os.Stdout, "\nCategory number, [a]ll, or [q]uit: ")
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		choice = strings.TrimSpace(choice)
		if choice == "q" || choice == "Q" || errors.Is(err, io.EOF) {
			return nil
		}
		if choice == "a" || choice == "A" {
			if err := pageFindings(reader, "All findings", findings, color); err != nil {
				if errors.Is(err, errMenuQuit) {
					return nil
				}
				return err
			}
			continue
		}
		index, convErr := strconv.Atoi(choice)
		if convErr != nil || index < 1 || index > len(groups) {
			continue
		}
		if err := pageFindings(reader, groups[index-1].Name, groups[index-1].Findings, color); err != nil {
			if errors.Is(err, errMenuQuit) {
				return nil
			}
			return err
		}
	}
}

func pageFindings(reader *bufio.Reader, title string, findings []*Finding, color bool) error {
	const pageSize = 8
	for start := 0; start < len(findings); start += pageSize {
		clearScreen(os.Stdout)
		end := start + pageSize
		if end > len(findings) {
			end = len(findings)
		}
		fmt.Fprintf(os.Stdout, "%s  %d-%d of %d\n", colorize(color, ansiCyan, title), start+1, end, len(findings))
		for _, finding := range findings[start:end] {
			printFinding(os.Stdout, finding, color)
		}
		if end == len(findings) {
			fmt.Fprint(os.Stdout, "\n[Enter] back to categories, [q] quit: ")
		} else {
			fmt.Fprint(os.Stdout, "\n[Enter] next page, [b] back, [q] quit: ")
		}
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		choice = strings.TrimSpace(choice)
		if choice == "q" || choice == "Q" || errors.Is(err, io.EOF) {
			return errMenuQuit
		}
		if choice == "b" || choice == "B" || end == len(findings) {
			return nil
		}
	}
	return nil
}

func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\x1b[2J\x1b[H")
}
