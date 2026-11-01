import Foundation
import AVFoundation
import Speech

/// High-performance native macOS speech capture engine adapted from Flow Widget.
/// Computes real-time RMS audio power to drive live waveform visualizer,
/// and streams live transcription via Apple's SFSpeechRecognizer.
class SpeechManager: NSObject, SFSpeechRecognizerDelegate {
    private let speechRecognizer = SFSpeechRecognizer(locale: Locale(identifier: "en-US"))
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private let audioEngine = AVAudioEngine()

    var onAudioLevelChange: ((Float) -> Void)?
    var onTranscriptUpdate: ((String) -> Void)?
    private var accumulatedTranscript: String = ""

    func startRecording(completion: @escaping (Bool, String?) -> Void) {
        // Request Microphone and Speech permissions
        SFSpeechRecognizer.requestAuthorization { authStatus in
            guard authStatus == .authorized else {
                completion(false, "Speech recognition permission denied in macOS Settings")
                return
            }

            AVCaptureDevice.requestAccess(for: .audio) { granted in
                guard granted else {
                    completion(false, "Microphone access denied in macOS Settings")
                    return
                }

                do {
                    try self.setupAudioSessionAndStart()
                    completion(true, nil)
                } catch {
                    completion(false, error.localizedDescription)
                }
            }
        }
    }

    private func setupAudioSessionAndStart() throws {
        accumulatedTranscript = ""

        recognitionTask?.cancel()
        recognitionTask = nil

        recognitionRequest = SFSpeechAudioBufferRecognitionRequest()
        guard let recognitionRequest = recognitionRequest else {
            throw NSError(domain: "Flow", code: -1, userInfo: [NSLocalizedDescriptionKey: "Unable to allocate speech request"])
        }

        recognitionRequest.shouldReportPartialResults = true
        recognitionRequest.taskHint = .dictation

        let inputNode = audioEngine.inputNode
        let recordingFormat = inputNode.outputFormat(forBus: 0)

        inputNode.removeTap(onBus: 0)
        inputNode.installTap(onBus: 0, bufferSize: 1024, format: recordingFormat) { [weak self] buffer, _ in
            self?.recognitionRequest?.append(buffer)

            // Calculate Audio RMS Power for Waveform Visualizer: RMS = sqrt(1/N * sum(x[i]^2))
            guard let channelData = buffer.floatChannelData?[0] else { return }
            let frameLength = Int(buffer.frameLength)
            var sum: Float = 0
            for i in 0..<frameLength {
                sum += channelData[i] * channelData[i]
            }
            let rms = sqrt(sum / Float(frameLength))
            let normalizedLevel = min(max(rms * 12.0, 0.05), 1.0)
            self?.onAudioLevelChange?(normalizedLevel)
        }

        audioEngine.prepare()
        try audioEngine.start()

        recognitionTask = speechRecognizer?.recognitionTask(with: recognitionRequest) { [weak self] result, error in
            if let result = result {
                let text = result.bestTranscription.formattedString
                self?.accumulatedTranscript = text
                self?.onTranscriptUpdate?(text)
            }
        }
    }

    func stopRecording() -> String {
        audioEngine.stop()
        audioEngine.inputNode.removeTap(onBus: 0)
        recognitionRequest?.endAudio()
        recognitionTask?.finish()

        return accumulatedTranscript
    }
}
