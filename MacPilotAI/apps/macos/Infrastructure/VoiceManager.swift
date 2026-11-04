import Foundation
import AVFoundation
import Speech

@MainActor
class VoiceManager: NSObject, ObservableObject, SFSpeechRecognizerDelegate, AVAudioPlayerDelegate {
    @Published var isListening: Bool = false {
        didSet { onStateChanged?() }
    }
    @Published var liveTranscript: String = ""
    @Published var audioLevel: Float = 0.0
    @Published var isSpeaking: Bool = false {
        didSet { onStateChanged?() }
    }
    @Published var isVoiceOutputEnabled: Bool = true
    @Published var errorMessage: String? = nil
    @Published var requiresPermissionPrompt: Bool = false

    var onCommandRecognized: ((String) -> Void)?
    var onStateChanged: (() -> Void)?
    var shouldContinuousListen: Bool = false

    private let speechRecognizer = SFSpeechRecognizer(locale: Locale(identifier: "en-US"))
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private let audioEngine = AVAudioEngine()
    private var audioPlayer: AVAudioPlayer?

    private var silenceTimer: Timer?
    private var lastSpeechTime: Date = Date()
    private var currentSynthesisProcess: Process?
    private var currentSayProcess: Process?
    private var speechEndCooldown: Date = Date.distantPast

    // Safeguard to prevent recursive cancellation/restart storms
    private var isIntentionalCancellation: Bool = false
    private var isRestartingCapture: Bool = false
    private var isRequestingAuth: Bool = false

    // Frame-level debouncer to prevent multiple speech triggers in a single frame
    private var lastSpokenText: String = ""
    private var lastSpeakAttempt: Date = Date.distantPast

    override init() {
        super.init()
        self.speechRecognizer?.delegate = self
        self.isListening = false
    }

    private func checkPermissionsOnInit() {
        // App starts in quiet standby mode. Mic and vision are strictly user-initiated.
        self.isListening = false
    }

    func requestPermissions(completion: @escaping (Bool) -> Void) {
        guard !isRequestingAuth else {
            completion(false)
            return
        }
        isRequestingAuth = true
        AVCaptureDevice.requestAccess(for: .audio) { [weak self] micGranted in
            guard let self = self else { return }
            guard micGranted else {
                Task { @MainActor in
                    self.isRequestingAuth = false
                    self.requiresPermissionPrompt = true
                    completion(false)
                }
                return
            }

            SFSpeechRecognizer.requestAuthorization { [weak self] authStatus in
                Task { @MainActor in
                    guard let self = self else { return }
                    self.isRequestingAuth = false
                    let granted = (authStatus == .authorized)
                    self.requiresPermissionPrompt = !granted
                    if granted {
                        self.beginAudioEngineCapture()
                    }
                    completion(granted)
                }
            }
        }
    }

    // MARK: - 100% Human British Male Voice Output (Ryan Neural / Daniel)
    // Strictly ZERO female robot voices (Zero Samantha, Zero Nora, Zero Simone)
    func speak(_ text: String, immediate: Bool = true) {
        guard isVoiceOutputEnabled else { return }
        let clean = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !clean.isEmpty else { return }

        let now = Date()
        // Deduplicate & debounce: drop identical speech requested within 1.5s
        if clean == lastSpokenText && now.timeIntervalSince(lastSpeakAttempt) < 1.5 {
            print("🔇 Suppressed duplicate voice frame: \(clean.prefix(40))...")
            return
        }
        lastSpokenText = clean
        lastSpeakAttempt = now

        // Immediately stop any currently playing audio and mute mic recognition
        stopSpeaking()
        self.isSpeaking = true
        self.liveTranscript = ""
        self.silenceTimer?.invalidate()
        self.silenceTimer = nil
        self.speechEndCooldown = Date().addingTimeInterval(2.5)

        // 100% High-Fidelity Realistic Neural Human Voice (RyanNeural via edge-tts)
        // Strictly ZERO robotic speech synthesis
        speakRealisticNeural(clean)
    }

