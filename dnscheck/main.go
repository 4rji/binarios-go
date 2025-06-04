package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func line() {
	fmt.Println("_________________________________________________________\n")
}

func checkQuad9HTTP() {
	fmt.Println("[+]  Checando salida HTTP (on.quad9.net)...")
	resp, err := http.Get("https://on.quad9.net/")
	if err != nil {
		fmt.Println("\033[1;31m❌  Error al conectar con on.quad9.net\033[0m\n")
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.Contains(strings.ToLower(scanner.Text()), "yes") {
			fmt.Println("\033[1;32m✅  Estás usando Quad9 (según on.quad9.net)\033[0m\n")
			return
		}
	}
	fmt.Println("\033[1;31m❌  No estás usando Quad9 (según on.quad9.net)\033[0m\n")
}

func dig(query string, server string) string {
	out, err := exec.Command("dig", "+short", query, "@"+server).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func digVersionBind(server string) string {
	out, err := exec.Command("dig", "@"+server, "version.bind", "txt", "chaos", "+short").Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "\"")
}

func getLocalResolver() string {
	out, err := exec.Command("dig", "google.com").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, ";; SERVER:") {
			return strings.TrimSpace(strings.Split(line, ":")[1])
		}
	}
	return ""
}

func analyzeASN(ip string) (bool, string) {
	out, err := exec.Command("whois", ip).Output()
	if err != nil {
		return false, ""
	}
	lines := strings.Split(string(out), "\n")
	summary := []string{}
	isQuad9 := false

	for _, line := range lines {
		l := strings.ToLower(line)
		if strings.Contains(l, "quad9") || strings.Contains(l, "as19281") {
			isQuad9 = true
		}
		if strings.Contains(l, "orgname") ||
			strings.Contains(l, "netname") ||
			strings.Contains(l, "descr") ||
			strings.Contains(l, "origin") ||
			strings.Contains(l, "owner") {
			summary = append(summary, "   "+line)
		}
	}
	fmt.Println("[+]  Información WHOIS de " + ip + ":")
	for _, s := range summary {
		fmt.Println(s)
	}
	return isQuad9, strings.Join(summary, "\n")
}

func main() {
	line()

	checkQuad9HTTP()
	line()

	fmt.Println("[+]  Proband​o consulta directa a Quad9 (whoami.quad9.net)...")
	ip := dig("whoami.quad9.net", "9.9.9.9")
	if ip != "" {
		fmt.Printf("\033[1;32m✅  Consulta directa llegó. IP pública: %s\033[0m\n\n", ip)
	} else {
		fmt.Println("\033[1;33m⚠️   No hubo respuesta de whoami.quad9.net\033[0m\n")
	}
	line()

	fmt.Println("[+]  Consultando versión del nodo Quad9...")
	version := digVersionBind("9.9.9.9")
	if version != "" {
		fmt.Printf("\033[1;32m✅  Nodo respondió con versión: %s\033[0m\n\n", version)
	} else {
		fmt.Println("\033[1;33m⚠️   Nodo no respondió a version.bind\033[0m\n")
	}
	line()

	fmt.Println("[+]  Servidor DNS usado por consultas locales...")
	localDNS := getLocalResolver()
	if localDNS != "" {
		fmt.Printf("🧩  Resolver local en uso: %s\n\n", localDNS)
	} else {
		fmt.Println("\033[1;33m⚠️   No se detectó servidor DNS local\033[0m\n")
	}
	line()

	fmt.Println("[+]  Detectando IP pública usada por el resolver...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resolver := net.Resolver{PreferGo: true}
	publicIPs, err := resolver.LookupHost(ctx, "whoami.akamai.net")
	if err != nil || len(publicIPs) == 0 {
		fmt.Println("\033[1;33m⚠️   No se pudo obtener la IP externa del resolver\033[0m\n")
	} else {
		ip := publicIPs[0]
		fmt.Printf("\033[1;32m🌐  IP externa detectada: %s\033[0m\n\n", ip)

		isQuad9, _ := analyzeASN(ip)

		if isQuad9 {
			fmt.Println("\033[1;32m✅  Resolver DNS detectado pertenece a Quad9 directamente.\033[0m\n")
		} else {
			fmt.Println("\033[1;33m⚠️   Resolver DNS detectado NO pertenece directamente a Quad9.\033[0m\n")
		}

		// Elegir binario según arquitectura
		var locipBin string
		if runtime.GOARCH == "arm64" {
			locipBin = "locipm"
		} else {
			locipBin = "locip"
		}

		fmt.Printf("[+]  Ejecutando %s -i %s...\n\n", locipBin, ip)

		locipOut, err := exec.Command(locipBin, "-i", ip).CombinedOutput()
		if err != nil {
			fmt.Printf("   Error ejecutando %s: %v\n", locipBin, err)
		} else {
			fmt.Println(string(locipOut))
		}
	}

	line()
}
