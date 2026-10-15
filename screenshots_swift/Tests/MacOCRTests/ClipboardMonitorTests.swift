import AppKit
import XCTest
@testable import MacOCR

final class ClipboardMonitorTests: XCTestCase {
    func testTextToRecordDecision() {
        func snap(_ string: String?, concealed: Bool = false, transient: Bool = false) -> ClipboardSnapshot {
            ClipboardSnapshot(changeCount: 1, string: string, isConcealed: concealed, isTransient: transient)
        }
        XCTAssertEqual(ClipboardMonitor.textToRecord(snap("hello")), "hello")
        XCTAssertEqual(ClipboardMonitor.textToRecord(snap("  keep spaces  ")), "  keep spaces  ")
        XCTAssertNil(ClipboardMonitor.textToRecord(snap(nil)))
        XCTAssertNil(ClipboardMonitor.textToRecord(snap(" \n\t")))
        XCTAssertNil(ClipboardMonitor.textToRecord(snap("secret", concealed: true)))
        XCTAssertNil(ClipboardMonitor.textToRecord(snap("temp", transient: true)))
    }

    @MainActor
    func testRecordsEachNewCopyOnlyOnce() {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        var recorded: [String] = []
        let monitor = ClipboardMonitor(pasteboard: board) { recorded.append($0) }

        board.clearContents(); board.setString("hello", forType: .string)
        monitor.poll()
        monitor.poll() // No change since last poll: no duplicate.
        board.clearContents(); board.setString("world", forType: .string)
        monitor.poll()
        XCTAssertEqual(recorded, ["hello", "world"])
    }

    @MainActor
    func testSkipsConcealedCopies() {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        var recorded: [String] = []
        let monitor = ClipboardMonitor(pasteboard: board) { recorded.append($0) }

        board.clearContents()
        board.setString("s3cret", forType: .string)
        board.setString("", forType: ClipboardMonitor.concealedType)
        monitor.poll()
        XCTAssertEqual(recorded, [])
    }

    @MainActor
    func testSyncBaselineHidesTheAppsOwnWrites() {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        var recorded: [String] = []
        let monitor = ClipboardMonitor(pasteboard: board) { recorded.append($0) }

        board.clearContents(); board.setString("ocr result", forType: .string)
        monitor.syncBaseline() // App wrote this itself; mark as seen.
        monitor.poll()
        XCTAssertEqual(recorded, [])
    }

    @MainActor
    func testPauseStopsRecordingAndResumesWithoutBackfill() {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        var recorded: [String] = []
        let monitor = ClipboardMonitor(pasteboard: board) { recorded.append($0) }

        monitor.setPaused(true)
        board.clearContents(); board.setString("while paused", forType: .string)
        monitor.poll()
        XCTAssertEqual(recorded, [])

        // Resuming does not retroactively grab what was copied during the pause.
        monitor.setPaused(false)
        monitor.poll()
        XCTAssertEqual(recorded, [])

        board.clearContents(); board.setString("after resume", forType: .string)
        monitor.poll()
        XCTAssertEqual(recorded, ["after resume"])
    }
}
