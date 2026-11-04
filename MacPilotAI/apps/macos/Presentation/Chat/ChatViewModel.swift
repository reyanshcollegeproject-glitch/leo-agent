import Foundation
import SwiftUI

@MainActor
class ChatViewModel: ObservableObject {
    @Published var messages: [ChatMessage] = []
    @Published var inputText: String = ""
    @Published var isExecuting: Bool = false
    @Published var currentStatus: String = "● Ready"
    @Published var selectedMode: AgentMode = .computer

    private let client: AgentClient
    private var activeTaskId: String?

    init(client: AgentClient) {
        self.client = client
        seedWelcome()
    }

    private func seedWelcome() {
        let welcome = ChatMessage(
            sender: .assistant,
            text: "Good day! I am L.E.O., ready to assist you. Tell me what you'd like to do."
        )
        messages.append(welcome)
    }

    func submitGoal(_ goal: String) {
        let trimmed = goal.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }

        // User message
        let userMsg = ChatMessage(sender: .user, text: trimmed)
        messages.append(userMsg)

        // Assistant placeholder
        let placeholder = ChatMessage(
            sender: .assistant,
            text: "Analyzing instructions and orchestrating execution plan...",
            isExecuting: true
        )
        messages.append(placeholder)
        self.isExecuting = true
        self.currentStatus = "● Executing: \(trimmed)"

        // Show screen border glow in computer mode
        if selectedMode == .computer {
            BorderOverlayManager.shared.show()
        }

        Task {
            do {
                let taskId = try await client.submitTask(goal: trimmed, mode: selectedMode.rawValue)
                self.activeTaskId = taskId
            } catch {
                BorderOverlayManager.shared.hide()
                if let lastIdx = messages.indices.last {
                    messages[lastIdx] = ChatMessage(
                        id: messages[lastIdx].id,
                        sender: .assistant,
                        text: "Error submitting task: \(error.localizedDescription)",
                        isExecuting: false
                    )
                }
                self.isExecuting = false
                self.currentStatus = "● Ready"
            }
        }
    }

    func cancelActiveTask() {
        guard let id = activeTaskId, isExecuting else { return }
        Task {
            try? await client.cancelTask(id: id)
            self.activeTaskId = nil
            self.isExecuting = false
            self.currentStatus = "● Cancelled"
            BorderOverlayManager.shared.hide()
            if let lastIdx = messages.indices.last, messages[lastIdx].isExecuting {
                messages[lastIdx] = ChatMessage(
                    id: messages[lastIdx].id,
                    sender: .assistant,
                    text: "⏹️ Task stopped by user.",
                    steps: messages[lastIdx].steps,
                    isExecuting: false
                )
            }
        }
    }

    func handleStreamEvent(_ event: [String: Any]) {
        let type = event["type"] as? String ?? ""
        _ = event["task_id"] as? String ?? ""

        guard let lastIdx = messages.indices.last, messages[lastIdx].isExecuting else {
            return
        }

        if type == "step_started" || type == "step_completed" {
            let stepNum = event["step"] as? Int ?? 1
            let speech = event["speech"] as? String ?? ""
            let thought = event["thought"] as? String ?? ""
            let message = event["message"] as? String ?? ""
            let success = event["success"] as? Bool ?? true
            let actionDict = event["action"] as? [String: Any]
            let actionType = actionDict?["action"] as? String ?? "ACTION"
            let command = actionDict?["command"] as? String

            let step = TaskStepItem(
                stepNumber: stepNum,
                actionType: actionType.uppercased(),
                command: command,
                thought: thought,
                speech: speech,
                message: message,
                success: success
            )

            var updated = messages[lastIdx]
            updated.steps.append(step)
            let spokenText = !speech.isEmpty ? speech : thought
            if !spokenText.isEmpty {
                updated = ChatMessage(
                    id: updated.id,
                    sender: .assistant,
                    text: spokenText,
                    timestamp: updated.timestamp,
                    steps: updated.steps,
                    isExecuting: true
                )
            }
            messages[lastIdx] = updated
            self.currentStatus = "● [Step #\(stepNum)] \(actionType)"
        } else if type == "task_completed" {
            BorderOverlayManager.shared.hide()
            let elapsed = event["elapsed_seconds"] as? Double ?? 0.0
            let speech = event["speech"] as? String ?? ""
            let thought = event["thought"] as? String ?? "Task completed successfully."
            let finalText = !speech.isEmpty ? speech : thought

            var updated = messages[lastIdx]
            updated = ChatMessage(
                id: updated.id,
                sender: .assistant,
                text: "✅ \(finalText) (Completed in \(String(format: "%.1f", elapsed))s)",
                timestamp: updated.timestamp,
                steps: updated.steps,
                isExecuting: false
            )
            messages[lastIdx] = updated
            self.isExecuting = false
            self.currentStatus = "● Ready"
        } else if type == "task_failed" {
            BorderOverlayManager.shared.hide()
            let msg = event["message"] as? String ?? "Task failed"
            var updated = messages[lastIdx]
            updated = ChatMessage(
                id: updated.id,
                sender: .assistant,
                text: "⚠️ \(msg)",
                timestamp: updated.timestamp,
                steps: updated.steps,
                isExecuting: false
            )
            messages[lastIdx] = updated
            self.isExecuting = false
            self.currentStatus = "● Ready"
        }
    }
}
