import Carbon
import XCTest
@testable import MacOCR

final class HotkeyManagerTests: XCTestCase {
    @MainActor
    func testNativeDispatchConflictAndUnregistration() async throws {
        var activations = 0
        let manager = HotkeyManager(keyCode: UInt32(kVK_ANSI_2), modifiers: UInt32(cmdKey | shiftKey),
                                     id: 1, description: "Command + Shift + 2") { activations += 1 }
        do { try manager.register() }
        catch { throw XCTSkip("Interactive hotkey registration unavailable: \(error.localizedDescription)") }
        defer { manager.unregister() }
        let duplicate = HotkeyManager(keyCode: UInt32(kVK_ANSI_2), modifiers: UInt32(cmdKey | shiftKey),
                                       id: 1, description: "Command + Shift + 2") {}
        XCTAssertThrowsError(try duplicate.register())
        defer { duplicate.unregister() }

        for _ in 0..<10 {
            var event: EventRef?
            XCTAssertEqual(CreateEvent(nil, OSType(kEventClassKeyboard), UInt32(kEventHotKeyPressed),
                                       GetCurrentEventTime(), EventAttributes(kEventAttributeUserEvent), &event), noErr)
            let delivered = try XCTUnwrap(event)
            defer { ReleaseEvent(delivered) }
            var identifier = EventHotKeyID(signature: 0x4D4F4352, id: 1)
            XCTAssertEqual(SetEventParameter(delivered, EventParamName(kEventParamDirectObject),
                                            EventParamType(typeEventHotKeyID), MemoryLayout<EventHotKeyID>.size, &identifier), noErr)
            XCTAssertEqual(SendEventToEventTarget(delivered, GetApplicationEventTarget()), noErr)
        }
        XCTAssertEqual(activations, 10)
        manager.unregister()
        try duplicate.register()
    }
}
