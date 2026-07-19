# Charm.land Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the raw-ANSI + fzf todo TUI with a self-contained Bubble Tea dashboard — 4-panel layout, no external fuzzy-finder, 100% Go.

**Architecture:** Single Bubble Tea program with a Model that owns all state. The 4 panels (status bar, list, detail+source, keybindings) are rendered via Lip Gloss in `View()`. Script discovery/filtering logic in `scripts.go` stays unchanged; only the UI layer is replaced.

**Tech Stack:** `bubbletea`, `bubbles` (textinput, viewport, spinner), `lipgloss`, `glamour` — all from `github.com/charmbracelet/*`.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modify | Add 4 Charm deps |
| `styles.go` | **Create** | Lip Gloss style definitions (replaces `colors.go`) |
| `model.go` | **Create** | Bubble Tea Model, Update, View — all TUI logic |
| `main.go` | **Rewrite** | Entry point only: `tea.NewProgram(initialModel()).Run()` + exec handoff |
| `scripts.go` | Modify | Add `filterScripts([]Script, string) []Script` |
| `colors.go` | **Delete** | Replaced by `styles.go` |
| `ui.go` | **Delete** | Formatting absorbed into `model.go` `View()` |
| `box.go` | **Delete** | Replaced by Lip Gloss borders |
| `types.go` | Unchanged | — |
| `descriptions.go` | Unchanged | — |
| `clipboard.go` | Unchanged | — |

---

## Task 1: Add Charm dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the four Charm libraries**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/glamour@latest
go mod tidy
```

- [ ] **Step 2: Verify go.mod has the deps**

```bash
grep charmbracelet go.mod
```

Expected output (versions may differ):
```
github.com/charmbracelet/bubbles v0.21.0
github.com/charmbracelet/bubbletea v1.3.4
github.com/charmbracelet/glamour v0.9.1
github.com/charmbracelet/lipgloss v1.1.0
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add charmbracelet/bubbletea, bubbles, lipgloss, glamour"
```

---

## Task 2: Create `styles.go`

**Files:**
- Create: `todo/styles.go`
- Delete: `todo/colors.go`

- [ ] **Step 1: Create `styles.go`**

```go
package main

import "github.com/charmbracelet/lipgloss"

const (
	colorAccent     = lipgloss.Color("#58a6ff")
	colorScriptName = lipgloss.Color("#f0883e")
	colorGreen      = lipgloss.Color("#7ee787")
	colorMuted      = lipgloss.Color("#6e7681")
	colorBorder     = lipgloss.Color("#21262d")
	colorText       = lipgloss.Color("#e6edf3")
	colorSubtext    = lipgloss.Color("#8b949e")
	colorSelected   = lipgloss.Color("#161b22")
)

type Styles struct {
	StatusBar   lipgloss.Style
	ListHeader  lipgloss.Style
	ListItem    lipgloss.Style
	ListItemSel lipgloss.Style
	ListTag     lipgloss.Style
	DetailName  lipgloss.Style
	DetailBadge lipgloss.Style
	DetailMeta  lipgloss.Style
	DetailBody  lipgloss.Style
	SrcHeader   lipgloss.Style
	SrcLine     lipgloss.Style
	SrcLineNum  lipgloss.Style
	KeyBind     lipgloss.Style
	KeyLabel    lipgloss.Style
	SearchBox   lipgloss.Style
	MatchHL     lipgloss.Style
}

