import Foundation

class AppLifecycleManager {
    static let shared = AppLifecycleManager()
    private var engineProcess: Process?

    func startEngineIfNeeded() {
        // Check if engine is already responding
        let healthURL = URL(string: "http://127.0.0.1:8089/api/v1/health")!
        var request = URLRequest(url: healthURL)
        request.timeoutInterval = 0.5

        let sem = DispatchSemaphore(value: 0)
        var isAlive = false

        URLSession.shared.dataTask(with: request) { _, resp, _ in
            if let http = resp as? HTTPURLResponse, http.statusCode == 200 {
                isAlive = true
            }
            sem.signal()
        }.resume()

        _ = sem.wait(timeout: .now() + 0.6)
        if isAlive {
            print("⚡ MacPilotAI Agent Engine is already running.")
            return
        }

        // Spawn engine binary
        let currentDir = FileManager.default.currentDirectoryPath
        var possiblePaths: [String] = []
        if let resPath = Bundle.main.resourceURL?.appendingPathComponent("bin/leo-engine").path {
            possiblePaths.append(resPath)
        }
        if let resPath = Bundle.main.resourceURL?.appendingPathComponent("bin/macpilot").path {
            possiblePaths.append(resPath)
        }
        possiblePaths.append(contentsOf: [
            "\(currentDir)/services/agent-engine/bin/leo-engine",
            "\(currentDir)/services/agent-engine/bin/macpilot",
            "\(currentDir)/MacPilotAI/services/agent-engine/bin/leo-engine",
            "\(currentDir)/MacPilotAI/services/agent-engine/bin/macpilot",
            "/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/agent-engine/bin/leo-engine",
            "/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/agent-engine/bin/macpilot"
        ])

        guard let binaryPath = possiblePaths.first(where: { FileManager.default.fileExists(atPath: $0) }) else {
            print("⚠️ MacPilotAI engine binary not found. Please compile via: go build -o bin/macpilot cmd/macpilot/main.go")
            return
        }

        let p = Process()
        p.executableURL = URL(fileURLWithPath: binaryPath)
        p.currentDirectoryURL = URL(fileURLWithPath: "/tmp")
        p.arguments = ["--port", "8089"]
        p.standardInput = FileHandle.nullDevice

        let logPath = "/tmp/macpilot_engine.log"
        FileManager.default.createFile(atPath: logPath, contents: nil)
        if let handle = FileHandle(forWritingAtPath: logPath) {
            p.standardOutput = handle
            p.standardError = handle
        }

        do {
            try p.run()
            self.engineProcess = p
            print("🚀 Spawned MacPilotAI Agent Engine (PID: \(p.processIdentifier)).")
        } catch {
            print("❌ Failed to spawn agent engine: \(error.localizedDescription)")
        }
    }

    func stopEngine() {
        if let p = engineProcess, p.isRunning {
            p.terminate()
            print("🛑 Terminated MacPilotAI Agent Engine.")
            engineProcess = nil
        }
    }
}
