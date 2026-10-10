import AppKit
import Darwin

@main
enum MacOCR {
    @MainActor
    static func main() {
        do {
            let configuration = try Configuration(arguments: Array(CommandLine.arguments.dropFirst()))
            if configuration.help { print(Configuration.usage); return }
            let application = NSApplication.shared
            application.setActivationPolicy(.prohibited)
            let service = OCRService(configuration: configuration)
            try service.start()
            withExtendedLifetime(service) { application.run() }
        } catch {
            FileHandle.standardError.write(Data("macocr: \(error.localizedDescription)\n".utf8))
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

    init(configuration: Configuration) { self.configuration = configuration }

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
        Task {
            defer {
                capture.cleanup()
                busy = false
                if !configuration.once && !shuttingDown {
                    log("Ready for the next capture (Command + Shift + 2).")
                }
            }
            do {
                log("Select a region; Escape cancels.")
                let imageURL = try await capture.capture()
                log("Capture saved. Recognizing text locally…")
                let mode = configuration.mode
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
                log("Copied \(text.count) characters to the clipboard.")
                capture.cleanup()
                if configuration.once { shutdown(code: 0) }
            } catch MacOCRError.cancelled {
                log("Capture canceled; clipboard unchanged.")
                if configuration.once { shutdown(code: 130) }
            } catch {
                log(error.localizedDescription)
                if configuration.once { shutdown(code: 2) }
            }
        }
    }

    private func shutdown(code: Int32) {
        guard !shuttingDown else { return }
        shuttingDown = true
        hotkey?.unregister()
        capture.stop()
        signals.forEach { $0.cancel() }
        Darwin.exit(code)
    }

    private func log(_ message: String) {
        FileHandle.standardError.write(Data("macocr: \(message)\n".utf8))
    }
}
