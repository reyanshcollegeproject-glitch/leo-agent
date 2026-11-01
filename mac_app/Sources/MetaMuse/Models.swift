import Foundation

enum MessageSender: String, Codable {
    case user
    case assistant
    case system
}

struct AgentStep: Identifiable, Codable {
    let id: Int
    let action: String
    let thought: String
    let message: String
    let success: Bool

    init(id: Int, action: String, thought: String, message: String, success: Bool = true) {
        self.id = id
        self.action = action
        self.thought = thought
        self.message = message
        self.success = success
    }
}

struct ChatMessage: Identifiable {
    let id: UUID
    let sender: MessageSender
    let text: String
    let timestamp: Date
    var steps: [AgentStep]
    var isExecuting: Bool

    init(
        id: UUID = UUID(),
        sender: MessageSender,
        text: String,
        timestamp: Date = Date(),
        steps: [AgentStep] = [],
        isExecuting: Bool = false
    ) {
        self.id = id
        self.sender = sender
        self.text = text
        self.timestamp = timestamp
        self.steps = steps
        self.isExecuting = isExecuting
    }
}

enum AppAgentState: String {
    case idle = "ONLINE 24/7"
    case running = "EXECUTING..."
    case listening = "LISTENING (VOICE)..."
    case paused = "PAUSED"
    case completed = "COMPLETED"
    case stopped = "STOPPED"
}
