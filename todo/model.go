package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type appMode int

const (
	modeLoading appMode = iota
	modeBrowse
	modeSearch
	modeDetail
)

const (
	imagePreviewWidth  = 88
	imagePreviewHeight = 36
)

type searchMode int

const (
	searchNameDesc searchMode = iota
	searchContent
)

type loadedMsg struct {
	scripts      []Script
	descriptions Descriptions
	err          error
}

type contentSearchMsg struct {
	scripts []Script
	err     error
}

type imageMsg struct {
	name   string
	output string // chafa ANSI output
}

type warningMsg struct {
	text string
}

type clearWarningMsg struct{}

type tickMsg struct{}

type Model struct {
	scripts      []Script
	descriptions Descriptions
	filtered     []Script
	cursor       int
	offset       int

	input      textinput.Model
	searchMode searchMode
	query      string

	pendingExec string

	mode         appMode
	width        int
	height       int
	detailOffset int

	spinner    spinner.Model
	err        error
	styles     Styles
	imageCache map[string]string // scriptName -> chafa output
	warning    string            // temporary warning message
	warningExp time.Time         // when warning expires
}

func initialModel() Model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	ti := textinput.New()
	ti.Placeholder = "query//"
	ti.CharLimit = 64
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorScriptName).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorAccent)
	ti.Width = 40

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

func clearWarningAfterDelay() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func loadImageCmd(scriptName string) tea.Cmd {
	return func() tea.Msg {
		chafaPath, err := exec.LookPath("chafa")
		if err != nil {
			return warningMsg{text: "chafa required for image previews"}
		}
		for _, ext := range []string{"webp", "png"} {
			imgPath := fmt.Sprintf("/opt/4rji/img-bin/%s.%s", scriptName, ext)
			if _, err := os.Stat(imgPath); err != nil {
				continue
			}
			sizeArg := fmt.Sprintf("%dx%d", imagePreviewWidth, imagePreviewHeight)
			out, err := exec.Command(chafaPath, "--size", sizeArg, "--animate", "false", imgPath).Output()
			if err != nil {
				return imageMsg{name: scriptName, output: ""}
			}
			// strip cursor hide/show escape sequences chafa emits
			result := strings.ReplaceAll(string(out), "\x1b[?25l", "")
			result = strings.ReplaceAll(result, "\x1b[?25h", "")
			return imageMsg{name: scriptName, output: result}
		}
		return imageMsg{name: scriptName, output: ""}
	}
}

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
		} else {
			m.scripts = msg.scripts
			m.descriptions = msg.descriptions
			m.filtered = msg.scripts
		}
		m.mode = modeBrowse
		return m, nil

	case contentSearchMsg:
		if msg.err == nil {
			m.filtered = msg.scripts
			m.cursor = 0
			m.offset = 0
		}
		return m, nil

	case imageMsg:
		if m.imageCache == nil {
			m.imageCache = make(map[string]string)
		}
		m.imageCache[msg.name] = msg.output
		return m, nil

	case warningMsg:
		m.warning = msg.text
		m.warningExp = time.Now().Add(3 * time.Second)
		return m, clearWarningAfterDelay()

	case clearWarningMsg:
		m.warning = ""
		return m, nil

	case tickMsg:
		if m.warning != "" && time.Now().After(m.warningExp) {
			m.warning = ""
			return m, nil
		}
		if m.warning != "" {
			return m, clearWarningAfterDelay()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.mode == modeSearch {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refilter()
		return m, cmd
	}

	return m, nil
}

func (m *Model) refilter() {
	newQuery := m.input.Value()
	if newQuery == m.query {
		return
	}
	m.query = newQuery
	if m.searchMode == searchContent {
		return
	}
	m.filtered = filterScripts(m.scripts, m.query)
	m.cursor = 0
	m.offset = 0
}

func (m Model) returnFromDetail() (tea.Model, tea.Cmd) {
	m.detailOffset = 0
	if m.query != "" {
		m.mode = modeSearch
		m.input.Focus()
		return m, tea.Batch(tea.ClearScreen, textinput.Blink)
	}
	m.mode = modeBrowse
	return m, tea.ClearScreen
}

func (m Model) enterDetail() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	m.mode = modeDetail
	m.detailOffset = 0
	scriptName := m.filtered[m.cursor].Name
	if m.imageCache == nil {
		m.imageCache = make(map[string]string)
	}
	if _, cached := m.imageCache[scriptName]; !cached {
		return m, loadImageCmd(scriptName)
	}
	return m, nil
}

