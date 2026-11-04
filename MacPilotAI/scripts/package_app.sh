#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SWIFT_DIR="$REPO_ROOT/apps/macos"
ENGINE_DIR="$REPO_ROOT/services/agent-engine"

echo "=========================================================="
echo "🚀 Building L.E.O. MacPilot AI Production Release App Bundle"
echo "=========================================================="

# 1. Build Native Input Controller Binary (Mouse, Keyboard, Screen Mapping)
echo "📦 Step 1a: Compiling Native Input Controller (macpilot-input)..."
mkdir -p "$ENGINE_DIR/bin"
swiftc -O "$REPO_ROOT/packages/input-controller/main.swift" -o "$ENGINE_DIR/bin/macpilot-input"
echo "✅ Native Input Controller ready: $ENGINE_DIR/bin/macpilot-input"

# 1b. Build Go Agent Engine Binary
echo "📦 Step 1b: Compiling Go Agent Engine..."
cd "$ENGINE_DIR"
go build -o "$ENGINE_DIR/bin/macpilot" cmd/macpilot/main.go
echo "✅ Go Agent Engine binary ready: $ENGINE_DIR/bin/macpilot"

# 2. Build Swift Desktop App Release Binary
echo "📦 Step 2: Compiling Swift macOS Desktop App (Release)..."
cd "$SWIFT_DIR"
swift build -c release
SWIFT_BIN="$SWIFT_DIR/.build/release/MacPilot"

if [ ! -f "$SWIFT_BIN" ]; then
    echo "❌ Swift release binary not found at $SWIFT_BIN"
    exit 1
fi
echo "✅ Swift release binary built: $SWIFT_BIN"

# 3. Create .app Bundle Layout
APP_BUNDLE="$REPO_ROOT/LEO.app"
echo "📦 Step 3: Assembling $APP_BUNDLE..."
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources/bin"

# Copy Swift Binary
cp "$SWIFT_BIN" "$APP_BUNDLE/Contents/MacOS/LEO"
chmod +x "$APP_BUNDLE/Contents/MacOS/LEO"

# Copy Go Engine Binary & Input Controller into Resources/bin
cp "$ENGINE_DIR/bin/macpilot" "$APP_BUNDLE/Contents/Resources/bin/leo-engine"
cp "$ENGINE_DIR/bin/macpilot" "$APP_BUNDLE/Contents/Resources/bin/macpilot"
chmod +x "$APP_BUNDLE/Contents/Resources/bin/leo-engine"
chmod +x "$APP_BUNDLE/Contents/Resources/bin/macpilot"

cp "$ENGINE_DIR/bin/macpilot-input" "$APP_BUNDLE/Contents/Resources/bin/leo-input"
cp "$ENGINE_DIR/bin/macpilot-input" "$APP_BUNDLE/Contents/Resources/bin/macpilot-input"
chmod +x "$APP_BUNDLE/Contents/Resources/bin/leo-input"
chmod +x "$APP_BUNDLE/Contents/Resources/bin/macpilot-input"

# Copy noVNC web assets
if [ -d "$ENGINE_DIR/web" ]; then
    mkdir -p "$APP_BUNDLE/Contents/Resources/web"
    cp -R "$ENGINE_DIR/web/"* "$APP_BUNDLE/Contents/Resources/web/"
fi

# Copy Human Speech Script & Audio Assets
cp "$REPO_ROOT/scripts/speak_human.sh" "$APP_BUNDLE/Contents/Resources/speak_human.sh"
chmod +x "$APP_BUNDLE/Contents/Resources/speak_human.sh"

if [ -f "$REPO_ROOT/../.env" ]; then
    cp "$REPO_ROOT/../.env" "$APP_BUNDLE/Contents/Resources/.env"
fi

if [ -d "$REPO_ROOT/apps/macos/Resources/audio" ]; then
    mkdir -p "$APP_BUNDLE/Contents/Resources/audio"
    cp -R "$REPO_ROOT/apps/macos/Resources/audio/"* "$APP_BUNDLE/Contents/Resources/audio/"
fi

# Write Info.plist
cat <<EOF > "$APP_BUNDLE/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>LEO</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>ai.leo.desktop</string>
    <key>CFBundleName</key>
    <string>LEO</string>
    <key>CFBundleDisplayName</key>
    <string>L.E.O.</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSpeechRecognitionUsageDescription</key>
    <string>L.E.O. transcribes your spoken instructions into real-time macOS automation actions.</string>
    <key>NSMicrophoneUsageDescription</key>
    <string>L.E.O. uses your microphone for real-time voice commands and autonomous computer control.</string>
    <key>NSScreenCaptureUsageDescription</key>
    <string>L.E.O. uses ScreenCaptureKit and Apple Vision to see your screen in real time and converse about what you are working on.</string>
    <key>NSAppleEventsUsageDescription</key>
    <string>L.E.O. requires Apple Events automation to control Finder, Chrome, Terminal, and system applications.</string>
</dict>
</plist>
EOF

# 4. Ad-Hoc Code Sign with Entitlements
echo "📦 Step 4: Code Signing LEO.app with TCC entitlements..."
codesign -s - --force --deep --identifier "ai.leo.desktop" "$APP_BUNDLE"

PARENT_BUNDLE="$(cd "$REPO_ROOT/.." && pwd)/LEO.app"
echo "📦 Step 5: Copying unified bundle to workspace root: $PARENT_BUNDLE"
rm -rf "$PARENT_BUNDLE"
cp -R "$APP_BUNDLE" "$PARENT_BUNDLE"
codesign -s - --force --deep --identifier "ai.leo.desktop" "$PARENT_BUNDLE"

# Cleanup any old redundant MacPilot.app
rm -rf "$REPO_ROOT/MacPilot.app"

echo "=========================================================="
echo "🎉 L.E.O.app created and signed successfully!"
echo "📍 Location: $PARENT_BUNDLE"
echo "=========================================================="
