import XCTest
@testable import MacOCR

final class OCRHistoryTests: XCTestCase {
    func testPreservesCompleteUnicodeTextAndOrdersNewestFirst() throws {
        var history = OCRHistory()
        let original = "    acción 👋    corazón\n\n        print(value)\n"
        history.add(original, mode: .code, at: Date(timeIntervalSince1970: 1))
        history.add("next", mode: .normal, at: Date(timeIntervalSince1970: 2))
        XCTAssertEqual(history.entries.map(\.text), ["next", original])
        let entry = try XCTUnwrap(history.entries.last)
        XCTAssertEqual(history.entry(id: entry.id)?.text, original)
        XCTAssertEqual(entry.mode, .code)
    }

    func testDeduplicatesAndKeepsMostRecentEntries() {
        var history = OCRHistory(capacity: 2)
        history.add("one", mode: .normal)
        history.add("two", mode: .normal)
        history.add("one", mode: .normal)
        XCTAssertEqual(history.entries.map(\.text), ["one", "two"])
        history.add("three", mode: .normal)
        XCTAssertEqual(history.entries.map(\.text), ["three", "one"])
    }

    func testMemoryLimitAndClearAndEmptyInputs() {
        var history = OCRHistory(byteLimit: 5)
        XCTAssertFalse(history.add(" \n", mode: .normal))
        XCTAssertTrue(history.add("abc", mode: .normal))
        XCTAssertFalse(history.add("too large", mode: .normal))
        XCTAssertEqual(history.entries.first?.text, "abc")
        history.add("éé", mode: .code) // Four UTF-8 bytes, not two.
        XCTAssertEqual(history.entries.map(\.text), ["éé"])
        history.clear()
        XCTAssertTrue(history.entries.isEmpty)
        var disabled = OCRHistory(capacity: 0)
        XCTAssertFalse(disabled.add("text", mode: .normal))
        XCTAssertTrue(disabled.entries.isEmpty)
    }

    func testPreviewDoesNotModifyStoredText() throws {
        var history = OCRHistory()
        let text = "    " + String(repeating: "é", count: 100) + "\nsecond line"
        history.add(text, mode: .code)
        let entry = try XCTUnwrap(history.entries.first)
        XCTAssertTrue(entry.preview.hasSuffix("…"))
        XCTAssertFalse(entry.preview.contains("\n"))
        XCTAssertEqual(entry.text, text)
    }
}
