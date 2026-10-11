# Voice tools

This folder contains the voice generation and playback code extracted from the presentation:

- `main.go`: standalone Go CLI that converts text or text files to MP3 and plays the result.
- `script`: convenient launcher for the Go CLI.
- `tts-service.js`: presentation server module that generates and caches MP3 files and extracts `.note-digi` blocks from `index.html`.
- `tts-player.js`: presentation browser module that reads the current slide notes, handles `.tts-pause` elements, and plays each segment.

The generated voice is AI-generated, not human. The API key is sent only to OpenAI and is never stored in the MP3 or repository.

## Requirements

- Go 1.22 or newer.
- An OpenAI API key in `OPENAI_API_KEY`.

```bash
export OPENAI_API_KEY="sk-..."
```

No npm packages or third-party Go dependencies are required by the Go CLI.

## Usage

Run the command without arguments to display the colored help menu:

```bash
cd audio-tools
go run .
```

Convert text passed directly on the command line:

```bash
go run . "Hello, this is a voice test"
```

Control the speaking style with `-instructions`, followed by instructions in quotes. Put this option before the text to read:

```bash
./script -instructions "Habla en español, con tono cálido y pausado." "Hola, esta es una prueba de voz."
```

Here, `string` in the help menu means a text value; replace it with your instructions. These instructions replace the default speaking style. The CLI omits them when using `tts-1` or `tts-1-hd`.

Convert a text file. A single argument that matches an existing file is read automatically:

```bash
go run . article.txt
```

You can also select a file explicitly or read from standard input:

```bash
go run . --file article.txt
cat article.txt | go run . --file -
```

From the repository root, the launcher provides the same behavior:

```bash
./audio-tools/script "Hello from the launcher"
./audio-tools/script notes.txt
```

The output is written to `voces/` in the directory where the command was launched. The MP3 is played automatically. Repeating the same text and configuration reuses the cached file.

## Options

```text
--file article.txt           read text from a file; use - for stdin
--no-play                    create the MP3 without playing it
--voice cedar                change the voice
--model gpt-4o-mini-tts      change the model
--speed 0.98                 change the speech speed
--instructions "..."         change the speaking style
--output-dir ./other-folder  change the output directory
```

The program also reads `OPENAI_TTS_MODEL`, `OPENAI_TTS_VOICE`, `OPENAI_TTS_SPEED`, and `OPENAI_TTS_INSTRUCTIONS`.

## Persistent CLI settings

The Go CLI and `./script` automatically read `~/.voices_env` at startup. To create it, copy the example from this directory (without overwriting an existing file):

```bash
cp -n voices_env.example ~/.voices_env
chmod 600 ~/.voices_env
```

Edit it with your preferred values:

```dotenv
OPENAI_TTS_MODEL=gpt-4o-mini-tts
OPENAI_TTS_VOICE=cedar
OPENAI_TTS_SPEED=0.98
OPENAI_TTS_INSTRUCTIONS="Habla en español, con tono natural, claro y conversacional."
```

You may also set `OPENAI_API_KEY` in this file. Priority is command options, then environment variables (including explicitly empty values), then the file, then built-in defaults. Put options before the text:

```bash
./script --voice cedar --speed 1.1 "Hola, esta es una prueba."
```

Changes apply on the next invocation. A missing file is fine. Blank lines, full-line `#` comments, optional `export`, and single or double quotes are supported. Values are literal: shell commands and variable references are never expanded. Only the five settings above are read. Malformed assignments report the file and line without printing their values.

This file configures the standalone Go CLI; the presentation's JavaScript service continues to read its process environment.

## Long files

Before sending the request, the CLI prints the character count and an approximate token count. Inputs estimated at 1,000 tokens or more display a warning because longer requests use more billable input tokens and may cost more.

`gpt-4o-mini-tts` accepts at most 2,000 input tokens. The local estimate is intentionally approximate, so very large files may need to be split into smaller files.

## Playback

- macOS: `afplay`.
- Linux: `mpv`, `ffplay`, or `xdg-open`.
- Windows: PowerShell.

Use `--no-play` on a system without an audio player.

## Install as `script`

```bash
go -C audio-tools build -o "$HOME/.local/bin/script" .
script "Text to turn into speech"
```
