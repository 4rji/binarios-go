# pingm

`pingm` is a small Go CLI that checks which SSH hosts from your `~/.ssh/config` are reachable.

It reads each `Host` entry, resolves its `HostName` and `Port` values, then tries a short TCP connection to report hosts as online or offline. Online hosts include an approximate connection latency in milliseconds, and results are grouped by host-name prefix for easier scanning.

## What it does

- Reads SSH host definitions from `~/.ssh/config`.
- Uses `HostName` and `Port` when present; defaults to port `22`.
- Checks hosts concurrently with a 1-second TCP timeout.
- Prints separate `Offline` and `Online` sections.
- Shows latency for reachable hosts.

## Usage

```sh
./pingm
```

No arguments are required. The tool only inspects your SSH config and attempts TCP connectivity checks; it does not start SSH sessions.
