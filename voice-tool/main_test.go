package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSlug(t *testing.T) {
	got := slug("Hello, this is a voice test!")
	want := "hello-this-is-a-voice-test"
	if got != want {
		t.Fatalf("slug() = %q, want %q", got, want)
	}
}

func TestEnsureAudioUsesExistingFileWithoutAPIKey(t *testing.T) {
	request := speechRequest{
		Model:  defaultModel,
		Input:  "Texto ya generado",
		Voice:  defaultVoice,
		Speed:  defaultSpeed,
		Format: "mp3",
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(payload)
	fileName := slug(request.Input) + "-" + hex.EncodeToString(hash[:8]) + ".mp3"
	filePath := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(filePath, []byte("fake mp3"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENAI_API_KEY", "")
	gotPath, cached, err := ensureAudio(filepath.Dir(filePath), request)
	if err != nil {
		t.Fatal(err)
	}
	if !cached {
		t.Fatal("ensureAudio() did not report the existing file as cached")
	}
	if gotPath != filePath {
		t.Fatalf("ensureAudio() path = %q, want %q", gotPath, filePath)
	}
}

func TestResolveInputReadsExistingFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(filePath, []byte("  Read this file aloud.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	text, source, err := resolveInput("", []string{filePath})
	if err != nil {
		t.Fatal(err)
	}
	if text != "Read this file aloud." {
		t.Fatalf("resolveInput() text = %q", text)
	}
	if source != filePath {
		t.Fatalf("resolveInput() source = %q, want %q", source, filePath)
	}
}

func TestResolveInputKeepsPlainText(t *testing.T) {
	text, source, err := resolveInput("", []string{"This", "is", "plain", "text"})
	if err != nil {
		t.Fatal(err)
	}
	if text != "This is plain text" || source != "command line" {
		t.Fatalf("resolveInput() = %q, %q", text, source)
	}
}
