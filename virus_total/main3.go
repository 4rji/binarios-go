// vt_curl.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	vtURLTmpl = "https://www.virustotal.com/api/v3/ip_addresses/%s"
	sleepVT   = 15 * time.Second
)

func usage() { fmt.Println("Uso: vt_curl [IP | archivo.txt]") }

func vtLookup(apiKey, ip string) {
	cmd := exec.Command("curl", "-sS",
		"-H", "x-apikey: "+apiKey,
		fmt.Sprintf(vtURLTmpl, ip),
	)

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s → Error: %v\n", ip, err)
		return
	}

	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
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
		fmt.Println("Error: VT_API_KEY no definida.")
		os.Exit(1)
	}
	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}

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
			vtLookup(apiKey, ip)
			time.Sleep(sleepVT)
		}
		if err := sc.Err(); err != nil {
			fmt.Printf("Error leyendo archivo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	vtLookup(apiKey, strings.TrimSpace(arg))
}
