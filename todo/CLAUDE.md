# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

**todo** is an interactive terminal UI (Bubble Tea) that discovers, previews and runs
the scripts in `/opt/4rji/bin/`. It offers name/description search, a grep-based content
search, per-script detail views with `chafa` image previews, source preview via a pager,
and clipboard-copy-then-execute.

> Note: this tool was rewritten from an fzf-based design to a Bubble Tea TUI. There is no
> longer any fzf, `colors.go`, `box.go` or `ui.go` — ignore older references to those.

## Architecture

- **Entry point** (`main.go`): starts the Bubble Tea program; after it exits, if the model
  has a `pendingExec` script, `executeScript` runs it (clears the screen, copies the
  command to the clipboard, execs the script, and exits with its exit code).
- **TUI model** (`model.go`, the bulk of the code): the Bubble Tea `Model`, its `Init` /
  `Update` / `View`, key handling per mode, the detail view, and all text-wrapping helpers.
  Modes: `modeLoading`, `modeBrowse`, `modeSearch`, `modeDetail`. Search modes:
  `searchNameDesc` and `searchContent`.
- **Data layer** (`scripts.go`): reads `/opt/4rji/bin/README.md`, merges executable files
  from the directory, applies `.todoignore`, scores/filters results, and runs content search.
- **Descriptions** (`descriptions.go`, `types.go`): loads `descriptions.json`; `types.go`
  holds `Script`, `DetailedDescription` (custom `UnmarshalJSON` accepting a string or a
  list of strings for `detailed_desc`) and `Descriptions` with a case-insensitive `Lookup`.
- **Styles** (`styles.go`): lipgloss color constants (cyberpunk palette) and the `Styles` struct.
- **Clipboard** (`clipboard.go`): cross-platform copy (`pbcopy` / `xclip` / `xsel`).

## Build, run, test

```sh
cd todo
go build -o todo
go run .
go test ./...
go test ./... -run TestWrapTextLines -v   # single test
```

Go 1.24 (see `go.mod`). Dependencies: the charmbracelet stack (bubbletea, bubbles,
lipgloss, glamour) — this module is NOT dependency-free.

## Hard-coded paths

`/opt/4rji/bin` and `/opt/4rji/img-bin` are hard-coded in several places:
`model.go` (`loadDataCmd`, `loadImageCmd`, `executeScript`, `openSourcePreview`),
`scripts.go` (`getCombinedScripts`, `searchInFiles`, `parseReadme` call sites) and
`descriptions.go` (`loadDescriptions`). Change all of them together if making paths
configurable.

## Data sources

```
/opt/4rji/bin/README.md         # "name  short description" per line; # headers are skipped
/opt/4rji/bin/descriptions.json # optional detailed descriptions, keyed by script name
/opt/4rji/bin/.todoignore       # optional exclude list (one name per line, # comments)
/opt/4rji/img-bin/{name}.{webp,png}  # optional preview image
```

## Search & scoring

- **Word-boundary matching** (`containsWord`): regex `\b...\b`, so "ping" won't match inside
  "spinger".
- **Scoring** (`filterScripts`): exact name +170, whole-word name +120, name substring +80;
  whole-word desc +60, desc substring +25. Ties broken by stable sort then alphabetical.
- **Content search** (`searchInFiles`): shells out to `grep -r -i -E` across the bin dir.
  There is a large hard-coded `excludedFiles` list of binaries/large files; `.todoignore`
  entries are also passed as `--exclude`. (Simplification opportunity: macOS/GNU `grep -I`
  skips binary files automatically and could replace most of that list.)

## External tools (runtime, optional)

`chafa` (image previews), `bat` (source preview, falls back to `less`/`cat`), `grep`
(content search), `pbcopy`/`xclip`/`xsel` (clipboard). All resolved via `exec.LookPath`
and degraded gracefully if missing.

## Gotchas

1. **Execute exits the process**: `executeScript` calls `os.Exit`, so the TUI never resumes
   after running a script; it passes through the script's exit code.
2. **Args are not wired up yet**: `executeScript` accepts `args`, but `main.go` always passes
   `nil`. Passing arguments to a script from the UI is not implemented.
3. **Descriptions.json is optional**: a load error is swallowed in `loadDataCmd`; the UI
   continues with README-only data.
4. **Text wrapping is custom**: `wrapTextLines` / `wrapTextLine` / `splitLongWord` handle
   indentation preservation and long-word splitting, packed up to the target width. These
   are covered by `model_test.go`.
