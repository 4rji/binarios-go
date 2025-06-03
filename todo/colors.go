package main

// ANSI color and style definitions
const (
	// Basic colors
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"

	// Theme colors (matching the image)
	ThemeBlue   = "\033[38;5;33m"              // Bright blue for the top bar
	ThemeCyan   = "\033[38;5;43m"              // Cyan for the path
	ThemeYellow = "\033[48;5;226m\033[38;5;0m" // Yellow background with black text
	ThemeGreen  = "\033[38;5;46m"              // Bright green for commands

	// Text effects
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"
	Blink     = "\033[5m"
	Reverse   = "\033[7m"
)
