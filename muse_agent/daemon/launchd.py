from __future__ import annotations

import os
import plistlib
import subprocess
import sys
from pathlib import Path
from typing import Any, Dict, Optional


class LaunchdManager:
    """
    Native macOS launchd service installer and lifecycle supervisor.
    Ensures the Meta Muse Autonomous Agent runs 24/7 in the background
    and automatically restarts on system reboot or unexpected crash.
    """

    SERVICE_LABEL = "com.meta.muse.agent"

    def __init__(self, workspace_dir: Optional[Path] = None):
        self.workspace_dir = workspace_dir or Path.cwd()
        self.plist_path = Path.home() / "Library" / "LaunchAgents" / f"{self.SERVICE_LABEL}.plist"

    def generate_plist_dict(self) -> Dict[str, Any]:
        """Construct standard macOS LaunchAgent plist configuration."""
        venv_python = self.workspace_dir / "venv" / "bin" / "python3"
        if not venv_python.exists():
            venv_python = Path(sys.executable)

        entry_script = self.workspace_dir / "venv" / "bin" / "muse-agent"

        log_stdout = self.workspace_dir / "logs" / "daemon_stdout.log"
        log_stderr = self.workspace_dir / "logs" / "daemon_stderr.log"
        log_stdout.parent.mkdir(parents=True, exist_ok=True)

        return {
            "Label": self.SERVICE_LABEL,
            "ProgramArguments": [
                str(venv_python),
                str(entry_script),
                "daemon",
                "start"
            ],
            "WorkingDirectory": str(self.workspace_dir),
            "RunAtLoad": True,
            "KeepAlive": {
                "SuccessfulExit": False,
                "Crashed": True
            },
            "StandardOutPath": str(log_stdout),
            "StandardErrorPath": str(log_stderr),
            "EnvironmentVariables": {
                "PATH": f"{self.workspace_dir}/venv/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
                "PYTHONUNBUFFERED": "1"
            }
        }

    def install(self) -> bool:
        """Write plist file and register with macOS launchctl."""
        self.plist_path.parent.mkdir(parents=True, exist_ok=True)
        plist_data = self.generate_plist_dict()

        with open(self.plist_path, "wb") as f:
            plistlib.dump(plist_data, f)

        # Load with launchctl
        try:
            subprocess.run(["launchctl", "unload", str(self.plist_path)], capture_output=True)
            res = subprocess.run(["launchctl", "load", "-w", str(self.plist_path)], capture_output=True, text=True)
            return res.returncode == 0
        except Exception as e:
            return False

    def uninstall(self) -> bool:
        """Unload and remove plist file."""
        if self.plist_path.exists():
            try:
                subprocess.run(["launchctl", "unload", str(self.plist_path)], capture_output=True)
                self.plist_path.unlink()
                return True
            except Exception:
                return False
        return True

    def get_status(self) -> Dict[str, Any]:
        """Check if service is loaded and running via launchctl."""
        is_installed = self.plist_path.exists()
        is_running = False
        pid = None

        try:
            res = subprocess.run(["launchctl", "list"], capture_output=True, text=True)
            for line in res.stdout.splitlines():
                if self.SERVICE_LABEL in line:
                    parts = line.split()
                    if parts[0] != "-":
                        pid = int(parts[0])
                        is_running = True
                    break
        except Exception:
            pass

        return {
            "installed": is_installed,
            "running": is_running,
            "pid": pid,
            "plist_path": str(self.plist_path)
        }
