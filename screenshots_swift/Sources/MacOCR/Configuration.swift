import Foundation

enum OCRMode: String, Sendable {
    case normal, code
}

struct Configuration: Sendable {
    var once = false
    var mode: OCRMode = .normal
    var help = false
    var menuBar = false

    init(arguments: [String]) throws {
        var index = 0
        while index < arguments.count {
            switch arguments[index] {
            case "--help", "-h": help = true
            case "--once": once = true
            case "--menu-bar": menuBar = true
            case "--mode":
                index += 1
                guard index < arguments.count, let mode = OCRMode(rawValue: arguments[index]) else {
                    throw MacOCRError.message("--mode requires 'normal' or 'code'.")
                }
                self.mode = mode
            default: throw MacOCRError.message("Unknown option: \(arguments[index]). Use --help.")
            }
            index += 1
        }
    }

    static let usage = """
    MacOCR — local screen OCR for macOS 14+ (Apple Silicon)

    Usage: macocr [--once] [--mode normal|code] [--menu-bar] [--help]

      --once          Select a region, recognize, copy, and exit.
      --menu-bar      Show a status icon, history, and Quit menu.
      --mode normal   Readable lines and paragraphs (default).
      --mode code     Estimate indentation and column spacing from geometry.
      --help, -h      Show this help.

    Without --once, press Command + Shift + 2 to capture. Drag a rectangle;
    Escape cancels. Control-C stops the service. Screen Recording permission
    is required; OCR is performed entirely on this Mac.
    Code spacing is approximate, especially for proportional fonts.
    """
}

enum MacOCRError: Error, LocalizedError, Sendable {
    case cancelled
    case message(String)

    var errorDescription: String? {
        switch self {
        case .cancelled: return "Capture canceled; clipboard unchanged."
        case .message(let message): return message
        }
    }
}
