# Deadnet (Go)

Reimplementation of the original Python-based Deadnet tool in Go. It retains the same command-line options and attack flow (ARP poisoning for IPv4 plus Router Advertisement spoofing for IPv6) but no longer depends on Python or Scapy.


## Usage
Run the binary as root because raw sockets are required. Example:
```bash
wifito -i eth0 -m 24 -s 5
```

### Options
- `-i, --network-interface` (required): Interface name (e.g., `eth0`).
- `-m, --set-cidrlen`: IPv4 subnet CIDR length (default `/24`).
- `-s, --sleep-interval`: Seconds to wait between poisoning cycles (default `5`).
- `-g, --gateway-ipv4`: Manually set the IPv4 gateway.
- `-M, --gateway-mac`: Manually set the gateway MAC address.
- `-6, --disable-ipv6`: Disable IPv6 RA spoofing.
- `-p, --set-preflen`: IPv6 prefix length for the RA prefix option (default `64`).

The Go binary prints the same colored status messages as the Python script and exits cleanly on `Ctrl+C`.
