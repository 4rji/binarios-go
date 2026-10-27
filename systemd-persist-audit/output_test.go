package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestFilterMenuFindingsSearchesDetailsAndCombinesFilters(t *testing.T) {
	items := []Finding{
		newFinding("Execution", SeveritySuspicious, 90, "Shell command", "alpha.service", "/etc/systemd/alpha.service", 3, "ExecStart=/bin/sh", "Review it"),
		newFinding("Timers", SeverityReview, 42, "Scheduled execution", "backup.timer", "/etc/systemd/backup.timer", 0, "OnCalendar=daily", "Review schedule"),
		newFinding("Permissions", SeverityCritical, 100, "World-writable artifact", "odd.service", "/etc/systemd/odd.service", 0, "mode=0666", "Fix permissions"),
	}
	pointers := filterFindings(items, "", SeverityInfo)

	got := filterMenuFindings(pointers, "oncalendar", "Timers", SeverityReview)
	if len(got) != 1 || got[0].Unit != "backup.timer" {
		t.Fatalf("filter result = %#v, want backup.timer", got)
	}

	got = filterMenuFindings(pointers, "service", "", SeverityCritical)
	if len(got) != 1 || got[0].Unit != "odd.service" {
		t.Fatalf("critical search result = %#v, want odd.service", got)
	}
}

func TestRunMenuSearchAndOpenFinding(t *testing.T) {
	items := []Finding{
		newFinding("Execution", SeveritySuspicious, 90, "Shell command", "alpha.service", "/etc/systemd/alpha.service", 3, "ExecStart=/bin/sh", "Review it"),
		newFinding("Timers", SeverityReview, 42, "Scheduled execution", "backup.timer", "/etc/systemd/backup.timer", 0, "OnCalendar=daily", "Review schedule"),
	}
	pointers := filterFindings(items, "", SeverityInfo)
	reader := bufio.NewReader(strings.NewReader("/oncalendar\n1\n\nq\n"))
	var out bytes.Buffer

	if err := runMenuIO(reader, &out, ScanResult{FilesScanned: 2, Duration: "1ms"}, pointers, false); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"search: \"oncalendar\"", "backup.timer", "FINDING DETAIL", "OnCalendar=daily"} {
		if !strings.Contains(output, want) {
			t.Errorf("menu output does not contain %q", want)
		}
	}
}

func TestPageCountAlwaysHasOnePage(t *testing.T) {
	for _, tc := range []struct {
		items int
		want  int
	}{{0, 1}, {1, 1}, {10, 1}, {11, 2}, {20, 2}} {
		if got := pageCount(tc.items, 10); got != tc.want {
			t.Errorf("pageCount(%d, 10) = %d, want %d", tc.items, got, tc.want)
		}
	}
}
