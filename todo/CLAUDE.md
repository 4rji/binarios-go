# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is the **todo** tool — an interactive CLI script selector and executor. It helps users discover and run scripts from `/opt/4rji/bin/` with intelligent search, detailed descriptions, image previews, and script viewing/execution.

## Architecture

The tool follows a linear data flow:

1. **Script Discovery** (`scripts.go`, `descriptions.go`)
   - Reads script names and descriptions from `/opt/4rji/bin/README.md`
   - Loads detailed JSON descriptions from `/opt/4rji/bin/descriptions.json`
   - Supplements with executable files found in the bin directory
   - Returns a combined list with deduplication

2. **Search & Filtering** (`scripts.go`)
   - **Word boundary matching**: Uses regex `\b...\b` to match whole words, avoiding partial matches (e.g., "copy" won't match "copycat")
   - **Scoring system**: Name matches score 120+ points; description matches score 60. Exact name matches get a +50 bonus.
   - **Two search modes**:
     - Default: Name + description search via README/JSON
     - Content search (`-s` flag): Grep-based search across actual script file contents
   - F5 toggles between modes during interactive session

3. **Formatting & Display** (`ui.go`, `colors.go`, `box.go`)
   - Formats script list with aligned names and descriptions
   - Uses ANSI color codes and box-drawing characters for styled output
   - Different formatting for content search mode vs. normal mode

4. **Interactive Selection** (`main.go`)
   - Delegates to **fzf** for interactive fuzzy finding
   - Pre-filters results based on current query before passing to fzf
   - fzf handles final selection, Enter key binding, and F5 special key

5. **Detail View & Execution** (`main.go`)
   - Displays script info: name, short/detailed descriptions
   - Shows image preview from `/opt/4rji/img-bin/{scriptName}.{webp,png}` using **chafa**
   - User can view script source (`bat` or fallback to `cat`)
   - Executes selected script, copies command to clipboard first
   - Proper shell quoting for scripts with special characters

6. **Clipboard Integration** (`clipboard.go`)
   - Cross-platform support: macOS (`pbcopy`), Linux (`xclip` or `xsel`)
   - Warns if clipboard tool unavailable but continues execution

## Code Organization

- **main.go** (362 lines): Event loop, main menu, fzf integration, detailed description view, script execution
- **scripts.go** (346 lines): Script discovery, README parsing, file searching, filtering/scoring logic
- **types.go** (28 lines): Data structures (Script, DetailedDescription, Descriptions)
- **colors.go** (28 lines): ANSI color and text effect constants
- **ui.go** (71 lines): Display formatting (script list, fancy boxes)
- **box.go** (10 lines): Box-drawing characters
- **descriptions.go** (24 lines): JSON description loading
- **clipboard.go** (40 lines): Cross-platform clipboard abstraction

**Key files for each task:**
- Adding search features: `scripts.go` (filterScriptChoices, scoreChoice)
- UI styling changes: `colors.go`, `ui.go`, `box.go`
- Changing data sources: `scripts.go` (getCombinedScripts, parseReadme), `descriptions.go`
- Script execution logic: `main.go` (executeScript, showDetailedDescription)

## Hard-Coded Paths

The tool assumes this directory structure:

```
/opt/4rji/bin/
├── README.md                 # Script list and short descriptions
├── descriptions.json         # (Optional) Detailed descriptions
└── [executable scripts]

/opt/4rji/img-bin/
└── [scriptName].{webp,png}   # (Optional) Script preview images
```

These paths are scattered throughout the codebase. If refactoring to make paths configurable, update:
- `main.go`: lines 151, 220
- `scripts.go`: lines 132, 173, 218, 298
- `descriptions.go`: line 11
- `main.go` (showImage): line 16

## Common Development Tasks

### Build and Run

```bash
cd todo
go build -o todo        # Build binary to current directory
go run .               # Run with no arguments (interactive mode)
go run . -s tunnel     # Run with search query
go run . --search net  # Same as above (long flag)
```

### Testing Search Behavior

```bash
# Test name/description search (default mode)
go run . veri          # Searches README/descriptions for "veri"

# Test content search (grep mode)
go run . -s tunnel     # Searches script file contents for "tunnel"
```

### Debug Output

Add print statements in key functions:
- `filterScriptChoices()`: See which scripts match and their scores
- `parseReadme()`: Confirm scripts loaded from README
- `getCombinedScripts()`: Track merging of README + bin directory scripts
- `executeScript()`: Verify command quoting and execution

### Format and Edge Cases

- **Special characters in script names**: Handled in `executeScript()` with single-quote escaping
- **ANSI color stripping**: `stripANSI()` in `scripts.go` removes formatting before display/comparison
- **Word boundary matching**: `containsWord()` uses regex to prevent false positives (e.g., "ping" won't match "spinger")
- **Empty results**: Both search modes return early with no output if no matches found

## External Dependencies

- **fzf**: Interactive fuzzy finder (must be in PATH)
- **chafa**: ASCII art converter for image preview display (optional, gracefully skipped if missing)
- **bat**: Syntax-highlighted file viewer (optional; falls back to raw output if missing)
- **grep**: Used in content search mode (`-s` flag)
- **pbcopy** (macOS), **xclip** or **xsel** (Linux): Clipboard tools

All are checked at runtime via `exec.LookPath()`. Missing tools are handled gracefully except fzf (if fzf is missing, the tool falls back to picking the first result).

## Go Version

Go 1.24.1 (specified in go.mod, no external dependencies in go.mod)

## Key Patterns and Gotchas

1. **Scoring is stable-sorted**: Stable sort means equal-score items preserve their original order, then alphabetical sort breaks ties. This avoids randomness.

2. **ANSI codes in choices**: Script names and descriptions are colored before passing to fzf. fzf's `--ansi` flag preserves this coloring. The `stripANSI()` function handles removal when extracting clean names.

3. **fzf expects newline-delimited input**: Script list is joined with `\n` and piped to fzf via stdin. `--print-query` and `--delimiter` work together to parse the selection.

4. **Word boundary regex**: The pattern `\b...\b` requires word boundaries (space, punctuation, or string start/end). Useful for avoiding "pingm" matching inside "pingmesh".

5. **Descriptions.json is optional**: The tool gracefully handles missing JSON; it logs an error but continues with README-only data.

6. **Content search excludes binary/large files**: The hardcoded exclude list in `searchInFiles()` prevents grep from searching vendored binaries, large files, or non-script formats (like `.zip`, `.exe`). Extend this list if adding new binary/large files to `/opt/4rji/bin/`.

7. **Script execution and exit**: `executeScript()` calls `os.Exit()` unconditionally after script completion, passing through the script's exit code. This means the todo tool's main loop never resumes after execution.
