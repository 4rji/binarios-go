import AppKit
import XCTest
@testable import MacOCR

final class ScreenCaptureTests: XCTestCase {
    @MainActor
    func testRepeatedSuccessfulCaptureRecognizesAndCopiesText() async throws {
        let image = try OCRFixture.image(lines: ["Hello world", "Español: acción y corazón"])
        let root = image.deletingLastPathComponent()
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try XCTUnwrap(Bundle.module.url(forResource: "successful-capture", withExtension: "sh", subdirectory: "Fixtures"))
        let executable = root.appendingPathComponent("capture-helper")
        try FileManager.default.copyItem(at: fixture, to: executable)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: executable.path)
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        let capture = ScreenCapture(executableURL: executable, temporaryRoot: root, pasteboard: board, preflightAccess: { true })
        for mode in [OCRMode.normal, .code] {
            board.clearContents()
            board.setString("sentinel", forType: .string)
            let started = ContinuousClock.now
            let saved = try await capture.capture()
            XCTAssertLessThan(started.duration(to: .now), .seconds(2))
            let text = TextFormatter().format(try OCREngine().recognize(imageURL: saved, mode: mode), mode: mode)
            try ClipboardManager().copy(text, to: board)
            XCTAssertTrue(try XCTUnwrap(board.string(forType: .string)).contains("Hello world"))
            XCTAssertTrue(try XCTUnwrap(board.string(forType: .string)).contains("acción y corazón"))
            capture.cleanup()
            XCTAssertEqual(Set(try FileManager.default.contentsOfDirectory(atPath: root.path)), ["fixture.png", "capture-helper"])
        }
    }

    @MainActor
    func testCaptureCompletionDoesNotWaitForDescendantStderrEOF() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("MacOCR-capture-test-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try XCTUnwrap(Bundle.module.url(forResource: "inherited-stderr", withExtension: "sh", subdirectory: "Fixtures"))
        let executable = root.appendingPathComponent("capture-helper")
        try FileManager.default.copyItem(at: fixture, to: executable)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: executable.path)
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        let capture = ScreenCapture(executableURL: executable, temporaryRoot: root, pasteboard: board, preflightAccess: { true })
        let started = ContinuousClock.now
        do {
            _ = try await capture.capture()
            XCTFail("No image was created")
        } catch MacOCRError.cancelled { }
        XCTAssertLessThan(started.duration(to: .now), .seconds(2), "A surviving helper must not hold capture completion hostage to pipe EOF")
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: root.path), ["capture-helper"])
    }

    @MainActor
    func testCancellationCleansTemporaryDirectoryAndKeepsClipboard() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("MacOCR-capture-test-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: root) }
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        board.setString("sentinel", forType: .string)
        let count = board.changeCount
        // Both native cancellation outcomes: zero/no file or exit 1/no file.
        for executable in ["/usr/bin/true", "/usr/bin/false"] {
            let capture = ScreenCapture(executableURL: URL(fileURLWithPath: executable), temporaryRoot: root,
                                        pasteboard: board, preflightAccess: { true }, requestAccess: { XCTFail("Unexpected permission request"); return false })
            do {
                _ = try await capture.capture()
                XCTFail("Expected cancellation")
            } catch MacOCRError.cancelled {
                // Expected: no image produced, no clipboard write, no leaked directory.
            }
            XCTAssertTrue(try FileManager.default.contentsOfDirectory(atPath: root.path).isEmpty)
            XCTAssertEqual(board.string(forType: .string), "sentinel")
            XCTAssertEqual(board.changeCount, count)
        }
    }

    @MainActor
    func testMissingExecutableCleansTemporaryDirectory() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("MacOCR-capture-test-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: root) }
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        let capture = ScreenCapture(executableURL: root.appendingPathComponent("missing"), temporaryRoot: root,
                                    pasteboard: board, preflightAccess: { true })
        do {
            _ = try await capture.capture()
            XCTFail("Expected launch failure")
        } catch {
            XCTAssertTrue(error.localizedDescription.contains("Could not start screencapture"))
        }
        XCTAssertTrue(try FileManager.default.contentsOfDirectory(atPath: root.path).isEmpty)
    }

    @MainActor
    func testPermissionDenialDoesNotStartCapture() async throws {
        let board = NSPasteboard.withUniqueName()
        defer { board.releaseGlobally() }
        board.setString("sentinel", forType: .string)
        var requests = 0
        let capture = ScreenCapture(pasteboard: board, preflightAccess: { false }, requestAccess: { requests += 1; return false })
        do {
            _ = try await capture.capture()
            XCTFail("Expected permission error")
        } catch {
            XCTAssertTrue(error.localizedDescription.contains("Screen Recording permission is missing"))
        }
        XCTAssertEqual(requests, 1)
        XCTAssertEqual(board.string(forType: .string), "sentinel")
    }
}
