import SwiftUI

struct Theme {
    // Obsidian Black & Charcoal
    static let bgBlack = Color(red: 0.04, green: 0.05, blue: 0.07)
    static let cardCharcoal = Color(red: 0.08, green: 0.10, blue: 0.14)
    static let cardHover = Color(red: 0.12, green: 0.15, blue: 0.20)
    static let borderGraphite = Color(red: 0.18, green: 0.22, blue: 0.28)
    static let borderSubtle = Color(red: 0.14, green: 0.17, blue: 0.22)

    // Crisp Studio White & Silvers
    static let textWhite = Color(red: 0.98, green: 0.99, blue: 1.0)
    static let textSecondary = Color(red: 0.72, green: 0.78, blue: 0.86)
    static let textMuted = Color(red: 0.48, green: 0.54, blue: 0.63)

    // Apple Royal Blue (Zero Neon!)
    static let appleBlue = Color(red: 0.05, green: 0.45, blue: 0.96)
    static let appleBlueHover = Color(red: 0.12, green: 0.52, blue: 1.0)
    static let appleBlueSoft = Color(red: 0.05, green: 0.45, blue: 0.96).opacity(0.16)
    static let appleBlueBorder = Color(red: 0.05, green: 0.45, blue: 0.96).opacity(0.35)

    // Severity & Safety Colors
    static let successBlue = Color(red: 0.15, green: 0.60, blue: 0.95)
    static let alertRed = Color(red: 0.92, green: 0.25, blue: 0.28)
    static let warningOrange = Color(red: 0.95, green: 0.55, blue: 0.15)
}

