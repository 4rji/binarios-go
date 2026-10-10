# MacOCR

Local screen OCR for macOS 14+ on Apple Silicon. Uses Apple's Vision framework to recognize English and Spanish and copy the result to your clipboard. No network access or third-party dependencies.

## Build and start

Requires Swift 6+ and Xcode Command Line Tools. Run from this project directory:

```sh
swift build -c release --arch arm64
MACOCR_BIN_DIR="$(swift build -c release --arch arm64 --show-bin-path)"
"$MACOCR_BIN_DIR/macocr"
```

Keep the process running to capture repeatedly:

1. Press **Command + Shift + 2**.
2. Drag a rectangle around the text. **Escape** cancels.
3. Wait for `Copied … characters to the clipboard`.
4. Paste with **Command + V**.

The first recognition may take longer while macOS initializes Vision. Stop MacOCR with **Control + C** in its terminal.

## Options

```sh
"$MACOCR_BIN_DIR/macocr" --mode code   # Estimate indentation and column spacing
"$MACOCR_BIN_DIR/macocr" --once        # Capture once, copy, and exit
"$MACOCR_BIN_DIR/macocr" --help
```

Normal formatting is the default. Code spacing is approximate. Use `--once --mode code` to combine both options. Start only one service instance.

## Permissions

Allow the requesting terminal or MacOCR in **System Settings → Privacy & Security → Screen Recording** (or **Screen & System Audio Recording**), then restart it. Select without holding Control. Empty or canceled captures leave your clipboard unchanged.

The current version runs in the terminal and has no Dock or menu-bar icon. [Additional details and LaunchAgent setup](docs/REFERENCE.md).
