# GRUB State Guard (Go)

Small utility that snapshots GRUB-related files, detects changes, and can restore the baseline. Intended for temporary use in lab/competition environments.

## What it does
- Tracks key GRUB configuration paths (config dirs + boot/EFI files).
- Stores hashes, metadata, and a compressed state archive under `/var/lib/os-system`.
- Detects changes and can restore the saved state.
- Optionally skips `/boot` paths for lighter scans.
- Uses `chattr +i` on its own state files (if supported) to make them harder to delete.

## Commands
`init`, `check`, `restore`, `status`, `lock`, `unlock`

## Flags (short only)
- `-c` run `check` (alias for the `check` subcommand)
- `-n` skip GRUB rebuild after restore
- `-s` scan-only (detect changes, do not restore)
- `-b` exclude `/boot` and EFI paths

## Examples
```bash
go build -o guard sysstate.go
./guard init
./guard check -s
./guard check -n
./guard init -b
```

## Notes
- Requires root; the binary re-execs itself via `sudo` when needed.
- `chattr` is best-effort and depends on filesystem support.
