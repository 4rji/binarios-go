import XCTest
@testable import MacOCR

final class ConfigurationTests: XCTestCase {
    func testDefaults() throws {
        let config = try Configuration(arguments: [])
        XCTAssertFalse(config.once)
        XCTAssertEqual(config.mode, .normal)
    }

    func testOptions() throws {
        let config = try Configuration(arguments: ["--once", "--mode", "code"])
        XCTAssertTrue(config.once)
        XCTAssertEqual(config.mode, .code)
        XCTAssertTrue(try Configuration(arguments: ["--help"]).help)
    }

    func testInvalidArguments() {
        for args in [["--mode"], ["--mode", "unknown"], ["--bogus"], ["filename.png"]] {
            XCTAssertThrowsError(try Configuration(arguments: args))
        }
    }
}
