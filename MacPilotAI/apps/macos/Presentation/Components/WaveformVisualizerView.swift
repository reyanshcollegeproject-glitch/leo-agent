import SwiftUI

struct WaveformVisualizerView: View {
    let level: Float
    let isActive: Bool
    let barCount: Int = 7

    var body: some View {
        HStack(spacing: 3) {
            ForEach(0..<barCount, id: \.self) { index in
                RoundedRectangle(cornerRadius: 2)
                    .fill(isActive ? barColor(for: index) : Theme.textMuted.opacity(0.35))
                    .frame(width: 3, height: barHeight(for: index))
                    .animation(
                        .spring(response: 0.18, dampingFraction: 0.55),
                        value: level
                    )
            }
        }
        .frame(height: 24)
    }

    private func barHeight(for index: Int) -> CGFloat {
        if !isActive {
            return 4
        }
        // Varied multiplier across bars to create natural audio wave shape
        let multipliers: [CGFloat] = [0.4, 0.7, 1.0, 1.2, 0.9, 0.6, 0.35]
        let baseMultiplier = multipliers[index % multipliers.count]
        let scaled = CGFloat(level) * 22.0 * baseMultiplier
        return max(5.0, min(scaled, 24.0))
    }

    private func barColor(for index: Int) -> Color {
        if index == 3 || index == 2 || index == 4 {
            return Theme.appleBlue
        } else {
            return Theme.textWhite.opacity(0.85)
        }
    }
}

