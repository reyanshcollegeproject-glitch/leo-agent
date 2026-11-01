import Foundation
import SwiftUI
import AppKit

@MainActor
class AgentBridge: ObservableObject {
    @Published var messages: [ChatMessage] = []
    @Published var state: AppAgentState = .idle
    @Published var hasScreenPermission: Bool = true
    @Published var voiceFeedbackEnabled: Bool = true
    @Published var isListeningVoice: Bool = false

    private var activeProcess: Process?
    private let workspacePath: String

    init() {
        // Resolve path to auto agent repo
        self.workspacePath = "/Users/reyansh/Desktop/contribute/auto agent"
        checkPermissions()
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

    private func seedWelcomeMessage() {
        let welcome = ChatMessage(
            sender: .assistant,
            text: "Hello! I am Meta Muse, your 24/7 autonomous macOS assistant. You can chat or talk to me to control apps, play music, automate tasks, and navigate your Mac.",
            timestamp: Date(),
            steps: []
        )
        messages.append(welcome)
    }

    func executeGoal(_ goal: String) {
        let trimmed = goal.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }

        // 1. Append user message to chat
        let userMsg = ChatMessage(sender: .user, text: trimmed)
        messages.append(userMsg)

        // 2. Append assistant placeholder message
        let assistantMsgId = UUID()
        let assistantMsg = ChatMessage(
            id: assistantMsgId,
            sender: .assistant,
            text: "Analyzing goal and planning execution...",
            steps: [],
            isExecuting: true
        )
        messages.append(assistantMsg)
        self.state = .running

        // 3. Find Python executable in workspace venv
        let pythonPath = "\(workspacePath)/venv/bin/python"
        guard FileManager.default.fileExists(atPath: pythonPath) else {
            updateAssistantMessage(
                id: assistantMsgId,
                text: "Error: Python virtual environment not found at \(pythonPath)",
                isExecuting: false
            )
            self.state = .idle
            return
        }

        // 4. Launch asynchronous CLI process with --json-stream
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

                    // Process complete newline-delimited lines
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
                    self?.updateAssistantMessage(
                        id: assistantMsgId,
                        text: "Execution error: \(error.localizedDescription)",
                        isExecuting: false
                    )
                    self?.state = .idle
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
        } else if type == "finished" {
            let success = json["success"] as? Bool ?? false
            let elapsed = json["elapsed_seconds"] as? Double ?? 0.0
            let totalSteps = json["total_steps"] as? Int ?? 0

            let completionText = success
                ? "Task completed successfully in \(String(format: "%.1f", elapsed))s across \(totalSteps) steps."
                : "Task terminated or exceeded step limit."

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
        if let lastIdx = messages.indices.last, messages[lastIdx].isExecuting {
            var updated = messages[lastIdx]
            updated.isExecuting = false
            updated = ChatMessage(
                id: updated.id,
                sender: .assistant,
                text: "Emergency stop triggered by user.",
                timestamp: updated.timestamp,
                steps: updated.steps,
                isExecuting: false
            )
            messages[lastIdx] = updated
        }
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

    func startVoiceListening() {
        guard !isListeningVoice else { return }
        self.isListeningVoice = true
        self.state = .listening
        speakText("Listening.")

        // Invoke python voice listener
        let pythonPath = "\(workspacePath)/venv/bin/python"
        Task.detached(priority: .userInitiated) { [weak self, pythonPath] in
            let p = Process()
            p.executableURL = URL(fileURLWithPath: pythonPath)
            p.arguments = ["-c", """
from muse_agent.voice.engine import VoiceEngine
v = VoiceEngine()
txt = v.listen_and_transcribe(duration_seconds=3.5)
if txt:
    print(txt)
"""]
            let pipe = Pipe()
            p.standardOutput = pipe
            try? p.run()
            p.waitUntilExit()

            let data = pipe.fileHandleForReading.readDataToEndOfFile()
            let transcribed = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)

            await MainActor.run {
                self?.isListeningVoice = false
                self?.state = .idle
                if let query = transcribed, !query.isEmpty {
                    self?.executeGoal(query)
                } else {
                    self?.speakText("I didn't catch that. Please try again.")
                }
            }
        }
    }
}
