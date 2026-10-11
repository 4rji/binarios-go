package splash

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"

	"github.com/muesli/termenv"
)

type rgb struct{ r, g, b int }

var (
	darkness  = rgb{10, 5, 21}     // The menu's base background (#0a0515); light themes tint toward it.
	highlight = rgb{218, 225, 230} // The Death Star's brightest surface color.

	backgroundOnce sync.Once
	background     = darkness
)

// terminalBackground asks the terminal for its default background once.
// Terminals that do not answer get termenv's fallback: the COLORFGBG hint, or
// black. It must run before Bubble Tea starts reading input, so the terminal's
// reply cannot be mistaken for keystrokes.
func terminalBackground() rgb {
	backgroundOnce.Do(func() {
		if color := termenv.NewOutput(os.Stdout).BackgroundColor(); color != nil {
			c := termenv.ConvertToRGB(color)
			background = rgb{int(c.R*255 + 0.5), int(c.G*255 + 0.5), int(c.B*255 + 0.5)}
		}
	})
	return background
}

func mix(from, to rgb, amount float64) rgb {
	return rgb{
		from.r + int(float64(to.r-from.r)*amount),
		from.g + int(float64(to.g-from.g)*amount),
		from.b + int(float64(to.b-from.b)*amount),
	}
}

// tint moves bg toward the Death Star's highlight by level (0 to 1). Light
// backgrounds tint toward darkness instead, so the star stays visible on them.
// Levels are quantized so neighboring cells can share color sequences.
func tint(bg rgb, level float64) rgb {
	target := highlight
	if 2126*bg.r+7152*bg.g+722*bg.b > 10000*128 {
		target = darkness
	}
	return mix(bg, target, math.Round(max(0, min(1, level))*shades)/shades)
}

func writeForeground(out io.Writer, c rgb) {
	fmt.Fprintf(out, "\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b)
}

func indexedColor(n int) rgb {
	if n < 16 {
		return [...]rgb{
			{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
			{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}[n]
	}
	if n >= 232 {
		gray := 8 + (n-232)*10
		return rgb{gray, gray, gray}
	}
	levels := [...]int{0, 95, 135, 175, 215, 255}
	n -= 16
	return rgb{levels[n/36], levels[n/6%6], levels[n%6]}
}
