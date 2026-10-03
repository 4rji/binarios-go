package splash

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const menuDuration = 900 * time.Millisecond

// FadeTick requests another menu frame without blocking input or data loading.
type FadeTick time.Time

// Fade gradually reveals an existing ANSI view, preserving its final colors.
// Its zero value leaves views unchanged.
type Fade struct {
	enabled  bool
	base     rgb // The terminal's background, where the transition starts.
	started  time.Time
	progress float64
}

// NewFade enables the menu transition only where the startup splash can run.
// Call it before the Bubble Tea program starts; see [NewBackdrop].
func NewFade() Fade {
	if !canAnimate() {
		return Fade{}
	}
	return Fade{enabled: true, base: terminalBackground()}
}

// Start begins the transition once the menu's data is ready.
func (f *Fade) Start() tea.Cmd {
	if !f.enabled || !f.started.IsZero() {
		return nil
	}
	f.started = time.Now()
	return fadeTick()
}

// Update advances the transition and stops scheduling frames at full brightness.
func (f *Fade) Update(tick FadeTick) tea.Cmd {
	if !f.enabled || f.started.IsZero() {
		return nil
	}
	f.progress = smoothstep(time.Time(tick).Sub(f.started).Seconds() / menuDuration.Seconds())
	if f.progress >= 1 {
		f.enabled = false
		return nil
	}
	return fadeTick()
}

// View keeps the loading view invisible until Start, then fades in the ready menu.
func (f Fade) View(view string) string {
	if !f.enabled {
		return view
	}
	return fadeView(view, f.base, f.progress)
}

func fadeTick() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg { return FadeTick(t) })
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// fadeView blends every color in view from base (alpha 0) to itself (alpha 1).
func fadeView(view string, base rgb, alpha float64) string {
	if alpha >= 1 {
		return view
	}
	alpha = max(0, alpha)
	fadedColor := func(code int, color rgb) string {
		c := mix(base, color, alpha)
		return fmt.Sprintf("%d;2;%d;%d;%d", code, c.r, c.g, c.b)
	}
	foreground := fadedColor(38, rgb{232, 232, 255})
	background := "49" // Cells without a background keep the terminal's own.
	defaults := foreground + ";" + background

	// Lip Gloss may emit truecolor, 256-color or 16-color SGR depending on the
	// terminal. Convert just the colors; leave text, layout and other ANSI intact.
	faded := sgr.ReplaceAllStringFunc(view, func(sequence string) string {
		parts := strings.Split(sequence[2:len(sequence)-1], ";")
		var codes []string
		for i := 0; i < len(parts); i++ {
			code, err := strconv.Atoi(parts[i])
			if err != nil && parts[i] != "" {
				codes = append(codes, parts[i])
				continue
			}
			switch {
			case code == 0:
				codes = append(codes, "0", defaults)
			case code == 39:
				codes = append(codes, foreground)
			case code == 49:
				codes = append(codes, background)
			case (code == 38 || code == 48) && i+4 < len(parts) && parts[i+1] == "2":
				r, er := strconv.Atoi(parts[i+2])
				g, eg := strconv.Atoi(parts[i+3])
				b, eb := strconv.Atoi(parts[i+4])
				if er == nil && eg == nil && eb == nil {
					codes = append(codes, fadedColor(code, rgb{r, g, b}))
					i += 4
				} else {
					codes = append(codes, parts[i])
				}
			case (code == 38 || code == 48) && i+2 < len(parts) && parts[i+1] == "5":
				n, err := strconv.Atoi(parts[i+2])
				if err == nil && n >= 0 && n <= 255 {
					codes = append(codes, fadedColor(code, indexedColor(n)))
					i += 2
				} else {
					codes = append(codes, parts[i])
				}
			case code >= 30 && code <= 37:
				codes = append(codes, fadedColor(38, indexedColor(code-30)))
			case code >= 90 && code <= 97:
				codes = append(codes, fadedColor(38, indexedColor(code-90+8)))
			case code >= 40 && code <= 47:
				codes = append(codes, fadedColor(48, indexedColor(code-40)))
			case code >= 100 && code <= 107:
				codes = append(codes, fadedColor(48, indexedColor(code-100+8)))
			default:
				codes = append(codes, parts[i])
			}
		}
		return "\x1b[" + strings.Join(codes, ";") + "m"
	})
	return "\x1b[" + defaults + "m" + faded + resetColor
}
