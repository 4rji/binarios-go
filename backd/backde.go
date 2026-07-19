// Menu → elegir proceso → vista “actividad” (refresco configurable, 10s default).
// En actividad: [r]/[q]/[k]/[t] funcionan sin Enter; kill pide una confirmacion con Enter. Sin "golang.org/x/term".
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	stdnet "net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

var logger *log.Logger

type listCommandAction int

const (
	listCommandIgnore listCommandAction = iota
	listCommandRescan
	listCommandQuit
	listCommandSelect
	listCommandEditRefresh
	listCommandShowHelp
)

type activityCommandAction int

const (
	activityCommandNone activityCommandAction = iota
	activityCommandQuit
	activityCommandBack
	activityCommandKill
	activityCommandEditRefresh
	activityCommandHelp
)

const killConfirmSteps = 1

/* ANSI */
const (
	RED     = "\033[31m"
	YELLOW  = "\033[33m"
	BLUE    = "\033[34m"
	MAGENTA = "\033[35m"
	CYAN    = "\033[36m"
	WHITE   = "\033[97m"
	BOLD    = "\033[1m"
	DIM     = "\033[2m"
	RESET   = "\033[0m"

	MENU_PINK   = "\033[38;2;255;100;200m"
	MENU_VIOLET = "\033[38;2;200;120;255m"
	MENU_BLUE   = "\033[38;2;150;150;255m"
)

var EXCLUDED = []string{"firefox", "chrome", "google-chrome"}

type item struct {
	Idx    int
	PID    int32
	Name   string
	User   string
	Remote string
	Proto  string
	Src    string
}

func init() {
	if runningUnderGoTest() {
		logger = log.New(io.Discard, "", 0)
		return
	}
	if os.Geteuid() != 0 {
		code, err := rerunWithSudo(os.Args[1:])
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				fmt.Fprintf(os.Stderr, "no se pudo relanzar con sudo: %v\n", err)
			}
		}
		os.Exit(code)
	}
	f, err := os.OpenFile("backde.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger = log.New(os.Stdout, "", 0)
	} else {
		logger = log.New(f, "", 0)
	}
}

func main() {
	refreshSeconds := parseRefreshSeconds(os.Args[1:])
	for {
		pid, name, shouldQuit, refreshSeconds := listView(refreshSeconds)
		if shouldQuit {
			return
		}
		shouldQuit, refreshSeconds = activityView(pid, name, refreshSeconds)
		if shouldQuit {
			return
		}
	}
}

/* ---------- Listado una sola línea ---------- */

func listOnce() []item {
	conns, err := psnet.Connections("inet")
	if err != nil {
		logger.Printf("%sError: %v%s\n", RED, err, RESET)
		return nil
	}
	seen := map[int32]bool{}
	out := []item{}
	for _, c := range conns {
		isUDP := c.Type == syscall.SOCK_DGRAM
		if !isUDP {
			if c.Status != "ESTABLISHED" || c.Raddr.IP == "127.0.0.1" || c.Raddr.IP == "" {
				continue
			}
		}
		p, err := process.NewProcess(c.Pid)
		if err != nil {
			continue
		}
		name, _ := p.Name()
		if contains(EXCLUDED, strings.ToLower(name)) {
			continue
		}
		if seen[c.Pid] {
			continue
		}
		seen[c.Pid] = true
		user, _ := p.Username()
		exe, _ := p.Exe()
		remote := ""
		if isUDP {
			if c.Raddr.IP != "" {
				remote = formatAddr(c.Raddr.IP, c.Raddr.Port)
			} else {
				remote = formatAddr(c.Laddr.IP, c.Laddr.Port)
			}
		} else {
			remote = formatAddr(c.Raddr.IP, c.Raddr.Port)
		}
		proto := "TCP"
		if isUDP {
			proto = "UDP"
		}
		it := item{
			PID:    c.Pid,
			Name:   name,
			User:   user,
			Remote: remote,
			Proto:  proto,
			Src:    classifyExecSource(exe),
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].PID < out[j].PID
		}
		return out[i].Name < out[j].Name
	})
	for i := range out {
		out[i].Idx = i + 1
	}
	return out
}

func printList(items []item) {
	fmt.Printf("\n%s%sProcesses With Active Connections%s\n", BOLD, MAGENTA, RESET)
	for _, it := range items {
		fmt.Printf("%s%2d)%s %s%s%s %s[%s]%s (PID %d) %sEndpoint:%s %s%-21s%s %sSource:%s %s%-4s%s %sUser:%s %s\n",
			CYAN, it.Idx, RESET, BLUE, it.Name, RESET, DIM, it.Proto, RESET, it.PID,
			DIM, RESET, YELLOW, it.Remote, RESET, DIM, RESET, CYAN, it.Src, RESET, DIM, RESET, it.User)
	}
}

func listView(refreshSeconds int) (int32, string, bool, int) {
	if runningUnderGoTest() {
		return listViewLineMode(refreshSeconds)
	}
	return listViewRawMode(refreshSeconds)
}

