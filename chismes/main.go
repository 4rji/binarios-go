package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	colorReset   = "\033[0m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorRed     = "\033[31m"
	colorBlue    = "\033[34m"
)

var stdinReader = bufio.NewReader(os.Stdin)

type ifaceInfo struct {
	Name string
	IP   string
}

type summaryOutput struct {
	Lines        []string
	UniqueIPs    []string
	TopTarget    string
	TopTargetMAC string
	Ports        []string
}

type trafficRecord struct {
	src    string
	dst    string
	port   string
	macSrc string
	macDst string
}

func main() {
	captureArg := parseArgs()

	printBanner()

	helpScript := locateHelpScript()

	if captureArg != "" {
		fmt.Printf("%sFound capture argument:%s %s\n", colorCyan, colorReset, captureArg)
		analyzeExistingCapture(captureArg)
		if !askYesNo("Return to the menu for more actions?", false) {
			return
		}
	}

	for {
		switch mainMenu() {
		case "1":
			file := captureFlow()
			if file != "" {
				postCaptureOptions(file)
			}
		case "2":
			path := promptCapturePath()
			analyzeExistingCapture(path)
		case "3":
			showHelpScript(helpScript)
		default:
			fmt.Println(colorGreen + "Bye!" + colorReset)
			return
		}
	}
}

