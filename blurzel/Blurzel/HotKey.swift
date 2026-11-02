import Carbon.HIToolbox
import Foundation

extension Notification.Name {
    static let hotKeyPressed = Notification.Name("BlurzelHotKeyPressed")
}

/// Registers ⇧⌘1 as a global shortcut. Carbon hot keys need no extra
/// permissions and work inside the App Sandbox.
enum HotKey {
    static func register() {
        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        InstallEventHandler(GetApplicationEventTarget(), { _, _, _ in
            NotificationCenter.default.post(name: .hotKeyPressed, object: nil)
            return noErr
        }, 1, &eventType, nil, nil)

        var hotKeyRef: EventHotKeyRef?
        let hotKeyID = EventHotKeyID(signature: OSType(0x424C_5A4C), id: 1) // "BLZL"
        RegisterEventHotKey(UInt32(kVK_ANSI_1), UInt32(cmdKey | shiftKey), hotKeyID, GetApplicationEventTarget(), 0, &hotKeyRef)
    }
}
