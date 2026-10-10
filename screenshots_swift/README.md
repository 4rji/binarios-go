# MacOCR

Local screen OCR for macOS 14+ on Apple Silicon. Uses Apple's Vision framework to recognize English and Spanish and copy the result to your clipboard. No network access or third-party dependencies.

## Menu-bar app

Build the app with Swift 6+ and Xcode Command Line Tools:

```sh
sh Scripts/build-app.sh
```

Double-click **`dist/MacOCR.app`**. Its OCR icon appears beside the clock, with no window or Dock icon. Left-click or right-click it to open the menu:

- **Capture Text:** select a region, or press **Command + Shift + 2**.
- **History:** click a previous capture to copy its full text again.
- **Normal / Code Formatting:** choose the formatting mode.
- **Quit MacOCR:** stop the app and release the shortcut.

The icon changes while selecting or recognizing text. History keeps the latest 20 distinct captures in memory (up to 1 MiB total); quitting clears it. No recognized text is saved to disk. History previews are shortened, but copying preserves the complete text.

## Terminal version

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
"$MACOCR_BIN_DIR/macocr" --menu-bar    # Show the icon and history from the CLI
"$MACOCR_BIN_DIR/macocr" --once        # Capture once, copy, and exit
"$MACOCR_BIN_DIR/macocr" --help
```

Normal formatting is the default. Code spacing is approximate. Use `--once --mode code` to combine both options. Start only one service instance.

## Permissions

Allow the requesting terminal or MacOCR in **System Settings → Privacy & Security → Screen Recording** (or **Screen & System Audio Recording**), then restart it. Select without holding Control. Empty or canceled captures leave your clipboard unchanged.

The app bundle may need its own Screen Recording permission even if the terminal was already authorized. [Additional details and LaunchAgent setup](docs/REFERENCE.md).
