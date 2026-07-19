// vt_ip_light.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	vtURLTmpl = "https://www.virustotal.com/api/v3/ip_addresses/%s"
	sleepVT   = 15 * time.Second
)

func usage() { fmt.Println("Uso: vt_ip_light [IP | archivo.txt]") }

func isValidIP(s string) bool { return net.ParseIP(strings.TrimSpace(s)) != nil }

func vtLookup(client *http.Client, apiKey, ip string) {
	req, err := http.NewRequest("GET", fmt.Sprintf(vtURLTmpl, ip), nil)
	if err != nil {
		fmt.Printf("%s → Error: %v\n", ip, err)
		return
	}
	req.Header.Set("x-apikey", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s → Error: %v\n", ip, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("%s → Error %d\n", ip, resp.StatusCode)
		return
	}

	// Parse minimal fields only
	var root map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		fmt.Printf("%s → Error: %v\n", ip, err)
		return
	}

	getInt := func(m map[string]any, k string) int {
		if v, ok := m[k]; ok {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
		return 0
	}

	data, _ := root["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	stats, _ := attrs["last_analysis_stats"].(map[string]any)

	fmt.Printf("%s → %d maliciosos / %d sospechosos\n",
		ip, getInt(stats, "malicious"), getInt(stats, "suspicious"))
}

func main() {
	apiKey := os.Getenv("VT_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: la variable de entorno VT_API_KEY no está definida.")
		os.Exit(1)
	}
	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	arg := os.Args[1]

	if strings.HasSuffix(strings.ToLower(arg), ".txt") {
		f, err := os.Open(arg)
		if err != nil {
			fmt.Printf("Error abriendo archivo: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		for sc.Scan() {
			ip := strings.TrimSpace(sc.Text())
			if ip == "" {
				continue
			}
			if !isValidIP(ip) {
				fmt.Printf("%s → IP inválida\n", ip)
				continue
			}
			vtLookup(client, apiKey, ip)
			time.Sleep(sleepVT)
		}
		if err := sc.Err(); err != nil {
			fmt.Printf("Error leyendo archivo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if !isValidIP(arg) {
		fmt.Printf("%s → IP inválida\n", arg)
		os.Exit(1)
	}
	vtLookup(client, apiKey, arg)
}
