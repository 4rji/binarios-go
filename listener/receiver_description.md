Receiver (TCP file listener)

What it does:
- Listens on 0.0.0.0 for incoming TCP connections (default port 1235).
- Saves each connection’s payload to disk as sequential files like file1, file2, etc.
- Prints your local IPv4 interfaces so you know which IP to target from the sender.

How to run from source:
- Default settings: go run ./cmd/receiver
- Custom port and filename prefix: go run ./cmd/receiver 9000 loot-
- Send from another host: nc -q 0 <listener-ip> 1235 < payload.bin
- Stop with Ctrl+C; the listener shuts down cleanly.

How to install from GitHub:
- Install with Go: go install github.com/4rji/binarios-go/listener/cmd/receiver@latest
- The binary lands in $GOBIN (or $GOPATH/bin). Requires Go 1.21+.
