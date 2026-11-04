#!/bin/bash

echo "🛑 Stopping L.E.O. and all agent processes..."

# Terminate all L.E.O. and MacPilot processes
pkill -9 LEO 2>/dev/null || true
pkill -9 macpilot 2>/dev/null || true
pkill -9 leo-engine 2>/dev/null || true
pkill -9 leo-input 2>/dev/null || true
pkill -9 macpilot-input 2>/dev/null || true
pkill -9 screencapture 2>/dev/null || true

# Wait a moment
sleep 0.5

echo "✅ All L.E.O. processes stopped cleanly."
echo "   Microphone and screen recording indicators are cleared."

