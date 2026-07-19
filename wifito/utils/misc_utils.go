package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var macRegex = regexp.MustCompile(`^([0-9a-fA-F]{2}[:-]){5}([0-9a-fA-F]{2})$`)

func IsValidMAC(mac string) bool {
	return macRegex.MatchString(mac)
}

func RandomMAC() (net.HardwareAddr, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	buf[0] &= 0xfe
	buf[0] |= 0x02
	return net.HardwareAddr(buf), nil
}

func MacToIPv6LL(macAddr, prefix string) (string, error) {
	hw, err := net.ParseMAC(macAddr)
	if err != nil {
		return "", err
	}
	if len(hw) != 6 {
		return "", fmt.Errorf("invalid mac length")
	}
	eui := make([]byte, 8)
	copy(eui[0:3], hw[0:3])
	eui[3] = 0xff
	eui[4] = 0xfe
	copy(eui[5:], hw[3:])
	eui[0] ^= 0x02

	pref := prefix
	if !strings.Contains(pref, "::") {
		pref = fmt.Sprintf("%s::", pref)
	}
	ip := net.ParseIP(pref)
	if ip == nil {
		return "", fmt.Errorf("invalid prefix")
	}
	base := ip.To16()
	if base == nil {
		return "", fmt.Errorf("invalid IPv6 prefix")
	}
	ll := make(net.IP, net.IPv6len)
	copy(ll[:8], base[:8])
	copy(ll[8:], eui)
	return ll.String(), nil
}

func GetTimestampMs() int64 {
	return time.Now().UnixNano() / 1_000_000
}

func OSIsLinux() bool {
	return runtime.GOOS == "linux"
}

func OSIsWindows() bool {
	return runtime.GOOS == "windows"
}

func IPv6Checksum(src, dst net.IP, payload []byte) uint16 {
	sum := checksum16(src.To16())
	sum += checksum16(dst.To16())

	pseudo := make([]byte, 8)
	binary.BigEndian.PutUint32(pseudo[0:4], uint32(len(payload)))
	pseudo[7] = 58
	sum += checksum16(pseudo)
	sum += checksum16(payload)

	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func checksum16(data []byte) uint32 {
	var sum uint32
	for i := 0; i+1 <= len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}