func (m Model) maxDetailOffset() int {
	if m.height <= 0 {
		return 0
	}
	lines := m.detailLines()
	if len(lines) <= m.height {
		return 0
	}
	return len(lines) - m.height
}

func (m Model) scrollDetail(delta int) (tea.Model, tea.Cmd) {
	m.detailOffset += delta
	if m.detailOffset < 0 {
		m.detailOffset = 0
	}
	if maxOffset := m.maxDetailOffset(); m.detailOffset > maxOffset {
		m.detailOffset = maxOffset
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode first so textinput gets priority
	if m.mode == modeSearch {
		switch msg.String() {
		case "esc":
			m.mode = modeBrowse
			m.query = ""
			m.input.SetValue("")
			m.input.Blur()
			m.filtered = m.scripts
			m.cursor = 0
			m.offset = 0
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil
		case "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
			return m, nil
		case "pgup":
			vis := m.visibleRows()
			m.cursor -= vis
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset = m.cursor
			return m, nil
		case "pgdown":
			vis := m.visibleRows()
			m.cursor += vis
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
			return m, nil
		case "enter":
			m.input.Blur()
			return m.enterDetail()
		}
		// All other keys go to textinput in search mode
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refilter()
		if m.searchMode == searchContent && m.query != "" && m.input.Value() != m.query {
			return m, contentSearchCmd(m.input.Value())
		}
		return m, cmd
	}

	if m.mode == modeDetail {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "r", "R", "q":
			return m.returnFromDetail()
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			scriptName := m.filtered[m.cursor].Name
			_ = copyToClipboard(scriptName)
			m.pendingExec = scriptName
			return m, tea.Quit
		case " ", "v":
			return m.openSourcePreview()
		case "up", "k":
			return m.scrollDetail(-1)
		case "down", "j":
			return m.scrollDetail(1)
		case "pgup":
			page := max(1, m.height-2)
			return m.scrollDetail(-page)
		case "pgdown":
			page := max(1, m.height-2)
			return m.scrollDetail(page)
		case "home", "g":
			m.detailOffset = 0
			return m, nil
		case "end", "G":
			m.detailOffset = m.maxDetailOffset()
			return m, nil
		}
		return m, nil
	}

	// Handle non-search modes
	switch msg.String() {

	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.mode == modeDetail {
			return m.returnFromDetail()
		}
		return m, nil

	case "/":
		if m.mode == modeBrowse {
			m.mode = modeSearch
			m.searchMode = searchNameDesc
			m.input.Focus()
			return m, textinput.Blink
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			vis := m.visibleRows()
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
		return m, nil

	case "pgup":
		vis := m.visibleRows()
		m.cursor -= vis
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.offset = m.cursor
		return m, nil

	case "pgdown":
		vis := m.visibleRows()
		m.cursor += vis
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor >= m.offset+vis {
			m.offset = m.cursor - vis + 1
		}
		return m, nil

	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		return m.enterDetail()

	case "r", "R", "q":
		if m.mode == modeDetail {
			return m.returnFromDetail()
		}
		// fall through to typing-starts-search in browse mode

	case " ":
		if m.mode == modeDetail {
			return m.openSourcePreview()
		}
		return m, nil

	case "v":
		return m.openSourcePreview()
	}

	// typing in browse mode starts search
	if m.mode == modeBrowse && len(msg.Runes) > 0 {
		m.mode = modeSearch
		m.searchMode = searchNameDesc
		m.input.Focus()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refilter()
		return m, cmd
	}

	return m, nil
}

func (m Model) openSourcePreview() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", m.filtered[m.cursor].Name)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return m, nil
	}

	pager := ""
	for _, p := range []string{"less", "most", "more"} {
		if path, err := exec.LookPath(p); err == nil {
			pager = path
			break
		}
	}

	batPath, batErr := exec.LookPath("bat")
	hasBat := batErr == nil

	var cmd *exec.Cmd
	switch {
	case hasBat && pager != "":
		cmd = exec.Command(batPath, "--style=numbers", "--color=always", "--paging=always", scriptPath)
		cmd.Env = append(os.Environ(), "BAT_PAGER="+pager)
	case hasBat:
		shellCmd := fmt.Sprintf("%s --style=numbers --color=always --paging=never %s; printf '\\n[press enter to return]'; read _",
			batPath, shellQuote(scriptPath))
		cmd = exec.Command("sh", "-c", shellCmd)
	case pager != "":
		cmd = exec.Command(pager, scriptPath)
	default:
		shellCmd := fmt.Sprintf("cat %s; printf '\\n[press enter to return]'; read _", shellQuote(scriptPath))
		cmd = exec.Command("sh", "-c", shellCmd)
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return nil })
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (m Model) visibleRows() int {
	// title(1) + tips(1) + subheader(1) + sep(1) = 4 fixed lines
	// search mode: searchrow(1) + sep(1) = 2 fixed lines
	headerLines := 4
	if m.mode == modeSearch {
		headerLines = 2
	}
	rows := m.height - headerLines - 1
	if rows < 1 {
		rows = 1
	}
	return rows
}

