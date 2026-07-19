# gocatty

gocatty is a tiny Go-based reverse shell listener that behaves more like a proper terminal than a plain `nc` session. When a connection arrives it puts your local TTY in raw mode, forwards stdin/stdout to the socket, and automatically tries to spawn a PTY on the remote side (using `python3`/`python`) plus exports `TERM=xterm-256color`. The goal is to immediately land in a fully interactive shell without manually typing the bootstrap commands.

## Features

- Listens on `0.0.0.0:1234` and prints when a remote host connects.
- Sets the local terminal to raw mode and restores it on exit (including Ctrl+C).
- Pipes stdin/stdout bidirectionally, like `nc -nlvp 1234`.
- On connect, sends the PTY + `TERM` commands so the remote shell is usable right away.

## Requirements

- Go 1.21+ (tested with the Go version declared in `go.mod`).
- `python3` or `python` available on the remote target so the PTY bootstrap succeeds. If neither exists you will still get a connection, but may need to fix the terminal manually.

## Installation

```bash
# Clone or download the repo, then from its root run:
go build -o gocatty .
```

This produces a `gocatty` binary in the current directory. You can also install it into your GOPATH/bin:

```bash
go install .
```

## Usage

1. Start the listener on the machine waiting for the reverse shell (port is optional, default `1234`):
   ```bash
   ./gocatty        # defaults to 1234
   ./gocatty 4444   # custom port
   ```
   The program prints `Listening on 0.0.0.0:<PORT>` and waits for a single connection.

2. Trigger your reverse shell payload to connect back to `host:1234`.

3. Once connected, gocatty logs the remote address, pushes the PTY bootstrap commands, and drops you into the remote shell with proper line editing/history support.

4. Press Ctrl+C to exit; the program restores your terminal settings automatically.

## Listener

### Flags

- `--script`: runs `./script <port>` before the listener starts.
- Usage: `./gocatty [port]` (port is optional; defaults to `1234`).
- `--listener PORT`: explicit listen port (alternative to positional `[port]`).
- `--windows`: disables the remote PTY bootstrap and enables local echo (useful for Windows shells).
- `--no-pty` / `--skip-pty`: do not send remote bootstrap commands.
- `--local-echo`: show what you type even if the remote shell does not echo.
- `--no-raw` / `--cooked`: do not put your local terminal in raw mode (useful for some Windows shells).
- `--crlf`: translate Enter to CRLF when sending (helpful for some Windows shells).

### Windows shells

If you are catching a Windows reverse shell, use `--windows` so you can see what you type and avoid sending Linux bootstrap commands:

```bash
./gocatty --windows 1234
```

### Customizing

Currently the listen address/port and the bootstrap command list are defined near the top of `listener.go`. Edit those values and rebuild if you need different behavior (e.g., another port or additional setup commands).

## Caveats

- Only a single connection is handled per run. Restart the binary for another session.
- PTY bootstrap relies on the remote shell accepting the commands verbatim. If the remote environment requires different tooling, adjust `bootstrapRemoteShell` accordingly.

Use responsibly and only against systems you are authorized to test.

## receiver (TCP file dropbox)

`cmd/receiver` is a small TCP listener that saves every incoming connection to disk. By default it binds to `0.0.0.0:1235` and writes sequential filenames such as `file1`, `file2`, etc., creating directories on the fly if your prefix includes a path. It creates output files exclusively so existing files are not overwritten, accepts multiple transfers concurrently, prints the active transfer count plus human-readable byte totals, and closes active connections on Ctrl+C so shutdown does not hang on a stalled client. It also logs the available local IPv4 addresses so you know which IP to target from the sender.

### Install from GitHub

```bash
go install github.com/4rji/binarios-go/listener/cmd/receiver@latest
```

This installs a `receiver` binary into `$GOBIN` (or `$GOPATH/bin`). Requires Go 1.21+.

### Run from source

```bash
go run ./cmd/receiver                                      # defaults to port 1235, prefix "file"
go run ./cmd/receiver 9000 loot-                           # custom port and filename prefix
go run ./cmd/receiver --port 9000 --prefix loot-           # explicit flags
go run ./cmd/receiver --output-dir incoming --prefix drop- # save into incoming/drop-N
```

From another host, send a file with netcat:

```bash
nc -q 0 <listener-ip> 1235 < payload.bin
```

Press Ctrl+C to stop the listener; it shuts down gracefully.
