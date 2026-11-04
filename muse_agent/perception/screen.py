from __future__ import annotations

import base64
import io
import os
import subprocess
import tempfile
from dataclasses import dataclass
from typing import Tuple, Optional
import mss
from PIL import Image, ImageStat

try:
    import Quartz
    QUARTZ_AVAILABLE = True
except ImportError:
    QUARTZ_AVAILABLE = False

try:
    import pyautogui
    PYAUTOGUI_AVAILABLE = True
except ImportError:
    PYAUTOGUI_AVAILABLE = False


@dataclass
class ScreenInfo:
    logical_width: int
    logical_height: int
    physical_width: int
    physical_height: int
    scale_factor: float


class ScreenPerceiver:
    """
    High-performance, Retina-aware screen capture and geometry manager for laptop automation.
    Handles coordinate scaling between logical OS coordinates (where clicks occur)
    and physical pixels (what the VLM sees).
    """

    def __init__(self, display_index: int = 0):
        self.display_index = display_index
        self._screen_info: Optional[ScreenInfo] = None
        self.last_capture_was_blank = False

    @staticmethod
    def has_screen_recording_permission() -> bool:
        """Check whether macOS has granted Screen Recording permission."""
        if QUARTZ_AVAILABLE:
            try:
                return Quartz.CGPreflightScreenCaptureAccess()
            except Exception:
                pass
        return True

    @staticmethod
    def request_screen_recording_permission() -> None:
        """Trigger macOS prompt and open Screen Recording Privacy Settings."""
        if QUARTZ_AVAILABLE:
            try:
                Quartz.CGRequestScreenCaptureAccess()
            except Exception:
                pass
        try:
            subprocess.run(["open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"])
        except Exception:
            pass

    @staticmethod
    def is_image_blank(img: Image.Image) -> bool:
        """Check if image is solid dark/blank due to missing OS permissions."""
        try:
            stat = ImageStat.Stat(img)
            # If standard deviation is 0 or extremely low, image has no visible desktop elements
            return all(s < 1.0 for s in stat.stddev)
        except Exception:
            return False

    def _detect_screen_geometry(self) -> ScreenInfo:
        """Detect display resolution, logical size, and Retina scale factor."""
        logical_w, logical_h = 1440, 900
        if PYAUTOGUI_AVAILABLE:
            try:
                size = pyautogui.size()
                logical_w, logical_h = int(size.width), int(size.height)
            except Exception:
                pass

        # Capture a test frame with mss to get physical pixel size
        try:
            with mss.MSS() as sct:
                monitors = sct.monitors
                # monitor[0] is all monitors combined, monitor[1] is primary
                mon = monitors[min(self.display_index + 1, len(monitors) - 1)]
                phys_w = mon["width"]
                phys_h = mon["height"]
        except Exception:
            phys_w, phys_h = logical_w, logical_h

        scale = round(phys_w / max(logical_w, 1), 2)
        if scale < 1.0:
            scale = 1.0

        return ScreenInfo(
            logical_width=logical_w,
            logical_height=logical_h,
            physical_width=phys_w,
            physical_height=phys_h,
            scale_factor=scale
        )

    @property
    def info(self) -> ScreenInfo:
        if self._screen_info is None:
            self._screen_info = self._detect_screen_geometry()
        return self._screen_info

    def capture(self) -> Image.Image:
        """
        Capture the current screen.
        Uses mss for high performance, with native fallbacks.
        """
        try:
            with mss.MSS() as sct:
                monitors = sct.monitors
                mon = monitors[min(self.display_index + 1, len(monitors) - 1)]
                sct_img = sct.grab(mon)
                img = Image.frombytes("RGB", sct_img.size, sct_img.bgra, "raw", "BGRX")
                self.last_capture_was_blank = self.is_image_blank(img)
                return img
        except Exception as e:
            # Fallback 1: screencapture on macOS
            if os.name != "nt":
                with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tf:
                    temp_path = tf.name
                try:
                    subprocess.run(["screencapture", "-x", "-m", temp_path], check=True, timeout=5)
                    img = Image.open(temp_path).convert("RGB")
                    os.unlink(temp_path)
                    return img
                except Exception:
                    if os.path.exists(temp_path):
                        os.unlink(temp_path)

            # Fallback 2: PyAutoGUI screenshot
            if PYAUTOGUI_AVAILABLE:
                return pyautogui.screenshot().convert("RGB")

            raise RuntimeError(f"Failed to capture screen: {e}")

    def capture_base64(self, max_width: Optional[int] = None, format: str = "PNG") -> Tuple[str, Image.Image]:
        """
        Capture the screen, optionally downscale for token efficiency,
        and return (base64_encoded_str, PIL_Image).
        """
        img = self.capture()
        if max_width and img.width > max_width:
            ratio = max_width / float(img.width)
            new_height = int(img.height * ratio)
            resized = img.resize((max_width, new_height), Image.Resampling.LANCZOS)
        else:
            resized = img

        buffer = io.BytesIO()
        if format.upper() == "JPEG":
            resized.save(buffer, format="JPEG", quality=85)
        else:
            resized.save(buffer, format="PNG", optimize=True)
        b64 = base64.b64encode(buffer.getvalue()).decode("utf-8")
        return b64, resized

    def normalized_to_logical(self, norm_x: float, norm_y: float) -> Tuple[int, int]:
        """
        Convert normalized [0, 1000] coordinates (used by Meta Muse & VLMs)
        into exact OS logical pixel coordinates (for mouse clicks).
        """
        log_x = int(round((norm_x / 1000.0) * self._screen_info.logical_width))
        log_y = int(round((norm_y / 1000.0) * self._screen_info.logical_height))
        # Clamp to screen bounds
        log_x = max(0, min(log_x, self._screen_info.logical_width - 1))
        log_y = max(0, min(log_y, self._screen_info.logical_height - 1))
        return log_x, log_y

    def logical_to_normalized(self, log_x: int, log_y: int) -> Tuple[int, int]:
        """Convert logical OS coordinates to [0, 1000] normalized space."""
        norm_x = int(round((log_x / self._screen_info.logical_width) * 1000.0))
        norm_y = int(round((log_y / self._screen_info.logical_height) * 1000.0))
        return norm_x, norm_y
