import Carbon

@MainActor
final class HotkeyManager {
    private var hotkey: EventHotKeyRef?
    private var handler: EventHandlerRef?
    private let action: () -> Void
    private static let signature: OSType = 0x4D4F4352 // MOCR

    init(action: @escaping () -> Void) { self.action = action }

    func register() throws {
        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        let context = Unmanaged.passUnretained(self).toOpaque()
        let handlerStatus = InstallEventHandler(GetApplicationEventTarget(), { _, event, context in
            guard let event, let context else { return OSStatus(eventNotHandledErr) }
            var identifier = EventHotKeyID()
            let status = GetEventParameter(event, EventParamName(kEventParamDirectObject),
                                           EventParamType(typeEventHotKeyID), nil,
                                           MemoryLayout<EventHotKeyID>.size, nil, &identifier)
            guard status == noErr, identifier.signature == 0x4D4F4352, identifier.id == 1 else {
                return OSStatus(eventNotHandledErr)
            }
            // Carbon delivers application events on NSApplication's main thread.
            MainActor.assumeIsolated {
                Unmanaged<HotkeyManager>.fromOpaque(context).takeUnretainedValue().action()
            }
            return noErr
        }, 1, &eventType, context, &handler)
        guard handlerStatus == noErr else {
            throw MacOCRError.message("Could not install the hotkey handler (OSStatus \(handlerStatus)). Run in a logged-in macOS GUI session.")
        }
        let identifier = EventHotKeyID(signature: Self.signature, id: 1)
        // Carbon otherwise permits different processes to share this shortcut,
        // which could start multiple screen selectors for a single key press.
        let status = RegisterEventHotKey(UInt32(kVK_ANSI_2), UInt32(cmdKey | shiftKey), identifier,
                                         GetApplicationEventTarget(), OptionBits(kEventHotKeyExclusive), &hotkey)
        guard status == noErr else {
            unregister()
            let reason = status == eventHotKeyExistsErr ? "Shortcut already registered by another application." : "Registration failed."
            throw MacOCRError.message("Command + Shift + 2: \(reason) (OSStatus \(status)). Check System Settings > Keyboard > Keyboard Shortcuts and other hotkey applications. Use --once if the shortcut is unavailable.")
        }
    }

    func unregister() {
        if let hotkey { UnregisterEventHotKey(hotkey) }
        if let handler { RemoveEventHandler(handler) }
        hotkey = nil
        handler = nil
    }
}
