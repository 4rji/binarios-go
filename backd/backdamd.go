// backd – Monitor de conexiones con auto-sudo y bucle infinito
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdnet "net"
	"strings"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

/* auto-sudo */
func init() {
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		exe, _ = filepath.EvalSymlinks(exe)
		cmd := exec.Command("sudo", append([]string{"-E", exe}, os.Args[1:]...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		_ = cmd.Run()
		os.Exit(0)
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

/* procesos a ignorar */
var EXCLUDED = []string{"firefox", "chrome", "google-chrome"}

func main() {
	for {
		check()
		time.Sleep(5 * time.Second)
	}
}

func check() {
	conns, err := psnet.Connections("inet")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", RED, err, RESET)
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
		fmt.Println(MAGENTA + strings.Repeat("=", 50) + RESET)
		fmt.Println(GREEN + "[+] Connection found" + RESET)
		printInfo(p, c)
		fmt.Println(MAGENTA + strings.Repeat("=", 50) + RESET)
	}
}

func printInfo(p *process.Process, c psnet.ConnectionStat) {
	name, _ := p.Name()
	pid := p.Pid
	status, _ := p.Status()
	user, _ := p.Username()
	exe, _ := p.Exe()
	cmd, _ := p.Cmdline()

	fmt.Printf("%s[+] Process Name: %s%s%s\n", YELLOW, BLUE, name, RESET)
	fmt.Printf("%s[+] Process PID: %s%d%s\n", YELLOW, BLUE, pid, RESET)
	fmt.Printf("%s[+] Process Status: %s%s%s\n", YELLOW, BLUE, status, RESET)
	fmt.Printf("%s[+] User: %s%s%s\n", YELLOW, BLUE, user, RESET)
	fmt.Printf("%s[+] Executable Path: %s%s%s\n", YELLOW, BLUE, exe, RESET)
	fmt.Printf("%s[+] Command Line: %s%s%s\n", YELLOW, BLUE, cmd, RESET)
	fmt.Printf("%s[+] Local Address: %s%s:%d%s\n", YELLOW, BLUE, c.Laddr.IP, c.Laddr.Port, RESET)
	fmt.Printf("%s[+] Remote Address: %s%s:%d%s\n", YELLOW, BLUE, c.Raddr.IP, c.Raddr.Port, RESET)

	if host, err := stdnet.LookupAddr(c.Raddr.IP); err == nil && len(host) > 0 {
		fmt.Printf("%s[+] Remote Hostname: %s%s%s\n", YELLOW, BLUE, host[0], RESET)
	} else {
		fmt.Printf("%s[+] Remote Hostname: %sNo disponible%s\n", YELLOW, RED, RESET)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
