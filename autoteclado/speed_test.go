package main

import (
	"flag"
	"io"
	"reflect"
	"testing"
	"time"
)

func TestResolveSpeed(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		clipboard bool
		want      typingSpeed
		wantErr   bool
	}{
		{"file default", nil, false, typingSpeed{45, 500, -1}, false},
		{"clipboard default", nil, true, typingSpeed{45, 500, 0}, false},
		{"maximum", []string{"-r"}, false, typingSpeed{45, 500, 0}, false},
		{"ten percent less", []string{"-r1"}, false, typingSpeed{45, 500, 1}, false},
		{"twenty percent less", []string{"-r2"}, true, typingSpeed{45, 500, 2}, false},
		{"ninety percent less", []string{"-r9"}, true, typingSpeed{45, 500, 9}, false},
		{"manual clipboard", []string{"-min", "10", "-max", "30"}, true, typingSpeed{10, 30, -1}, false},
		{"manual zero", []string{"-min", "0", "-max", "0"}, true, typingSpeed{0, 0, -1}, false},
		{"disabled preset", []string{"-r=false", "-r2"}, false, typingSpeed{45, 500, 2}, false},
		{"multiple presets", []string{"-r", "-r2"}, false, typingSpeed{}, true},
		{"two numbered presets", []string{"-r1", "-r2"}, true, typingSpeed{}, true},
		{"manual plus preset", []string{"-min", "45", "-r"}, false, typingSpeed{}, true},
		{"preset plus manual", []string{"-r2", "-max", "500"}, true, typingSpeed{}, true},
		{"negative pause", []string{"-min", "-1"}, false, typingSpeed{}, true},
		{"inverted range", []string{"-min", "600"}, false, typingSpeed{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			min := fs.Int("min", 45, "")
			max := fs.Int("max", 500, "")
			levels := registerRapidFlags(fs)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			got, err := resolveSpeed(fs, *min, *max, *levels, tt.clipboard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("speed = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRapidPacerRelativeSpeed(t *testing.T) {
	for level := 0; level <= 9; level++ {
		t.Run(rapidFlagName(level), func(t *testing.T) {
			start := time.Unix(0, 0)
			clock := start
			var sleeps int
			pace := rapidPacer(level, func() time.Time { return clock }, func(wait time.Duration) {
				if wait <= 0 {
					t.Fatalf("nonpositive wait: %v", wait)
				}
				sleeps++
				clock = clock.Add(wait)
			})
			// Cada envío tarda 252 ms. El objetivo es el porcentaje de velocidad
			// anunciado, no ese porcentaje de aumento de la duración.
			const sendTime = 252 * time.Millisecond
			for i := 0; i < 10; i++ {
				clock = clock.Add(sendTime)
				if pace != nil {
					pace(sendTime)
				}
			}
			elapsed := clock.Sub(start)
			// Velocidad relativa = tiempo dedicado a enviar / tiempo total.
			gotPercent := float64(10*sendTime) / float64(elapsed) * 100
			wantPercent := float64(100 - level*10)
			if diff := gotPercent - wantPercent; diff < -0.0001 || diff > 0.0001 {
				t.Errorf("speed = %.6f%%, want %.6f%%", gotPercent, wantPercent)
			}
			if level == 0 && (pace != nil || sleeps != 0) {
				t.Error("maximum speed must not schedule sleeps")
			}
		})
	}
}

func TestRapidPacerCompensatesForOversleep(t *testing.T) {
	clock := time.Unix(0, 0)
	var waits []time.Duration
	pace := rapidPacer(5, func() time.Time { return clock }, func(wait time.Duration) {
		waits = append(waits, wait)
		clock = clock.Add(wait)
		if len(waits) == 1 {
			clock = clock.Add(7 * time.Millisecond) // El sistema tarda más en despertar.
		}
	})
	for i := 0; i < 2; i++ {
		clock = clock.Add(10 * time.Millisecond)
		pace(10 * time.Millisecond)
	}
	want := []time.Duration{10 * time.Millisecond, 3 * time.Millisecond}
	if !reflect.DeepEqual(waits, want) {
		t.Errorf("waits = %v, want %v", waits, want)
	}
}
