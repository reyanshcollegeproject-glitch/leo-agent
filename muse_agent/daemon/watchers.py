from __future__ import annotations

import os
import subprocess
import time
from dataclasses import dataclass
from typing import Optional
import psutil


@dataclass
class SystemSnapshot:
    cpu_percent: float
    memory_percent: float
    disk_free_gb: float
    battery_percent: float
    is_charging: bool
    user_idle_seconds: float
    is_user_active: bool  # Active if touched keyboard/mouse within last 60 seconds


class SystemWatcher:
    """
    24/7 Proactive System Telemetry & Idle Monitor.
    Detects hardware resources, battery levels, thermal load, and user presence
    to ensure the agent operates smoothly in the background without disturbing the user.
    """

    def __init__(self, idle_threshold_seconds: float = 180.0):
        self.idle_threshold_seconds = idle_threshold_seconds

    def get_user_idle_seconds(self) -> float:
        """Get seconds since the last physical user mouse/keyboard input on macOS."""
        if os.name != "nt":
            try:
                cmd = "ioreg -c IOHIDSystem | awk '/HIDIdleTime/ {print int($NF/1000000000); exit}'"
                out = subprocess.check_output(cmd, shell=True, text=True, timeout=2).strip()
                if out.isdigit():
                    return float(out)
            except Exception:
                pass
        return 0.0

    def capture_snapshot(self) -> SystemSnapshot:
        """Capture live system resource and state metrics."""
        cpu = psutil.cpu_percent(interval=None)
        mem = psutil.virtual_memory().percent
        disk = psutil.disk_usage("/").free / (1024 ** 3)  # in GB

        bat = psutil.sensors_battery()
        bat_percent = float(bat.percent) if bat else 100.0
        is_charging = bool(bat.power_plugged) if bat else True

        idle_sec = self.get_user_idle_seconds()
        is_active = (idle_sec < 60.0)

        return SystemSnapshot(
            cpu_percent=round(cpu, 1),
            memory_percent=round(mem, 1),
            disk_free_gb=round(disk, 2),
            battery_percent=round(bat_percent, 1),
            is_charging=is_charging,
            user_idle_seconds=round(idle_sec, 1),
            is_user_active=is_active
        )

    def is_safe_for_background_gui(self, snapshot: Optional[SystemSnapshot] = None) -> bool:
        """
        Returns True if the agent is safe to control the mouse/GUI in the background.
        If user is actively typing or clicking (idle < threshold), returns False to avoid hijacking.
        """
        snap = snapshot or self.capture_snapshot()
        # If battery is critical (<15%) and not charging, pause heavy background tasks
        if snap.battery_percent < 15.0 and not snap.is_charging:
            return False
        # If user is actively using the screen, defer background GUI takeovers
        if snap.user_idle_seconds < self.idle_threshold_seconds:
            return False
        return True

