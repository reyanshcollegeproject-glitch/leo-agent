#!/bin/bash
set -e

WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAC_PILOT_DIR="$WORKSPACE_DIR/MacPilotAI"

echo "=========================================================="
echo "🦁 Building L.E.O. - One Unified Desktop AI System"
echo "   Live Executive Operator for macOS"
echo "=========================================================="

# 1. Compile Native Input Controller (macpilot-input / leo-input)
echo "📦 [1/4] Compiling Native Input Controller (CoreGraphics HID)..."
mkdir -p "$MAC_PILOT_DIR/services/agent-engine/bin"
swiftc -O "$MAC_PILOT_DIR/packages/input-controller/main.swift" -o "$MAC_PILOT_DIR/services/agent-engine/bin/leo-input"
cp "$MAC_PILOT_DIR/services/agent-engine/bin/leo-input" "$MAC_PILOT_DIR/services/agent-engine/bin/macpilot-input"
echo "✅ Native Input Controller compiled: leo-input & macpilot-input"

# 2. Compile Go Agent Engine (Meta AI multi-model brain + Computer Vision + Controller)
echo "📦 [2/4] Compiling Go Agent Engine..."
cd "$MAC_PILOT_DIR/services/agent-engine"
go build -o bin/leo-engine cmd/macpilot/main.go
cp bin/leo-engine bin/macpilot
echo "✅ Go Agent Engine compiled: leo-engine & macpilot"

# 3. Compile Swift Desktop Release App
echo "📦 [3/4] Compiling Swift macOS Floating UI (Release)..."
cd "$MAC_PILOT_DIR/apps/macos"
swift build -c release
echo "✅ Swift app compiled."

# 4. Package and Code-sign Unified LEO.app
echo "📦 [4/4] Packaging unified LEO.app..."
cd "$MAC_PILOT_DIR"
bash scripts/package_app.sh

echo ""
echo "=========================================================="
echo "🎉 Build Complete: LEO.app is ready at:"
echo "   $WORKSPACE_DIR/LEO.app"
echo "=========================================================="

