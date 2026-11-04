# 🏆 MacPilot AI — Meta Global AI Developer Hackathon 2026

> **An Autonomous, Real-Time macOS Copilot Powered by Meta AI Muse Wonder 1.0, Live Web Search, and Native Computer Control.**

---

## 🌟 The Vision
Current AI assistants are trapped in browser tabs or text chats. **MacPilot AI** liberates AI to directly navigate, operate, inspect, and automate the MacBook operating system in real-time — speaking with a human neural voice, searching the live internet, and executing tasks across apps with evidence-based verification.

---

## 🧠 Meta AI Frontier Brain Architecture

MacPilot AI implements an event-driven **OODA (Observe, Orient, Decide, Act, Verify)** cognitive loop directly powered by **Meta AI Muse Wonder 1.0** (with 500+ reasoning tokens per decision):

```
       ┌────────────────────────────────────────────────────────┐
       │                   USER SPEECH INPUT                    │
       │    (On-Device Neural Speech Recognition: <180ms)      │
       └───────────────────────────┬────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │             META AI MUSE WONDER 1.0 BRAIN              │
       │     (Autonomous Step Planning & Natural Speech Gen)    │
       └───────────────────────────┬────────────────────────────┘
                                   │
            ┌──────────────────────┴──────────────────────┐
            ▼                                             ▼
 ┌──────────────────────┐                     ┌──────────────────────┐
 │    REAL-TIME VOICE   │                     │  COMPUTER EXECUTION  │
 │  (Human Neural TTS)  │                     │   (Go Agent Engine)  │
 └──────────────────────┘                     └──────────┬───────────┘
                                                         │
                ┌────────────────────────────────────────┼────────────────────────────────────────┐
                ▼                                        ▼                                        ▼
      ┌──────────────────┐                     ┌──────────────────┐                     ┌──────────────────┐
      │ LIVE WEB SEARCH  │                     │  MAC OS CONTROL  │                     │ SENTINEL SAFETY  │
      │ (DuckDuckGo Lite)│                     │(Apps, Shell, AX) │                     │ (Human Approval) │
      └──────────────────┘                     └──────────────────┘                     └──────────────────┘
```

---

## 🚀 Key Capabilities Demonstrated

### 1. 🌐 Autonomous Real-Time Web Search
- Queries the live internet in real-time with zero API key dependencies.
- Synthesizes fresh news, documentation, and facts with source citations.
- **Example**: *"Search latest news on Meta AI Hackathon 2026"*
  - *Agent Output*: Discovers the $1M prize pool, virtual dates, and $150 Meta Model API credits, and reads out the verified summary.

### 2. 💻 Deep MacBook Computer Control
- **App Management**: Opens and controls Chrome, Safari, Finder, Terminal, Calculator, etc.
- **System Telemetry**: Reads battery percentage, frontmost app name, volume, and screen metrics.
- **Spotlight File Search**: Finds files across the Mac in <10ms via `mdfind`.
- **Media Automation**: Searches and autoplays music and videos on YouTube.
- **Example**: *"Check my MacBook battery and frontmost application"*
  - *Agent Output*: *"All checked! Your MacBook battery is at 28%, and your frontmost app is Electron."*

### 3. 🎙️ Natural Human Voice & Speech
- **Studio-Grade Human Voice**: Replaced legacy robotic synthesizers with Microsoft Neural and Apple Siri Premium Neural voices.
- **On-Device Whisper / Neural Speech Recognition**: Speech is transcribed locally on the Apple Silicon Neural Engine without cloud latency.
- **Full-Duplex Listening**: The microphone remains active for continuous multi-turn conversations.

### 4. 🛡️ Enterprise Sentinel Safety Policy
- Commands matching destructive patterns (`rm -rf /`, `mkfs`, fork bombs) are instantly blocked.
- High-consequence commands (`sudo`, `killall`, `rm -f`) pause execution and require explicit approval through the on-screen `ApprovalView` card.
- Exit codes, file existence, and process trees are verified before any task is marked complete.

---

## 🎬 3-Minute Hackathon Demo Script

### Demo 1: Conversational Companion
- **Trigger**: Double-tap `Fn` or click the floating widget at the top of your Mac.
- **Say**: *"Hey MacPilot, what can you do on my laptop?"*
- **Outcome**: Meta AI speaks back in natural human voice explaining its autonomous capabilities.

### Demo 2: Autonomous Live Web Search
- **Say**: *"Search latest news on Meta AI Hackathon 2026"*
- **Outcome**: The agent triggers `web_search`, retrieves live internet search results, synthesizes details (10 days, $1M prize pool, $150 API credits), and speaks the answer.

### Demo 3: MacBook Hardware & System Telemetry
- **Say**: *"Check my MacBook battery and frontmost application"*
- **Outcome**: The agent inspects system battery via `pmset` and frontmost app via AppleScript, and gives advice ("Battery is at 28%, you might want to plug in soon").

### Demo 4: Media & Computer Control
- **Say**: *"Open Chrome and play the weekend song"*
- **Outcome**: Opens Google Chrome and plays the song with autoplay.

---

## 🏃 Running the Application

```bash
cd "MacPilotAI"
./scripts/run.sh
```

**Shortcuts**:
- **Double-Tap `Fn`**: Toggle voice listening from any application.
- **`⌥ Space` (Option + Space)**: Open or dismiss floating HUD capsule.