// bgFill pads content to full terminal width with bg color fill.
func bgFill(content string, width int, bg lipgloss.Color) string {
	visible := lipgloss.Width(content)
	trail := max(0, width-visible)
	fill := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", trail))
	return content + fill
}

// inBg applies a foreground style + bg background to a text segment.
func inBg(s lipgloss.Style, text string, bg lipgloss.Color) string {
	return s.Background(bg).Render(text)
}

// sep returns a full-width horizontal separator line.
func sep(width int, fg lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(fg).Render(strings.Repeat("─", width)) + "\n"
}

func writeWrappedText(sb *strings.Builder, text string, width, indent int, fg, bg lipgloss.Color, maxLines int) {
	if strings.TrimSpace(text) == "" {
		return
	}

	textWidth := width - indent - 2
	if textWidth < 1 {
		textWidth = 1
	}

	lines := wrapTextLines(text, textWidth)
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines-1], "...")
	}

	style := lipgloss.NewStyle().Foreground(fg)
	prefix := strings.Repeat(" ", max(0, indent))
	for _, line := range lines {
		sb.WriteString(bgFill(inBg(style, prefix+line, bg), width, bg) + "\n")
	}
}

func wrapTextLines(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")

	rawLines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		raw = strings.TrimRight(raw, " \t")
		if strings.TrimSpace(raw) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapTextLine(raw, width)...)
	}
	return lines
}

func wrapTextLine(line string, width int) []string {
	indent := leadingWhitespace(line)
	indent = fitIndent(indent, width)
	indentWidth := lipgloss.Width(indent)
	available := width - indentWidth
	if available < 1 {
		indent = ""
		indentWidth = 0
		available = width
	}

	words := strings.Fields(strings.TrimLeft(line, " "))
	if len(words) == 0 {
		return []string{indent}
	}

	var lines []string
	current := indent
	currentWidth := indentWidth
	for _, word := range words {
		wordWidth := lipgloss.Width(word)
		if currentWidth > indentWidth && currentWidth+1+wordWidth <= width {
			current += " " + word
			currentWidth += 1 + wordWidth
			continue
		}
		if currentWidth == indentWidth && indentWidth+wordWidth <= width {
			current += word
			currentWidth += wordWidth
			continue
		}
		if currentWidth > indentWidth {
			lines = append(lines, current)
			current = indent
			currentWidth = indentWidth
		}
		if wordWidth > available {
			chunks := splitLongWord(word, available)
			for i, chunk := range chunks {
				if i == len(chunks)-1 {
					current = indent + chunk
					currentWidth = indentWidth + lipgloss.Width(chunk)
					continue
				}
				lines = append(lines, indent+chunk)
			}
			continue
		}
		current = indent + word
		currentWidth = indentWidth + wordWidth
	}
	if currentWidth > indentWidth || len(lines) == 0 {
		lines = append(lines, current)
	}
	return lines
}

func leadingWhitespace(text string) string {
	for i, r := range text {
		if r != ' ' && r != '\t' {
			return text[:i]
		}
	}
	return text
}

func fitIndent(indent string, width int) string {
	if width <= 1 {
		return ""
	}
	for lipgloss.Width(indent) >= width {
		runes := []rune(indent)
		if len(runes) == 0 {
			return ""
		}
		indent = string(runes[:len(runes)-1])
	}
	return indent
}

