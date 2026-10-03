// Package splash draws a short, character-built Death Star before the CLI starts.
package splash

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	width    = 64
	height   = 30 // Terminal cells are approximately twice as tall as they are wide.
	duration = 1800 * time.Millisecond
	fps      = 30

	enterScreen = "\x1b[?1049h\x1b[?25l\x1b[0m"
	leaveScreen = "\x1b[?1049l\x1b[?25h"
	resetColor  = "\x1b[0m"
)

type cell struct {
	glyph rune
	shade float64
}

type artwork [height][width]cell
type terminalSize func() (int, int, error)

// Play runs for 1.8 seconds on interactive terminals of at least 66 by 32 cells.
// Redirected output, dumb terminals, size changes and I/O errors skip the splash.
// The normal screen, cursor and colors are restored before returning, panicking
// or forwarding a termination signal. Input and terminal modes are not changed.
func Play() {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) || os.Getenv("TERM") == "dumb" {
		return
	}
	getSize := func() (int, int, error) { return term.GetSize(fd) }
	columns, rows, err := getSize()
	if err != nil || !fits(columns, rows) {
		return
	}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(interrupts)

	sig := play(os.Stdout, getSize, interrupts)
	// Stop only our handler, after cleanup. Drain any signal received during
	// cleanup so an interrupt cannot accidentally start the normal application.
	signal.Stop(interrupts)
	if sig == nil {
		select {
		case sig = <-interrupts:
		default:
		}
	}
	if sig != nil {
		// Preserve the signal's normal termination behavior after restoring the
		// terminal. The exit is a fallback on systems that cannot signal themselves.
		if process, err := os.FindProcess(os.Getpid()); err == nil {
			_ = process.Signal(sig)
		}
		if number, ok := sig.(syscall.Signal); ok {
			os.Exit(128 + int(number))
		}
		os.Exit(1)
	}
}

func fits(columns, rows int) bool {
	// A margin prevents right-edge wrapping and bottom-edge scrolling.
	return columns >= width+2 && rows >= height+2
}

func play(out io.Writer, getSize terminalSize, interrupts <-chan os.Signal) os.Signal {
	columns, rows, err := getSize()
	if err != nil || !fits(columns, rows) {
		return nil
	}
	left, top := (columns-width)/2+1, (rows-height)/2+1

	// Use a temporary alternate screen to preserve existing terminal content
	// regardless of where the shell's cursor was. Neither frames nor cleanup
	// clear the whole screen or emit newlines.
	defer func() {
		var cleanup strings.Builder
		cleanup.WriteString(resetColor)
		if columns, rows, err := getSize(); err == nil {
			count := min(width, columns-left+1)
			if count > 0 {
				for y := 0; y < height && top+y <= rows; y++ {
					fmt.Fprintf(&cleanup, "\x1b[%d;%dH\x1b[%dX", top+y, left, count)
				}
			}
		}
		cleanup.WriteString(leaveScreen)
		_, _ = io.WriteString(out, cleanup.String())
	}()

	if _, err := io.WriteString(out, enterScreen); err != nil {
		return nil
	}
	star := deathStar()
	colors := palette()
	started := time.Now()
	ticker := time.NewTicker(time.Second / fps)
	defer ticker.Stop()
	timer := time.NewTimer(duration)
	defer timer.Stop()

	if _, err := io.WriteString(out, frame(star, colors, left, top, 0)); err != nil {
		return nil
	}
	for {
		select {
		case sig := <-interrupts:
			return sig
		case <-timer.C:
			return nil
		case <-ticker.C:
			// Stop on resize instead of drawing outside the new viewport. Cleanup
			// clips its erase commands to the current terminal dimensions.
			w, h, err := getSize()
			if err != nil || w != columns || h != rows {
				return nil
			}
			progress := min(1, time.Since(started).Seconds()/duration.Seconds())
			if _, err := io.WriteString(out, frame(star, colors, left, top, progress)); err != nil {
				return nil
			}
		}
	}
}

