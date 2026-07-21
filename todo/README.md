# todo

An interactive terminal UI to discover, preview and run the scripts in
`/opt/4rji/bin/`. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## What it does

- Lists every script found in `/opt/4rji/bin/` (from `README.md`, `descriptions.json`
  and executable files in the directory).
- Fuzzy-ish search over names and descriptions, with a content-search mode that
  greps inside the script files.
- Detail view per script: short + detailed description, plus an image preview
  from `/opt/4rji/img-bin/{name}.{webp,png}` rendered with `chafa`.
- View the source of a script in a pager (`bat` if available, otherwise `less`/`cat`).
- Run the selected script; the command is copied to the clipboard first.

## Build & run

```sh
cd todo
go build -o todo     # build the binary
go run .             # run the interactive UI
go test ./...        # run tests
```

## Keybindings

| Key            | Action                                  |
| -------------- | --------------------------------------- |
| type / `/`     | start searching (name + description)    |
| `↑`/`↓`, `j`/`k` | move selection                        |
| `PgUp`/`PgDn`  | page up / down                          |
| `Enter`        | open detail view (then `Enter` to run)  |
| `Space`        | preview the script source in a pager    |
| `Esc`          | cancel search / leave detail view       |
| `Ctrl+C`       | quit                                    |

## Data sources

```
/opt/4rji/bin/
├── README.md            # script names + short descriptions (one per line)
├── descriptions.json    # optional detailed descriptions
├── .todoignore          # optional list of scripts to hide (one per line, # comments)
└── [executable scripts]

/opt/4rji/img-bin/
└── [name].{webp,png}    # optional preview images
```

## Optional external tools

Resolved at runtime with `exec.LookPath`; missing ones degrade gracefully.

- `chafa` — image previews in the detail view.
- `bat` — syntax-highlighted source preview (falls back to `less`/`cat`).
- `grep` — content-search mode.
- `pbcopy` (macOS) / `xclip` or `xsel` (Linux) — clipboard.
