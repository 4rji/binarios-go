// listener.go – recibe reverse shells y Ctrl‑C no lo cierra
package main

import (
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

func main() {
	ln, err := net.Listen("tcp", ":1234") // puerto local
	if err != nil { return }
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil { return }
	defer conn.Close()

	// ignora Ctrl‑C y Ctrl‑\
	signal.Ignore(syscall.SIGINT, syscall.SIGQUIT)

	// terminal local en raw para que ^C pase al remoto
	oldState, _ := term.MakeRaw(int(os.Stdin.Fd()))
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go io.Copy(conn, os.Stdin) // teclado → red
	io.Copy(os.Stdout, conn)   // red → pantalla
}