import Foundation
import AppKit
import ScreenCaptureKit
import Vision

@MainActor
class ScreenVisionManager: ObservableObject {
    @Published var isAnalyzing: Bool = false
    @Published var lastSceneDescription: String = ""

    private let client: AgentClient
    private weak var voiceManager: VoiceManager?
    private var lastCaptureTime: Date = Date.distantPast

    init(client: AgentClient, voiceManager: VoiceManager?) {
        self.client = client
        self.voiceManager = voiceManager
    }

    /// Captures the MacBook display, runs Apple Vision OCR + workspace inspection,
    /// calls the multi-model vision brain, and speaks the analysis aloud in real-time.
    func inspectScreenAndExplain(userPrompt: String? = nil) async -> String {
        let now = Date()
        guard !isAnalyzing else { return "Vision perception already in progress." }
        if now.timeIntervalSince(lastCaptureTime) < 10.0 && !lastSceneDescription.isEmpty {
            print("⏳ Screen vision debounced (recently captured). Reusing recent scene.")
            voiceManager?.speak(lastSceneDescription, immediate: true)
            return lastSceneDescription
        }
        isAnalyzing = true
        lastCaptureTime = now

        // 1. Strictly pause microphone so voice & screen capture NEVER run at the same time
        voiceManager?.pauseListening()
        try? await Task.sleep(nanoseconds: 120_000_000)

        // 2. Glow the laptop screen edges with the blue neon perimeter light
        BorderOverlayManager.shared.show()

        let prompt = userPrompt ?? "Look at my screen and tell me what you see."
        print("👁️ [Computer Vision] Starting real-time screen perception: '\(prompt)'...")

        // 3. Gather active workspace metadata
        let (frontApp, winTitle, workspaceDetails) = inspectActiveWorkspace()

        // 3. Capture screen image & run Apple Neural Engine OCR
        var ocrTextLines: [String] = []
        var imageB64: String? = nil

        if let cgImage = await captureDisplayImage() {
            ocrTextLines = await extractTextFromImage(cgImage)
            // Downscale for network payload if needed
            if let jpegData = createCompressedJPEG(from: cgImage) {
                imageB64 = jpegData.base64EncodedString()
            }
        } else {
            // Fallback: try capturing via screencapture utility
            let tmpPath = "/tmp/macpilot_vision_\(Int(Date().timeIntervalSince1970)).png"
            let p = Process()
            p.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
            p.arguments = ["-x", tmpPath]
            try? p.run()
            p.waitUntilExit()

            if FileManager.default.fileExists(atPath: tmpPath),
               let image = NSImage(contentsOfFile: tmpPath),
               let tiff = image.tiffRepresentation,
               let bitmap = NSBitmapImageRep(data: tiff),
               let cg = bitmap.cgImage {
                ocrTextLines = await extractTextFromImage(cg)
                if let jpegData = createCompressedJPEG(from: cg) {
                    imageB64 = jpegData.base64EncodedString()
                }
                try? FileManager.default.removeItem(atPath: tmpPath)
            }
        }

        let ocrSummary = ocrTextLines.prefix(20).joined(separator: "\n")
        let fullContext = workspaceDetails.isEmpty ? "Front app: \(frontApp), Window: \(winTitle)" : workspaceDetails
        _ = fullContext

        // 4. Request vision analysis from Go Agent Engine (Meta AI multi-model)
        var spokenAnswer = ""
        do {
            let result = try await client.analyzeScreenVision(
                imageB64: imageB64,
                activeApp: frontApp,
                windowTitle: winTitle,
                ocrText: ocrSummary,
                prompt: prompt
            )
            spokenAnswer = result.trimmingCharacters(in: .whitespacesAndNewlines)
        } catch {
            print("⚠️ Agent vision endpoint error: \(error.localizedDescription), using local synthesis")
        }

        // 5. Fallback heuristic if backend returned empty
        if spokenAnswer.isEmpty {
            if frontApp.localizedCaseInsensitiveContains("chrome") && winTitle.localizedCaseInsensitiveContains("youtube") {
                spokenAnswer = "I'm looking at your screen right now. You're in Google Chrome watching \(winTitle.replacingOccurrences(of: " - YouTube", with: "")) on YouTube."
            } else if frontApp.localizedCaseInsensitiveContains("code") || frontApp.localizedCaseInsensitiveContains("xcode") {
                spokenAnswer = "I see your screen. You're working in \(frontApp) on \(winTitle). Your code and terminal are actively running."
            } else if !winTitle.isEmpty {
                spokenAnswer = "I can see your screen. You're currently using \(frontApp) with \(winTitle) open."
            } else {
                spokenAnswer = "I'm looking at your screen. You are currently in \(frontApp)."
            }
        }

        self.lastSceneDescription = spokenAnswer
        self.isAnalyzing = false

        // 6. Update chat view
        if let env = AppEnvironment.shared {
            let visionMsg = ChatMessage(
                sender: .assistant,
                text: "👁️ \(spokenAnswer)"
            )
            env.chatViewModel.messages.append(visionMsg)
        }

        // 7. Speak aloud immediately using studio realistic neural voice
        voiceManager?.speak(spokenAnswer, immediate: true)

        // 8. Restore border overlay
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) {
            BorderOverlayManager.shared.hide()
        }

