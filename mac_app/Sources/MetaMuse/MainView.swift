import SwiftUI

struct MainView: View {
    @StateObject private var bridge = AgentBridge()
    @State private var inputText: String = ""
    @State private var isPinnedOnTop: Bool = true

    let suggestions = [
        "open chrome and play weekend song",
        "open desktop in finder",
        "calculate 512 * 128",
        "mute volume",
        "open terminal"
    ]

    var body: some View {
        ZStack {
            Theme.bgBlack.ignoresSafeArea()

            VStack(spacing: 0) {
                // 1. Header Bar
                headerBar

                // 2. Permission Banner (if macOS Screen Recording is needed)
                if !bridge.hasScreenPermission {
                    permissionBanner
                }

                // 3. Conversational Message Stream
                chatScrollView

                // 4. Quick Suggestion Pills
                suggestionBar

                // 5. Bottom Interactive Input Section
                inputSection
            }
        }
        .frame(minWidth: 580, idealWidth: 640, minHeight: 640, idealHeight: 720)
        .onAppear {
            setWindowTopmost(isPinnedOnTop)
        }
    }

    // MARK: - Header Bar
    private var headerBar: some View {
        HStack(spacing: 12) {
            // App Icon & Title
            HStack(spacing: 8) {
                ZStack {
                    Circle()
                        .fill(Theme.appleBlueSoft)
                        .frame(width: 32, height: 32)
                    Image(systemName: "bolt.fill")
                        .font(.system(size: 14, weight: .bold))
                        .foregroundColor(Theme.appleBlue)
                }

                VStack(alignment: .leading, spacing: 1) {
                    Text("META MUSE")
                        .font(.system(size: 13, weight: .heavy, design: .rounded))
                        .foregroundColor(Theme.textWhite)
                    Text("24/7 Autonomous macOS Desktop Agent")
                        .font(.system(size: 10, weight: .medium))
                        .foregroundColor(Theme.textSecondary)
                }
            }

            Spacer()

            // State Badge
            HStack(spacing: 6) {
                Circle()
                    .fill(stateBadgeColor)
                    .frame(width: 7, height: 7)
                Text(bridge.state.rawValue)
                    .font(.system(size: 10, weight: .bold))
                    .foregroundColor(Theme.textWhite)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 5)
            .background(Theme.cardCharcoal)
            .cornerRadius(12)
            .overlay(
                RoundedRectangle(cornerRadius: 12)
                    .stroke(Theme.borderGraphite, lineWidth: 1)
            )

            // Voice Feedback Toggle
            Button(action: {
                bridge.voiceFeedbackEnabled.toggle()
            }) {
                Image(systemName: bridge.voiceFeedbackEnabled ? "speaker.wave.2.fill" : "speaker.slash.fill")
                    .font(.system(size: 13, weight: .medium))
                    .foregroundColor(bridge.voiceFeedbackEnabled ? Theme.appleBlue : Theme.textMuted)
                    .padding(7)
                    .background(Theme.cardCharcoal)
                    .cornerRadius(8)
            }
            .buttonStyle(.plain)
            .help("Toggle Voice Audio Feedback")

            // Pin on Top Toggle
            Button(action: {
                isPinnedOnTop.toggle()
                setWindowTopmost(isPinnedOnTop)
            }) {
                Image(systemName: isPinnedOnTop ? "pin.fill" : "pin")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundColor(isPinnedOnTop ? Theme.appleBlue : Theme.textMuted)
                    .padding(7)
                    .background(Theme.cardCharcoal)
                    .cornerRadius(8)
            }
            .buttonStyle(.plain)
            .help("Pin window on top")
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .background(Theme.cardCharcoal.opacity(0.85))
        .overlay(
            Rectangle()
                .frame(height: 1)
                .foregroundColor(Theme.borderSubtle),
            alignment: .bottom
        )
    }

    private var stateBadgeColor: Color {
        switch bridge.state {
        case .idle: return Theme.appleBlue
        case .running: return Theme.appleBlueHover
        case .listening: return Theme.textWhite
        case .completed: return Theme.successBlue
        case .paused: return Theme.textSecondary
        case .stopped: return Theme.alertRed
        }
    }

    // MARK: - Permission Banner
    private var permissionBanner: some View {
        HStack(spacing: 10) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundColor(Theme.appleBlue)
                .font(.system(size: 13))

            Text("macOS Screen Recording permission enables vision. Native commands remain active.")
                .font(.system(size: 11, weight: .medium))
                .foregroundColor(Theme.textSecondary)

            Spacer()

            Button("Grant Access") {
                bridge.openScreenRecordingSettings()
            }
            .font(.system(size: 11, weight: .bold))
            .foregroundColor(Theme.textWhite)
            .padding(.horizontal, 10)
            .padding(.vertical, 4)
            .background(Theme.appleBlue)
            .cornerRadius(6)
            .buttonStyle(.plain)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Theme.cardCharcoal)
        .overlay(
            Rectangle()
                .frame(height: 1)
                .foregroundColor(Theme.appleBlueBorder),
            alignment: .bottom
        )
    }

