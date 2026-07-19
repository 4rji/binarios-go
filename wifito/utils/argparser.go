package utils

import (
	"flag"
	"fmt"
	"os"
)

const (
	defaultCIDR     = 24
	defaultSleepSec = 5
	defaultPrefLen  = 64
)

type Arguments struct {
	Iface         string
	CIDRLen       int
	SleepInterval int
	GatewayIPv4   string
	GatewayMAC    string
	DisableIPv6   bool
	PrefixLen     int
}

func ParseArgs() Arguments {
	args := Arguments{}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s -i <iface> [options]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&args.Iface, "i", "", "network interface (ex: eth0)")
	flag.StringVar(&args.Iface, "network-interface", "", "network interface (ex: eth0)")
	flag.IntVar(&args.CIDRLen, "m", defaultCIDR, fmt.Sprintf("set IPv4 subnet cidr length (default /%d)", defaultCIDR))
	flag.IntVar(&args.CIDRLen, "set-cidrlen", defaultCIDR, fmt.Sprintf("set IPv4 subnet cidr length (default /%d)", defaultCIDR))
	flag.IntVar(&args.SleepInterval, "s", defaultSleepSec, fmt.Sprintf("sleep time between arp poison attempts (default %d sec)", defaultSleepSec))
	flag.IntVar(&args.SleepInterval, "sleep-interval", defaultSleepSec, fmt.Sprintf("sleep time between arp poison attempts (default %d sec)", defaultSleepSec))
	flag.StringVar(&args.GatewayIPv4, "g", "", "set gateway IPv4 manually")
	flag.StringVar(&args.GatewayIPv4, "gateway-ipv4", "", "set gateway IPv4 manually")
	flag.StringVar(&args.GatewayMAC, "M", "", "set gateway MAC address manually")
	flag.StringVar(&args.GatewayMAC, "gateway-mac", "", "set gateway MAC address manually")
	flag.BoolVar(&args.DisableIPv6, "6", false, "disable IPv6 dead router attack")
	flag.BoolVar(&args.DisableIPv6, "disable-ipv6", false, "disable IPv6 dead router attack")
	flag.IntVar(&args.PrefixLen, "p", defaultPrefLen, fmt.Sprintf("set IPv6 prefix length (default %d)", defaultPrefLen))
	flag.IntVar(&args.PrefixLen, "set-preflen", defaultPrefLen, fmt.Sprintf("set IPv6 prefix length (default %d)", defaultPrefLen))

	flag.Parse()

	if args.Iface == "" {
		flag.Usage()
		os.Exit(1)
	}

	return args
}
