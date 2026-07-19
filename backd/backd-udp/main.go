// backd – Monitor TCP/UDP mostrando solo conexiones ACTIVAS (UDP con peer real)
package main

import (
	"fmt"
	"io"
	"log"
	stdnet "net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var logger *log.Logger

func init() {
	// auto-sudo
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		exe, _ = filepath.EvalSymlinks(exe)
		cmd := exec.Command("sudo", append([]string{"-E", exe}, os.Args[1:]...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		_ = cmd.Run()
		os.Exit(0)
	}
	// logger stdout + archivo (sin timestamp automático)
	f, err := os.OpenFile("backd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger = log.New(os.Stdout, "", 0)
	} else {
		mw := io.MultiWriter(os.Stdout, f)
		logger = log.New(mw, "", 0)
	}
}

/* Colores ANSI */
var (
	RED     = "\033[31m"
	GREEN   = "\033[32m"
	YELLOW  = "\033[33m"
	BLUE    = "\033[34m"
	MAGENTA = "\033[35m"
	RESET   = "\033[0m"
)

/* Filtros */
var EXCLUDED = []string{
	"firefox", "chrome", "google-chrome",
	"avahi-daemon", "kdeconnectd", "systemd-resolved", "mdnsd", "mdnsresponder", "dnsmasq",
}
var IGNORE_UDP_PORTS = map[uint32]struct{}{
	5353: {}, // mDNS
	1900: {}, // SSDP
	1716: {}, // KDE Connect
}

/* Utils */
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
func hasPeer(c psnet.ConnectionStat) bool {
	if c.Raddr.Port == 0 {
		return false
	}
	ip := strings.TrimSpace(c.Raddr.IP)
	if ip == "" || ip == "::" || ip == "0.0.0.0" {
		return false
	}
	return true
}

func main() {
	for {
		logger.Printf("\n%s%s%s\n", MAGENTA, strings.Repeat("═", 80), RESET)
		logger.Printf("%sSCAN @ %s%s\n", GREEN, time.Now().Format("2006-01-02 15:04:05"), RESET)
		logger.Printf("%s%s%s\n\n", MAGENTA, strings.Repeat("═", 80), RESET)

		check()
		time.Sleep(5 * time.Second)
	}
}

func check() {
	scanTCP()
	scanUDP()
}

func scanTCP() {
	conns, err := psnet.Connections("tcp")
	if err != nil {
		logger.Printf("%sError TCP: %v%s\n", RED, err, RESET)
		return
	}
	for _, c := range conns {
		if c.Status != "ESTABLISHED" || c.Raddr.IP == "127.0.0.1" {
			continue
		}
		if c.Pid <= 0 {
			continue
		}
		p, err := process.NewProcess(c.Pid)
		if err != nil {
			continue
		}
		name, _ := p.Name()
		if contains(EXCLUDED, strings.ToLower(name)) {
			continue
		}
		logger.Println(GREEN + "[+] TCP active" + RESET)
		printInfo(p, c)
	}
}

func scanUDP() {
	kinds := []string{"udp", "udp4", "udp6"}
	seen := make(map[string]struct{})
	for _, kind := range kinds {
		conns, err := psnet.Connections(kind)
		if err != nil {
			continue
		}
		for _, c := range conns {
			// ignora puertos ruidosos
			if _, ok := IGNORE_UDP_PORTS[c.Laddr.Port]; ok {
				continue
			}
			// solo activas con peer real
			if !hasPeer(c) {
				continue
			}
			if c.Pid <= 0 {
				continue
			}
			p, err := process.NewProcess(c.Pid)
			if err != nil {
				continue
			}
			name, _ := p.Name()
			if contains(EXCLUDED, strings.ToLower(name)) {
				continue
			}
			key := fmt.Sprintf("%s|%d|%s|%d|%s|%d",
				kind, c.Pid, c.Laddr.IP, c.Laddr.Port, c.Raddr.IP, c.Raddr.Port)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			logger.Println(GREEN + "[+] UDP active" + RESET)
			printInfo(p, c)
		}
	}
}

func printInfo(p *process.Process, c psnet.ConnectionStat) {
	name, _ := p.Name()
	pid := p.Pid
	statusArr, _ := p.Status()
	status := strings.Join(statusArr, ",")
	user, _ := p.Username()
	exe, _ := p.Exe()
	cmd, _ := p.Cmdline()

	logger.Printf("%sProcess: %s%s (PID %d)%s\n", YELLOW, BLUE, name, pid, RESET)
	if status != "" {
		logger.Printf("%sStatus: %s%s%s\n", YELLOW, BLUE, status, RESET)
	}
	logger.Printf("%sUser: %s%s%s\n", YELLOW, BLUE, user, RESET)
	logger.Printf("%sExec: %s%s%s\n", YELLOW, BLUE, exe, RESET)
	logger.Printf("%sCmd: %s%s%s\n", YELLOW, BLUE, cmd, RESET)
	logger.Printf("%sLocal: %s%s:%d%s\n", YELLOW, BLUE, c.Laddr.IP, c.Laddr.Port, RESET)

	logger.Printf("%sRemote: %s%s:%d%s\n", YELLOW, BLUE, c.Raddr.IP, c.Raddr.Port, RESET)
	if c.Raddr.IP != "" {
		if host, err := stdnet.LookupAddr(c.Raddr.IP); err == nil && len(host) > 0 {
			logger.Printf("%sRemote Hostname: %s%s%s\n", YELLOW, BLUE, host[0], RESET)
		} else {
			logger.Printf("%sRemote Hostname: %sNo disponible%s\n", YELLOW, RED, RESET)
		}
	}

	logger.Println()
}
