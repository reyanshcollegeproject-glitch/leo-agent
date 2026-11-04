# MacPilot AI 🚀
### Production Autonomous macOS Computer-Control & Real-Time Voice Agent

MacPilot AI is an industry-grade autonomous macOS laptop agent designed with an event-driven **SwiftUI (Native macOS Desktop App) + Go (Agent Engine) + SQLite (Persistence)** architecture. 

It docks seamlessly at the top of your MacBook screen as a floating executive capsule, listens and talks in real time with continuous sub-50ms vocal responses, and controls applications (Finder, Chrome, Terminal, system settings) under a strict Sentinel policy engine with human approval for consequential operations.

---

## 🎨 Executive Design System (Strictly Zero Neon)
Built specifically following executive, distraction-free macOS ergonomics:
- **Obsidian Black**: `#0A0D14` (Window background and HUD capsule)
- **Charcoal Card**: `#141A24` (Telemetry and message cards)
- **Graphite Border**: `#2E3847` (Precision 1px container boundaries)
- **Studio White**: `#FFFFFF` (Crisp text and primary visual elements)
- **Apple Royal Blue**: `#007AFF` (Accents, active speech wave, and confirmation buttons)
- **Strictly Zero Neon Colors**: No cyan, neon lime, or high-saturation glow.

---

## ⚡ Real-Time Voice & Execution Architecture
Unlike turn-based agents that record 10 seconds of audio into a file, transcribe slowly, and wait for an LLM response before speaking:
1. **Live Microphone Buffer Streaming**: `AVAudioEngine` installs an audio tap streaming 1024-sample PCM buffers with real-time RMS metering directly feeding an animated 7-bar audio visualizer.
2. **Instant Live Transcription**: Interim speech recognition results update live on screen word-for-word as you speak.
3. **Auto-Silence Execution**: Automatic silence detection triggers execution within 1.2s of your last word (or via manual Enter / tap).
4. **Instant Spoken Voice**: The agent responds vocally in real time via low-latency `AVSpeechSynthesizer` ("On it. Opening Chrome and playing your song", "Desktop opened in Finder", "Done.") without disk writing lag.

---

## ⌨️ Global Shortcuts & Activation
- **Double-Tap `Fn` (Globe) Key**: Instantly toggles the MacPilot voice listening and brings the widget to front from any application on macOS.
- **`⌥ Space` (Option + Space)**: Global Carbon hotkey trigger.
- **macOS Menu Bar**: Persistent status bar icon (`bolt.circle.fill`) with one-click HUD toggle and quit options.
- **Top-of-Mac Floating Widget**: Draggable, stays floating above all full-screen spaces and desktops.

---

## 📂 Monorepo Structure

```text
MacPilotAI/
├── apps/
│   └── macos/                      # Native Swift + SwiftUI Desktop Application
│       ├── App/                    # App lifecycle, NSApplicationDelegate, AppEnvironment
│       ├── Presentation/           # Executive SwiftUI Cockpit & Floating Capsule
│       │   ├── Chat/               # ChatView, ChatViewModel, MessageRowView
│       │   ├── Approvals/          # Human-in-the-loop ApprovalView
│       │   └── Components/         # Theme, WaveformVisualizerView
│       ├── Infrastructure/         # VoiceManager, AgentClient, EventStreamClient
│       ├── Domain/                 # TaskItem, TaskStepItem, ApprovalItem models
│       └── Platform/               # HotKeyManager, PermissionManager
│
├── services/
│   └── agent-engine/               # Pure Go Cognitive & Execution Engine
│       ├── cmd/macpilot/           # Engine entrypoint & CLI server
│       ├── internal/
│       │   ├── gateway/            # HTTP REST + WebSocket event streaming
│       │   ├── orchestrator/       # OODA Cognitive Loop
│       │   ├── policy/             # Sentinel policy engine (Human-in-the-loop)
│       │   ├── approvals/          # Thread-safe approval channels
│       │   ├── computer/           # macOS AppleScript & Shell controller
│       │   ├── providers/          # Multi-provider AI gateway (Meta Muse Wonder 1.0)
│       │   ├── persistence/        # Pure Go SQLite storage (modernc.org/sqlite)
│       │   └── verification/       # Action verifier (Exit codes, disk state, pgrep)
│       └── tests/                  # Engine unit & integration tests
│
├── scripts/
│   ├── build.sh                    # Compiles Go binary, Swift binary, and packages MacPilot.app
│   ├── package_app.sh              # Assembles signed .app bundle with Info.plist entitlements
│   └── run.sh                      # One-click launcher
│
└── MacPilot.app                    # Signed native macOS application bundle
```

---

## 🛡️ Security & Sentinel Policy Engine

The AI model **never directly executes arbitrary system commands**. All proposed actions pass through the Go Sentinel Policy Engine:
1. **SAFE Actions**: Harmless lookups, Finder views, Chrome navigation, and volume controls execute automatically.
2. **CONSEQUENTIAL Actions**: Operations matching destructive patterns (`rm -f`, `sudo`, `killall`, `chmod -R`) suspend execution and trigger a high-severity `ApprovalView` in the UI and spoken voice warning.
3. **BLOCKED Actions**: Catastrophic commands (`rm -rf /`, `mkfs`, fork bombs) are instantly blocked with zero execution.

---

## 🚀 Quick Start

### 1. Build Both Subsystems & Package App
```bash
cd MacPilotAI
./scripts/build.sh
```

### 2. Run MacPilot AI
```bash
./scripts/run.sh
```
Or open `MacPilot.app` directly from Finder.

### 3. Try Spoken or Typed Commands
- *"open chrome and play weekend song"*
- *"open desktop in finder"*
- *"calculate 512 * 128"*
- *"mute volume"*
- *"open terminal"*

