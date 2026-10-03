package splash

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFadePreservesViewAndFinalColors(t *testing.T) {
	tests := []struct {
		name, view, halfway string
	}{
		{"truecolor", "\x1b[1;38;2;210;105;221;48;2;110;45;61m▪ menu\x1b[0m\n", "38;2;110;55;121;48;2;60;25;41"},
		{"256 colors", "\x1b[38;5;196;48;5;232m▫ menu\x1b[m\n", "38;2;132;3;11;48;2;9;6;15"},
		{"16 colors", "\x1b[91;44m· menu\x1b[39;49m\n", "38;2;132;3;11;48;2;5;3;74"},
		{"plain text", "◈ menu\n", "38;2;121;118;138"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fadeView(tt.view, 1); got != tt.view {
				t.Errorf("completed transition changed the original view: %q", got)
			}
			got := fadeView(tt.view, 0.5)
			if !strings.Contains(got, tt.halfway) {
				t.Errorf("missing halfway colors %q in %q", tt.halfway, got)
			}
			if sgr.ReplaceAllString(got, "") != sgr.ReplaceAllString(tt.view, "") {
				t.Error("transition changed view text or layout")
			}
			dark := fadeView(tt.view, 0)
			for _, color := range regexp.MustCompile(`(?:38|48);2;(\d+);(\d+);(\d+)`).FindAllStringSubmatch(dark, -1) {
				if color[1] != "10" || color[2] != "5" || color[3] != "21" {
					t.Errorf("visible color before fade-in: %q", color[0])
				}
			}
			if !strings.HasSuffix(got, resetColor) {
				t.Error("transition leaked its colors past the view")
			}
		})
	}
}

func TestFadeWaitsForDataAndStopsAtFullBrightness(t *testing.T) {
	var disabled Fade
	if disabled.Start() != nil || disabled.View("menu") != "menu" {
		t.Error("disabled transition changed the menu")
	}
	f := Fade{enabled: true}
	if f.Update(FadeTick(time.Now())) != nil {
		t.Error("transition scheduled frames before data was ready")
	}
	if f.View("loading") == "loading" {
		t.Error("loading view was shown at full brightness before the menu")
	}
	if f.Start() == nil {
		t.Fatal("ready menu did not start its fade-in")
	}
	if f.Update(FadeTick(f.started.Add(menuDuration/2))) == nil || f.progress != 0.5 {
		t.Errorf("halfway transition = %v", f.progress)
	}
	if f.Update(FadeTick(f.started.Add(menuDuration))) != nil || f.View("menu") != "menu" {
		t.Error("completed transition kept animating or changed the final view")
	}
}

func TestDeathStarAppearsAndFadesToDarkness(t *testing.T) {
	star, colors := deathStar(), palette()
	colorCodes := regexp.MustCompile(`\x1b\[38;2;\d+;\d+;\d+m`)
	for _, progress := range []float64{0, 1} {
		for _, code := range colorCodes.FindAllString(frame(star, colors, 2, 2, progress), -1) {
			if code != colors[0] {
				t.Errorf("visible Death Star at progress %.1f: %q", progress, code)
			}
		}
	}
	if codes := colorCodes.FindAllString(frame(star, colors, 2, 2, 0.5), -1); len(codes) < 2 {
		t.Error("Death Star did not emerge into multiple brightness levels")
	}
}
