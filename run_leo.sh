#!/bin/bash

WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_PATH="$WORKSPACE_DIR/LEO.app"

echo "=========================================================="
echo "🚀 Launching L.E.O. (Live Executive Operator)"
echo "=========================================================="

# 1. Clean up any existing instances
pkill -9 LEO 2>/dev/null || true
pkill -9 macpilot 2>/dev/null || true
pkill -9 leo-engine 2>/dev/null || true

# 2. Check that LEO.app exists
if [ ! -d "$APP_PATH" ]; then
    echo "⚠️ LEO.app not found. Running build_leo.sh first..."
    bash "$WORKSPACE_DIR/build_leo.sh"
fi

# 3. Open LEO.app
echo "✨ Starting L.E.O.app..."
open "$APP_PATH"

# 4. Wait for engine backend to become healthy
echo "⏳ Waiting for L.E.O. agent engine on 127.0.0.1:8089..."
for i in {1..30}; do
    if curl -s http://127.0.0.1:8089/api/v1/health >/dev/null 2>&1; then
        echo "✅ L.E.O. Agent Engine is online and healthy!"
        echo ""
        echo "=========================================================="
        echo "🦁 L.E.O. is active on your MacBook!"
        echo "   • Floating top capsule: ⌥ Space to toggle"
        echo "   • Studio Voice: Push-to-Talk (Mic button or Double-tap Fn)"
        echo "   • Computer Vision: Eye button or say 'Look at my screen'"
        echo "   • Direct Mouse Control: 'move mouse to center', 'click', 'scroll'"
        echo "   • Virtual Computer: Click 🖥️ or say 'start computer'"
        echo "   • To stop cleanly: bash stop_leo.sh"
        echo "=========================================================="
        exit 0
    fi
    sleep 0.5
done

echo "ℹ️ LEO.app launched. UI is active in the menu bar."

