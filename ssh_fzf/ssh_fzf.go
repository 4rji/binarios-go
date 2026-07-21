// ssh_fzf.go
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type SSHHost struct {
	Alias    string
	Hostname string
	Port     string
	User     string
	Online   bool
}

type rgbColor struct {
	r int
	g int
	b int
}

// colWidths holds padded column widths computed from the host list.
type colWidths struct {
	alias, host, port, user int
}

func computeWidths(hosts []SSHHost) colWidths {
	cw := colWidths{alias: 6, host: 10, port: 2, user: 4}
	for _, h := range hosts {
		hn, port := hostAndPort(h)
		if n := len(h.Alias); n > cw.alias {
			cw.alias = n
		}
		if n := len(hn); n > cw.host {
			cw.host = n
		}
		if n := len(port); n > cw.port {
			cw.port = n
		}
		if n := len(h.User); n > cw.user {
			cw.user = n
		}
	}
	return cw
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "preview":
			runPreview()
			return
		case "netdiag":
			runNetDiag()
			return
		}
	}

	hosts, err := parseSSHConfig()
	if err != nil {
		log.Fatalf("error leyendo ~/.ssh/config: %v", err)
	}
	if len(hosts) == 0 {
		log.Fatalf("no se encontraron hosts en ~/.ssh/config")
	}

	timeout := statusTimeout()
	self, err := os.Executable()
	if err != nil {
		log.Fatalf("no se pudo obtener ruta del binario: %v", err)
	}

	cw := computeWidths(hosts)

	// col1=alias(hidden), col2=status(hidden), col3..=display
	// {1}=alias, {2}="online"|"offline" — preview skips re-probe.
	previewWindow := func(tool string) string {
		if tool == "" {
			return "right:50%:wrap"
		}
		return "right:90%:wrap"
	}
	previewCmd := func(tool string) string {
		cmd := fmt.Sprintf("%s preview '{1}' '{2}'", shellQuote(self))
		if tool != "" {
			cmd += " " + shellQuote(tool)
		}
		return cmd
	}
	previewAction := func(tool string) string {
		action := fmt.Sprintf(
			"change-preview-window(%s)+change-preview(%s)",
			previewWindow(tool),
			previewCmd(tool),
		)
		if tool == "" {
			return action + "+change-preview-label()"
		}
		return action + fmt.Sprintf("+change-preview-label(%s)", tool)
	}
	diagBind := func(key, tool string) string {
		return fmt.Sprintf(
			"%s:transform:if [ \"$FZF_PREVIEW_LABEL\" = %s ]; then printf '%%s' %s; else printf '%%s' %s; fi",
			key,
			shellQuote(tool),
			shellQuote(previewAction("")),
			shellQuote(previewAction(tool)),
		)
	}
	diagExecBind := func(key, tool string) string {
		return fmt.Sprintf("%s:execute(%s netdiag %s '{1}')", key, shellQuote(self), tool)
	}
	cmd := exec.Command("fzf",
		"--ansi",
		"--delimiter", "\t",
		"--no-sort",
		"--tiebreak=index",
		"--with-nth=3..",
		"--layout=reverse",
		"--border=rounded",
		"--border-label", " ❯ SSH ",
		"--prompt", "  ",
		"--pointer", "▌",
		"--color", "bg+:#1e2030,fg+:#c8d3f5,hl+:#82aaff,hl:#82aaff,border:#589ed7,header:#636da6,pointer:#ff966c,prompt:#82aaff,label:#82aaff,info:#636da6",
		"--header", "  ↵ ssh\n  D dig   G gping   M mtr\n  P trip  T tracepath   R traceroute",
		"--preview-window", previewWindow(""),
		"--preview", previewCmd(""),
		"--bind", "ctrl-s:toggle-sort",
		"--bind", diagBind("D", "dig"),
		"--bind", diagExecBind("G", "gping"),
		"--bind", diagBind("M", "mtr"),
		"--bind", diagExecBind("P", "trip"),
		"--bind", diagBind("T", "tracepath"),
		"--bind", diagBind("R", "traceroute"),
	)

	pr, pw := io.Pipe()
	cmd.Stdin = pr

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("fzf no disponible: %v", err)
	}

	go func() {
		defer pw.Close()
		streamHosts(hosts, timeout, pw, cw)
	}()

	if err := cmd.Wait(); err != nil {
		os.Exit(0)
	}

	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return
	}

	alias := sanitizeSSHTarget(extractAlias(selected))
	if alias == "" {
		return
	}

	sshCmd := exec.Command("ssh", "--", alias)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	_ = sshCmd.Run()
}