    // MARK: - Chat Stream
    private var chatScrollView: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(spacing: 14) {
                    ForEach(bridge.messages) { message in
                        MessageRow(message: message)
                            .id(message.id)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 14)
            }
            .onChange(of: bridge.messages.count) { _ in
                if let lastId = bridge.messages.last?.id {
                    withAnimation {
                        proxy.scrollTo(lastId, anchor: .bottom)
                    }
                }
            }
        }
    }

    // MARK: - Suggestion Pills
    private var suggestionBar: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                ForEach(suggestions, id: \.self) { suggestion in
                    Button(action: {
                        bridge.executeGoal(suggestion)
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: "sparkle")
                                .font(.system(size: 9))
                                .foregroundColor(Theme.appleBlue)
                            Text(suggestion)
                                .font(.system(size: 11, weight: .medium))
                                .foregroundColor(Theme.textWhite)
                        }
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background(Theme.cardCharcoal)
                        .cornerRadius(12)
                        .overlay(
                            RoundedRectangle(cornerRadius: 12)
                                .stroke(Theme.borderGraphite, lineWidth: 1)
                        )
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 6)
        }
    }

    // MARK: - Input Section
    private var inputSection: some View {
        VStack(spacing: 0) {
            Divider()
                .background(Theme.borderSubtle)

            HStack(spacing: 10) {
                // Voice / Mic Button
                Button(action: {
                    bridge.startVoiceListening()
                }) {
                    ZStack {
                        Circle()
                            .fill(bridge.isListeningVoice ? Theme.alertRed : Theme.cardCharcoal)
                            .frame(width: 36, height: 36)
                            .overlay(
                                Circle()
                                    .stroke(bridge.isListeningVoice ? Theme.alertRed : Theme.borderGraphite, lineWidth: 1)
                            )

                        Image(systemName: bridge.isListeningVoice ? "waveform" : "mic.fill")
                            .font(.system(size: 14))
                            .foregroundColor(bridge.isListeningVoice ? .white : Theme.appleBlue)
                    }
                }
                .buttonStyle(.plain)
                .help("Talk with Voice (Microphone)")

                // Main Text Input Field
                TextField("Ask anything or give a desktop goal (Press Return)...", text: $inputText)
                    .textFieldStyle(.plain)
                    .font(.system(size: 13))
                    .foregroundColor(Theme.textWhite)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 9)
                    .background(Theme.cardCharcoal)
                    .cornerRadius(8)
                    .overlay(
                        RoundedRectangle(cornerRadius: 8)
                            .stroke(Theme.borderGraphite, lineWidth: 1)
                    )
                    .onSubmit {
                        submitCurrentGoal()
                    }

                // Emergency Stop Button (when running)
                if bridge.state == .running {
                    Button(action: {
                        bridge.emergencyStop()
                    }) {
                        Image(systemName: "stop.circle.fill")
                            .font(.system(size: 24))
                            .foregroundColor(Theme.alertRed)
                    }
                    .buttonStyle(.plain)
                    .help("Emergency Stop")
                }

                // Send Button
                Button(action: {
                    submitCurrentGoal()
                }) {
                    ZStack {
                        Circle()
                            .fill(Theme.appleBlue)
                            .frame(width: 36, height: 36)

                        Image(systemName: "arrow.up")
                            .font(.system(size: 14, weight: .bold))
                            .foregroundColor(.white)
                    }
                }
                .buttonStyle(.plain)
                .disabled(inputText.trimmingCharacters(in: .whitespaces).isEmpty)
                .opacity(inputText.trimmingCharacters(in: .whitespaces).isEmpty ? 0.4 : 1.0)
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            .background(Theme.bgBlack)
        }
    }

    private func submitCurrentGoal() {
        let goal = inputText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !goal.isEmpty else { return }
        inputText = ""
        bridge.executeGoal(goal)
    }

    private func setWindowTopmost(_ top: Bool) {
        if let window = NSApplication.shared.windows.first {
            window.level = top ? .floating : .normal
        }
    }
}

