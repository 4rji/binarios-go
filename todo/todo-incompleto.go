package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// Definición de colores ANSI
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	Bold        = "\033[1m"
)

type Script struct {
	Name string
	Desc string
}

func parseReadme(filename string) ([]Script, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var scripts []Script
	scanner := bufio.NewScanner(file)
	reCategory := regexp.MustCompile(`^#+\s*.*`)
	for scanner.Scan() {
		line := scanner.Text()
		if !reCategory.MatchString(line) && strings.TrimSpace(line) != "" {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				script := parts[0]
				desc := ""
				if len(parts) > 1 {
					desc = strings.Join(parts[1:], " ")
				}
				scripts = append(scripts, Script{Name: script, Desc: desc})
			}
		}
	}
	return scripts, scanner.Err()
}

func printSeparator() {
	fmt.Printf("\n%s%s%s\n", ColorCyan, strings.Repeat("=", 80), ColorReset)
}

func formatScriptList(scripts []Script) []string {
	var choices []string
	maxNameLength := 0
	for _, s := range scripts {
		if len(s.Name) > maxNameLength {
			maxNameLength = len(s.Name)
		}
	}

	for _, s := range scripts {
		// Formatear en columnas con padding
		padding := strings.Repeat(" ", maxNameLength-len(s.Name))
		choices = append(choices, fmt.Sprintf("%s%s%s%s | %s", ColorRed, s.Name, padding, ColorReset, s.Desc))
	}
	return choices
}

func main() {
	scripts, err := parseReadme("README.md")
	if err != nil {
		fmt.Printf("%sError al leer README.md: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if len(scripts) == 0 {
		fmt.Printf("%sNo se encontraron scripts en el README%s\n", ColorYellow, ColorReset)
		return
	}

	scriptChoices := formatScriptList(scripts)

	for {
		printSeparator()
		fmt.Printf("%s%sBienvenido al selector de scripts%s%s\n", Bold, ColorBlue, ColorReset, ColorReset)
		printSeparator()

		var selectedScript string
		promptScript := &survey.Select{
			Message: fmt.Sprintf("%s%sBusca un script:%s", Bold, ColorBlue, ColorReset),
			Options: scriptChoices,
			PageSize: 14, // Mostrar 14 líneas en la búsqueda
		}
		
		err = survey.AskOne(promptScript, &selectedScript, survey.WithFilter(func(filter string, value string, index int) bool {
			// Remover los códigos de color para la búsqueda
			cleanValue := strings.ReplaceAll(value, ColorRed, "")
			cleanValue = strings.ReplaceAll(cleanValue, ColorReset, "")
			cleanValue = strings.ReplaceAll(cleanValue, " | ", " ") // Remover el separador de columnas
			return strings.Contains(strings.ToLower(cleanValue), strings.ToLower(filter))
		}))
		
		if err != nil {
			fmt.Printf("%sError al seleccionar el script: %v%s\n", ColorRed, err, ColorReset)
			return
		}

		// Limpiar los códigos de color y el formato de columnas para el procesamiento
		cleanSelected := strings.ReplaceAll(selectedScript, ColorRed, "")
		cleanSelected = strings.ReplaceAll(cleanSelected, ColorReset, "")
		parts := strings.SplitN(cleanSelected, " | ", 2)
		if len(parts) < 2 {
			fmt.Printf("%sSelección inválida%s\n", ColorRed, ColorReset)
			continue
		}

		// Limpiar pantalla (terminales compatibles)
		fmt.Print("\033[H\033[2J")
		printSeparator()
		fmt.Printf("%s%sPreview del script seleccionado:%s%s\n", Bold, ColorPurple, ColorReset, ColorReset)
		printSeparator()
		fmt.Printf("\n%sScript:%s %s%s%s\n", ColorYellow, ColorReset, ColorRed, strings.TrimSpace(parts[0]), ColorReset)
		fmt.Printf("%sDescripción:%s %s%s%s\n", ColorYellow, ColorReset, ColorWhite, strings.TrimSpace(parts[1]), ColorReset)
		fmt.Printf("%sEjemplo de ejecución:%s %s./%s%s\n\n", ColorYellow, ColorReset, ColorWhite, strings.TrimSpace(parts[0]), ColorReset)
		printSeparator()

		var confirm bool
		promptConfirm := &survey.Confirm{
			Message: fmt.Sprintf("%s¿Confirmar selección?%s", ColorBlue, ColorReset),
			Default: false,
		}
		if err = survey.AskOne(promptConfirm, &confirm); err != nil {
			fmt.Printf("%sError al confirmar selección: %v%s\n", ColorRed, err, ColorReset)
			return
		}
		if confirm {
			fmt.Printf("\n%sScript confirmado: %s%s%s\n", ColorGreen, ColorRed, strings.TrimSpace(parts[0]), ColorReset)
			printSeparator()
			break
		}
	}
}
