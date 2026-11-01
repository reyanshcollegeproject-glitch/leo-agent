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
                .onReceive(NotificationCenter.default.publisher(for: NSNotification.Name("TriggerVoiceDictation"))) { _ in
                    // Global hotkey Option+Space triggered
                }
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 580, height: 680)
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    private var hotKeyRef: EventHotKeyRef?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        setupGlobalHotKey()
    }

    private func setupGlobalHotKey() {
        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))

        InstallEventHandler(GetApplicationEventTarget(), { (_, inEvent, _) -> OSStatus in
            DispatchQueue.main.async {
                NotificationCenter.default.post(name: NSNotification.Name("TriggerVoiceDictation"), object: nil)
            }
            return noErr
        }, 1, &eventType, nil, nil)

        // Option + Space hotkey (kVK_Space is 49 / 0x31)
        let hotKeyID = EventHotKeyID(signature: OSType(0x4D555345) /* 'MUSE' */, id: 1)
        let status = RegisterEventHotKey(
            UInt32(kVK_Space),
            UInt32(optionKey),
            hotKeyID,
            GetApplicationEventTarget(),
            0,
            &hotKeyRef
        )

        if status == noErr {
            print("⌨️ Meta Muse Global hotkey [Option + Space] registered.")
        }
    }
}

struct WindowConfigurator: NSViewRepresentable {
    func makeNSView(context: Context) -> NSView {
        let view = NSView()
        DispatchQueue.main.async {
            if let window = view.window {
                window.title = "Meta Muse Autonomous Agent"
                window.titleVisibility = .hidden
                window.titlebarAppearsTransparent = true
                window.isMovableByWindowBackground = true
                window.backgroundColor = NSColor(red: 0.04, green: 0.05, blue: 0.07, alpha: 1.0)
                window.level = .floating // Floating widget by default
                window.makeKeyAndOrderFront(nil)
            }
        }
        return view
    }

    func updateNSView(_ nsView: NSView, context: Context) {}
}
