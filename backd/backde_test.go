package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseListCommand(t *testing.T) {
	items := []item{
		{Idx: 1, PID: 101, Name: "ssh"},
		{Idx: 2, PID: 202, Name: "nginx"},
	}

	tests := []struct {
		name       string
		input      string
		items      []item
		wantAction listCommandAction
		wantPID    int32
		wantName   string
	}{
		{
			name:       "empty enter triggers rescan",
			input:      "",
			items:      items,
			wantAction: listCommandRescan,
		},
		{
			name:       "spaces trigger rescan",
			input:      "   ",
			items:      items,
			wantAction: listCommandRescan,
		},
		{
			name:       "r triggers rescan",
			input:      "r",
			items:      items,
			wantAction: listCommandRescan,
		},
		{
			name:       "q triggers quit",
			input:      "q",
			items:      items,
			wantAction: listCommandQuit,
		},
		{
			name:       "t enters refresh edit",
			input:      "t",
			items:      items,
			wantAction: listCommandEditRefresh,
		},
		{
			name:       "question mark opens help",
			input:      "?",
			items:      items,
			wantAction: listCommandShowHelp,
		},
		{
			name:       "list index selects process pid",
			input:      "1",
			items:      items,
			wantAction: listCommandSelect,
			wantPID:    101,
		},
		{
			name:       "numeric input can target pid directly",
			input:      "333",
			items:      items,
			wantAction: listCommandSelect,
			wantPID:    333,
		},
		{
			name:       "text input targets process name",
			input:      "ssh",
			items:      items,
			wantAction: listCommandSelect,
			wantName:   "ssh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAction, gotPID, gotName := parseListCommand(tt.input, tt.items)

			if gotAction != tt.wantAction {
				t.Fatalf("action = %v, want %v", gotAction, tt.wantAction)
			}
			if gotPID != tt.wantPID {
				t.Fatalf("pid = %d, want %d", gotPID, tt.wantPID)
			}
			if gotName != tt.wantName {
				t.Fatalf("name = %q, want %q", gotName, tt.wantName)
			}
		})
	}
}

func TestClassifyExecSource(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		want string
	}{
		{name: "system binary", exe: "/usr/bin/ssh", want: "sys"},
		{name: "usr local binary", exe: "/usr/local/bin/nmap", want: "usr"},
		{name: "home binary", exe: "/Users/bellaquita/bin/evil", want: "home"},
		{name: "tmp binary", exe: "/private/tmp/dropper", want: "tmp"},
		{name: "unknown binary", exe: "/srv/custom/agent", want: "unk"},
		{name: "empty path", exe: "", want: "unk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyExecSource(tt.exe)
			if got != tt.want {
				t.Fatalf("classifyExecSource(%q) = %q, want %q", tt.exe, got, tt.want)
			}
		})
	}
}

func TestParseActivityKey(t *testing.T) {
	tests := []struct {
		name               string
		key                byte
		killConfirmPending int
		hasTarget          bool
		wantAction         activityCommandAction
		wantNext           int
		wantMessage        string
	}{
		{name: "q quits immediately", key: 'q', wantAction: activityCommandQuit},
		{name: "r goes back immediately", key: 'r', wantAction: activityCommandBack},
		{name: "enter goes back immediately", key: '\n', wantAction: activityCommandBack},
		{name: "t enters refresh edit", key: 't', wantAction: activityCommandEditRefresh},
		{name: "question mark opens help", key: '?', wantAction: activityCommandHelp},
		{name: "k without target warns", key: 'k', hasTarget: false, wantMessage: "No active process"},
		{name: "k with target arms kill", key: 'k', hasTarget: true, wantNext: 1, wantMessage: "press Enter to confirm"},
		{name: "enter confirms armed kill", key: '\n', killConfirmPending: 1, hasTarget: true, wantAction: activityCommandKill},
		{name: "other key cancels pending kill", key: 'x', killConfirmPending: 1, hasTarget: true, wantMessage: "Kill canceled."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseActivityKey(tt.key, tt.killConfirmPending, tt.hasTarget)

			if got.action != tt.wantAction {
				t.Fatalf("action = %v, want %v", got.action, tt.wantAction)
			}
			if got.nextKillConfirm != tt.wantNext {
				t.Fatalf("nextKillConfirm = %d, want %d", got.nextKillConfirm, tt.wantNext)
			}
			if tt.wantMessage != "" && !strings.Contains(got.message, tt.wantMessage) {
				t.Fatalf("message = %q, want substring %q", got.message, tt.wantMessage)
			}
		})
	}
}

