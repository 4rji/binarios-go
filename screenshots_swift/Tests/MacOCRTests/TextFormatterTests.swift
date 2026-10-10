import XCTest
@testable import MacOCR

final class TextFormatterTests: XCTestCase {
    private let formatter = TextFormatter()

    private func fragment(_ text: String, x: CGFloat = 0, y: CGFloat, width: CGFloat? = nil) -> OCRFragment {
        OCRFragment(text: text, box: CGRect(x: x, y: y, width: width ?? CGFloat(text.count) * 10, height: 12))
    }

    func testNormalReadingOrderAndUnicode() {
        let input = [fragment("  Español:   acción  ", y: 20), fragment("world", x: 70, y: 0.5), fragment("Hello", y: 0)]
        XCTAssertEqual(formatter.format(input, mode: .normal), "Hello world\nEspañol: acción")
    }

    func testParagraphGap() {
        let input = [fragment("One", y: 0), fragment("Two", y: 20), fragment("Three", y: 60)]
        XCTAssertEqual(formatter.format(input, mode: .normal), "One\nTwo\n\nThree")
    }

    func testCodeIndentation() {
        let input = [fragment("if ready {", y: 0), fragment("print(value)", x: 40, y: 20), fragment("}", y: 40)]
        XCTAssertEqual(formatter.format(input, mode: .code), "if ready {\n    print(value)\n}")
    }

    func testTerminalAndTableColumns() {
        let input = [fragment("NAME", y: 0), fragment("PID", x: 120, y: 0),
                     fragment("macocr", y: 20), fragment("123", x: 120, y: 20)]
        XCTAssertEqual(formatter.format(input, mode: .code), "NAME        PID\nmacocr      123")
    }

    func testWordGeometryRestoresCollapsedSpaces() {
        let words = [OCRWord(text: "let", box: CGRect(x: 0, y: 0, width: 30, height: 12)),
                     OCRWord(text: "value", box: CGRect(x: 80, y: 0, width: 50, height: 12))]
        let input = [OCRFragment(text: "let value", box: CGRect(x: 0, y: 0, width: 130, height: 12), words: words)]
        XCTAssertEqual(formatter.format(input, mode: .code), "let     value")
    }

    func testKnownMultipleSpacesSurvive() {
        XCTAssertEqual(formatter.format([fragment("one   two", y: 0)], mode: .code), "one   two")
    }

    func testCodeBlankLines() {
        let input = [fragment("a", y: 0), fragment("b", y: 20), fragment("c", y: 80)]
        XCTAssertEqual(formatter.format(input, mode: .code), "a\nb\n\n\nc")
    }

    func testEmptyAndInvalidGeometry() {
        XCTAssertEqual(formatter.format([], mode: .normal), "")
        XCTAssertEqual(formatter.format([fragment("   ", y: 0)], mode: .code), "")
        let invalid = OCRFragment(text: "bad", box: CGRect(x: CGFloat.nan, y: 0, width: 10, height: 12))
        XCTAssertEqual(formatter.format([invalid], mode: .code), "")
    }

    func testOverlappingWordsDoNotLoseText() {
        let words = [OCRWord(text: "foo", box: CGRect(x: 0, y: 0, width: 50, height: 12)),
                     OCRWord(text: "foo", box: CGRect(x: 0, y: 0, width: 50, height: 12))]
        let input = [OCRFragment(text: "foo  foo", box: CGRect(x: 0, y: 0, width: 50, height: 12), words: words)]
        XCTAssertEqual(formatter.format(input, mode: .code), "foo  foo")
    }

    func testSingleIndentedLineUsesRelativeOrigin() {
        XCTAssertEqual(formatter.format([fragment("text", x: 90, y: 0)], mode: .code), "text")
    }
}
