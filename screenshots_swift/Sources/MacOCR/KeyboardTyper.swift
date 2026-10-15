import CoreGraphics
import Foundation

/// Types text by synthesizing Unicode keyboard events, bypassing the system
/// clipboard entirely. Some virtualized/remote-desktop apps grab the keyboard
/// but ignore Cmd+V, so this lets the clipboard contents be "typed" instead.
struct KeyboardTyper: Sendable {
    /// Confirmation threshold: longer text takes visibly long to "type" and
    /// could look like the app is stuck, so the caller should confirm first.
    static let confirmationThreshold = 450

    /// Delay between keystrokes so apps that poll their event queue (Electron,
    /// VM consoles, remote-desktop clients) don't drop characters typed too fast.
    private let interKeyDelay: TimeInterval

    init(interKeyDelay: TimeInterval = 0.004) {
        self.interKeyDelay = interKeyDelay
    }

    func type(_ text: String) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    try self.typeSynchronously(text)
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    private func typeSynchronously(_ text: String) throws {
        guard let source = CGEventSource(stateID: .combinedSessionState) else {
            throw MacOCRError.message("Could not create a keyboard event source.")
        }
        guard let keyDown = CGEvent(keyboardEventSource: source, virtualKey: 0, keyDown: true),
              let keyUp = CGEvent(keyboardEventSource: source, virtualKey: 0, keyDown: false) else {
            throw MacOCRError.message("Could not create synthetic keyboard events.")
        }
        for character in text {
            // keyboardSetUnicodeString overrides the glyph regardless of virtualKey,
            // so one key-down/key-up pair per character is enough, no keycode map needed.
            let units = Array(String(character).utf16)
            guard !units.isEmpty else { continue }
            keyDown.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
            keyUp.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
            keyDown.post(tap: .cgSessionEventTap)
            keyUp.post(tap: .cgSessionEventTap)
            if interKeyDelay > 0 { Thread.sleep(forTimeInterval: interKeyDelay) }
        }
    }
}
