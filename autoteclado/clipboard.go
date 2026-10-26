package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func clipboardText() ([]byte, error) {
	return readClipboardText(runtime.GOOS, func() ([]byte, error) {
		return exec.Command("/usr/bin/pbpaste").Output()
	})
}

func readClipboardText(goos string, read func() ([]byte, error)) ([]byte, error) {
	if goos != "darwin" {
		return nil, fmt.Errorf("-p solo está disponible en macOS; usa -f archivo en este sistema")
	}
	data, err := read()
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el portapapeles de macOS: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("el portapapeles no contiene texto; copia el texto y vuelve a ejecutar con -p")
	}
	return data, nil
}
