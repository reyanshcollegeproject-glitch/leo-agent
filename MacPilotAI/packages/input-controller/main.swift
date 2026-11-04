import Foundation
import CoreGraphics
import AppKit

// AI Space resolution (standard Anthropic computer-use dimensions)
let AI_WIDTH: Double = 1280.0
let AI_HEIGHT: Double = 800.0

func getScreenDimensions() -> (Double, Double) {
    if let screen = NSScreen.main {
        let frame = screen.frame
        return (Double(frame.width), Double(frame.height))
    }
    return (1440.0, 900.0) // standard fallback
}

func mapFromAISpace(x: Double, y: Double) -> CGPoint {
    let (screenWidth, screenHeight) = getScreenDimensions()
    let mappedX = (x * screenWidth) / AI_WIDTH
    let mappedY = (y * screenHeight) / AI_HEIGHT
    return CGPoint(x: mappedX, y: mappedY)
}

func parseNamedTarget(_ name: String) -> CGPoint? {
    let (w, h) = getScreenDimensions()
    switch name.lowercased() {
    case "center", "middle":
        return CGPoint(x: w / 2.0, y: h / 2.0)
    case "top_left", "topleft", "top-left":
        return CGPoint(x: 100.0, y: 100.0)
    case "top_right", "topright", "top-right":
        return CGPoint(x: max(100.0, w - 100.0), y: 100.0)
    case "bottom_left", "bottomleft", "bottom-left":
        return CGPoint(x: 100.0, y: max(100.0, h - 100.0))
    case "bottom_right", "bottomright", "bottom-right":
        return CGPoint(x: max(100.0, w - 100.0), y: max(100.0, h - 100.0))
    case "menu_bar", "menubar":
        return CGPoint(x: w / 2.0, y: 15.0)
    case "dock":
        return CGPoint(x: w / 2.0, y: max(20.0, h - 35.0))
    default:
        return nil
    }
}

func getCurrentCursor() -> CGPoint {
    return CGEvent(source: nil)?.location ?? CGPoint(x: 640.0, y: 400.0)
}

func moveMouse(to point: CGPoint, smooth: Bool = true) {
    if smooth {
        let current = getCurrentCursor()
        let steps = 14
        for i in 1...steps {
            let t = Double(i) / Double(steps)
            let curX = current.x + (point.x - current.x) * t
            let curY = current.y + (point.y - current.y) * t
            let event = CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: CGPoint(x: curX, y: curY), mouseButton: .left)
            event?.post(tap: .cghidEventTap)
            usleep(4000) // 4ms per step (~56ms total human-like glide)
        }
    }
    let finalEvent = CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left)
    finalEvent?.post(tap: .cghidEventTap)
}

func leftClick(at point: CGPoint) {
    moveMouse(to: point, smooth: true)
    usleep(20000)
    let down = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)
    let up = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)
    down?.post(tap: .cghidEventTap)
    usleep(30000)
    up?.post(tap: .cghidEventTap)
}

func rightClick(at point: CGPoint) {
    moveMouse(to: point, smooth: true)
    usleep(20000)
    let down = CGEvent(mouseEventSource: nil, mouseType: .rightMouseDown, mouseCursorPosition: point, mouseButton: .right)
    let up = CGEvent(mouseEventSource: nil, mouseType: .rightMouseUp, mouseCursorPosition: point, mouseButton: .right)
    down?.post(tap: .cghidEventTap)
    usleep(30000)
    up?.post(tap: .cghidEventTap)
}

func doubleClick(at point: CGPoint) {
    moveMouse(to: point, smooth: true)
    usleep(20000)
    let down1 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)
    down1?.setIntegerValueField(.mouseEventClickState, value: 1)
    let up1 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)
    up1?.setIntegerValueField(.mouseEventClickState, value: 1)

    let down2 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)
    down2?.setIntegerValueField(.mouseEventClickState, value: 2)
    let up2 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)
    up2?.setIntegerValueField(.mouseEventClickState, value: 2)

    down1?.post(tap: .cghidEventTap)
    up1?.post(tap: .cghidEventTap)
    usleep(40000)
    down2?.post(tap: .cghidEventTap)
    up2?.post(tap: .cghidEventTap)
}

func tripleClick(at point: CGPoint) {
    doubleClick(at: point)
    usleep(40000)
    let down3 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)
    down3?.setIntegerValueField(.mouseEventClickState, value: 3)
    let up3 = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)
    up3?.setIntegerValueField(.mouseEventClickState, value: 3)
    down3?.post(tap: .cghidEventTap)
    up3?.post(tap: .cghidEventTap)
}

