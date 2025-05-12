package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	Name        string `json:"name"`
	ShortDesc   string `json:"short_desc"`
	DetailedDesc string `json:"detailed_desc"`
}

type Descriptions map[string]DetailedDescription

func loadDescriptions() (Descriptions, error) {
	file, err := os.Open("descriptions.json")
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

func showDetailedDescription(scriptName string, descriptions Descriptions) {
	if desc, ok := descriptions[scriptName]; ok {
		printSeparator()
		fmt.Printf("%s%sDetailed script description:%s%s\n", Bold, ColorPurple, ColorReset, ColorReset)
		printSeparator()
		fmt.Printf("\n%sScript:%s %s%s%s\n", ColorYellow, ColorReset, ColorRed, scriptName, ColorReset)
		fmt.Printf("%sShort description:%s %s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.ShortDesc, ColorReset)
		fmt.Printf("\n%sDetailed description:%s\n%s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.DetailedDesc, ColorReset)
	} else {
		fmt.Printf("\n%sNo detailed. Basic description only on selector. %s\n", ColorYellow, ColorReset)
		
	}
}

func main() {
	// Clear screen at start
	fmt.Print("\033[H\033[2J")

	descriptions, err := loadDescriptions()
	if err != nil {
		fmt.Printf("%sWarning: Could not load descriptions file: %v%s\n", ColorYellow, err, ColorReset)
	}

	
	scripts, err := parseReadme("README.md")
	if err != nil {
		fmt.Printf("%sError reading README.md: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if len(scripts) == 0 {
		fmt.Printf("%sNo scripts found in README%s\n", ColorYellow, ColorReset)
		return
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
			PageSize: 14,
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

		// Show detailed description
		showDetailedDescription(scriptName, descriptions)
		
		printSeparator()

		// Wait for Enter key
		fmt.Printf("%sPress Enter to return...%s", ColorBlue, ColorReset)
		var input string
		fmt.Scanln(&input)

		// Clear screen and return to menu
		fmt.Print("\033[H\033[2J")
	}
} 