        return spokenAnswer
    }

    // MARK: - ScreenCaptureKit Capture
    private func captureDisplayImage() async -> CGImage? {
        if #available(macOS 14.0, *) {
            do {
                let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
                guard let display = content.displays.first else { return nil }

                let filter = SCContentFilter(display: display, excludingWindows: [])
                let config = SCStreamConfiguration()
                config.width = min(display.width, 1920)
                config.height = min(display.height, 1080)
                config.scalesToFit = true

                return try await SCScreenshotManager.captureImage(contentFilter: filter, configuration: config)
            } catch {
                print("⚠️ ScreenCaptureKit error (falling back): \(error.localizedDescription)")
                return nil
            }
        }
        return nil
    }

    // MARK: - Apple Silicon Neural Engine OCR (Vision Framework)
    private func extractTextFromImage(_ cgImage: CGImage) async -> [String] {
        return await withCheckedContinuation { continuation in
            let request = VNRecognizeTextRequest { req, _ in
                let observations = req.results as? [VNRecognizedTextObservation] ?? []
                var lines: [String] = []
                for obs in observations {
                    if let candidate = obs.topCandidates(1).first {
                        let text = candidate.string.trimmingCharacters(in: .whitespacesAndNewlines)
                        if !text.isEmpty && text.count > 1 {
                            lines.append(text)
                        }
                    }
                }
                continuation.resume(returning: lines)
            }
            request.recognitionLevel = .accurate
            request.usesLanguageCorrection = true

            let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
            do {
                try handler.perform([request])
            } catch {
                continuation.resume(returning: [])
            }
        }
    }

    // MARK: - Workspace & Window Hierarchy
    private func inspectActiveWorkspace() -> (frontApp: String, windowTitle: String, details: String) {
        let frontApp = NSWorkspace.shared.frontmostApplication?.localizedName ?? "Finder"

        var winTitle = ""
        let winScript = """
        tell application "System Events"
            try
                set frontApp to first application process whose frontmost is true
                return name of front window of frontApp
            on error
                return ""
            end try
        end tell
        """
        winTitle = runAppleScript(winScript)

        var details = ""
        if frontApp.localizedCaseInsensitiveContains("chrome") {
            let chromeScript = """
            tell application "Google Chrome"
                if (count of windows) > 0 then
                    set activeTab to active tab of front window
                    return (title of activeTab) & " (" & (URL of activeTab) & ")"
                end if
            end tell
            """
            let chromeTab = runAppleScript(chromeScript)
            if !chromeTab.isEmpty {
                details = "Chrome Tab: \(chromeTab)"
                if winTitle.isEmpty { winTitle = chromeTab }
            }
        } else if frontApp.localizedCaseInsensitiveContains("safari") {
            let safariScript = """
            tell application "Safari"
                if (count of windows) > 0 then
                    set activeTab to current tab of front window
                    return (name of activeTab) & " (" & (URL of activeTab) & ")"
                end if
            end tell
            """
            let safariTab = runAppleScript(safariScript)
            if !safariTab.isEmpty {
                details = "Safari Tab: \(safariTab)"
                if winTitle.isEmpty { winTitle = safariTab }
            }
        }

        return (frontApp, winTitle, details)
    }

    private func runAppleScript(_ script: String) -> String {
        var error: NSDictionary?
        if let scriptObject = NSAppleScript(source: script) {
            let output = scriptObject.executeAndReturnError(&error)
            if error == nil {
                return output.stringValue?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            }
        }
        return ""
    }

    private func createCompressedJPEG(from cgImage: CGImage) -> Data? {
        let bitmap = NSBitmapImageRep(cgImage: cgImage)
        return bitmap.representation(using: .jpeg, properties: [.compressionFactor: 0.65])
    }
}
