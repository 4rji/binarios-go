import XCTest
@testable import MacOCR

final class OCRHistoryTests: XCTestCase {
    func testPreservesCompleteUnicodeTextAndOrdersNewestFirst() throws {
        var history = OCRHistory()
        let original = "    acción 👋    corazón\n\n        print(value)\n"
        history.add(original, source: .ocr(.code), at: Date(timeIntervalSince1970: 1))
        history.add("next", source: .clipboard, at: Date(timeIntervalSince1970: 2))
        XCTAssertEqual(history.entries.map(\.text), ["next", original])
        let entry = try XCTUnwrap(history.entries.last)
        XCTAssertEqual(history.entry(id: entry.id)?.text, original)
        XCTAssertEqual(entry.source, .ocr(.code))
        XCTAssertEqual(history.entries.first?.source, .clipboard)
    }

    func testDeduplicatesAndKeepsMostRecentEntries() {
        var history = OCRHistory(capacity: 2)
        history.add("one", source: .ocr(.normal))
        history.add("two", source: .ocr(.normal))
        history.add("one", source: .ocr(.normal))
        XCTAssertEqual(history.entries.map(\.text), ["one", "two"])
        history.add("three", source: .ocr(.normal))
        XCTAssertEqual(history.entries.map(\.text), ["three", "one"])
    }

    func testIdenticalTextCollapsesAcrossSources() {
        var history = OCRHistory()
        history.add("shared", source: .ocr(.normal))
        history.add("shared", source: .clipboard)
        XCTAssertEqual(history.entries.map(\.text), ["shared"])
        // The most recent add wins the single surviving entry's source.
        XCTAssertEqual(history.entries.first?.source, .clipboard)
    }

    func testMemoryLimitAndClearAndEmptyInputs() {
        var history = OCRHistory(byteLimit: 5)
        XCTAssertFalse(history.add(" \n", source: .clipboard))
        XCTAssertTrue(history.add("abc", source: .clipboard))
        XCTAssertFalse(history.add("too large", source: .clipboard))
        XCTAssertEqual(history.entries.first?.text, "abc")
        history.add("éé", source: .ocr(.code)) // Four UTF-8 bytes, not two.
        XCTAssertEqual(history.entries.map(\.text), ["éé"])
        history.clear()
        XCTAssertTrue(history.entries.isEmpty)
        var disabled = OCRHistory(capacity: 0)
        XCTAssertFalse(disabled.add("text", source: .clipboard))
        XCTAssertTrue(disabled.entries.isEmpty)
    }

    func testPreviewDoesNotModifyStoredText() throws {
        var history = OCRHistory()
        let text = "    " + String(repeating: "é", count: 100) + "\nsecond line"
        history.add(text, source: .ocr(.code))
        let entry = try XCTUnwrap(history.entries.first)
        XCTAssertTrue(entry.preview.hasSuffix("…"))
        XCTAssertFalse(entry.preview.contains("\n"))
        XCTAssertEqual(entry.text, text)
    }
}
