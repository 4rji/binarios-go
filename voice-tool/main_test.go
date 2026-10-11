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
	gotPath, cached, err := ensureAudio(filepath.Dir(filePath), request, "")
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

func TestReadVoicesEnv(t *testing.T) {
	tests := []struct {
		name, content, want string
		wantErr             bool
	}{
		{"plain", "OPENAI_TTS_VOICE=cedar\n", "cedar", false},
		{"quoted and comments", "# settings\n\nexport OPENAI_TTS_VOICE = 'another voice'\n", "another voice", false},
		{"literal shell text", "OPENAI_TTS_VOICE=\"$(echo secret)\"\n", "$(echo secret)", false},
		{"missing assignment", "OPENAI_TTS_VOICE\n", "", true},
		{"unclosed quote", "OPENAI_TTS_VOICE=\"cedar\n", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".voices_env")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			settings, err := readVoicesEnv(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && settings["OPENAI_TTS_VOICE"] != tt.want {
				t.Fatalf("voice = %q, want %q", settings["OPENAI_TTS_VOICE"], tt.want)
			}
		})
	}
	settings, err := readVoicesEnv(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(settings) != 0 {
		t.Fatalf("missing file: %v, %v", settings, err)
	}
}

func TestRunVoicesEnvPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"OPENAI_TTS_MODEL", "OPENAI_TTS_VOICE", "OPENAI_TTS_SPEED", "OPENAI_TTS_INSTRUCTIONS"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OPENAI_API_KEY", "")
	if err := os.WriteFile(filepath.Join(home, ".voices_env"), []byte("OPENAI_TTS_MODEL=file-model\nOPENAI_TTS_VOICE=file-voice\nOPENAI_TTS_SPEED=0.8\nOPENAI_TTS_INSTRUCTIONS='Habla tranquilo.'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"file", "environment", "flags"} {
		t.Run(mode, func(t *testing.T) {
			request := speechRequest{Model: "file-model", Voice: "file-voice", Speed: 0.8, Instructions: "Habla tranquilo.", Input: "Test", Format: "mp3"}
			output := t.TempDir()
			args := []string{"--no-play", "--output-dir", output}
			if mode != "file" {
				t.Setenv("OPENAI_TTS_MODEL", "env-model")
				t.Setenv("OPENAI_TTS_VOICE", "env-voice")
				t.Setenv("OPENAI_TTS_SPEED", "1.1")
				request.Model, request.Voice, request.Speed = "env-model", "env-voice", 1.1
			}
			if mode == "flags" {
				args = append(args, "--model", "flag-model", "--voice", "flag-voice", "--speed", "1.2")
				request.Model, request.Voice, request.Speed = "flag-model", "flag-voice", 1.2
			}
			payload, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(payload)
			path := filepath.Join(output, slug(request.Input)+"-"+hex.EncodeToString(hash[:8])+".mp3")
			if err := os.WriteFile(path, []byte("cached audio"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := run(append(args, "Test")); err != nil {
				t.Fatal(err)
			}
		})
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
