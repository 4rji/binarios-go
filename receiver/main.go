package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultReceivePort = 1235
	defaultFilePrefix  = "file"
)

const (
	colorRed     = "\033[91m"
	colorDarkRed = "\033[31m"
	colorBlue    = "\033[94m"
	colorYellow  = "\033[93m"
	colorCyan    = "\033[96m"
	colorReset   = "\033[0m"
)

type ifaceAddress struct {
	Name string
	IP   string
}

type receiverOptions struct {
	Port      string
	Prefix    string
	OutputDir string
}

func main() {
	opts, err := parseReceiveArgs(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		printReceiveUsage()
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		printReceiveUsage()
		os.Exit(1)
	}

	if err := runFileReceiver(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%sreceiver error: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func defaultReceiverOptions() receiverOptions {
	return receiverOptions{
		Port:      strconv.Itoa(defaultReceivePort),
		Prefix:    defaultFilePrefix,
		OutputDir: ".",
	}
}

func parseReceiveArgs(args []string) (receiverOptions, error) {
	opts := defaultReceiverOptions()

	var help bool
	flags := flag.NewFlagSet("receiver", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.Port, "port", opts.Port, "listen port")
	flags.StringVar(&opts.Prefix, "prefix", opts.Prefix, "output filename prefix")
	flags.StringVar(&opts.OutputDir, "output-dir", opts.OutputDir, "directory where received files are saved")
	flags.BoolVar(&help, "help", false, "show usage")
	flags.BoolVar(&help, "h", false, "show usage")

	if err := flags.Parse(args); err != nil {
		return receiverOptions{}, err
	}
	if help {
		return opts, flag.ErrHelp
	}

	setFlags := make(map[string]bool)
	flags.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	positionals := flags.Args()
	if len(positionals) > 2 {
		return receiverOptions{}, errors.New("usage: receiver [flags] [port] [file-prefix]")
	}
	if len(positionals) >= 1 {
		if setFlags["port"] {
			return receiverOptions{}, errors.New("do not combine --port with a positional port")
		}
		opts.Port = positionals[0]
	}
	if len(positionals) == 2 {
		if setFlags["prefix"] {
			return receiverOptions{}, errors.New("do not combine --prefix with a positional file prefix")
		}
		opts.Prefix = positionals[1]
	}

	if err := validateReceiverOptions(opts); err != nil {
		return receiverOptions{}, err
	}
	return opts, nil
}

func printReceiveUsage() {
	program := filepath.Base(os.Args[0])
	fmt.Printf("%sUsage: %s [flags] [port] [file-prefix]%s\n", colorRed, program, colorReset)
	fmt.Printf("%s  Listens on 0.0.0.0:<port> (default 1235) and saves each connection to <output-dir>/<file-prefix>N%s\n", colorCyan, colorReset)
	fmt.Printf("%sFlags:%s\n", colorYellow, colorReset)
	fmt.Printf("%s  --port PORT        Listen port (alternative to positional [port])%s\n", colorCyan, colorReset)
	fmt.Printf("%s  --prefix PREFIX    Output filename prefix (alternative to positional [file-prefix])%s\n", colorCyan, colorReset)
	fmt.Printf("%s  --output-dir DIR   Directory for received files (default current directory)%s\n", colorCyan, colorReset)
	fmt.Printf("%s  Example sender: nc <listener-ip> 1235 < file_to_send%s\n", colorBlue, colorReset)
	fmt.Printf("%s  Example sender with -q 0: nc -q 0 <listener-ip> 1235 < file_send%s\n", colorBlue, colorReset)
	fmt.Printf("%s  Example receiver: %s --port 9000 --prefix loot- --output-dir incoming%s\n", colorBlue, program, colorReset)
}

func runFileReceiver(opts receiverOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := net.JoinHostPort("0.0.0.0", opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	return serveFileReceiver(ctx, ln, opts)
}

func serveFileReceiver(ctx context.Context, ln net.Listener, opts receiverOptions) error {
	opts = normalizeReceiverOptions(opts)
	if err := validateReceiverOptions(opts); err != nil {
		_ = ln.Close()
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		_ = ln.Close()
		return fmt.Errorf("creating output directory %q: %w", opts.OutputDir, err)
	}

	pathPrefix := receiverPathPrefix(opts)
	displayDir := opts.OutputDir
	if absDir, err := filepath.Abs(opts.OutputDir); err == nil {
		displayDir = absDir
	}

	active := newActiveConnections()
	var wg sync.WaitGroup

	stopWatcher := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			fmt.Println("\nSignal received, shutting down active transfers...")
			_ = ln.Close()
			active.closeAll()
		case <-stopWatcher:
		}
	}()
	defer close(stopWatcher)
	defer func() {
		_ = ln.Close()
		active.closeAll()
		wg.Wait()
	}()

	fmt.Print(renderStartupMenu(opts.Port, displayDir, availableIPv4Addresses()))

	counter := 1
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				fmt.Fprintf(os.Stderr, "temporary accept error: %v\n", err)
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		filename, file, err := createNextAvailableFile(pathPrefix, &counter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error picking filename: %v\n", err)
			conn.Close()
			continue
		}

		release := active.add(conn)
		activeNow := active.count()
		wg.Add(1)
		go func(conn net.Conn, filename string, file *os.File, activeNow int) {
			defer wg.Done()
			defer release()

			started := time.Now()
			fmt.Printf("Receiving from %s -> %s (active: %d)\n", conn.RemoteAddr(), filename, activeNow)
			bytes, err := writeConnectionToOpenFile(conn, file)
			if err != nil {
				if ctx.Err() != nil {
					fmt.Fprintf(os.Stderr, "stopped %s after %s: %v\n", filename, formatBytes(bytes), err)
					return
				}
				fmt.Fprintf(os.Stderr, "error writing %s after %s: %v\n", filename, formatBytes(bytes), err)
				return
			}
			fmt.Printf("Saved %s (%s, %s).\n", filename, formatBytes(bytes), time.Since(started).Round(time.Millisecond))
		}(conn, filename, file, activeNow)
	}
}

