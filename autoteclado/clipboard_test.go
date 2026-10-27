package main

import (
	"errors"
	"testing"
)

func TestReadClipboardTextPreservesContent(t *testing.T) {
	for _, text := range []string{"echo hola", "  año café\r\n\tsegunda línea\n", " \t\n"} {
		t.Run(text, func(t *testing.T) {
			calls := 0
			got, err := readClipboardText("darwin", func() ([]byte, error) {
				calls++
				return []byte(text), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != text || calls != 1 {
				t.Errorf("clipboard = %q (%d reads), want %q (one read)", got, calls, text)
			}
		})
	}
}

func TestReadClipboardTextRejectsEmptyAndReadErrors(t *testing.T) {
	failure := errors.New("pbpaste failed")
	for _, readErr := range []error{nil, failure} {
		_, err := readClipboardText("darwin", func() ([]byte, error) { return nil, readErr })
		if err == nil {
			t.Fatal("expected an error before typing an empty or unreadable clipboard")
		}
		if readErr != nil && !errors.Is(err, readErr) {
			t.Errorf("error = %v, want wrapped %v", err, readErr)
		}
	}
}

func TestReadClipboardTextRejectsUnsupportedOS(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			_, err := readClipboardText(goos, func() ([]byte, error) {
				t.Fatal("must not run pbpaste on an unsupported OS")
				return nil, nil
			})
			if err == nil {
				t.Fatal("expected an unsupported OS error")
			}
		})
	}
}
