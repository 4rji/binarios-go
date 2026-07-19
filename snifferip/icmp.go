package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

func protoName(p byte) string {
	switch p {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	default:
		return fmt.Sprintf("%d", p)
	}
}

func main() {
	// Raw ICMP socket (Linux/macOS). Requires root or cap_net_raw.
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "socket:", err)
		os.Exit(1)
	}
	defer syscall.Close(fd)

	// Bind to 0.0.0.0 (all local interfaces)
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Port: 0}); err != nil {
		fmt.Fprintln(os.Stderr, "bind:", err)
		os.Exit(1)
	}

	buf := make([]byte, 65535)
	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "recvfrom:", err)
			continue
		}
		if n < 20 {
			continue
		}

		proto := buf[9]
		src := net.IP(buf[12:16]).String()
		dst := net.IP(buf[16:20]).String()

		fmt.Printf("Protocol: %s %s -> %s\n", protoName(proto), src, dst)
	}
}
