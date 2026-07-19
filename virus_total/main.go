// vt_ip_check.go
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
	vtURLTmpl  = "https://www.virustotal.com/api/v3/ip_addresses/%s"
	geoURLTmpl = "http://ip-api.com/json/%s"
	sleepVT    = 15 * time.Second
)

type vtResp struct {
	Data struct {
		Attributes struct {
			LastAnalysisStats struct {
				Malicious   int `json:"malicious"`
				Suspicious  int `json:"suspicious"`
				Harmless    int `json:"harmless"`
				Undetected  int `json:"undetected"`
				Timeout     int `json:"timeout"`
				Confirmed   int `json:"confirmed-timeout"`
				Failure     int `json:"failure"`
				TypeUnsupported int `json:"type-unsupported"`
			} `json:"last_analysis_stats"`
		} `json:"attributes"`
	} `json:"data"`
}

type geoResp struct {
	Status  string `json:"status"`
	City    string `json:"city"`
	Country string `json:"country"`
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
	}
}

func getGeo(client *http.Client, ip string) (city, country string) {
	req, err := http.NewRequest("GET", fmt.Sprintf(geoURLTmpl, ip), nil)
	if err != nil {
		return "Desconocida", "Desconocido"
	}

	resp, err := client.Do(req)
	if err != nil {
		return "Desconocida", "Desconocido"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "Desconocida", "Desconocido"
	}

	var g geoResp
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return "Desconocida", "Desconocido"
	}

	if strings.ToLower(g.Status) != "success" {
		return "Desconocida", "Desconocido"
	}

	if g.City == "" {
		g.City = "Desconocida"
	}
	if g.Country == "" {
		g.Country = "Desconocido"
	}
	return g.City, g.Country
}

func vtLookup(client *http.Client, apiKey, ip string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf(vtURLTmpl, ip), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-apikey", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("%s → Error %d\n", ip, resp.StatusCode)
		return nil
	}

	var v vtResp
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}

	stats := v.Data.Attributes.LastAnalysisStats
	city, country := getGeo(client, ip)

	fmt.Printf("%s → %d maliciosos / %d sospechosos → %s, %s\n",
		ip, stats.Malicious, stats.Suspicious, city, country)
	return nil
}

func looksLikeFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".txt")
}

func isValidIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

func usage() {
	fmt.Println("Uso: vt_ip_check [IP | archivo.txt]")
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

	arg := os.Args[1]
	client := httpClient()

	if looksLikeFile(arg) {
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

			if err := vtLookup(client, apiKey, ip); err != nil {
				fmt.Printf("%s → Error: %v\n", ip, err)
			}
			time.Sleep(sleepVT) // respetar límite de VT
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

	if err := vtLookup(client, apiKey, arg); err != nil {
		fmt.Printf("%s → Error: %v\n", arg, err)
		os.Exit(1)
	}
}
