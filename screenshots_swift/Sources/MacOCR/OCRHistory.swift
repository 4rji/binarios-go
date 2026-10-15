import Foundation

/// Where a history entry came from: a screen OCR capture (keeping its
/// formatting mode) or plain text the user copied to the clipboard.
enum HistorySource: Sendable, Equatable {
    case ocr(OCRMode)
    case clipboard

    /// Short human label used in the menu tooltip.
    var label: String {
        switch self {
        case .ocr(let mode): return "\(mode.rawValue) OCR"
        case .clipboard: return "copied"
        }
    }
}

struct OCRHistoryEntry: Identifiable, Sendable {
    let id: UUID
    let text: String
    let source: HistorySource
    let capturedAt: Date

    var preview: String {
        let line = text.split(whereSeparator: { $0.isNewline }).first.map(String.init) ?? text
        let compact = line.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
        return compact.count > 64 ? String(compact.prefix(64)) + "…" : compact
    }
}

/// Session-only history of OCR captures and clipboard copies: bounded
/// memory, no text written to disk.
struct OCRHistory {
    private(set) var entries: [OCRHistoryEntry] = []
    let capacity: Int
    let byteLimit: Int

    init(capacity: Int = 40, byteLimit: Int = 1_048_576) {
        self.capacity = max(0, capacity)
        self.byteLimit = max(0, byteLimit)
    }

    @discardableResult
    mutating func add(_ text: String, source: HistorySource, at date: Date = Date()) -> Bool {
        guard capacity > 0, text.utf8.count <= byteLimit,
              !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return false }
        // Identical text collapses to a single, most-recent entry regardless of
        // source, so re-copying an earlier item just moves it back to the top.
        entries.removeAll { $0.text == text }
        entries.insert(OCRHistoryEntry(id: UUID(), text: text, source: source, capturedAt: date), at: 0)
        while entries.count > capacity || entries.reduce(0, { $0 + $1.text.utf8.count }) > byteLimit {
            entries.removeLast()
        }
        return true
    }

    func entry(id: UUID) -> OCRHistoryEntry? { entries.first { $0.id == id } }
    mutating func clear() { entries.removeAll() }
}