func TestApplyRefreshEditKey(t *testing.T) {
	tests := []struct {
		name       string
		startBuf   string
		key        byte
		wantBuf    string
		wantDone   bool
		wantCancel bool
		wantValid  bool
		wantValue  int
	}{
		{name: "digit appends", startBuf: "1", key: '5', wantBuf: "15"},
		{name: "backspace removes", startBuf: "15", key: 127, wantBuf: "1"},
		{name: "escape cancels", startBuf: "15", key: 27, wantCancel: true},
		{name: "enter with valid number submits", startBuf: "15", key: '\n', wantDone: true, wantValid: true, wantValue: 15},
		{name: "enter with zero submits", startBuf: "0", key: '\n', wantDone: true, wantValid: true, wantValue: 0},
		{name: "enter with empty buffer invalidates", startBuf: "", key: '\n', wantDone: true},
		{name: "non digit ignored", startBuf: "1", key: 'x', wantBuf: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRefreshEditKey(tt.key, tt.startBuf)

			if got.buffer != tt.wantBuf {
				t.Fatalf("buffer = %q, want %q", got.buffer, tt.wantBuf)
			}
			if got.done != tt.wantDone {
				t.Fatalf("done = %v, want %v", got.done, tt.wantDone)
			}
			if got.cancel != tt.wantCancel {
				t.Fatalf("cancel = %v, want %v", got.cancel, tt.wantCancel)
			}
			if got.valid != tt.wantValid {
				t.Fatalf("valid = %v, want %v", got.valid, tt.wantValid)
			}
			if got.value != tt.wantValue {
				t.Fatalf("value = %d, want %d", got.value, tt.wantValue)
			}
		})
	}
}

func TestApplyListKey(t *testing.T) {
	items := []item{
		{Idx: 1, PID: 101, Name: "ssh"},
		{Idx: 2, PID: 202, Name: "nginx"},
	}

	tests := []struct {
		name       string
		startBuf   string
		key        byte
		wantBuf    string
		wantSubmit bool
		wantAction listCommandAction
		wantPID    int32
	}{
		{
			name:     "printable appends to buffer",
			startBuf: "1",
			key:      '2',
			wantBuf:  "12",
		},
		{
			name:     "backspace removes last char",
			startBuf: "12",
			key:      127,
			wantBuf:  "1",
		},
		{
			name:       "enter submits selection",
			startBuf:   "1",
			key:        '\n',
			wantSubmit: true,
			wantAction: listCommandSelect,
			wantPID:    101,
		},
		{
			name:       "enter with empty buffer rescans",
			startBuf:   "",
			key:        '\n',
			wantSubmit: true,
			wantAction: listCommandRescan,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyListKey(tt.key, tt.startBuf, items)

			if got.buffer != tt.wantBuf {
				t.Fatalf("buffer = %q, want %q", got.buffer, tt.wantBuf)
			}
			if got.submit != tt.wantSubmit {
				t.Fatalf("submit = %v, want %v", got.submit, tt.wantSubmit)
			}
			if got.action != tt.wantAction {
				t.Fatalf("action = %v, want %v", got.action, tt.wantAction)
			}
			if got.pid != tt.wantPID {
				t.Fatalf("pid = %d, want %d", got.pid, tt.wantPID)
			}
		})
	}
}

func TestFormatProcessStatus(t *testing.T) {
	tests := []struct {
		name   string
		status []string
		want   string
	}{
		{name: "single status", status: []string{"sleep"}, want: "sleep"},
		{name: "multiple states", status: []string{"sleep", "stop"}, want: "sleep, stop"},
		{name: "empty status", status: nil, want: "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProcessStatus(tt.status)
			if got != tt.want {
				t.Fatalf("formatProcessStatus(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestWrapTextSplitsLongValues(t *testing.T) {
	got := wrapText("uno dos tres cuatro cinco", 10)
	want := []string{"uno dos", "tres", "cuatro", "cinco"}

	if len(got) != len(want) {
		t.Fatalf("wrapText returned %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrapText line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderActivitySnapshot(t *testing.T) {
	snapshot := activitySnapshot{
		Name:           "dnsmasq",
		PID:            938,
		User:           "nobody",
		Status:         "sleep",
		ParentPID:      1,
		ParentName:     "systemd",
		Uptime:         81*time.Hour + 11*time.Minute + 51*time.Second,
		Nice:           20,
		Exec:           "/usr/bin/dnsmasq",
		Cmdline:        "/usr/bin/dnsmasq --conf-file=/var/lib/libvirt/dnsmasq/default.conf --leasefile-ro --dhcp-script=/usr/lib/libvirt/libvirt_leaseshelper",
		CPU:            0,
		MemPercent:     0,
		RSS:            "2.8 MiB",
		Threads:        1,
		FDs:            12,
		CtxSwitchVol:   431,
		CtxSwitchInvol: 3,
		HasIO:          true,
		HasIORate:      true,
		IOReadRate:     "0 B",
		IOWriteRate:    "0 B",
		IOReadTotal:    "0 B",
		IOWriteTotal:   "0 B",
		TCPEstablished: 0,
		UDPCount:       3,
		Remotes:        []string{"udp *:0", "udp *:0", "udp *:0"},
	}

	got := renderActivitySnapshot(snapshot)

	for _, want := range []string{
		"Process",
		"Name:",
		"dnsmasq",
		"State:",
		"sleep",
		"Execution",
		"Command:",
		"Resources",
		"CtxSwitch:",
		"IO",
		"Read/s:",
		"Network",
		"TCP Established:",
		"Remote Peers:",
		"- udp *:0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderActivitySnapshot() missing %q in output:\n%s", want, got)
		}
	}
}
