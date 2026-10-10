import XCTest
import CoreGraphics
import CoreText
import ImageIO
import UniformTypeIdentifiers
@testable import MacOCR

enum OCRFixture {
    /// These fixtures are rendered offscreen; no screen access or clipboard mutation.
    static func image(lines: [String], fontName: String = "Menlo", size: CGFloat = 36) throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("MacOCR-test-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: false)
        let url = folder.appendingPathComponent("fixture.png")
        let width = 1600, height = 600
        let context = try XCTUnwrap(CGContext(data: nil, width: width, height: height, bitsPerComponent: 8,
            bytesPerRow: width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(gray: 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        let font = CTFontCreateWithName(fontName as CFString, size, nil)
        for (index, text) in lines.enumerated() {
            let attributes: [NSAttributedString.Key: Any] = [
                NSAttributedString.Key(kCTFontAttributeName as String): font,
                NSAttributedString.Key(kCTForegroundColorAttributeName as String): CGColor(gray: 0, alpha: 1)
            ]
            let line = CTLineCreateWithAttributedString(NSAttributedString(string: text, attributes: attributes))
            context.textPosition = CGPoint(x: 50, y: 520 - index * 65)
            CTLineDraw(line, context)
        }
        let bitmap = try XCTUnwrap(context.makeImage())
        let destination = try XCTUnwrap(CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, bitmap, nil)
        XCTAssertTrue(CGImageDestinationFinalize(destination))
        return url
    }
}

final class OCREngineTests: XCTestCase {
    private func image(lines: [String], fontName: String = "Menlo", size: CGFloat = 36) throws -> URL {
        try OCRFixture.image(lines: lines, fontName: fontName, size: size)
    }

    private func cleanup(_ url: URL) { try? FileManager.default.removeItem(at: url.deletingLastPathComponent()) }

    func testEnglishSpanishAndMultiline() throws {
        let url = try image(lines: ["Hello world", "Español: acción y corazón", "This is another line"], fontName: "Helvetica")
        defer { cleanup(url) }
        let result = TextFormatter().format(try OCREngine().recognize(imageURL: url, mode: .normal), mode: .normal)
        XCTAssertTrue(result.contains("Hello world"), result)
        XCTAssertTrue(result.contains("acción y corazón"), result)
        XCTAssertTrue(result.contains("This is another line"), result)
        XCTAssertEqual(result.components(separatedBy: "\n").filter { !$0.isEmpty }.count, 3)
    }

    func testRealVisionCodeAndTerminal() throws {
        let url = try image(lines: ["if ready {", "    print(value)", "}", "NAME        PID", "macocr      123"])
        defer { cleanup(url) }
        let result = TextFormatter().format(try OCREngine().recognize(imageURL: url, mode: .code), mode: .code)
        let lines = result.components(separatedBy: "\n")
        let indented = try XCTUnwrap(lines.first(where: { $0.contains("print") }), result)
        XCTAssertTrue(indented.hasPrefix("   "), result)
        XCTAssertTrue(result.contains("NAME"), result)
        XCTAssertTrue(result.contains("123"), result)
        let heading = try XCTUnwrap(lines.first(where: { $0.contains("NAME") }), result)
        let row = try XCTUnwrap(lines.first(where: { $0.contains("macocr") }), result)
        let headingColumn = try XCTUnwrap(heading.range(of: "PID"), result)
        let rowColumn = try XCTUnwrap(row.range(of: "123"), result)
        XCTAssertLessThanOrEqual(abs(heading.distance(from: heading.startIndex, to: headingColumn.lowerBound) - row.distance(from: row.startIndex, to: rowColumn.lowerBound)), 2, result)
    }

    func testEmptyImage() throws {
        let url = try image(lines: [])
        defer { cleanup(url) }
        XCTAssertTrue(try OCREngine().recognize(imageURL: url, mode: .normal).isEmpty)
    }

    func testUnreadableImage() {
        XCTAssertThrowsError(try OCREngine().recognize(imageURL: URL(fileURLWithPath: "/private/tmp/MacOCR-nonexistent-\(UUID().uuidString).png"), mode: .normal))
    }
}
