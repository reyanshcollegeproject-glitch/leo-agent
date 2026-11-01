import Foundation

/// Speech polisher from Flow Widget.
/// Strips filler words (um, uh, you know), cleans false starts, and formats dictation into clear intent commands.
class AIPolisher {
    func polish(rawTranscript: String) -> String {
        var text = rawTranscript.trimmingCharacters(in: .whitespacesAndNewlines)
        if text.isEmpty { return "" }

        // 1. Remove common filler words
        let fillers = [
            "\\bum\\b", "\\buh\\b", "\\ber\\b", "\\bah\\b",
            "\\byou know\\b", "\\blike basically\\b", "\\bkind of\\b"
        ]
        for pattern in fillers {
            if let regex = try? NSRegularExpression(pattern: "(?i)" + pattern, options: []) {
                text = regex.stringByReplacingMatches(
                    in: text,
                    options: [],
                    range: NSRange(location: 0, length: text.utf16.count),
                    withTemplate: ""
                )
            }
        }

        // 2. Resolve self-corrections (e.g., "open spotify wait no chrome" -> "open chrome")
        let correctionPatterns = [
            "(?i)\\b(?:wait no|sorry no|I mean)\\s+(.+)$",
            "(?i)\\b(?:no wait|actually)\\s+(.+)$"
        ]
        for pattern in correctionPatterns {
            if let regex = try? NSRegularExpression(pattern: pattern, options: []),
               let match = regex.firstMatch(in: text, options: [], range: NSRange(location: 0, length: text.utf16.count)) {
                if let targetRange = Range(match.range(at: 1), in: text) {
                    text = String(text[targetRange])
                }
            }
        }

        // 3. Clean up multiple whitespaces
        if let wsRegex = try? NSRegularExpression(pattern: "\\s+", options: []) {
            text = wsRegex.stringByReplacingMatches(
                in: text,
                options: [],
                range: NSRange(location: 0, length: text.utf16.count),
                withTemplate: " "
            )
        }

        return text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

