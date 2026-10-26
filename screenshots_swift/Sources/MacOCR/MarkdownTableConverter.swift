import Foundation

/// Converts a GitHub-flavored markdown table into tab-separated values, so
/// typing it with KeyboardTyper lands one cell per column in spreadsheet
/// grids (Excel, Sheets) instead of one long string in a single cell.
struct MarkdownTableConverter {
    /// Scans for a header + `---` separator pair anywhere in the text (not just
    /// at the very start), since users often copy a table along with the
    /// surrounding prose that introduced it.
    static func tsv(from text: String) -> String? {
        let lines = text.components(separatedBy: .newlines)
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        guard lines.count >= 2 else { return nil }
        guard let headerIndex = (0..<lines.count - 1).first(where: { index in
            isTableRow(lines[index]) && isSeparatorRow(lines[index + 1])
        }) else { return nil }
        var rows = [lines[headerIndex]]
        var index = headerIndex + 2
        while index < lines.count, isTableRow(lines[index]) {
            rows.append(lines[index])
            index += 1
        }
        let tsvRows = rows.map { cells(of: $0).joined(separator: "\t") }
        return tsvRows.joined(separator: "\n")
    }

    private static func isTableRow(_ line: String) -> Bool {
        line.contains("|")
    }

    /// The row under the header made only of dashes/colons/pipes, e.g. `|---|:--:|`.
    private static func isSeparatorRow(_ line: String) -> Bool {
        let stripped = line.trimmingCharacters(in: CharacterSet(charactersIn: "|"))
        guard !stripped.isEmpty, stripped.contains("-") else { return false }
        let allowed = CharacterSet(charactersIn: "-: |")
        return stripped.unicodeScalars.allSatisfy { allowed.contains($0) }
    }

    private static func cells(of line: String) -> [String] {
        var trimmed = line.trimmingCharacters(in: .whitespaces)
        if trimmed.hasPrefix("|") { trimmed.removeFirst() }
        if trimmed.hasSuffix("|") { trimmed.removeLast() }
        return trimmed.components(separatedBy: "|").map { $0.trimmingCharacters(in: .whitespaces) }
    }
}
