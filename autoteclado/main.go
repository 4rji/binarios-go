// autoteclado simula actividad de tecleo con robotgo (modos -f y -p) o dentro de una
// ventana falsa en la terminal (modo por defecto).
//
// Sin flags:   abre una "ventana" falsa dentro de la terminal y "teclea" dentro
//
//	un script de Python de ejemplo (escaneo de red con nmap) en ráfagas de 10
//	caracteres con pausas de 20 segundos; mueve el ratón real en cada pausa y
//	repite el script en bucle. No toca el teclado real ni necesita permisos de
//	accesibilidad para el tecleo. Ctrl+C sale limpio sin guardar nada: usa el
//	buffer de pantalla alternativo, así que al salir la terminal queda igual.
//
// Con -f FILE: teclea únicamente el contenido de FILE con el teclado real y
//
//	termina (sin ratón, sin ventana).
//
// Con -p: teclea el texto del portapapeles de macOS, a velocidad máxima por
// defecto, tras 3 segundos. -t cambia la espera y -r/-r1…-r9 la velocidad.
//
// Funciona en macOS, Linux (X11) y Windows. En macOS hay que dar permiso a la
// terminal/app en Ajustes del Sistema → Privacidad y Seguridad → Accesibilidad
// para que el movimiento del ratón (y los modos -f/-p) funcionen.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-vgo/robotgo"
	"golang.org/x/term"
)

