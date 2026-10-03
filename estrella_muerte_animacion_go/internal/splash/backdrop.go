package splash

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Backdrop keeps the Death Star faintly visible behind a Bubble Tea view, in
// the place and at the brightness the splash ends on. Its zero value leaves
// views unchanged.
type Backdrop struct {
	star       *artwork
	background rgb
}

// NewBackdrop enables the backdrop on interactive terminals. Call it before the
// Bubble Tea program starts, since it may ask the terminal for its background.
func NewBackdrop() Backdrop {
	if !interactive() {
		return Backdrop{}
	}
	star := deathStar()
	return Backdrop{star: &star, background: terminalBackground()}
}

// View centers the Death Star in a screen of the given size and draws it into
// the blank cells of view. Text, backgrounds and layout are kept: only spaces
// become glyphs, tinted from the background behind them. Short views gain blank
// lines down to the star, never beyond the screen height.
func (b Backdrop) View(view string, columns, rows int) string {
	if b.star == nil || !fits(columns, rows) {
		return view
	}
	left, top := (columns-width)/2, (rows-height)/2
	trailing := strings.HasSuffix(view, "\n")
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	for len(lines) < top+height {
		lines = append(lines, "")
	}
	for y := range b.star {
		lines[top+y] = b.overlay(lines[top+y], left, &b.star[y])
	}
	if trailing {
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines, "\n")
}

func (b Backdrop) overlay(line string, left int, row *[width]cell) string {
	var out strings.Builder
	out.Grow(len(line) + width*24)
	style := sgrState{fg: "39", bg: b.background}
	var current rgb
	tinted := false // A glyph color replaces the line's foreground.
	restore := func() {
		if tinted {
			out.WriteString("\x1b[" + style.fg + "m")
			tinted = false
		}
	}
	// put writes one cell, drawing a glyph only where the line has a space.
	column := 0
	put := func(text string) {
		x := column - left
		if text == " " && x >= 0 && x < width && row[x].glyph != 0 && !style.reverse {
			color := tint(style.bg, row[x].shade*backdropLevel)
			if !tinted || color != current {
				writeForeground(&out, color)
				current, tinted = color, true
			}
			out.WriteRune(row[x].glyph)
			return
		}
		restore()
		out.WriteString(text)
	}

	var state byte
	for len(line) > 0 {
		seq, cells, n, next := ansi.DecodeSequence(line, state, nil)
		line, state = line[n:], next
		if cells > 0 {
			put(seq)
			column += cells
			continue
		}
		restore()
		out.WriteString(seq)
		if isSGR(seq) {
			style.apply(seq[2:len(seq)-1], b.background)
		}
	}
	// Past the text, the terminal shows the line's current background.
	for ; column < left+width; column++ {
		put(" ")
	}
	restore()
	return out.String()
}

// sgrState tracks just enough of a line's styling to tint and restore cells.
type sgrState struct {
	fg      string // Parameters that restore the foreground, such as "39".
	bg      rgb
	reverse bool
}

func isSGR(seq string) bool {
	if len(seq) < 3 || !strings.HasPrefix(seq, "\x1b[") || seq[len(seq)-1] != 'm' {
		return false
	}
	return strings.Trim(seq[2:len(seq)-1], "0123456789;") == ""
}

func (s *sgrState) apply(params string, terminal rgb) {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil && parts[i] != "" {
			continue
		}
		switch {
		case code == 0:
			*s = sgrState{fg: "39", bg: terminal}
		case code == 7:
			s.reverse = true
		case code == 27:
			s.reverse = false
		case code == 39 || code >= 30 && code <= 37 || code >= 90 && code <= 97:
			s.fg = parts[i]
		case code == 49:
			s.bg = terminal
		case code >= 40 && code <= 47:
			s.bg = indexedColor(code - 40)
		case code >= 100 && code <= 107:
			s.bg = indexedColor(code - 100 + 8)
		case (code == 38 || code == 48) && i+4 < len(parts) && parts[i+1] == "2":
			if code == 38 {
				s.fg = strings.Join(parts[i:i+5], ";")
			} else {
				r, er := strconv.Atoi(parts[i+2])
				g, eg := strconv.Atoi(parts[i+3])
				b, eb := strconv.Atoi(parts[i+4])
				if er == nil && eg == nil && eb == nil {
					s.bg = rgb{r, g, b}
				}
			}
			i += 4
		case (code == 38 || code == 48) && i+2 < len(parts) && parts[i+1] == "5":
			if code == 38 {
				s.fg = strings.Join(parts[i:i+3], ";")
			} else if n, err := strconv.Atoi(parts[i+2]); err == nil && n >= 0 && n <= 255 {
				s.bg = indexedColor(n)
			}
			i += 2
		}
	}
}