func newStyles() Styles {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)

	return Styles{
		StatusBar: border.
			BorderBottom(true).
			PaddingLeft(1).
			PaddingRight(1),

		ListHeader: border.
			BorderBottom(true).
			Foreground(colorMuted).
			PaddingLeft(2),

		ListItem: lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(colorMuted),

		ListItemSel: lipgloss.NewStyle().
			PaddingLeft(0).
			Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
			BorderForeground(colorAccent).
			Foreground(colorText).
			Background(colorSelected),

		ListTag: lipgloss.NewStyle().
			Foreground(colorMuted),

		DetailName: lipgloss.NewStyle().
			Foreground(colorScriptName).
			Bold(true),

		DetailBadge: lipgloss.NewStyle().
			Foreground(colorGreen).
			PaddingLeft(1).
			PaddingRight(1),

		DetailMeta: lipgloss.NewStyle().
			Foreground(colorMuted),

		DetailBody: lipgloss.NewStyle().
			Foreground(colorSubtext),

		SrcHeader: border.
			BorderTop(true).
			BorderBottom(true).
			Foreground(colorMuted).
			PaddingLeft(2),

		SrcLine: lipgloss.NewStyle().
			Foreground(colorSubtext),

		SrcLineNum: lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(3),

		KeyBind: lipgloss.NewStyle().
			Foreground(colorAccent),

		KeyLabel: lipgloss.NewStyle().
			Foreground(colorMuted),

		SearchBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Foreground(colorAccent).
			PaddingLeft(1).
			PaddingRight(1),

		MatchHL: lipgloss.NewStyle().
			Background(colorAccent).
			Foreground(lipgloss.Color("#0d1117")),
	}
}
```

- [ ] **Step 2: Delete `colors.go`**

```bash
rm /Users/bellaquita/github/binarios-go/todo/colors.go
```

- [ ] **Step 3: Verify build still fails (colors.go is gone, but nothing uses styles.go yet)**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go build ./... 2>&1 | head -20
```

Expected: compile errors referencing `ColorRed`, `ColorReset`, etc. — that's correct, we'll fix in Task 7.

- [ ] **Step 4: Commit**

```bash
git add styles.go
git rm colors.go
git commit -m "feat: add lipgloss styles, remove raw ANSI colors"
```

---

## Task 3: Add `filterScripts` to `scripts.go`

**Files:**
- Modify: `todo/scripts.go`

The existing `filterScriptChoices` works on formatted strings (`"name · desc"`). The new TUI works directly with `[]Script`, so we need a typed version.

- [ ] **Step 1: Add `filterScripts` at the bottom of `scripts.go`**

```go
// filterScripts filters and scores a []Script slice by query, returning matches sorted by score.
func filterScripts(scripts []Script, query string) []Script {
	if query == "" {
		return scripts
	}
	type scored struct {
		s     Script
		score int
	}
	var results []scored
	queryLower := strings.ToLower(query)
	for _, s := range scripts {
		score := 0
		if containsWord(s.Name, query) {
			score += 120
			if strings.EqualFold(s.Name, query) {
				score += 50
			}
		}
		if containsWord(s.Desc, query) {
			score += 60
		} else if strings.Contains(strings.ToLower(s.Desc), queryLower) {
			score += 25
		}
		if score > 0 {
			results = append(results, scored{s: s, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return strings.ToLower(results[i].s.Name) < strings.ToLower(results[j].s.Name)
		}
		return results[i].score > results[j].score
	})
	out := make([]Script, len(results))
	for i, r := range results {
		out[i] = r.s
	}
	return out
}
```

- [ ] **Step 2: Commit**

```bash
git add scripts.go
git commit -m "feat: add filterScripts for direct Script slice filtering"
```

---

## Task 4: Create `model.go` — types, struct, Init, loading

**Files:**
- Create: `todo/model.go`

- [ ] **Step 1: Create `model.go` with types, Model struct, initialModel, Init, and load command**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type appMode int

const (
	modeLoading appMode = iota
	modeBrowse
	modeSearch
)

type searchMode int

const (
	searchNameDesc searchMode = iota
	searchContent
)

// loadedMsg carries initial data back from the async load goroutine.
type loadedMsg struct {
	scripts      []Script
	descriptions Descriptions
	err          error
}

// contentSearchMsg carries grep results from the async content search goroutine.
type contentSearchMsg struct {
	scripts []Script
	err     error
}

// Model is the Bubble Tea application state.
type Model struct {
	scripts      []Script
	descriptions Descriptions
	filtered     []Script
	cursor       int
	scrollOffset int

	input      textinput.Model
	searchMode searchMode
	query      string

	sourceVP      viewport.Model
	sourceVisible bool

	pendingExec string // script name; non-empty triggers execution after tea.Quit

	mode   appMode
	width  int
	height int

	spinner spinner.Model
	err     error
	styles  Styles
}

func initialModel() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ti := textinput.New()
	ti.Placeholder = "search scripts…"
	ti.CharLimit = 64
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorAccent)

	return Model{
		mode:    modeLoading,
		spinner: sp,
		input:   ti,
		styles:  newStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadDataCmd())
}

func loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		descriptions, _ := loadDescriptions()
		scripts, err := getCombinedScripts("/opt/4rji/bin/README.md", "/opt/4rji/bin")
		if err != nil {
			return loadedMsg{err: err}
		}
		sort.Slice(scripts, func(i, j int) bool {
			return strings.ToLower(scripts[i].Name) < strings.ToLower(scripts[j].Name)
		})
		return loadedMsg{scripts: scripts, descriptions: descriptions}
	}
}

func contentSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		scripts, err := searchInFiles(query)
		return contentSearchMsg{scripts: scripts, err: err}
	}
}
```

- [ ] **Step 2: Verify it compiles (model.go only, missing Update/View — expected errors)**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go build ./... 2>&1 | grep -v "undefined"
```

Expected: errors about missing Update/View methods (they come in next tasks), plus errors from old files that reference ColorReset etc. That's fine for now.

- [ ] **Step 3: Commit**

```bash
git add model.go
git commit -m "feat: add model types, struct, Init, and async load command"
```

---

## Task 5: Implement `Update()` in `model.go`

**Files:**
- Modify: `todo/model.go`

- [ ] **Step 1: Append `Update()` to `model.go`**

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.mode == modeLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case loadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.mode = modeBrowse
			return m, nil
		}
		m.scripts = msg.scripts
		m.descriptions = msg.descriptions
		m.filtered = msg.scripts
		m.mode = modeBrowse
		return m, nil

	case contentSearchMsg:
		if msg.err == nil {
			m.filtered = msg.scripts
			m.cursor = 0
			m.scrollOffset = 0
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.mode == modeSearch {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		newQuery := m.input.Value()
		if newQuery != m.query {
			m.query = newQuery
			if m.searchMode == searchContent && newQuery != "" {
				return m, contentSearchCmd(newQuery)
			}
			m.filtered = filterScripts(m.scripts, m.query)
			m.cursor = 0
			m.scrollOffset = 0
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c", "q":
		if m.mode == modeSearch {
			m.mode = modeBrowse
			m.query = ""
			m.input.SetValue("")
			m.input.Blur()
			m.filtered = m.scripts
			m.cursor = 0
			m.scrollOffset = 0
			return m, nil
		}
		return m, tea.Quit

	case "esc":
		if m.mode == modeSearch {
			m.mode = modeBrowse
			m.query = ""
			m.input.SetValue("")
			m.input.Blur()
			m.filtered = m.scripts
			m.cursor = 0
			m.scrollOffset = 0
		}
		return m, nil

	case "/":
		if m.mode == modeBrowse {
			m.mode = modeSearch
			m.searchMode = searchNameDesc
			m.input.Focus()
		}
		return m, textinput.Blink

	case "tab":
		if m.mode == modeSearch {
			if m.searchMode == searchNameDesc {
				m.searchMode = searchContent
				if m.query != "" {
					return m, contentSearchCmd(m.query)
				}
			} else {
				m.searchMode = searchNameDesc
				m.filtered = filterScripts(m.scripts, m.query)
				m.cursor = 0
				m.scrollOffset = 0
			}
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			visRows := m.listVisibleRows()
			if m.cursor >= m.scrollOffset+visRows {
				m.scrollOffset = m.cursor - visRows + 1
			}
		}
		return m, nil

	case "pgup":
		visRows := m.listVisibleRows()
		m.cursor -= visRows
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.scrollOffset = m.cursor
		return m, nil

	case "pgdown":
		visRows := m.listVisibleRows()
		m.cursor += visRows
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor >= m.scrollOffset+visRows {
			m.scrollOffset = m.cursor - visRows + 1
		}
		return m, nil

	case "space":
		m.sourceVisible = !m.sourceVisible
		return m, nil

	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		scriptName := m.filtered[m.cursor].Name
		_ = copyToClipboard(scriptName)
		m.pendingExec = scriptName
		return m, tea.Quit

	case "v":
		if len(m.filtered) == 0 {
			return m, nil
		}
		scriptName := m.filtered[m.cursor].Name
		scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", scriptName)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return m, nil
		}
		batPath, err := exec.LookPath("bat")
		var cmd *exec.Cmd
		if err == nil {
			cmd = exec.Command(batPath, "--style=numbers", "--color=always", "--language=bash", "--paging=always", scriptPath)
		} else {
			cmd = exec.Command("less", scriptPath)
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return nil })

	case "i":
		if len(m.filtered) == 0 {
			return m, nil
		}
		scriptName := m.filtered[m.cursor].Name
		imgBase := fmt.Sprintf("/opt/4rji/img-bin/%s", scriptName)
		imgPath := ""
		if _, err := os.Stat(imgBase + ".webp"); err == nil {
			imgPath = imgBase + ".webp"
		} else if _, err := os.Stat(imgBase + ".png"); err == nil {
			imgPath = imgBase + ".png"
		}
		if imgPath == "" {
			return m, nil
		}
		chafaPath, err := exec.LookPath("chafa")
		if err != nil {
			return m, nil
		}
		cmd := exec.Command(chafaPath, "--size", "80x40", imgPath)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return nil })
	}

	if m.mode == modeSearch {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		newQuery := m.input.Value()
		if newQuery != m.query {
			m.query = newQuery
			if m.searchMode == searchContent && newQuery != "" {
				return m, contentSearchCmd(newQuery)
			}
			m.filtered = filterScripts(m.scripts, m.query)
			m.cursor = 0
			m.scrollOffset = 0
		}
		return m, cmd
	}

	return m, nil
}

