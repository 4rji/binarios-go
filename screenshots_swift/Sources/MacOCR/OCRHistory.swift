import Foundation

struct OCRHistoryEntry: Identifiable, Sendable {
    let id: UUID
    let text: String
    let mode: OCRMode
    let capturedAt: Date

    var preview: String {
        let line = text.split(whereSeparator: { $0.isNewline }).first.map(String.init) ?? text
        let compact = line.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
        return compact.count > 64 ? String(compact.prefix(64)) + "…" : compact
    }
}

/// Session-only history: bounded memory, no text written to disk.
struct OCRHistory {
    private(set) var entries: [OCRHistoryEntry] = []
    let capacity: Int
    let byteLimit: Int

    init(capacity: Int = 20, byteLimit: Int = 1_048_576) {
        self.capacity = max(0, capacity)
        self.byteLimit = max(0, byteLimit)
    }

    @discardableResult
    mutating func add(_ text: String, mode: OCRMode, at date: Date = Date()) -> Bool {
        guard capacity > 0, text.utf8.count <= byteLimit,
              !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return false }
        entries.removeAll { $0.text == text && $0.mode == mode }
        entries.insert(OCRHistoryEntry(id: UUID(), text: text, mode: mode, capturedAt: date), at: 0)
        while entries.count > capacity || entries.reduce(0, { $0 + $1.text.utf8.count }) > byteLimit {
            entries.removeLast()
        }
        return true
    }

    func entry(id: UUID) -> OCRHistoryEntry? { entries.first { $0.id == id } }
    mutating func clear() { entries.removeAll() }
}
