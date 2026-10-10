import AppKit
import XCTest
@testable import MacOCR

final class MenuBarControllerTests: XCTestCase {
    @MainActor
    func testHistoryMenuCopiesCompleteTextAndQuitDispatches() async throws {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        var history = OCRHistory()
        let original = "    Español: acción 👋\n        preserved    spaces"
        history.add(original, mode: .code)
        var quitRequested = false
        let controller = MenuBarController(onCapture: {}, onQuit: { quitRequested = true }, onMode: { _ in }, onCopy: { id in
            if let entry = history.entry(id: id) { try? ClipboardManager().copy(entry.text, to: board) }
        }, onClear: { history.clear() })
        defer { controller.remove() }
        controller.update(status: .ready, busy: false, mode: .code, history: history.entries, message: nil)
        let historyRoot = try XCTUnwrap(controller.menu.items.first { $0.title.hasPrefix("History (") })
        let historyMenu = try XCTUnwrap(historyRoot.submenu)
        historyMenu.performActionForItem(at: 0)
        XCTAssertEqual(board.string(forType: .string), original)
        let quitIndex = try XCTUnwrap(controller.menu.items.firstIndex { $0.title == "Quit MacOCR" })
        controller.menu.performActionForItem(at: quitIndex)
        XCTAssertTrue(quitRequested)
        let clearIndex = try XCTUnwrap(historyMenu.items.firstIndex { $0.title == "Clear History" })
        historyMenu.performActionForItem(at: clearIndex)
        XCTAssertTrue(history.entries.isEmpty)
    }

    @MainActor
    func testBusyStateKeepsQuitAvailableAndPreventsCompetingClipboardActions() async throws {
        var history = OCRHistory()
        history.add("text", mode: .normal)
        let controller = MenuBarController(onCapture: {}, onQuit: {}, onMode: { _ in }, onCopy: { _ in }, onClear: {})
        defer { controller.remove() }
        controller.update(status: .recognizing, busy: true, mode: .normal, history: history.entries, message: "Processing locally")
        XCTAssertFalse(try XCTUnwrap(controller.menu.items.first { $0.title == "Capture Text" }).isEnabled)
        XCTAssertTrue(try XCTUnwrap(controller.menu.items.first { $0.title == "Quit MacOCR" }).isEnabled)
        let root = try XCTUnwrap(controller.menu.items.first { $0.title.hasPrefix("History (") })
        XCTAssertFalse(try XCTUnwrap(root.submenu?.items.first).isEnabled)
    }

    @MainActor
    func testModeSelectionDispatchesNativeAction() async throws {
        var chosen: OCRMode?
        let controller = MenuBarController(onCapture: {}, onQuit: {}, onMode: { chosen = $0 }, onCopy: { _ in }, onClear: {})
        defer { controller.remove() }
        controller.update(status: .ready, busy: false, mode: .normal, history: [], message: nil)
        let codeIndex = try XCTUnwrap(controller.menu.items.firstIndex { $0.title == "Code Formatting" })
        controller.menu.performActionForItem(at: codeIndex)
        XCTAssertEqual(chosen, .code)
    }
}
