package main

import "github.com/charmbracelet/lipgloss"

const (
	// foreground neons - cyberpunk: magenta + blue + yellow
	colorAccent     = lipgloss.Color("#cc00ff") // softer magenta cyberpunk
	colorScriptName = lipgloss.Color("#ffff00") // bright yellow
	colorGreen      = lipgloss.Color("#00ffaa") // cyan-green
	colorMuted      = lipgloss.Color("#556688") // muted blue-gray
	colorText       = lipgloss.Color("#e8e8ff") // cool white
	colorSubtext    = lipgloss.Color("#7788aa") // medium blue-gray
	colorPurple     = lipgloss.Color("#0088ff") // electric blue
	colorOrange     = lipgloss.Color("#ff00aa") // magenta-pink

	// backgrounds
	colorBgBase     = lipgloss.Color("#0a0515") // very dark navy
	colorBgHeader   = lipgloss.Color("#1a0033") // dark magenta header
	colorBgSubHdr   = lipgloss.Color("#001a33") // navy subheader
	colorBgRow      = lipgloss.Color("#0b0818") // dark row
	colorBgRowAlt   = lipgloss.Color("#0d0f15") // alt dark row
	colorBgSelected = lipgloss.Color("#330055") // dark magenta selection
	colorBorder     = lipgloss.Color("#cc00ff") // magenta separator

	// keep for legacy usage
	colorSelected = colorBgSelected
)

type Styles struct {
	MatchHL lipgloss.Style
}

func newStyles() Styles {
	return Styles{
		MatchHL: lipgloss.NewStyle().
			Background(colorScriptName).
			Foreground(lipgloss.Color("#000000")).
			Bold(true),
	}
}
