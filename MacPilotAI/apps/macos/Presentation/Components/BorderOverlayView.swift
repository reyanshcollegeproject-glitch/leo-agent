import SwiftUI
import AppKit

struct BorderOverlayView: View {
    @State private var isPulsing: Bool = false

    var body: some View {
        ZStack {
            // Top Glow
            VStack {
                LinearGradient(
                    gradient: Gradient(colors: [
                        Theme.appleBlue.opacity(0.85),
                        Theme.appleBlueSoft.opacity(0.45),
                        Color.clear
                    ]),
                    startPoint: .top,
                    endPoint: .bottom
                )
                .frame(height: 32)
                Spacer()
            }

            // Bottom Glow
            VStack {
                Spacer()
                LinearGradient(
                    gradient: Gradient(colors: [
                        Color.clear,
                        Theme.appleBlueSoft.opacity(0.45),
                        Theme.appleBlue.opacity(0.85)
                    ]),
                    startPoint: .top,
                    endPoint: .bottom
                )
                .frame(height: 32)
            }

            // Left Glow
            HStack {
                LinearGradient(
                    gradient: Gradient(colors: [
                        Theme.appleBlue.opacity(0.85),
                        Theme.appleBlueSoft.opacity(0.45),
                        Color.clear
                    ]),
                    startPoint: .leading,
                    endPoint: .trailing
                )
                .frame(width: 32)
                Spacer()
            }

            // Right Glow
            HStack {
                Spacer()
                LinearGradient(
                    gradient: Gradient(colors: [
                        Color.clear,
                        Theme.appleBlueSoft.opacity(0.45),
                        Theme.appleBlue.opacity(0.85)
                    ]),
                    startPoint: .leading,
                    endPoint: .trailing
                )
                .frame(width: 32)
            }

            // Elegant perimeter neon rim with smooth glow shadow
            RoundedRectangle(cornerRadius: 10)
                .stroke(
                    LinearGradient(
                        colors: [Theme.appleBlue, Theme.appleBlueSoft, Color.cyan.opacity(0.8), Theme.appleBlue],
                        startPoint: .topLeading,
                        endPoint: .bottomTrailing
                    ),
                    lineWidth: 3
                )
                .shadow(color: Theme.appleBlue.opacity(isPulsing ? 0.9 : 0.5), radius: 12)
        }
        .ignoresSafeArea()
        .opacity(isPulsing ? 1.0 : 0.7)
        .animation(.easeInOut(duration: 1.2).repeatForever(autoreverses: true), value: isPulsing)
        .onAppear {
            isPulsing = true
        }
    }
}

class BorderOverlayManager {
    static let shared = BorderOverlayManager()
    private var window: NSWindow?

    private init() {}

    func show() {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            if self.window == nil {
                guard let screen = NSScreen.main else { return }
                let overlay = NSWindow(
                    contentRect: screen.frame,
                    styleMask: [.borderless],
                    backing: .buffered,
                    defer: false
                )
                overlay.isOpaque = false
                overlay.backgroundColor = .clear
                overlay.ignoresMouseEvents = true
                overlay.level = .floating
                overlay.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
                overlay.contentView = NSHostingView(rootView: BorderOverlayView())
                self.window = overlay
            }
            self.window?.orderFront(nil)
        }
    }

    func hide() {
        DispatchQueue.main.async { [weak self] in
            self?.window?.orderOut(nil)
        }
    }
}

