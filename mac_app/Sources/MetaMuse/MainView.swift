import SwiftUI
import AppKit

struct MainView: View {
    @StateObject private var bridge = AgentBridge()
    @State private var inputText: String = ""
    @State private var isExpanded: Bool = true
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
                // 1. Flow Voice Capsule Bar (Always Visible)
                capsuleBar

                if isExpanded {
                    // 2. Permission Banner (if needed)
                    if !bridge.hasScreenPermission {
                        permissionBanner
                    }

                    // 3. Conversational Message Stream
                    chatScrollView

                    // 4. Quick Suggestion Pills
                    suggestionBar

                    // 5. Bottom Interactive Text Input Section
                    inputSection
                }
            }
        }
        .frame(
            minWidth: isExpanded ? 580 : 480,
            idealWidth: isExpanded ? 640 : 480,
            maxWidth: isExpanded ? 720 : 480,
            minHeight: isExpanded ? 600 : 58,
            idealHeight: isExpanded ? 680 : 58,
            maxHeight: isExpanded ? 800 : 58
        )
        .clipShape(RoundedRectangle(cornerRadius: isExpanded ? 16 : 29))
        .overlay(
            RoundedRectangle(cornerRadius: isExpanded ? 16 : 29)
                .stroke(Theme.borderGraphite, lineWidth: 1)
        )
        .animation(.spring(response: 0.35, dampingFraction: 0.8), value: isExpanded)
        .onAppear {
            setWindowTopmost(isPinnedOnTop)
        }
        .onReceive(NotificationCenter.default.publisher(for: NSNotification.Name("TriggerVoiceDictation"))) { _ in
            bridge.toggleVoiceRecording()
        }
    }

    // MARK: - Flow Voice Capsule Bar
    private var capsuleBar: some View {
        HStack(spacing: 12) {
            // 1. Circular Mic Button (Flow Voice Trigger)
            Button(action: {
                bridge.toggleVoiceRecording()
            }) {
                ZStack {
                    Circle()
                        .fill(bridge.isRecordingVoice ? Theme.textWhite : Theme.appleBlue)
                        .frame(width: 40, height: 40)
                        .shadow(color: Theme.appleBlue.opacity(0.3), radius: 6, x: 0, y: 2)

                    Image(systemName: bridge.isRecordingVoice ? "stop.fill" : "mic.fill")
                        .font(.system(size: 15, weight: .bold))
                        .foregroundColor(bridge.isRecordingVoice ? Theme.appleBlue : .white)
                }
            }
            .buttonStyle(.plain)
            .help("Click or Press [⌥ Space] to Speak Instructions")

            // 2. Status & Live Speech Label
            VStack(alignment: .leading, spacing: 2) {
                Text(bridge.statusText)
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundColor(Theme.textWhite)
                    .lineLimit(1)
                    .truncationMode(.tail)

                HStack(spacing: 6) {
                    Circle()
                        .fill(stateColor)
                        .frame(width: 6, height: 6)

                    Text(bridge.state.rawValue)
                        .font(.system(size: 9, weight: .bold))
                        .foregroundColor(Theme.textSecondary)

                    Text("• Meta Muse")
                        .font(.system(size: 9, weight: .medium))
                        .foregroundColor(Theme.textMuted)
                }
            }

            Spacer()

            // 3. 7-Bar Dynamic RMS Audio Waveform
            WaveformView(
                audioLevel: bridge.audioLevel,
                isRecording: bridge.isRecordingVoice,
                isExecuting: bridge.state == .running
            )
            .frame(width: 60)

            // 4. Voice Audio Speaker Toggle
            Button(action: {
                bridge.voiceFeedbackEnabled.toggle()
            }) {
                Image(systemName: bridge.voiceFeedbackEnabled ? "speaker.wave.2.fill" : "speaker.slash.fill")
                    .font(.system(size: 12))
                    .foregroundColor(bridge.voiceFeedbackEnabled ? Theme.appleBlue : Theme.textMuted)
                    .padding(6)
                    .background(Theme.cardCharcoal)
                    .cornerRadius(8)
            }
            .buttonStyle(.plain)
            .help("Toggle Voice Audio Speech")

            // 5. Expand / Collapse Drawer Button
            Button(action: {
                withAnimation {
                    isExpanded.toggle()
                    adjustWindowSize(expanded: isExpanded)
                }
            }) {
                ZStack {
                    Circle()
                        .fill(Theme.cardCharcoal)
                        .frame(width: 28, height: 28)
                        .overlay(
                            Circle()
                                .stroke(Theme.borderGraphite, lineWidth: 1)
                        )

                    Image(systemName: isExpanded ? "chevron.down" : "chevron.up")
                        .font(.system(size: 11, weight: .bold))
                        .foregroundColor(Theme.appleBlue)
                }
            }
            .buttonStyle(.plain)
            .help(isExpanded ? "Collapse to Floating Capsule" : "Expand Full Agent View")
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        .background(Theme.cardCharcoal.opacity(0.92))
    }

    private var stateColor: Color {
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
                .font(.system(size: 12))

            Text("macOS Screen Recording permission enables vision. Direct commands active.")
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
        .padding(.vertical, 7)
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
                LazyVStack(spacing: 12) {
                    ForEach(bridge.messages) { message in
                        MessageRow(message: message)
                            .id(message.id)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 12)
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

    // MARK: - Bottom Input Section
    private var inputSection: some View {
        VStack(spacing: 0) {
            Divider()
                .background(Theme.borderSubtle)

            HStack(spacing: 10) {
                TextField("Type command or goal (Press Return)...", text: $inputText)
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
            .padding(.vertical, 10)
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

    private func adjustWindowSize(expanded: Bool) {
        if let window = NSApplication.shared.windows.first {
            var frame = window.frame
            let targetHeight: CGFloat = expanded ? 680 : 58
            let targetWidth: CGFloat = expanded ? 640 : 480
            frame.origin.y += (frame.size.height - targetHeight)
            frame.size.height = targetHeight
            frame.size.width = targetWidth
            window.setFrame(frame, display: true, animate: true)
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
        Text(message.text)
            .font(.system(size: 13, weight: .medium))
            .foregroundColor(Theme.textWhite)
            .padding(.horizontal, 14)
            .padding(.vertical, 9)
            .background(Theme.appleBlue)
            .cornerRadius(14)
    }

    private var assistantBubble: some View {
        VStack(alignment: .leading, spacing: 8) {
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

            Text(message.text)
                .font(.system(size: 13))
                .foregroundColor(Theme.textWhite)
                .lineSpacing(3)

            if !message.steps.isEmpty {
                VStack(alignment: .leading, spacing: 5) {
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
        .padding(13)
        .background(Theme.cardCharcoal)
        .cornerRadius(12)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Theme.borderGraphite, lineWidth: 1)
        )
    }
}
