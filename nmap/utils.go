package main

import (
	"bytes"
	"os/exec"
	"strings"
)

// Ejecuta un comando y retorna su salida
func run(cmd string, args ...string) (string, error) {
	command := exec.Command(cmd, args...)
	var out bytes.Buffer
	command.Stdout = &out
	command.Stderr = &out
	err := command.Run()
	return out.String(), err
}

// Obtiene la primera línea no vacía de una cadena
func getFirstLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}