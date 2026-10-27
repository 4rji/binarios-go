# systemd-persist-audit

`systemd-persist-audit` is a read-only systemd persistence triage tool for Linux. It turns the
original `systemd-persist-audit` shell workflow into a single Go binary with
risk scoring, compact summaries, JSON output, color, and interactive browsing.

The scanner checks services, timers, drop-ins, local units, permissions,
ownership, symlinks, restart behavior, risky execution paths, shell commands,
network transfer tools, and encoded or inline code. When it runs against the
live root filesystem, it also asks `systemctl` which services and timers are
enabled and active.

## Build

Go 1.22 or newer is required only to build the binary:

```sh
mkdir -p bin
go build -trimpath -ldflags="-s -w" -o bin/systemd-persist-audit .
sudo install -m 0755 bin/systemd-persist-audit /usr/local/bin/systemd-persist-audit
```

It can also be installed directly from GitHub:

```sh
go install github.com/4rji/binarios-go/systemd-persist-audit@latest
```

The resulting binary has no Go package dependencies. Build Linux variants from
macOS or another platform with:

```sh
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/systemd-persist-audit-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/systemd-persist-audit-linux-arm64 .
```

These commands write static `amd64` and `arm64` binaries to `dist/`. Validate
changes with `go test ./...` and `go vet ./...`.

## Use

```sh
sudo systemd-persist-audit
sudo systemd-persist-audit --view fzf
sudo systemd-persist-audit --view menu
systemd-persist-audit --view full --severity suspicious
systemd-persist-audit --view json --no-systemctl > findings.json
```

The default `auto` view behaves according to its environment:

- interactive terminal: searchable, risk-sorted built-in browser;
- redirected output: compact summary suitable for logs and automation.

In the built-in browser, enter a displayed number to open a finding, `/text` to
search, `s` to cycle the minimum severity, and `c` to filter by category. Use
`Enter`/`n` and `p` to move between pages. The controls remain visible at the
bottom of the screen, and `h` opens the complete key reference.

The optional `--view fzf` mode remains available. Inside `fzf`, type to filter
by severity, score, category, unit, or title. Press `Ctrl-S` to toggle match
sorting, `Tab` to select multiple findings, and `Enter` to print selected
details after the browser closes.

Useful filters and automation options:

```sh
# Only high-confidence leads
systemd-persist-audit --view full --severity suspicious

# Browse timer findings
systemd-persist-audit --view fzf --category timers

# CI/response automation: exit 3 on suspicious or critical findings
systemd-persist-audit --view summary --fail-on suspicious

# Inspect a mounted or extracted Linux filesystem without querying systemd
systemd-persist-audit --root /mnt/host --no-systemctl --view full
```

`NO_COLOR=1` and `--no-color` disable ANSI colors. The scan does not change,
enable, disable, start, or stop any unit. A finding is a triage lead, not proof
of compromise; validate it against the host baseline and change history.

## Original script

The existing `systemd-persist-audit` Bash script remains in the repository as
the reference implementation. The Go binary keeps the same command name.