    // Realistic Neural Human Voice using edge-tts studio neural model
    private func speakRealisticNeural(_ text: String) {
        var possibleScriptPaths: [String] = []
        if let resPath = Bundle.main.path(forResource: "speak_human", ofType: "sh") {
            possibleScriptPaths.append(resPath)
        }
        if let resURL = Bundle.main.resourceURL?.appendingPathComponent("speak_human.sh").path {
            possibleScriptPaths.append(resURL)
        }
        possibleScriptPaths.append(contentsOf: [
            "/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/scripts/speak_human.sh",
            "scripts/speak_human.sh"
        ])

        guard let scriptPath = possibleScriptPaths.first(where: { FileManager.default.fileExists(atPath: $0) }) else {
            print("⚠️ speak_human.sh not found.")
            self.isSpeaking = false
            return
        }

        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/bash")
        p.arguments = [scriptPath, text]
        self.currentSayProcess = p

        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            do {
                try p.run()
                p.waitUntilExit()
            } catch {
                print("⚠️ Realistic Neural Speech process error: \(error)")
            }
            Task { @MainActor [weak self] in
                guard let self = self else { return }
                self.currentSayProcess = nil
                self.isSpeaking = false
                self.speechEndCooldown = Date().addingTimeInterval(1.5)
                self.liveTranscript = ""

                // Ensure listening stays off after speaking to prevent acoustic feedback loop
                self.shouldContinuousListen = false
            }
        }
    }

    private func playAudioFile(at path: String, isTemporary: Bool = true) {
        let url = URL(fileURLWithPath: path)
        do {
            let player = try AVAudioPlayer(contentsOf: url)
            player.delegate = self
            self.audioPlayer = player
            player.prepareToPlay()
            if player.play() {
                self.isSpeaking = true
            } else {
                if isTemporary { try? FileManager.default.removeItem(at: url) }
                self.isSpeaking = false
            }
        } catch {
            print("⚠️ AVAudioPlayer playback error: \(error)")
            if isTemporary { try? FileManager.default.removeItem(at: url) }
            self.isSpeaking = false
        }
    }

    func stopSpeaking() {
        if let p = currentSynthesisProcess, p.isRunning {
            p.terminate()
            currentSynthesisProcess = nil
        }

        if let p = currentSayProcess, p.isRunning {
            p.terminate()
            currentSayProcess = nil
        }

        if let player = audioPlayer {
            if player.isPlaying {
                player.stop()
            }
            if let url = player.url, url.path.contains("macpilot_speech_") {
                try? FileManager.default.removeItem(at: url)
            }
            audioPlayer = nil
        }

        self.speechEndCooldown = Date().addingTimeInterval(0.6)
        self.isSpeaking = false
    }

    // MARK: - Real-Time Continuous Speech Recognition
    func toggleListening() {
        if isListening {
            shouldContinuousListen = false
            pauseListening()
        } else {
            shouldContinuousListen = true
            startListening()
        }
    }

    func startListening() {
        stopSpeaking()
        errorMessage = nil

        let micStatus = AVCaptureDevice.authorizationStatus(for: .audio)
        let speechStatus = SFSpeechRecognizer.authorizationStatus()

        // 1. If already authorized, directly begin capture
        if micStatus == .authorized && speechStatus == .authorized {
            beginAudioEngineCapture()
            return
        }

        // 2. Prevent concurrent / recursive permission prompts
        guard !isRequestingAuth else {
            print("⏳ Speech/Microphone authorization request already in progress.")
            return
        }

        // 3. If explicitly denied or restricted, do not trigger system prompts again
        if micStatus == .denied || micStatus == .restricted || speechStatus == .denied || speechStatus == .restricted {
            self.errorMessage = "Please enable Microphone & Speech Recognition in System Settings."
            self.requiresPermissionPrompt = true
            self.isListening = false
            return
        }

        // 4. Request permissions once cleanly
        isRequestingAuth = true
        AVCaptureDevice.requestAccess(for: .audio) { [weak self] micGranted in
            guard let self = self else { return }
            guard micGranted else {
                Task { @MainActor in
                    self.isRequestingAuth = false
                    self.isListening = false
                    self.errorMessage = "Microphone access was denied."
                }
                return
            }

            SFSpeechRecognizer.requestAuthorization { [weak self] authStatus in
                Task { @MainActor in
                    guard let self = self else { return }
                    self.isRequestingAuth = false
                    if authStatus == .authorized {
                        self.beginAudioEngineCapture()
                    } else {
                        self.isListening = false
                        self.errorMessage = "Speech recognition access was not granted."
                    }
                }
            }
        }
    }

    private func beginAudioEngineCapture() {
        silenceTimer?.invalidate()
        silenceTimer = nil

        isIntentionalCancellation = true
        recognitionTask?.cancel()
        recognitionTask = nil
        isIntentionalCancellation = false

        if audioEngine.isRunning {
            audioEngine.stop()
            audioEngine.inputNode.removeTap(onBus: 0)
        }

        let req = SFSpeechAudioBufferRecognitionRequest()
        req.shouldReportPartialResults = true
        req.taskHint = .dictation
        if speechRecognizer?.supportsOnDeviceRecognition == true {
            req.requiresOnDeviceRecognition = true
        }
        self.recognitionRequest = req

        let inputNode = audioEngine.inputNode
        let recordingFormat = inputNode.outputFormat(forBus: 0)

        inputNode.installTap(onBus: 0, bufferSize: 1024, format: recordingFormat) { [weak self] buffer, _ in
            guard let self = self else { return }

            // Drop mic buffer if agent is speaking (acoustic echo cancellation)
            if self.isSpeaking || Date() < self.speechEndCooldown {
                return
            }

            self.recognitionRequest?.append(buffer)

            guard let channelData = buffer.floatChannelData?[0] else { return }
            let frameLength = Int(buffer.frameLength)
            var sum: Float = 0
            for i in 0..<frameLength {
                sum += channelData[i] * channelData[i]
            }
            let rms = sqrt(sum / Float(max(frameLength, 1)))
            let normalized = min(max(rms * 12.0, 0.04), 1.0)

            Task { @MainActor [weak self] in
                self?.audioLevel = normalized
            }
        }

        audioEngine.prepare()
        do {
            try audioEngine.start()
            self.isListening = true
            print("🎙️ Real-time voice listening active (Mic ready).")
        } catch {
            self.errorMessage = "Failed to start audio engine: \(error.localizedDescription)"
            return
        }

        startRecognitionTask(with: req)
    }

    private func startRecognitionTask(with request: SFSpeechAudioBufferRecognitionRequest) {
        recognitionTask = speechRecognizer?.recognitionTask(with: request) { [weak self] result, error in
            Task { @MainActor in
                guard let self = self else { return }

                if self.isSpeaking || Date() < self.speechEndCooldown {
                    return
                }

                if let result = result {
                    let text = result.bestTranscription.formattedString
                    self.liveTranscript = text
                    self.lastSpeechTime = Date()

                    // Conversational pause trigger: 0.85s silence before dispatching
                    self.silenceTimer?.invalidate()
                    self.silenceTimer = Timer.scheduledTimer(withTimeInterval: 0.85, repeats: false) { [weak self] _ in
                        Task { @MainActor in
                            guard let self = self, self.isListening, !self.isSpeaking, Date() >= self.speechEndCooldown else { return }
                            let trimmed = self.liveTranscript.trimmingCharacters(in: .whitespacesAndNewlines)
                            let words = trimmed.components(separatedBy: .whitespacesAndNewlines).filter { !$0.isEmpty }
                            let singleWordCmds = ["stop", "cancel", "help", "status", "mute", "unmute", "clear", "hey", "hello", "hi"]
                            let isCommand = words.count >= 2 || singleWordCmds.contains(trimmed.lowercased())
                            if !trimmed.isEmpty && isCommand {
                                print("⚡ Spoken command recognized: \(trimmed)")
                                self.dispatchAndPauseListening(command: trimmed)
                            }
                        }
                    }
                }

                if let error = error {
                    if self.isIntentionalCancellation { return }
                    let nsError = error as NSError
                    if nsError.code == 216 || (nsError.domain == "kAFAssistantErrorDomain" && nsError.code == 1110) {
                        return
                    }

                    if SFSpeechRecognizer.authorizationStatus() != .authorized {
                        print("🎙️ Speech authorization not granted. Pausing capture.")
                        self.pauseListening()
                        return
                    }

                    if self.isListening && !self.isSpeaking && !self.isRestartingCapture {
                        self.isRestartingCapture = true
                        DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) { [weak self] in
                            guard let self = self else { return }
                            self.isRestartingCapture = false
                            if self.isListening && !self.isSpeaking && SFSpeechRecognizer.authorizationStatus() == .authorized {
                                self.restartCaptureLoop()
                            }
                        }
                    }
                }
            }
        }
    }

    private func dispatchAndPauseListening(command: String) {
        self.liveTranscript = ""
        silenceTimer?.invalidate()
        silenceTimer = nil

        // Immediately pause mic to prevent any acoustic echo or self-hearing during execution & speech
        pauseListening()
        onCommandRecognized?(command)
    }

    private func restartCaptureLoop() {
        guard audioEngine.isRunning, isListening else { return }

        isIntentionalCancellation = true
        recognitionTask?.cancel()
        recognitionTask = nil
        recognitionRequest?.endAudio()
        recognitionRequest = nil
        isIntentionalCancellation = false

        let req = SFSpeechAudioBufferRecognitionRequest()
        req.shouldReportPartialResults = true
        req.taskHint = .dictation
        if speechRecognizer?.supportsOnDeviceRecognition == true {
            req.requiresOnDeviceRecognition = true
        }
        self.recognitionRequest = req

        startRecognitionTask(with: req)
    }

    func pauseListening() {
        silenceTimer?.invalidate()
        silenceTimer = nil

        if audioEngine.isRunning {
            audioEngine.stop()
            audioEngine.inputNode.removeTap(onBus: 0)
        }

        isIntentionalCancellation = true
        recognitionRequest?.endAudio()
        recognitionTask?.finish()
        recognitionTask = nil
        recognitionRequest = nil
        isIntentionalCancellation = false

        self.isListening = false
        self.audioLevel = 0.0
        self.liveTranscript = ""
    }

    // MARK: - AVAudioPlayerDelegate
    nonisolated func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully flag: Bool) {
        Task { @MainActor in
            self.isSpeaking = false
            self.speechEndCooldown = Date().addingTimeInterval(0.3)
            if let url = player.url, url.path.contains("macpilot_speech_") {
                try? FileManager.default.removeItem(at: url)
            }
        }
    }
}
