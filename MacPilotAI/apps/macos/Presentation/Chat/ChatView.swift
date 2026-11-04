import SwiftUI

struct ChatView: View {
    @EnvironmentObject var env: AppEnvironment
    @State private var pulseOrb: Bool = false

    var body: some View {
        HStack(spacing: 11) {
            // MARK: - 1. Pulse Beacon & Brand
            HStack(spacing: 8) {
                ZStack {
                    Circle()
                        .fill(beaconGlowColor.opacity(pulseOrb ? 0.35 : 0.15))
                        .frame(width: 28, height: 28)
                        .scaleEffect(pulseOrb ? 1.15 : 0.95)
                        .animation(.easeInOut(duration: 1.2).repeatForever(autoreverses: true), value: pulseOrb)

                    Circle()
                        .fill(beaconColor)
                        .frame(width: 9, height: 9)
                }

                VStack(alignment: .leading, spacing: 1) {
                    Text("L.E.O.")
                        .font(.system(size: 12, weight: .heavy, design: .rounded))
                        .foregroundColor(Theme.textWhite)
                    Text("Meta AI Brain")
                        .font(.system(size: 9, weight: .semibold))
                        .foregroundColor(Theme.appleBlue)
                }
                .fixedSize()
            }
            .onAppear {
                pulseOrb = true
            }

            // MARK: - 2. Real-Time Audio Waveform Visualizer
            WaveformVisualizerView(
                level: env.voiceManager.audioLevel,
                isActive: env.voiceManager.isListening || env.voiceManager.isSpeaking
            )
            .frame(width: 32)

            // MARK: - 3. Dynamic Center Status & Live Transcript Pill
            centerStatusPill

            Spacer(minLength: 4)

            // MARK: - 4. Pending Approval Banner (Inline)
            if let approval = env.pendingApproval {
                HStack(spacing: 6) {
                    Text("Allow \(approval.command.prefix(15))...?")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(.yellow)

                    Button("Allow") {
                        Task {
                            try? await env.agentClient.resolveApproval(id: approval.id, approved: true)
                            env.pendingApproval = nil
                        }
                    }
                    .font(.system(size: 9, weight: .heavy))
                    .foregroundColor(.white)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 3)
                    .background(Color.green)
                    .cornerRadius(6)

                    Button("Deny") {
                        Task {
                            try? await env.agentClient.resolveApproval(id: approval.id, approved: false)
                            env.pendingApproval = nil
                        }
                    }
                    .font(.system(size: 9, weight: .heavy))
                    .foregroundColor(.white)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 3)
                    .background(Color.red)
                    .cornerRadius(6)
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(Color.yellow.opacity(0.15))
                .cornerRadius(10)
            } else {
                // MARK: - 5. Mode Selector Pill (🖥️ Computer vs ⚡ Background)
                HStack(spacing: 2) {
                    ForEach(AgentMode.allCases) { mode in
                        Button(action: {
                            withAnimation(.easeInOut(duration: 0.15)) {
                                env.chatViewModel.selectedMode = mode
                            }
                            if mode == .computer {
                                AppDelegate.shared?.showComputerWindow()
                            }
                        }) {
                            HStack(spacing: 3) {
                                Image(systemName: mode == .computer ? "display" : "bolt.fill")
                                    .font(.system(size: 9, weight: .bold))
                                Text(mode == .computer ? "Computer" : "Background")
                                    .font(.system(size: 9, weight: .bold))
                                    .fixedSize()
                            }
                            .foregroundColor(env.chatViewModel.selectedMode == mode ? .white : Theme.textMuted)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background(env.chatViewModel.selectedMode == mode ? Theme.appleBlue : Color.clear)
                            .cornerRadius(8)
                        }
                        .buttonStyle(.plain)
                    }
                }
                .padding(2)
                .background(Theme.cardCharcoal)
                .cornerRadius(10)
                .overlay(
                    RoundedRectangle(cornerRadius: 10)
                        .stroke(Theme.borderSubtle, lineWidth: 1)
                )
            }

            // MARK: - 6. Emergency Stop Kill Switch Button (active during execution)
            if env.chatViewModel.isExecuting || env.voiceManager.isSpeaking {
                Button(action: {
                    env.emergencyStop()
                }) {
                    HStack(spacing: 4) {
                        Image(systemName: "stop.fill")
                            .font(.system(size: 8, weight: .heavy))
                        Text("Stop")
                            .font(.system(size: 10, weight: .bold))
                    }
                    .foregroundColor(.white)
                    .padding(.horizontal, 9)
                    .padding(.vertical, 5)
                    .background(Color.red.opacity(0.9))
                    .cornerRadius(10)
                }
                .buttonStyle(.plain)
                .help("Emergency Stop (⌘⇧S or Esc)")
                .transition(.scale.combined(with: .opacity))
            }

            // MARK: - 7. Real-Time Mic Toggle Button
            Button(action: {
                withAnimation(.easeInOut(duration: 0.2)) {
                    env.voiceManager.toggleListening()
                }
            }) {
                HStack(spacing: 5) {
                    Image(systemName: env.voiceManager.isListening ? "mic.fill" : "mic.slash.fill")
                        .font(.system(size: 11, weight: .bold))
                        .foregroundColor(env.voiceManager.isListening ? .white : Theme.textMuted)

                    Text(env.voiceManager.isListening ? "Listening" : "Muted")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(env.voiceManager.isListening ? .white : Theme.textMuted)
                        .fixedSize()
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 5)
                .background(env.voiceManager.isListening ? Theme.appleBlue : Theme.cardCharcoal)
                .cornerRadius(12)
            }
            .buttonStyle(.plain)
            .help("Toggle Real-Time Voice (or Double-Tap Fn)")

            // MARK: - 7b. Real-Time Computer Vision Eye Button
            Button(action: {
                Task {
                    _ = await env.screenVisionManager.inspectScreenAndExplain(userPrompt: "Look at my screen and tell me what you see.")
                }
            }) {
                HStack(spacing: 4) {
                    Image(systemName: env.screenVisionManager.isAnalyzing ? "eye.trianglebadge.exclamationmark.fill" : "eye.fill")
                        .font(.system(size: 11, weight: .bold))
                        .foregroundColor(env.screenVisionManager.isAnalyzing ? Theme.appleBlue : Theme.textWhite)
                    Text("Vision")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(env.screenVisionManager.isAnalyzing ? Theme.appleBlue : Theme.textWhite)
                        .fixedSize()
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 5)
                .background(env.screenVisionManager.isAnalyzing ? Theme.cardCharcoal : Theme.cardCharcoal.opacity(0.8))
                .cornerRadius(12)
                .overlay(
                    RoundedRectangle(cornerRadius: 12)
                        .stroke(env.screenVisionManager.isAnalyzing ? Theme.appleBlue : Theme.borderSubtle, lineWidth: 1)
                )
            }
            .buttonStyle(.plain)
            .help("See Screen & Talk in Real Time (⌘⇧H)")

            // MARK: - 8. Voice Speaker Output Toggle
            Button(action: {
                env.voiceManager.isVoiceOutputEnabled.toggle()
                if !env.voiceManager.isVoiceOutputEnabled {
                    env.voiceManager.stopSpeaking()
                }
            }) {
                Image(systemName: env.voiceManager.isVoiceOutputEnabled ? "speaker.wave.2.fill" : "speaker.slash.fill")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundColor(env.voiceManager.isVoiceOutputEnabled ? Theme.textWhite : Theme.textMuted)
                    .frame(width: 26, height: 26)
                    .background(Theme.cardCharcoal)
                    .clipShape(Circle())
            }
            .buttonStyle(.plain)
            .help("Mute / Unmute Real-Time Agent Voice")
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 8)
        .frame(height: 52)
        .background(
            RoundedRectangle(cornerRadius: 26, style: .continuous)
                .fill(Theme.bgBlack.opacity(0.96))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 26, style: .continuous)
                .stroke(capsuleBorderColor, lineWidth: 1.2)
        )
        .shadow(color: Color.black.opacity(0.7), radius: 18, x: 0, y: 8)
        .padding(2)
    }

    private var centerStatusPill: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(statusDotColor)
                .frame(width: 6, height: 6)

            Text(statusDisplayString)
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(Theme.textWhite)
                .lineLimit(1)
                .truncationMode(.tail)
                .frame(maxWidth: 190, alignment: .leading)
        }
        .padding(.horizontal, 9)
        .padding(.vertical, 4.5)
        .background(Theme.cardCharcoal)
        .cornerRadius(10)
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .stroke(Theme.borderSubtle, lineWidth: 1)
        )
    }

    private var statusDisplayString: String {
        if !env.voiceManager.liveTranscript.isEmpty {
            return "\"\(env.voiceManager.liveTranscript)\""
        }
        if env.voiceManager.isSpeaking {
            return "Speaking..."
        }
        if env.chatViewModel.isExecuting {
            return env.chatViewModel.currentStatus
        }
        if env.voiceManager.isListening {
            return "Listening..."
        }
        return "Ready"
    }

    private var statusDotColor: Color {
        if !env.voiceManager.liveTranscript.isEmpty {
            return Theme.appleBlue
        }
        if env.voiceManager.isSpeaking {
            return Color.cyan
        }
        if env.chatViewModel.isExecuting {
            return Color.orange
        }
        if env.voiceManager.isListening {
            return Color.green
        }
        return Color.gray
    }

    private var beaconColor: Color {
        if env.chatViewModel.isExecuting {
            return Color.orange
        }
        if env.voiceManager.isSpeaking {
            return Color.cyan
        }
        if env.voiceManager.isListening {
            return Theme.appleBlue
        }
        return Color.gray
    }

    private var beaconGlowColor: Color {
        if env.chatViewModel.isExecuting {
            return Color.orange
        }
        if env.voiceManager.isSpeaking {
            return Color.cyan
        }
        if env.voiceManager.isListening {
            return Theme.appleBlue
        }
        return Theme.cardCharcoal
    }

    private var capsuleBorderColor: Color {
        if env.chatViewModel.isExecuting {
            return Color.orange.opacity(0.8)
        }
        if env.voiceManager.isSpeaking {
            return Color.cyan.opacity(0.8)
        }
        if env.voiceManager.isListening {
            return Theme.appleBlue.opacity(0.7)
        }
        return Theme.borderSubtle
    }
}
