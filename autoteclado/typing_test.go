package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTypeTextUsesKeyEventsForAllPrintableASCII(t *testing.T) {
	var text strings.Builder
	var want []string
	for r := rune(' '); r <= '~'; r++ {
		text.WriteRune(r)
		key := string(r)
		if r == ' ' {
			key = "space"
		}
		want = append(want, key)
	}

	var got []string
	keyboard := keyboardWriter{
		keyTap: func(key string, _ ...interface{}) error {
			got = append(got, key)
			return nil
		},
		typeUnicode: func(text string, _ ...int) {
			t.Errorf("ASCII %q used Unicode injection; web consoles would receive key A", text)
		},
	}
	if err := keyboard.typeText(text.String(), nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("key events = %q, want %q", got, want)
	}
}

func TestTypeTextPacesOnlySuccessfullySentCharacters(t *testing.T) {
	failure := errors.New("failed to press enter")
	var sent, paced int
	keyboard := keyboardWriter{
		keyTap: func(key string, _ ...interface{}) error {
			if key == "enter" {
				return failure
			}
			sent++
			return nil
		},
		typeUnicode: func(string, ...int) { sent++ },
	}
	err := keyboard.typeText("a\r\t ñ\nmore", func(time.Duration) {
		paced++
		if paced != sent {
			t.Errorf("paced %d characters after sending %d", paced, sent)
		}
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if sent != 4 || paced != 4 {
		t.Errorf("sent = %d, paced = %d, want four successful characters", sent, paced)
	}
}

func TestTypeTextPreservesTextAndLineEndings(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"shell command", "echo \"Ab_19!\"\r\n\t./x -v\n", []string{
			"key:e", "key:c", "key:h", "key:o", "key:space", "key:\"",
			"key:A", "key:b", "key:_", "key:1", "key:9", "key:!", "key:\"",
			"key:enter", "key:tab", "key:.", "key:/", "key:x", "key:space",
			"key:-", "key:v", "key:enter",
		}},
		{"unicode fallback", "año café", []string{
			"key:a", "text:ñ", "key:o", "key:space", "key:c", "key:a", "key:f", "text:é",
		}},
		{"carriage returns", "\r\r\n", []string{"key:enter"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			keyboard := keyboardWriter{
				keyTap: func(key string, _ ...interface{}) error {
					got = append(got, "key:"+key)
					return nil
				},
				typeUnicode: func(text string, _ ...int) {
					got = append(got, "text:"+text)
				},
			}
			if err := keyboard.typeText(tt.text, nil); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("events = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeTextStopsBeforeSendingMoreKeysAfterError(t *testing.T) {
	for _, failedKey := range []string{"x", "enter", "tab", "space"} {
		t.Run(failedKey, func(t *testing.T) {
			failure := errors.New("keyboard unavailable")
			var got []string
			keyboard := keyboardWriter{
				keyTap: func(key string, _ ...interface{}) error {
					got = append(got, key)
					if key == failedKey {
						return failure
					}
					return nil
				},
				typeUnicode: func(string, ...int) {
					t.Fatal("sent Unicode text after a keyboard failure")
				},
			}
			err := keyboard.typeText("x\n\t ñ\n", nil)
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
			allKeys := []string{"x", "enter", "tab", "space"}
			for i, key := range allKeys {
				if key == failedKey && !reflect.DeepEqual(got, allKeys[:i+1]) {
					t.Errorf("key events = %q, want %q", got, allKeys[:i+1])
				}
			}
		})
	}
}
