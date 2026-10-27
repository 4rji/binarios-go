package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

const version = "0.1.0"

type options struct {
	view      string
	root      string
	category  string
	minLevel  string
	limit     int
	noColor   bool
	noSystemd bool
	failOn    string
	showHelp  bool
	showVer   bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "spaudit:", err)
		fmt.Fprintln(os.Stderr, "Try 'spaudit --help' for usage.")
		return 2
	}
	if opts.showHelp {
		printUsage(os.Stdout)
		return 0
	}
	if opts.showVer {
		fmt.Printf("spaudit %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}

	minSeverity, err := parseSeverity(opts.minLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "spaudit:", err)
		return 2
	}

	result := scan(scanOptions{Root: opts.root, UseSystemctl: !opts.noSystemd})
	findings := filterFindings(result.Findings, opts.category, minSeverity)
	sortFindings(findings)
	assignIDs(findings)

	color := !opts.noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
	view := opts.view
	if view == "auto" {
		if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			if _, err := exec.LookPath("fzf"); err == nil {
				view = "fzf"
			} else {
				view = "menu"
			}
		} else {
			view = "summary"
		}
	}

	switch view {
	case "summary":
		printSummary(os.Stdout, result, findings, opts.limit, color)
	case "full":
		printFull(os.Stdout, result, findings, color)
	case "json":
		if err := printJSON(os.Stdout, result, findings); err != nil {
			fmt.Fprintln(os.Stderr, "spaudit: could not encode JSON:", err)
			return 1
		}
	case "fzf":
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			fmt.Fprintln(os.Stderr, "spaudit: fzf view requires an interactive terminal")
			return 2
		}
		if err := runFZF(findings, color); err != nil {
			if errors.Is(err, errFZFUnavailable) {
				fmt.Fprintln(os.Stderr, "spaudit: fzf is not installed; using the built-in menu")
				if err := runMenu(result, findings, color); err != nil {
					fmt.Fprintln(os.Stderr, "spaudit:", err)
					return 1
				}
			} else if !errors.Is(err, errFZFCancelled) {
				fmt.Fprintln(os.Stderr, "spaudit:", err)
				return 1
			}
		}
	case "menu":
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			fmt.Fprintln(os.Stderr, "spaudit: menu view requires an interactive terminal")
			return 2
		}
		if err := runMenu(result, findings, color); err != nil {
			fmt.Fprintln(os.Stderr, "spaudit:", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "spaudit: unknown view %q\n", view)
		return 2
	}

	if opts.failOn != "" {
		threshold, err := parseSeverity(opts.failOn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "spaudit:", err)
			return 2
		}
		for _, finding := range findings {
			if finding.Severity >= threshold {
				return 3
			}
		}
	}
	return 0
}

func parseFlags(args []string) (options, error) {
	var opts options
	fs := flag.NewFlagSet("spaudit", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	fs.StringVar(&opts.view, "view", "auto", "output view: auto, summary, menu, fzf, full, or json")
	fs.StringVar(&opts.root, "root", "/", "offline root filesystem to inspect")
	fs.StringVar(&opts.category, "category", "", "only show categories containing this text")
	fs.StringVar(&opts.minLevel, "severity", "info", "minimum severity: info, review, suspicious, or critical")
	fs.IntVar(&opts.limit, "limit", 12, "maximum findings in summary view (0 means none)")
	fs.BoolVar(&opts.noColor, "no-color", false, "disable ANSI colors")
	fs.BoolVar(&opts.noSystemd, "no-systemctl", false, "inspect files only; do not query the running system")
	fs.StringVar(&opts.failOn, "fail-on", "", "exit 3 when this severity or higher is found")
	fs.BoolVar(&opts.showHelp, "help", false, "show help")
	fs.BoolVar(&opts.showHelp, "h", false, "show help")
	fs.BoolVar(&opts.showVer, "version", false, "show version")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	validViews := map[string]bool{"auto": true, "summary": true, "menu": true, "fzf": true, "full": true, "json": true}
	if !validViews[opts.view] {
		return opts, fmt.Errorf("invalid --view %q", opts.view)
	}
	if opts.limit < 0 {
		return opts, fmt.Errorf("--limit cannot be negative")
	}
	absRoot, err := normalizeRoot(opts.root)
	if err != nil {
		return opts, err
	}
	opts.root = absRoot
	return opts, nil
}

func filterFindings(findings []Finding, category string, min Severity) []*Finding {
	needle := strings.ToLower(strings.TrimSpace(category))
	filtered := make([]*Finding, 0, len(findings))
	for i := range findings {
		f := &findings[i]
		if f.Severity < min {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(f.Category), needle) {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

func sortFindings(findings []*Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		if findings[i].Unit != findings[j].Unit {
			return findings[i].Unit < findings[j].Unit
		}
		return findings[i].Title < findings[j].Title
	})
}

func assignIDs(findings []*Finding) {
	for i, finding := range findings {
		finding.ID = fmt.Sprintf("%06d", i+1)
	}
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, `spaudit - read-only systemd persistence triage

Usage:
  spaudit [options]

Views:
  auto       Use fzf on an interactive terminal, the built-in menu otherwise,
             and a compact summary when output is redirected (default).
  summary    Counts, category rollup, and the highest-risk findings.
  menu       Dependency-free category browser.
  fzf        Searchable list with detail preview and multi-select.
  full       Every finding, grouped by category and risk-sorted.
  json       Machine-readable scan result.

Options:
  --view MODE          auto|summary|menu|fzf|full|json
  --category TEXT      restrict results to matching categories
  --severity LEVEL     info|review|suspicious|critical (default: info)
  --limit N            findings shown in summary view (default: 12)
  --root PATH          inspect an offline Linux root filesystem
  --no-systemctl       inspect files only
  --no-color           disable ANSI colors (NO_COLOR is also honored)
  --fail-on LEVEL      exit 3 if LEVEL or higher is present
  --version            print version
  -h, --help           show this help

Examples:
  sudo spaudit
  sudo spaudit --view fzf
  spaudit --view full --severity suspicious
  spaudit --view json --no-systemctl > findings.json

The scanner never modifies units or starts/stops services. Findings are triage
signals, not proof of compromise.`)
}