func locateHelpScript() string {
	scriptDir := filepath.Dir(os.Args[0])
	candidates := []string{
		filepath.Join(scriptDir, "ayuda.sh"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "ayuda.sh"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fall back to the first guess to keep an actionable path in messages.
	return candidates[0]
}

func parseArgs() string {
	var readFile string
	flag.StringVar(&readFile, "r", "", "Read an existing capture file")
	flag.StringVar(&readFile, "read", "", "Read an existing capture file")
	flag.Parse()

	if readFile == "" && flag.NArg() > 0 {
		readFile = flag.Arg(0)
	}
	return readFile
}

func printBanner() {
	fmt.Println(colorGreen + "================================================" + colorReset)
	fmt.Printf("%sChismes (Go)%s\n", colorCyan, colorReset)
	fmt.Println(colorGreen + "================================================" + colorReset)
}

func mainMenu() string {
	fmt.Println()
	fmt.Println(colorYellow + "What do you want to do?" + colorReset)
	fmt.Println(colorCyan + "  [1]" + colorReset + " Capture live traffic with tcpdump")
	fmt.Println(colorCyan + "  [2]" + colorReset + " Analyze an existing capture (.pcap/.cap)")
	fmt.Println(colorCyan + "  [3]" + colorReset + " View tcpdump help (ayuda.sh)")
	fmt.Println(colorCyan + "  [4]" + colorReset + " Exit")

	key := readKey("Pick an option [4]: ")
	if key == "" {
		return "4"
	}
	key = strings.ToLower(key)
	switch key {
	case "1", "2", "3", "4":
		return key
	default:
		fmt.Println(colorRed + "Invalid choice, exiting." + colorReset)
		return "4"
	}
}

func prompt(question string) string {
	fmt.Print(question)
	text, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(text)
}

func readKey(question string) string {
	fmt.Print(question)

	// Save current terminal state
	stateCmd := exec.Command("stty", "-g")
	stateCmd.Stdin = os.Stdin
	state, err := stateCmd.Output()
	if err != nil {
		// Fallback to standard prompt
		return prompt("")
	}

	// Switch to cbreak/no-echo for single key read
	cbCmd := exec.Command("stty", "-icanon", "min", "1", "-echo")
	cbCmd.Stdin = os.Stdin
	if err := cbCmd.Run(); err != nil {
		return prompt("")
	}

	defer func() {
		restore := exec.Command("stty", string(bytes.TrimSpace(state)))
		restore.Stdin = os.Stdin
		_ = restore.Run()
	}()

	buf := make([]byte, 1)
	if _, err := os.Stdin.Read(buf); err != nil {
		return ""
	}
	fmt.Println()
	return string(buf)
}

func askYesNo(question string, defaultYes bool) bool {
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}
	answer := strings.ToLower(prompt(question + suffix))
	if answer == "" {
		return defaultYes
	}
	switch answer[0] {
	case 'y':
		return true
	case 'n':
		return false
	default:
		return defaultYes
	}
}

func promptCapturePath() string {
	for {
		path := prompt("Path to capture file: ")
		if path == "" {
			fmt.Println(colorRed + "Please provide a capture path." + colorReset)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			fmt.Printf("%sCannot read %s: %v%s\n", colorRed, path, err, colorReset)
			continue
		}
		return path
	}
}

func analyzeExistingCapture(path string) {
	if path == "" {
		fmt.Println(colorRed + "No capture file specified." + colorReset)
		return
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("%sCapture file not found: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("%sAnalyzing capture:%s %s\n", colorYellow, colorReset, path)
	if !askYesNo("Process the capture to extract IP/port data?", true) {
		fmt.Println(colorBlue + "Skipping processing." + colorReset)
		return
	}

	result, err := summarizeCapture(path)
	if err != nil {
		fmt.Printf("%sUnable to build summary: %v%s\n", colorRed, err, colorReset)
		return
	}
	if len(result.Lines) == 0 {
		fmt.Println(colorRed + "No traffic records were found in the capture." + colorReset)
		return
	}

	fmt.Printf("%sTop target:%s %s\n", colorGreen, colorReset, result.TopTarget)

	var domains []string
	for {
		fmt.Println()
		fmt.Println(colorYellow + "What do you want to view?" + colorReset)
		fmt.Println(colorCyan + "  [1]" + colorReset + " Attacked IPs + ports (top target)")
		fmt.Println(colorCyan + "  [2]" + colorReset + " Unique IPs")
		fmt.Println(colorCyan + "  [3]" + colorReset + " Unique ports")
		fmt.Println(colorCyan + "  [4]" + colorReset + " Top target only")
		fmt.Println(colorCyan + "  [5]" + colorReset + " Run reverse DNS (dig -x) and show domains")
		fmt.Println(colorCyan + "  [6]" + colorReset + " Save summary to file")
		fmt.Println(colorCyan + "  [7]" + colorReset + " Save domains to file (runs dig if needed)")
		fmt.Println(colorCyan + "  [b]" + colorReset + " Back")

		choice := strings.ToLower(readKey("Choose [b]: "))
		if choice == "" || choice == "b" {
			return
		}

		switch choice {
		case "1":
			fmt.Println(colorMagenta + "Attacked IPs + ports:\n" + colorReset + strings.Join(result.Lines, "\n"))
			if result.TopTargetMAC != "" {
				fmt.Printf("%sCompromised host MAC:%s %s\n", colorGreen, colorReset, result.TopTargetMAC)
			} else {
				fmt.Println(colorYellow + "Compromised host MAC: not found" + colorReset)
			}
		case "2":
			fmt.Println(colorMagenta + "Unique IPs:\n" + colorReset + strings.Join(result.UniqueIPs, "\n"))
		case "3":
			fmt.Println(colorMagenta + "Unique ports:\n" + colorReset + strings.Join(result.Ports, "\n"))
		case "4":
			fmt.Printf("%sTop target:%s %s\n", colorGreen, colorReset, result.TopTarget)
		case "5":
			if len(domains) == 0 {
				if !askYesNo("Run dig -x for unique IPs now?", true) {
					continue
				}
				dlist, err := resolveDomainsList(result.UniqueIPs)
				if err != nil {
					fmt.Printf("%sReverse DNS failed: %v%s\n", colorRed, err, colorReset)
					continue
				}
				domains = dlist
			}
			if len(domains) == 0 {
				fmt.Println(colorRed + "No domains found." + colorReset)
			} else {
				fmt.Println(colorMagenta + "Domains:\n" + colorReset + strings.Join(domains, "\n"))
			}
		case "6":
			if !askYesNo("Save attacked IP/port summary to a file?", true) {
				continue
			}
			outFile := chooseOutputFile("ips-ports")
			if err := writeLines(outFile, result.Lines); err != nil {
				fmt.Printf("%sCould not write %s: %v%s\n", colorRed, outFile, err, colorReset)
			} else {
				fmt.Printf("%sSummary saved to%s %s\n", colorGreen, colorReset, outFile)
			}
		case "7":
			if len(domains) == 0 {
				if !askYesNo("Run dig -x for unique IPs now?", true) {
					continue
				}
				dlist, err := resolveDomainsList(result.UniqueIPs)
				if err != nil {
					fmt.Printf("%sReverse DNS failed: %v%s\n", colorRed, err, colorReset)
					continue
				}
				domains = dlist
			}
			if len(domains) == 0 {
				fmt.Println(colorRed + "No domains to save." + colorReset)
				continue
			}
			if !askYesNo("Save domains to a file?", true) {
				continue
			}
			outFile := chooseOutputFile("domains")
			if err := writeLines(outFile, domains); err != nil {
				fmt.Printf("%sCould not write %s: %v%s\n", colorRed, outFile, err, colorReset)
			} else {
				fmt.Printf("%sDomains saved to%s %s\n", colorGreen, colorReset, outFile)
			}
		default:
			fmt.Println(colorRed + "Invalid option." + colorReset)
		}
	}
}

func chooseOutputFile(defaultName string) string {
	name := defaultName
	if _, err := os.Stat(defaultName); err == nil {
		if askYesNo(fmt.Sprintf("%s exists. Overwrite?", defaultName), true) {
			return defaultName
		}
		name = fmt.Sprintf("%s_%s", defaultName, time.Now().Format("20060102150405"))
		fmt.Printf("%sUsing%s %s\n", colorYellow, colorReset, name)
	}
	return name
}

func summarizeCapture(path string) (summaryOutput, error) {
	if err := ensureCommand("tshark"); err != nil {
		return summaryOutput{}, err
	}

	cmd := exec.Command("tshark", "-r", path, "-T", "fields", "-E", "separator=,", "-e", "ip.src", "-e", "ip.dst", "-e", "tcp.dstport", "-e", "udp.dstport", "-e", "eth.src", "-e", "eth.dst")
	raw, err := cmd.Output()
	if err != nil {
		return summaryOutput{}, fmt.Errorf("tshark failed: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	destCount := map[string]int{}
	uniqueIPs := map[string]struct{}{}
	uniquePorts := map[string]struct{}{}
	var records []trafficRecord

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ",")
		for len(parts) < 6 {
			parts = append(parts, "")
		}

		src := strings.TrimSpace(parts[0])
		dst := strings.TrimSpace(parts[1])
		tcpPort := strings.TrimSpace(parts[2])
		udpPort := strings.TrimSpace(parts[3])
		macSrc := strings.TrimSpace(parts[4])
		macDst := strings.TrimSpace(parts[5])
		port := tcpPort
		if port == "" {
			port = udpPort
		}
		if port == "" {
			port = "-"
		}

		if dst != "" {
			destCount[dst]++
		}
		if src != "" {
			uniqueIPs[src] = struct{}{}
		}
		if dst != "" {
			uniqueIPs[dst] = struct{}{}
		}
		if port != "-" {
			uniquePorts[port] = struct{}{}
		}
		if src == "" && dst == "" && port == "-" {
			continue
		}
		records = append(records, trafficRecord{src: src, dst: dst, port: port, macSrc: macSrc, macDst: macDst})
	}

	if len(destCount) == 0 {
		return summaryOutput{}, errors.New("no IP traffic found (tshark ip.src/ip.dst were empty)")
	}

	topTarget, topHits := "", 0
	for dst, count := range destCount {
		if count > topHits {
			topTarget = dst
			topHits = count
		}
	}

	lines, topMAC := summarizeTopTarget(records, topTarget)
	ipList := mapsToSortedSlice(uniqueIPs)
	portList := mapsToSortedSlice(uniquePorts)

	return summaryOutput{Lines: lines, UniqueIPs: ipList, TopTarget: topTarget, TopTargetMAC: topMAC, Ports: portList}, nil
}

func summarizeTopTarget(records []trafficRecord, target string) ([]string, string) {
	type key struct {
		src  string
		dst  string
		port string
	}
	type lineStats struct {
		count     int
		srcMacHit map[string]int
		dstMacHit map[string]int
	}
	stats := map[key]*lineStats{}
	macCounts := map[string]int{}
	var lines []string
	for _, rec := range records {
		if rec.dst != target {
			if rec.src != target {
				continue
			}
		}
		k := key{src: rec.src, dst: rec.dst, port: rec.port}
		entry := stats[k]
		if entry == nil {
			entry = &lineStats{
				srcMacHit: map[string]int{},
				dstMacHit: map[string]int{},
			}
			stats[k] = entry
		}
		entry.count++
		if rec.macSrc != "" {
			entry.srcMacHit[rec.macSrc]++
		}
		if rec.macDst != "" {
			entry.dstMacHit[rec.macDst]++
		}

		if rec.dst == target && rec.macDst != "" {
			macCounts[rec.macDst]++
		}
		if rec.src == target && rec.macSrc != "" {
			macCounts[rec.macSrc]++
		}
	}

	for k, entry := range stats {
		if entry == nil || entry.count == 0 {
			continue
		}
		srcMac := pickTopMac(entry.srcMacHit)
		dstMac := pickTopMac(entry.dstMacHit)
		line := fmt.Sprintf("%s (%s) %s (%s) %s (%d)", k.src, srcMac, k.dst, dstMac, k.port, entry.count)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)

	topMAC := ""
	topHits := 0
	for mac, count := range macCounts {
		if count > topHits {
			topMAC = mac
			topHits = count
		}
	}
	return lines, topMAC
}

func pickTopMac(counts map[string]int) string {
	if len(counts) == 0 {
		return "-"
	}
	top := ""
	topHits := 0
	for mac, count := range counts {
		if count > topHits {
			top = mac
			topHits = count
		}
	}
	if top == "" {
		return "-"
	}
	return top
}

func mapsToSortedSlice(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func resolveDomainsList(ips []string) ([]string, error) {
	if len(ips) == 0 {
		return nil, errors.New("no IPs available to resolve")
	}
	if err := ensureCommand("dig"); err != nil {
		return nil, err
	}

	var domains []string
	for _, ip := range ips {
		if net.ParseIP(ip) == nil {
			continue
		}
		cmd := exec.Command("dig", "+short", "-x", ip)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSuffix(strings.TrimSpace(line), ".")
			if line != "" {
				domains = append(domains, line)
				break
			}
		}
	}

	if len(domains) == 0 {
		return nil, errors.New("no PTR records returned by dig")
	}
	return dedupe(domains), nil
}

func dedupe(values []string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

func captureFlow() string {
	if err := ensureCommand("tcpdump"); err != nil {
		fmt.Println(colorRed + err.Error() + colorReset)
		return ""
	}

	ifaces := collectInterfaces()
	fmt.Println(colorYellow + "Available interfaces:" + colorReset)
	for i, iface := range ifaces {
		ipLabel := iface.IP
		ipColor := colorMagenta
		if iface.IP == "No IP" {
			ipColor = colorRed
		}
		fmt.Printf("  [%d] %s%s%s (%s%s%s)\n", i+1, colorCyan, iface.Name, colorReset, ipColor, ipLabel, colorReset)
	}

	choiceStr := prompt("Pick interface [1 for any]: ")
	if choiceStr == "" {
		choiceStr = "1"
	}
	index, err := strconv.Atoi(choiceStr)
	if err != nil || index < 1 || index > len(ifaces) {
		fmt.Println(colorRed + "Invalid choice, defaulting to 'any'." + colorReset)
		index = 1
	}
	selected := ifaces[index-1].Name

	port := ""
	if askYesNo("Listen on a specific port?", false) {
		rawPort := prompt("Port number (default TCP; use /udp or /any): ")
		if rawPort != "" {
			filter, err := portFilterFromInput(rawPort)
			if err != nil {
				fmt.Println(colorRed + err.Error() + colorReset)
				return ""
			}
			port = filter
		}
	}

	outFile := pickCaptureFile()
	if outFile == "" {
		return ""
	}

	if err := runTcpdump(selected, port, outFile); err != nil {
		fmt.Printf("%sCapture failed: %v%s\n", colorRed, err, colorReset)
		return ""
	}
	fmt.Printf("%sCapture saved to%s %s\n", colorGreen, colorReset, outFile)
	return outFile
}

func collectInterfaces() []ifaceInfo {
	ifaces := []ifaceInfo{{Name: "any", IP: ""}}
	sysIfaces, err := net.Interfaces()
	if err != nil {
		return ifaces
	}
	for _, iface := range sysIfaces {
		ipLabel := "No IP"
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ip := ipNet.IP.To4(); ip != nil {
					ipLabel = ip.String()
					break
				}
			}
		}
		ifaces = append(ifaces, ifaceInfo{Name: iface.Name, IP: ipLabel})
	}
	return ifaces
}

func isNumeric(val string) bool {
	_, err := strconv.Atoi(val)
	return err == nil
}

func portFilterFromInput(input string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return "", nil
	}

	port := trimmed
	proto := "tcp"
	if strings.Contains(trimmed, "/") {
		parts := strings.SplitN(trimmed, "/", 2)
		port = strings.TrimSpace(parts[0])
		proto = strings.TrimSpace(parts[1])
		if proto == "" {
			proto = "tcp"
		}
	}

	if port == "" || !isNumeric(port) {
		return "", errors.New("Invalid port. Cancelling capture.")
	}

	switch proto {
	case "tcp", "t":
		return "tcp port " + port, nil
	case "udp", "u":
		return "udp port " + port, nil
	case "any", "all", "*":
		return "port " + port, nil
	default:
		return "", errors.New("Invalid protocol. Use /udp or /any.")
	}
}

func pickCaptureFile() string {
	defaultName := "capture.cap"
	if _, err := os.Stat(defaultName); err == nil {
		if askYesNo(defaultName+" exists. Overwrite?", true) {
			return defaultName
		}
		newName := fmt.Sprintf("capture_%s.cap", time.Now().Format("20060102150405"))
		fmt.Printf("%sUsing%s %s\n", colorYellow, colorReset, newName)
		return newName
	}
	return defaultName
}

func runTcpdump(iface, port, outFile string) error {
	args := []string{"tcpdump", "-i", iface}
	if port != "" {
		// port already contains the filter fragment (e.g., "tcp port 80").
		args = append(args, strings.Split(port, " ")...)
	}
	args = append(args, "-w", outFile, "-v")

	cmdName := args[0]
	cmdArgs := args[1:]
	if os.Geteuid() != 0 {
		cmdName = "sudo"
		cmdArgs = args
	}

	fmt.Printf("%sRunning:%s %s %s\n", colorGreen, colorReset, cmdName, strings.Join(cmdArgs, " "))
	fmt.Println(colorYellow + "Press Ctrl+C when you want to stop the capture." + colorReset)

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil
		}
		return err
	}
	return nil
}

func postCaptureOptions(captureFile string) {
	if askYesNo("Quickly dump the capture with tshark now?", false) {
		runTshark(captureFile)
	}
	analyzeExistingCapture(captureFile)
}

func runTshark(path string) {
	if err := ensureCommand("tshark"); err != nil {
		fmt.Println(colorRed + err.Error() + colorReset)
		return
	}
	cmd := exec.Command("tshark", "-r", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sFailed to run tshark: %v%s\n", colorRed, err, colorReset)
	}
}

func showHelpScript(path string) {
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("%sHelp script not found at %s%s\n", colorRed, path, colorReset)
		return
	}
	cmd := exec.Command("bash", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sFailed to run ayuda.sh: %v%s\n", colorRed, err, colorReset)
	}
}

func ensureCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s is required but not found in PATH", name)
	}
	return nil
}