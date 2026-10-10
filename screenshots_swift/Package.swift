// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "MacOCR",
    platforms: [.macOS(.v14)],
    products: [.executable(name: "macocr", targets: ["MacOCR"])],
    targets: [
        .executableTarget(name: "MacOCR"),
        .testTarget(name: "MacOCRTests", dependencies: ["MacOCR"], resources: [.copy("Fixtures")])
    ]
)
