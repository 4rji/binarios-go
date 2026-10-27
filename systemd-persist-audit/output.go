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
	fmt.Fprintf(out, "%s  %s\n", colorize(color, ansiCyan, "SYSTEMD-PERSIST-AUDIT"), colorize(color, ansiDim, "systemd persistence triage"))
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
		Tool: "systemd-persist-audit", Version: version, StartedAt: result.StartedAt.Format("2006-01-02T15:04:05.000Z07:00"),
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
	tempDir, err := os.MkdirTemp("", "systemd-persist-audit-fzf-")
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
		"--prompt=systemd-audit> ", "--pointer=▶", "--marker=✓",
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
	return runMenuIO(reader, os.Stdout, result, findings, color)
}

const menuPageSize = 7

type menuState struct {
	query    string
	category string
	minLevel Severity
	page     int
}

func runMenuIO(reader *bufio.Reader, out io.Writer, result ScanResult, findings []*Finding, color bool) error {
	state := menuState{minLevel: SeverityInfo}
	for {
		visible := filterMenuFindings(findings, state.query, state.category, state.minLevel)
		pages := pageCount(len(visible), menuPageSize)
		if state.page >= pages {
			state.page = pages - 1
		}
		if state.page < 0 {
			state.page = 0
		}

		clearScreen(out)
		renderMenu(out, result, visible, state, color)
		fmt.Fprint(out, "\nCommand: ")
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		choice = strings.TrimSpace(choice)
		switch strings.ToLower(choice) {
		case "q", "quit", "exit":
			return nil
		case "h", "?", "help":
			if err := showMenuHelp(reader, out, color); err != nil {
				return err
			}
		case "w", "warnings":
			if err := showMenuWarnings(reader, out, result.Warnings, color); err != nil {
				return err
			}
		case "r", "reset":
			state = menuState{minLevel: SeverityInfo}
		case "s", "severity":
			state.minLevel = nextSeverity(state.minLevel)
			state.page = 0
		case "c", "category":
			category, err := chooseMenuCategory(reader, out, findings, state.category, color)
			if err != nil {
				return err
			}
			state.category = category
			state.page = 0
		case "", "n", "next":
			if pages > 1 {
				state.page = (state.page + 1) % pages
			}
		case "p", "prev", "previous":
			if pages > 1 {
				state.page = (state.page - 1 + pages) % pages
			}
		default:
			if strings.HasPrefix(choice, "/") {
				state.query = strings.TrimSpace(strings.TrimPrefix(choice, "/"))
				state.page = 0
				continue
			}
			index, convErr := strconv.Atoi(choice)
			firstOnPage := state.page*menuPageSize + 1
			lastOnPage := firstOnPage + menuPageSize - 1
			if lastOnPage > len(visible) {
				lastOnPage = len(visible)
			}
			if convErr != nil || index < firstOnPage || index > lastOnPage {
				continue
			}
			if err := browseFinding(reader, out, visible, index-1, color); err != nil {
				if errors.Is(err, errMenuQuit) {
					return nil
				}
				return err
			}
		}
	}
}

