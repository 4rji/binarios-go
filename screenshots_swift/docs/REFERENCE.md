# MacOCR reference

A lightweight native macOS command-line OCR service. Press **Command + Shift + 2**, drag a screen rectangle, and paste the recognized text. MacOCR has no window, menu-bar item, or Dock icon. The system's native screenshot selector appears only during capture.

Swift, Vision, AppKit, Carbon, and `/usr/sbin/screencapture` are the only runtime components. No packages, external OCR engines, network calls, cloud processing, or telemetry. Requires macOS 14+ and Apple Silicon. Build with Swift 6+ (Xcode or the corresponding Command Line Tools).

## Build and run

Run from the project root:

```sh
swift build -c release --arch arm64
swift test --arch arm64
```

Locate the resulting executable (the exact directory varies by SwiftPM build backend):

```sh
swift build -c release --arch arm64 --show-bin-path
```

Use the `macocr` executable in that directory:

```sh
macocr                       # continuous background service; normal mode
macocr --once                # capture, copy, exit
macocr --mode normal
macocr --mode code
macocr --once --mode code
macocr --help
```

The examples assume the executable's directory is on your PATH. You can instead invoke it by its full path. For unattended use, copy it to a stable per-user location such as `$HOME/.local/bin/macocr`. No Xcode project, app bundle, administrator rights, or Python installation is needed.

To repeat captures with the keyboard shortcut, run `macocr` **without `--once`** and leave that process running. `--once` deliberately exits after one operation. During each operation the terminal reports selection, local recognition, clipboard completion, and readiness for the next shortcut. The first Vision request can take longer while macOS initializes its local models; wait for `Copied … characters to the clipboard` before pasting. Hotkey presses while capture/OCR is active report that it is still in progress.

## Capture and permissions

Run `macocr --once` from your terminal first. macOS may ask for Screen Recording permission. Enable the requesting terminal or `macocr` under **System Settings → Privacy & Security → Screen Recording** (called **Screen & System Audio Recording** on some releases). Quit and reopen the requesting process after changing permission. macOS controls which responsible process appears in this list; running under a LaunchAgent can have a different permission identity from your terminal.

Drag a rectangle, then release the mouse. Escape cancels. Do not hold Control while selecting: the native utility interprets it as “capture to clipboard”; MacOCR restores the prior clipboard image side effect and treats a selection without a saved file as canceled. It does not revert ordinary text copied by other apps while the selector is open. Avoid copying a different image while a selection is active, since the system clipboard does not expose which process supplied it.

Screen Recording permission is checked at capture time, including after revocation. Permissions cannot be granted silently. Carbon hotkeys do not require an Accessibility event tap. Hotkeys and the native selector require a logged-in interactive GUI session; a LaunchDaemon or ordinary SSH-only session is unsuitable. Secure Input or system-reserved shortcuts can prevent delivery even after successful registration. If the shortcut is unavailable, check Keyboard Shortcuts and other apps, or use `--once`. Registration conflicts produce an OSStatus diagnostic.

## Formatting

- **Normal:** accurate recognition with language correction and automatic language detection, prioritizing English and Spanish. Groups fragments into lines, sorts top-to-bottom and left-to-right, removes excess whitespace, and inserts paragraph breaks for large vertical gaps.
- **Code:** accurate recognition with language correction disabled to reduce unwanted changes to identifiers and commands. Uses word bounding boxes and a median estimated character width to reconstruct indentation, spacing between tokens, aligned columns, and visible blank lines. Uses spaces rather than inventing tabs. Keeps known internal whitespace when geometry is unavailable.

OCR cannot recover original whitespace perfectly. Accurate Vision recognition supplies word-level boxes, not precise character boxes. Spacing is most reliable for monospace screenshots with several lines and a common left margin. Indentation is relative to the leftmost recognized text: absolute indentation cannot be inferred from an arbitrary crop. A single line has no external indentation reference. Cropped characters, tiny text, ligatures, unusual punctuation, proportional fonts, wide Unicode glyphs, and multiple independent page columns can reduce recognition or reading-order accuracy. Verify copied source code before running it. No resizing, filtering, or other unnecessary image processing is performed.

Clipboard writes use Unicode strings and occur only after nonempty OCR succeeds. Canceled selections, capture errors, Vision errors, and empty results do not initiate a text clipboard write. Logs contain status and character counts, never recognized text. Captures use a unique private temporary directory (mode 0700) and are removed after success, failure, cancellation, or handled termination. SIGKILL, power loss, or an OS crash cannot run cleanup; any surviving `MacOCR-<UUID>` directories in the user's macOS temporary directory can be removed after confirming no capture is running.