func main() {
	file := flag.String("f", "", "`archivo` cuyo contenido se teclea en la ventana enfocada; al terminar, sale")
	clipboard := flag.Bool("p", false, "teclea el texto del portapapeles de macOS; velocidad máxima por defecto")
	startSeconds := flag.Int("t", 3, "`segundos` de espera antes de empezar; tiempo para enfocar la ventana (con -f o -p)")
	minMs := flag.Int("min", 45, "pausa mínima entre teclas en `ms`; menor pausa = más velocidad")
	maxMs := flag.Int("max", 500, "pausa máxima entre teclas en `ms`; debe ser mayor o igual que -min")
	rapidLevels := registerRapidFlags(flag.CommandLine)
	flag.Usage = printUsage
	flag.Parse()

	if *clipboard && *file != "" {
		printError("elige una sola fuente de texto: -f archivo o -p para el portapapeles")
		os.Exit(2)
	}
	if *startSeconds < 0 {
		printError("-t debe ser 0 o más segundos de espera antes de empezar")
		os.Exit(2)
	}
	speed, err := resolveSpeed(flag.CommandLine, *minMs, *maxMs, *rapidLevels, *clipboard)
	if err != nil {
		printError("%v", err)
		os.Exit(2)
	}
	if speed.rapidLevel >= 0 && *file == "" && !*clipboard {
		printError("las velocidades -r/-r1…-r9 requieren -f archivo o -p")
		os.Exit(2)
	}

	// Lee la fuente antes de la cuenta atrás y teclea ese contenido una sola vez.
	if *file != "" || *clipboard {
		var data []byte
		label := "Tecleando el archivo"
		if *clipboard {
			data, err = clipboardText()
			label = "Tecleando el portapapeles"
		} else {
			data, err = os.ReadFile(*file)
			if err != nil {
				err = fmt.Errorf("no se pudo leer %q: %w", *file, err)
			}
		}
		if err != nil {
			printError("%v", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, styleFor(os.Stderr).paint(colorCyan, "Velocidad: "+speed.description()))
		countdown(*startSeconds, label)
		keyboard := keyboardWriter{keyTap: robotgo.KeyTap, typeUnicode: robotgo.Type}
		if err := keyboard.typeText(string(data), speed.pacer()); err != nil {
			printError("%v", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "\n"+styleFor(os.Stderr).paint(colorGreen, "Listo."))
		return
	}

	// Por defecto: ventana falsa que "teclea" un script de Python en bucle y
	// mueve el ratón. Ctrl+C sale limpio sin guardar nada.
	fakeWindow(*minMs, *maxMs)
}

func printUsage() {
	out := flag.CommandLine.Output()
	style := styleFor(out)
	fmt.Fprintln(out, style.paint(colorHeading, "AUTOTECLADO"))
	fmt.Fprintf(out, "\n%s %s\n", style.paint(colorHeading, "Uso:"),
		style.paint(colorGreen, os.Args[0]+" (-f archivo | -p) [-t segundos]"))
	fmt.Fprintln(out, "     "+style.paint(colorGreen, "[-r | -r1 … -r9 | -min ms -max ms]"))
	fmt.Fprintln(out, style.paint(colorDim, "Sin -f ni -p: abre la ventana falsa y mueve el ratón. Ctrl+C para salir."))
	fmt.Fprintln(out, "\n"+style.paint(colorHeading, "Opciones:"))
	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == "r" || (len(f.Name) == 2 && f.Name[0] == 'r' && f.Name[1] >= '1' && f.Name[1] <= '9') {
			return // Se muestran juntos en la tabla de velocidades.
		}
		name, usage := flag.UnquoteUsage(f)
		fmt.Fprint(out, "  "+style.paint(colorCyan, "-"+f.Name))
		if name != "" {
			fmt.Fprint(out, " "+name)
		}
		fmt.Fprint(out, "\n      "+usage)
		if f.DefValue != "" && f.DefValue != "false" {
			fmt.Fprintf(out, " %s", style.paint(colorDim, "(por defecto: "+f.DefValue+")"))
		}
		fmt.Fprintln(out)
	})
	fmt.Fprintln(out, "\n"+style.paint(colorHeading, "Velocidades rápidas (con -f o -p):"))
	for level := 0; level <= 9; level++ {
		color := colorGreen
		if level >= 6 {
			color = colorYellow
		} else if level >= 3 {
			color = colorCyan
		}
		fmt.Fprintf(out, "  %s %s\n", style.paint(color, fmt.Sprintf("%-4s", "-"+rapidFlagName(level))), rapidDescription(level))
	}
	fmt.Fprintln(out, "  Con -p se usa -r por defecto. Todos los porcentajes se comparan con -r.")
	fmt.Fprintln(out, "  La velocidad real depende del sistema y de la aplicación que recibe las teclas.")
	fmt.Fprintln(out, "  Elige un solo nivel; no se combina con -min/-max.")
	fmt.Fprintln(out, "\n"+style.paint(colorHeading, "Velocidad manual (-min y -max):"))
	fmt.Fprintln(out, "  Después de cada tecla espera una pausa aleatoria entre -min y -max.")
	fmt.Fprintln(out, "  Son milisegundos: "+style.paint(colorCyan, "1000 ms = 1 segundo")+". "+
		style.paint(colorGreen, "Valores menores = más rápido."))
	fmt.Fprintln(out, "  Por ejemplo, -min 10 -max 30 espera entre 0,01 y 0,03 segundos por tecla.")
	fmt.Fprintln(out, "  Con -f, sin opciones de velocidad, las pausas son de 45 a 500 ms.")
	fmt.Fprintln(out, "  "+style.paint(colorYellow, "-t es solo la espera inicial en segundos; no cambia la velocidad."))
	fmt.Fprintln(out, "\n"+style.paint(colorHeading, "Ejemplos:"))
	fmt.Fprintf(out, "  Portapapeles rápido, tras 3 segundos: %s\n", style.paint(colorGreen, os.Args[0]+" -p"))
	fmt.Fprintf(out, "  Archivo a velocidad máxima:          %s\n", style.paint(colorGreen, os.Args[0]+" -f notas.txt -t 5 -r"))
	fmt.Fprintf(out, "  Portapapeles un 20%% más lento:        %s\n", style.paint(colorCyan, os.Args[0]+" -p -r2"))
	fmt.Fprintf(out, "  Portapapeles a la mitad de velocidad: %s\n", style.paint(colorYellow, os.Args[0]+" -p -t 5 -r5"))
}

// countdown avisa por stderr y espera n segundos para que el usuario enfoque
// la ventana objetivo.
func countdown(n int, label string) {
	style := styleFor(os.Stderr)
	for i := n; i > 0; i-- {
		fmt.Fprintf(os.Stderr, "\r%s ", style.paint(colorYellow, fmt.Sprintf("%s en %d...", label, i)))
		time.Sleep(time.Second)
	}
	fmt.Fprintf(os.Stderr, "\r%s        \n", style.paint(colorGreen, label+": ¡ya!"))
}

