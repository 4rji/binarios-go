import AppKit
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
    private var hotkey: HotkeyManager?
    private var signals: [DispatchSourceSignal] = []
    private var busy = false
    private var shuttingDown = false
    private let showsMenuBar: Bool
    private var menuBar: MenuBarController?
    private var history = OCRHistory()
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
            let manager = HotkeyManager { [weak self] in self?.trigger() }
            try manager.register()
            hotkey = manager
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
                    }
                )
                refreshMenu()
            }
            log("Ready (\(configuration.mode.rawValue) mode). Press Command + Shift + 2; Control-C to stop.")
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
                let savedToHistory = showsMenuBar && history.add(text, mode: mode)
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

    private func shutdown(code: Int32) {
        guard !shuttingDown else { return }
        shuttingDown = true
        hotkey?.unregister()
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
        menuBar?.update(status: status, busy: busy, mode: mode, history: history.entries, message: statusMessage)
    }

    private func copyHistory(_ id: UUID) {
        guard !busy, let entry = history.entry(id: id) else { return }
        do {
            try ClipboardManager().copy(entry.text)
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
