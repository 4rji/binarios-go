package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultModel        = "gpt-4o-mini-tts"
	defaultVoice        = "cedar"
	defaultSpeed        = 0.98
	defaultInstructions = "Speak like a friendly technical co-host. Keep the tone clear, natural, and conversational. Use a steady pace, avoid sounding scripted, and add subtle emphasis to important ideas. Sound helpful, confident, and easy to listen to."
	longInputTokens     = 1000
	modelInputTokens    = 2000
)

var (
	colorBlue   = "\033[1;34m"
	colorCyan   = "\033[1;36m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"
)

type config struct {
	model        string
	voice        string
	instructions string
	speed        float64
	outputDir    string
	play         bool
}

type speechRequest struct {
	Model        string  `json:"model"`
	Input        string  `json:"input"`
	Voice        string  `json:"voice"`
	Instructions string  `json:"instructions,omitempty"`
	Speed        float64 `json:"speed"`
	Format       string  `json:"response_format"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	flags := flag.NewFlagSet("script", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	cfg := config{}
	flags.StringVar(&cfg.model, "model", envOr("OPENAI_TTS_MODEL", defaultModel), "TTS model")
	flags.StringVar(&cfg.voice, "voice", envOr("OPENAI_TTS_VOICE", defaultVoice), "TTS voice")
	flags.StringVar(&cfg.instructions, "instructions", envOr("OPENAI_TTS_INSTRUCTIONS", defaultInstructions), "speaking style instructions")
	flags.Float64Var(&cfg.speed, "speed", envFloat("OPENAI_TTS_SPEED", defaultSpeed), "speech speed")
	flags.StringVar(&cfg.outputDir, "output-dir", filepath.Join(workingDir, "voces"), "output directory")
	inputFile := flags.String("file", "", "read text from a file (use - for stdin)")
	noPlay := flags.Bool("no-play", false, "create the MP3 without playing it")
	flags.Usage = func() {
		printUsage(flags)
	}

	if len(args) == 0 {
		flags.Usage()
		return nil
	}
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	cfg.play = !*noPlay
	text, source, err := resolveInput(*inputFile, flags.Args())
	if err != nil {
		return err
	}
	if text == "" {
		flags.Usage()
		return errors.New("no text was provided")
	}
	if cfg.speed <= 0 {
		return errors.New("speed must be greater than zero")
	}

	printInputSummary(cfg.model, source, text)

	request := speechRequest{
		Model:        cfg.model,
		Input:        text,
		Voice:        cfg.voice,
		Instructions: cfg.instructions,
		Speed:        cfg.speed,
		Format:       "mp3",
	}
	if cfg.model == "tts-1" || cfg.model == "tts-1-hd" {
		request.Instructions = ""
	}

	filePath, cached, err := ensureAudio(cfg.outputDir, request)
	if err != nil {
		return err
	}
	if cached {
		fmt.Printf("%s✓ Cached audio:%s %s\n", colorGreen, colorReset, filePath)
	} else {
		fmt.Printf("%s✓ Audio created:%s %s\n", colorGreen, colorReset, filePath)
	}

	if cfg.play {
		fmt.Printf("%s▶ Playing AI-generated voice...%s\n", colorCyan, colorReset)
		if err := playAudio(filePath); err != nil {
			return fmt.Errorf("the MP3 was saved, but playback failed: %w", err)
		}
	}
	return nil
}

func ensureAudio(outputDir string, request speechRequest) (string, bool, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", false, fmt.Errorf("prepare request: %w", err)
	}

	hash := sha256.Sum256(payload)
	fileName := fmt.Sprintf("%s-%s.mp3", slug(request.Input), hex.EncodeToString(hash[:8]))
	filePath := filepath.Join(outputDir, fileName)
	if info, err := os.Stat(filePath); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return filePath, true, nil
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return "", false, errors.New("set OPENAI_API_KEY to create new audio")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create %s: %w", outputDir, err)
	}

	httpRequest, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return "", false, fmt.Errorf("create HTTP request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(httpRequest)
	if err != nil {
		return "", false, fmt.Errorf("call OpenAI: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return "", false, fmt.Errorf("OpenAI returned %s: %s", response.Status, strings.TrimSpace(string(details)))
	}

	temporary, err := os.CreateTemp(outputDir, ".voz-*.mp3")
	if err != nil {
		return "", false, fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := io.Copy(temporary, response.Body); err != nil {
		temporary.Close()
		return "", false, fmt.Errorf("save audio: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", false, fmt.Errorf("close audio file: %w", err)
	}
	if err := os.Rename(temporaryPath, filePath); err != nil {
		return "", false, fmt.Errorf("finish audio file: %w", err)
	}
	return filePath, false, nil
}

func playAudio(filePath string) error {
	type player struct {
		name string
		args []string
	}

	var players []player
	switch runtime.GOOS {
	case "darwin":
		players = []player{{name: "afplay", args: []string{filePath}}}
	case "windows":
		players = []player{{name: "powershell", args: []string{"-NoProfile", "-Command", "Start-Process", "-Wait", filePath}}}
	default:
		players = []player{
			{name: "mpv", args: []string{"--no-terminal", filePath}},
			{name: "ffplay", args: []string{"-nodisp", "-autoexit", "-loglevel", "error", filePath}},
			{name: "xdg-open", args: []string{filePath}},
		}
	}

	for _, candidate := range players {
		if _, err := exec.LookPath(candidate.name); err != nil {
			continue
		}
		command := exec.Command(candidate.name, candidate.args...)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}
	return errors.New("no compatible audio player was found; use --no-play to skip playback")
}

func resolveInput(inputFile string, args []string) (string, string, error) {
	if inputFile != "" {
		if len(args) > 0 {
			return "", "", errors.New("use either --file or a text/file argument, not both")
		}
		return readInputFile(inputFile)
	}

	if len(args) == 1 {
		if info, err := os.Stat(args[0]); err == nil && info.Mode().IsRegular() {
			return readInputFile(args[0])
		}
	}
	return strings.TrimSpace(strings.Join(args, " ")), "command line", nil
}

func readInputFile(path string) (string, string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", "", fmt.Errorf("read input file %q: %w", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", "", fmt.Errorf("input file %q is empty", path)
	}
	if path == "-" {
		return text, "standard input", nil
	}
	return text, path, nil
}

func printUsage(flags *flag.FlagSet) {
	out := flags.Output()
	fmt.Fprintf(out, "\n%sVOICE GENERATOR%s  %sOpenAI text-to-speech CLI%s\n", colorBlue, colorReset, colorDim, colorReset)
	fmt.Fprintf(out, "%s──────────────────────────────────────────────%s\n\n", colorBlue, colorReset)
	fmt.Fprintf(out, "%sUSAGE%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  go run . \"Text to turn into speech\"")
	fmt.Fprintln(out, "  go run . article.txt")
	fmt.Fprintln(out, "  go run . --file article.txt")
	fmt.Fprintln(out, "  ./script \"Text to turn into speech\"")
	fmt.Fprintf(out, "\n%sWHAT IT DOES%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  1. Reads text from an argument, file, or stdin.")
	fmt.Fprintln(out, "  2. Creates an MP3 with OpenAI TTS.")
	fmt.Fprintln(out, "  3. Saves it in ./voces and plays it.")
	fmt.Fprintf(out, "\n%sOPTIONS%s\n", colorCyan, colorReset)
	flags.PrintDefaults()
	fmt.Fprintf(out, "\n%sSETUP%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  export OPENAI_API_KEY=\"sk-...\"")
	fmt.Fprintf(out, "\n%sTIP%s  Use --no-play on servers without audio.\n\n", colorYellow, colorReset)
}

func printInputSummary(model, source, text string) {
	estimatedTokens := (len([]rune(text)) + 3) / 4
	fmt.Printf("%sInput:%s %s · %d characters · ~%d tokens\n", colorCyan, colorReset, source, len([]rune(text)), estimatedTokens)
	if estimatedTokens >= longInputTokens {
		fmt.Printf("%s⚠ Long input:%s this request may use many billable input tokens and cost more.\n", colorYellow, colorReset)
	}
	if model == defaultModel && estimatedTokens >= modelInputTokens {
		fmt.Printf("%s⚠ Model limit:%s gpt-4o-mini-tts accepts at most 2,000 input tokens; shorten or split the file if the request fails.\n", colorYellow, colorReset)
	}
}

func slug(text string) string {
	var result strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(text) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
		if result.Len() >= 40 {
			break
		}
	}
	value := strings.Trim(result.String(), "-")
	if value == "" {
		return "voz"
	}
	return value
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envFloat(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