// listVisibleRows returns how many script rows fit in the list pane.
func (m Model) listVisibleRows() int {
	// total height minus: status bar (2) + list header (2) + keybindings bar (2)
	rows := m.height - 6
	if rows < 1 {
		rows = 1
	}
	return rows
}
```

- [ ] **Step 2: Commit**

```bash
git add model.go
git commit -m "feat: implement Update and key handling"
```

---

## Task 6: Implement `View()` helper methods in `model.go`

**Files:**
- Modify: `todo/model.go`

- [ ] **Step 1: Append view helpers to `model.go`**

```go
// ── View helpers ──────────────────────────────────────────────

func (m Model) viewLoading() string {
	msg := m.spinner.View() + " Loading scripts…"
	if m.err != nil {
		msg = "Error: " + m.err.Error()
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(colorAccent).
		Render(msg)
}

func (m Model) viewStatusBar() string {
	appName := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("⬡ 4rji")

	countStr := fmt.Sprintf("● %d scripts", len(m.scripts))
	count := lipgloss.NewStyle().Foreground(colorMuted).Render(countStr)

	modeStr := "normal"
	if m.mode == modeSearch {
		modeStr = "search"
		if m.searchMode == searchContent {
			modeStr = "content"
		}
	}
	modeR := lipgloss.NewStyle().Foreground(colorMuted).Render(modeStr)

	hint := lipgloss.NewStyle().Foreground(colorMuted).Render("ctrl+/ help")

	var center string
	if m.mode == modeSearch {
		matches := fmt.Sprintf("%d matches", len(m.filtered))
		matchR := lipgloss.NewStyle().Foreground(colorScriptName).Render(matches)
		center = m.styles.SearchBox.Render("⌕ "+m.input.View()) + "  " + matchR
	} else {
		center = lipgloss.NewStyle().Foreground(colorMuted).Render("/ to search")
	}

	gap := m.width - lipgloss.Width(appName) - lipgloss.Width(center) - lipgloss.Width(count) - lipgloss.Width(modeR) - lipgloss.Width(hint) - 8
	if gap < 1 {
		gap = 1
	}
	sp := strings.Repeat(" ", gap/4)

	row := appName + sp + center + sp + count + "  " + modeR + "  " + hint
	return m.styles.StatusBar.Width(m.width - 2).Render(row)
}

func (m Model) viewList(width, height int) string {
	header := m.styles.ListHeader.Width(width - 2).Render("Scripts")
	headerH := lipgloss.Height(header)
	visRows := height - headerH
	if visRows < 0 {
		visRows = 0
	}

	var rows []string
	for i := m.scrollOffset; i < len(m.filtered) && len(rows) < visRows; i++ {
		s := m.filtered[i]
		name := s.Name
		tag := lipgloss.NewStyle().Foreground(colorMuted).Render(guessTag(name))

		nameW := width - lipgloss.Width(tag) - 4
		if nameW < 1 {
			nameW = 1
		}
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		namePad := strings.Repeat(" ", nameW-len(name))

		line := name + namePad + tag
		if i == m.cursor {
			rows = append(rows, m.styles.ListItemSel.Width(width-2).Render(line))
		} else {
			rows = append(rows, m.styles.ListItem.Width(width-2).Render(line))
		}
	}

	// Pad remaining rows so the pane has a fixed height
	for len(rows) < visRows {
		rows = append(rows, lipgloss.NewStyle().Width(width).Render(""))
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)
}

// guessTag returns a short category tag based on the script name.
func guessTag(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "ping") || strings.Contains(n, "net") || strings.Contains(n, "nmap") ||
		strings.Contains(n, "scan") || strings.Contains(n, "traf") || strings.Contains(n, "loc"):
		return "net"
	case strings.Contains(n, "back") || strings.Contains(n, "shell") || strings.Contains(n, "rev") ||
		strings.Contains(n, "expl") || strings.Contains(n, "pwn"):
		return "xpl"
	case strings.Contains(n, "kas") || strings.Contains(n, "inst") || strings.Contains(n, "sys"):
		return "sys"
	case strings.Contains(n, "copy") || strings.Contains(n, "file") || strings.Contains(n, "dir"):
		return "fs"
	default:
		return "tool"
	}
}

