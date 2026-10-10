import XCTest
@testable import MacOCR

final class ConfigurationTests: XCTestCase {
    func testDefaults() throws {
        let config = try Configuration(arguments: [])
        XCTAssertFalse(config.once)
        XCTAssertFalse(config.menuBar)
        XCTAssertEqual(config.mode, .normal)
    }

    func testOptions() throws {
        let config = try Configuration(arguments: ["--once", "--mode", "code"])
        XCTAssertTrue(config.once)
        XCTAssertEqual(config.mode, .code)
        XCTAssertTrue(try Configuration(arguments: ["--help"]).help)
        XCTAssertTrue(try Configuration(arguments: ["--menu-bar"]).menuBar)
    }

    func testInvalidArguments() {
        for args in [["--mode"], ["--mode", "unknown"], ["--bogus"], ["filename.png"]] {
            XCTAssertThrowsError(try Configuration(arguments: args))
        }
    }
}
