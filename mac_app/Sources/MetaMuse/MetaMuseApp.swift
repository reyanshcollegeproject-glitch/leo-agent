import SwiftUI
import AppKit
import Carbon.HIToolbox

@main
struct MetaMuseApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            MainView()
                .background(WindowConfigurator())
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 620, height: 680)
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    private var hotKeyRef: EventHotKeyRef?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        FnDoubleTapDetector.shared.start()
        setupCarbonFallbackHotKey()
    }

    private func setupCarbonFallbackHotKey() {
        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))

        InstallEventHandler(GetApplicationEventTarget(), { (_, inEvent, _) -> OSStatus in
            DispatchQueue.main.async {
                NotificationCenter.default.post(name: NSNotification.Name("TriggerVoiceDictation"), object: nil)
            }
            return noErr
        }, 1, &eventType, nil, nil)

        // Option + Space hotkey (kVK_Space is 49 / 0x31)
        let hotKeyID = EventHotKeyID(signature: OSType(0x4D555345) /* 'MUSE' */, id: 1)
        let _ = RegisterEventHotKey(
            UInt32(kVK_Space),
            UInt32(optionKey),
            hotKeyID,
            GetApplicationEventTarget(),
            0,
            &hotKeyRef
        )
    }
}

/// Detects double-tap of the Fn (Function / Globe) key on MacBook keyboards.
class FnDoubleTapDetector {
    static let shared = FnDoubleTapDetector()
    private var lastFnDownTime: TimeInterval = 0
    private var isFnDown: Bool = false
    private var globalMonitor: Any?
    private var localMonitor: Any?

    func start() {
        let fnMask: UInt = 0x800000 // Function / Globe key modifier mask

        let handleEvent: (NSEvent) -> Void = { [weak self] event in
            guard let self = self else { return }
            let isDown = (event.modifierFlags.rawValue & fnMask) != 0

            if isDown && !self.isFnDown {
                self.isFnDown = true
                let now = Date().timeIntervalSince1970
                let interval = now - self.lastFnDownTime

                if interval > 0.05 && interval <= 0.45 {
                    print("🔥 Double-tap Fn detected: Opening Meta Muse Agent!")
                    self.lastFnDownTime = 0
                    DispatchQueue.main.async {
                        // Bring window to top & front
                        if let window = NSApplication.shared.windows.first {
                            window.makeKeyAndOrderFront(nil)
                            NSApp.activate(ignoringOtherApps: true)
                        }
                        NotificationCenter.default.post(name: NSNotification.Name("TriggerVoiceDictation"), object: nil)
                    }
                } else {
                    self.lastFnDownTime = now
                }
            } else if !isDown && self.isFnDown {
                self.isFnDown = false
            }
        }

        self.globalMonitor = NSEvent.addGlobalMonitorForEvents(matching: .flagsChanged, handler: handleEvent)
        self.localMonitor = NSEvent.addLocalMonitorForEvents(matching: .flagsChanged) { event in
            handleEvent(event)
            return event
        }
        print("⌨️ Fn Double-Tap listener registered successfully.")
    }
}

struct WindowConfigurator: NSViewRepresentable {
    func makeNSView(context: Context) -> NSView {
        let view = NSView()
        DispatchQueue.main.async {
            if let window = view.window, let screen = NSScreen.main {
                window.title = "Meta Muse Autonomous Agent"
                window.titleVisibility = .hidden
                window.titlebarAppearsTransparent = true
                window.isMovableByWindowBackground = true
                window.backgroundColor = NSColor(red: 0.04, green: 0.05, blue: 0.07, alpha: 1.0)
                window.level = .floating // Floats on top 24/7
                window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]

                // Position exactly at the TOP OF THE MAC SCREEN (10px below menubar)
                let screenFrame = screen.visibleFrame
                let width: CGFloat = 620
                let height: CGFloat = 680
                let x = screenFrame.origin.x + (screenFrame.width - width) / 2
                let y = screenFrame.origin.y + screenFrame.height - height - 10
                window.setFrame(NSRect(x: x, y: y, width: width, height: height), display: true)
                window.makeKeyAndOrderFront(nil)
            }
        }
        return view
    }

    func updateNSView(_ nsView: NSView, context: Context) {}
}