func middleClick(at point: CGPoint) {
    moveMouse(to: point, smooth: true)
    usleep(20000)
    let down = CGEvent(mouseEventSource: nil, mouseType: .otherMouseDown, mouseCursorPosition: point, mouseButton: .center)
    let up = CGEvent(mouseEventSource: nil, mouseType: .otherMouseUp, mouseCursorPosition: point, mouseButton: .center)
    down?.post(tap: .cghidEventTap)
    usleep(30000)
    up?.post(tap: .cghidEventTap)
}

func mouseDrag(from start: CGPoint, to end: CGPoint) {
    moveMouse(to: start, smooth: true)
    usleep(40000)
    let down = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: start, mouseButton: .left)
    down?.post(tap: .cghidEventTap)
    usleep(40000)

    // Smooth drag steps
    let steps = 18
    for i in 1...steps {
        let t = Double(i) / Double(steps)
        let curX = start.x + (end.x - start.x) * t
        let curY = start.y + (end.y - start.y) * t
        let drag = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDragged, mouseCursorPosition: CGPoint(x: curX, y: curY), mouseButton: .left)
        drag?.post(tap: .cghidEventTap)
        usleep(6000)
    }

    let up = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: end, mouseButton: .left)
    up?.post(tap: .cghidEventTap)
}

func scroll(direction: String, amount: Int32) {
    let units = (direction.lowercased() == "up" ? amount : -amount) * 3
    let scrollEvent = CGEvent(scrollWheelEvent2Source: nil, units: .line, wheelCount: 1, wheel1: units, wheel2: 0, wheel3: 0)
    scrollEvent?.post(tap: .cghidEventTap)
}

func pressKeyCombo(combo: String) {
    let parts = combo.lowercased().split(separator: "+").map { String($0).trimmingCharacters(in: .whitespaces) }
    var flags: CGEventFlags = []
    var mainKey = ""

    for part in parts {
        switch part {
        case "cmd", "command", "super":
            flags.insert(.maskCommand)
        case "ctrl", "control":
            flags.insert(.maskControl)
        case "alt", "option", "opt":
            flags.insert(.maskAlternate)
        case "shift":
            flags.insert(.maskShift)
        default:
            mainKey = part
        }
    }

    let keyMap: [String: CGKeyCode] = [
        "return": 0x24, "enter": 0x24,
        "tab": 0x30,
        "space": 0x31,
        "delete": 0x33, "backspace": 0x33,
        "escape": 0x35, "esc": 0x35,
        "up": 0x7E, "down": 0x7D, "left": 0x7B, "right": 0x7C,
        "a": 0x00, "s": 0x01, "d": 0x02, "f": 0x03, "h": 0x04, "g": 0x05, "z": 0x06, "x": 0x07, "c": 0x08, "v": 0x09,
        "b": 0x0B, "q": 0x0C, "w": 0x0D, "e": 0x0E, "r": 0x0F, "y": 0x10, "t": 0x11, "1": 0x12, "2": 0x13, "3": 0x14,
        "4": 0x15, "6": 0x16, "5": 0x17, "=": 0x18, "9": 0x19, "7": 0x1A, "-": 0x1B, "8": 0x1C, "0": 0x1D, "]": 0x1E,
        "o": 0x1F, "u": 0x20, "[": 0x21, "i": 0x22, "p": 0x23, "l": 0x25, "j": 0x26, "'": 0x27, "k": 0x28, ";": 0x29,
        "\\": 0x2A, ",": 0x2B, "/": 0x2C, "n": 0x2D, "m": 0x2E, ".": 0x2F
    ]

    if let code = keyMap[mainKey] {
        let down = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: true)
        down?.flags = flags
        let up = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: false)
        up?.flags = flags
        down?.post(tap: .cghidEventTap)
        usleep(15000)
        up?.post(tap: .cghidEventTap)
    }
}

func typeText(text: String) {
    let utf16Chars = Array(text.utf16)
    for char in utf16Chars {
        var unichar = char
        if let down = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: true) {
            down.keyboardSetUnicodeString(stringLength: 1, unicodeString: &unichar)
            down.post(tap: .cghidEventTap)
        }
        if let up = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: false) {
            up.keyboardSetUnicodeString(stringLength: 1, unicodeString: &unichar)
            up.post(tap: .cghidEventTap)
        }
        usleep(3500)
    }
}

func listOpenWindows() -> String {
    guard let windowList = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
        return "[]"
    }
    var results: [[String: Any]] = []
    for win in windowList {
        let app = win[kCGWindowOwnerName as String] as? String ?? ""
        let title = win[kCGWindowName as String] as? String ?? ""
        let pid = win[kCGWindowOwnerPID as String] as? Int ?? 0
        let layer = win[kCGWindowLayer as String] as? Int ?? 0
        if layer == 0 && (!app.isEmpty || !title.isEmpty) {
            results.append([
                "app": app,
                "title": title,
                "pid": pid
            ])
        }
    }
    if let data = try? JSONSerialization.data(withJSONObject: results),
       let json = String(data: data, encoding: .utf8) {
        return json
    }
    return "[]"
}

