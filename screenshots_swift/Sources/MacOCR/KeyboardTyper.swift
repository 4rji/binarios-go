import Carbon
import CoreGraphics
import Foundation

/// Types text by synthesizing keyboard events, bypassing the system clipboard
/// entirely. Some virtualized/remote-desktop apps grab the keyboard but ignore
/// Cmd+V, so this lets the clipboard contents be "typed" instead.
///
/// Printable ASCII is posted as real key presses (actual virtual keycode plus a
/// Shift flag when needed). Web terminals and VM/remote-desktop consoles
/// (noVNC, Guacamole, xterm.js) read the keycode of the event, not the Unicode
/// string overlaid with keyboardSetUnicodeString, so a keycode-0 ("a") event
/// with a Unicode glyph lands as a stream of "a" in those clients. Non-ASCII
/// characters, which have no simple keycode, still use the Unicode-string path.
struct KeyboardTyper: Sendable {
    /// Confirmation threshold: longer text takes visibly long to "type" and
    /// could look like the app is stuck, so the caller should confirm first.
    static let confirmationThreshold = 450

    /// Delay between keystrokes so apps that poll their event queue (Electron,
    /// VM consoles, remote-desktop clients) don't drop characters typed too fast.
    /// WebMKS/VNC consoles (ESXi) tunnel keystrokes over a WebSocket and drop or
    /// reorder characters sent faster than ~15 ms, so the default is kept well
    /// above that; native fields tolerate it fine.
    private let interKeyDelay: TimeInterval

    /// Tab/Enter drive column/row changes in spreadsheet grids (Excel, Sheets),
    /// which need more time to commit the active cell than a plain character does.
    private let tabNewlineDelay: TimeInterval

    init(interKeyDelay: TimeInterval = 0.02, tabNewlineDelay: TimeInterval = 0.05) {
        self.interKeyDelay = interKeyDelay
        self.tabNewlineDelay = tabNewlineDelay
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
        // The OCR hotkey (e.g. Shift+Option+…) is usually still physically held
        // the instant typing starts. Events posted to the HID tap are injected
        // below the modifier-combining layer, so the live hardware Shift/Option
        // get OR'd onto every keystroke — output came out UPPERCASE and, with
        // Option, as Alt/meta (ESC-prefixed in terminals: "^[R^[O^[X…"). Wait
        // for the user to let go of the modifiers before typing a single key.
        Self.waitForModifiersToClear()

        // Private state, not combinedSessionState: the event's own flags are
        // used as-is instead of being merged with the session modifier state.
        guard let source = CGEventSource(stateID: .privateState) else {
            throw MacOCRError.message("Could not create a keyboard event source.")
        }
        guard let keyDown = CGEvent(keyboardEventSource: source, virtualKey: 0, keyDown: true),
              let keyUp = CGEvent(keyboardEventSource: source, virtualKey: 0, keyDown: false) else {
            throw MacOCRError.message("Could not create synthetic keyboard events.")
        }
        // No modifiers on the Unicode-fallback events either.
        keyDown.flags = []
        keyUp.flags = []
        let keyMap = Self.asciiKeyMap()
        for character in text {
            // Tab and newline are control characters: keyboardSetUnicodeString
            // posts them as a Unicode glyph that spreadsheets and most text
            // fields silently ignore, so they must be posted as real key presses
            // (virtualKey 48 = Tab, 36 = Return) to actually move between cells.
            if let virtualKey = Self.controlKeyCode(for: character) {
                try postKeyStroke(source: source, virtualKey: virtualKey)
                Thread.sleep(forTimeInterval: tabNewlineDelay)
                continue
            }
            // Printable ASCII: post the real keycode (with Shift when the layout
            // needs it) so keycode-reading clients receive the right character
            // instead of a stream of "a" from the Unicode-only keycode-0 event.
            if let stroke = keyMap[character] {
                try postKeyStroke(source: source, virtualKey: stroke.keyCode, shift: stroke.shift)
                if interKeyDelay > 0 { Thread.sleep(forTimeInterval: interKeyDelay) }
                continue
            }
            // Fallback for non-ASCII (accents, emoji, …): no simple keycode, so
            // overlay the glyph with keyboardSetUnicodeString. Native text fields
            // read this; keycode-only remote consoles may still drop it.
            let units = Array(String(character).utf16)
            guard !units.isEmpty else { continue }
            keyDown.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
            keyUp.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
            keyDown.post(tap: .cghidEventTap)
            keyUp.post(tap: .cghidEventTap)
            if interKeyDelay > 0 { Thread.sleep(forTimeInterval: interKeyDelay) }
        }
    }

