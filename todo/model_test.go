package main

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWrapTextLinesPreservesBlankAndIndentedLines(t *testing.T) {
	got := wrapTextLines("Uso:\n  node audiodown.js <URL> [--wait 10000]\n\nArgumentos:", 28)
	want := []string{
		"Uso:",
		"  node audiodown.js <URL>",
		"  [--wait 10000]",
		"",
		"Argumentos:",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrapTextLines() = %#v, want %#v", got, want)
	}
}

func TestWrapTextLinesSplitsLongWords(t *testing.T) {
	// A word longer than the wrap width is split into chunks, each padded to
	// the 2-space indent and packed up to (but not over) the target width.
	got := wrapTextLines("  https://example.com/very-long-path", 14)
	want := []string{
		"  https://exam",
		"  ple.com/very",
		"  -long-path",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrapTextLines() = %#v, want %#v", got, want)
	}
}

func TestVStartsSearchFromBrowseMode(t *testing.T) {
	m := initialModel()
	m.mode = modeBrowse

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(Model)

	if got.mode != modeSearch {
		t.Fatalf("mode = %v, want %v", got.mode, modeSearch)
	}
	if got.input.Value() != "v" {
		t.Fatalf("input value = %q, want %q", got.input.Value(), "v")
	}
}