// keyboardWriter separa el envío de eventos para poder comprobar el tecleo sin
// escribir en la aplicación que tenga el foco durante las pruebas.
type keyboardWriter struct {
	keyTap      func(string, ...interface{}) error
	typeUnicode func(string, ...int)
}

// typeText envía ASCII imprimible como pulsaciones reales, incluidos los
// modificadores que KeyTap añade para mayúsculas y símbolos. En macOS, el envío
// Unicode usa siempre el código de tecla 0 (A), que algunas consolas web leen
// en lugar del texto. Se reserva ese envío para caracteres fuera de ASCII.
func (k keyboardWriter) typeText(text string, pace func(time.Duration)) error {
	for _, r := range text {
		start := time.Now()
		var err error
		switch r {
		case '\n':
			err = k.keyTap("enter")
		case '\t':
			err = k.keyTap("tab")
		case '\r':
			// ignora el retorno de carro (archivos CRLF)
			continue
		case ' ':
			err = k.keyTap("space")
		default:
			if r >= '!' && r <= '~' {
				err = k.keyTap(string(r))
			} else {
				k.typeUnicode(string(r))
			}
		}
		if err != nil {
			return fmt.Errorf("no se pudo teclear %q: %w", r, err)
		}
		if pace != nil {
			pace(time.Since(start))
		}
	}
	return nil
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

// --- Ventana falsa (modo por defecto) ----------------------------------------
//
// Dibuja una ventana con caracteres de caja dentro de la propia terminal y
// "teclea" dentro de ella un script de Python (de ejemplo: escaneo de red con
// nmap). Teclea en ráfagas de 10 caracteres y luego hace una pausa de 20
// segundos; durante cada pausa mueve el ratón real. Al terminar el script
// vuelve a empezar en bucle. Usa el buffer de pantalla alternativo para que
// Ctrl+C restaure la terminal tal como estaba (no guarda nada en el historial).

const (
	burstChars = 10               // caracteres por ráfaga antes de pausar
	burstPause = 20 * time.Second // pausa entre ráfagas y entre repeticiones
)

// fakeWindow dibuja la ventana y teclea el script en bucle hasta Ctrl+C.
func fakeWindow(minMs, maxMs int) {
	cols, rows := termSize()

	winW := cols - 4
	winH := rows - 4
	if winW < 24 {
		winW = 24
	}
	if winH < 6 {
		winH = 6
	}
	left := (cols-winW)/2 + 1
	top := (rows-winH)/2 + 1
	if left < 1 {
		left = 1
	}
	if top < 1 {
		top = 1
	}

	contentLeft := left + 2
	contentTop := top + 3
	contentW := winW - 4
	contentH := winH - 4

	// Entra en el buffer alternativo y oculta el cursor.
	fmt.Print("\033[?1049h\033[?25l")
	restore := func() { fmt.Print("\033[?25h\033[?1049l") }

	// Ctrl+C (o SIGTERM): restaura la terminal y sale sin guardar nada.
	// os.Exit corta cualquier pausa en curso.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		restore()
		os.Exit(0)
	}()

	drawFrame(left, top, winW, winH, "scan.py — nmap (Ctrl+C para salir)")

	move := func(r, c int) { fmt.Printf("\033[%d;%dH", r, c) }

	// mu serializa todas las escrituras a la terminal, porque el contador de
	// segundos corre en otra goroutine y no debe mezclarse con el tecleo.
	var mu sync.Mutex
	start := time.Now()

	lines := make([]string, 0, contentH) // líneas ya terminadas, visibles
	cur := ""                            // línea en construcción

	// redrawLocked repinta el contenido. El llamador ya debe tener mu tomado.
	redrawLocked := func() {
		for i := 0; i < contentH; i++ {
			line := ""
			if i < len(lines) {
				line = lines[i]
			}
			move(contentTop+i, contentLeft)
			fmt.Printf("\033[32m%s\033[0m", padRunes(line, contentW))
		}
	}

	// putChar añade un carácter al contenido, gestionando saltos de línea y
	// el ajuste de línea cuando se llena el ancho.
	putChar := func(ch rune) {
		mu.Lock()
		defer mu.Unlock()
		if ch == '\n' || len([]rune(cur)) >= contentW {
			lines = append(lines, cur)
			cur = ""
			if len(lines) >= contentH {
				lines = lines[len(lines)-(contentH-1):] // desplaza hacia arriba
			}
			redrawLocked()
			if ch == '\n' {
				return
			}
		}
		cur += string(ch)
		move(contentTop+len(lines), contentLeft+len([]rune(cur))-1)
		fmt.Printf("\033[32m%c\033[0m", ch)
	}

	// Contador de segundos en el borde inferior, actualizado cada segundo por
	// una goroutine (sigue contando también durante las pausas).
	bottomRow := top + winH - 1
	drawCounter := func() {
		mu.Lock()
		defer mu.Unlock()
		move(bottomRow, left+2)
		fmt.Printf("\033[36m[ %ds ]\033[0m", int(time.Since(start).Seconds()))
	}
	go func() {
		drawCounter()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			drawCounter()
		}
	}()

	script := pythonScript()
	for { // repite el script en bucle
		lines = lines[:0]
		cur = ""
		mu.Lock()
		redrawLocked()
		mu.Unlock()

		typed := 0
		for _, ch := range script {
			putChar(ch)
			sleepRand(minMs, maxMs)
			typed++
			if typed%burstChars == 0 {
				randomMouseMove()      // mueve el ratón real durante la pausa
				time.Sleep(burstPause) // la interrumpe Ctrl+C vía os.Exit
			}
		}
		// Pausa antes de reiniciar (si la última ráfaga no fue completa).
		if typed%burstChars != 0 {
			randomMouseMove()
			time.Sleep(burstPause)
		}
	}
}

