package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

//go:embed profiles.json
var profilesData []byte

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

type profileVariant struct {
	Label        string  `json:"label"`
	Model        string  `json:"model"`
	Voice        string  `json:"voice"`
	Speed        float64 `json:"speed"`
	Instructions string  `json:"instructions"`
}

type profile struct {
	Description string           `json:"description"`
	Variants    []profileVariant `json:"variants"`
}

// loadProfiles parses the embedded profiles.json.
func loadProfiles() (map[string]profile, error) {
	var profiles map[string]profile
	if err := json.Unmarshal(profilesData, &profiles); err != nil {
		return nil, fmt.Errorf("parse embedded profiles.json: %w", err)
	}
	return profiles, nil
}

func profileNames(profiles map[string]profile) []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolveProfile parses "name" or "name:N" and returns the selected variant.
// N is 1-based; when omitted the first variant is used.
func resolveProfile(profiles map[string]profile, selector string) (profileVariant, error) {
	name, number, hasNumber := strings.Cut(selector, ":")
	name = strings.TrimSpace(name)
	prof, ok := profiles[name]
	if !ok {
		return profileVariant{}, fmt.Errorf("unknown profile %q (available: %s)", name, strings.Join(profileNames(profiles), ", "))
	}
	if len(prof.Variants) == 0 {
		return profileVariant{}, fmt.Errorf("profile %q has no variants", name)
	}
	index := 0
	if hasNumber {
		n, err := strconv.Atoi(strings.TrimSpace(number))
		if err != nil {
			return profileVariant{}, fmt.Errorf("profile variant must be a number: %q", number)
		}
		if n < 1 || n > len(prof.Variants) {
			return profileVariant{}, fmt.Errorf("profile %q has variants 1-%d, got %d", name, len(prof.Variants), n)
		}
		index = n - 1
	}
	return prof.Variants[index], nil
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
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	settings, err := readVoicesEnv(filepath.Join(home, ".voices_env"))
	if err != nil {
		return err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	flags := flag.NewFlagSet("voces", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	cfg := config{}
	instructionsDefault := settings.value("OPENAI_TTS_INSTRUCTIONS", defaultInstructions)
	var profileSel string
	flags.StringVar(&cfg.model, "model", settings.value("OPENAI_TTS_MODEL", defaultModel), "TTS model")
	flags.StringVar(&cfg.voice, "voice", settings.value("OPENAI_TTS_VOICE", defaultVoice), "TTS voice")
	flags.StringVar(&cfg.instructions, "instructions", instructionsDefault, "speaking style instructions")
	flags.StringVar(&cfg.instructions, "in", instructionsDefault, "alias for -instructions")
	flags.StringVar(&profileSel, "profile", "", "voice profile; name or name:N (e.g. femeninas, tecnico:2, list)")
	flags.StringVar(&profileSel, "p", "", "alias for -profile")
	flags.Float64Var(&cfg.speed, "speed", defaultSpeed, "speech speed")
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

	visited := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	instructionsSet := visited["instructions"] || visited["in"]

	if !visited["speed"] {
		cfg.speed, err = strconv.ParseFloat(settings.value("OPENAI_TTS_SPEED", "0.98"), 64)
		if err != nil {
			return errors.New("OPENAI_TTS_SPEED must be a number")
		}
	}

	if profileSel != "" {
		profiles, err := loadProfiles()
		if err != nil {
			return err
		}
		if profileSel == "list" || profileSel == "help" {
			printProfiles(profiles)
			return nil
		}
		variant, err := resolveProfile(profiles, profileSel)
		if err != nil {
			return err
		}
		// Profile overrides env/.voices_env/defaults, but explicit flags still win.
		if !visited["model"] && variant.Model != "" {
			cfg.model = variant.Model
		}
		if !visited["voice"] {
			cfg.voice = variant.Voice
		}
		if !visited["speed"] {
			cfg.speed = variant.Speed
		}
		if !instructionsSet {
			cfg.instructions = variant.Instructions
		}
		name, _, _ := strings.Cut(profileSel, ":")
		fmt.Printf("%sProfile:%s %s · %s · voice %s · speed %g\n", colorCyan, colorReset, name, variant.Label, cfg.voice, cfg.speed)
	}
	text, source, err := resolveInput(*inputFile, flags.Args())
	if err != nil {
		return err
	}
	if text == "" {
		flags.Usage()
		return errors.New("no text was provided")
	}
	if cfg.speed <= 0 || math.IsNaN(cfg.speed) || math.IsInf(cfg.speed, 0) {
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

	filePath, cached, err := ensureAudio(cfg.outputDir, request, settings.value("OPENAI_API_KEY", ""))
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

func ensureAudio(outputDir string, request speechRequest, apiKey string) (string, bool, error) {
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

	apiKey = strings.TrimSpace(apiKey)
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
	fmt.Fprintln(out, "  voces \"Text to turn into speech\"")
	fmt.Fprintln(out, "  voces article.txt")
	fmt.Fprintln(out, "  voces --file article.txt")
	fmt.Fprintln(out, "  voces -p femeninas \"Text to turn into speech\"")
	fmt.Fprintln(out, "  voces -p femeninas:5 \"Text to turn into speech\"")
	fmt.Fprintln(out, "  voces -p tecnico:2 article.txt")
	fmt.Fprintln(out, "  voces -p list")
	fmt.Fprintln(out, "  voces -in \"Habla en español, con tono cálido y pausado.\" \"Hola, esta es una prueba de voz.\"")
	fmt.Fprintf(out, "\n%sWHAT IT DOES%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  1. Reads text from an argument, file, or stdin.")
	fmt.Fprintln(out, "  2. Creates an MP3 with OpenAI TTS.")
	fmt.Fprintln(out, "  3. Saves it in ./voces and plays it.")
	fmt.Fprintf(out, "\n%sOPTIONS%s\n", colorCyan, colorReset)
	flags.PrintDefaults()
	fmt.Fprintf(out, "\n%sPROFILES%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  -p / -profile selects a saved voice preset (model, voice, speed, instructions).")
	fmt.Fprintln(out, "  Use name or name:N to pick a variant; run 'voces -p list' to see them all.")
	fmt.Fprintln(out, "  Explicit -voice/-speed/-in override the profile.")
	fmt.Fprintf(out, "\n%sSETUP%s\n", colorCyan, colorReset)
	fmt.Fprintln(out, "  export OPENAI_API_KEY=\"sk-...\"")
	fmt.Fprintln(out, "  Settings are read automatically from ~/.voices_env.")
	fmt.Fprintln(out, "  Priority: command options > environment > ~/.voices_env > defaults.")
	fmt.Fprintf(out, "\n%sTIP%s  Use --no-play on servers without audio.\n\n", colorYellow, colorReset)
}

func printProfiles(profiles map[string]profile) {
	out := os.Stdout
	fmt.Fprintf(out, "\n%sVOICE PROFILES%s\n", colorBlue, colorReset)
	for _, name := range profileNames(profiles) {
		prof := profiles[name]
		fmt.Fprintf(out, "\n%s%s%s  %s%s%s\n", colorCyan, name, colorReset, colorDim, prof.Description, colorReset)
		for i, v := range prof.Variants {
			fmt.Fprintf(out, "  %s%d%s %s (voice %s, speed %g)\n", colorDim, i+1, colorReset, v.Label, v.Voice, v.Speed)
		}
	}
	fmt.Fprintf(out, "\n%sUSE%s  voces -p <name> \"text\"   or   voces -p <name>:<n> \"text\"\n\n", colorYellow, colorReset)
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

type voiceSettings map[string]string

func (s voiceSettings) value(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return strings.TrimSpace(value)
	}
	if value, ok := s[name]; ok {
		return value
	}
	return fallback
}

// readVoicesEnv reads literal values, without executing shell commands or expanding variables.
func readVoicesEnv(path string) (voiceSettings, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return voiceSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()
	settings := voiceSettings{}
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")
		name, value, ok := strings.Cut(text, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" {
			return nil, fmt.Errorf("%s:%d: expected NAME=value", path, line)
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			if len(value) < 2 || value[len(value)-1] != value[0] {
				return nil, fmt.Errorf("%s:%d: unclosed quote", path, line)
			}
			value = value[1 : len(value)-1]
		}
		switch name {
		case "OPENAI_TTS_MODEL", "OPENAI_TTS_VOICE", "OPENAI_TTS_SPEED", "OPENAI_TTS_INSTRUCTIONS", "OPENAI_API_KEY":
			settings[name] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return settings, nil
}
