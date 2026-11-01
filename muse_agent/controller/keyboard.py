from __future__ import annotations

import os
import subprocess
import time
from typing import List

try:
    import pyautogui
    PYAUTOGUI_AVAILABLE = True
except ImportError:
    PYAUTOGUI_AVAILABLE = False


class KeyboardController:
    """
    Robust keyboard controller with multi-platform key translation,
    realistic typing, and atomic clipboard fallback for complex unicode strings.
    """

    KEY_MAP = {
        "cmd": "command",
        "super": "command",
        "win": "command" if os.name != "nt" else "win",
        "ctrl": "ctrl",
        "control": "ctrl",
        "alt": "alt",
        "opt": "option" if os.name != "nt" else "alt",
        "option": "option" if os.name != "nt" else "alt",
        "shift": "shift",
        "enter": "enter",
        "return": "return",
        "esc": "esc",
        "escape": "escape",
        "space": "space",
        "tab": "tab",
        "backspace": "backspace",
        "delete": "delete",
        "up": "up",
        "down": "down",
        "left": "left",
        "right": "right",
    }

    def _normalize_key(self, key: str) -> str:
        k = key.strip().lower()
        return self.KEY_MAP.get(k, k)

    def type_text(self, text: str, delay_per_char: float = 0.015, force_clipboard: bool = False) -> None:
        """
        Type text. If string has special unicode or newlines, use clipboard paste for 100% fidelity.
        """
        if not text:
            return

        has_special = any(ord(c) > 127 or c in "\n\t" for c in text)
        if force_clipboard or has_special or len(text) > 60:
            self.paste_text(text)
            return

        if PYAUTOGUI_AVAILABLE:
            pyautogui.write(text, interval=delay_per_char)

    def paste_text(self, text: str) -> None:
        """Write text to OS clipboard and execute paste shortcut."""
        # macOS native pbcopy
        if os.name != "nt":
            try:
                proc = subprocess.Popen(["pbcopy"], stdin=subprocess.PIPE)
                proc.communicate(text.encode("utf-8"))
            except Exception:
                pass
        time.sleep(0.05)
        # Paste shortcut: command+v on macOS
        self.press_hotkey(["cmd", "v"] if os.name != "nt" else ["ctrl", "v"])
        time.sleep(0.08)

    def press_hotkey(self, keys: List[str] | str) -> None:
        """
        Press hotkey combination.
        Accepts list `['cmd', 'space']` or string `'cmd+space'`.
        """
        if isinstance(keys, str):
            key_list = [k.strip() for k in keys.replace("+", " ").replace("-", " ").split()]
        else:
            key_list = list(keys)

        normalized = [self._normalize_key(k) for k in key_list]
        if PYAUTOGUI_AVAILABLE:
            pyautogui.hotkey(*normalized)

    def press_key(self, key: str) -> None:
        """Press single key."""
        norm_key = self._normalize_key(key)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.press(norm_key)

    def key_down(self, key: str) -> None:
        norm_key = self._normalize_key(key)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.keyDown(norm_key)

    def key_up(self, key: str) -> None:
        norm_key = self._normalize_key(key)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.keyUp(norm_key)

