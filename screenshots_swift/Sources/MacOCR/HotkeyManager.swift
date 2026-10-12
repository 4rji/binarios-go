import Carbon

@MainActor
final class HotkeyManager {
    private var hotkey: EventHotKeyRef?
    private var handler: EventHandlerRef?
    private let action: () -> Void
    private let id: UInt32
    private let keyCode: UInt32
    private let modifiers: UInt32
    private let description: String
    private static let signature: OSType = 0x4D4F4352 // MOCR

    init(keyCode: UInt32, modifiers: UInt32, id: UInt32, description: String, action: @escaping () -> Void) {
        self.keyCode = keyCode
        self.modifiers = modifiers
        self.id = id
        self.description = description
        self.action = action
    }

    func register() throws {
        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        let context = Unmanaged.passUnretained(self).toOpaque()
        let handlerStatus = InstallEventHandler(GetApplicationEventTarget(), { _, event, context in
            guard let event, let context else { return OSStatus(eventNotHandledErr) }
            var identifier = EventHotKeyID()
            let status = GetEventParameter(event, EventParamName(kEventParamDirectObject),
                                           EventParamType(typeEventHotKeyID), nil,
                                           MemoryLayout<EventHotKeyID>.size, nil, &identifier)
            let manager = Unmanaged<HotkeyManager>.fromOpaque(context).takeUnretainedValue()
            guard status == noErr, identifier.signature == HotkeyManager.signature, identifier.id == manager.id else {
                return OSStatus(eventNotHandledErr)
            }
            // Carbon delivers application events on NSApplication's main thread.
            MainActor.assumeIsolated { manager.action() }
            return noErr
        }, 1, &eventType, context, &handler)
        guard handlerStatus == noErr else {
            throw MacOCRError.message("Could not install the hotkey handler for \(description) (OSStatus \(handlerStatus)). Run in a logged-in macOS GUI session.")
        }
        let identifier = EventHotKeyID(signature: Self.signature, id: id)
        // Carbon otherwise permits different processes to share this shortcut,
        // which could start multiple screen selectors for a single key press.
        let status = RegisterEventHotKey(keyCode, modifiers, identifier,
                                         GetApplicationEventTarget(), OptionBits(kEventHotKeyExclusive), &hotkey)
        guard status == noErr else {
            unregister()
            let reason = status == eventHotKeyExistsErr ? "Shortcut already registered by another application." : "Registration failed."
            throw MacOCRError.message("\(description): \(reason) (OSStatus \(status)). Check System Settings > Keyboard > Keyboard Shortcuts and other hotkey applications. Use --once if the shortcut is unavailable.")
        }
    }

    func unregister() {
        if let hotkey { UnregisterEventHotKey(hotkey) }
        if let handler { RemoveEventHandler(handler) }
        hotkey = nil
        handler = nil
    }
}