// pythonScript devuelve un script de Python de ejemplo (escaneo de red sencillo
// con sockets y nmap) que se teclea dentro de la ventana falsa.
func pythonScript() string {
	return `#!/usr/bin/env python3
# Escaneo basico de una red /24 y nmap sobre los hosts con puertos abiertos.
import socket
import subprocess

RED = "192.168.1."
PUERTOS = [22, 80, 443, 3389, 8080]

def abierto(ip, puerto):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(0.5)
    ok = s.connect_ex((ip, puerto)) == 0
    s.close()
    return ok

def nmap(ip):
    r = subprocess.run(["nmap", "-sV", "-Pn", ip],
                       capture_output=True, text=True)
    return r.stdout

def main():
    for host in range(1, 255):
        ip = RED + str(host)
        for puerto in PUERTOS:
            if abierto(ip, puerto):
                print(f"[+] {ip}:{puerto} abierto")
                print(nmap(ip))

if __name__ == "__main__":
    main()
`
}

// drawFrame dibuja el marco de la ventana: borde superior, barra de título,
// separador, filas de contenido vacías y borde inferior.
func drawFrame(left, top, w, h int, title string) {
	move := func(r, c int) { fmt.Printf("\033[%d;%dH", r, c) }
	const cyan, reset = "\033[36m", "\033[0m"
	inner := w - 2

	move(top, left)
	fmt.Printf("%s┌%s┐%s", cyan, strings.Repeat("─", inner), reset)

	bar := "● ● ●  " + title
	move(top+1, left)
	fmt.Printf("%s│%s %s%s│%s", cyan, reset, padRunes(bar, inner-1), cyan, reset)

	move(top+2, left)
	fmt.Printf("%s├%s┤%s", cyan, strings.Repeat("─", inner), reset)

	for i := 0; i < h-4; i++ {
		move(top+3+i, left)
		fmt.Printf("%s│%s%s%s│%s", cyan, reset, strings.Repeat(" ", inner), cyan, reset)
	}

	move(top+h-1, left)
	fmt.Printf("%s└%s┘%s", cyan, strings.Repeat("─", inner), reset)
}

// padRunes recorta o rellena con espacios s hasta exactamente w runas, para que
// el ancho visible sea constante (los caracteres de caja son multibyte).
func padRunes(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return string(r[:w])
	}
	return s + strings.Repeat(" ", w-len(r))
}

// termSize devuelve el tamaño de la terminal, o 80x24 si no se puede detectar.
func termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24
	}
	return w, h
}
