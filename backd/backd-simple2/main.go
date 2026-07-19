package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	stdnet "net"
	"strings"
	"time"
	"io"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var logger *log.Logger

func init() {
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		exe, _ = filepath.EvalSymlinks(exe)
		cmd := exec.Command("sudo", append([]string{"-E", exe}, os.Args[1:]...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		_ = cmd.Run()
		os.Exit(0)
	}
	f, err := os.OpenFile("backd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger = log.New(os.Stdout, "", 0)
	} else {
		mw := io.MultiWriter(os.Stdout, f)
		logger = log.New(mw, "", 0) // <<< sin fecha/hora automáticas
	}
}

/* colores ANSI */
var (
	RED     = "\033[31m"
	GREEN   = "\033[32m"
	YELLOW  = "\033[33m"
	BLUE    = "\033[34m"
	MAGENTA = "\033[35m"
	RESET   = "\033[0m"
)

var EXCLUDED = []string{"firefox", "chrome", "google-chrome"}

func main() {
	for {
		// separador visual con timestamp manual
		logger.Printf("\n%s%s%s\n", MAGENTA, strings.Repeat("═", 80), RESET)
		logger.Printf("%sSCAN @ %s%s\n", GREEN, time.Now().Format("2006-01-02 15:04:05"), RESET)
		logger.Printf("%s%s%s\n\n", MAGENTA, strings.Repeat("═", 80), RESET)

		check()
		time.Sleep(5 * time.Second)
	}
}

func check() {
	conns, err := psnet.Connections("inet")
	if err != nil {
		logger.Printf("%sError: %v%s\n", RED, err, RESET)
		return
	}
	for _, c := range conns {
		if c.Status != "ESTABLISHED" || c.Raddr.IP == "127.0.0.1" {
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
		logger.Println(GREEN + "[+] Connection found" + RESET)
		printInfo(p, c)
	}
}

func printInfo(p *process.Process, c psnet.ConnectionStat) {
	name, _ := p.Name()
	pid := p.Pid
	status, _ := p.Status()
	user, _ := p.Username()
	exe, _ := p.Exe()
	cmd, _ := p.Cmdline()

	logger.Printf("%sProcess: %s%s (PID %d)%s\n", YELLOW, BLUE, name, pid, RESET)
	logger.Printf("%sStatus: %s%s%s\n", YELLOW, BLUE, status, RESET)
	logger.Printf("%sUser: %s%s%s\n", YELLOW, BLUE, user, RESET)
	logger.Printf("%sExec: %s%s%s\n", YELLOW, BLUE, exe, RESET)
	logger.Printf("%sCmd: %s%s%s\n", YELLOW, BLUE, cmd, RESET)
	logger.Printf("%sLocal: %s%s:%d%s\n", YELLOW, BLUE, c.Laddr.IP, c.Laddr.Port, RESET)
	logger.Printf("%sRemote: %s%s:%d%s\n", YELLOW, BLUE, c.Raddr.IP, c.Raddr.Port, RESET)

	if host, err := stdnet.LookupAddr(c.Raddr.IP); err == nil && len(host) > 0 {
		logger.Printf("%sRemote Hostname: %s%s%s\n", YELLOW, BLUE, host[0], RESET)
	} else {
		logger.Printf("%sRemote Hostname: %sNo disponible%s\n", YELLOW, RED, RESET)
	}

	// <<< espacio vacío entre conexiones
	logger.Println()
}


func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
