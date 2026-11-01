from __future__ import annotations

import os
import subprocess
import time
from typing import Dict, List, Optional


class MacOSController:
    """
    High-speed native macOS automation subsystem via AppleScript and system utilities.
    Bypasses visual guessing for core OS operations (app launching, window switching, spotlight).
    """

    @staticmethod
    def is_macos() -> bool:
        return os.name != "nt" and subprocess.run(["uname"], capture_output=True, text=True).stdout.strip() == "Darwin"

    @staticmethod
    def run_applescript(script: str) -> str:
        """Execute an AppleScript snippet and return its trimmed stdout."""
        try:
            res = subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=8)
            return res.stdout.strip()
        except Exception as e:
            return f"Error: {e}"

    def launch_app(self, app_name: str) -> bool:
        """Launch or bring an application to the front."""
        script = f'''
        tell application "{app_name}"
            activate
        end tell
        '''
        res = self.run_applescript(script)
        time.sleep(0.5)
        return "Error" not in res

    def get_frontmost_app(self) -> Dict[str, str]:
        """Return the frontmost active application and window title."""
        script = '''
        tell application "System Events"
            set frontApp to first application process whose frontmost is true
            set appName to name of frontApp
            try
                set winTitle to name of front window of frontApp
            on error
                set winTitle to ""
            end try
            return appName & "|||" & winTitle
        end tell
        '''
        output = self.run_applescript(script)
        if "|||" in output:
            parts = output.split("|||", 1)
            return {"app": parts[0].strip(), "window": parts[1].strip()}
        return {"app": output.strip(), "window": ""}

    def list_running_apps(self) -> List[str]:
        """Return list of running visible GUI applications."""
        script = 'tell application "System Events" to get name of every process whose visible is true'
        out = self.run_applescript(script)
        if not out or "Error" in out:
            return []
        return [app.strip() for app in out.split(",")]

    def trigger_spotlight(self, query: str) -> None:
        """Trigger macOS Spotlight, type query, and press return."""
        script = f'''
        tell application "System Events"
            key code 49 using {{command down}}
            delay 0.3
            keystroke "{query}"
            delay 0.4
            key code 36
        end tell
        '''
        self.run_applescript(script)
        time.sleep(0.8)

    def read_clipboard(self) -> str:
        """Read current system clipboard text."""
        try:
            res = subprocess.run(["pbpaste"], capture_output=True, text=True, timeout=3)
            return res.stdout
        except Exception:
            return ""

    def write_clipboard(self, text: str) -> None:
        """Write string to system clipboard."""
        try:
            p = subprocess.Popen(["pbcopy"], stdin=subprocess.PIPE)
            p.communicate(text.encode("utf-8"))
        except Exception:
            pass

    def show_notification(self, title: str, message: str) -> None:
        """Show native macOS notification."""
        script = f'display notification "{message}" with title "{title}"'
        self.run_applescript(script)

