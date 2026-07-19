//go:build aix || linux || solaris || zos

package main

import "golang.org/x/sys/unix"

const termiosReadReq = unix.TCGETS
const termiosWriteReq = unix.TCSETS
