package main

import (
	"fmt"
	"strings"
)

// printSeparator prints a colored separator line
func printSeparator() {
	colors := []string{ColorRed, ColorBlue}
	width := 80
	segmentWidth := width / len(colors)

	for _, color := range colors {
		fmt.Print(Dim + color + strings.Repeat("═", segmentWidth))
	}
	fmt.Println(ColorReset)
}

// formatScriptList formats the list of scripts for display
func formatScriptList(scripts []Script, grepMode bool) []string {
	var result []string
	if grepMode {
		for _, script := range scripts {
			line := fmt.Sprintf("%s%s%s\t%s", ColorGreen, script.Name, ColorReset, script.Desc)
			result = append(result, line)
		}
	} else {
		maxNameLength := 0
		for _, s := range scripts {
			if len(s.Name) > maxNameLength {
				maxNameLength = len(s.Name)
			}
		}
		maxNameLength += 2
		for _, s := range scripts {
			padding := strings.Repeat(" ", maxNameLength-len(s.Name))
			formattedName := fmt.Sprintf("%s%s%s%s", ThemeCyan, s.Name, ColorReset, padding)
			description := fmt.Sprintf("%s%s%s%s", Dim, ThemeBlue, s.Desc, ColorReset)
			result = append(result, fmt.Sprintf("%s · %s", formattedName, description))
		}
	}
	return result
}

// getScriptName extracts the script name from the selected option
func getScriptName(selectedScript string) string {
	cleanSelected := strings.ReplaceAll(selectedScript, ColorRed, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, ColorReset, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, ThemeCyan, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, ThemeBlue, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, Bold, "")
	cleanSelected = strings.ReplaceAll(cleanSelected, Dim, "")

	// Split by spaces and take the first part
	parts := strings.Fields(cleanSelected)
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

// printFancyBox prints a fancy box with title and content
func printFancyBox(title string, content string) {
	width := 60
	fmt.Printf("%s%s%s%s%s\n", ThemeBlue, BoxTopLeft, strings.Repeat(BoxHorizontal, width-2), BoxTopRight, ColorReset)
	fmt.Printf("%s%s %s%s%s%s%s%s\n", ThemeBlue, BoxVertical, ThemeCyan, Bold, title, ColorReset, strings.Repeat(" ", width-3-len(title)), BoxVertical)
	fmt.Printf("%s%s%s%s%s\n", ThemeBlue, BoxBottomLeft, strings.Repeat(BoxHorizontal, width-2), BoxBottomRight, ColorReset)
	fmt.Println(content)
}
