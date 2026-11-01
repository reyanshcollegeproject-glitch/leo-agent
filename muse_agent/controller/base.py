from __future__ import annotations

from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


class ActionType(str, Enum):
    # Mouse Actions
    CLICK = "click"
    DOUBLE_CLICK = "double_click"
    RIGHT_CLICK = "right_click"
    TRIPLE_CLICK = "triple_click"
    MOUSE_DOWN = "mouse_down"
    MOUSE_UP = "mouse_up"
    MOVE_TO = "move_to"
    DRAG_TO = "drag_to"
    SCROLL = "scroll"

    # Keyboard Actions
    TYPE_TEXT = "type_text"
    PRESS_HOTKEY = "press_hotkey"
    KEY_DOWN = "key_down"
    KEY_UP = "key_up"

    # OS & Application Actions
    LAUNCH_APP = "launch_app"
    SWITCH_APP = "switch_app"
    SPOTLIGHT = "spotlight"
    RUN_COMMAND = "run_command"
    READ_CLIPBOARD = "read_clipboard"
    WRITE_CLIPBOARD = "write_clipboard"

    # Agent Meta Actions
    WAIT = "wait"
    CLICK_ELEMENT = "click_element"  # Set-of-Marks ID click
    TASK_COMPLETE = "task_complete"
    FAIL = "fail"


class AgentAction(BaseModel):
    """
    Structured representation of an action decided by Meta Muse.
    """
    action: ActionType = Field(description="Action type to perform")
    # Coordinates in normalized [0, 1000] space
    x: Optional[int] = Field(default=None, description="Normalized X coordinate [0, 1000]")
    y: Optional[int] = Field(default=None, description="Normalized Y coordinate [0, 1000]")
    element_id: Optional[int] = Field(default=None, description="Set-of-Marks UI element ID")
    text: Optional[str] = Field(default=None, description="Text string to type or clipboard content")
    hotkey: Optional[str] = Field(default=None, description="Hotkey combination (e.g. 'cmd+space', 'enter')")
    app_name: Optional[str] = Field(default=None, description="Target application name")
    command: Optional[str] = Field(default=None, description="Bash shell command string")
    scroll_amount: Optional[int] = Field(default=None, description="Scroll amount: positive=up, negative=down")
    seconds: Optional[float] = Field(default=None, description="Sleep/wait duration in seconds")
    thought: Optional[str] = Field(default=None, description="Agent's reasoning before performing action")
    expected_outcome: Optional[str] = Field(default=None, description="Anticipated visual change or state result")


class ActionResult(BaseModel):
    """
    Result returned after executing an AgentAction.
    """
    success: bool
    action: AgentAction
    message: str
    visual_change_detected: bool = False
    details: Dict[str, Any] = Field(default_factory=dict)