func listViewLineMode(refreshSeconds int) (int32, string, bool, int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdCh := startInputReader(ctx)
	refreshLabel := formatRefreshLabel(refreshSeconds)
	scanCount := 0
	editingRefresh := false
	showingHelp := false
	statusMessage := ""

	renderPrompt := func(itemCount, remaining int) {
		countdown := ""
		if !editingRefresh && !showingHelp && refreshSeconds > 0 {
			countdown = fmt.Sprintf("  (next scan in %ds)", remaining)
		}

		label := "Selection"
		switch {
		case showingHelp:
			label = "Press Enter to return"
		case editingRefresh:
			label = "Refresh time in seconds (0 disables auto-refresh, Enter cancels)"
		case itemCount == 0:
			label = "Input"
		}

		fmt.Printf("\r\033[2K%s%s (q=quit):%s", label, countdown, RESET)
	}

scanLoop:
	for {
		items := listOnce()
		scanCount++
		now := time.Now().Format("2006-01-02 15:04:05")
		logger.Printf("LIST SCAN #%d @ %s (%s) items=%d\n", scanCount, now, runtime.GOOS, len(items))
		for _, it := range items {
			logger.Printf("  %d) %s [%s] PID=%d Src=%s User=%s Remote=%s\n",
				it.Idx, it.Name, it.Proto, it.PID, it.Src, it.User, it.Remote)
		}

		clear()
		fmt.Println(renderPanel("Main Menu", []string{
			"Choose a list number, or type a PID / process name to jump directly.",
			"[Enter] Refresh now   [t] Refresh time   [?] Help   [q] Quit",
			"Auto-refresh: " + refreshLabel,
		}))
		if showingHelp {
			fmt.Println(renderListHelp(refreshLabel))
		} else if len(items) == 0 {
			fmt.Println(YELLOW + "No active connections right now. Monitoring..." + RESET)
		} else {
			printList(items)
		}
		if statusMessage != "" {
			fmt.Println()
			fmt.Println(statusMessage)
		}
		renderPrompt(len(items), refreshSeconds)

		if !editingRefresh && !showingHelp && refreshSeconds > 0 {
			remaining := refreshSeconds
			ticker := time.NewTicker(1 * time.Second)
			for remaining > 0 {
				select {
				case cmd, ok := <-cmdCh:
					ticker.Stop()
					if !ok {
						return 0, "", true, refreshSeconds
					}
					if showingHelp {
						showingHelp = false
						statusMessage = ""
						continue scanLoop
					}
					if editingRefresh {
						refreshSeconds, statusMessage = parseRefreshLineInput(cmd, refreshSeconds)
						refreshLabel = formatRefreshLabel(refreshSeconds)
						editingRefresh = false
						continue scanLoop
					}

					action, pid, name := parseListCommand(cmd, items)
					switch action {
					case listCommandQuit:
						return 0, "", true, refreshSeconds
					case listCommandRescan:
						statusMessage = ""
						continue scanLoop
					case listCommandSelect:
						return pid, name, false, refreshSeconds
					case listCommandEditRefresh:
						editingRefresh = true
						statusMessage = CYAN + "Refresh edit: type seconds and press Enter." + RESET
						continue scanLoop
					case listCommandShowHelp:
						showingHelp = true
						statusMessage = ""
						continue scanLoop
					default:
						continue scanLoop
					}
				case <-ticker.C:
					remaining--
					if remaining > 0 {
						renderPrompt(len(items), remaining)
					}
				}
			}
			ticker.Stop()
			continue
		}

		cmd, ok := <-cmdCh
		if !ok {
			return 0, "", true, refreshSeconds
		}
		if showingHelp {
			showingHelp = false
			statusMessage = ""
			continue
		}
		if editingRefresh {
			refreshSeconds, statusMessage = parseRefreshLineInput(cmd, refreshSeconds)
			refreshLabel = formatRefreshLabel(refreshSeconds)
			editingRefresh = false
			continue
		}

		action, pid, name := parseListCommand(cmd, items)
		switch action {
		case listCommandQuit:
			return 0, "", true, refreshSeconds
		case listCommandRescan:
			statusMessage = ""
			continue
		case listCommandSelect:
			return pid, name, false, refreshSeconds
		case listCommandEditRefresh:
			editingRefresh = true
			statusMessage = CYAN + "Refresh edit: type seconds and press Enter." + RESET
			continue
		case listCommandShowHelp:
			showingHelp = true
			statusMessage = ""
			continue
		default:
			continue
		}
	}
}

