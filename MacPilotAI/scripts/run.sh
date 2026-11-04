#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
APP_PATH="$REPO_ROOT/MacPilot.app"
ENGINE_BIN="$REPO_ROOT/services/agent-engine/bin/macpilot"

if [ ! -d "$APP_PATH" ] || [ ! -f "$ENGINE_BIN" ]; then
    echo "⚠️ Binaries missing. Running build.sh first..."
    "$SCRIPT_DIR/build.sh"
fi

# Ensure Agent Engine is running on port 8089
if ! curl -s http://127.0.0.1:8089/api/v1/health >/dev/null 2>&1; then
    echo "⚡ Starting MacPilot AI Engine daemon on port 8089..."
    cd "$REPO_ROOT"
    nohup "$ENGINE_BIN" --port 8089 > /tmp/macpilot_engine.log 2>&1 &
    sleep 0.8
    if curl -s http://127.0.0.1:8089/api/v1/health >/dev/null 2>&1; then
        echo "✅ Agent Engine is healthy and listening on http://127.0.0.1:8089"
    else
        echo "⚠️ Engine starting, check /tmp/macpilot_engine.log"
    fi
else
    echo "✅ MacPilot AI Agent Engine already active on http://127.0.0.1:8089"
fi

echo "=========================================================="
echo "🚀 Launching MacPilot AI Desktop Application"
echo "=========================================================="
echo "💡 Shortcuts & Controls:"
echo "   👉 Double-Tap [Fn] key anywhere in macOS to activate/talk"
echo "   👉 Press [⌥ Space] (Option + Space) to trigger"
echo "   👉 Menu Bar icon at the top of your screen"
echo "   👉 Floating Executive Widget docked at the top of macOS"
echo "=========================================================="

# Relaunch app cleanly
killall MacPilot 2>/dev/null || true
open "$APP_PATH"

echo "✅ MacPilot AI is running!"
