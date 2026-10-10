import AppKit
import XCTest
@testable import MacOCR

final class ClipboardManagerTests: XCTestCase {
    @MainActor
    func testUnicodeAndWhitespaceAndEmptyFailure() async throws {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        let manager = ClipboardManager()
        let text = "Español: acción 👋\n    código    alineado"
        try manager.copy(text, to: board)
        XCTAssertEqual(board.string(forType: .string), text)
        let count = board.changeCount
        XCTAssertThrowsError(try manager.copy(" \n", to: board))
        XCTAssertEqual(board.string(forType: .string), text)
        XCTAssertEqual(board.changeCount, count)
    }

    @MainActor
    func testNativeCaptureImageSideEffectIsRestored() async throws {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        board.setString("previous clipboard", forType: .string)
        let guardState = CaptureClipboardGuard(board: board)
        board.clearContents()
        board.setData(Data([1, 2, 3]), forType: .png)
        try guardState.restoreCaptureSideEffect(board: board, fileWasCreated: false)
        XCTAssertEqual(board.string(forType: .string), "previous clipboard")
    }

    @MainActor
    func testCaptureGuardKeepsUnrelatedTextAndNormalCapture() async throws {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        board.setString("previous", forType: .string)
        let guardState = CaptureClipboardGuard(board: board)
        board.clearContents()
        board.setString("new text from another app", forType: .string)
        try guardState.restoreCaptureSideEffect(board: board, fileWasCreated: false)
        XCTAssertEqual(board.string(forType: .string), "new text from another app")
        board.clearContents()
        board.setData(Data([1]), forType: .png)
        let count = board.changeCount
        try guardState.restoreCaptureSideEffect(board: board, fileWasCreated: true)
        XCTAssertEqual(board.changeCount, count)
    }
}
