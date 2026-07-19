// go mod init mirror && go get github.com/gocolly/colly/v2
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "uso: %s <IP|HOST|URL>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "ej:  %s 10.129.95.241\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "ej:  %s http://10.129.95.241/\n", os.Args[0])
		os.Exit(2)
	}

	target := strings.TrimSpace(os.Args[1])
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	if !strings.HasSuffix(target, "/") {
		target += "/"
	}

	startURL, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	allowedHost := startURL.Host

	outDir := "mirror_" + safeName(allowedHost)
	_ = os.MkdirAll(outDir, 0o755)

	visitedPath := filepath.Join(outDir, "urls.txt")
	visitedFile, err := os.OpenFile(visitedPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer visitedFile.Close()

	var (
		mu   sync.Mutex
		seen = make(map[string]struct{})
	)

	markSeen := func(u string) bool {
		mu.Lock()
		defer mu.Unlock()
		if _, ok := seen[u]; ok {
			return false
		}
		seen[u] = struct{}{}
		fmt.Fprintln(visitedFile, u)
		return true
	}

	c := colly.NewCollector(
		colly.AllowedDomains(allowedHost),
		colly.MaxDepth(6),
	)

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 8})

	// Guarda cualquier respuesta (HTML, css, js, imgs, etc.)
	c.OnResponse(func(r *colly.Response) {
		saveResponse(outDir, r)
	})

	// Sigue links y assets dentro del mismo host
	c.OnHTML("a[href], link[href], script[src], img[src], source[src]", func(e *colly.HTMLElement) {
		attr := "href"
		if e.Attr("src") != "" {
			attr = "src"
		}

		raw := strings.TrimSpace(e.Attr(attr))
		if raw == "" ||
			strings.HasPrefix(raw, "mailto:") ||
			strings.HasPrefix(raw, "javascript:") ||
			strings.HasPrefix(raw, "#") {
			return
		}

		abs := e.Request.AbsoluteURL(raw)
		if abs == "" {
			return
		}

		pu, err := url.Parse(abs)
		if err != nil || pu.Host != allowedHost {
			return
		}

		if markSeen(abs) {
			_ = e.Request.Visit(abs)
		}
	})

	// Arranque
	markSeen(startURL.String())
	if err := c.Visit(startURL.String()); err != nil {
		panic(err)
	}
	c.Wait()

	fmt.Println("OK")
	fmt.Println("salida:", outDir)
	fmt.Println("urls:", visitedPath)
}

func saveResponse(outDir string, r *colly.Response) {
	u, err := url.Parse(r.Request.URL.String())
	if err != nil {
		return
	}

	p := u.Path
	if p == "" || strings.HasSuffix(p, "/") {
		p = p + "index.html"
	}

	ct := r.Headers.Get("Content-Type")
	ext := strings.ToLower(filepath.Ext(p))

	// Si parece HTML y no tiene ext, pon .html
	if ext == "" && strings.Contains(ct, "text/html") {
		p += ".html"
	}

	// Si hay query, mete hash para evitar colisiones
	if u.RawQuery != "" {
		h := sha1.Sum([]byte(u.RawQuery))
		p = strings.TrimSuffix(p, ext) + "_" + hex.EncodeToString(h[:6]) + ext
	}

	full := filepath.Join(outDir, filepath.FromSlash(strings.TrimPrefix(p, "/")))
	_ = os.MkdirAll(filepath.Dir(full), 0o755)
	_ = os.WriteFile(full, r.Body, 0o644)

	fmt.Println("saved:", full)
}

func safeName(s string) string {
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}
