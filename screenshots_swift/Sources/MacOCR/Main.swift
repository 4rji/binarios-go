import AppKit
import Carbon
import Darwin

@main
enum MacOCR {
    @MainActor
    static func main() {
        var showsMenuBar = false
        do {
            let configuration = try Configuration(arguments: Array(CommandLine.arguments.dropFirst()))
            if configuration.help { print(Configuration.usage); return }
            showsMenuBar = !configuration.once && (configuration.menuBar || Bundle.main.bundleIdentifier == "com.macocr.app")
            let application = NSApplication.shared
            application.setActivationPolicy(showsMenuBar ? .accessory : .prohibited)
            let service = OCRService(configuration: configuration, showsMenuBar: showsMenuBar)
            try service.start()
            withExtendedLifetime(service) { application.run() }
        } catch {
            FileHandle.standardError.write(Data("macocr: \(error.localizedDescription)\n".utf8))
            if showsMenuBar {
                let alert = NSAlert()
                alert.messageText = "MacOCR could not start"
                alert.informativeText = error.localizedDescription
                alert.addButton(withTitle: "Quit")
                NSApplication.shared.activate(ignoringOtherApps: true)
                alert.runModal()
            }
            Darwin.exit(2)
        }
    }
}

@MainActor
private final class OCRService {
    private let configuration: Configuration
    private let capture = ScreenCapture()
    private var captureHotkey: HotkeyManager?
    private var typeHotkey: HotkeyManager?
    private var tableTypeHotkey: HotkeyManager?
    private var signals: [DispatchSourceSignal] = []
    private var busy = false
    private var shuttingDown = false
    private let showsMenuBar: Bool
    private var menuBar: MenuBarController?
    private var history = OCRHistory()
    private var clipboardMonitor: ClipboardMonitor?
    private var mode: OCRMode
    private var status: OCRStatus = .ready
    private var statusMessage: String?

    init(configuration: Configuration, showsMenuBar: Bool) {
        self.configuration = configuration
        self.showsMenuBar = showsMenuBar
        self.mode = configuration.mode
    }

    func start() throws {
        for number in [SIGINT, SIGTERM] {
            signal(number, SIG_IGN)
            let source = DispatchSource.makeSignalSource(signal: number, queue: .main)
            source.setEventHandler { [weak self] in
                MainActor.assumeIsolated { self?.shutdown(code: 128 + number) }
            }
            source.resume()
            signals.append(source)
        }
        if configuration.once {
            DispatchQueue.main.async { [weak self] in self?.trigger() }
        } else {
            let manager = HotkeyManager(keyCode: UInt32(kVK_ANSI_2), modifiers: UInt32(cmdKey | shiftKey),
                                         id: 1, description: "Command + Shift + 2") { [weak self] in self?.trigger() }
            try manager.register()
            captureHotkey = manager
            let typist = HotkeyManager(keyCode: UInt32(kVK_ANSI_2), modifiers: UInt32(optionKey | shiftKey),
                                        id: 2, description: "Option + Shift + 2") { [weak self] in self?.triggerTypeClipboard() }
            try typist.register()
            typeHotkey = typist
            let tableTypist = HotkeyManager(keyCode: UInt32(kVK_ANSI_1), modifiers: UInt32(optionKey | shiftKey),
                                             id: 3, description: "Option + Shift + 1") { [weak self] in self?.triggerTypeTable() }
            try tableTypist.register()
            tableTypeHotkey = tableTypist
            if showsMenuBar {
                menuBar = MenuBarController(
                    onCapture: { [weak self] in self?.trigger() },
                    onQuit: { [weak self] in self?.shutdown(code: 0) },
                    onMode: { [weak self] value in
                        guard let self, !self.busy else { return }
                        self.mode = value
                        self.refreshMenu()
                    },
                    onCopy: { [weak self] id in self?.copyHistory(id) },
                    onClear: { [weak self] in
                        guard let self, !self.busy else { return }
                        self.history.clear()
                        self.statusMessage = "History cleared."
                        self.refreshMenu()
                    },
                    onTogglePause: { [weak self] in
                        guard let self, let monitor = self.clipboardMonitor else { return }
                        monitor.setPaused(!monitor.isPaused)
                        self.statusMessage = monitor.isPaused ? "Clipboard capture paused." : "Clipboard capture on."
                        self.refreshMenu()
                    }
                )
                let monitor = ClipboardMonitor { [weak self] text in
                    guard let self, self.history.add(text, source: .clipboard) else { return }
                    self.refreshMenu()
                }
                monitor.start()
                clipboardMonitor = monitor
                refreshMenu()
            }
            log("Ready (\(configuration.mode.rawValue) mode). Press Command + Shift + 2 to capture, Option + Shift + 2 to type the clipboard, Option + Shift + 1 to type a markdown table as TSV; Control-C to stop.")
        }
    }

