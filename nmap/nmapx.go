package main

import (
	"fmt"
	"os"
)

func main() {
	// Check for root privileges
	if os.Geteuid() != 0 {
		binPath, err := os.Executable()
		if err != nil {
			binPath = os.Args[0]
		}
		fmt.Printf("\n[!] This program must be run as root.\n")
		fmt.Printf("    Please run: sudo %s <target>\n\n", binPath)
		os.Exit(1)
	}

	// Verificar argumentos
	if len(os.Args) < 2 {
		fmt.Println("\nUsage: go run nmapx.go utils.go ui.go monitor.go scanner.go report.go <target>")
		fmt.Println("\n  <target> specifies what to scan. It can be:")
		fmt.Println("    - A single IP address (e.g. 192.168.1.10)")
		fmt.Println("    - A CIDR range (e.g. 192.168.1.0/24)")
		fmt.Println("    - A hostname (e.g. example.com)")
		fmt.Println("\nExamples:")
		fmt.Println("  go run nmapx.go utils.go ui.go monitor.go scanner.go report.go 192.168.1.0/24")
		fmt.Println("  go run nmapx.go utils.go ui.go monitor.go scanner.go report.go 10.0.0.5")
		fmt.Println("  go run nmapx.go utils.go ui.go monitor.go scanner.go report.go example.com")
		os.Exit(1)
	}

	// Configurar la aplicación
	state := setupUI()
	state.target = os.Args[1]

	// Iniciar monitores
	startProcessMonitor(state)
	startPortsFileMonitor(state)
	setupOutputRedirection(state)

	// Iniciar escaneo
	startScan(state)

	// Iniciar la UI
	if err := state.app.SetRoot(state.flex, true).Run(); err != nil {
		panic(err)
	}
}
