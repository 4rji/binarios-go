package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"runtime"
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
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <local_ip>\n", os.Args[0])
		os.Exit(1)
	}
	host := os.Args[1]
	ip := net.ParseIP(host).To4()
	if ip == nil {
		fmt.Fprintln(os.Stderr, "invalid IPv4:", host)
		os.Exit(1)
	}

	// Match the Python behavior:
	// - Windows: IPPROTO_IP + RCVALL -> capture all IPv4 protocols
	// - Others: IPPROTO_ICMP -> only ICMP
	proto := syscall.IPPROTO_ICMP
	if runtime.GOOS == "windows" {
		proto = syscall.IPPROTO_IP
	}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, proto)
	if err != nil {
		fmt.Fprintln(os.Stderr, "socket:", err)
		os.Exit(1)
	}
	defer syscall.Close(fd)

	sa := &syscall.SockaddrInet4{Port: 0}
	copy(sa.Addr[:], ip)

	if err := syscall.Bind(fd, sa); err != nil {
		fmt.Fprintln(os.Stderr, "bind:", err)
		os.Exit(1)
	}

	// IP_HDRINCL like the Python script (harmless here; we parse header anyway)
	_ = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)

	// Windows promiscuous mode: SIO_RCVALL / RCVALL_ON
	if runtime.GOOS == "windows" {
		var bytesReturned uint32
		in := uint32(1) // RCVALL_ON
		// SIO_RCVALL = 0x98000001
		if err := syscall.WSAIoctl(
			syscall.Handle(fd),
			0x98000001,
			(*byte)(unsafePointer(&in)),
			uint32(4),
			nil,
			0,
			&bytesReturned,
			nil,
			0,
		); err != nil {
			fmt.Fprintln(os.Stderr, "WSAIoctl SIO_RCVALL:", err)
			os.Exit(1)
		}
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

		// IPv4 header: protocol @ byte 9, src 12:16, dst 16:20
		protoNum := buf[9]
		src := net.IP(buf[12:16]).String()
		dst := net.IP(buf[16:20]).String()

		// quick sanity: version should be 4
		v := buf[0] >> 4
		if v != 4 {
			continue
		}

		// total length (optional)
		totalLen := binary.BigEndian.Uint16(buf[2:4])

		fmt.Printf("Protocol: %s %s -> %s (len=%d)\n", protoName(protoNum), src, dst, totalLen)
	}
}

// minimal unsafe helper to avoid importing unsafe everywhere in main logic
func unsafePointer(p *uint32) unsafe.Pointer { return unsafe.Pointer(p) }
