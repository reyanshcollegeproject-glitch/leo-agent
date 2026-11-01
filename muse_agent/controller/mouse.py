from __future__ import annotations

import time
from typing import Optional, Tuple
from muse_agent.perception.screen import ScreenPerceiver

try:
    import pyautogui
    # Configure PyAutoGUI fail-safe
    pyautogui.FAILSAFE = True
    pyautogui.PAUSE = 0.05
    PYAUTOGUI_AVAILABLE = True
except ImportError:
    PYAUTOGUI_AVAILABLE = False


class MouseController:
    """
    Sub-pixel precision mouse controller for macOS and cross-platform desktop automation.
    Translates normalized [0, 1000] model outputs to OS logical coordinates.
    """

    def __init__(self, perceiver: ScreenPerceiver):
        self.perceiver = perceiver

    def _resolve_coords(self, norm_x: int, norm_y: int) -> Tuple[int, int]:
        return self.perceiver.normalized_to_logical(norm_x, norm_y)

    def move_to(self, norm_x: int, norm_y: int, duration: float = 0.15) -> None:
        """Move cursor smoothly to normalized coordinate."""
        log_x, log_y = self._resolve_coords(norm_x, norm_y)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.moveTo(log_x, log_y, duration=duration)

    def click(self, norm_x: int, norm_y: int) -> None:
        """Click at normalized coordinate."""
        log_x, log_y = self._resolve_coords(norm_x, norm_y)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.click(log_x, log_y)

    def double_click(self, norm_x: int, norm_y: int) -> None:
        """Double click at normalized coordinate."""
        log_x, log_y = self._resolve_coords(norm_x, norm_y)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.doubleClick(log_x, log_y)

    def right_click(self, norm_x: int, norm_y: int) -> None:
        """Right click at normalized coordinate."""
        log_x, log_y = self._resolve_coords(norm_x, norm_y)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.rightClick(log_x, log_y)

    def triple_click(self, norm_x: int, norm_y: int) -> None:
        """Triple click to select full lines or paragraphs."""
        log_x, log_y = self._resolve_coords(norm_x, norm_y)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.tripleClick(log_x, log_y)

    def mouse_down(self, norm_x: Optional[int] = None, norm_y: Optional[int] = None) -> None:
        """Press and hold mouse button."""
        if norm_x is not None and norm_y is not None:
            self.move_to(norm_x, norm_y, duration=0.1)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.mouseDown()

    def mouse_up(self, norm_x: Optional[int] = None, norm_y: Optional[int] = None) -> None:
        """Release mouse button."""
        if norm_x is not None and norm_y is not None:
            self.move_to(norm_x, norm_y, duration=0.1)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.mouseUp()

    def drag_to(self, start_norm: Tuple[int, int], end_norm: Tuple[int, int], duration: float = 0.5) -> None:
        """Drag from start coordinate to end coordinate."""
        sx, sy = self._resolve_coords(*start_norm)
        ex, ey = self._resolve_coords(*end_norm)
        if PYAUTOGUI_AVAILABLE:
            pyautogui.moveTo(sx, sy)
            time.sleep(0.05)
            pyautogui.dragTo(ex, ey, duration=duration, button="left")

    def scroll(self, amount: int) -> None:
        """
        Scroll wheel.
        amount > 0: scroll up
        amount < 0: scroll down
        """
        if PYAUTOGUI_AVAILABLE:
            pyautogui.scroll(amount)

    def get_position(self) -> Tuple[int, int]:
        """Return current mouse position in normalized [0, 1000] coordinates."""
        if PYAUTOGUI_AVAILABLE:
            pos = pyautogui.position()
            return self.perceiver.logical_to_normalized(pos.x, pos.y)
        return (0, 0)
