import Foundation

class AgentClient {
    private let baseURL: URL

    init(baseURL: URL = URL(string: "http://127.0.0.1:8089")!) {
        self.baseURL = baseURL
    }

    func submitTask(goal: String, model: String = "muse-wonder-1.0", mode: String = "computer") async throws -> String {
        let endpoint = baseURL.appendingPathComponent("/api/v1/tasks")
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let payload: [String: Any] = [
            "goal": goal,
            "model": model,
            "mode": mode,
            "max_steps": 25,
            "stream_ws": true
        ]
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 202 else {
            throw NSError(domain: "MacPilot", code: -1, userInfo: [NSLocalizedDescriptionKey: "Failed to submit task"])
        }

        let json = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        return json?["task_id"] as? String ?? ""
    }

    func cancelTask(id: String) async throws {
        var components = URLComponents(url: baseURL.appendingPathComponent("/api/v1/tasks/cancel"), resolvingAgainstBaseURL: false)
        components?.queryItems = [URLQueryItem(name: "id", value: id)]
        guard let url = components?.url else { return }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        _ = try? await URLSession.shared.data(for: request)
    }

    func resolveApproval(id: String, approved: Bool) async throws {
        var components = URLComponents(url: baseURL.appendingPathComponent("/api/v1/approvals"), resolvingAgainstBaseURL: false)
        components?.queryItems = [
            URLQueryItem(name: "id", value: id),
            URLQueryItem(name: "approved", value: approved ? "true" : "false")
        ]
        guard let url = components?.url else { return }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"

        let (_, response) = try await URLSession.shared.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            throw NSError(domain: "MacPilot", code: -1, userInfo: [NSLocalizedDescriptionKey: "Failed to resolve approval"])
        }
    }

    // MARK: - Virtual Computer Controls

    func getComputerStatus() async throws -> ComputerStatusModel? {
        let endpoint = baseURL.appendingPathComponent("/api/v1/computers/status")
        let (data, response) = try await URLSession.shared.data(from: endpoint)
        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            return nil
        }
        return try? JSONDecoder().decode(ComputerStatusModel.self, from: data)
    }

    func startComputer() async throws {
        var request = URLRequest(url: baseURL.appendingPathComponent("/api/v1/computers/start"))
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }

    func stopComputer() async throws {
        var request = URLRequest(url: baseURL.appendingPathComponent("/api/v1/computers/stop"))
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }

    func restartComputer() async throws {
        var request = URLRequest(url: baseURL.appendingPathComponent("/api/v1/computers/restart"))
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }

    func setComputerMode(mode: String) async throws {
        var components = URLComponents(url: baseURL.appendingPathComponent("/api/v1/computers/mode"), resolvingAgainstBaseURL: false)
        components?.queryItems = [URLQueryItem(name: "mode", value: mode)]
        guard let url = components?.url else { return }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }

    func getComputerMetrics() async throws -> ComputerMetricsModel? {
        let endpoint = baseURL.appendingPathComponent("/api/v1/computers/metrics")
        let (data, response) = try await URLSession.shared.data(from: endpoint)
        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            return nil
        }
        return try? JSONDecoder().decode(ComputerMetricsModel.self, from: data)
    }

    // MARK: - Computer Vision Perception API
    func analyzeScreenVision(imageB64: String?, activeApp: String, windowTitle: String, ocrText: String, prompt: String) async throws -> String {
        let endpoint = baseURL.appendingPathComponent("/api/v1/vision/analyze")
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let payload: [String: Any] = [
            "image": imageB64 ?? "",
            "active_app": activeApp,
            "window_title": windowTitle,
            "ocr_text": ocrText,
            "prompt": prompt
        ]
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            throw NSError(domain: "MacPilot", code: -1, userInfo: [NSLocalizedDescriptionKey: "Failed to analyze screen"])
        }
        let json = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        return json?["analysis"] as? String ?? ""
    }
}
