# sshc

`sshc` is a small helper to run the same command across every host defined in your local `~/.ssh/config`. It executes the command on each host concurrently over SSH, prints successful hosts in green, and highlights offline hosts in red.

## Installation

```bash
GO111MODULE=on go install github.com/4rji/binarios-go/sshc@latest
```

This installs the `sshc` binary into `$GOPATH/bin` (or `$HOME/go/bin` by default).

## Usage

```bash
sshc <command> [args...]
```

- `sshc uptime`
- `sshc whoami`
- `sshc sudo systemctl restart nginx`

`sshc` reads each `Host` entry from your SSH config file and skips wildcards such as `*` or `?`.

The tool attempts up to 10 simultaneous SSH sessions, using non-interactive mode to avoid password prompts. Hosts that cannot be reached or return an error are reported as offline.

## Requirements

- Go 1.21 or newer to build/install.
- A valid `~/.ssh/config` file with one or more `Host` entries.
- Passwordless SSH access (via keys or agent) to the target hosts.

## Notes

- Unknown host keys are accepted automatically and are **not** written to `known_hosts`.
- Connection attempts time out after 5 seconds.
- Output from each host is trimmed and printed below the colored hostname label.
