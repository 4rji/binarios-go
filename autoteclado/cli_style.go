package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	colorHeading = "1;36"
	colorCyan    = "36"
	colorGreen   = "32"
	colorYellow  = "33"
	colorRed     = "1;31"
	colorDim     = "2"
)

type cliStyle struct {
	enabled bool
}

// styleFor colorea solo la terminal que recibe el mensaje, nunca un archivo
// o una tubería. NO_COLOR permite desactivarlo también en una terminal.
func styleFor(out io.Writer) cliStyle {
	file, ok := out.(interface{ Fd() uintptr })
	return cliStyle{enabled: ok && os.Getenv("NO_COLOR") == "" &&
		os.Getenv("TERM") != "dumb" && term.IsTerminal(int(file.Fd()))}
}

func (s cliStyle) paint(color, text string) string {
	if !s.enabled {
		return text
	}
	return "\033[" + color + "m" + text + "\033[0m"
}

func printError(format string, args ...interface{}) {
	style := styleFor(os.Stderr)
	fmt.Fprintln(os.Stderr, style.paint(colorRed, "error:")+" "+fmt.Sprintf(format, args...))
}
