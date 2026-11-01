import Foundation
import SwiftUI
import AppKit

@MainActor
class AgentBridge: ObservableObject {
    @Published var messages: [ChatMessage] = []
    @Published var state: AppAgentState = .idle
    @Published var hasScreenPermission: Bool = true
    @Published var voiceFeedbackEnabled: Bool = true

    // Voice State from Flow Widget
    @Published var isRecordingVoice: Bool = false
    @Published var audioLevel: Float = 0.0
    @Published var liveTranscript: String = ""
    @Published var statusText: String = "Click Mic or ⌥Space to Talk"

    private let speechManager = SpeechManager()
    private let polisher = AIPolisher()
    private var activeProcess: Process?
    private let workspacePath: String

    init() {
        self.workspacePath = "/Users/reyansh/Desktop/contribute/auto agent"
        checkPermissions()
        setupSpeechCallbacks()
        seedWelcomeMessage()
    }

    func checkPermissions() {
        self.hasScreenPermission = CGPreflightScreenCaptureAccess()
    }

    func openScreenRecordingSettings() {
        CGRequestScreenCaptureAccess()
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture") {
            NSWorkspace.shared.open(url)
        }
    }

    private func setupSpeechCallbacks() {
        speechManager.onAudioLevelChange = { [weak self] level in
            Task { @MainActor in
                self?.audioLevel = level
            }
        }

        speechManager.onTranscriptUpdate = { [weak self] text in
            Task { @MainActor in
                self?.liveTranscript = text
                if !text.isEmpty {
                    self?.statusText = text
                }
            }
        }
    }

    private func seedWelcomeMessage() {
        let welcome = ChatMessage(
            sender: .assistant,
            text: "Hello! I am Meta Muse, your 24/7 autonomous macOS assistant. You can chat or press [⌥ Space] to speak instructions like 'open chrome and play weekend song'.",
            timestamp: Date(),
            steps: []
        )
        messages.append(welcome)
    }

    // MARK: - Voice Toggle (Flow Widget Capsule Integration)
    func toggleVoiceRecording() {
        if isRecordingVoice {
            stopVoiceAndExecute()
        } else {
            startVoiceRecording()
        }
    }

    private func startVoiceRecording() {
        guard !isRecordingVoice else { return }
        self.isRecordingVoice = true
        self.state = .listening
        self.statusText = "Listening (speak now)..."
        self.liveTranscript = ""

        speechManager.startRecording { [weak self] success, error in
            Task { @MainActor in
                if !success {
                    self?.isRecordingVoice = false
                    self?.state = .idle
                    self?.statusText = error ?? "Microphone access needed"
                }
            }
        }
    }

    private func stopVoiceAndExecute() {
        self.isRecordingVoice = false
        self.audioLevel = 0.0

        let raw = speechManager.stopRecording()
        let clean = polisher.polish(rawTranscript: raw)

        if clean.isEmpty {
            self.state = .idle
            self.statusText = "No speech detected"
            Task {
                try? await Task.sleep(nanoseconds: 1_500_000_000)
                if self.state == .idle {
                    self.statusText = "Click Mic or ⌥Space to Talk"
                }
            }
            return
        }

        self.statusText = "✨ \(clean)"
        executeGoal(clean)
    }

    // MARK: - Execution Engine
    func executeGoal(_ goal: String) {
        let trimmed = goal.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }

        // 1. Add user message
        let userMsg = ChatMessage(sender: .user, text: trimmed)
        messages.append(userMsg)

        // 2. Add assistant placeholder
        let assistantMsgId = UUID()
        let assistantMsg = ChatMessage(
            id: assistantMsgId,
            sender: .assistant,
            text: "Analyzing goal and executing across macOS...",
            steps: [],
            isExecuting: true
        )
        messages.append(assistantMsg)
        self.state = .running
        self.statusText = "⚡ Executing: \(trimmed)..."

        let pythonPath = "\(workspacePath)/venv/bin/python"
        guard FileManager.default.fileExists(atPath: pythonPath) else {
            updateAssistantMessage(id: assistantMsgId, text: "Error: Virtual environment not found.", isExecuting: false)
            self.state = .idle
            self.statusText = "Execution failed"
            return
        }

        // 3. Launch process with --json-stream
        Task.detached(priority: .userInitiated) { [weak self, workspacePath] in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: pythonPath)
            process.currentDirectoryURL = URL(fileURLWithPath: workspacePath)
            process.arguments = ["-m", "muse_agent.cli.main", "run", trimmed, "--json-stream"]

            var env = ProcessInfo.processInfo.environment
            env["PYTHONUNBUFFERED"] = "1"
            process.environment = env

            let stdoutPipe = Pipe()
            process.standardOutput = stdoutPipe
            process.standardError = Pipe()

            await MainActor.run {
                self?.activeProcess = process
            }

            let fileHandle = stdoutPipe.fileHandleForReading

            do {
                try process.run()

                var buffer = Data()
                while process.isRunning || fileHandle.availableData.count > 0 {
                    let chunk = fileHandle.availableData
                    if chunk.isEmpty {
                        try? await Task.sleep(nanoseconds: 50_000_000)
                        if !process.isRunning { break }
                        continue
                    }
                    buffer.append(chunk)

                    while let newlineIndex = buffer.firstIndex(of: UInt8(ascii: "\n")) {
                        let lineData = buffer.subdata(in: 0..<newlineIndex)
                        buffer.removeSubrange(0...newlineIndex)

                        if let line = String(data: lineData, encoding: .utf8), !line.trimmingCharacters(in: .whitespaces).isEmpty {
                            await self?.handleJsonLine(line, msgId: assistantMsgId)
                        }
                    }
                }

                process.waitUntilExit()

                await MainActor.run {
                    self?.finishExecution(msgId: assistantMsgId)
                }
            } catch {
                await MainActor.run {
                    self?.updateAssistantMessage(id: assistantMsgId, text: "Execution error: \(error.localizedDescription)", isExecuting: false)
                    self?.state = .idle
                    self?.statusText = "Error during execution"
                }
            }
        }
    }

    private func handleJsonLine(_ line: String, msgId: UUID) {
        guard let data = line.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            return
        }

        if type == "step" {
            let stepNum = json["step"] as? Int ?? 1
            let action = json["action"] as? String ?? ""
            let thought = json["thought"] as? String ?? ""
            let message = json["message"] as? String ?? ""
            let success = json["success"] as? Bool ?? true

            let step = AgentStep(
                id: stepNum,
                action: action,
                thought: thought,
                message: message,
                success: success
            )

            if let idx = messages.firstIndex(where: { $0.id == msgId }) {
                messages[idx].steps.append(step)
                if !thought.isEmpty {
                    messages[idx] = ChatMessage(
                        id: messages[idx].id,
                        sender: .assistant,
                        text: thought,
                        timestamp: messages[idx].timestamp,
                        steps: messages[idx].steps,
                        isExecuting: true
                    )
                }
            }
            self.statusText = "[#\(stepNum)] \(action)"
        } else if type == "finished" {
            let success = json["success"] as? Bool ?? false
            let elapsed = json["elapsed_seconds"] as? Double ?? 0.0
            let totalSteps = json["total_steps"] as? Int ?? 0

            let completionText = success
                ? "Task completed successfully in \(String(format: "%.1f", elapsed))s across \(totalSteps) steps."
                : "Task finished."

            if let idx = messages.firstIndex(where: { $0.id == msgId }) {
                messages[idx] = ChatMessage(
                    id: messages[idx].id,
                    sender: .assistant,
                    text: completionText,
                    timestamp: messages[idx].timestamp,
                    steps: messages[idx].steps,
                    isExecuting: false
                )
            }

            self.statusText = success ? "✅ Task Completed" : "Task Finished"
            if voiceFeedbackEnabled && success {
                speakText("Task completed.")
            }
        }
    }

    private func finishExecution(msgId: UUID) {
        self.state = .idle
        self.activeProcess = nil
        if let idx = messages.firstIndex(where: { $0.id == msgId }) {
            var updated = messages[idx]
            updated.isExecuting = false
            messages[idx] = updated
        }
        Task {
            try? await Task.sleep(nanoseconds: 2_500_000_000)
            if self.state == .idle {
                self.statusText = "Click Mic or ⌥Space to Talk"
            }
        }
    }

    private func updateAssistantMessage(id: UUID, text: String, isExecuting: Bool) {
        if let idx = messages.firstIndex(where: { $0.id == id }) {
            messages[idx] = ChatMessage(
                id: messages[idx].id,
                sender: .assistant,
                text: text,
                timestamp: messages[idx].timestamp,
                steps: messages[idx].steps,
                isExecuting: isExecuting
            )
        }
    }

    func emergencyStop() {
        if let proc = activeProcess, proc.isRunning {
            proc.terminate()
            activeProcess = nil
        }
        self.state = .stopped
        self.statusText = "🛑 Emergency Stop Triggered"
        speakText("Agent stopped.")
    }

    func speakText(_ text: String) {
        guard voiceFeedbackEnabled, !text.isEmpty else { return }
        Task.detached(priority: .background) {
            let p = Process()
            p.executableURL = URL(fileURLWithPath: "/usr/bin/say")
            p.arguments = ["-v", "Samantha", text]
            try? p.run()
        }
    }
}
