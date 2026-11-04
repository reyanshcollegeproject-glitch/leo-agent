import Foundation
import AppKit
import Carbon.HIToolbox

@MainActor
class HotKeyManager: ObservableObject {
    static let shared = HotKeyManager()

    var onTrigger: (() -> Void)?
    var onEmergencyStop: (() -> Void)?
    var onHelpMode: (() -> Void)?

    private var triggerHotKeyRef: EventHotKeyRef?
    private var stopHotKeyRef: EventHotKeyRef?
    private var helpHotKeyRef: EventHotKeyRef?
    private var globalMonitor: Any?
    private var localMonitor: Any?
    private var lastFnTapTime: TimeInterval = 0

    init() {
        setupFnDoubleTap()
        setupCarbonHotKeys()
    }

    deinit {
        if let g = globalMonitor {
            NSEvent.removeMonitor(g)
        }
        if let l = localMonitor {
            NSEvent.removeMonitor(l)
        }
        if let ref = triggerHotKeyRef {
            UnregisterEventHotKey(ref)
        }
        if let ref = stopHotKeyRef {
            UnregisterEventHotKey(ref)
        }
        if let ref = helpHotKeyRef {
            UnregisterEventHotKey(ref)
        }
    }

    // MARK: - Double-Tap Fn (Globe) Key Monitor
    private func setupFnDoubleTap() {
        globalMonitor = NSEvent.addGlobalMonitorForEvents(matching: .flagsChanged) { [weak self] event in
            Task { @MainActor in
                self?.handleFlagsChanged(event)
            }
        }

        localMonitor = NSEvent.addLocalMonitorForEvents(matching: .flagsChanged) { [weak self] event in
            Task { @MainActor in
                self?.handleFlagsChanged(event)
            }
            return event
        }
    }

    private func handleFlagsChanged(_ event: NSEvent) {
        let isFnPressed = event.modifierFlags.contains(.function) || (event.modifierFlags.rawValue & 0x800000 != 0)

        if isFnPressed {
            let now = Date().timeIntervalSince1970
            let diff = now - lastFnTapTime

            if diff > 0.05 && diff < 0.45 {
                print("⚡ Double-Tap Fn detected! Triggering L.E.O. widget.")
                self.onTrigger?()
                lastFnTapTime = 0
            } else {
                lastFnTapTime = now
            }
        }
    }

    // MARK: - Carbon Global Hotkeys
    private func setupCarbonHotKeys() {
        var eventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard),
            eventKind: UInt32(kEventHotKeyPressed)
        )

        InstallEventHandler(GetApplicationEventTarget(), { (_, inEvent, _) -> OSStatus in
            var hotKeyID = EventHotKeyID()
            let err = GetEventParameter(
                inEvent,
                EventParamName(kEventParamDirectObject),
                EventParamType(typeEventHotKeyID),
                nil,
                MemoryLayout<EventHotKeyID>.size,
                nil,
                &hotKeyID
            )

            if err == noErr {
                Task { @MainActor in
                    switch hotKeyID.id {
                    case 1:
                        HotKeyManager.shared.onTrigger?()
                    case 2:
                        print("🛑 Emergency Stop HotKey (⌘⇧S) pressed.")
                        HotKeyManager.shared.onEmergencyStop?()
                    case 3:
                        print("📸 Help Mode HotKey (⌘⇧H) pressed.")
                        HotKeyManager.shared.onHelpMode?()
                    default:
                        break
                    }
                }
            }
            return noErr
        }, 1, &eventType, nil, nil)

        let sig = OSType(0x4D50494C) // 'MPIL'

        // 1. Option + Space (Toggle Widget)
        let triggerID = EventHotKeyID(signature: sig, id: 1)
        _ = RegisterEventHotKey(
            UInt32(kVK_Space),
            UInt32(optionKey),
            triggerID,
            GetApplicationEventTarget(),
            0,
            &triggerHotKeyRef
        )

        // 2. Command + Shift + S (Emergency Stop Kill Switch)
        let stopID = EventHotKeyID(signature: sig, id: 2)
        _ = RegisterEventHotKey(
            UInt32(kVK_ANSI_S),
            UInt32(cmdKey | shiftKey),
            stopID,
            GetApplicationEventTarget(),
            0,
            &stopHotKeyRef
        )

        // 3. Command + Shift + H (Help Mode - Screen capture context)
        let helpID = EventHotKeyID(signature: sig, id: 3)
        _ = RegisterEventHotKey(
            UInt32(kVK_ANSI_H),
            UInt32(cmdKey | shiftKey),
            helpID,
            GetApplicationEventTarget(),
            0,
            &helpHotKeyRef
        )

        print("⌨️ L.E.O. global hotkeys active: [⌥ Space], [⌘⇧S Stop], [⌘⇧H Help]")
    }
}
