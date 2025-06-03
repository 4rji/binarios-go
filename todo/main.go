package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// showImage displays an image for a script if available
func showImage(scriptName string) bool {
	imgPath := fmt.Sprintf("/opt/4rji/img-bin/%s", scriptName)

	if _, err := os.Stat(imgPath + ".webp"); err == nil {
		cmd := exec.Command("chafa", "--size", "80x40", imgPath+".webp")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		fmt.Printf("%s%s%s\n", ColorCyan, string(output), ColorReset)
		return true
	}

	if _, err := os.Stat(imgPath + ".png"); err == nil {
		cmd := exec.Command("chafa", "--size", "80x40", imgPath+".png")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		fmt.Printf("%s%s%s\n", ColorCyan, string(output), ColorReset)
		return true
	}

	return false
}

// showDetailedDescription shows detailed information about a script
func showDetailedDescription(scriptName string, descriptions Descriptions, scripts []Script) {
	fmt.Print("\033[H\033[2J\033[3J")
	fmt.Print("\033[H")
	if desc, ok := descriptions[scriptName]; ok {
		fmt.Printf("%s", Dim)
		printSeparator()
		fmt.Printf("%s", ColorReset)
		fmt.Printf("\n%sScript:%s %s%s%s\n", ColorYellow, ColorReset, ColorRed, scriptName, ColorReset)
		fmt.Printf("%sShort description:%s %s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.ShortDesc, ColorReset)
		fmt.Printf("\n%sDetailed description:%s\n%s%s%s\n", ColorYellow, ColorReset, ColorWhite, desc.DetailedDesc, ColorReset)
	} else {
		for _, scr := range scripts {
			if scr.Name == scriptName {
				width := 80
				padding := (width - len(scr.Name)) / 2
				centeredName := strings.Repeat(" ", padding) + scr.Name
				fmt.Printf("%s", Dim)
				printSeparator()
				fmt.Printf("%s", ColorReset)
				fmt.Printf("%s%s%s\n", ColorCyan, centeredName, ColorReset)
				fmt.Printf("%s%s%s\n", ColorWhite, scr.Desc, ColorReset)
				break
			}
		}
	}
	fmt.Print("\n\n\n")
	fmt.Printf("%s", Dim)
	printSeparator()
	fmt.Printf("%s", ColorReset)
	if showImage(scriptName) {
		fmt.Print("")
	}
	fmt.Printf("%s", Dim)
	printSeparator()
	fmt.Printf("%s", ColorReset)
	fmt.Printf("\n%sOptions:%s\n", ColorYellow, ColorReset)
	fmt.Printf("%s[%sv%s] View script with less\n", ColorWhite, ColorCyan, ColorWhite)
	fmt.Printf("%s[%se%s] Execute script\n", ColorWhite, ColorCyan, ColorWhite)
	fmt.Printf("%s[%sEnter%s] Return to menu\n", ColorWhite, ColorCyan, ColorWhite)
	fmt.Printf("\n%sSelect an option: %s", ColorCyan, ColorReset)

	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()
	if err != nil {
		return
	}
	switch char {
	case 'v', 'V':
		viewScriptWithLess(scriptName)
		showDetailedDescription(scriptName, descriptions, scripts)
	case 'e', 'E':
		executeScript(scriptName, nil)
	default:
		return
	}
}

// viewScriptWithLess views a script using less or bat
func viewScriptWithLess(scriptName string) {
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", scriptName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("%sScript file not found: %s%s\n", ColorRed, scriptPath, ColorReset)
		return
	}
	batPath, err := exec.LookPath("bat")
	if err == nil {
		cmd := exec.Command(batPath, "--style=numbers", "--color=always", "--language=bash", scriptPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		return
	}
	cmd := exec.Command("less", "-R", scriptPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// executeScript executes a script
func executeScript(scriptName string, args []string) {
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", scriptName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Printf("%sScript file not found: %s%s\n\n%sPress Enter to return...%s", ColorRed, scriptPath, ColorReset, ColorCyan, ColorReset)
		bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}
	var commandParts []string
	quotedScriptPath := scriptPath
	if strings.ContainsAny(scriptPath, " '\"`$*&|(){}[];<>?!\\#") {
		quotedScriptPath = "'" + strings.ReplaceAll(scriptPath, "'", "'\\''") + "'"
	}
	commandParts = append(commandParts, quotedScriptPath)
	for _, arg := range args {
		quotedArg := arg
		if strings.ContainsAny(arg, " '\"`$*&|(){}[];<>?!\\#") {
			quotedArg = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
		commandParts = append(commandParts, quotedArg)
	}
	fullCommand := strings.Join(commandParts, " ")
	fmt.Print("\033[H\033[2J")
	err := copyToClipboard(fullCommand)
	if err != nil {
		fmt.Printf("%sNote: Could not copy to clipboard (%v)%s\n%sPlease copy the command manually:%s\n\n", ColorYellow, err, ColorReset, ColorYellow, ColorReset)
	} else {
		fmt.Printf("%sCommand copied to clipboard:%s\n", ColorCyan, ColorReset)
	}
	fmt.Printf("%s%s%s\n\n%sYou can now run this command in your terminal.%s\n%sPress Enter to exit...%s", ThemeGreen, fullCommand, ColorReset, ColorYellow, ColorReset, ColorCyan, ColorReset)
	bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(0)
}

func main() {
	fmt.Print("\033[H\033[2J")

	// Flags
	searchFlag := flag.Bool("s", false, "Buscar en el contenido de los archivos")
	searchFlagLong := flag.Bool("search", false, "Buscar en el contenido de los archivos (long)")
	flag.Parse()
	args := flag.Args()

	isContentSearch := *searchFlag || *searchFlagLong
	currentQuery := ""
	if isContentSearch && len(args) > 0 {
		currentQuery = strings.Join(args, " ")
	} else if !isContentSearch && len(args) > 0 {
		currentQuery = strings.Join(args, " ")
	}

	descriptions, err := loadDescriptions()
	if err != nil {
		descriptions = make(Descriptions)
	}

	var scripts []Script
	var err2 error

	if isContentSearch {
		scripts, err2 = searchInFiles(currentQuery)
		if err2 != nil {
			return
		}
	} else {
		scripts, err2 = getCombinedScripts("/opt/4rji/bin/README.md", "/opt/4rji/bin")
		if err2 != nil {
			return
		}
	}

	if len(scripts) == 0 {
		return
	}

	var scriptChoices []string
	if isContentSearch {
		scriptChoices = formatScriptList(scripts, true)
	} else {
		scriptChoices = formatScriptList(scripts, false)
	}

	sort.Slice(scriptChoices, func(i, j int) bool {
		cleanA := strings.ReplaceAll(scriptChoices[i], ColorRed, "")
		cleanA = strings.ReplaceAll(cleanA, ColorReset, "")
		cleanA = strings.ReplaceAll(cleanA, ThemeCyan, "")
		cleanA = strings.ReplaceAll(cleanA, ThemeBlue, "")
		cleanA = strings.ReplaceAll(cleanA, Bold, "")
		cleanA = strings.ReplaceAll(cleanA, Dim, "")
		cleanB := strings.ReplaceAll(scriptChoices[j], ColorRed, "")
		cleanB = strings.ReplaceAll(cleanB, ColorReset, "")
		cleanB = strings.ReplaceAll(cleanB, ThemeCyan, "")
		cleanB = strings.ReplaceAll(cleanB, ThemeBlue, "")
		cleanB = strings.ReplaceAll(cleanB, Bold, "")
		cleanB = strings.ReplaceAll(cleanB, Dim, "")
		partsA := strings.SplitN(cleanA, "·", 2)
		partsB := strings.SplitN(cleanB, "·", 2)
		if len(partsA) < 2 || len(partsB) < 2 {
			return false
		}
		nameA := strings.TrimSpace(partsA[0])
		nameB := strings.TrimSpace(partsB[0])
		return strings.ToLower(nameA) < strings.ToLower(nameB)
	})

	for {
		fmt.Print("\033[H\033[2J")
		header := fmt.Sprintf("\n%s╭─────────────────────────────────────────────╮%s\n%s│%s %s4rji Script Selector%s                        %s│%s\n%s╰─────────────────────────────────────────────╯%s\n",
			ThemeBlue, ColorReset,
			ThemeBlue, ColorReset, Bold, ColorReset, ThemeBlue, ColorReset,
			ThemeBlue, ColorReset,
		)
		fmt.Print(header)
		if !isContentSearch {
			fmt.Printf("%sTip: Use -s or --search to search inside file contents.%s\n\n", ColorYellow, ColorReset)
		}
		if isContentSearch {
			printFancyBox("Search Results", fmt.Sprintf("Files containing '%s':", currentQuery))
		} else {
			printFancyBox("Available Scripts", "Choose a script from the list below:")
		}
		var selectedScript string
		fzfPath, err := exec.LookPath("fzf")
		if err == nil {
			args := []string{
				"--ansi",
				"--bind", "ctrl-u:clear-query",
				"--print-query",
				"--delimiter", "·",
				"--nth", "1",
				"--prompt", "Search> ",
				"--height=40",
			}
			if currentQuery != "" {
				args = append(args, "--query", currentQuery)
			}
			cmd := exec.Command(fzfPath, args...)
			cmd.Stdin = strings.NewReader(strings.Join(scriptChoices, "\n"))
			cmd.Stderr = os.Stderr
			out, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
					os.Exit(0)
				}
				currentQuery = ""
				continue
			}
			outStr := string(out)
			parts := strings.SplitN(outStr, "\n", 2)
			currentQuery = parts[0]
			if len(parts) > 1 {
				selectedScript = strings.TrimRight(parts[1], "\n")
			} else {
				selectedScript = ""
			}
		} else {
			if len(scriptChoices) > 0 {
				selectedScript = scriptChoices[0]
			} else {
				return
			}
		}

		if selectedScript == "" {
			continue
		}

		scriptName := getScriptName(selectedScript)
		if scriptName == "" {
			currentQuery = ""
			continue
		}
		if err := copyToClipboard(scriptName); err != nil {
			continue
		}
		fmt.Print("\033[H\033[2J")
		showDetailedDescription(scriptName, descriptions, scripts)
	}
}