// streamHosts probes all hosts concurrently. Online hosts are written to w as
// they resolve (appear in fzf immediately). Offline hosts are buffered and
// flushed only after all probes finish, keeping online-first ordering without
// blocking the fzf startup.
func streamHosts(hosts []SSHHost, timeout time.Duration, w io.Writer, cw colWidths) {
	ch := make(chan SSHHost, len(hosts))
	sem := make(chan struct{}, statusWorkerLimit(len(hosts)))
	var wg sync.WaitGroup

	for _, h := range hosts {
		wg.Add(1)
		go func(h SSHHost) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			hn, port := hostAndPort(h)
			h.Online = isOnline(hn, port, timeout)
			ch <- h
		}(h)
	}
	go func() { wg.Wait(); close(ch) }()

	bw := bufio.NewWriter(w)
	var offline []SSHHost

	for h := range ch {
		if h.Online {
			writeHostLine(bw, h, cw)
			_ = bw.Flush()
		} else {
			offline = append(offline, h)
		}
	}
	for _, h := range offline {
		writeHostLine(bw, h, cw)
	}
	_ = bw.Flush()
}

func writeHostLine(w io.Writer, h SSHHost, cw colWidths) {
	hn, port := hostAndPort(h)
	statusKey := "offline"

	var icon, aColor, hColor, pColor, uColor string
	const reset = "\033[0m"

	if h.Online {
		statusKey = "online"
		icon = "\033[32m❯\033[0m "
		aColor = "\033[1;34m" // bold blue
		hColor = "\033[36m"   // cyan
		pColor = "\033[33m"   // yellow
		uColor = "\033[37m"   // white
	} else {
		icon = "  "
		aColor = "\033[2;37m" // dim
		hColor = "\033[2;37m"
		pColor = "\033[2;37m"
		uColor = "\033[2;37m"
	}
	aColor = subnetHostNameColor(hn, h.Online, aColor)

	display := fmt.Sprintf("%s%s%-*s%s  %s%-*s%s  %s%-*s%s  %s%-*s%s",
		icon,
		aColor, cw.alias, h.Alias, reset,
		hColor, cw.host, hn, reset,
		pColor, cw.port, port, reset,
		uColor, cw.user, h.User, reset,
	)

	fmt.Fprintf(w, "%s\t%s\t%s\n", h.Alias, statusKey, display)
}

func subnetHostNameColor(hostname string, online bool, fallback string) string {
	ip := net.ParseIP(hostname).To4()
	if ip == nil {
		return fallback
	}
	hue := (int(ip[0])*53 + int(ip[1])*29 + int(ip[2])*43 + int(ip[2])*int(ip[2])*7) % 360
	r, g, b := hsvToRGB(hue, 70, 95)
	if online {
		return fmt.Sprintf("\033[1;38;2;%d;%d;%dm", r, g, b)
	}
	return fmt.Sprintf("\033[2;38;2;%d;%d;%dm", r, g, b)
}

