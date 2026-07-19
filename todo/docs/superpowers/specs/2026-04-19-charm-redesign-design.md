# Design: Charm.land Redesign — todo TUI

**Date:** 2026-04-19  
**Status:** Approved

---

## Overview

Redesign the `todo` CLI tool replacing the raw-ANSI + fzf approach with a self-contained Bubble Tea TUI. No external fuzzy-finder dependency. All interaction happens inside a single Go TUI process.

---

## Layout: Dashboard 4-Panel

```
┌─ ⬡ 4rji ──────────────────────── ● 847 scripts · normal · ctrl+/ help ─┐
├──────────────────────────┬─────────────────────────────────────────────┤
│ Scripts                  │ pingz                    executable  4.2 KB  │
│ ─────────────────────── │ ─────────────────────────────────────────── │
│ ▸ pingz      network    │ ICMP sweep utility — fast host discovery     │
│   pingm      network    │                                              │
│   pingg      network    │ Scans subnets using ICMP echo requests...    │
│   nmap       recon      │                                              │
│   nmapX      recon      │ 📁 /opt/4rji/bin/pingz                      │
│   trafico    monitoring │ ─────────────────────────────────────────── │
│   nets       network    │ Source                          space toggle │
│   backd      exploit    │ 1  #!/bin/bash                               │
│   ...                   │ 2  set -euo pipefail                         │
│                         │ 3  # Fast ICMP sweep                         │
│                         │ 5  TARGET="${1:-}"                           │
├──────────────────────────┴─────────────────────────────────────────────┤
│  ↵ execute  v view src  / search  tab content mode  space preview  q   │
└─────────────────────────────────────────────────────────────────────────┘
```

Four regions:
1. **Status bar** (top) — app name, count, mode. Dark/transparent, single border separator.
2. **List pane** (left 38%) — scrollable script list with category tags. Selected item highlighted with left border accent.
3. **Detail + Source pane** (right 62%) — split vertically: description top (~40%), raw source inline preview bottom (~60%). Source panel toggled via `space`. The inline preview is plain text (no bat — bat is only used for the full-screen `v` view).
4. **Keybindings bar** (bottom) — context-sensitive, updates based on current mode.

---

## States / Modes

| Mode | Trigger | Description |
|------|---------|-------------|
| `browse` | default | Navigate list with ↑/↓, preview updates live |
| `search` | `/` | textinput activates in status bar, list filters in real-time |
| `content` | `tab` | Search mode switches to grep-based content search |
| `source` | `v` | Full-screen viewport showing script source via `bat` or raw |
| `loading` | startup | Spinner while loading scripts and descriptions |

---

## Data Flow

```
startup
  └─ tea.Program init (mode=loading, spinner running)
       └─ tea.Cmd goroutine: loadDescriptions() + getCombinedScripts()
            └─ returns loadedMsg{scripts, descriptions} → Update() → mode=browse

keypress
  ├─ '/' → mode=search, focus textinput
  ├─ 'tab' → toggle searchMode (nameDesc ↔ content)
  ├─ ↑/↓ → move cursor, update right pane
  ├─ 'space' → toggle source preview visibility
  ├─ 'v' → tea.ExecProcess(bat script) → suspends TUI, bat runs full-screen, TUI resumes on exit
  ├─ 'Enter' → tea.Quit + executeScript() — quits TUI cleanly before handing off to script
  ├─ 'Esc' → cancel search / return to browse
  └─ 'q'/ctrl+c → quit

textinput change (search mode)
  └─ filterScriptChoices(query) → filtered []Script → re-render list
```

---

## Architecture

### Files to keep unchanged
- `scripts.go` — all discovery, filtering, scoring logic
- `types.go` — Script, DetailedDescription, Descriptions
- `descriptions.go` — JSON loading
- `clipboard.go` — cross-platform clipboard

### Files to replace/add

| File | Replaces | Purpose |
|------|----------|---------|
| `model.go` | `main.go` (loop) | Bubble Tea Model, Update, View |
| `styles.go` | `colors.go` | Lip Gloss style definitions |
| `main.go` | `main.go` (flags+loop) | tea.Program entry point only |
| ~~`ui.go`~~ | deleted | formatting absorbed into model.go View() |
| ~~`box.go`~~ | deleted | replaced by Lip Gloss borders |

### Model struct

```go
type searchMode int
const (
    searchNameDesc searchMode = iota
    searchContent
)

type appMode int
const (
    modeBrowse appMode = iota
    modeSearch
    modeSource
    modeLoading
)

type Model struct {
    // Data
    scripts      []Script
    descriptions Descriptions
    filtered     []Script
    cursor       int

    // Search
    input      textinput.Model
    searchMode searchMode
    query      string

    // Viewports
    sourceVP viewport.Model
    sourceVisible bool

    // State
    mode   appMode
    width  int
    height int

    // Loading
    spinner spinner.Model
    err     error
}
```

---

## Styling (Lip Gloss)

Theme: dark, minimal, no heavy backgrounds on bars.

| Element | Color |
|---------|-------|
| App name accent | `#58a6ff` |
| Selected item border | `#58a6ff` |
| Script name (detail) | `#f0883e` |
| Executable badge | `#7ee787` |
| Muted text / tags | `#6e7681` |
| Separator borders | `#21262d` |
| Search input border | `#58a6ff` |
| Match highlight | `#58a6ff` bg |

Status bar and keybindings bar: no background, single `border-bottom`/`border-top` in `#21262d`.

---

## Dependencies to add

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss
github.com/charmbracelet/glamour
```

Run `go mod tidy` inside `todo/` after adding.

---

## Behavior preserved from current tool

- Script execution: `executeScript()` unchanged — copies to clipboard, clears screen, `os.Exit` with script exit code
- Content search: grep-based `searchInFiles()` unchanged, triggered by `tab` in search mode
- `v` key: suspends TUI via `tea.ExecProcess`, runs `bat --style=numbers --color=always` (fallback: raw file read), resumes TUI on exit
- Image preview via `chafa`: triggered from detail view via separate keybinding `i` (fallback: skip silently if chafa missing)
- Clipboard copy on selection (before execute)
- `F5` behavior replaced by `tab` toggle

---

## Out of scope

- No arg-based startup query (flag `-s`/`--search` removed — all interaction is TUI)
- No changes to `/opt/4rji/bin/` path structure
- No tests (tool has no existing tests)
