import Foundation
import SwiftUI

@MainActor
class AppEnvironment: ObservableObject {
    static var shared: AppEnvironment?

    @Published var agentClient: AgentClient
    @Published var eventStream: EventStreamClient
    @Published var permissions: PermissionManager
    @Published var voiceManager: VoiceManager
    @Published var screenVisionManager: ScreenVisionManager
    @Published var chatViewModel: ChatViewModel
    @Published var pendingApproval: ApprovalItem?
    @Published var isExpanded: Bool = true

    // Event deduplication store
    private var handledEventKeys = Set<String>()
    private var lastSpokenText: String = ""
    private var lastSpokenTimestamp: Date = Date.distantPast

    init() {
        let client = AgentClient()
        self.agentClient = client
        self.eventStream = EventStreamClient()
        self.permissions = PermissionManager()
        let vm = VoiceManager()
        self.voiceManager = vm
        self.screenVisionManager = ScreenVisionManager(client: client, voiceManager: vm)
        let chatVM = ChatViewModel(client: client)
        self.chatViewModel = chatVM
        AppEnvironment.shared = self

        self.eventStream.onEventReceived = { [weak self] event in
            self?.handleStreamEvent(event)
        }
        self.eventStream.connect()

        self.voiceManager.onCommandRecognized = { [weak self] text in
            guard let self = self else { return }
            guard !self.voiceManager.isSpeaking else { return }

            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard trimmed.count >= 3 else { return }

            let lower = trimmed.lowercased()
            let isVisionIntent = (lower.contains("look at") && lower.contains("screen")) ||
                                 (lower.contains("what is on") && lower.contains("screen")) ||
                                 (lower.contains("what's on") && lower.contains("screen")) ||
                                 lower.contains("what do you see") ||
                                 lower.contains("can you see my screen") ||
                                 lower.contains("analyze my screen") ||
                                 lower.contains("describe my screen")

            if isVisionIntent {
                Task {
                    _ = await self.screenVisionManager.inspectScreenAndExplain(userPrompt: trimmed)
                }
            } else {
                self.submitTask(trimmed, fromVoice: true)
            }
        }

        self.voiceManager.onStateChanged = { [weak self] in
            self?.updateBorderHaloState()
        }

        HotKeyManager.shared.onTrigger = { [weak self] in
            self?.toggleVoiceFromHotKey()
        }

        HotKeyManager.shared.onEmergencyStop = { [weak self] in
            self?.emergencyStop()
        }

        HotKeyManager.shared.onHelpMode = { [weak self] in
            self?.triggerHelpMode()
        }
    }

    func submitTask(_ goal: String, fromVoice: Bool = false) {
        let trimmed = goal.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }

        // Glow the laptop screen edges with the blue neon halo
        BorderOverlayManager.shared.show()

        chatViewModel.submitGoal(trimmed)
    }

    func updateBorderHaloState() {
        if voiceManager.isListening || voiceManager.isSpeaking || chatViewModel.isExecuting {
            BorderOverlayManager.shared.show()
        } else {
            BorderOverlayManager.shared.hide()
        }
    }

    func toggleVoiceFromHotKey() {
        voiceManager.toggleListening()
        updateBorderHaloState()
    }

    func emergencyStop() {
        print("🛑 Executing Emergency Stop Kill Switch...")
        chatViewModel.cancelActiveTask()
        voiceManager.stopSpeaking()
        BorderOverlayManager.shared.hide()
        voiceManager.speak("Emergency stop engaged.")
    }

    func triggerHelpMode() {
        Task {
            _ = await screenVisionManager.inspectScreenAndExplain(userPrompt: "Analyze my current screen and tell me what you see.")
        }
    }

    private func handleStreamEvent(_ event: [String: Any]) {
        let type = event["type"] as? String ?? ""
        let taskID = event["task_id"] as? String ?? ""
        let step = event["step"] as? Int ?? 0

        let eventKey = "\(taskID):\(type):\(step)"
        if handledEventKeys.contains(eventKey) {
            return
        }
        handledEventKeys.insert(eventKey)
        if handledEventKeys.count > 250 {
            handledEventKeys.removeAll()
        }

        chatViewModel.handleStreamEvent(event)

        if type == "approval_requested" {
            let thought = event["thought"] as? String ?? "Dangerous operation requires approval"
            let action = event["action"] as? [String: Any]
            let cmd = action?["command"] as? String ?? ""

            self.pendingApproval = ApprovalItem(
                id: UUID().uuidString,
                taskId: taskID,
                command: cmd,
                reason: thought,
                severity: "high",
                createdAt: Date()
            )
            voiceManager.speak("Action requires your approval.")
        } else if type == "step_started" {
            BorderOverlayManager.shared.show()
            let speech = (event["speech"] as? String) ?? ""
            let clean = speech.trimmingCharacters(in: .whitespacesAndNewlines)
            if !clean.isEmpty && clean != lastSpokenText {
                lastSpokenText = clean
                lastSpokenTimestamp = Date()
                voiceManager.speak(clean, immediate: true)
            }
        } else if type == "task_started" {
            BorderOverlayManager.shared.show()
        } else if type == "task_completed" {
            self.pendingApproval = nil
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) {
                BorderOverlayManager.shared.hide()
            }

            let speech = (event["speech"] as? String) ?? (event["message"] as? String) ?? ""
            let clean = speech.trimmingCharacters(in: .whitespacesAndNewlines)
            if !clean.isEmpty {
                let now = Date()
                if clean != lastSpokenText || now.timeIntervalSince(lastSpokenTimestamp) > 1.2 {
                    lastSpokenText = clean
                    lastSpokenTimestamp = now
                    voiceManager.speak(clean, immediate: true)
                }
            }
        } else if type == "task_failed" || type == "approval_resolved" {
            self.pendingApproval = nil
            BorderOverlayManager.shared.hide()
        }
    }
}
