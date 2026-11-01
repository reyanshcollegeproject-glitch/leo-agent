from __future__ import annotations

import os
import subprocess
from pathlib import Path
from typing import List, Optional


class DesktopWindowController:
    """
    Native macOS Desktop & Window Automation Subsystem.
    Provides direct control over desktop windows, Finder file operations,
    and system settings without requiring any browser or web interfaces.
    """

    @staticmethod
    def _run_applescript(script: str) -> str:
        try:
            res = subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=5)
            return res.stdout.strip()
        except Exception as e:
            return f"Error: {e}"

    def open_folder(self, path: Path | str) -> bool:
        """Open a directory in macOS Finder."""
        p = str(Path(path).expanduser().resolve())
        script = f'tell application "Finder" to open (POSIX file "{p}")'
        out = self._run_applescript(script)
        return "Error" not in out

    def reveal_file(self, path: Path | str) -> bool:
        """Reveal a file or folder in Finder and highlight it."""
        p = str(Path(path).expanduser().resolve())
        script = f'tell application "Finder" to reveal (POSIX file "{p}")'
        out = self._run_applescript(script)
        return "Error" not in out

    def get_selected_finder_items(self) -> List[str]:
        """Return the POSIX paths of currently selected items in Finder."""
        script = '''
        tell application "Finder"
            set selectedItems to selection
            set pathList to {}
            repeat with anItem in selectedItems
                set end of pathList to POSIX path of (anItem as text)
            end repeat
            return pathList
        end tell
        '''
        out = self._run_applescript(script)
        if not out or "Error" in out:
            return []
        return [p.strip() for p in out.split(",") if p.strip()]

    def minimize_all_windows(self) -> None:
        """Minimize all open application windows to show desktop."""
        script = '''
        tell application "Finder"
            set desktopBounds to bounds of window of desktop
        end tell
        tell application "System Events"
            key code 103 -- Mission Control / Show Desktop key
        end tell
        '''
        self._run_applescript(script)

    def center_active_window(self, width: int = 1200, height: int = 800) -> None:
        """Center and resize the frontmost window on the primary screen."""
        script = f'''
        tell application "Finder"
            set {{dLeft, dTop, dRight, dBottom}} to bounds of window of desktop
            set screenW to dRight - dLeft
            set screenH to dBottom - dTop
        end tell
        set targetW to {width}
        set targetH to {height}
        set posX to (screenW - targetW) / 2
        set posY to (screenH - targetH) / 2
        tell application "System Events"
            set frontApp to first application process whose frontmost is true
            try
                set position of front window of frontApp to {{posX, posY}}
                set size of front window of frontApp to {{targetW, targetH}}
            end try
        end tell
        '''
        self._run_applescript(script)

    def tile_window_left(self) -> None:
        """Snap frontmost window to the left half of the display."""
        script = '''
        tell application "Finder"
            set {dLeft, dTop, dRight, dBottom} to bounds of window of desktop
            set halfW to (dRight - dLeft) / 2
            set screenH to (dBottom - dTop)
        end tell
        tell application "System Events"
            set frontApp to first application process whose frontmost is true
            try
                set position of front window of frontApp to {0, 0}
                set size of front window of frontApp to {halfW, screenH}
            end try
        end tell
        '''
        self._run_applescript(script)

    def tile_window_right(self) -> None:
        """Snap frontmost window to the right half of the display."""
        script = '''
        tell application "Finder"
            set {dLeft, dTop, dRight, dBottom} to bounds of window of desktop
            set halfW to (dRight - dLeft) / 2
            set screenH to (dBottom - dTop)
        end tell
        tell application "System Events"
            set frontApp to first application process whose frontmost is true
            try
                set position of front window of frontApp to {halfW, 0}
                set size of front window of frontApp to {halfW, screenH}
            end try
        end tell
        '''
        self._run_applescript(script)

    def set_system_volume(self, percent: int) -> None:
        """Set macOS system audio volume (0-100)."""
        vol = max(0, min(100, percent))
        script = f"set volume output volume {vol}"
        self._run_applescript(script)

    def toggle_dark_mode(self) -> None:
        """Toggle macOS Dark Mode on or off."""
        script = '''
        tell application "System Events"
            tell appearance preferences
                set dark mode to not dark mode
            end tell
        end tell
        '''
        self._run_applescript(script)

