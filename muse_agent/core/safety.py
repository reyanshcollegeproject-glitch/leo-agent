from __future__ import annotations

import re
from typing import Callable, List, Optional
from muse_agent.controller.base import ActionType, AgentAction


class SafetyViolationError(Exception):
    """Raised when an action violates the Sentinel safety policy."""
    pass


class EmergencyStopError(Exception):
    """Raised when an emergency stop / killswitch is triggered."""
    pass


class SentinelGuardrail:
    """
    Meta Muse Sentinel Safety and Permission Guardrail.
    Monitors, intercepts, and gates autonomous actions to prevent
    destructive operations, unconfirmed data erasure, and runaway execution.
    """

    DEFAULT_BLOCKED_PATTERNS = [
        r"rm\s+-rf\s+[/~]",
        r"rm\s+-rf\s+\*",
        r":\(\)\s*\{\s*:\|:&\s*\};:",  # Fork bomb
        r"mkfs",
        r"dd\s+if=",
        r"chmod\s+-R\s+777\s+/",
        r"shutdown",
        r"reboot",
        r">\s*/dev/sd[a-z]",
    ]

    DESTRUCTIVE_KEYWORDS = [
        "delete permanently",
        "empty trash",
        "wipe drive",
        "erase all content",
        "confirm purchase",
        "transfer funds",
    ]

    def __init__(
        self,
        blocked_commands: Optional[List[str]] = None,
        require_confirmation: bool = True,
        confirmation_callback: Optional[Callable[[AgentAction], bool]] = None
    ):
        self.blocked_patterns = self.DEFAULT_BLOCKED_PATTERNS + (blocked_commands or [])
        self.require_confirmation = require_confirmation
        self.confirmation_callback = confirmation_callback
        self.emergency_stop_requested = False

    def trigger_emergency_stop(self) -> None:
        """Globally halt all agent actions immediately."""
        self.emergency_stop_requested = True

    def reset(self) -> None:
        self.emergency_stop_requested = False

    def validate_action(self, action: AgentAction) -> None:
        """
        Inspect an action prior to execution.
        Raises SafetyViolationError or EmergencyStopError if disallowed.
        """
        if self.emergency_stop_requested:
            raise EmergencyStopError("Agent halted by Sentinel Emergency Kill Switch!")

        # 1. Inspect Shell Commands
        if action.action == ActionType.RUN_COMMAND and action.command:
            cmd = action.command.strip()
            for pattern in self.blocked_patterns:
                if re.search(pattern, cmd, re.IGNORECASE):
                    raise SafetyViolationError(
                        f"Action blocked by Sentinel Guardrail: Destructive command detected matching '{pattern}': '{cmd}'"
                    )

        # 2. Inspect Destructive Keywords in typed text
        if action.action in (ActionType.TYPE_TEXT, ActionType.WRITE_CLIPBOARD) and action.text:
            text_lower = action.text.lower()
            for kw in self.DESTRUCTIVE_KEYWORDS:
                if kw in text_lower:
                    if self.require_confirmation and self.confirmation_callback:
                        approved = self.confirmation_callback(action)
                        if not approved:
                            raise SafetyViolationError(f"Action requires user confirmation: Contains '{kw}'")
                    else:
                        raise SafetyViolationError(
                            f"Action flagged by Sentinel: High-risk keyword '{kw}' detected without explicit confirmation."
                        )

        # 3. User Confirmation Hook for Destructive Actions
        if action.action == ActionType.RUN_COMMAND and self.require_confirmation and self.confirmation_callback:
            # Let callback confirm non-whitelisted commands if needed
            approved = self.confirmation_callback(action)
            if not approved:
                raise SafetyViolationError(f"Command execution rejected by user: '{action.command}'")
