from __future__ import annotations

import subprocess
import time
from typing import Dict, Optional, Tuple
from muse_agent.controller.base import ActionType, AgentAction, ActionResult
from muse_agent.controller.mouse import MouseController
from muse_agent.controller.keyboard import KeyboardController
from muse_agent.controller.macos import MacOSController
from muse_agent.core.safety import SentinelGuardrail, SafetyViolationError, EmergencyStopError
from muse_agent.perception.screen import ScreenPerceiver


class OSController:
    """
    Unified OS Execution Engine.
    Executes AgentActions across Mouse, Keyboard, macOS System Events, and Shell,
    enforcing Sentinel safety validation on every operation.
    """

    def __init__(
        self,
        perceiver: ScreenPerceiver,
        sentinel: Optional[SentinelGuardrail] = None
    ):
        self.perceiver = perceiver
        self.sentinel = sentinel or SentinelGuardrail()
        self.mouse = MouseController(perceiver)
        self.keyboard = KeyboardController()
        self.macos = MacOSController()

    def execute(
        self,
        action: AgentAction,
        som_element_map: Optional[Dict[int, Tuple[int, int]]] = None
    ) -> ActionResult:
        """
        Execute an AgentAction and return structured ActionResult.
        If action refers to a Set-of-Marks element_id, its coordinates are automatically resolved.
        """
        # 1. Enforce Sentinel Safety Gate
        try:
            self.sentinel.validate_action(action)
        except (SafetyViolationError, EmergencyStopError) as e:
            return ActionResult(
                success=False,
                action=action,
                message=str(e),
                details={"safety_violation": True, "error": str(e)}
            )

        # 2. Resolve coordinates if element_id was specified
        x, y = action.x, action.y
        if action.element_id is not None and som_element_map and action.element_id in som_element_map:
            x, y = som_element_map[action.element_id]

        start_time = time.time()
        try:
            # --- MOUSE ACTIONS ---
            if action.action == ActionType.CLICK:
                if x is None or y is None:
                    raise ValueError("Click action requires x and y coordinates")
                self.mouse.click(x, y)
                msg = f"Clicked at ({x}, {y})"

            elif action.action == ActionType.DOUBLE_CLICK:
                if x is None or y is None:
                    raise ValueError("Double click requires coordinates")
                self.mouse.double_click(x, y)
                msg = f"Double clicked at ({x}, {y})"

            elif action.action == ActionType.RIGHT_CLICK:
                if x is None or y is None:
                    raise ValueError("Right click requires coordinates")
                self.mouse.right_click(x, y)
                msg = f"Right clicked at ({x}, {y})"

            elif action.action == ActionType.TRIPLE_CLICK:
                if x is None or y is None:
                    raise ValueError("Triple click requires coordinates")
                self.mouse.triple_click(x, y)
                msg = f"Triple clicked at ({x}, {y})"

            elif action.action == ActionType.MOVE_TO:
                if x is None or y is None:
                    raise ValueError("Move requires coordinates")
                self.mouse.move_to(x, y)
                msg = f"Moved cursor to ({x}, {y})"

            elif action.action == ActionType.DRAG_TO:
                if x is None or y is None:
                    raise ValueError("Drag requires target coordinates")
                cur_pos = self.mouse.get_position()
                self.mouse.drag_to(cur_pos, (x, y))
                msg = f"Dragged from {cur_pos} to ({x}, {y})"

            elif action.action == ActionType.SCROLL:
                amount = action.scroll_amount if action.scroll_amount is not None else -5
                self.mouse.scroll(amount)
                msg = f"Scrolled by {amount} units"

            # --- KEYBOARD ACTIONS ---
            elif action.action == ActionType.TYPE_TEXT:
                text = action.text or ""
                self.keyboard.type_text(text)
                msg = f"Typed text: '{text[:40]}...'" if len(text) > 40 else f"Typed text: '{text}'"

            elif action.action == ActionType.PRESS_HOTKEY:
                hotkey = action.hotkey or ""
                self.keyboard.press_hotkey(hotkey)
                msg = f"Pressed hotkey: {hotkey}"

            # --- OS & APPLICATION ACTIONS ---
            elif action.action == ActionType.LAUNCH_APP:
                app_name = action.app_name or ""
                if self.macos.is_macos():
                    success = self.macos.launch_app(app_name)
                    msg = f"Launched application: {app_name} (success={success})"
                else:
                    subprocess.Popen([app_name], shell=True)
                    msg = f"Launched application via shell: {app_name}"

            elif action.action == ActionType.SWITCH_APP:
                app_name = action.app_name or ""
                if self.macos.is_macos():
                    self.macos.launch_app(app_name)
                    msg = f"Switched to application: {app_name}"
                else:
                    self.keyboard.press_hotkey(["alt", "tab"])
                    msg = f"Switched app via Alt+Tab"

            elif action.action == ActionType.SPOTLIGHT:
                query = action.text or action.app_name or ""
                if self.macos.is_macos():
                    self.macos.trigger_spotlight(query)
                    msg = f"Triggered Spotlight for: '{query}'"
                else:
                    self.keyboard.press_hotkey(["win"])
                    time.sleep(0.2)
                    self.keyboard.type_text(query)
                    time.sleep(0.2)
                    self.keyboard.press_key("enter")
                    msg = f"Triggered Windows search for: '{query}'"

            elif action.action == ActionType.RUN_COMMAND:
                cmd = action.command or ""
                res = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=15)
                output_sample = (res.stdout + res.stderr)[:200]
                msg = f"Executed shell command: '{cmd[:50]}...' -> exit code {res.returncode}"
                return ActionResult(
                    success=(res.returncode == 0),
                    action=action,
                    message=msg,
                    details={"stdout": res.stdout, "stderr": res.stderr, "exit_code": res.returncode}
                )

            elif action.action == ActionType.READ_CLIPBOARD:
                clip = self.macos.read_clipboard()
                msg = f"Read clipboard: {clip[:50]}..."
                return ActionResult(success=True, action=action, message=msg, details={"clipboard": clip})

            elif action.action == ActionType.WRITE_CLIPBOARD:
                text = action.text or ""
                self.macos.write_clipboard(text)
                msg = f"Wrote {len(text)} characters to clipboard"

            # --- META ACTIONS ---
            elif action.action == ActionType.WAIT:
                sec = action.seconds or 1.0
                time.sleep(sec)
                msg = f"Waited {sec}s"

            elif action.action == ActionType.TASK_COMPLETE:
                msg = f"Task completed successfully: {action.thought or 'All objectives achieved'}"
                return ActionResult(success=True, action=action, message=msg, details={"done": True})

            elif action.action == ActionType.FAIL:
                msg = f"Task aborted by agent: {action.thought or 'Unrecoverable state'}"
                return ActionResult(success=False, action=action, message=msg, details={"aborted": True})

            else:
                msg = f"Unknown action type: {action.action}"
                return ActionResult(success=False, action=action, message=msg)

            duration = round(time.time() - start_time, 3)
            return ActionResult(
                success=True,
                action=action,
                message=msg,
                details={"execution_time_seconds": duration}
            )

        except Exception as e:
            return ActionResult(
                success=False,
                action=action,
                message=f"Action execution failed: {e}",
                details={"error": str(e)}
            )

