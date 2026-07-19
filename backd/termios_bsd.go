//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package main

import "golang.org/x/sys/unix"

const termiosReadReq = unix.TIOCGETA
const termiosWriteReq = unix.TIOCSETA
