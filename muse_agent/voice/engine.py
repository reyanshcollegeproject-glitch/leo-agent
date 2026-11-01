from __future__ import annotations

import io
import logging
import os
import subprocess
import tempfile
import threading
from typing import Optional

logger = logging.getLogger("muse_agent.voice.engine")

try:
    import sounddevice as sd
    import numpy as np
    from scipy.io import wavfile
    SOUNDDEVICE_AVAILABLE = True
except ImportError:
    SOUNDDEVICE_AVAILABLE = False

try:
    import speech_recognition as sr
    SPEECH_REC_AVAILABLE = True
except ImportError:
    SPEECH_REC_AVAILABLE = False


class VoiceEngine:
    """
    High-performance, low-latency Voice Engine for macOS laptop autonomous agents.
    Provides:
    1. Instant native Text-to-Speech (TTS) via macOS /usr/bin/say with background threading.
    2. Real-time microphone capture & speech-to-text (STT) transcription via sounddevice & speech_recognition.
    """

    def __init__(self, voice_name: str = "Samantha", speech_rate: int = 190):
        self.voice_name = voice_name
        self.speech_rate = speech_rate
        self.is_speaking = False
        self.is_listening = False
        self._current_process: Optional[subprocess.Popen] = None
        self._lock = threading.Lock()

    def speak(self, text: str, blocking: bool = False) -> None:
        """
        Speak text aloud using macOS native high-quality speech synthesis.
        Runs asynchronously in background thread by default so it never blocks UI or agent reasoning.
        """
        if not text or not text.strip():
            return

        # Sanitize text for speech (remove markdown symbols, URLs, JSON brackets)
        clean_text = text.replace("`", "").replace("*", "").replace("#", "")
        if "http://" in clean_text or "https://" in clean_text:
            clean_text = clean_text.split("http")[0] + " web link opened."

        def _do_speak():
            with self._lock:
                self.stop_speaking()
                self.is_speaking = True
                try:
                    # Prefer Samantha or default system voice
                    cmd = ["/usr/bin/say", "-r", str(self.speech_rate), clean_text]
                    self._current_process = subprocess.Popen(cmd)
                    self._current_process.wait()
                except Exception as e:
                    logger.warning(f"Voice speech synthesis error: {e}")
                finally:
                    self.is_speaking = False
                    self._current_process = None

        if blocking:
            _do_speak()
        else:
            threading.Thread(target=_do_speak, daemon=True).start()

    def stop_speaking(self) -> None:
        """Immediately silence active speech."""
        if self._current_process and self._current_process.poll() is None:
            try:
                self._current_process.terminate()
            except Exception:
                pass
            self._current_process = None
        self.is_speaking = False

    def listen_and_transcribe(self, duration_seconds: float = 4.0, samplerate: int = 16000) -> Optional[str]:
        """
        Record audio from the laptop microphone for duration_seconds and transcribe to text.
        """
        if not SOUNDDEVICE_AVAILABLE or not SPEECH_REC_AVAILABLE:
            logger.warning("Voice listening dependencies not available.")
            return None

        self.is_listening = True
        temp_wav = None
        try:
            logger.info(f"Listening to microphone for {duration_seconds}s...")
            # Record single-channel 16kHz audio
            recording = sd.rec(
                int(duration_seconds * samplerate),
                samplerate=samplerate,
                channels=1,
                dtype="int16"
            )
            sd.wait()

            # Save to temporary WAV
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tf:
                temp_wav = tf.name

            wavfile.write(temp_wav, samplerate, recording)

            # Transcribe with SpeechRecognition
            recognizer = sr.Recognizer()
            with sr.AudioFile(temp_wav) as source:
                audio_data = recognizer.record(source)

            transcription = recognizer.recognize_google(audio_data)
            logger.info(f"Voice transcription result: '{transcription}'")
            return transcription.strip()

        except sr.UnknownValueError:
            logger.info("Speech unrecorded or unclear.")
            return None
        except Exception as e:
            logger.warning(f"Microphone transcription error: {e}")
            return None
        finally:
            self.is_listening = False
            if temp_wav and os.path.exists(temp_wav):
                try:
                    os.unlink(temp_wav)
                except Exception:
                    pass
