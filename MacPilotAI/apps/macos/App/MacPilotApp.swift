import SwiftUI
import AppKit

@main
struct MacPilotApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @StateObject private var environment = AppEnvironment()

    var body: some Scene {
        WindowGroup {
            ChatView()
                .environmentObject(environment)
                .background(WindowConfigurator())
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 760, height: 56)
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    static var shared: AppDelegate?
    weak var mainWindow: NSWindow?
    var computerWindow: NSWindow?
    var statusItem: NSStatusItem?

    func applicationDidFinishLaunching(_ notification: Notification) {
        AppDelegate.shared = self
        NSApp.setActivationPolicy(.accessory) // Floating macOS accessory widget
        AppLifecycleManager.shared.startEngineIfNeeded()
        setupStatusItem()
    }

    func applicationWillTerminate(_ notification: Notification) {
        AppLifecycleManager.shared.stopEngine()
    }

    private func setupStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = statusItem?.button {
            button.image = NSImage(systemSymbolName: "bolt.circle.fill", accessibilityDescription: "L.E.O.")
            button.action = #selector(toggleWidgetWindow)
            button.target = self
        }

        let menu = NSMenu()
        let toggleItem = NSMenuItem(title: "Toggle L.E.O. Capsule (⌥ Space)", action: #selector(toggleWidgetWindow), keyEquivalent: "")
        toggleItem.target = self
        menu.addItem(toggleItem)

        let computerItem = NSMenuItem(title: "🖥️ AI Virtual Computer Desktop", action: #selector(showComputerWindow), keyEquivalent: "c")
        computerItem.target = self
        menu.addItem(computerItem)

        menu.addItem(NSMenuItem.separator())
        let quitItem = NSMenuItem(title: "Quit L.E.O.", action: #selector(quitApp), keyEquivalent: "q")
        quitItem.target = self
        menu.addItem(quitItem)
        statusItem?.menu = menu
    }

    @MainActor
    @objc func showComputerWindow() {
        if let win = computerWindow {
            win.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }

        let win = NSWindow(
            contentRect: NSRect(x: 120, y: 120, width: 980, height: 680),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        win.title = "L.E.O. Dedicated Virtual Computer"
        win.titlebarAppearsTransparent = true
        win.backgroundColor = NSColor(red: 0.08, green: 0.09, blue: 0.12, alpha: 1.0)
        win.center()
        win.isReleasedWhenClosed = false

        if let env = AppEnvironment.shared {
            let hosting = NSHostingView(rootView: AIComputerView().environmentObject(env))
            win.contentView = hosting
        }

        self.computerWindow = win
        win.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @objc func toggleWidgetWindow() {
        guard let window = mainWindow else { return }
        if window.isVisible && window.isKeyWindow {
            window.orderOut(nil)
        } else {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
        }
    }

    @objc func quitApp() {
        NSApp.terminate(nil)
    }
}

struct WindowConfigurator: NSViewRepresentable {
    func makeNSView(context: Context) -> NSView {
        let view = NSView()
        DispatchQueue.main.async {
            guard let window = view.window, let screen = NSScreen.main else { return }
            AppDelegate.shared?.mainWindow = window

            window.title = "L.E.O."
            window.titleVisibility = .hidden
            window.titlebarAppearsTransparent = true
            window.isMovableByWindowBackground = true
            window.backgroundColor = .clear
            window.isOpaque = false
            window.hasShadow = false
            window.level = .floating
            window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]

            let screenFrame = screen.visibleFrame
            let width: CGFloat = 760
            let height: CGFloat = 56
            let x = screenFrame.origin.x + (screenFrame.width - width) / 2
            let y = screenFrame.origin.y + screenFrame.height - height - 10

            let frame = NSRect(x: x, y: y, width: width, height: height)
            window.setFrame(frame, display: true, animate: false)
            window.makeKeyAndOrderFront(nil)
        }
        return view
    }

    func updateNSView(_ nsView: NSView, context: Context) {}
}
