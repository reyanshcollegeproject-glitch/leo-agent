import SwiftUI

struct WaveformView: View {
    var audioLevel: Float
    var isRecording: Bool
    var isExecuting: Bool

    // Multipliers for the 7 frequency bands (symmetric bell curve)
    private let multipliers: [Float] = [0.5, 0.8, 1.2, 1.5, 1.2, 0.8, 0.5]

    var body: some View {
        HStack(spacing: 4) {
            ForEach(0..<7, id: \.self) { i in
                barView(index: i)
            }
        }
        .frame(height: 32)
    }

    @ViewBuilder
    private func barView(index: Int) -> some View {
        let baseHeight: CGFloat = isRecording ? 6.0 : (isExecuting ? 8.0 : 4.0)
        let maxHeight: CGFloat = 28.0
        let mult = multipliers[index]
        let scaled = isRecording
            ? min(max(CGFloat(audioLevel * mult), 0.0), 1.0) * (maxHeight - baseHeight)
            : (isExecuting ? CGFloat(sin(Double(index) * 0.8 + Date().timeIntervalSince1970 * 4) * 6 + 6) : 0)

        RoundedRectangle(cornerRadius: 2)
            .fill(barColor)
            .frame(width: 3.5, height: baseHeight + scaled)
            .animation(.interactiveSpring(response: 0.15, dampingFraction: 0.7), value: audioLevel)
    }

    private var barColor: Color {
        if isRecording {
            return Theme.appleBlue
        } else if isExecuting {
            return Theme.appleBlueHover
        } else {
            return Theme.textMuted.opacity(0.4)
        }
    }
}

