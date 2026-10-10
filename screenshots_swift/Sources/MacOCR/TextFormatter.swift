import Foundation

struct TextFormatter: Sendable {
    private struct Line {
        var fragments: [OCRFragment]
        var box: CGRect
    }

    func format(_ input: [OCRFragment], mode: OCRMode) -> String {
        let fragments = input.filter {
            !$0.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty &&
            $0.box.width > 0 && $0.box.height > 0 &&
            $0.box.minX.isFinite && $0.box.minY.isFinite &&
            $0.box.maxX.isFinite && $0.box.maxY.isFinite
        }
        guard !fragments.isEmpty else { return "" }
        let lines = groupLines(fragments)
        let heights = lines.map { $0.box.height }
        let typicalHeight = median(heights)
        let gaps = zip(lines, lines.dropFirst()).map { $1.box.midY - $0.box.midY }.filter { $0 > 0 }
        // The smaller half avoids paragraph gaps distorting the estimated line pitch.
        let sortedGaps = gaps.sorted()
        let pitch = max(typicalHeight, median(Array(sortedGaps.prefix(max(1, (sortedGaps.count + 1) / 2)))))
        let origin = fragments.map { $0.box.minX }.min() ?? 0
        let cellWidth = estimateCellWidth(fragments)
        var output: [String] = []
        for (index, line) in lines.enumerated() {
            if index > 0 {
                let distance = line.box.midY - lines[index - 1].box.midY
                if mode == .code {
                    let blankCount = min(100, max(0, Int((distance / pitch).rounded()) - 1))
                    output.append(contentsOf: repeatElement("", count: blankCount))
                } else if distance > pitch * 1.65 {
                    output.append("")
                }
            }
            if mode == .normal {
                output.append(line.fragments.map {
                    $0.text.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
                }.joined(separator: " "))
            } else {
                output.append(codeLine(line, origin: origin, cellWidth: cellWidth))
            }
        }
        return output.joined(separator: "\n")
    }

    private func groupLines(_ fragments: [OCRFragment]) -> [Line] {
        var lines: [Line] = []
        for fragment in fragments.sorted(by: {
            $0.box.midY == $1.box.midY ? $0.box.minX < $1.box.minX : $0.box.midY < $1.box.midY
        }) {
            if let index = lines.indices.last(where: {
                abs(lines[$0].box.midY - fragment.box.midY) <= min(lines[$0].box.height, fragment.box.height) * 0.45
            }) {
                lines[index].fragments.append(fragment)
                lines[index].box = lines[index].box.union(fragment.box)
            } else {
                lines.append(Line(fragments: [fragment], box: fragment.box))
            }
        }
        return lines.sorted { $0.box.midY < $1.box.midY }.map { line in
            Line(fragments: line.fragments.sorted { $0.box.minX < $1.box.minX }, box: line.box)
        }
    }

    private func estimateCellWidth(_ fragments: [OCRFragment]) -> CGFloat {
        let samples = fragments.flatMap { fragment -> [CGFloat] in
            let words = fragment.words.isEmpty ? [OCRWord(text: fragment.text, box: fragment.box)] : fragment.words
            return words.compactMap { word in
                let count = word.text.count
                guard count > 0, word.box.width.isFinite, word.box.width > 0 else { return nil }
                return word.box.width / CGFloat(count)
            }
        }
        return max(1, median(samples))
    }

    private func codeLine(_ line: Line, origin: CGFloat, cellWidth: CGFloat) -> String {
        var result = ""
        for fragment in line.fragments {
            let words = fragment.words.isEmpty
                ? [OCRWord(text: fragment.text.trimmingCharacters(in: .whitespacesAndNewlines), box: fragment.box)]
                : fragment.words
            var previousEnd = fragment.text.startIndex
            for word in words {
                guard word.box.minX.isFinite else { continue }
                let column = Int(min(4096, max(0, ((word.box.minX - origin) / cellWidth).rounded())))
                // Keep known internal whitespace as a lower bound when geometry overlaps.
                var knownSpaces = 0
                if !fragment.words.isEmpty,
                   let range = fragment.text.range(of: word.text, range: previousEnd..<fragment.text.endIndex) {
                    knownSpaces = fragment.text[previousEnd..<range.lowerBound].count
                    previousEnd = range.upperBound
                }
                let required = result.isEmpty ? column : max(column - result.count, max(1, knownSpaces))
                result += String(repeating: " ", count: min(4096, max(0, required))) + word.text
            }
        }
        return result
    }

    private func median(_ values: [CGFloat]) -> CGFloat {
        guard !values.isEmpty else { return 0 }
        let sorted = values.sorted(), middle = sorted.count / 2
        return sorted.count.isMultiple(of: 2) ? (sorted[middle - 1] + sorted[middle]) / 2 : sorted[middle]
    }
}