func normalizeReceiverOptions(opts receiverOptions) receiverOptions {
	defaults := defaultReceiverOptions()
	if opts.Port == "" {
		opts.Port = defaults.Port
	}
	if opts.Prefix == "" {
		opts.Prefix = defaults.Prefix
	}
	if opts.OutputDir == "" {
		opts.OutputDir = defaults.OutputDir
	}
	return opts
}

func validateReceiverOptions(opts receiverOptions) error {
	if err := validatePort(opts.Port); err != nil {
		return err
	}
	if strings.TrimSpace(opts.Prefix) == "" {
		return errors.New("file prefix must not be empty")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return errors.New("output directory must not be empty")
	}
	return nil
}

func receiverPathPrefix(opts receiverOptions) string {
	if filepath.IsAbs(opts.Prefix) {
		return opts.Prefix
	}
	return filepath.Join(opts.OutputDir, opts.Prefix)
}

type activeConnections struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func newActiveConnections() *activeConnections {
	return &activeConnections{conns: make(map[net.Conn]struct{})}
}

func (a *activeConnections) add(conn net.Conn) func() {
	a.mu.Lock()
	a.conns[conn] = struct{}{}
	a.mu.Unlock()

	return func() {
		a.mu.Lock()
		delete(a.conns, conn)
		a.mu.Unlock()
	}
}

func (a *activeConnections) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.conns)
}

