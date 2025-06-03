package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type HostEntry struct {
	Name string
	IP   string
}

func parseSSHConfig(path string) []HostEntry {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer file.Close()

	var entries []HostEntry
	scanner := bufio.NewScanner(file)
	var currentHost string
	var skip bool

	reComment := regexp.MustCompile(`#.*`)
	for scanner.Scan() {
		line := strings.TrimSpace(reComment.ReplaceAllString(scanner.Text(), ""))
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		value := parts[1]

		switch key {
		case "host":
			currentHost = value
			entries = append(entries, HostEntry{Name: currentHost, IP: "IP no encontrada"})
			skip = false
		case "hostname":
			if !skip && currentHost != "" {
				for i := range entries {
					if entries[i].Name == currentHost {
						entries[i].IP = value
						skip = true
						break
					}
				}
			}
		}
	}
	return entries
}

func pingHost(ip string) bool {
	cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
	err := cmd.Run()
	return err == nil
}

func main() {
	fast := len(os.Args) > 1 && os.Args[1] == "-f"
	entries := parseSSHConfig(os.Getenv("HOME") + "/.ssh/config")

	fmt.Println("\n\033[1;35m_________________________________________________________\033[0m\n")
	fmt.Println("List of available hosts:")

	var wg sync.WaitGroup
	statusMap := make(map[int]string)
	var mu sync.Mutex

	for i := range entries {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := entries[i].IP
			status := ip
			if !fast && ip != "IP no encontrada" {
				if pingHost(ip) {
					status = fmt.Sprintf("\033[0;31m%s\033[0m (\033[0;32monline\033[0m)", ip)
				} else {
					status = fmt.Sprintf("\033[0;37m%s\033[0m (\033[0;37moffline\033[0m)", ip)
				}
			} else {
				status = fmt.Sprintf("\033[0;33m%s\033[0m", ip)
			}
			mu.Lock()
			statusMap[i] = status
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	keys := make([]int, 0, len(entries))
	for i := range entries {
		keys = append(keys, i)
	}
	sort.Ints(keys)

	for _, i := range keys {
		fmt.Printf("%d. \033[0;34m%s\033[0m - %s\n", i+1, entries[i].Name, statusMap[i])
	}

	fmt.Print("\n\033[1;35m_________________________________________________________\033[0m\n\n")
	fmt.Print("\033[33mEnter the number of the host you want to connect to: \033[0m")
	var choiceStr string
	fmt.Scanln(&choiceStr)

	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(entries) {
		fmt.Println("\033[31mError: Please enter a valid number within the range.\033[0m")
		os.Exit(1)
	}

	chosen := entries[choice-1]
	fmt.Printf("\n\033[33mConnecting to \033[0;34m%s\033[0m (\033[0;33m%s\033[0m)...\033[0m\n\n", chosen.Name, chosen.IP)
	cmd := exec.Command("ssh", chosen.Name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
