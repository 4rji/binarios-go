import AppKit

enum OCRStatus {
    case ready, selecting, recognizing, error

    var title: String {
        switch self {
        case .ready: return "MacOCR — Ready"
        case .selecting: return "MacOCR — Selecting region"
        case .recognizing: return "MacOCR — Recognizing text…"
        case .error: return "MacOCR — Attention needed"
        }
    }

    var symbol: String {
        switch self {
        case .ready: return "text.viewfinder"
        case .selecting: return "viewfinder"
        case .recognizing: return "hourglass"
        case .error: return "exclamationmark.triangle"
        }
    }
}

@MainActor
final class MenuBarController: NSObject {
    let menu = NSMenu()
    private let statusItem: NSStatusItem
    private let onCapture: () -> Void
    private let onQuit: () -> Void
    private let onMode: (OCRMode) -> Void
    private let onCopy: (UUID) -> Void
    private let onClear: () -> Void
    private let timeFormatter: DateFormatter

    init(onCapture: @escaping () -> Void, onQuit: @escaping () -> Void,
         onMode: @escaping (OCRMode) -> Void, onCopy: @escaping (UUID) -> Void,
         onClear: @escaping () -> Void) {
        self.onCapture = onCapture
        self.onQuit = onQuit
        self.onMode = onMode
        self.onCopy = onCopy
        self.onClear = onClear
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        timeFormatter = DateFormatter()
        timeFormatter.dateFormat = "HH:mm"
        super.init()
        menu.autoenablesItems = false
        statusItem.menu = menu
        statusItem.button?.sendAction(on: [.leftMouseUp, .rightMouseUp])
        statusItem.button?.setAccessibilityLabel("MacOCR")
    }

    func update(status: OCRStatus, busy: Bool, mode: OCRMode,
                history: [OCRHistoryEntry], message: String?) {
        let image = NSImage(systemSymbolName: status.symbol, accessibilityDescription: status.title)
        image?.isTemplate = true
        statusItem.button?.image = image
        // A readable fallback keeps the service visible if an SF Symbol is unavailable.
        statusItem.button?.title = image == nil ? "OCR" : ""
        statusItem.button?.toolTip = [status.title, message].compactMap { $0 }.joined(separator: "\n")
        menu.removeAllItems()
        menu.addItem(label(status.title))
        if let message, !message.isEmpty {
            menu.addItem(label(String(message.prefix(130))))
        }
        menu.addItem(.separator())
        let capture = item("Capture Text", action: #selector(captureText))
        capture.keyEquivalent = "2"
        capture.keyEquivalentModifierMask = [.command, .shift]
        capture.isEnabled = !busy
        menu.addItem(capture)

        let historyItem = NSMenuItem(title: "History (\(history.count))", action: nil, keyEquivalent: "")
        let historyMenu = NSMenu()
        historyMenu.autoenablesItems = false
        if history.isEmpty { historyMenu.addItem(label("No captures yet")) }
        for entry in history {
            let title = "\(timeFormatter.string(from: entry.capturedAt)) · \(entry.preview)"
            let row = item(title, action: #selector(copyHistory(_:)))
            row.representedObject = entry.id.uuidString
            row.toolTip = "Click to copy full text (\(entry.mode.rawValue) mode)\n" + String(entry.text.prefix(1_000))
            row.isEnabled = !busy
            historyMenu.addItem(row)
        }
        historyMenu.addItem(.separator())
        let clear = item("Clear History", action: #selector(clearHistory))
        clear.isEnabled = !history.isEmpty && !busy
        historyMenu.addItem(clear)
        historyItem.submenu = historyMenu
        menu.addItem(historyItem)

        menu.addItem(.separator())
        for value in [OCRMode.normal, .code] {
            let row = item(value == .normal ? "Normal Formatting" : "Code Formatting", action: #selector(changeMode(_:)))
            row.representedObject = value.rawValue
            row.state = mode == value ? .on : .off
            row.isEnabled = !busy
            menu.addItem(row)
        }
        menu.addItem(.separator())
        let quit = item("Quit MacOCR", action: #selector(quitApp))
        quit.keyEquivalent = "q"
        menu.addItem(quit)
    }

    func remove() { NSStatusBar.system.removeStatusItem(statusItem) }

    private func item(_ title: String, action: Selector) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: "")
        item.target = self
        return item
    }

    private func label(_ text: String) -> NSMenuItem {
        let item = NSMenuItem(title: text, action: nil, keyEquivalent: "")
        item.isEnabled = false
        return item
    }

    @objc private func captureText() {
        // Let menu tracking finish before presenting the native region selector.
        DispatchQueue.main.async { [weak self] in self?.onCapture() }
    }
    @objc private func quitApp() { onQuit() }
    @objc private func clearHistory() { onClear() }
    @objc private func copyHistory(_ sender: NSMenuItem) {
        guard let value = sender.representedObject as? String, let id = UUID(uuidString: value) else { return }
        onCopy(id)
    }
    @objc private func changeMode(_ sender: NSMenuItem) {
        guard let value = sender.representedObject as? String, let mode = OCRMode(rawValue: value) else { return }
        onMode(mode)
    }
}