// MARK: - Message Row Component
struct MessageRow: View {
    let message: ChatMessage

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            if message.sender == .user {
                Spacer(minLength: 40)
                userBubble
            } else {
                assistantBubble
                Spacer(minLength: 40)
            }
        }
    }

    private var userBubble: some View {
        VStack(alignment: .trailing, spacing: 4) {
            Text(message.text)
                .font(.system(size: 13, weight: .medium))
                .foregroundColor(Theme.textWhite)
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
                .background(Theme.appleBlue)
                .cornerRadius(14)
        }
    }

    private var assistantBubble: some View {
        VStack(alignment: .leading, spacing: 8) {
            // Header tag
            HStack(spacing: 6) {
                Image(systemName: "bolt.circle.fill")
                    .foregroundColor(Theme.appleBlue)
                    .font(.system(size: 13))
                Text("Meta Muse")
                    .font(.system(size: 11, weight: .bold))
                    .foregroundColor(Theme.textSecondary)

                if message.isExecuting {
                    ProgressView()
                        .scaleEffect(0.5)
                        .frame(width: 14, height: 14)
                }
            }

            // Thought / Answer text
            Text(message.text)
                .font(.system(size: 13))
                .foregroundColor(Theme.textWhite)
                .lineSpacing(3)

            // Step Action Badges
            if !message.steps.isEmpty {
                VStack(alignment: .leading, spacing: 6) {
                    ForEach(message.steps) { step in
                        HStack(alignment: .center, spacing: 8) {
                            Text("Step #\(step.id)")
                                .font(.system(size: 10, weight: .bold))
                                .foregroundColor(Theme.appleBlue)
                                .padding(.horizontal, 6)
                                .padding(.vertical, 2)
                                .background(Theme.appleBlueSoft)
                                .cornerRadius(4)

                            Text(step.action)
                                .font(.system(size: 10, weight: .heavy))
                                .foregroundColor(Theme.textWhite)

                            Text(step.message)
                                .font(.system(size: 10))
                                .foregroundColor(Theme.textSecondary)
                                .lineLimit(1)
                                .truncationMode(.tail)

                            Spacer()

                            Image(systemName: step.success ? "checkmark.circle.fill" : "xmark.circle.fill")
                                .font(.system(size: 11))
                                .foregroundColor(step.success ? Theme.appleBlue : Theme.alertRed)
                        }
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(Theme.bgBlack)
                        .cornerRadius(6)
                        .overlay(
                            RoundedRectangle(cornerRadius: 6)
                                .stroke(Theme.borderSubtle, lineWidth: 1)
                        )
                    }
                }
                .padding(.top, 4)
            }
        }
        .padding(14)
        .background(Theme.cardCharcoal)
        .cornerRadius(12)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Theme.borderGraphite, lineWidth: 1)
        )
    }
}