func listViewRawMode(refreshSeconds int) (int32, string, bool, int) {
	keyboard, err := newKeypressSession(os.Stdin)
	if err != nil {
		logger.Printf("LIST RAW MODE unavailable, fallback to line mode: %v\n", err)
		return listViewLineMode(refreshSeconds)
	}
	defer keyboard.Close()

	refreshLabel := formatRefreshLabel(refreshSeconds)
	scanCount := 0
	inputBuf := ""
	refreshInput := ""
	statusMessage := ""
	editingRefresh := false
	showingHelp := false

	renderPrompt := func(itemCount, remaining int) {
		countdown := ""
		if !editingRefresh && !showingHelp && refreshSeconds > 0 {
			countdown = fmt.Sprintf("  (next scan in %ds)", remaining)
		}

		label := "Selection"
		input := inputBuf
		switch {
		case showingHelp:
			label = "Press any key to return"
			input = ""
		case editingRefresh:
			label = "Refresh time in seconds (0 disables auto-refresh, Esc cancels)"
			input = refreshInput
		case itemCount == 0:
			label = "Input"
		}

		fmt.Printf("\r\033[2K%s%s (q=quit): %s%s%s", label, countdown, CYAN, input, RESET)
	}

scanLoop:
	for {
		items := listOnce()
		scanCount++
		now := time.Now().Format("2006-01-02 15:04:05")
		logger.Printf("LIST SCAN #%d @ %s (%s) items=%d\n", scanCount, now, runtime.GOOS, len(items))
		for _, it := range items {
			logger.Printf("  %d) %s [%s] PID=%d Src=%s User=%s Remote=%s\n",
				it.Idx, it.Name, it.Proto, it.PID, it.Src, it.User, it.Remote)
		}

		clear()
		fmt.Println(renderPanel("Main Menu", []string{
			"Choose a list number, or type a PID / process name to jump directly.",
			"[Enter] Refresh now   [t] Refresh time   [?] Help   [q] Quit",
			"Auto-refresh: " + refreshLabel,
		}))
		if showingHelp {
			fmt.Println(renderListHelp(refreshLabel))
		} else if len(items) == 0 {
			fmt.Println(YELLOW + "No active connections right now. Monitoring..." + RESET)
		} else {
			printList(items)
		}
		if statusMessage != "" {
			fmt.Println()
			fmt.Println(statusMessage)
		}

		remaining := refreshSeconds
		renderPrompt(len(items), remaining)

		for {
			wait := time.Duration(-1)
			if !editingRefresh && !showingHelp && refreshSeconds > 0 {
				if remaining == 0 {
					continue scanLoop
				}
				wait = time.Second
			}

			key, ok, err := keyboard.ReadKey(wait)
			if err != nil {
				if err == io.EOF {
					return 0, "", true, refreshSeconds
				}
				logger.Printf("LIST RAW MODE read error, fallback to line mode: %v\n", err)
				return listViewLineMode(refreshSeconds)
			}
			if !ok {
				if !editingRefresh && !showingHelp && refreshSeconds > 0 {
					remaining--
					renderPrompt(len(items), remaining)
				}
				continue
			}

			if showingHelp {
				showingHelp = false
				statusMessage = ""
				continue scanLoop
			}

			if editingRefresh {
				editResult := applyRefreshEditKey(key, refreshInput)
				refreshInput = editResult.buffer
				if editResult.cancel {
					editingRefresh = false
					refreshInput = ""
					statusMessage = YELLOW + "Refresh change canceled." + RESET
					continue scanLoop
				}
				if editResult.done {
					editingRefresh = false
					if !editResult.valid {
						refreshInput = ""
						statusMessage = RED + "Invalid refresh value. Enter a whole number greater than or equal to 0." + RESET
						continue scanLoop
					}
					refreshSeconds = editResult.value
					refreshLabel = formatRefreshLabel(refreshSeconds)
					refreshInput = ""
					statusMessage = CYAN + "Refresh updated to " + refreshLabel + "." + RESET
					continue scanLoop
				}
				renderPrompt(len(items), remaining)
				continue
			}

			if inputBuf == "" {
				switch key {
				case 'q', 'Q':
					fmt.Println()
					return 0, "", true, refreshSeconds
				case 'r', 'R':
					statusMessage = ""
					continue scanLoop
				case 't', 'T':
					editingRefresh = true
					refreshInput = ""
					statusMessage = CYAN + "Refresh edit: type seconds and press Enter." + RESET
					continue scanLoop
				case '?':
					showingHelp = true
					statusMessage = ""
					continue scanLoop
				}
			}

			result := applyListKey(key, inputBuf, items)
			inputBuf = result.buffer

			if result.submit {
				switch result.action {
				case listCommandQuit:
					fmt.Println()
					return 0, "", true, refreshSeconds
				case listCommandRescan:
					inputBuf = ""
					statusMessage = ""
					continue scanLoop
				case listCommandSelect:
					fmt.Println()
					return result.pid, result.name, false, refreshSeconds
				case listCommandEditRefresh:
					inputBuf = ""
					editingRefresh = true
					refreshInput = ""
					statusMessage = CYAN + "Refresh edit: type seconds and press Enter." + RESET
					continue scanLoop
				case listCommandShowHelp:
					inputBuf = ""
					showingHelp = true
					statusMessage = ""
					continue scanLoop
				default:
					renderPrompt(len(items), remaining)
					continue
				}
			}

			renderPrompt(len(items), remaining)
		}
	}
}

/* ---------- Selección ---------- */

func prompt(msg string) string {
	fmt.Print(msg)
	r := bufio.NewReader(os.Stdin)
	text, _ := r.ReadString('\n')
	return strings.TrimSpace(text)
}

var digitsOnly = regexp.MustCompile(`^\d+$`)

func resolveChoice(choice string, items []item) (int32, string) {
	// índice o PID
	if digitsOnly.MatchString(choice) {
		n, _ := strconv.Atoi(choice)
		for _, it := range items {
			if it.Idx == n {
				return it.PID, ""
			}
		}
		// PID directo
		pid, _ := strconv.Atoi(choice)
		if pid > 0 {
			return int32(pid), ""
		}
	}
	// nombre parcial o exacto
	if choice != "" {
		return 0, choice
	}
	return 0, ""
}

func parseListCommand(cmd string, items []item) (listCommandAction, int32, string) {
	cmd = strings.TrimSpace(cmd)
	switch strings.ToLower(cmd) {
	case "":
		return listCommandRescan, 0, ""
	case "q":
		return listCommandQuit, 0, ""
	case "r":
		return listCommandRescan, 0, ""
	case "t":
		return listCommandEditRefresh, 0, ""
	case "?":
		return listCommandShowHelp, 0, ""
	}

	pid, name := resolveChoice(cmd, items)
	if pid == 0 && name == "" {
		return listCommandIgnore, 0, ""
	}
	return listCommandSelect, pid, name
}

type activityCommandResult struct {
	action          activityCommandAction
	nextKillConfirm int
	message         string
}

type listKeyResult struct {
	buffer string
	submit bool
	action listCommandAction
	pid    int32
	name   string
}

type refreshEditResult struct {
	buffer string
	done   bool
	cancel bool
	value  int
	valid  bool
}

func parseActivityKey(key byte, killConfirmPending int, hasTarget bool) activityCommandResult {
	switch key {
	case 'q', 'Q':
		return activityCommandResult{action: activityCommandQuit}
	case 'r', 'R':
		return activityCommandResult{action: activityCommandBack}
	case 't', 'T':
		return activityCommandResult{action: activityCommandEditRefresh}
	case '?':
		return activityCommandResult{action: activityCommandHelp}
	case 'k', 'K':
		if !hasTarget {
			return activityCommandResult{message: RED + "No active process is selected." + RESET}
		}
		return activityCommandResult{
			nextKillConfirm: killConfirmSteps,
			message:         YELLOW + killConfirmPrompt(killConfirmSteps) + RESET,
		}
	case '\r', '\n':
		if killConfirmPending == 0 {
			return activityCommandResult{action: activityCommandBack}
		}
		if !hasTarget {
			return activityCommandResult{message: RED + "Kill canceled: the process is no longer active." + RESET}
		}
		remaining := killConfirmPending - 1
		if remaining == 0 {
			return activityCommandResult{action: activityCommandKill}
		}
		return activityCommandResult{
			nextKillConfirm: remaining,
			message:         YELLOW + killConfirmPrompt(remaining) + RESET,
		}
	default:
		if killConfirmPending > 0 {
			return activityCommandResult{message: YELLOW + "Kill canceled." + RESET}
		}
		return activityCommandResult{}
	}
}