    private func trigger() {
        guard !shuttingDown else { return }
        guard !busy else {
            log("Capture or OCR is still in progress; please wait for the clipboard confirmation.")
            return
        }
        busy = true
        status = .selecting
        statusMessage = "Drag a rectangle. Escape cancels."
        refreshMenu()
        let operationMode = mode
        Task {
            defer {
                capture.cleanup()
                // Control-drag can leave an image on the clipboard that the guard
                // restores; treat the post-capture clipboard as already seen.
                clipboardMonitor?.syncBaseline()
                busy = false
                refreshMenu()
                if !configuration.once && !shuttingDown {
                    log("Ready for the next capture (Command + Shift + 2).")
                }
            }
            do {
                log("Select a region; Escape cancels.")
                let imageURL = try await capture.capture()
                status = .recognizing
                statusMessage = "Processing locally; the first OCR may take longer."
                refreshMenu()
                log("Capture saved. Recognizing text locally…")
                let mode = operationMode
                let text = try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<String, Error>) in
                    DispatchQueue.global(qos: .userInitiated).async {
                        let result: Result<String, Error> = autoreleasepool {
                            Result {
                                let fragments = try OCREngine().recognize(imageURL: imageURL, mode: mode)
                                return TextFormatter().format(fragments, mode: mode)
                            }
                        }
                        continuation.resume(with: result)
                    }
                }
                guard !shuttingDown else { return }
                try ClipboardManager().copy(text)
                clipboardMonitor?.syncBaseline()
                let savedToHistory = showsMenuBar && history.add(text, source: .ocr(mode))
                status = .ready
                statusMessage = "Copied \(text.count) characters." + (showsMenuBar && !savedToHistory ? " Too large for history." : "")
                log("Copied \(text.count) characters to the clipboard.")
                capture.cleanup()
                if configuration.once { shutdown(code: 0) }
            } catch MacOCRError.cancelled {
                status = .ready
                statusMessage = "Capture canceled; clipboard unchanged."
                log("Capture canceled; clipboard unchanged.")
                if configuration.once { shutdown(code: 130) }
            } catch {
                status = .error
                statusMessage = error.localizedDescription
                log(error.localizedDescription)
                if configuration.once { shutdown(code: 2) }
            }
        }
    }

    private func triggerTypeClipboard() {
        guard !shuttingDown else { return }
        guard !busy else {
            log("Capture or typing is still in progress; please wait.")
            return
        }
        guard let text = NSPasteboard.general.string(forType: .string), !text.isEmpty else {
            log("Clipboard is empty or does not contain text.")
            return
        }
        typeText(text)
    }

    private func triggerTypeTable() {
        guard !shuttingDown else { return }
        guard !busy else {
            log("Capture or typing is still in progress; please wait.")
            return
        }
        guard let clipboardText = NSPasteboard.general.string(forType: .string), !clipboardText.isEmpty else {
            log("Clipboard is empty or does not contain text.")
            return
        }
        guard let text = MarkdownTableConverter.tsv(from: clipboardText) else {
            log("Clipboard does not contain a markdown table (expected '|' columns and a '---' separator row).")
            return
        }
        typeText(text)
    }

    private func typeText(_ text: String) {
        let needsConfirmation = text.count > KeyboardTyper.confirmationThreshold
        if needsConfirmation {
            let previousApp = NSWorkspace.shared.frontmostApplication
            let alert = NSAlert()
            alert.messageText = "Type \(text.count) characters?"
            alert.informativeText = "This simulates keyboard input into the active app, one keystroke at a time, which takes a few seconds for long text. After you confirm, typing starts in the previous app after a one-second delay."
            alert.addButton(withTitle: "Type")
            alert.addButton(withTitle: "Cancel")
            NSApplication.shared.activate(ignoringOtherApps: true)
            guard alert.runModal() == .alertFirstButtonReturn else {
                log("Typing canceled; clipboard unchanged.")
                return
            }
            previousApp?.activate(options: [])
        }
        busy = true
        status = .typing
        statusMessage = "Typing \(text.count) characters…"
        refreshMenu()
        Task {
            defer {
                busy = false
                refreshMenu()
                if !shuttingDown { log("Ready (Command + Shift + 2 to capture, Option + Shift + 2 to type, Option + Shift + 1 for a markdown table).") }
            }
            do {
                if needsConfirmation {
                    try await Task.sleep(for: .seconds(1))
                }
                guard !shuttingDown else { return }
                try await KeyboardTyper().type(text)
                guard !shuttingDown else { return }
                status = .ready
                statusMessage = "Typed \(text.count) characters."
                log("Typed \(text.count) characters from the clipboard.")
            } catch {
                status = .error
                statusMessage = error.localizedDescription
                log(error.localizedDescription)
            }
        }
    }

    private func shutdown(code: Int32) {
        guard !shuttingDown else { return }
        shuttingDown = true
        captureHotkey?.unregister()
        typeHotkey?.unregister()
        tableTypeHotkey?.unregister()
        clipboardMonitor?.stop()
        menuBar?.remove()
        history.clear()
        capture.stop()
        signals.forEach { $0.cancel() }
        Darwin.exit(code)
    }

    private func log(_ message: String) {
        FileHandle.standardError.write(Data("macocr: \(message)\n".utf8))
    }

    private func refreshMenu() {
        menuBar?.update(status: status, busy: busy, mode: mode, history: history.entries,
                        clipboardPaused: clipboardMonitor?.isPaused ?? false, message: statusMessage)
    }

    private func copyHistory(_ id: UUID) {
        guard !busy, let entry = history.entry(id: id) else { return }
        do {
            try ClipboardManager().copy(entry.text)
            clipboardMonitor?.syncBaseline()
            status = .ready
            statusMessage = "Copied \(entry.text.count) characters from history."
            log("Copied \(entry.text.count) characters from history.")
        } catch {
            status = .error
            statusMessage = error.localizedDescription
            log(error.localizedDescription)
        }
        refreshMenu()
    }
}
