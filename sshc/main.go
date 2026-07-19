// massssh.go – salida limpia, verde OK / rojo offline
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const (
	green = "\033[32m"
	red   = "\033[31m"
	reset = "\033[0m"
)

func hosts() []string {
	f, err := os.Open(os.Getenv("HOME") + "/.ssh/config")
	if err != nil {
		return nil
	}
	defer f.Close()

	var list []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(t, "Host ") {
			h := strings.Fields(t)[1]
			if !strings.ContainsAny(h, "*?") {
				list = append(list, h)
			}
		}
	}
	return list
}

func run(host string, cmd []string, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()
	sem <- struct{}{}
	defer func() { <-sem }()

	sshArgs := []string{
		"-o", "BatchMode=yes", // no password prompts
		"-o", "StrictHostKeyChecking=no", // auto-aceptar huella
		"-o", "UserKnownHostsFile=/dev/null", // no guardar
		"-o", "ConnectTimeout=5",
		host,
	}
	sshArgs = append(sshArgs, cmd...)

	out, err := exec.Command("ssh", sshArgs...).CombinedOutput()
	if err != nil {
		fmt.Printf("%s%s offline%s\n", red, host, reset)
		return
	}
	fmt.Printf("%s%s%s\n%s\n", green, host, reset, strings.TrimSpace(string(out)))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: sshc <comando>")
		return
	}
	cmd := os.Args[1:]

	sem := make(chan struct{}, 10) // concurrencia máx.
	var wg sync.WaitGroup
	for _, h := range hosts() {
		wg.Add(1)
		go run(h, cmd, &wg, sem)
	}
	wg.Wait()
}
