package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type HostConfig struct {
	HostName string
	Port     string
}

func main() {
	usr, _ := user.Current()
	configPath := filepath.Join(usr.HomeDir, ".ssh", "config")
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println("No ~/.ssh/config encontrado")
		return
	}
	defer file.Close()

	hosts := map[string]*HostConfig{}
	var currentHosts []string

	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`(?i)^\s*(host|hostname|port)\s+(.+)$`)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		key := strings.ToLower(matches[1])
		val := strings.TrimSpace(matches[2])

		switch key {
		case "host":
			currentHosts = strings.Fields(val)
			for _, h := range currentHosts {
				if _, ok := hosts[h]; !ok {
					hosts[h] = &HostConfig{HostName: h, Port: "22"}
				}
			}
		case "hostname":
			for _, h := range currentHosts {
				hosts[h].HostName = val
			}
		case "port":
			for _, h := range currentHosts {
				hosts[h].Port = val
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := []string{}

	for name, cfg := range hosts {
		wg.Add(1)
		go func(name string, cfg *HostConfig) {
			defer wg.Done()
			address := net.JoinHostPort(cfg.HostName, cfg.Port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			elapsed := time.Since(start)
			status := ""
			lat := ""
			if err == nil {
				conn.Close()
				status = "\033[32monline\033[0m"
				lat = fmt.Sprintf(" \033[33m(%dms)\033[0m", elapsed.Milliseconds())
			}
			line := fmt.Sprintf("\033[34m%s\033[0m --- %s:%s --- %s%s", name, cfg.HostName, cfg.Port, status, lat)
			mu.Lock()
			results = append(results, line)
			mu.Unlock()
		}(name, cfg)
	}
	wg.Wait()

	sort.Strings(results)
	fmt.Println("\n=== SSH Host Status ===\n")
	for _, r := range results {
		fmt.Println(r)
	}
	fmt.Println("\n========================")
}