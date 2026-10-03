package splash

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
)

func TestPlaySkipsUnavailableSpace(t *testing.T) {
	tests := []struct {
		name          string
		columns, rows int
		err           error
	}{
		{"narrow", 65, 40, nil},
		{"short", 80, 31, nil},
		{"unknown", 0, 0, nil},
		{"size error", 80, 40, errors.New("no terminal size")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			getSize := func() (int, int, error) { return tt.columns, tt.rows, tt.err }
			if sig := play(&out, getSize, nil); sig != nil || out.Len() != 0 {
				t.Fatalf("skipped splash emitted output or signal: %q, %v", out.String(), sig)
			}
		})
	}
}

func TestPlaySkipsRedirectedStdout(t *testing.T) {
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	saved := os.Stdout
	os.Stdout = out
	defer func() { os.Stdout = saved }()
	Play()
	info, err := out.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatal("redirected stdout received splash output")
	}
}

type recordingWriter struct {
	bytes.Buffer
	writes  int
	failAt  int
	panicAt int
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.panicAt {
		panic("write failed")
	}
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

// Keep io.WriteString on the same failure path as Write.
func (w *recordingWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func TestPlayRestoresTerminal(t *testing.T) {
	tests := []struct {
		name            string
		interrupt       os.Signal
		resize          bool
		failAt, panicAt int
	}{
		{name: "completed"},
		{name: "interrupt", interrupt: os.Interrupt},
		{name: "termination", interrupt: syscall.SIGTERM},
		{name: "resize", resize: true},
		{name: "partial setup", failAt: 1},
		{name: "frame error", failAt: 2},
		{name: "panic", panicAt: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &recordingWriter{failAt: tt.failAt, panicAt: tt.panicAt}
			interrupts := make(chan os.Signal, 1)
			if tt.interrupt != nil {
				interrupts <- tt.interrupt
			}
			calls := 0
			getSize := func() (int, int, error) {
				calls++
				if tt.resize && calls > 1 {
					return 40, 10, nil
				}
				return 80, 40, nil
			}
			var got os.Signal
			var panicked any
			started := time.Now()
			func() {
				defer func() { panicked = recover() }()
				got = play(out, getSize, interrupts)
			}()
			if got != tt.interrupt {
				t.Errorf("signal = %v, want %v", got, tt.interrupt)
			}
			if (panicked != nil) != (tt.panicAt != 0) {
				t.Errorf("panic = %v", panicked)
			}
			if tt.name == "completed" && time.Since(started) < duration {
				t.Error("animation ended before its configured duration")
			}
			output := out.String()
			if !strings.HasSuffix(output, leaveScreen) {
				t.Fatal("normal screen and cursor were not restored")
			}
			cleanup := output[strings.LastIndex(output, resetColor):]
			if !strings.Contains(cleanup, "X") {
				t.Error("animation area was not erased")
			}
			if tt.resize {
				commands := regexp.MustCompile(`\x1b\[(\d+);(\d+)H\x1b\[(\d+)X`).FindAllStringSubmatch(cleanup, -1)
				if len(commands) == 0 {
					t.Fatal("no clipped erase commands")
				}
				for _, command := range commands {
					row, _ := strconv.Atoi(command[1])
					column, _ := strconv.Atoi(command[2])
					count, _ := strconv.Atoi(command[3])
					if row > 10 || column+count-1 > 40 {
						t.Errorf("erase outside resized viewport: %q", command[0])
					}
				}
			}
			assertNoScrolling(t, output)
		})
	}
}

func assertNoScrolling(t *testing.T, output string) {
	t.Helper()
	for _, forbidden := range []string{"\n", "\r", "\x1b[2J", "\x1b[3J", "\x1b[K"} {
		if strings.Contains(output, forbidden) {
			t.Errorf("output contains scrolling or whole-screen erase sequence %q", forbidden)
		}
	}
}

func TestFrameBoundsAndMovingHighlight(t *testing.T) {
	positions := regexp.MustCompile(`\x1b\[(\d+);(\d+)H`)
	colorCodes := regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)
	star, colors := deathStar(), palette()
	early, late := frame(star, colors, 2, 2, 0.33), frame(star, colors, 2, 2, 0.73)
	for _, output := range []string{early, late} {
		assertNoScrolling(t, output)
		commands := positions.FindAllStringSubmatch(output, -1)
		lines := positions.Split(output, -1)[1:]
		if len(lines) != height {
			t.Fatalf("frame has %d rows, want %d", len(lines), height)
		}
		for y, line := range lines {
			if commands[y][1] != strconv.Itoa(y+2) || commands[y][2] != "2" {
				t.Errorf("row %d positioned outside its rectangle", y)
			}
			if count := utf8.RuneCountInString(colorCodes.ReplaceAllString(line, "")); count != width {
				t.Errorf("row %d has %d cells, want %d", y, count, width)
			}
		}
	}
	if colorCodes.ReplaceAllString(early, "") != colorCodes.ReplaceAllString(late, "") {
		t.Error("animation changed the silhouette instead of its brightness")
	}
	// Read actual ANSI red values at two surface points on opposite sides. The
	// left must be brighter early and the right brighter late, rather than both
	// changing together as they would with a whole-logo blink.
	redAt := func(output string, x, y int) int {
		line := positions.Split(output, -1)[y+1]
		red, column := 0, 0
		for len(line) > 0 {
			if loc := colorCodes.FindStringSubmatchIndex(line); loc != nil && loc[0] == 0 {
				red, _ = strconv.Atoi(line[loc[2]:loc[3]])
				line = line[loc[1]:]
				continue
			}
			if column == x {
				return red
			}
			_, n := utf8.DecodeRuneInString(line)
			line = line[n:]
			column++
		}
		t.Fatal("missing surface cell")
		return 0
	}
	if redAt(early, 13, 20) <= redAt(late, 13, 20) || redAt(early, 50, 20) >= redAt(late, 50, 20) {
		t.Error("highlight did not move from the left side to the right side")
	}
}
