import AppKit

@MainActor
struct ClipboardManager {
    func copy(_ text: String, to pasteboard: NSPasteboard = .general) throws {
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw MacOCRError.message("No text recognized; clipboard unchanged.")
        }
        // Prepare a complete item before acquiring the clipboard. No clear on OCR failure.
        let item = NSPasteboardItem()
        guard item.setString(text, forType: .string) else {
            throw MacOCRError.message("Could not prepare Unicode clipboard text.")
        }
        pasteboard.clearContents()
        guard pasteboard.writeObjects([item]) else {
            throw MacOCRError.message("Could not write to the clipboard.")
        }
    }
}

/// The native selector allows Control-drag to send an image directly to the
/// clipboard. Restore that side effect before OCR, including on cancellation.
@MainActor
struct CaptureClipboardGuard {
    private let changeCount: Int
    private let items: [[NSPasteboard.PasteboardType: Data]]

    init(board: NSPasteboard = .general) {
        changeCount = board.changeCount
        items = (board.pasteboardItems ?? []).map { item in
            var values: [NSPasteboard.PasteboardType: Data] = [:]
            for type in item.types { if let data = item.data(forType: type) { values[type] = data } }
            return values
        }
    }

    func restoreCaptureSideEffect(board: NSPasteboard = .general, fileWasCreated: Bool) throws {
        // Never revert ordinary clipboard edits made while the selector is open.
        guard !fileWasCreated, board.changeCount != changeCount,
              board.string(forType: .string) == nil,
              board.data(forType: .png) != nil || board.data(forType: .tiff) != nil else { return }
        let restored = items.map { values in
            let item = NSPasteboardItem()
            for (type, data) in values { item.setData(data, forType: type) }
            return item
        }
        board.clearContents()
        guard restored.isEmpty || board.writeObjects(restored) else {
            throw MacOCRError.message("Could not restore clipboard after Control-selection. Select a region without holding Control.")
        }
    }
}