func applyRefreshEditKey(key byte, buffer string) refreshEditResult {
	switch key {
	case 27:
		return refreshEditResult{cancel: true}
	case '\r', '\n':
		value := strings.TrimSpace(buffer)
		if value == "" {
			return refreshEditResult{done: true}
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return refreshEditResult{done: true}
		}
		return refreshEditResult{done: true, valid: true, value: n}
	case 127, '\b':
		if len(buffer) == 0 {
			return refreshEditResult{buffer: buffer}
		}
		return refreshEditResult{buffer: buffer[:len(buffer)-1]}
	default:
		if key >= '0' && key <= '9' {
			return refreshEditResult{buffer: buffer + string(key)}
		}
		return refreshEditResult{buffer: buffer}
	}
}

func applyListKey(key byte, buffer string, items []item) listKeyResult {
	switch key {
	case '\r', '\n':
		action, pid, name := parseListCommand(buffer, items)
		return listKeyResult{
			submit: true,
			action: action,
			pid:    pid,
			name:   name,
		}
	case 127, '\b':
		if len(buffer) == 0 {
			return listKeyResult{buffer: buffer}
		}
		return listKeyResult{buffer: buffer[:len(buffer)-1]}
	default:
		if key >= 32 && key <= 126 {
			return listKeyResult{buffer: buffer + string(key)}
		}
		return listKeyResult{buffer: buffer}
	}
}

func killConfirmPrompt(remaining int) string {
	switch remaining {
	case 1:
		return "Kill armed: press Enter to confirm."
	default:
		return ""
	}
}

func formatRefreshLabel(refreshSeconds int) string {
	if refreshSeconds == 0 {
		return "no auto-refresh"
	}
	return fmt.Sprintf("%ds", refreshSeconds)
}

func parseRefreshLineInput(input string, current int) (int, string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return current, YELLOW + "Refresh change canceled." + RESET
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return current, RED + "Invalid refresh value. Enter a whole number greater than or equal to 0." + RESET
	}
	return n, CYAN + "Refresh updated to " + formatRefreshLabel(n) + "." + RESET
}

func renderListHelp(refreshLabel string) string {
	var b strings.Builder
	b.WriteString(BOLD)
	b.WriteString(CYAN)
	b.WriteString("Main Menu Help")
	b.WriteString(RESET)
	b.WriteString("\n")
	b.WriteString(DIM)
	b.WriteString("A compact live view of processes that currently hold active network connections.")
	b.WriteString(RESET)
	b.WriteString("\n\n")
	b.WriteString("  • Pick a list number to inspect a process in detail.\n")
	b.WriteString("  • Enter a PID to jump directly to that process.\n")
	b.WriteString("  • Enter part of a process name to match it quickly.\n")
	b.WriteString("  • Press Enter to rescan immediately.\n")
	b.WriteString("  • Press t to change the refresh interval.\n")
	b.WriteString("  • Press q to quit.\n")
	b.WriteString("\n")
	b.WriteString("Refresh:\n")
	b.WriteString("  • Current setting: " + refreshLabel + "\n")
	b.WriteString("  • Use 0 to disable auto-refresh.\n")
	b.WriteString("\n")
	b.WriteString(DIM)
	b.WriteString("Press any key to return.")
	b.WriteString(RESET)
	return b.String()
}

func renderActivityHelp(target string, refreshLabel string) string {
	var b strings.Builder
	b.WriteString(BOLD)
	b.WriteString(CYAN)
	b.WriteString("Activity View Help")
	b.WriteString(RESET)
	b.WriteString("\n")
	b.WriteString(DIM)
	b.WriteString("A focused live summary for ")
	b.WriteString(target)
	b.WriteString(": identity, resources, I/O, and remote peers.")
	b.WriteString(RESET)
	b.WriteString("\n\n")
	b.WriteString("  • Press Enter or r to go back to the main menu.\n")
	b.WriteString("  • Press k to arm a kill, then Enter to confirm.\n")
	b.WriteString("  • Press t to change the refresh interval.\n")
	b.WriteString("  • Press q to quit the program.\n")
	b.WriteString("\n")
	b.WriteString("Refresh:\n")
	b.WriteString("  • Current setting: " + refreshLabel + "\n")
	b.WriteString("  • Use 0 to disable auto-refresh.\n")
	b.WriteString("\n")
	b.WriteString(DIM)
	b.WriteString("Press any key to return.")
	b.WriteString(RESET)
	return b.String()
}

func classifyExecSource(exe string) string {
	clean := strings.ToLower(strings.TrimSpace(filepath.Clean(exe)))
	if clean == "" || clean == "." {
		return "unk"
	}

	switch {
	case hasPathPrefix(clean, "/tmp"),
		hasPathPrefix(clean, "/var/tmp"),
		hasPathPrefix(clean, "/private/tmp"),
		hasPathPrefix(clean, "/dev/shm"):
		return "tmp"
	case hasPathPrefix(clean, "/users"),
		hasPathPrefix(clean, "/home"),
		hasPathPrefix(clean, "/root"):
		return "home"
	case hasPathPrefix(clean, "/usr/local"),
		hasPathPrefix(clean, "/opt"),
		hasPathPrefix(clean, "/snap"),
		hasPathPrefix(clean, "/applications"):
		return "usr"
	case hasPathPrefix(clean, "/bin"),
		hasPathPrefix(clean, "/sbin"),
		hasPathPrefix(clean, "/usr/bin"),
		hasPathPrefix(clean, "/usr/sbin"),
		hasPathPrefix(clean, "/usr/libexec"),
		hasPathPrefix(clean, "/system"):
		return "sys"
	default:
		return "unk"
	}
}

