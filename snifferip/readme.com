# Raw IP Sniffer (Go)

Minimal raw-socket network sniffer written in Go.  
It prints **protocol, source IP, and destination IP** for captured packets.

## Files

### `icmp.go` (Linux / macOS)
- Uses `SOCK_RAW + IPPROTO_ICMP`
- Captures **only ICMP**
- Detects:
  - ping
  - traceroute
  - ICMP errors
- Equivalent to the original Python script on Linux

### `main.go` (Windows)
- Uses `SOCK_RAW + IPPROTO_IP`
- Enables **promiscuous mode** with `SIO_RCVALL`
- Captures **all IPv4 traffic**:
  - ICMP
  - TCP
  - UDP
- Equivalent to the original Python script on Windows

## Output
