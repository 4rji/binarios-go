import Foundation
import Vision
import ImageIO

struct OCRWord: Sendable {
    let text: String
    let box: CGRect
}

struct OCRFragment: Sendable {
    let text: String
    /// Pixel coordinates with the origin at the top left.
    let box: CGRect
    let words: [OCRWord]

    init(text: String, box: CGRect, words: [OCRWord] = []) {
        self.text = text
        self.box = box
        self.words = words
    }
}

struct OCREngine: Sendable {
    func recognize(imageURL: URL, mode: OCRMode) throws -> [OCRFragment] {
        guard let source = CGImageSourceCreateWithURL(imageURL as CFURL, nil),
              let image = CGImageSourceCreateImageAtIndex(source, 0, nil) else {
            throw MacOCRError.message("Screenshot is missing, unreadable, or not a valid image.")
        }
        let request = VNRecognizeTextRequest()
        request.recognitionLevel = .accurate
        request.automaticallyDetectsLanguage = true
        request.recognitionLanguages = ["en-US", "es-ES"]
        request.usesLanguageCorrection = mode == .normal
        do {
            try VNImageRequestHandler(cgImage: image, options: [:]).perform([request])
        } catch {
            throw MacOCRError.message("Vision OCR failed: \(error.localizedDescription)")
        }
        let width = CGFloat(image.width), height = CGFloat(image.height)
        func pixels(_ rect: CGRect) -> CGRect {
            CGRect(x: rect.minX * width, y: (1 - rect.maxY) * height,
                   width: rect.width * width, height: rect.height * height)
        }
        return (request.results ?? []).compactMap { observation in
            guard let candidate = observation.topCandidates(1).first,
                  !candidate.string.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
            var words: [OCRWord] = []
            if mode == .code {
                // Accurate recognition provides WORD precision, even for a single-character
                // range. Query tokens rather than treating repeated word boxes as glyphs.
                let text = candidate.string
                var start = text.startIndex
                while start < text.endIndex {
                    if text[start].isWhitespace { start = text.index(after: start); continue }
                    var end = start
                    while end < text.endIndex && !text[end].isWhitespace { end = text.index(after: end) }
                    if let rectangle = try? candidate.boundingBox(for: start..<end) {
                        words.append(OCRWord(text: String(text[start..<end]), box: pixels(rectangle.boundingBox)))
                    } else {
                        // Partial geometry would silently discard text. Fall back to the full line.
                        words = []
                        break
                    }
                    start = end
                }
            }
            return OCRFragment(text: candidate.string, box: pixels(observation.boundingBox), words: words)
        }
    }
}