func splitLongWord(word string, width int) []string {
	if width < 1 {
		width = 1
	}
	var chunks []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range word {
		part := string(r)
		partWidth := lipgloss.Width(part)
		if currentWidth > 0 && currentWidth+partWidth > width {
			chunks = append(chunks, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += partWidth
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	if len(chunks) == 0 {
		return []string{word}
	}
	return chunks
}

func singleLineText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func (m Model) detailLines() []string {
	if len(m.filtered) == 0 {
		return nil
	}
	s := m.filtered[m.cursor]
	w := m.width
	if w == 0 {
		w = 80
	}

	var sb strings.Builder

	bg := colorBgBase
	orange := lipgloss.NewStyle().Foreground(colorOrange)

	blank := func(width int) string { return bgFill("", width, bg) + "\n" }

	shortDesc := s.Desc
	detailedDesc := ""
	if d, ok := m.descriptions.Lookup(s.Name); ok {
		if d.ShortDesc != "" {
			shortDesc = d.ShortDesc
		}
		detailedDesc = d.DetailedDesc
	}

	// ── title band (full width) ───────────────────────────────────
	sb.WriteString(bgFill("", w, colorBgHeader) + "\n")
	sb.WriteString(sep(w, colorScriptName))

	// ── body: image left, details right ──────────────────────────
	imgOutput := ""
	if m.imageCache != nil {
		imgOutput = m.imageCache[s.Name]
	}

	// build details block (full width)
	var det strings.Builder
	blue := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	section := func(label string) {
		det.WriteString(bgFill(inBg(blue, "  "+label, bg), w, bg) + "\n")
	}

	det.WriteString(blank(w))
	section("Script")
	writeWrappedText(&det, s.Name, w, 4, colorAccent, bg, 0)
	det.WriteString(blank(w))
	section("Description")
	writeWrappedText(&det, shortDesc, w, 4, colorText, bg, 0)

	if detailedDesc != "" {
		det.WriteString(blank(w))
		det.WriteString(bgFill(inBg(orange, "  Details", bg), w, bg) + "\n")
		writeWrappedText(&det, detailedDesc, w, 4, colorText, bg, 0)
	}

	// details always full width
	sb.WriteString(det.String())

	// image below description
	if imgOutput != "" && w >= imagePreviewWidth+2 {
		sb.WriteString(blank(w))
		sb.WriteString(sep(w, colorMuted))
		// indent image slightly
		for _, line := range strings.Split(strings.TrimRight(imgOutput, "\n"), "\n") {
			sb.WriteString("  " + line + "\n")
		}
		sb.WriteString(blank(w))
	}

	// ── keybindings ───────────────────────────────────────────────
	sb.WriteString(sep(w, colorScriptName))
	sb.WriteString(blank(w))
	kb := func(k, label string) string {
		return k + " " + label
	}
	keysRow := strings.Join([]string{
		kb("↑/↓", "scroll"),
		kb("v", "view source"),
		kb("r", "back"),
		kb("↵", "execute"),
	}, "   ")
	writeWrappedText(&sb, keysRow, w, 2, colorMuted, bg, 0)
	sb.WriteString(blank(w))

	// warning below footer
	if m.warning != "" && time.Now().Before(m.warningExp) {
		warnStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
		sb.WriteString(bgFill(warnStyle.Render("  ⚠ "+m.warning), w, bg) + "\n")
	}

	return strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
}

func (m Model) viewDetail() string {
	lines := m.detailLines()
	if len(lines) == 0 {
		return ""
	}
	if m.height > 0 && len(lines) > m.height {
		maxOffset := len(lines) - m.height
		offset := m.detailOffset
		if offset < 0 {
			offset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		lines = lines[offset : offset+m.height]
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m Model) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	if m.mode == modeLoading {
		msg := m.spinner.View() + " " +
			lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("INITIALIZING SYS://4rji")
		if m.err != nil {
			msg = lipgloss.NewStyle().Foreground(colorScriptName).Render("ERR// ") + m.err.Error()
		}
		return bgFill(msg, w, colorBgBase) + "\n"
	}

	if m.mode == modeDetail {
		return m.viewDetail()
	}

	var sb strings.Builder

	// show warning if active and not expired
	if m.warning != "" && time.Now().Before(m.warningExp) {
		warnStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
		sb.WriteString(bgFill(warnStyle.Render("  ⚠ "+m.warning), w, colorBgBase) + "\n")
	}

	cyan := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	pink := lipgloss.NewStyle().Foreground(colorScriptName).Bold(true)
	muted := lipgloss.NewStyle().Foreground(colorMuted)
	white := lipgloss.NewStyle().Foreground(colorText)

	if m.mode == modeSearch {
		modeLabel := "NAME+DESC"
		if m.searchMode == searchContent {
			modeLabel = "CONTENT"
		}
		modeSt := inBg(pink, " ["+modeLabel+"] ", colorBgSubHdr)
		inputArea := inBg(cyan, "◈ ", colorBgSubHdr) + m.input.View()
		hitsStr := inBg(muted, fmt.Sprintf("  %d hits", len(m.filtered)), colorBgSubHdr)
		hintStr := inBg(muted, "  [ESC] cancel", colorBgSubHdr)
		sb.WriteString(bgFill(modeSt+inputArea+hitsStr+hintStr, w, colorBgSubHdr) + "\n")
		sb.WriteString(sep(w, colorScriptName))
	} else {
		// ── header ────────────────────────────────────────────────
		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(colorAccent).
			Bold(true)
		sb.WriteString(bgFill(headerStyle.Render("  ◈  SCRIPT SELECTOR  "), w, colorAccent) + "\n")

		// tip line
		tips := inBg(muted, "  ", colorBgBase) +
			inBg(muted, "[SPC/V]", colorBgBase) + inBg(white, " preview", colorBgBase) +
			inBg(muted, "  [↵]", colorBgBase) + inBg(white, " open", colorBgBase) +
			inBg(muted, "  [^C]", colorBgBase) + inBg(white, " exit", colorBgBase)
		sb.WriteString(bgFill(tips, w, colorBgBase) + "\n")

		// subheader band
		count := fmt.Sprintf("%d loaded", len(m.filtered))
		subHdr := inBg(cyan, " ◈ SCRIPTS ", colorBgSubHdr) +
			inBg(muted, " "+count, colorBgSubHdr)
		sb.WriteString(bgFill(subHdr, w, colorBgSubHdr) + "\n")
		sb.WriteString(sep(w, colorPurple))
	}

	// script list
	vis := m.visibleRows()
	nameW := 20
	for _, s := range m.filtered {
		if len(s.Name) > nameW {
			nameW = len(s.Name)
		}
	}
	if nameW > 32 {
		nameW = 32
	}

	end := m.offset + vis
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		s := m.filtered[i]
		name := s.Name
		desc := singleLineText(s.Desc)
		if d, ok := m.descriptions.Lookup(s.Name); ok && d.ShortDesc != "" {
			desc = singleLineText(d.ShortDesc)
		}

		maxDesc := w - nameW - 10
		if maxDesc < 0 {
			maxDesc = 0
		}
		descRunes := []rune(desc)
		if len(descRunes) > maxDesc && maxDesc > 3 {
			desc = string(descRunes[:maxDesc-1]) + "…"
		}

		namePad := strings.Repeat(" ", max(0, nameW-len([]rune(name))))

		if i == m.cursor {
			bg := colorBgSelected
			cursor := inBg(pink, "▶▶ ", bg)
			nameStr := inBg(lipgloss.NewStyle().Foreground(colorAccent).Bold(true), highlightMatch(name, m.query), bg)
			padStr := inBg(muted, namePad, bg)
			descStr := inBg(lipgloss.NewStyle().Foreground(colorText), "  "+desc, bg)
			sb.WriteString(bgFill(cursor+nameStr+padStr+descStr, w, bg) + "\n")
		} else {
			rowBg := colorBgRow
			if i%2 == 0 {
				rowBg = colorBgRowAlt
			}
			idx := inBg(muted, "   ", rowBg)
			nameStr := inBg(lipgloss.NewStyle().Foreground(colorText), highlightMatch(name, m.query), rowBg)
			padStr := inBg(muted, namePad, rowBg)
			descStr := inBg(lipgloss.NewStyle().Foreground(colorMuted), "  "+desc, rowBg)
			sb.WriteString(bgFill(idx+nameStr+padStr+descStr, w, rowBg) + "\n")
		}
	}

	return sb.String()
}

func highlightMatch(text, query string) string {
	if query == "" {
		return text
	}
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx < 0 || idx+len(q) > len(text) {
		return text
	}
	hl := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	return text[:idx] + hl.Render(text[idx:idx+len(q)]) + text[idx+len(q):]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
