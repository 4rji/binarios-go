# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Repository shape

This is **not** a single Go module — it's a monorepo of ~30 independent CLI tools (mostly networking, recon, and pentesting utilities), each living in its own top-level directory with its own `go.mod`. There is no `go.work` file at the root, so tooling that walks "the whole repo" (e.g. `go test ./...` from the root) will fail. Always `cd` into the specific tool's directory before running any `go` command.

Each subdirectory is treated as a self-contained project with its own dependencies, module path, and (when applicable) build/test conventions. Several are vendored or lightly forked from upstream projects — preserve their structure rather than refactoring across module boundaries.

## Working in a single tool

The default workflow for any tool `<name>/`:

```sh
cd <name>
go build ./...
go test ./...
go run .            # if main package is at the module root
```

For tools with a `cmd/` layout (e.g. `backd`, `listener`, `trafico`), the entry points are `cmd/<binary>/main.go`:

```sh
cd trafico
go run ./cmd/trafico
go build -o /tmp/trafico ./cmd/trafico
```

Run a single test:

```sh
go test ./<package> -run TestName -v
```

## Tools with custom Makefiles

A few subprojects override the defaults — use the Makefile rather than raw `go` commands:

- **`honeypot/`** (Beelzebub fork)
  - `make test.unit` / `make test.unit.verbose` — unit tests
  - `make test.dependencies.start` then `make test.integration` — integration tests (gated by `INTEGRATION=1` env var; requires docker-compose stack from `integration_test/docker-compose.yml`)
  - `make beelzebub.start` / `make beelzebub.stop` — run the honeypot via docker-compose

- **`tun2socks/`** (xjasonlyu/tun2socks fork)
  - `make tun2socks` — build for the host platform into `build/`
  - `make linux-amd64`, `make darwin-arm64`, `make windows-amd64`, etc. — cross-compile (full matrix is in the Makefile under `UNIX_ARCH_LIST` / `WINDOWS_ARCH_LIST`)
  - `make all` — build the common five-platform set
  - `make releases` — produce zipped artifacts
  - `make lint` — runs `golangci-lint` for darwin/windows/linux/freebsd/openbsd
  - Build embeds version info via `-ldflags` (`internal/version.Version` + `GitCommit`); CGO is disabled by default

## Output binaries

The directory `binarios-go/` (sibling of the source tools, same name as the repo) holds **prebuilt binaries** produced from these sources. It is not Go source — do not run `go` commands inside it. The naming convention adds a trailing letter for OS variants (e.g. `pingm` = macOS build of `pingz/pingm.go`, `locipm` for macOS, `nmapX` for the nmap TUI, etc.).

## Archive workflow

`comprimidos.tar.xz` at the repo root is created/extracted with the commands documented in `pasos_comprimir`:

```sh
tar -cJvf comprimidos.tar.xz comprimidos/   # compress
tar -xvf  comprimidos.tar.xz                 # extract
```

## Conventions when adding or modifying a tool

- Keep each tool self-contained. Don't introduce cross-tool imports — the modules are intentionally independent (different `go` versions across modules: 1.19 in `nmap`, 1.24 in `honeypot`, 1.25 in `tun2socks`, etc.).
- If you bump a dependency, run `go mod tidy` **inside that module's directory only**.
- Forks of upstream projects (`honeypot` = beelzebub, `tun2socks` = xjasonlyu, `sandfly-processdecloak`, `ezuri`) keep their original module paths and licenses — preserve `LICENSE`, `SECURITY.md`, and upstream layout so future merges stay clean.
- `nmap/old-files/` is intentionally retained legacy code (its own `go.mod`); do not delete or "clean up".
