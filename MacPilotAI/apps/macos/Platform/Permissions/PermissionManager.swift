import Foundation
import AppKit
import AVFoundation

class PermissionManager: ObservableObject {
    @Published var hasScreenRecording: Bool = false
    @Published var hasAccessibility: Bool = false
    @Published var hasMicrophone: Bool = false

    init() {
        self.hasAccessibility = AXIsProcessTrusted()
    }

    func checkAll() {
        self.hasAccessibility = AXIsProcessTrusted()
        self.hasMicrophone = AVCaptureDevice.authorizationStatus(for: .audio) == .authorized
    }

    func openScreenRecordingSettings() {
        CGRequestScreenCaptureAccess()
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture") {
            NSWorkspace.shared.open(url)
        }
    }

    func openAccessibilitySettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility") {
            NSWorkspace.shared.open(url)
        }
    }

    func requestMicrophoneAccess(completion: @escaping (Bool) -> Void) {
        AVCaptureDevice.requestAccess(for: .audio) { [weak self] granted in
            DispatchQueue.main.async {
                self?.hasMicrophone = granted
                completion(granted)
            }
        }
    }
}

