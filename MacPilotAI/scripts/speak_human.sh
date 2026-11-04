#!/bin/bash
# MacPilot AI Studio-Quality Human Neural Speech (Single instance, zero echo)
TEXT="$1"
VOICE="${2:-en-GB-RyanNeural}"

if [ -z "$TEXT" ]; then
    exit 0
fi

MY_PID=$$

# Kill any other running speak_human.sh scripts to guarantee single-voice exclusivity
for pid in $(pgrep -f "speak_human\.sh" 2>/dev/null); do
    if [ "$pid" != "$MY_PID" ]; then
        kill -9 "$pid" 2>/dev/null || true
    fi
done

# Kill any running edge-tts or active audio playback
pkill -9 -f "edge-tts" 2>/dev/null || true
pkill -9 -f "afplay.*macpilot_speech" 2>/dev/null || true
pkill -9 -f "/usr/bin/say" 2>/dev/null || true

TMP_MP3="/tmp/macpilot_speech_${MY_PID}.mp3"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PARENT_ROOT="$(cd "$REPO_ROOT/.." && pwd)"

EDGE_TTS=""
if [ -x "/Users/reyansh/Desktop/contribute/auto agent/.venv/bin/edge-tts" ]; then
    EDGE_TTS="/Users/reyansh/Desktop/contribute/auto agent/.venv/bin/edge-tts"
elif [ -x "$PARENT_ROOT/.venv/bin/edge-tts" ]; then
    EDGE_TTS="$PARENT_ROOT/.venv/bin/edge-tts"
elif [ -x "$REPO_ROOT/.venv/bin/edge-tts" ]; then
    EDGE_TTS="$REPO_ROOT/.venv/bin/edge-tts"
elif command -v edge-tts >/dev/null 2>&1; then
    EDGE_TTS="$(command -v edge-tts)"
fi

cleanup() {
    rm -f "$TMP_MP3" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

if [ -n "$EDGE_TTS" ]; then
    "$EDGE_TTS" --voice "$VOICE" --text "$TEXT" --write-media "$TMP_MP3" >/dev/null 2>&1
    if [ -f "$TMP_MP3" ] && [ -s "$TMP_MP3" ]; then
        # Ensure no other afplay is running right before we speak
        pkill -9 -f "afplay.*macpilot_speech" 2>/dev/null || true
        afplay "$TMP_MP3"
        exit 0
    fi
fi
exit 0
