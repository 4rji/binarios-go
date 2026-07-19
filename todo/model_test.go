package main

import (
	"reflect"
	"testing"
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
	got := wrapTextLines("  https://example.com/very-long-path", 14)
	want := []string{
		"  https://ex",
		"  ample.com/v",
		"  ery-long-pa",
		"  th",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrapTextLines() = %#v, want %#v", got, want)
	}
}