func renderMenu(out io.Writer, result ScanResult, findings []*Finding, state menuState, color bool) {
	counts := severityCounts(findings)
	pages := pageCount(len(findings), menuPageSize)
	start := state.page * menuPageSize
	end := start + menuPageSize
	if end > len(findings) {
		end = len(findings)
	}

	fmt.Fprintf(out, "%s  %s\n", colorize(color, ansiCyan, "SYSTEMD PERSISTENCE AUDIT"), colorize(color, ansiDim, "read-only triage"))
	fmt.Fprintf(out, "%d artifacts scanned in %s", result.FilesScanned, result.Duration)
	if result.Systemctl {
		fmt.Fprint(out, " | runtime state included")
	}
	fmt.Fprintln(out)
	if len(result.Warnings) > 0 {
		fmt.Fprintf(out, "%s %d (press w to view)\n", colorize(color, ansiYellow, "Warnings:"), len(result.Warnings))
	}
	fmt.Fprintf(out, "Matching risk: %s %d  %s %d  %s %d  %s %d\n",
		colorize(color, ansiMagenta, "critical"), counts[SeverityCritical],
		colorize(color, ansiRed, "suspicious"), counts[SeveritySuspicious],
		colorize(color, ansiYellow, "review"), counts[SeverityReview],
		colorize(color, ansiGreen, "info"), counts[SeverityInfo])

	category := "all"
	if state.category != "" {
		category = state.category
	}
	query := "none"
	if state.query != "" {
		query = strconv.Quote(state.query)
	}
	fmt.Fprintf(out, "Filters: severity >= %-10s  category: %-14s  search: %s\n",
		state.minLevel.String(), category, query)
	printMenuDivider(out, color)
	fmt.Fprintf(out, "%s  %d findings  |  page %d/%d\n\n",
		colorize(color, ansiBold, "Risk-sorted findings"), len(findings), state.page+1, pages)

	if len(findings) == 0 {
		fmt.Fprintln(out, colorize(color, ansiYellow, "No findings match the current filters. Press r to clear them."))
	} else {
		fmt.Fprintln(out, colorize(color, ansiCyan, fmt.Sprintf("  %-4s %-10s %-5s %-13s %s", "#", "RISK", "SCORE", "CATEGORY", "UNIT / PATH")))
		fmt.Fprintln(out, "  ---- ---------- ----- ------------- --------------------------------------")
		for i := start; i < end; i++ {
			finding := findings[i]
			location := finding.Unit
			if location == "" {
				location = finding.Path
			}
			fmt.Fprintf(out, "  %-4d %s %5d %-13s %s\n",
				i+1, severityTag(finding, color), finding.Score, truncate(finding.Category, 13),
				truncate(location, 38))
			fmt.Fprintf(out, "       %s\n", colorize(color, ansiBold, truncate(finding.Title, 70)))
		}
	}

	fmt.Fprintln(out)
	printMenuDivider(out, color)
	fmt.Fprintln(out, colorize(color, ansiCyan, "MENU"))
	fmt.Fprintf(out, "%s Open   %s Search   %s Severity   %s Category   %s Reset\n",
		menuKey(color, "[number]"), menuKey(color, "[/text]"), menuKey(color, "[s]"),
		menuKey(color, "[c]"), menuKey(color, "[r]"))
	fmt.Fprintf(out, "%s Next   %s Previous   %s Warnings   %s Help   %s Quit",
		menuKey(color, "[Enter/n]"), menuKey(color, "[p]"), menuKey(color, "[w]"),
		menuKey(color, "[h]"), menuKey(color, "[q]"))
}

func printMenuDivider(out io.Writer, color bool) {
	fmt.Fprintln(out, colorize(color, ansiDim, strings.Repeat("_", 76)))
}

func menuKey(color bool, key string) string {
	return colorize(color, ansiYellow, key)
}

