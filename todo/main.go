package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(Model); ok && m.pendingExec != "" {
		executeScript(m.pendingExec, nil)
	}
}

func executeScript(scriptName string, args []string) {
	scriptPath := fmt.Sprintf("/opt/4rji/bin/%s", scriptName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "script not found: %s\n", scriptPath)
		os.Exit(1)
	}

	var commandParts []string
	quotedPath := scriptPath
	if strings.ContainsAny(scriptPath, " '\"`$*&|(){}[];<>?!\\#") {
		quotedPath = "'" + strings.ReplaceAll(scriptPath, "'", "'\\''") + "'"
	}
	commandParts = append(commandParts, quotedPath)
	for _, arg := range args {
		quoted := arg
		if strings.ContainsAny(arg, " '\"`$*&|(){}[];<>?!\\#") {
			quoted = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
		commandParts = append(commandParts, quoted)
	}
	_ = copyToClipboard(strings.Join(commandParts, " "))

	fmt.Print("\033[H\033[2J\033[3J") // clear screen + scrollback
	cmd := exec.Command(scriptPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
