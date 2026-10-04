// autoteclado simula tecleo y movimiento de ratón "humano" usando robotgo.
//
// Sin flags:   escribe texto aleatorio y mueve el ratón a puntos aleatorios
//
//	en bucle, con 20 segundos de pausa entre ciclos, hasta Ctrl+C.
//
// Con -t FILE: teclea únicamente el contenido de FILE y termina (sin ratón).
//
// Funciona en macOS, Linux (X11) y Windows. En macOS hay que dar permiso a la
// terminal/app en Ajustes del Sistema → Privacidad y Seguridad → Accesibilidad.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-vgo/robotgo"
)

func main() {
	file := flag.String("t", "", "archivo a teclear (solo teclea su contenido y termina)")
	minMs := flag.Int("min", 45, "retardo mínimo entre teclas en ms")
	maxMs := flag.Int("max", 500, "retardo máximo entre teclas en ms")
	lead := flag.Int("lead", 3, "segundos de margen antes de empezar (para enfocar la ventana)")
	flag.Parse()

	if *minMs < 0 || *maxMs < *minMs {
		fmt.Fprintln(os.Stderr, "error: -min/-max inválidos (se requiere 0 <= min <= max)")
		os.Exit(2)
	}

	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: no se pudo leer %q: %v\n", *file, err)
			os.Exit(1)
		}
		countdown(*lead, "Tecleando el archivo")
		typeText(string(data), *minMs, *maxMs)
		fmt.Fprintln(os.Stderr, "\nListo.")
		return
	}

	// Modo aleatorio: Ctrl+C para salir.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	countdown(*lead, "Modo aleatorio (Ctrl+C para parar)")
	fmt.Fprintln(os.Stderr, "escribiendo y moviendo el ratón...")

	for {
		select {
		case <-stop:
			fmt.Fprintln(os.Stderr, "\nParado.")
			return
		default:
		}
		randomMouseMove()
		typeText(randomText(), *minMs, *maxMs)
		fmt.Fprintln(os.Stderr, "pausa de 20 segundos...")
		timer := time.NewTimer(20 * time.Second)
		select {
		case <-stop:
			timer.Stop()
			fmt.Fprintln(os.Stderr, "\nParado.")
			return
		case <-timer.C:
		}
	}
}

// countdown avisa por stderr y espera n segundos para que el usuario enfoque
// la ventana objetivo.
func countdown(n int, label string) {
	for i := n; i > 0; i-- {
		fmt.Fprintf(os.Stderr, "\r%s en %d... ", label, i)
		time.Sleep(time.Second)
	}
	fmt.Fprintf(os.Stderr, "\r%s: ¡ya!        \n", label)
}

// typeText teclea el texto carácter a carácter con retardos aleatorios para
// imitar a una persona. Los saltos de línea se envían como Enter.
func typeText(text string, minMs, maxMs int) {
	for _, r := range text {
		switch r {
		case '\n':
			robotgo.KeyTap("enter")
		case '\t':
			robotgo.KeyTap("tab")
		case '\r':
			// ignora el retorno de carro (archivos CRLF)
			continue
		default:
			robotgo.TypeStr(string(r))
		}
		sleepRand(minMs, maxMs)
	}
}

// randomText genera una frase corta aleatoria de palabras inventadas.
func randomText() string {
	const letras = "abcdefghijklmnopqrstuvwxyz"
	nPalabras := 2 + rand.Intn(6)
	palabras := make([]string, nPalabras)
	for i := range palabras {
		n := 3 + rand.Intn(7)
		var sb strings.Builder
		for j := 0; j < n; j++ {
			sb.WriteByte(letras[rand.Intn(len(letras))])
		}
		palabras[i] = sb.String()
	}
	out := strings.Join(palabras, " ")
	if rand.Intn(3) == 0 {
		out += "\n"
	} else {
		out += " "
	}
	return out
}

// randomMouseMove mueve el ratón de forma suave a un punto aleatorio de la
// pantalla.
func randomMouseMove() {
	w, h := robotgo.GetScreenSize()
	if w <= 0 || h <= 0 {
		return
	}
	x := rand.Intn(w)
	y := rand.Intn(h)
	robotgo.MoveSmooth(x, y, 0.5, 2.0)
}

// sleepRand duerme un tiempo aleatorio en el rango [minMs, maxMs] ms.
func sleepRand(minMs, maxMs int) {
	if maxMs <= minMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	d := minMs + rand.Intn(maxMs-minMs+1)
	time.Sleep(time.Duration(d) * time.Millisecond)
}
