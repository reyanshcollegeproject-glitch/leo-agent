import Foundation

enum TaskStatus: String, Codable {
    case queued = "QUEUED"
    case planning = "PLANNING"
    case running = "RUNNING"
    case waitingApproval = "WAITING_FOR_APPROVAL"
    case verifying = "VERIFYING"
    case completed = "COMPLETED"
    case failed = "FAILED"
    case cancelled = "CANCELLED"
}

enum AgentMode: String, CaseIterable, Identifiable, Codable {
    case computer = "computer"
    case background = "background"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .computer: return "🖥️ Computer"
        case .background: return "⚡ Background"
        }
    }
}

struct TaskStepItem: Identifiable, Codable {
    var id: String { "\(stepNumber)_\(actionType)" }
    let stepNumber: Int
    let actionType: String
    let command: String?
    let thought: String?
    let speech: String?
    let message: String?
    let success: Bool
}

struct TaskItem: Identifiable, Codable {
    let id: String
    let goal: String
    var status: TaskStatus
    var currentStep: Int
    var steps: [TaskStepItem]
    var error: String?
    let createdAt: Date
}

struct ApprovalItem: Identifiable, Codable {
    let id: String
    let taskId: String
    let command: String
    let reason: String
    let severity: String
    let createdAt: Date
}

enum MessageSender: String, Codable {
    case user
    case assistant
    case system
}

struct ChatMessage: Identifiable {
    let id: UUID
    let sender: MessageSender
    let text: String
    let timestamp: Date
    var steps: [TaskStepItem]
    var isExecuting: Bool

    init(
        id: UUID = UUID(),
        sender: MessageSender,
        text: String,
        timestamp: Date = Date(),
        steps: [TaskStepItem] = [],
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

// MARK: - Virtual Computer Models

struct ComputerModel: Codable {
    let id: String
    let name: String
    let os: String
    let status: String
    let cpuCores: Int
    let memoryMb: Int
    let diskSizeGb: Int
    let vncPort: Int
    let guestAgentPort: Int
    let controlMode: String

    enum CodingKeys: String, CodingKey {
        case id, name, os, status
        case cpuCores = "cpu_cores"
        case memoryMb = "memory_mb"
        case diskSizeGb = "disk_size_gb"
        case vncPort = "vnc_port"
        case guestAgentPort = "guest_agent_port"
        case controlMode = "control_mode"
    }
}

struct ComputerStatusModel: Codable {
    let computer: ComputerModel
    let vncReady: Bool
    let agentConnected: Bool

    enum CodingKeys: String, CodingKey {
        case computer
        case vncReady = "vnc_ready"
        case agentConnected = "agent_connected"
    }
}

struct ComputerMetricsModel: Codable {
    let cpuPercent: Double
    let memoryUsedMb: Double
    let memoryTotalMb: Double
    let diskUsedMb: Double
    let diskTotalMb: Double

    enum CodingKeys: String, CodingKey {
        case cpuPercent = "cpu_percent"
        case memoryUsedMb = "memory_used_mb"
        case memoryTotalMb = "memory_total_mb"
        case diskUsedMb = "disk_used_mb"
        case diskTotalMb = "disk_total_mb"
    }
}