func filterMenuFindings(findings []*Finding, query, category string, minLevel Severity) []*Finding {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]*Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Severity < minLevel || (category != "" && finding.Category != category) {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{
			finding.Severity.String(), finding.Category, finding.Title, finding.Unit,
			finding.Path, finding.Evidence, finding.Recommendation,
		}, " "))
		if query == "" || strings.Contains(haystack, query) {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func pageCount(items, pageSize int) int {
	if items == 0 {
		return 1
	}
	return (items + pageSize - 1) / pageSize
}

func nextSeverity(current Severity) Severity {
	if current >= SeverityCritical {
		return SeverityInfo
	}
	return current + 1
}

func browseFinding(reader *bufio.Reader, out io.Writer, findings []*Finding, index int, color bool) error {
	for {
		clearScreen(out)
		fmt.Fprintf(out, "%s  %d of %d\n", colorize(color, ansiCyan, "FINDING DETAIL"), index+1, len(findings))
		printMenuDivider(out, color)
		printFinding(out, findings[index], color)
		fmt.Fprintln(out)
		printMenuDivider(out, color)
		fmt.Fprintf(out, "%s Back   %s Next   %s Previous   %s Quit\n",
			menuKey(color, "[Enter/b]"), menuKey(color, "[n]"), menuKey(color, "[p]"), menuKey(color, "[q]"))
		fmt.Fprint(out, "Command: ")
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		switch strings.ToLower(strings.TrimSpace(choice)) {
		case "q", "quit":
			return errMenuQuit
		case "n", "next":
			index = (index + 1) % len(findings)
		case "p", "prev", "previous":
			index = (index - 1 + len(findings)) % len(findings)
		default:
			return nil
		}
	}
}

func chooseMenuCategory(reader *bufio.Reader, out io.Writer, findings []*Finding, current string, color bool) (string, error) {
	groups := groupFindings(findings)
	clearScreen(out)
	fmt.Fprintln(out, colorize(color, ansiCyan, "FILTER BY CATEGORY"))
	printMenuDivider(out, color)
	fmt.Fprintf(out, "   %s All categories\n", menuKey(color, "0)"))
	for i, group := range groups {
		marker := " "
		if group.Name == current {
			marker = colorize(color, ansiGreen, "*")
		}
		fmt.Fprintf(out, " %s %s %-18s %d findings\n", marker, menuKey(color, fmt.Sprintf("%2d)", i+1)), group.Name, len(group.Findings))
	}
	printMenuDivider(out, color)
	fmt.Fprint(out, "\nCategory number ([Enter] keeps current): ")
	choice, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return current, err
	}
	if errors.Is(err, io.EOF) {
		return current, nil
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return current, nil
	}
	index, convErr := strconv.Atoi(choice)
	if convErr != nil || index < 0 || index > len(groups) {
		return current, nil
	}
	if index == 0 {
		return "", nil
	}
	return groups[index-1].Name, nil
}

func showMenuHelp(reader *bufio.Reader, out io.Writer, color bool) error {
	clearScreen(out)
	fmt.Fprintln(out, colorize(color, ansiCyan, "BROWSER HELP"))
	printMenuDivider(out, color)
	fmt.Fprintln(out, "The list is always ordered from highest to lowest risk.")
	fmt.Fprintln(out)
	helpRows := [][2]string{
		{"number", "open the finding shown with that number"},
		{"/text", "search titles, units, paths, evidence, and recommendations"},
		{"/", "clear only the search text"},
		{"s", "cycle the minimum severity"},
		{"c", "choose one category (0 selects all)"},
		{"r", "clear every filter and return to the first page"},
		{"Enter/n", "next page (wraps after the last page)"},
		{"p", "previous page"},
		{"w", "show scanner warnings"},
		{"q", "quit"},
	}
	for _, row := range helpRows {
		fmt.Fprintf(out, "  %-20s %s\n", menuKey(color, "["+row[0]+"]"), row[1])
	}
	fmt.Fprintln(out, "\nInside a finding, n and p move directly between details.")
	printMenuDivider(out, color)
	fmt.Fprint(out, "\nPress Enter to return: ")
	_, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func showMenuWarnings(reader *bufio.Reader, out io.Writer, warnings []string, color bool) error {
	clearScreen(out)
	fmt.Fprintln(out, colorize(color, ansiCyan, "SCANNER WARNINGS"))
	printMenuDivider(out, color)
	if len(warnings) == 0 {
		fmt.Fprintln(out, "\nNo scanner warnings were reported.")
	} else {
		for i, warning := range warnings {
			fmt.Fprintf(out, "\n%d. %s\n", i+1, warning)
		}
	}
	printMenuDivider(out, color)
	fmt.Fprint(out, "\nPress Enter to return: ")
	_, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\x1b[2J\x1b[H")
}