## Background operation

Keep `macocr` running in a terminal, or launch it as a per-user LaunchAgent. It sleeps in `NSApplication`'s native event loop when idle; there is no timer, polling, or busy loop. A single operation owns capture and OCR; hotkey presses during that operation are ignored. Vision runs on a worker queue, leaving event and signal handling responsive. SIGINT and SIGTERM unregister the hotkey, stop the selector, and remove temporary captures before exiting.

For a LaunchAgent, edit the example at `Support/com.macocr.agent.plist`: replace **every** `YOUR_USER` path with an absolute path for your account and point `ProgramArguments` to your stable executable. LaunchAgent plists do not expand `~`, `$HOME`, or shell expressions. The example is limited to an Aqua login session, starts at login, and restarts only after abnormal exits.

After editing, install it yourself:

```sh
mkdir -p "$HOME/Library/LaunchAgents"
cp Support/com.macocr.agent.plist "$HOME/Library/LaunchAgents/com.macocr.agent.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.macocr.agent.plist"
```

Stop and unload it:

```sh
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.macocr.agent.plist"
```

Use a user-writable log location. If the agent reports missing permission, grant permission for the responsible process macOS identifies and restart the agent with `launchctl kickstart -k "gui/$(id -u)/com.macocr.agent"`. Stop any terminal instance first to avoid a duplicate shortcut registration. The project does not install or enable an agent automatically.

Exit codes: `0` for successful one-shot/help; `2` for arguments, setup, or one-shot processing failure; `130` for one-shot cancellation or SIGINT; `143` for SIGTERM. In service mode, capture/OCR errors are logged and the service remains available for the next shortcut.

## Verification

`swift test --arch arm64` tests parsing, paragraph/line ordering, Unicode, relative code indentation, reconstructed word spacing, repeated tokens, blank lines, terminal/table columns, invalid geometry, isolated clipboard success/failure and native capture side effects, capture cancellation/cleanup with harmless process substitutes, permission denial, launch failures, native Carbon event dispatch/conflicts/unregistration, unreadable/empty images, and actual Vision recognition of offscreen English, Spanish, multiline, code, and table fixtures. Regression tests verify that a surviving capture helper holding stderr open cannot block completion, and that repeated successful captures reach Vision and the clipboard in both modes. The fixture tests require local Vision model support but do not request screen permission or alter the general clipboard. The hotkey test skips when an external application already owns the shortcut; stop MacOCR before running the suite.

Interactive verification requires the logged-in user's permission and input:

1. Run `macocr --once`; select English and Spanish text. Paste into an editor and compare accents, paragraphs, and line breaks.
2. Run `macocr --once --mode code`; select a monospace terminal table and indented source code. Compare spacing and column alignment.
3. Copy a sentinel string, run `--once`, and press Escape. Verify exit 130, unchanged clipboard, and no remaining capture directory. Repeat with a blank region (exit 2).
4. Run service mode, activate the hotkey repeatedly while selecting/recognizing, and confirm one selector at a time. Cancel and retry; the next shortcut should work.
5. Revoke Screen Recording permission and trigger capture; confirm a diagnostic and unchanged clipboard. Regrant and relaunch.
6. Run two service instances; the second should report a shortcut conflict. Stop the first and verify registration succeeds again.
7. Leave the service idle and inspect CPU usage in Activity Monitor. Send SIGINT or SIGTERM during idle, capture, and OCR; confirm termination and cleanup.
8. Install the edited LaunchAgent, verify the shortcut works in the Aqua session, inspect its logs, and unload it when finished.

## Components

`Main.swift` owns lifecycle and operation scheduling; `Configuration.swift` parses arguments; `HotkeyManager.swift` owns Carbon registration; `ScreenCapture.swift` owns the native selector and temporary files; `OCREngine.swift` converts Vision results to pixel coordinates; `TextFormatter.swift` groups and reconstructs layout; `ClipboardManager.swift` handles Unicode clipboard writes and native capture side effects.

Apple references: [Vision language detection](https://developer.apple.com/documentation/vision/vnrecognizetextrequest/automaticallydetectslanguage), [recognition languages](https://developer.apple.com/documentation/vision/vnrecognizetextrequest/recognitionlanguages), and [accurate-mode word bounding boxes](https://developer.apple.com/documentation/vision/vnrecognizedtext/boundingbox(for:)). The installed macOS SDK headers and `man screencapture` document Carbon registration and native selection behavior.