func (a *activeConnections) closeAll() {
	a.mu.Lock()
	conns := make([]net.Conn, 0, len(a.conns))
	for conn := range a.conns {
		conns = append(conns, conn)
	}
	a.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func renderStartupMenu(port, outputDir string, addrs []ifaceAddress) string {
	lines := []string{
		"Listen  0.0.0.0:" + port + " (all interfaces)",
		"Save    " + outputDir,
		"Stop    Ctrl+C",
	}

	innerWidth := len(" Receiver ready ") + 12
	for _, line := range lines {
		if len(line) > innerWidth {
			innerWidth = len(line)
		}
	}

	title := " Receiver ready "
	targetIP := "<listener-ip>"
	if len(addrs) > 0 {
		targetIP = addrs[0].IP
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s╭─%s%s─╮%s\n", colorDarkRed, title, strings.Repeat("─", innerWidth-len(title)), colorReset)
	for _, line := range lines {
		fmt.Fprintf(&b, "%s│ %-*s │%s\n", colorDarkRed, innerWidth, line, colorReset)
	}
	fmt.Fprintf(&b, "%s╰%s╯%s\n\n", colorDarkRed, strings.Repeat("─", innerWidth+2), colorReset)

	fmt.Fprintf(&b, "%sSend a file from another host:%s\n", colorYellow, colorReset)
	fmt.Fprintf(&b, "%s  nc -q 0 %s %s < file_send%s\n", colorBlue, targetIP, port, colorReset)
	fmt.Fprintf(&b, "%s  nc      %s %s < file_send%s\n\n", colorBlue, targetIP, port, colorReset)

	if len(addrs) == 0 {
		fmt.Fprintf(&b, "%sNo active IPv4 interfaces detected; replace <listener-ip> with this host IP.%s\n", colorYellow, colorReset)
		return b.String()
	}

	maxIface := 0
	for _, addr := range addrs {
		if len(addr.Name) > maxIface {
			maxIface = len(addr.Name)
		}
	}

	fmt.Fprintf(&b, "%sLocal target IPs:%s\n", colorYellow, colorReset)
	for _, addr := range addrs {
		fmt.Fprintf(&b, "%s  %-*s  %s%s\n", colorCyan, maxIface, addr.Name, addr.IP, colorReset)
	}
	return b.String()
}

func printLocalIPs() {
	addrs := availableIPv4Addresses()
	if len(addrs) == 0 {
		fmt.Printf("%sNo active IPv4 interfaces detected; using 0.0.0.0.%s\n", colorYellow, colorReset)
		return
	}
	maxIface := len("Interface")
	maxIP := len("IP Address")
	for _, addr := range addrs {
		if len(addr.Name) > maxIface {
			maxIface = len(addr.Name)
		}
		if len(addr.IP) > maxIP {
			maxIP = len(addr.IP)
		}
	}
	border := fmt.Sprintf("+-%s-+-%s-+", strings.Repeat("-", maxIface), strings.Repeat("-", maxIP))
	fmt.Println("Available IPv4 addresses:")
	fmt.Printf("%s%s%s\n", colorCyan, border, colorReset)
	fmt.Printf("%s| %-*s | %-*s |%s\n", colorCyan, maxIface, "Interface", maxIP, "IP Address", colorReset)
	fmt.Printf("%s%s%s\n", colorCyan, border, colorReset)
	for _, addr := range addrs {
		fmt.Printf("%s| %-*s | %s", colorCyan, maxIface, addr.Name, colorBlue)
		fmt.Printf("%-*s", maxIP, addr.IP)
		fmt.Printf("%s |%s\n", colorCyan, colorReset)
	}
	fmt.Printf("%s%s%s\n", colorCyan, border, colorReset)
}

func availableIPv4Addresses() []ifaceAddress {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var addrs []ifaceAddress
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifaceAddrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				addrs = append(addrs, ifaceAddress{Name: iface.Name, IP: v4.String()})
			}
		}
	}
	return addrs
}

func createNextAvailableFile(prefix string, counter *int) (string, *os.File, error) {
	if *counter <= 0 {
		*counter = 1
	}
	for {
		candidate := fmt.Sprintf("%s%d", prefix, *counter)
		if err := ensureParentDir(candidate); err != nil {
			return "", nil, err
		}

		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			*counter++
			return candidate, file, nil
		}
		if errors.Is(err, os.ErrExist) {
			*counter++
			continue
		}
		return "", nil, fmt.Errorf("create file %q: %w", candidate, err)
	}
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %q: %w", dir, err)
		}
	}
	return nil
}

func writeConnectionToOpenFile(conn net.Conn, file *os.File) (int64, error) {
	defer conn.Close()

	written, err := io.Copy(file, conn)
	if err != nil {
		_ = file.Close()
		return written, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return written, err
	}
	if err := file.Close(); err != nil {
		return written, err
	}
	return written, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}

func validatePort(value string) error {
	p, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", value, err)
	}
	if p <= 0 || p > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