func hsvToRGB(h, s, v int) (int, int, int) {
	c := v * s / 100
	x := c * (60 - absInt(h%120-60)) / 60
	m := v - c

	var r, g, b int
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return (r + m) * 255 / 100, (g + m) * 255 / 100, (b + m) * 255 / 100
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func parseSSHConfig() ([]SSHHost, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	visited := make(map[string]bool)
	return parseSSHConfigFile(filepath.Join(home, ".ssh", "config"), home, visited)
}

func parseSSHConfigFile(path, home string, visited map[string]bool) ([]SSHHost, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if visited[abs] {
		return nil, nil // circular include guard
	}
	visited[abs] = true

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // silently skip missing includes
		}
		return nil, err
	}
	defer f.Close()

	var hosts []SSHHost
	var current *SSHHost

	flush := func() {
		if current != nil {
			hosts = append(hosts, *current)
			current = nil
		}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '#'); idx != -1 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		val := strings.Join(fields[1:], " ")

		switch key {
		case "host":
			flush()
			alias := firstRunnableHostAlias(fields[1:])
			if alias == "" {
				continue
			}
			current = &SSHHost{Alias: alias}
		case "hostname":
			if current != nil {
				current.Hostname = val
			}
		case "port":
			if current != nil {
				current.Port = val
			}
		case "user":
			if current != nil {
				current.User = val
			}
		case "include":
			flush()
			pattern := val
			if strings.HasPrefix(pattern, "~/") {
				pattern = filepath.Join(home, pattern[2:])
			} else if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(home, ".ssh", pattern)
			}
			matches, _ := filepath.Glob(pattern)
			for _, m := range matches {
				sub, _ := parseSSHConfigFile(m, home, visited)
				hosts = append(hosts, sub...)
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func firstRunnableHostAlias(patterns []string) string {
	for _, p := range patterns {
		if p == "*" || strings.HasPrefix(p, "!") {
			continue
		}
		if !isRunnableHostAlias(p) {
			continue
		}
		return p
	}
	return ""
}

func isRunnableHostAlias(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return !strings.ContainsAny(s, "*?[]")
}

func statusWorkerLimit(total int) int {
	limit := runtime.NumCPU() * 4
	if limit < 4 {
		limit = 4
	}
	if total < limit {
		return total
	}
	return limit
}

func isOnline(hostname, port string, timeout time.Duration) bool {
	address := net.JoinHostPort(hostname, port)
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func hostAndPort(h SSHHost) (string, string) {
	hostname := h.Hostname
	if hostname == "" {
		hostname = h.Alias
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	return hostname, port
}

func statusTimeout() time.Duration {
	const defaultTimeout = 400 * time.Millisecond
	const maxTimeout = 5 * time.Second
	if v := os.Getenv("SSH_FZF_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			d := time.Duration(ms) * time.Millisecond
			if d > maxTimeout {
				return maxTimeout
			}
			return d
		}
	}
	return defaultTimeout
}

func extractAlias(line string) string {
	if alias, _, ok := strings.Cut(line, "\t"); ok {
		return strings.TrimSpace(alias)
	}

	line = strings.TrimSpace(stripANSI(line))
	if strings.HasPrefix(line, "[") {
		if idx := strings.IndexAny(line, "]❯"); idx != -1 && len(line) > idx+1 {
			line = strings.TrimSpace(line[idx+1:])
		}
	}
	parts := strings.SplitN(line, " - ", 2)
	return strings.TrimSpace(parts[0])
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var pingLatencyRegexp = regexp.MustCompile(`(?m)(?:round-trip|rtt)[^=]*=\s*([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+)\s*ms`)
var pingTTLRegexp = regexp.MustCompile(`(?im)ttl[=:]([0-9]+)`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func sanitizeSSHTarget(s string) string {
	s = strings.TrimSpace(stripANSI(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Fields(s)[0]
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func normalizeDiagTool(s string) string {
	switch s {
	case "dig", "gping", "mtr", "tracepath", "traceroute", "trip":
		return s
	default:
		return ""
	}
}

func commandOutput(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	out := strings.TrimSpace(buf.String())

	if ctx.Err() == context.DeadlineExceeded {
		if out == "" {
			out = fmt.Sprintf("%s agotó el timeout de %s", name, timeout)
		}
		return out, ctx.Err()
	}

	return out, err
}

func resolveDiagTarget(hostname string) string {
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	ips, err := net.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return hostname
	}

	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}

	return ips[0].String()
}

func isInteractiveDiagTool(tool string) bool {
	switch tool {
	case "gping", "trip":
		return true
	default:
		return false
	}
}

func netDiagArgs(tool, hostname string) []string {
	target := resolveDiagTarget(hostname)

	switch tool {
	case "dig":
		return []string{"dig", hostname}
	case "gping":
		return []string{"gping", target}
	case "mtr":
		return []string{"mtr", "-rwzc", "20", target}
	case "trip":
		return []string{"trip", "--protocol", "tcp", target, "-G", "/opt/4rji/GeoLite2-City.mmdb", "-e"}
	case "tracepath":
		return []string{"tracepath", target}
	case "traceroute":
		return []string{"traceroute", "-P", "tcp", "-p", "22", target}
	default:
		return nil
	}
}

func netDiagCommand(tool, hostname string) ([]string, time.Duration) {
	cmdArgs := netDiagArgs(tool, hostname)
	if len(cmdArgs) == 0 {
		return nil, 0
	}

	switch tool {
	case "dig":
		return cmdArgs, 5 * time.Second
	case "gping":
		return cmdArgs, 8 * time.Second
	case "mtr":
		return cmdArgs, 30 * time.Second
	case "trip":
		return cmdArgs, 30 * time.Second
	case "tracepath":
		return cmdArgs, 20 * time.Second
	case "traceroute":
		return cmdArgs, 20 * time.Second
	default:
		return nil, 0
	}
}

func netDiagOutput(tool, hostname string) string {
	cmdArgs, timeout := netDiagCommand(tool, hostname)
	if len(cmdArgs) == 0 {
		return fmt.Sprintf("Herramienta no soportada: %s", tool)
	}

	out, err := commandOutput(timeout, cmdArgs[0], cmdArgs[1:]...)
	if err != nil && out == "" {
		out = err.Error()
	}

	if out == "" {
		out = "Sin salida."
	}

	return out
}

func indentBlock(s, prefix string) string {
	if s == "" {
		return prefix
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func probeGradientColor(index, total int) rgbColor {
	palette := []rgbColor{
		{r: 255, g: 100, b: 200},
		{r: 200, g: 120, b: 255},
		{r: 150, g: 150, b: 255},
	}
	if total <= 1 {
		return palette[0]
	}

	scale := 1024
	maxSegment := len(palette) - 1
	position := index * maxSegment * scale / (total - 1)
	segment := position / scale
	if segment >= maxSegment {
		return palette[maxSegment]
	}

	start := palette[segment]
	end := palette[segment+1]
	blend := position % scale

	return rgbColor{
		r: start.r + (end.r-start.r)*blend/scale,
		g: start.g + (end.g-start.g)*blend/scale,
		b: start.b + (end.b-start.b)*blend/scale,
	}
}

func colorizeProbeLine(s string) string {
	const reset = "\033[0m"

	runes := []rune(s)
	visible := 0
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			visible++
		}
	}
	if visible == 0 {
		return s
	}

	var b strings.Builder
	index := 0
	for _, r := range runes {
		if unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		c := probeGradientColor(index, visible)
		fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm%s", c.r, c.g, c.b, string(r))
		index++
	}
	b.WriteString(reset)

	return b.String()
}

func inferOSFromTTL(ttl int) string {
	switch {
	case ttl <= 64:
		return "Linux"
	case ttl <= 128:
		return "Windows"
	default:
		return "Other"
	}
}

func inferOSFromBanner(banner string) string {
	switch {
	case strings.Contains(banner, "Win32-OpenSSH"), strings.Contains(banner, "Microsoft"):
		return "Windows"
	case strings.Contains(banner, "OpenSSH"), strings.Contains(banner, "Dropbear"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func readSSHBanner(hostname, port string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(hostname, port))
	if err != nil {
		return ""
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	banner, err := bufio.NewReader(conn).ReadString('\n')
	if banner == "" && err != nil {
		return ""
	}

	return strings.TrimSpace(banner)
}

func hostProbeOutput(hostname, port string) string {
	pingOut, _ := commandOutput(2*time.Second, "ping", "-c", "1", "-W", "1", hostname)
	latency := "N/A"
	if match := pingLatencyRegexp.FindStringSubmatch(pingOut); len(match) > 2 {
		latency = match[2]
	}

	if match := pingTTLRegexp.FindStringSubmatch(pingOut); len(match) > 1 {
		ttl := match[1]
		if ttlValue, err := strconv.Atoi(ttl); err == nil {
			return fmt.Sprintf("latency=%sms ttl=%s os=%s", latency, ttl, inferOSFromTTL(ttlValue))
		}
	}

	banner := readSSHBanner(hostname, port)
	if banner == "" {
		banner = "no-banner"
	}

	return fmt.Sprintf("latency=%sms ttl=N/A os=%s ssh='%s'",
		latency,
		inferOSFromBanner(banner),
		strings.ReplaceAll(banner, "'", "\\'"),
	)
}

func runNetDiag() {
	// os.Args: [binary, "netdiag", tool, alias]
	if len(os.Args) < 4 {
		return
	}
	tool := normalizeDiagTool(os.Args[2])
	alias := sanitizeSSHTarget(os.Args[3])
	if alias == "" || tool == "" {
		return
	}

	hosts, _ := parseSSHConfig()
	var hostname string
	for _, h := range hosts {
		if h.Alias == alias {
			hostname, _ = hostAndPort(h)
			break
		}
	}
	if hostname == "" {
		hostname = alias // fallback: treat alias as hostname directly
	}

	if isInteractiveDiagTool(tool) {
		cmdArgs := netDiagArgs(tool, hostname)
		if len(cmdArgs) == 0 {
			return
		}

		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}

	fmt.Fprintln(os.Stdout, netDiagOutput(tool, hostname))
}

func runPreview() {
	if len(os.Args) < 3 {
		return
	}
	alias := sanitizeSSHTarget(os.Args[2])
	if alias == "" {
		alias = sanitizeSSHTarget(extractAlias(strings.Join(os.Args[2:], " ")))
		if alias == "" {
			return
		}
	}

	hosts, err := parseSSHConfig()
	if err != nil {
		fmt.Printf("Error leyendo ~/.ssh/config: %v\n", err)
		return
	}

	var target *SSHHost
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		fmt.Printf("Host %q not found in ~/.ssh/config\n", alias)
		return
	}

	hostname, port := hostAndPort(*target)
	tool := ""

	// {2} passed by fzf = "online"|"offline" — skip TCP re-probe for instant preview.
	var online bool
	if len(os.Args) >= 4 && (os.Args[3] == "online" || os.Args[3] == "offline") {
		online = os.Args[3] == "online"
		if len(os.Args) >= 5 {
			tool = normalizeDiagTool(os.Args[4])
		}
	} else {
		online = isOnline(hostname, port, statusTimeout())
		if len(os.Args) >= 4 {
			tool = normalizeDiagTool(os.Args[3])
		}
	}

	fmt.Print("\n\n")

	if tool != "" {
		fmt.Printf("%s\n", netDiagOutput(tool, hostname))
		return
	}

	const (
		reset  = "\033[0m"
		bold   = "\033[1m"
		dim    = "\033[2m"
		red    = "\033[31m"
		green  = "\033[32m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		cyan   = "\033[36m"
	)

	sep := dim + strings.Repeat("─", 34) + reset

	fmt.Printf("\n  %s%s SSH Host%s\n", bold, blue, reset)
	fmt.Printf("  %s\n\n", sep)
	fmt.Printf("  %s%-9s%s %s%s%s\n", cyan, "Alias", reset, bold, target.Alias, reset)
	fmt.Printf("  %s%-9s%s %s\n", cyan, "Hostname", reset, hostname)
	fmt.Printf("  %s%-9s%s %s\n", cyan, "Port", reset, port)
	if target.User != "" {
		fmt.Printf("  %s%-9s%s %s\n", cyan, "User", reset, target.User)
	}

	fmt.Printf("\n  %s%-9s%s %sssh %s%s\n", cyan, "Connect", reset, yellow, target.Alias, reset)

	fmt.Printf("\n  %s\n\n", sep)

	if online {
		fmt.Printf("  %s%s● Online%s\n\n", bold, green, reset)
	} else {
		fmt.Printf("  %s● Offline%s\n\n", red, reset)
	}

	fmt.Printf("  %s\n\n", sep)
	fmt.Printf("%s\n", indentBlock(colorizeProbeLine(hostProbeOutput(hostname, port)), "  "))
}
