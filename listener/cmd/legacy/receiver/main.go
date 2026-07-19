package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	defaultReceivePort = 1235
	defaultFilePrefix  = "file"
)

const (
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorCyan   = "\033[96m"
	colorReset  = "\033[0m"
)

type ifaceAddress struct {
	Name string
	IP   string
}

func main() {
	port, prefix, err := parseReceiveArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		printReceiveUsage()
		os.Exit(1)
	}

	if err := runFileReceiver(port, prefix); err != nil {
		fmt.Fprintf(os.Stderr, "%sreceiver error: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func parseReceiveArgs(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		return strconv.Itoa(defaultReceivePort), defaultFilePrefix, nil
	case 1:
		if err := validatePort(args[0]); err != nil {
			return "", "", err
		}
		return args[0], defaultFilePrefix, nil
	case 2:
		if err := validatePort(args[0]); err != nil {
			return "", "", err
		}
		if args[1] == "" {
			return "", "", errors.New("file prefix must not be empty")
		}
		return args[0], args[1], nil
	default:
		return "", "", errors.New("usage: recibe [port] [file-prefix]")
	}
}

func printReceiveUsage() {
	program := filepath.Base(os.Args[0])
	fmt.Printf("%sUsage: %s [port] [file-prefix]%s\n", colorRed, program, colorReset)
	fmt.Printf("%s  Listens on 0.0.0.0:<port> (default 1235) and saves each connection to <file-prefix>N%s\n", colorCyan, colorReset)
	fmt.Printf("%s  Example sender: nc <listener-ip> 1235 < file_to_send%s\n", colorGreen, colorReset)
	fmt.Printf("%s  Example sender with -q 0: nc -q 0 <listener-ip> 1235 < file_send%s\n", colorGreen, colorReset)
	fmt.Printf("%s  Other without -q 0: nc <listener-ip> 1235 < file_send%s\n", colorGreen, colorReset)
}

func runFileReceiver(port, prefix string) error {
	addr := net.JoinHostPort("0.0.0.0", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	fmt.Printf("%sListening for incoming files on %s. Press Ctrl+C to stop.%s\n", colorCyan, addr, colorReset)
	fmt.Printf("%sSend files from another host with:%s\n", colorYellow, colorReset)
	fmt.Printf("%snc -q 0 <listener-ip> %s < file_send%s\n", colorGreen, port, colorReset)
	fmt.Printf("%snc <listener-ip> %s < file_send%s\n", colorGreen, port, colorReset)
	printLocalIPs()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	shutdown := make(chan struct{})

	go func() {
		<-sigc
		fmt.Println("\nSignal received, shutting down...")
		ln.Close()
		close(shutdown)
	}()

	counter := 1
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-shutdown:
				return nil
			default:
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

		filename, err := nextAvailableFilename(prefix, &counter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error picking filename: %v\n", err)
			conn.Close()
			continue
		}

		fmt.Printf("Receiving from %s -> %s\n", conn.RemoteAddr(), filename)
		bytes, err := writeConnectionToFile(conn, filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", filename, err)
			continue
		}
		fmt.Printf("Saved %s (%d bytes).\n", filename, bytes)
	}
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
		fmt.Printf("%s| %-*s | %s", colorCyan, maxIface, addr.Name, colorGreen)
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

func nextAvailableFilename(prefix string, counter *int) (string, error) {
	if *counter <= 0 {
		*counter = 1
	}
	for {
		candidate := fmt.Sprintf("%s%d", prefix, *counter)
		_, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			*counter++
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		*counter++
	}
}

func writeConnectionToFile(conn net.Conn, path string) (int64, error) {
	defer conn.Close()

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, fmt.Errorf("creating directory %q: %w", dir, err)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, conn)
	if err != nil {
		return written, err
	}
	if err := file.Sync(); err != nil {
		return written, err
	}
	return written, nil
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
