# Repository Guidelines

## Project Structure & Module Organization
This directory is the `backd` Go module (`go.mod` at the module root). Main entry points live at the module root and under `cmd/`:
- `main.go` — base `backd` CLI
- `cmd/backd-daemon/` — daemon variant
- `cmd/backd-udp/` — TCP/UDP monitor
- `cmd/backde-legacy/` — interactive legacy UI
- `backde/` — supporting assets for the legacy variant

Keep changes scoped to the variant you are touching. This repository is a multi-binary layout, so run Go commands from `backd/`, not from the monorepo root.

## Build, Test, and Development Commands
Run commands from the module root:
```bash
cd backd
go build ./...                 # build all binaries in this module
go test ./...                  # run all Go tests in this module
go run .                       # run the base CLI
go run ./cmd/backde-legacy -- -2
```
Use `go run ./cmd/<name>` for a specific binary. The legacy UI expects flags after `--` when using `go run`.

## Coding Style & Naming Conventions
Use standard Go formatting: tabs for indentation, `gofmt` before review, and idiomatic package naming (short, lowercase, no underscores). Keep command-specific logic inside its own `cmd/<binary>/` directory. Prefer explicit flag names and small helper functions over large monolithic `main.go` blocks.

## Testing Guidelines
There are currently no `*_test.go` files in this module, so add tests with any non-trivial change. Use Go’s built-in `testing` package and name tests as `TestXxx`. Run targeted tests with:
```bash
go test ./... -run TestName -v
```
When fixing a bug, add or update a regression test when practical.

## Commit & Pull Request Guidelines
Recent history includes inconsistent messages such as `12`, so standardize on Conventional Commits going forward: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`. Do not add `Co-Authored-By` or AI attribution. PRs should include a short summary, affected binary paths (for example `cmd/backde-legacy/main.go`), manual test notes, and linked issues when applicable.

## Security & Runtime Notes
This tool inspects live network connections and may require `sudo` to see all processes. Do not commit local log files such as `backde.log`, and avoid hardcoding host-specific paths or credentials.
