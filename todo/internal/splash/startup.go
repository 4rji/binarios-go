package splash

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const InputDelay = 500 * time.Millisecond
const fastSpeed = 8

type StartupTick time.Time

// Startup runs the splash inside Bubble Tea so input has one reader throughout
// startup. Its zero value skips the animation.
type Startup struct {
	star                 *artwork
	base                 rgb
	started, accelerated time.Time
	progress             float64
	columns, rows        int
}

func NewStartup() Startup {
	if !canAnimate() {
		return Startup{}
	}
	star := deathStar()
	return Startup{star: &star, base: terminalBackground()}
}

func (s Startup) Active() bool { return s.star != nil }

// Start is called on the first window-size event, once the terminal is ready.
func (s *Startup) Start(columns, rows int, now time.Time) tea.Cmd {
	if !s.Active() {
		return nil
	}
	if !fits(columns, rows) || (!s.started.IsZero() && (columns != s.columns || rows != s.rows)) {
		s.star = nil
		return nil
	}
	if !s.started.IsZero() {
		return nil
	}
	s.columns, s.rows, s.started = columns, rows, now
	return startupTick()
}

// AcceptKey discards shortcut residue during the initial grace period. The
// first later key accelerates the remaining animation without jumping frames.
func (s *Startup) AcceptKey(now time.Time) bool {
	if s.started.IsZero() || now.Sub(s.started) < InputDelay {
		return false
	}
	if s.accelerated.IsZero() {
		s.accelerated = now
	}
	return true
}

func (s *Startup) Update(tick StartupTick) tea.Cmd {
	if !s.Active() || s.started.IsZero() {
		return nil
	}
	elapsed := time.Time(tick).Sub(s.started)
	if !s.accelerated.IsZero() {
		elapsed += (fastSpeed - 1) * max(0, time.Time(tick).Sub(s.accelerated))
	}
	s.progress = min(1, float64(elapsed)/float64(duration))
	if s.progress >= 1 {
		s.star = nil
		return nil
	}
	return startupTick()
}

func (s Startup) View() string {
	if !s.Active() || s.started.IsZero() {
		return ""
	}
	// Bubble Tea owns cursor placement; turn the standalone frame's positions
	// into ordinary lines so its renderer can track and clear the splash.
	view := frame(*s.star, s.base, 1, 1, s.progress)
	left, top := (s.columns-width)/2, (s.rows-height)/2
	for y := 0; y < height; y++ {
		prefix := "\n"
		if y == 0 {
			prefix = strings.Repeat("\n", top)
		}
		view = strings.Replace(view, fmt.Sprintf("\x1b[%d;1H", y+1), prefix+strings.Repeat(" ", left), 1)
	}
	return view + resetColor
}

func startupTick() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg { return StartupTick(t) })
}
