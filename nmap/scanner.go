package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// Detecta si el target es un dominio (no IP ni CIDR)
func isDomain(target string) bool {
	if strings.Contains(target, "/") {
		return false // CIDR
	}
	parts := strings.Split(target, ".")
	if len(parts) < 2 {
		return false // No es dominio
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	// Si no es todo numérico, probablemente es dominio
	if _, err := os.Stat(target); err == nil {
		return false // Es un archivo
	}
	for _, c := range target {
		if (c < '0' || c > '9') && c != '.' {
			return true
		}
	}
	return false
}

// Realiza la fase de descubrimiento de hosts
func performHostDiscovery(state *AppState) {
	fmt.Println("\033[1;34m[1] Host discovery\033[0m")
	fmt.Printf("DEBUG: Target before nmap: %s\n", state.target)

	if isDomain(state.target) {
		fmt.Println("[!] Target appears to be a domain. Skipping ping sweep and using domain directly.")
		hf, _ := os.Create(state.scanDir + "/hosts.txt")
		defer hf.Close()
		hf.WriteString(state.target + "\n")
		return
	}

	run("nmap", "-sn", state.target, "-oG", state.scanDir+"/pingsweep.gnmap")

	f, _ := os.Open(state.scanDir + "/pingsweep.gnmap")
	defer f.Close()
	hf, _ := os.Create(state.scanDir + "/hosts.txt")
	defer hf.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "Up") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				hf.WriteString(parts[1] + "\n")
			}
		}
	}
}

// Realiza el escaneo de puertos
func performPortScan(state *AppState) {
	fmt.Println("\033[1;34m[2] Port scan (fast mode)\033[0m")
	run("nmap", "-sS", "-sV", "-T4", "--top-ports", "1000", "-iL",
		state.scanDir+"/hosts.txt", "-oN", state.scanDir+"/ports.nmap")
}

// Inicia el proceso de escaneo
func startScan(state *AppState) {
	go func() {
		// Configurar directorios de salida
		ts := time.Now().Format("20060102_150405")
		state.scanDir = "test_" + ts
		os.MkdirAll(state.scanDir, 0755)
		state.htmlPath = state.scanDir + "/report.html"

		// Obtener información de red
		hostIP, _ := run("sh", "-c", `ifconfig en0 | grep "inet " | awk '{print $2}'`) // Assuming en0 is the primary interface
		gateway, _ := run("sh", "-c", `route -n get default | grep gateway | awk '{print $2}'`)

		// Realizar escaneo
		performHostDiscovery(state)
		performPortScan(state)

		// Generar reporte
		hostsData, _ := ioutil.ReadFile(state.scanDir + "/hosts.txt")
		portsData, _ := ioutil.ReadFile(state.scanDir + "/ports.nmap")
		htmlContent := generateHTMLReport(state, hostIP, gateway, hostsData, portsData)
		ioutil.WriteFile(state.htmlPath, []byte(htmlContent), 0644)

		// Mostrar popup con resultados
		showCompletionPopup(state)
	}()
}