// CLI Dispatcher
let args = CommandLine.arguments
guard args.count > 1 else {
    let (w, h) = getScreenDimensions()
    print("{\"width\": \(w), \"height\": \(h)}")
    exit(0)
}

let action = args[1].lowercased()

switch action {
case "dimensions":
    let (w, h) = getScreenDimensions()
    print("{\"width\": \(w), \"height\": \(h)}")

case "cursor":
    let cur = getCurrentCursor()
    print("{\"x\": \(cur.x), \"y\": \(cur.y)}")

case "windows":
    print(listOpenWindows())

case "move":
    if args.count == 3 {
        // Named target: "move center", "move top_left"
        if let pt = parseNamedTarget(args[2]) {
            moveMouse(to: pt, smooth: true)
            print("{\"status\": \"moved\", \"x\": \(pt.x), \"y\": \(pt.y)}")
        } else {
            print("{\"error\": \"unknown_target\", \"target\": \"\(args[2])\"}")
        }
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        moveMouse(to: pt, smooth: true)
        print("{\"status\": \"moved\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "raw_move":
    if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = CGPoint(x: x, y: y)
        moveMouse(to: pt, smooth: true)
        print("{\"status\": \"moved\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "click", "left_click":
    if args.count == 2 {
        let pt = getCurrentCursor()
        leftClick(at: pt)
        print("{\"status\": \"clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count == 3, let pt = parseNamedTarget(args[2]) {
        leftClick(at: pt)
        print("{\"status\": \"clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        leftClick(at: pt)
        print("{\"status\": \"clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "right_click":
    if args.count == 2 {
        let pt = getCurrentCursor()
        rightClick(at: pt)
        print("{\"status\": \"right_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        rightClick(at: pt)
        print("{\"status\": \"right_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "double_click":
    if args.count == 2 {
        let pt = getCurrentCursor()
        doubleClick(at: pt)
        print("{\"status\": \"double_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        doubleClick(at: pt)
        print("{\"status\": \"double_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "triple_click":
    if args.count == 2 {
        let pt = getCurrentCursor()
        tripleClick(at: pt)
        print("{\"status\": \"triple_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        tripleClick(at: pt)
        print("{\"status\": \"triple_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "middle_click":
    if args.count == 2 {
        let pt = getCurrentCursor()
        middleClick(at: pt)
        print("{\"status\": \"middle_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    } else if args.count >= 4, let x = Double(args[2]), let y = Double(args[3]) {
        let pt = mapFromAISpace(x: x, y: y)
        middleClick(at: pt)
        print("{\"status\": \"middle_clicked\", \"x\": \(pt.x), \"y\": \(pt.y)}")
    }

case "drag", "left_click_drag":
    if args.count >= 6,
       let sx = Double(args[2]), let sy = Double(args[3]),
       let ex = Double(args[4]), let ey = Double(args[5]) {
        let startPt = mapFromAISpace(x: sx, y: sy)
        let endPt = mapFromAISpace(x: ex, y: ey)
        mouseDrag(from: startPt, to: endPt)
        print("{\"status\": \"dragged\"}")
    }

case "scroll":
    let dir = args.count >= 3 ? args[2] : "down"
    let amt = args.count >= 4 ? (Int32(args[3]) ?? 3) : 3
    scroll(direction: dir, amount: amt)
    print("{\"status\": \"scrolled\", \"direction\": \"\(dir)\"}")

case "key":
    if args.count >= 3 {
        let combo = args[2]
        pressKeyCombo(combo: combo)
        print("{\"status\": \"key_pressed\", \"key\": \"\(combo)\"}")
    }

case "type":
    if args.count >= 3 {
        let text = args[2...].joined(separator: " ")
        typeText(text: text)
        print("{\"status\": \"typed\"}")
    }

case "screenshot":
    let outPath = args.count >= 3 ? args[2] : "/tmp/leo_screenshot_\(Int(Date().timeIntervalSince1970)).png"
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    p.arguments = ["-x", "-t", "png", outPath]
    try? p.run()
    p.waitUntilExit()
    print("{\"status\": \"captured\", \"path\": \"\(outPath)\"}")

case "launch":
    if args.count >= 3 {
        let app = args[2...].joined(separator: " ")
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        p.arguments = ["-a", app]
        try? p.run()
        p.waitUntilExit()
        print("{\"status\": \"launched\", \"app\": \"\(app)\"}")
    }

default:
    print("{\"error\": \"unknown_action\", \"action\": \"\(action)\"}")
}