func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func humanBytes(b uint64) string {
	const u = "KMGTPE"
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(1024), 0
	for n := b / 1024; n >= 1024 && exp < len(u)-1; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), u[exp])
}

type activitySnapshot struct {
	Name           string
	PID            int32
	User           string
	Status         string
	ParentPID      int32
	ParentName     string
	Uptime         time.Duration
	Nice           int32
	Exec           string
	Cmdline        string
	CPU            float64
	MemPercent     float32
	RSS            string
	Threads        int32
	FDs            int32
	CtxSwitchVol   int64
	CtxSwitchInvol int64
	HasIO          bool
	HasIORate      bool
	IOReadRate     string
	IOWriteRate    string
	IOReadTotal    string
	IOWriteTotal   string
	TCPEstablished int
	UDPCount       int
	Remotes        []string
}

type activityField struct {
	Label string
	Value string
}

func renderActivitySnapshot(snapshot activitySnapshot) string {
	const labelWidth = 16
	const valueWidth = 60

	var sections []string
	processFields := []activityField{
		{Label: "Name", Value: snapshot.Name},
		{Label: "PID", Value: fmt.Sprintf("%d", snapshot.PID)},
		{Label: "User", Value: blankIfEmpty(snapshot.User)},
		{Label: "State", Value: blankIfEmpty(snapshot.Status)},
		{Label: "Parent", Value: formatParent(snapshot.ParentPID, snapshot.ParentName)},
		{Label: "Uptime", Value: snapshot.Uptime.String()},
		{Label: "Nice", Value: fmt.Sprintf("%d", snapshot.Nice)},
	}
	sections = append(sections, renderActivitySection("Process", processFields, labelWidth, valueWidth))

	execFields := []activityField{}
	if snapshot.Exec != "" {
		execFields = append(execFields, activityField{Label: "Executable", Value: snapshot.Exec})
	}
	if snapshot.Cmdline != "" {
		execFields = append(execFields, activityField{Label: "Command", Value: snapshot.Cmdline})
	}
	if len(execFields) > 0 {
		sections = append(sections, renderActivitySection("Execution", execFields, labelWidth, valueWidth))
	}

	resourceFields := []activityField{
		{Label: "CPU", Value: fmt.Sprintf("%.1f%%", snapshot.CPU)},
		{Label: "Memory", Value: fmt.Sprintf("%.1f%%", snapshot.MemPercent)},
		{Label: "RSS", Value: blankIfEmpty(snapshot.RSS)},
		{Label: "Threads", Value: fmt.Sprintf("%d", snapshot.Threads)},
		{Label: "FDs", Value: fmt.Sprintf("%d", snapshot.FDs)},
		{Label: "CtxSwitch", Value: fmt.Sprintf("vol=%d | invol=%d", snapshot.CtxSwitchVol, snapshot.CtxSwitchInvol)},
	}
	sections = append(sections, renderActivitySection("Resources", resourceFields, labelWidth, valueWidth))

	if snapshot.HasIO {
		ioFields := []activityField{
			{Label: "Total Read", Value: snapshot.IOReadTotal},
			{Label: "Total Write", Value: snapshot.IOWriteTotal},
		}
		if snapshot.HasIORate {
			ioFields = append(ioFields,
				activityField{Label: "Read/s", Value: snapshot.IOReadRate + "/s"},
				activityField{Label: "Write/s", Value: snapshot.IOWriteRate + "/s"},
			)
		}
		sections = append(sections, renderActivitySection("IO", ioFields, labelWidth, valueWidth))
	}

	networkFields := []activityField{
		{Label: "TCP Established", Value: fmt.Sprintf("%d", snapshot.TCPEstablished)},
		{Label: "UDP Sockets", Value: fmt.Sprintf("%d", snapshot.UDPCount)},
	}
	networkSection := renderActivitySection("Network", networkFields, labelWidth, valueWidth)
	if len(snapshot.Remotes) > 0 {
		networkSection += "\n" + renderActivityList("Remote Peers", snapshot.Remotes, labelWidth, valueWidth)
	}
	sections = append(sections, networkSection)

	return strings.Join(sections, "\n\n")
}

