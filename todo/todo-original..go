package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// ANSI color definitions
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

type DetailedDescription struct {
	Name         string `json:"name"`
	ShortDesc    string `json:"short_desc"`
	DetailedDesc string `json:"detailed_desc"`
}

type Descriptions map[string]DetailedDescription

func loadDescriptions() (Descriptions, error) {
	file, err := os.Open("/opt/4rji/bin/descriptions.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var descriptions Descriptions
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&descriptions); err != nil {
		return nil, err
	}

	return descriptions, nil
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
		padding := strings.Repeat(" ", maxNameLength-len(s.Name))
		choices = append(choices, fmt.Sprintf("%s%s%s%s | %s", ColorRed, s.Name, padding, ColorReset, s.Desc))
	}
	return choices
}

func getScriptName(selectedScript string) string {
	cleanSelected := strings.ReplaceAll(selectedScript, ColorRed, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, ColorReset, "")
	parts := strings.SplitN(cleanSelected, " | ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

// Función showImage corregida: usa "chafa" para mostrar imágenes.
func showImage(scriptName string) bool {
	imgPath := fmt.Sprintf("/opt/4rji/img-bin/%s", scriptName)
	
	// Try WebP first
	if _, err := os.Stat(imgPath + ".webp"); err == nil {
		cmd := exec.Command("chafa", "--size", "80x40", imgPath+".webp")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%sError displaying image: %v%s\n", ColorRed, err, ColorReset)
			return false
		}
		fmt.Printf("%s%s%s\n", ColorCyan, string(output), ColorReset)
		return true
	}

	// Try PNG if WebP doesn't exist
	if _, err := os.Stat(imgPath + ".png"); err == nil {
		cmd := exec.Command("chafa", "--size", "80x40", imgPath+".png")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%sError displaying image: %v%s\n", ColorRed, err, ColorReset)
			return false
		}
		fmt.Printf("%s%s%s\n", ColorCyan, string(output), ColorReset)
		return true
	}

	return false
}

func showDetailedDescription(scriptName string, descriptions Descriptions) {
	if desc, ok := descriptions[scriptName]; ok {
		printSeparator()
		fmt.Printf("%s%sDetailed script description:%s%s\n", Bold, ColorPurple, ColorReset, ColorReset)
		printSeparator()
		fmt.Printf("\n%sScript:%s %s%s%s\n", ColorYellow, ColorReset, ColorRed, scriptName, ColorReset)
		fmt.Printf("%sShort description:%s %s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.ShortDesc, ColorReset)
		fmt.Printf("\n%sDetailed description:%s\n%s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.DetailedDesc, ColorReset)
	} else {
		showImage(scriptName)
	}
}

func main() {
	// Limpiar pantalla al iniciar
	fmt.Print("\033[H\033[2J")

	descriptions, err := loadDescriptions()
	if err != nil {
		fmt.Printf("%sWarning: Could not load descriptions file: %v%s\n", ColorYellow, err, ColorReset)
	}

	scripts, err := parseReadme("/opt/4rji/bin/README.md")
	if err != nil {
		fmt.Printf("%sError reading README.md: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if len(scripts) == 0 {
		fmt.Printf("%sNo scripts found in README%s\n", ColorYellow, ColorReset)
		return
	}

	if len(os.Args) > 1 {
		searchTerm := strings.Join(os.Args[1:], " ")
		var filteredScripts []Script
		for _, s := range scripts {
			if strings.Contains(strings.ToLower(s.Name), strings.ToLower(searchTerm)) || strings.Contains(strings.ToLower(s.Desc), strings.ToLower(searchTerm)) {
				filteredScripts = append(filteredScripts, s)
			}
		}
		if len(filteredScripts) == 0 {
			fmt.Printf("%sNo se encontró ningún script que coincida con '%s'%s\n", ColorYellow, searchTerm, ColorReset)
			return
		} else if len(filteredScripts) == 1 {
			showDetailedDescription(filteredScripts[0].Name, descriptions)
			fmt.Printf("%sPress Enter to exit...%s", ColorBlue, ColorReset)
			reader := bufio.NewReader(os.Stdin)
			_, _ = reader.ReadString('\n')
			return
		} else {
			scripts = filteredScripts
		}
	}

	scriptChoices := formatScriptList(scripts)

	for {
		printSeparator()
		fmt.Printf("%s%s     ***  4rji script selector   ***   %s%s\n", Bold, ColorGreen, ColorReset, ColorReset)
		fmt.Printf("%s%s%s", Bold, ColorBlue, ColorReset)
		printSeparator()

		var selectedScript string
		promptScript := &survey.Select{
			Message:  "",
			Options:  scriptChoices,
			PageSize: 40,
		}

		err = survey.AskOne(promptScript, &selectedScript, 
			survey.WithFilter(func(filter string, value string, index int) bool {
				cleanValue := strings.ReplaceAll(value, ColorRed, "")
				cleanValue = strings.ReplaceAll(cleanValue, ColorReset, "")
				cleanValue = strings.ReplaceAll(cleanValue, " | ", " ")
				return strings.Contains(strings.ToLower(cleanValue), strings.ToLower(filter))
			}),
			survey.WithIcons(func(icons *survey.IconSet) {
				icons.SelectFocus.Format = ""
				icons.MarkedOption.Format = ""
				icons.UnmarkedOption.Format = ""
			}),
			survey.WithStdio(os.Stdin, os.Stdout, os.Stderr),
		)

		if err != nil {
			fmt.Printf("%sError selecting script: %v%s\n", ColorRed, err, ColorReset)
			return
		}

		scriptName := getScriptName(selectedScript)
		if scriptName == "" {
			fmt.Printf("%sInvalid selection%s\n", ColorRed, ColorReset)
			continue
		}

		// Mostrar descripción detallada
		showDetailedDescription(scriptName, descriptions)
		
		printSeparator()

		// Espera por Enter
		fmt.Printf("%sPress Enter to return...%s", ColorBlue, ColorReset)
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')

		// Limpiar pantalla y volver al menú
		fmt.Print("\033[H\033[2J")
	}
}