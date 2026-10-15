import AppKit

/// A point-in-time read of the pasteboard, decoupled from AppKit so the
/// record/skip decision can be unit-tested without a live NSPasteboard.
struct ClipboardSnapshot {
    let changeCount: Int
    let string: String?
    let isConcealed: Bool
    let isTransient: Bool
}

/// Watches the general pasteboard and reports plain-text copies made in any
/// app so they can join the session history. macOS has no clipboard-change
/// event, so this polls `changeCount` on a timer. Text-only and in memory:
/// nothing is written to disk.
@MainActor
final class ClipboardMonitor {
    // Conventions password managers and clipboard utilities use to opt items
    // out of history. Respected so copied secrets are never recorded.
    static let concealedType = NSPasteboard.PasteboardType("org.nspasteboard.ConcealedType")
    static let transientType = NSPasteboard.PasteboardType("org.nspasteboard.TransientType")

    private let pasteboard: NSPasteboard
    private let onCopy: (String) -> Void
    private var lastChangeCount: Int
    private var timer: Timer?
    private(set) var isPaused = false

    init(pasteboard: NSPasteboard = .general, onCopy: @escaping (String) -> Void) {
        self.pasteboard = pasteboard
        self.onCopy = onCopy
        self.lastChangeCount = pasteboard.changeCount
    }

    func start(interval: TimeInterval = 0.5) {
        stop()
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.poll() }
        }
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func setPaused(_ paused: Bool) {
        isPaused = paused
        // Don't record whatever happens to be on the clipboard across a pause.
        syncBaseline()
    }

    /// Mark the current clipboard contents as already seen. Call right after the
    /// app writes the clipboard itself (OCR copy, re-copy from history, capture
    /// side effects) so those writes aren't mistaken for a user copy.
    func syncBaseline() { lastChangeCount = pasteboard.changeCount }

    /// One polling step. Internal (not private) so tests can drive it directly
    /// without spinning a run loop.
    func poll() {
        guard !isPaused else { return }
        let snapshot = ClipboardMonitor.snapshot(of: pasteboard)
        guard snapshot.changeCount != lastChangeCount else { return }
        lastChangeCount = snapshot.changeCount
        if let text = ClipboardMonitor.textToRecord(snapshot) {
            onCopy(text)
        }
    }

    static func snapshot(of pasteboard: NSPasteboard) -> ClipboardSnapshot {
        let types = pasteboard.types ?? []
        return ClipboardSnapshot(
            changeCount: pasteboard.changeCount,
            string: pasteboard.string(forType: .string),
            isConcealed: types.contains(concealedType),
            isTransient: types.contains(transientType)
        )
    }

    /// Pure decision: the text to record, or nil to skip. Skips concealed and
    /// transient items and anything without non-whitespace text.
    static func textToRecord(_ snapshot: ClipboardSnapshot) -> String? {
        guard !snapshot.isConcealed, !snapshot.isTransient,
              let text = snapshot.string,
              !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        return text
    }
}