func renderActivitySection(title string, fields []activityField, labelWidth, valueWidth int) string {
	var b strings.Builder
	b.WriteString(BOLD)
	b.WriteString(CYAN)
	b.WriteString(title)
	b.WriteString(RESET)
	b.WriteByte('\n')
	for _, field := range fields {
		if strings.TrimSpace(field.Value) == "" {
			continue
		}
		writeWrappedField(&b, field.Label, field.Value, labelWidth, valueWidth)
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderPanel(title string, lines []string) string {
	const contentWidth = 74

	title = fitRunes(title, contentWidth-4)
	var b strings.Builder
	titleWidth := len([]rune(title))
	topFill := strings.Repeat("═", maxInt(0, contentWidth-titleWidth-1))
	fmt.Fprintf(&b, "%s╔═ %s %s╗%s\n", MENU_VIOLET, gradientText(title), topFill, RESET)
	for _, line := range lines {
		wrapped := wrapText(line, contentWidth)
		if len(wrapped) == 0 {
			fmt.Fprintf(&b, "%s║ %-*s ║%s\n", MENU_VIOLET, contentWidth, "", RESET)
			continue
		}
		for _, part := range wrapped {
			padding := strings.Repeat(" ", maxInt(0, contentWidth-len([]rune(part))))
			fmt.Fprintf(&b, "%s║ %s%s%s%s ║%s\n", MENU_VIOLET, stylePanelLine(part), MENU_BLUE, padding, MENU_VIOLET, RESET)
		}
	}
	fmt.Fprintf(&b, "%s╚%s╝%s", MENU_VIOLET, strings.Repeat("═", contentWidth+2), RESET)
	return b.String()
}

func renderActivityList(label string, items []string, labelWidth, valueWidth int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %-*s\n", labelWidth, label+":")
	for _, item := range items {
		lines := wrapText(item, valueWidth-4)
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "    - %s\n", lines[0])
		for _, line := range lines[1:] {
			fmt.Fprintf(&b, "      %s\n", line)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeWrappedField(b *strings.Builder, label, value string, labelWidth, valueWidth int) {
	lines := wrapText(value, valueWidth)
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "  %-*s %s\n", labelWidth, label+":", lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(b, "  %-*s %s\n", labelWidth, "", line)
	}
}

func wrapText(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := ""
	for _, word := range words {
		parts := splitLongWord(word, width)
		for i, part := range parts {
			if current == "" {
				current = part
				continue
			}
			if i > 0 || len([]rune(current))+1+len([]rune(part)) > width {
				lines = append(lines, current)
				current = part
				continue
			}
			current += " " + part
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitLongWord(word string, width int) []string {
	if width <= 0 || len([]rune(word)) <= width {
		return []string{word}
	}
	runes := []rune(word)
	parts := make([]string, 0, (len(runes)/width)+1)
	for len(runes) > width {
		parts = append(parts, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func gradientText(text string) string {
	colors := []string{MENU_PINK, MENU_VIOLET, MENU_BLUE}
	var b strings.Builder
	for i, r := range text {
		b.WriteString(colors[i%len(colors)])
		b.WriteRune(r)
	}
	b.WriteString(RESET)
	return b.String()
}

func stylePanelLine(line string) string {
	var b strings.Builder
	b.WriteString(MENU_BLUE)
	for i := 0; i < len(line); {
		switch {
		case line[i] == '[':
			end := strings.IndexByte(line[i:], ']')
			if end >= 0 {
				b.WriteString(MENU_PINK)
				b.WriteString(BOLD)
				b.WriteString(line[i : i+end+1])
				b.WriteString(RESET)
				b.WriteString(MENU_BLUE)
				i += end + 1
				continue
			}
		case strings.HasPrefix(line[i:], "Auto-refresh:"):
			b.WriteString(MENU_VIOLET)
			b.WriteString("Auto-refresh:")
			b.WriteString(RESET)
			b.WriteString(MENU_BLUE)
			i += len("Auto-refresh:")
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	b.WriteString(RESET)
	return b.String()
}

func fitRunes(text string, max int) string {
	runes := []rune(text)
	if max <= 0 || len(runes) <= max {
		return text
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func formatProcessStatus(status []string) string {
	if len(status) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(status))
	for _, item := range status {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts = append(parts, item)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func formatParent(pid int32, name string) string {
	if name == "" {
		return fmt.Sprintf("PID %d", pid)
	}
	return fmt.Sprintf("%s (PID %d)", name, pid)
}

func blankIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func parentInfo(p *process.Process) (int32, string) {
	ppid, _ := p.Ppid()
	if ppid > 0 {
		if par, err := process.NewProcess(ppid); err == nil {
			nm, _ := par.Name()
			return ppid, nm
		}
	}
	return ppid, ""
}

/* ---------- Vista de actividad (teclas inmediatas) ---------- */
func activityView(pid int32, pname string, refreshSeconds int) (bool, int) {
	var byPID bool
	var wantPID int32
	var wantName string
	if pid > 0 {
		byPID = true
		wantPID = pid
	} else {
		byPID = false
		wantName = strings.ToLower(strings.TrimSpace(pname))
	}

	keyboard, err := newKeypressSession(os.Stdin)
	if err != nil {
		clear()
		fmt.Println(RED + "Immediate key mode could not be enabled: " + err.Error() + RESET)
		time.Sleep(2 * time.Second)
		return false, refreshSeconds
	}
	defer keyboard.Close()

	clear()
	header := fmt.Sprintf("Activity for %s", filterDesc(byPID, wantPID, wantName))
	refreshLabel := formatRefreshLabel(refreshSeconds)
	scanCount := 0

	var lastRead, lastWrite uint64
	var haveLast bool
	killConfirmPending := 0
	statusMessage := ""
	editingRefresh := false
	refreshInput := ""
	showingHelp := false

	for {
		scanCount++
		now := time.Now().Format("2006-01-02 15:04:05")
		logger.Printf("ACTIVITY SCAN #%d @ %s (%s) target=%s\n",
			scanCount, now, runtime.GOOS, filterDesc(byPID, wantPID, wantName))
		clear()
		target := pickTargetProcess(byPID, wantPID, wantName)
		var targetPID int32
		targetName := filterDesc(byPID, wantPID, wantName)
		fmt.Println(renderPanel(header, []string{
			"[Enter] Back   [r] Back   [k] Arm kill   [t] Refresh time   [?] Help   [q] Quit",
			"Auto-refresh: " + refreshLabel,
		}))
		if target == nil {
			fmt.Printf("%s%s%s\n", MAGENTA, strings.Repeat("═", 80), RESET)
			screenNow := time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("%sSCAN @ %s (%s)%s\n\n", CYAN, screenNow, runtime.GOOS, RESET)
			if showingHelp {
				fmt.Println(renderActivityHelp(targetName, refreshLabel))
			} else {
				fmt.Println(RED + "(no current match)" + RESET)
				logger.Println("  (no current match)")
			}
		} else {
			targetPID = target.Pid
			name, _ := target.Name()
			targetName = name
			fmt.Printf("%s%s%s\n", MAGENTA, strings.Repeat("═", 80), RESET)
			screenNow := time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("%sSCAN @ %s (%s)%s\n\n", CYAN, screenNow, runtime.GOOS, RESET)
			user, _ := target.Username()
			status, _ := target.Status()
			statusText := formatProcessStatus(status)
			cmdline, _ := target.Cmdline()
			cpu, _ := target.CPUPercent()
			memPerc, _ := target.MemoryPercent()
			memInfo, _ := target.MemoryInfo()
			thr, _ := target.NumThreads()
			fdc, _ := target.NumFDs()
			nice, _ := target.Nice()
			ctxsw, _ := target.NumCtxSwitches()
			ctxswVol, ctxswInvol := int64(0), int64(0)
			if ctxsw != nil {
				ctxswVol, ctxswInvol = ctxsw.Voluntary, ctxsw.Involuntary
			}
			ctime, _ := target.CreateTime()
			up := time.Since(time.UnixMilli(ctime)).Truncate(time.Second)

			ppid, pname := parentInfo(target)
			exe, _ := target.Exe()

			logger.Printf("  Process=%s PID=%d User=%s Status=%s\n", name, target.Pid, user, statusText)
			logger.Printf("  ParentPID=%d ParentName=%s Uptime=%v Nice=%d\n", ppid, pname, up, nice)
			if exe != "" {
				logger.Printf("  Exec=%s\n", exe)
			}
			if cmdline != "" {
				logger.Printf("  Cmd=%s\n", cmdline)
			}
			if memInfo != nil {
				logger.Printf("  CPU=%.1f%% MEM=%.1f%% RSS=%s Threads=%d FDs=%d CtxSw=v:%d nv:%d\n",
					cpu, memPerc, humanBytes(uint64(memInfo.RSS)), thr, fdc, ctxswVol, ctxswInvol)
			} else {
				logger.Printf("  CPU=%.1f%% MEM=%.1f%% Threads=%d FDs=%d CtxSw=v:%d nv:%d\n",
					cpu, memPerc, thr, fdc, ctxswVol, ctxswInvol)
			}

			snapshot := activitySnapshot{
				Name:           name,
				PID:            target.Pid,
				User:           user,
				Status:         statusText,
				ParentPID:      ppid,
				ParentName:     pname,
				Uptime:         up,
				Nice:           nice,
				Exec:           exe,
				Cmdline:        cmdline,
				CPU:            cpu,
				MemPercent:     memPerc,
				Threads:        thr,
				FDs:            fdc,
				CtxSwitchVol:   ctxswVol,
				CtxSwitchInvol: ctxswInvol,
			}
			if memInfo != nil {
				snapshot.RSS = humanBytes(uint64(memInfo.RSS))
			}

			// IO total (acumulado y tasa/intervalo)
			if ioctrs, err := target.IOCounters(); err == nil && ioctrs != nil {
				snapshot.HasIO = true
				snapshot.IOReadTotal = humanBytes(ioctrs.ReadBytes)
				snapshot.IOWriteTotal = humanBytes(ioctrs.WriteBytes)
				if haveLast && refreshSeconds > 0 {
					interval := uint64(refreshSeconds)
					dr := (ioctrs.ReadBytes - lastRead) / interval
					dw := (ioctrs.WriteBytes - lastWrite) / interval
					logger.Printf("  IO R/s=%s W/s=%s TotalR=%s TotalW=%s\n",
						humanBytes(dr), humanBytes(dw),
						humanBytes(ioctrs.ReadBytes), humanBytes(ioctrs.WriteBytes))
					snapshot.HasIORate = true
					snapshot.IOReadRate = humanBytes(dr)
					snapshot.IOWriteRate = humanBytes(dw)
				} else {
					logger.Printf("  IO R=%s W=%s\n",
						humanBytes(ioctrs.ReadBytes), humanBytes(ioctrs.WriteBytes))
				}
				lastRead, lastWrite = ioctrs.ReadBytes, ioctrs.WriteBytes
				haveLast = true
			}

			// Conexiones y RDNS
			pconns, _ := target.Connections()
			est := 0
			udp := 0
			rem := make([]string, 0, 5)
			for _, pc := range pconns {
				if pc.Type == syscall.SOCK_DGRAM {
					udp++
					if len(rem) < 5 {
						endp := ""
						if pc.Raddr.IP != "" {
							endp = formatAddr(pc.Raddr.IP, pc.Raddr.Port)
						} else {
							endp = formatAddr(pc.Laddr.IP, pc.Laddr.Port)
						}
						rem = append(rem, "udp "+endp)
					}
					continue
				}
				if pc.Status == "ESTABLISHED" && pc.Raddr.IP != "" {
					est++
					if len(rem) < 5 {
						host := pc.Raddr.IP
						if h, err := stdnet.LookupAddr(pc.Raddr.IP); err == nil && len(h) > 0 {
							host = h[0]
						}
						rem = append(rem, fmt.Sprintf("tcp %s (%s)", host, formatAddr(pc.Raddr.IP, pc.Raddr.Port)))
					}
				}
			}
			logger.Printf("  TCP_ESTABLISHED=%d UDP=%d Remotes=%s\n", est, udp, strings.Join(rem, " | "))
			snapshot.TCPEstablished = est
			snapshot.UDPCount = udp
			snapshot.Remotes = rem

			if showingHelp {
				fmt.Println(renderActivityHelp(targetName, refreshLabel))
			} else {
				fmt.Println(renderActivitySnapshot(snapshot))
			}
		}

		if statusMessage != "" {
			fmt.Println()
			fmt.Println(statusMessage)
		}
		if editingRefresh {
			fmt.Println()
			fmt.Printf("%sNew refresh%s in seconds %s(0 disables auto-refresh, Esc cancels)%s: %s%s%s\n",
				WHITE, RESET, DIM, RESET, CYAN, refreshInput, RESET)
		}

		wait := time.Duration(-1)
		if !editingRefresh && !showingHelp && refreshSeconds > 0 {
			wait = time.Duration(refreshSeconds) * time.Second
		}

		key, ok, err := keyboard.ReadKey(wait)
		if err != nil {
			if err == io.EOF {
				return true, refreshSeconds
			}
			statusMessage = RED + "Keyboard read error: " + err.Error() + RESET
			time.Sleep(1 * time.Second)
			continue
		}
		if !ok {
			continue
		}

		if showingHelp {
			showingHelp = false
			statusMessage = ""
			continue
		}

		if editingRefresh {
			editResult := applyRefreshEditKey(key, refreshInput)
			refreshInput = editResult.buffer
			if editResult.cancel {
				editingRefresh = false
				refreshInput = ""
				statusMessage = YELLOW + "Refresh change canceled." + RESET
			} else if editResult.done {
				editingRefresh = false
				refreshInput = ""
				if !editResult.valid {
					statusMessage = RED + "Invalid refresh value. Enter a whole number greater than or equal to 0." + RESET
				} else {
					refreshSeconds = editResult.value
					refreshLabel = formatRefreshLabel(refreshSeconds)
					haveLast = false
					statusMessage = CYAN + "Refresh updated to " + refreshLabel + "." + RESET
				}
			}
			continue
		}

		result := parseActivityKey(key, killConfirmPending, targetPID > 0)
		killConfirmPending = result.nextKillConfirm
		if result.message != "" {
			statusMessage = result.message
		}

		switch result.action {
		case activityCommandQuit:
			return true, refreshSeconds
		case activityCommandBack:
			return false, refreshSeconds
		case activityCommandKill:
			statusMessage = killTargetProcess(targetPID, targetName)
			killConfirmPending = 0
			time.Sleep(1 * time.Second)
		case activityCommandEditRefresh:
			killConfirmPending = 0
			editingRefresh = true
			refreshInput = ""
			statusMessage = CYAN + "Refresh edit: type seconds and press Enter." + RESET
		case activityCommandHelp:
			killConfirmPending = 0
			showingHelp = true
			statusMessage = ""
		}
	}
}

/* Leer una línea con timeout, requiere Enter */
func readLineWithTimeout(r *bufio.Reader, d time.Duration) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	ch := make(chan string, 1)
	go func() {
		s, _ := r.ReadString('\n')
		ch <- s
	}()
	select {
	case s := <-ch:
		return s, true
	case <-ctx.Done():
		return "", false
	}
}

func pickTargetProcess(byPID bool, wantPID int32, wantName string) *process.Process {
	if byPID {
		if p, err := process.NewProcess(wantPID); err == nil {
			return p
		}
		return nil
	}
	plist, err := process.Processes()
	if err != nil {
		return nil
	}
	for _, p := range plist {
		nm, _ := p.Name()
		if strings.Contains(strings.ToLower(nm), wantName) {
			return p
		}
	}
	return nil
}

func startInputReader(ctx context.Context) <-chan string {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		r := bufio.NewReader(os.Stdin)
		for {
			// permite cancelar
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := r.ReadString('\n') // bloquea hasta Enter
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)

			// no bloquees si nadie lee aún; respeta cancel
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func runningUnderGoTest() bool {
	return flag.Lookup("test.v") != nil || strings.HasSuffix(os.Args[0], ".test")
}

type keypressSession struct {
	fd    int
	state *unix.Termios
}

func newKeypressSession(file *os.File) (*keypressSession, error) {
	fd := int(file.Fd())
	state, err := unix.IoctlGetTermios(fd, termiosReadReq)
	if err != nil {
		return nil, err
	}

	newState := *state
	newState.Lflag &^= unix.ICANON | unix.ECHO
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, termiosWriteReq, &newState); err != nil {
		return nil, err
	}

	return &keypressSession{
		fd:    fd,
		state: state,
	}, nil
}

func (s *keypressSession) Close() error {
	if s == nil || s.state == nil {
		return nil
	}
	return unix.IoctlSetTermios(s.fd, termiosWriteReq, s.state)
}

func (s *keypressSession) ReadKey(timeout time.Duration) (byte, bool, error) {
	timeoutMS := -1
	if timeout >= 0 {
		timeoutMS = int(timeout / time.Millisecond)
		if timeout > 0 && timeoutMS == 0 {
			timeoutMS = 1
		}
	}

	pollFDs := []unix.PollFd{{
		Fd:     int32(s.fd),
		Events: unix.POLLIN,
	}}

	for {
		n, err := unix.Poll(pollFDs, timeoutMS)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		if n == 0 {
			return 0, false, nil
		}

		var buf [1]byte
		readN, err := unix.Read(s.fd, buf[:])
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		if readN == 0 {
			return 0, false, io.EOF
		}
		return buf[0], true, nil
	}
}

func rerunWithSudo(args []string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 1, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	cmd := exec.Command("sudo", append([]string{"-E", exe}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return exitCodeFromErr(err), err
	}
	return 0, nil
}

/* ---------- utils ---------- */

func clear() { fmt.Print("\033[2J\033[H") }

func filterDesc(byPID bool, pid int32, name string) string {
	if byPID {
		return fmt.Sprintf("PID %d", pid)
	}
	return fmt.Sprintf("nombre contiene %q", name)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func parseRefreshSeconds(args []string) int {
	refresh := 10
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if arg == "-r" || arg == "--refresh" {
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					if v < 0 {
						v = -v
					}
					refresh = v
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "-r=") || strings.HasPrefix(arg, "--refresh=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				if v, err := strconv.Atoi(parts[1]); err == nil {
					if v < 0 {
						v = -v
					}
					refresh = v
				}
			}
			continue
		}
		if digitsOnly.MatchString(strings.TrimPrefix(arg, "-")) {
			v, _ := strconv.Atoi(arg)
			if v < 0 {
				v = -v
			}
			refresh = v
		}
	}
	return refresh
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if code := exitErr.ExitCode(); code != 0 {
			return code
		}
	}
	return 1
}

func killTargetProcess(pid int32, name string) string {
	p, err := process.NewProcess(pid)
	if err != nil {
		return RED + "Could not open the process." + RESET
	}
	if err := p.Kill(); err != nil {
		return RED + "Kill failed: " + err.Error() + RESET
	}
	if name == "" {
		name = fmt.Sprintf("PID %d", pid)
	}
	return YELLOW + "Process terminated: " + name + RESET
}

func formatAddr(ip string, port uint32) string {
	if ip == "" || ip == "0.0.0.0" || ip == "::" {
		ip = "*"
	}
	return fmt.Sprintf("%s:%d", ip, port)
}
