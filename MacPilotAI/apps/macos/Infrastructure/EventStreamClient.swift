import Foundation

@MainActor
class EventStreamClient: ObservableObject {
    @Published var isConnected: Bool = false
    @Published var lastEvent: [String: Any]?

    var onEventReceived: (([String: Any]) -> Void)?

    private var webSocketTask: URLSessionWebSocketTask?
    private var session: URLSession?
    private let url: URL
    private var isReconnecting: Bool = false

    init(url: URL = URL(string: "ws://127.0.0.1:8089/ws/events")!) {
        self.url = url
    }

    func connect() {
        // Disconnect any existing task to prevent duplicate sockets
        disconnect()

        let session = URLSession(configuration: .default)
        self.session = session
        let task = session.webSocketTask(with: url)
        self.webSocketTask = task
        task.resume()
        self.isConnected = true
        self.isReconnecting = false
        listenForMessages(for: task)
    }

    func disconnect() {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        session?.invalidateAndCancel()
        session = nil
        self.isConnected = false
    }

    private func listenForMessages(for task: URLSessionWebSocketTask) {
        task.receive { [weak self] result in
            Task { @MainActor [weak self] in
                guard let self = self else { return }
                // Only process if this task is still the active one
                guard self.webSocketTask === task else { return }

                switch result {
                case .failure(let error):
                    self.isConnected = false
                    if !self.isReconnecting {
                        self.isReconnecting = true
                        print("⚠️ WebSocket disconnected: \(error.localizedDescription). Reconnecting in 2s...")
                        try? await Task.sleep(nanoseconds: 2_000_000_000)
                        if self.isReconnecting {
                            self.connect()
                        }
                    }

                case .success(let message):
                    var receivedData: Data?
                    switch message {
                    case .string(let text):
                        receivedData = text.data(using: .utf8)
                    case .data(let data):
                        receivedData = data
                    @unknown default:
                        break
                    }

                    if let data = receivedData,
                       let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
                        self.lastEvent = json
                        self.onEventReceived?(json)
                    }

                    // Continue listening on the same active task
                    self.listenForMessages(for: task)
                }
            }
        }
    }
}
