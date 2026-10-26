package main

import (
	"flag"
	"fmt"
	"time"
)

type typingSpeed struct {
	minMs, maxMs int
	rapidLevel   int // -1: pausas manuales; 0: máxima; 1..9: reducción de 10..90%.
}

func rapidFlagName(level int) string {
	if level == 0 {
		return "r"
	}
	return fmt.Sprintf("r%d", level)
}

func rapidDescription(level int) string {
	if level == 0 {
		return "máxima, sin pausas añadidas"
	}
	return fmt.Sprintf("%d%% de la máxima (%d%% menos, aproximadamente)", 100-level*10, level*10)
}

func registerRapidFlags(fs *flag.FlagSet) *[10]bool {
	levels := new([10]bool)
	for level := range levels {
		fs.BoolVar(&levels[level], rapidFlagName(level), false, rapidDescription(level)+" (con -f o -p)")
	}
	return levels
}

func resolveSpeed(fs *flag.FlagSet, minMs, maxMs int, levels [10]bool, clipboard bool) (typingSpeed, error) {
	speed := typingSpeed{minMs: minMs, maxMs: maxMs, rapidLevel: -1}
	if minMs < 0 || maxMs < minMs {
		return speed, fmt.Errorf("las pausas deben cumplir 0 <= -min <= -max (en milisegundos)")
	}
	for level, selected := range levels {
		if !selected {
			continue
		}
		if speed.rapidLevel >= 0 {
			return speed, fmt.Errorf("elige una sola velocidad: -r o un nivel entre -r1 y -r9")
		}
		speed.rapidLevel = level
	}
	manual := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "min" || f.Name == "max" {
			manual = true
		}
	})
	if speed.rapidLevel >= 0 && manual {
		return speed, fmt.Errorf("elige una velocidad -r/-r1…-r9 o pausas -min/-max; no se pueden combinar")
	}
	if clipboard && speed.rapidLevel < 0 && !manual {
		speed.rapidLevel = 0
	}
	return speed, nil
}

func (s typingSpeed) description() string {
	if s.rapidLevel >= 0 {
		return "-" + rapidFlagName(s.rapidLevel) + ": " + rapidDescription(s.rapidLevel)
	}
	return fmt.Sprintf("pausas entre %d y %d ms por tecla", s.minMs, s.maxMs)
}

func (s typingSpeed) pacer() func(time.Duration) {
	if s.rapidLevel < 0 {
		return func(time.Duration) { sleepRand(s.minMs, s.maxMs) }
	}
	return rapidPacer(s.rapidLevel, time.Now, time.Sleep)
}

// rapidPacer calcula la espera a partir del tiempo real de envío de las teclas.
// A un 80% de velocidad, el tiempo total debe ser envío / 0,8 (no envío * 1,2).
// El objetivo acumulado compensa las esperas que el sistema alargue al dormir.
func rapidPacer(level int, now func() time.Time, sleep func(time.Duration)) func(time.Duration) {
	if level == 0 {
		return nil
	}
	start := now()
	var target time.Duration
	return func(sendTime time.Duration) {
		target += sendTime * 10 / time.Duration(10-level)
		if remaining := start.Add(target).Sub(now()); remaining > 0 {
			sleep(remaining)
		}
	}
}