func deathStar() artwork {
	var star artwork
	for y := range star {
		for x := range star[y] {
			nx := (float64(x) + 0.5 - width/2) / (width / 2.0)
			ny := (float64(y) + 0.5 - height/2) / (height / 2.0)
			radius2 := nx*nx + ny*ny
			if radius2 > 1 {
				continue
			}
			depth := math.Sqrt(1 - radius2)
			longitude, latitude := math.Atan2(nx, depth), math.Asin(ny)
			light := max(0, 0.64*depth-0.38*nx-0.30*ny)
			c := cell{'▪', 0.48 + 0.46*light}
			// Staggered spherical panel seams, small plates and running lights.
			panel := math.Floor(latitude * 10)
			switch {
			case radius2 > 0.91:
				c.glyph, c.shade = '·', c.shade*0.8
			case math.Abs(math.Sin(latitude*10)) < 0.12:
				c.glyph, c.shade = '▫', c.shade*0.75
			case math.Abs(math.Sin(longitude*14+panel*0.65)) < 0.16:
				c.glyph, c.shade = '·', c.shade*0.55
			case (x*197+y*37+x*y*11)%83 == 0:
				c.glyph, c.shade = '■', 0.95
			case (x*13+y*7)%19 == 0:
				c.glyph = '▫'
			}

			// A continuous dark equatorial trench with raised edges and ports.
			switch y {
			case height/2 - 2, height/2 + 1:
				c.glyph, c.shade = '▪', 0.65
			case height/2 - 1:
				c.glyph, c.shade = '─', 0.28
			case height / 2:
				c.glyph, c.shade = '·', 0.16
				if x%7 == 0 {
					c.glyph, c.shade = '▫', 0.38
				}
			}

			// The recessed upper-right superlaser dish: bright outer rim,
			// concentric rings, eight radial ribs and a central emitter.
			dx, dy := nx-0.36, ny+0.43
			u, v := (0.96*dx+0.28*dy)/0.29, (-0.28*dx+0.96*dy)/0.26
			dish := math.Hypot(u, v)
			switch {
			case dish < 0.19:
				c.glyph, c.shade = '■', 0.75
			case dish < 0.88:
				c.glyph, c.shade = '·', 0.20+0.18*dish
				if math.Abs(math.Sin(math.Atan2(v, u)*4)) < 0.22 {
					c.glyph, c.shade = '▪', 0.52
				} else if dish > 0.50 && dish < 0.63 {
					c.glyph, c.shade = '▫', 0.40
				}
			case dish < 1.12:
				c.glyph, c.shade = '▫', 0.95
			}
			star[y][x] = c
		}
	}
	return star
}

func palette() [32]string {
	var colors [32]string
	for i := range colors {
		t := float64(i) / float64(len(colors)-1)
		colors[i] = fmt.Sprintf("\x1b[38;2;%d;%d;%dm",
			22+int(196*t), 28+int(197*t), 38+int(192*t))
	}
	return colors
}

func frame(star artwork, colors [32]string, left, top int, progress float64) string {
	var out strings.Builder
	out.Grow(width * height * 12)
	previousColor := -1
	for y, row := range star {
		fmt.Fprintf(&out, "\x1b[%d;%dH", top+y, left)
		for x, c := range row {
			if c.glyph == 0 {
				out.WriteByte(' ')
				continue
			}
			nx := (float64(x) + 0.5 - width/2) / (width / 2.0)
			ny := (float64(y) + 0.5 - height/2) / (height / 2.0)
			// A broad diagonal highlight travels left to right. A quieter sine
			// wave follows it, so neighboring cells brighten and fade gradually.
			distance := (nx + 0.25*ny - (-1.45 + 2.9*progress)) / 0.34
			wave := math.Exp(-distance * distance)
			shimmer := 0.5 + 0.5*math.Sin(4*nx+2*ny-6*progress)
			brightness := c.shade * (0.24 + 0.68*wave + 0.08*shimmer)
			index := int(brightness * float64(len(colors)-1))
			index = max(0, min(len(colors)-1, index))
			if index != previousColor {
				out.WriteString(colors[index])
				previousColor = index
			}
			out.WriteRune(c.glyph)
		}
	}
	return out.String()
}
