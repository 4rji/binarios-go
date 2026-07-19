package utils

import (
	"fmt"
	"os"
	"sync"
)

var (
	muted bool
	mu    sync.Mutex
)

func InvalidatePrint() {
	mu.Lock()
	muted = true
	mu.Unlock()
}

func Printf(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func PrintRaw(text string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprint(os.Stdout, text)
}

func IsMuted() bool {
	mu.Lock()
	defer mu.Unlock()
	return muted
}
