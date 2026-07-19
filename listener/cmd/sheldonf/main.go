package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const (
	colorRed     = "\033[91m"
	colorMagenta = "\033[95m"
	colorGreen   = "\033[92m"
	colorCyan    = "\033[96m"
	colorReset   = "\033[0m"
)

func main() {
	if len(os.Args) != 4 {
		printUsage()
		os.Exit(1)
	}

	ip := os.Args[1]
	port := os.Args[2]
	fileToSend := os.Args[3]

	if err := validatePort(port); err != nil {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	if err := sendFileOverTCP(ip, port, fileToSend); err != nil {
		fmt.Fprintf(os.Stderr, "%s%v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func printUsage() {
	program := filepath.Base(os.Args[0])
	fmt.Printf("%sUsage: %s <IP> <port> <file>%s\n", colorRed, program, colorReset)
	fmt.Printf("%sOn the other machine, run: nc -l -p 1235 > file%s\n", colorMagenta, colorReset)
	fmt.Printf("%sTo execute commands, place the command in a file and run%s\n", colorGreen, colorReset)
	fmt.Printf("%snc -l -p 1234 | /bin/bash%s\n", colorGreen, colorReset)
}

func validatePort(port string) error {
	value, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port %q", port)
	}
	if value <= 0 || value > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func sendFileOverTCP(ip, port, filePath string) error {
	addr := net.JoinHostPort(ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error establishing connection: %w", err)
	}
	defer conn.Close()

	fmt.Printf("%sConnection established with %s on port %s.%s\n", colorCyan, ip, port, colorReset)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Error opening file %q: %w", filePath, err)
	}
	defer file.Close()

	if _, err := io.Copy(conn, file); err != nil {
		return fmt.Errorf("Error sending the file: %w", err)
	}

	fmt.Printf("%sFile sent successfully.%s\n", colorGreen, colorReset)
	return nil
}
