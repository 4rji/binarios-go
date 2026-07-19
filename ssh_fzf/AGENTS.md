# Repository Guidelines

## Project Structure & Module Organization
This repository is a small single-binary Go CLI. The root contains [`ssh_fzf.go`](/home/nala/gitHub/binarios-go/ssh_fzf/ssh_fzf.go), which handles SSH config parsing, host status checks, `fzf` integration, and preview rendering. [`README.md`](/home/nala/gitHub/binarios-go/ssh_fzf/README.md) documents usage, and [`1.webp`](/home/nala/gitHub/binarios-go/ssh_fzf/1.webp) is the preview image used in documentation. There is no `go.mod`, `Makefile`, or dedicated `testdata/` directory yet, so keep changes focused and local.

## Build, Test, and Development Commands
- `go build -o ssh_fzf ssh_fzf.go`: build the local binary from the single source file.
- `go run ssh_fzf.go`: run the tool locally for manual testing.
- `gofmt -w ssh_fzf.go`: apply standard Go formatting before submitting changes.
- `go install github.com/4rji/binarios-go/ssh_fzf@latest`: intended install path from the README, but it will not work reliably until a `go.mod` file is added.

The CLI also requires `fzf`, `ssh`, and a populated `~/.ssh/config`.

## Coding Style & Naming Conventions
Follow standard Go style and let `gofmt` control indentation and spacing. Keep type names in `PascalCase` (for example, `SSHHost`) and helper functions in `camelCase` (for example, `parseSSHConfig`). Prefer small, single-purpose helpers and the standard library over new dependencies. Preserve the existing defensive handling around SSH target sanitization and `exec.Command("ssh", "--", alias)` when refactoring.

## Testing Guidelines
No automated tests are committed yet. Add table-driven tests in `*_test.go` files beside [`ssh_fzf.go`](/home/nala/gitHub/binarios-go/ssh_fzf/ssh_fzf.go), using names such as `TestParseSSHConfig` and `TestSanitizeSSHTarget`. Cover parsing edge cases, wildcard host filtering, and timeout/config defaults. Until the project is module-enabled, include a manual smoke test with `go run ssh_fzf.go` against a real or temporary SSH config.

## Commit & Pull Request Guidelines
Recent history uses terse subjects such as `Update model.go` and `Delete todo`; do better. Write short, imperative commit messages that describe the actual change, such as `Harden alias parsing for preview mode`. Pull requests should explain the behavior change, list commands run, and note any manual verification. Include a screenshot or terminal capture only when the `fzf` display or preview output changes.

## Configuration & Safety Notes
Document any changes to `SSH_FZF_TIMEOUT_MS` behavior in the README. Do not assume missing `Hostname` or `Port` fields are errors; the tool intentionally falls back to the alias and port `22`.
