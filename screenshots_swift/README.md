# MacOCR

Local screen OCR for macOS 14+ on Apple Silicon. Uses Apple's Vision framework to recognize English and Spanish and copy the result to your clipboard. No network access or third-party dependencies.

## Build the macOS app

Requirements: macOS 14+, Apple Silicon, and Swift 6+ from Xcode or Xcode Command Line Tools. No extra packages or Xcode project are needed.

From the project directory, run:

```sh
sh Scripts/build-app.sh
open dist/MacOCR.app
```

The script builds a release ARM64 executable, creates **`dist/MacOCR.app`**, and signs it locally with an ad-hoc signature. This is a local build, not a notarized distribution.

To launch it again, double-click **`dist/MacOCR.app`** or run `open dist/MacOCR.app` from this directory. You can also copy the app to your Applications folder. Quit the running app before rebuilding or replacing it.

## Using the app

Its OCR icon appears in the menu bar near the clock, with no window or Dock icon. Left-click or right-click it to open the menu:

- **Capture Text:** select a region, or press **Command + Shift + 2**.
- **History:** click a previous capture to copy its full text again.
- **Normal Formatting:** readable paragraphs with unnecessary spaces removed.
- **Code Formatting:** approximate indentation and aligned columns for code, terminals, and tables.
- **Quit MacOCR:** stop the app and release the shortcut.

The icon changes while selecting or recognizing text. History keeps the latest 20 distinct captures in memory (up to 1 MiB total); quitting clears it. No recognized text is saved to disk. History previews are shortened, but copying preserves the complete text.

Press **Command + Shift + 2**, drag around the text, and wait until the menu says **Copied … characters**. Paste with **Command + V**. **Escape** cancels selection. Keep MacOCR running to capture again.

The first OCR can take noticeably longer while macOS prepares its local Vision/Neural Engine models. Leave it running until it finishes; later captures should be faster. While processing, new captures are disabled. An hourglass means recognition is still in progress; the menu shows the current status or error.

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

## Tests and troubleshooting

Quit MacOCR first so the hotkey test can register the shortcut, then run:

```sh
swift test --arch arm64
```

- **No icon after opening:** check for a startup error or another MacOCR instance using the shortcut.
- **Capture fails:** check Screen Recording permission and restart MacOCR after granting it.
- **Recognition takes a long time:** allow the first local model initialization to finish. Open the icon's menu to check its status.
- **Shortcut conflict:** quit the other instance or check macOS Keyboard Shortcuts. You can also select **Capture Text** from the menu once the service is running.

For live diagnostic messages, quit the app and launch its executable from this directory:

```sh
./dist/MacOCR.app/Contents/MacOS/macocr --menu-bar
```

Leave that terminal open; use the menu's **Quit MacOCR** or **Control + C** to stop it. Diagnostic messages report progress and character counts, not recognized text.
