package main

import (
	"encoding/binary"
	"net"
	"syscall"
)

const (
	ethPALL  = 0x0003
	ethPARP  = 0x0806
	ethPIPv6 = 0x86DD
)

type rawSocket struct {
	fd      int
	ifIndex int
}

func newRawSocket(iface *net.Interface) (*rawSocket, error) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ethPALL)))
	if err != nil {
		return nil, err
	}
	addr := &syscall.SockaddrLinklayer{Protocol: htons(ethPALL), Ifindex: iface.Index}
	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	return &rawSocket{fd: fd, ifIndex: iface.Index}, nil
}

func (r *rawSocket) close() {
	if r == nil || r.fd < 0 {
		return
	}
	syscall.Close(r.fd)
	r.fd = -1
}

func (r *rawSocket) send(frame []byte, etherType uint16, dst net.HardwareAddr) error {
	if r == nil {
		return syscall.EINVAL
	}
	addr := &syscall.SockaddrLinklayer{
		Protocol: htons(etherType),
		Ifindex:  r.ifIndex,
		Halen:    uint8(len(dst)),
	}
	copy(addr.Addr[:], dst)
	return syscall.Sendto(r.fd, frame, 0, addr)
}

func buildEthernetFrame(dst, src net.HardwareAddr, etherType uint16, payload []byte) []byte {
	frame := make([]byte, 14+len(payload))
	copy(frame[0:6], dst)
	copy(frame[6:12], src)
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	copy(frame[14:], payload)
	return frame
}

func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}