func (m Model) viewDetail(width, height int) string {
	if len(m.filtered) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(colorMuted).Align(lipgloss.Center, lipgloss.Center).
			Render("No scripts")
	}

	script := m.filtered[m.cursor]
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", script.Name)

	// Header line: name + badge + size
	name := m.styles.DetailName.Render(script.Name)
	badge := ""
	sizeStr := ""
	if info, err := os.Stat(scriptPath); err == nil {
		if info.Mode()&0111 != 0 {
			badge = m.styles.DetailBadge.Render("executable")
		}
		sizeStr = m.styles.DetailMeta.Render(fmt.Sprintf("%.1f KB", float64(info.Size())/1024))
	}
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, name, "  ", badge, "  ", sizeStr)

	// Short description
	shortDesc := script.Desc
	detailedDesc := ""
	if d, ok := m.descriptions[script.Name]; ok {
		if d.ShortDesc != "" {
			shortDesc = d.ShortDesc
		}
		detailedDesc = d.DetailedDesc
	}

	shortR := m.styles.DetailBody.Width(width - 4).Render(shortDesc)

	// Detailed description via glamour (if available)
	detailR := ""
	if detailedDesc != "" {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width-4),
		)
		if err == nil {
			rendered, err := renderer.Render(detailedDesc)
			if err == nil {
				detailR = rendered
			}
		}
		if detailR == "" {
			detailR = m.styles.DetailBody.Width(width - 4).Render(detailedDesc)
		}
	}

	// Path
	pathR := m.styles.DetailMeta.Render("📁 " + scriptPath)

	sections := []string{headerLine, "", shortR}
	if detailR != "" {
		sections = append(sections, detailR)
	}
	sections = append(sections, "", pathR)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) viewSource(width, height int) string {
	if len(m.filtered) == 0 {
		return ""
	}
	script := m.filtered[m.cursor]
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", script.Name)

	header := m.styles.SrcHeader.Width(width - 2).
		Render("Source" + strings.Repeat(" ", max(0, width-18)) + "space toggle")
	headerH := lipgloss.Height(header)
	contentH := height - headerH
	if contentH < 1 {
		contentH = 1
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return lipgloss.JoinVertical(lipgloss.Left, header,
			lipgloss.NewStyle().Foreground(colorMuted).Render("(not readable)"))
	}

	lines := strings.Split(string(content), "\n")
	var rendered []string
	for i, line := range lines {
		if i >= contentH {
			break
		}
		lineNum := m.styles.SrcLineNum.Render(fmt.Sprintf("%3d", i+1))
		lineContent := m.styles.SrcLine.Width(width - 5).Render(line)
		rendered = append(rendered, lineNum+" "+lineContent)
	}
	// Pad to fill height
	for len(rendered) < contentH {
		rendered = append(rendered, lipgloss.NewStyle().Width(width).Render(""))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, rendered...)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m Model) viewKeybindings() string {
	bind := func(key, label string) string {
		return m.styles.KeyBind.Render(key) + " " + m.styles.KeyLabel.Render(label)
	}

	var parts []string
	parts = append(parts, bind("↵", "execute"))
	parts = append(parts, bind("v", "view src"))
	parts = append(parts, bind("i", "image"))
	if m.mode == modeSearch {
		parts = append(parts, bind("tab", "toggle mode"))
		parts = append(parts, bind("esc", "cancel"))
	} else {
		parts = append(parts, bind("/", "search"))
		parts = append(parts, bind("space", "preview"))
		parts = append(parts, bind("q", "quit"))
	}

	row := strings.Join(parts, "   ")
	return m.styles.StatusBar.Width(m.width - 2).Render(row)
}

