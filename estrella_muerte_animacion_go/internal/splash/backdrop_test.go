package splash

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func testBackdrop(bg rgb) Backdrop {
	star := deathStar()
	return Backdrop{star: &star, background: bg}
}

type testCell struct {
	text, fg string
	bg       rgb
	reverse  bool
}

// decodeCells returns each visible cell of line with the style it is drawn in.
// Cells on the terminal's own background report bg -1,-1,-1.
func decodeCells(line string) []testCell {
	terminal := rgb{-1, -1, -1}
	style := sgrState{fg: "39", bg: terminal}
	var cells []testCell
	var state byte
	for len(line) > 0 {
		seq, w, n, next := ansi.DecodeSequence(line, state, nil)
		line, state = line[n:], next
		if w > 0 {
			cells = append(cells, testCell{seq, style.fg, style.bg, style.reverse})
		} else if isSGR(seq) {
			style.apply(seq[2:len(seq)-1], terminal)
		}
	}
	return cells
}

func TestSplashEndsOnBackdrop(t *testing.T) {
	cursorMoves := regexp.MustCompile(`\x1b\[\d+;\d+H`)
	const columns, rows = width + 2, height + 2
	left, top := (columns-width)/2, (rows-height)/2
	for _, bg := range []rgb{darkness, light} {
		b := testBackdrop(bg)
		// Frames carry their colors from row to row; decode them as one run.
		splash := decodeCells(cursorMoves.ReplaceAllString(frame(*b.star, bg, left+1, top+1, 1), ""))
		backdropRows := strings.Split(b.View("", columns, rows), "\n")
		for y, row := range b.star {
			last := splash[y*width : (y+1)*width]
			kept := decodeCells(backdropRows[top+y])[left:]
			for x, c := range row {
				if c.glyph != 0 && last[x] != kept[x] {
					t.Fatalf("cell %d,%d on %v: splash ends on %+v, backdrop shows %+v", x, y, bg, last[x], kept[x])
				}
			}
		}
	}
}

func TestBackdropOnlyFillsBlankCells(t *testing.T) {
	b := testBackdrop(darkness)
	const columns, rows = 80, 36
	left, top := (columns-width)/2, (rows-height)/2
	rowBg := rgb{11, 8, 24}
	var view strings.Builder
	for y := 0; y < rows-1; y++ {
		text := fmt.Sprintf("%-*s", columns, fmt.Sprintf("  script-%02d   does something useful", y))
		fmt.Fprintf(&view, "\x1b[38;2;232;232;255;48;2;11;8;24m%s\x1b[0m\n", text)
	}
	got := b.View(view.String(), columns, rows)
	wantLines, gotLines := strings.Split(view.String(), "\n"), strings.Split(got, "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("view has %d lines, want %d", len(gotLines), len(wantLines))
	}
	glyphs := 0
	for y := range wantLines {
		want, have := decodeCells(wantLines[y]), decodeCells(gotLines[y])
		if len(have) != len(want) {
			t.Fatalf("line %d has %d cells, want %d", y, len(have), len(want))
		}
		for x := range want {
			if have[x] == want[x] {
				continue
			}
			sx, sy := x-left, y-top
			if want[x].text != " " || sx < 0 || sx >= width || sy < 0 || sy >= height {
				t.Fatalf("cell %d,%d changed from %+v to %+v", x, y, want[x], have[x])
			}
			c := b.star[sy][sx]
			wantGlyph := testCell{string(c.glyph), foreground(tint(rowBg, c.shade*backdropLevel)), rowBg, false}
			if have[x] != wantGlyph {
				t.Fatalf("cell %d,%d = %+v, want %+v", x, y, have[x], wantGlyph)
			}
			glyphs++
		}
	}
	if glyphs == 0 {
		t.Fatal("no Death Star cells were drawn")
	}
}

func TestBackdropRestoresStylesAndSkipsReverseVideo(t *testing.T) {
	b := testBackdrop(darkness)
	const columns, rows = 80, 36
	left, top := (columns-width)/2, (rows-height)/2
	// The equator row is covered by the Death Star from edge to edge.
	line := "\x1b[38;5;99m" + strings.Repeat(" ", left+31) + "x\x1b[7m \x1b[27m" + strings.Repeat(" ", 10) + "\x1b[0m"
	view := strings.Repeat("\n", top+height/2) + line + "\n"
	cells := decodeCells(strings.Split(b.View(view, columns, rows), "\n")[top+height/2])
	if cells[left+31] != (testCell{"x", "38;5;99", rgb{-1, -1, -1}, false}) {
		t.Errorf("text after a glyph lost its style: %+v", cells[left+31])
	}
	if cells[left+32].text != " " || !cells[left+32].reverse {
		t.Errorf("reverse-video cursor cell was drawn over: %+v", cells[left+32])
	}
	for _, x := range []int{30, 33} {
		c := b.star[height/2][x]
		want := foreground(tint(darkness, c.shade*backdropLevel))
		if cells[left+x].text != string(c.glyph) || cells[left+x].fg != want {
			t.Errorf("blank cell %d on the terminal background = %+v, want %q in %s", x, cells[left+x], c.glyph, want)
		}
	}
}

func TestBackdropFitsScreen(t *testing.T) {
	b := testBackdrop(darkness)
	for _, view := range []string{"", "menu", "menu\n"} {
		for _, rows := range []int{height + 2, 40} {
			got := b.View(view, 80, rows)
			if n := len(strings.Split(got, "\n")); n > rows {
				t.Errorf("view %q grew to %d lines on a %d-row screen", view, n, rows)
			}
			if !strings.HasPrefix(got, strings.TrimSuffix(view, "\n")) {
				t.Errorf("first line of %q changed: %q", view, got)
			}
			if strings.HasSuffix(got, "\n") != strings.HasSuffix(view, "\n") {
				t.Errorf("trailing newline of %q changed", view)
			}
		}
	}
	var disabled Backdrop
	for _, size := range [][2]int{{80, 40}, {width + 1, 40}, {80, height + 1}} {
		if got := b.View("menu\n", size[0], size[1]); size != [2]int{80, 40} && got != "menu\n" {
			t.Errorf("backdrop drawn on a %dx%d screen", size[0], size[1])
		}
		if got := disabled.View("menu\n", size[0], size[1]); got != "menu\n" {
			t.Errorf("zero Backdrop changed the view: %q", got)
		}
	}
}

func TestSGRState(t *testing.T) {
	terminal := rgb{1, 2, 3}
	tests := []struct {
		params string
		want   sgrState
	}{
		{"1;38;2;4;5;6;48;5;232", sgrState{"38;2;4;5;6", rgb{8, 8, 8}, false}},
		{"91;44", sgrState{"91", rgb{0, 0, 128}, false}},
		{"38;5;99;48;2;7;8;9;7", sgrState{"38;5;99", rgb{7, 8, 9}, true}},
		{"31;41;7;0", sgrState{"39", terminal, false}},
		{"31;41;7;", sgrState{"39", terminal, false}},
		{"31;41;7;39;49;27", sgrState{"39", terminal, false}},
	}
	for _, tt := range tests {
		s := sgrState{fg: "39", bg: terminal}
		s.apply(tt.params, terminal)
		if s != tt.want {
			t.Errorf("apply(%q) = %+v, want %+v", tt.params, s, tt.want)
		}
	}
	for seq, want := range map[string]bool{"\x1b[m": true, "\x1b[1;31m": true, "\x1b[>4;2m": false, "\x1b[2K": false} {
		if isSGR(seq) != want {
			t.Errorf("isSGR(%q) = %v", seq, !want)
		}
	}
}
