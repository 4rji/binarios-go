package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("Uso: liveserver ruta/pagina.html")
	}
	// Obtiene ruta absoluta del HTML
	target, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	url := "file://" + target

	// 1) Arranca Chromium (no headless) con rod
	u := launcher.New().
		Headless(false).
		MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// 2) Abre la página
	page := browser.MustPage(url)

	// 3) Configura watcher de fsnotify
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(target); err != nil {
		log.Fatal(err)
	}

	// 4) Al detectar WRITE, recarga (debounce 500 ms)
	go func() {
		var last time.Time
		for ev := range w.Events {
			if ev.Op&fsnotify.Write == fsnotify.Write {
				if time.Since(last) > 500*time.Millisecond {
					last = time.Now()
					log.Println("Recargando página…")
					page.MustReload()
				}
			}
		}
	}()
	go func() {
		for err := range w.Errors {
			log.Println("watcher error:", err)
		}
	}()

	// 5) Mantiene el proceso vivo
	select {}
}