// max() is a Go 1.21+ built-in — do NOT define it here.
```

- [ ] **Step 2: Commit**

```bash
git add model.go
git commit -m "feat: add View helper methods (list, detail, source, bars)"
```

---

## Task 7: Implement `View()` in `model.go`

**Files:**
- Modify: `todo/model.go`

- [ ] **Step 1: Append `View()` to `model.go`**

```go
func (m Model) View() string {
	if m.mode == modeLoading || m.width == 0 {
		return m.viewLoading()
	}

	statusBar := m.viewStatusBar()
	keybindings := m.viewKeybindings()
	statusH := lipgloss.Height(statusBar)
	keybindH := lipgloss.Height(keybindings)
	bodyH := m.height - statusH - keybindH
	if bodyH < 1 {
		bodyH = 1
	}

	leftW := int(float64(m.width) * 0.38)
	rightW := m.width - leftW - 1 // -1 for divider char

	// Left pane with right border acting as divider
	leftContent := m.viewList(leftW, bodyH)
	leftPane := lipgloss.NewStyle().
		Width(leftW).
		Height(bodyH).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(colorBorder).
		Render(leftContent)

	// Right pane: detail on top, source on bottom (if visible)
	var rightContent string
	if m.sourceVisible {
		detailH := int(float64(bodyH) * 0.4)
		sourceH := bodyH - detailH
		detail := m.viewDetail(rightW, detailH)
		source := m.viewSource(rightW, sourceH)
		rightContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(rightW).Height(detailH).Render(detail),
			lipgloss.NewStyle().Width(rightW).Height(sourceH).Render(source),
		)
	} else {
		detail := m.viewDetail(rightW, bodyH)
		rightContent = lipgloss.NewStyle().Width(rightW).Height(bodyH).Render(detail)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightContent)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, keybindings)
}
```

- [ ] **Step 2: Commit**

```bash
git add model.go
git commit -m "feat: implement View() layout composition"
```

---

## Task 8: Rewrite `main.go` and delete old UI files

**Files:**
- Rewrite: `todo/main.go`
- Delete: `todo/ui.go`, `todo/box.go`

- [ ] **Step 1: Replace `main.go` entirely**

`main.go` keeps `executeScript` (it's called after `p.Run()`) plus the new `main`. Replace the entire file with:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(Model); ok && m.pendingExec != "" {
		executeScript(m.pendingExec, nil)
	}
}

// executeScript copies the command to clipboard, clears the screen, and exec's the script.
// Calls os.Exit with the script's exit code — the TUI is already gone at this point.
func executeScript(scriptName string, args []string) {
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", scriptName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "script not found: %s\n", scriptPath)
		os.Exit(1)
	}

	var commandParts []string
	quotedPath := scriptPath
	if strings.ContainsAny(scriptPath, " '\"`$*&|(){}[];<>?!\\#") {
		quotedPath = "'" + strings.ReplaceAll(scriptPath, "'", "'\\''") + "'"
	}
	commandParts = append(commandParts, quotedPath)
	for _, arg := range args {
		quoted := arg
		if strings.ContainsAny(arg, " '\"`$*&|(){}[];<>?!\\#") {
			quoted = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
		commandParts = append(commandParts, quoted)
	}
	_ = copyToClipboard(strings.Join(commandParts, " "))

	fmt.Print("\033[H\033[2J")
	cmd := exec.Command(scriptPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
```

- [ ] **Step 2: Delete old UI files**

```bash
rm /Users/bellaquita/github/binarios-go/todo/ui.go
rm /Users/bellaquita/github/binarios-go/todo/box.go
```

- [ ] **Step 3: Remove old functions from `scripts.go` that are now unused**

Remove from `scripts.go` (the rest is deleted via `ui.go`/`box.go`/`colors.go` already gone):
- `ScriptChoice` type
- `scoreChoice`
- `sortScoredChoices`
- `extractChoiceName`
- `extractChoiceFields`
- `filterScriptChoicesWithGrep`
- `filterScriptChoices`

Keep: `containsWord`, `stripANSI`, `parseReadme`, `getCombinedScripts`, `searchInFiles`, `filterScripts`.

- [ ] **Step 4: Build**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go build ./...
```

Expected: clean build. If there are `undefined` errors, trace which file still references the deleted symbols and remove those references.

- [ ] **Step 5: Commit**

```bash
git add main.go scripts.go
git rm ui.go box.go
git commit -m "feat: rewrite main.go as tea.Program entry point, remove old UI files"
```

---

## Task 9: Smoke test and fix layout issues

**Files:**
- Modify: `todo/model.go` (as needed)

- [ ] **Step 1: Run the TUI**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go run .
```

Check:
- [ ] Status bar renders without overflow
- [ ] Script list scrolls with ↑/↓
- [ ] Selected item has blue left border
- [ ] Right pane shows name + description for selected script
- [ ] `space` toggles source preview
- [ ] `/` opens search, typing filters list in real-time
- [ ] `esc` exits search and restores full list
- [ ] `tab` in search mode switches to content search (grep)
- [ ] `v` suspends TUI and opens bat (or less), TUI resumes on exit
- [ ] `enter` quits TUI and executes the script
- [ ] `q` / `ctrl+c` quits cleanly

- [ ] **Step 2: Fix `listVisibleRows()` if list height is wrong**

The formula in `listVisibleRows()` must match the actual rendered heights. If the list overflows or leaves gaps, update it. The correct formula:

```go
func (m Model) listVisibleRows() int {
	statusH := lipgloss.Height(m.viewStatusBar())
	keyH := lipgloss.Height(m.viewKeybindings())
	listHeaderH := 2 // header line + border
	rows := m.height - statusH - keyH - listHeaderH
	if rows < 1 {
		rows = 1
	}
	return rows
}
```

- [ ] **Step 3: Fix status bar overflow if terminal is narrow**

If the status bar wraps or looks broken on narrow terminals (< 80 cols), add a width guard in `viewStatusBar()`:

```go
// Near the bottom of viewStatusBar, before the final Render:
if lipgloss.Width(row) > m.width-4 {
    row = appName + "  " + count + "  " + hint
}
```

- [ ] **Step 4: Commit fixes**

```bash
git add model.go
git commit -m "fix: layout height calculations and narrow terminal handling"
```

---

## Task 10: Final polish — match highlights and keybindings help

**Files:**
- Modify: `todo/model.go`

- [ ] **Step 1: Highlight query matches in list items**

In `viewList()`, after computing `name`, wrap matched characters with `m.styles.MatchHL`:

```go
// Replace this line in viewList():
//   line := name + namePad + tag
// With:
line := highlightMatch(name, m.query) + namePad + tag
```

Add the helper at the bottom of `model.go`:

```go
func highlightMatch(text, query string) string {
	if query == "" {
		return text
	}
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx < 0 {
		return text
	}
	hl := lipgloss.NewStyle().
		Background(colorAccent).
		Foreground(lipgloss.Color("#0d1117"))
	return text[:idx] + hl.Render(text[idx:idx+len(q)]) + text[idx+len(q):]
}
```

- [ ] **Step 2: Build and run**

```bash
cd /Users/bellaquita/github/binarios-go/todo
go build ./... && go run .
```

Type a query — matched characters should appear with a blue background in list items.

- [ ] **Step 3: Commit**

```bash
git add model.go
git commit -m "feat: highlight query matches in list items"
```

---

## Task 11: Save memory and write session summary

- [ ] **Step 1: Save architecture decision to engram**

```
mem_save:
  title: "Redesigned todo TUI with Charm.land stack"
  type: architecture
  content: |
    What: Replaced fzf + raw ANSI todo tool with full Bubble Tea TUI.
    Why: User wanted a beautiful self-contained Charm.land design — no external deps.
    Where: todo/model.go, todo/styles.go, todo/main.go
    Learned:
      - tea.ExecProcess suspends TUI, runs bat/chafa full-screen, resumes cleanly.
      - Execute (Enter) pattern: store pendingExec in Model, return tea.Quit, read in main.go after p.Run().
      - filterScripts() added to scripts.go to work directly on []Script (not formatted strings).
      - listVisibleRows() height formula must account for all rendered panel heights.
```

- [ ] **Step 2: Call mem_session_summary**

- [ ] **Step 3: Final commit**

```bash
git add .
git commit -m "feat: complete Charm.land TUI redesign"
```
