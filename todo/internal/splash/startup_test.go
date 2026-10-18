package splash

import (
	"strings"
	"testing"
	"time"
)

func TestStartupShortcutGuardAndAcceleration(t *testing.T) {
	star := deathStar()
	s := Startup{star: &star, base: darkness}
	start := time.Now()
	if s.AcceptKey(start) {
		t.Fatal("accepted key before the first frame")
	}
	if s.Start(80, 40, start) == nil {
		t.Fatal("did not schedule startup")
	}
	if s.AcceptKey(start.Add(InputDelay-time.Nanosecond)) || !s.accelerated.IsZero() {
		t.Fatal("shortcut residue accelerated the splash")
	}
	s.Update(StartupTick(start.Add(InputDelay)))
	before := s.progress
	if !s.AcceptKey(start.Add(InputDelay)) {
		t.Fatal("key at 500 ms was ignored")
	}
	s.Update(StartupTick(start.Add(InputDelay)))
	if s.progress != before {
		t.Fatal("acceleration jumped instead of continuing the animation")
	}
	s.AcceptKey(start.Add(InputDelay + 100*time.Millisecond))
	if s.accelerated != start.Add(InputDelay) {
		t.Fatal("another key restarted acceleration")
	}
	if s.Update(StartupTick(start.Add(650*time.Millisecond))) == nil || !s.Active() {
		t.Fatal("accelerated splash finished too early")
	}
	if s.Update(StartupTick(start.Add(670*time.Millisecond))) != nil || s.Active() {
		t.Fatal("accelerated splash did not finish at 8x speed")
	}
}

func TestStartupNormalDurationViewAndResize(t *testing.T) {
	star := deathStar()
	s := Startup{star: &star, base: darkness}
	start := time.Now()
	s.Start(80, 40, start)
	s.Update(StartupTick(start.Add(duration / 2)))
	view := s.View()
	if strings.Contains(view, "\x1b[1;1H") || strings.Contains(view, enterScreen) || strings.Contains(view, leaveScreen) {
		t.Fatal("startup moved the cursor or managed Bubble Tea's screen")
	}
	if got := strings.Count(view, "\n"); got != 34 {
		t.Fatalf("centered view has %d line breaks, want 34", got)
	}
	if s.Update(StartupTick(start.Add(duration-time.Nanosecond))) == nil || !s.Active() {
		t.Fatal("normal animation ended early")
	}
	if s.Update(StartupTick(start.Add(duration))) != nil || s.Active() {
		t.Fatal("normal animation did not finish")
	}
	s = Startup{star: &star}
	s.Start(80, 40, start)
	s.Start(81, 40, start)
	if s.Active() {
		t.Fatal("resize did not stop animation")
	}
}