    private func postKeyStroke(source: CGEventSource, virtualKey: CGKeyCode, shift: Bool = false) throws {
        guard let down = CGEvent(keyboardEventSource: source, virtualKey: virtualKey, keyDown: true),
              let up = CGEvent(keyboardEventSource: source, virtualKey: virtualKey, keyDown: false) else {
            throw MacOCRError.message("Could not create a synthetic key event.")
        }
        // Always set flags explicitly (clear to none unless Shift is wanted) so
        // no stray modifier from the triggering hotkey rides along.
        let flags: CGEventFlags = shift ? .maskShift : []
        down.flags = flags
        up.flags = flags
        down.post(tap: .cghidEventTap)
        up.post(tap: .cghidEventTap)
    }

    /// Blocks until no Shift/Control/Option/Command key is physically down, so a
    /// still-held triggering hotkey (the type action fires on Option+Shift+2)
    /// doesn't ride along on the HID-injected keys. Events posted to the HID tap
    /// are merged with the .hidSystemState modifier state, so that is the state
    /// that must read clear — .combinedSessionState alone misses it. Both are
    /// checked. Capped so a user who keeps a modifier down forever still gets text.
    private static func waitForModifiersToClear(timeout: TimeInterval = 3.0) {
        let modifierMask: CGEventFlags = [.maskShift, .maskControl, .maskAlternate, .maskCommand]
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            let hid = CGEventSource.flagsState(.hidSystemState)
            let session = CGEventSource.flagsState(.combinedSessionState)
            if hid.intersection(modifierMask).isEmpty,
               session.intersection(modifierMask).isEmpty {
                return
            }
            Thread.sleep(forTimeInterval: 0.01)
        }
    }

    private static func controlKeyCode(for character: Character) -> CGKeyCode? {
        switch character {
        case "\t": return 48 // kVK_Tab
        case "\n", "\r": return 36 // kVK_Return
        default: return nil
        }
    }

    /// Builds a map from each printable-ASCII character (including space) to the
    /// virtual keycode and Shift state that produce it on the *currently active*
    /// keyboard layout, so the fix works on non-US layouts too. Derived by
    /// asking UCKeyTranslate what glyph each keycode emits with and without
    /// Shift, then inverting that into character → keystroke.
    private static func asciiKeyMap() -> [Character: (keyCode: CGKeyCode, shift: Bool)] {
        guard let inputSource = TISCopyCurrentKeyboardLayoutInputSource()?.takeRetainedValue(),
              let layoutPtr = TISGetInputSourceProperty(inputSource, kTISPropertyUnicodeKeyLayoutData) else {
            return [:]
        }
        let layoutData = unsafeBitCast(layoutPtr, to: CFData.self) as Data
        var map: [Character: (keyCode: CGKeyCode, shift: Bool)] = [:]
        let keyboardType = UInt32(LMGetKbdType())

        layoutData.withUnsafeBytes { (raw: UnsafeRawBufferPointer) in
            guard let base = raw.baseAddress else { return }
            let keyLayout = base.assumingMemoryBound(to: UCKeyboardLayout.self)
            // Shift modifier in UCKeyTranslate's (EventModifiers >> 8) & 0xFF form.
            let shiftState = UInt32(shiftKey >> 8) & 0xFF
            for keyCode in 0..<CGKeyCode(128) {
                for (modifierState, shift) in [(UInt32(0), false), (shiftState, true)] {
                    var deadKeyState: UInt32 = 0
                    var length = 0
                    var chars = [UniChar](repeating: 0, count: 4)
                    let status = UCKeyTranslate(
                        keyLayout,
                        UInt16(keyCode),
                        UInt16(kUCKeyActionDisplay),
                        modifierState,
                        keyboardType,
                        OptionBits(kUCKeyTranslateNoDeadKeysBit),
                        &deadKeyState,
                        chars.count,
                        &length,
                        &chars)
                    guard status == noErr, length == 1 else { continue }
                    let scalarValue = UInt32(chars[0])
                    // Printable ASCII, 0x20 (space) through 0x7E (~).
                    guard scalarValue >= 0x20, scalarValue <= 0x7E,
                          let scalar = Unicode.Scalar(scalarValue) else { continue }
                    let character = Character(scalar)
                    // Prefer the no-Shift chord when a character has two sources.
                    if map[character] == nil {
                        map[character] = (keyCode, shift)
                    }
                }
            }
        }
        return map
    }
}
