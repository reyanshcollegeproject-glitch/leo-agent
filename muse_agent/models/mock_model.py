from __future__ import annotations

import asyncio
from typing import Any, Dict, List, Optional
from muse_agent.controller.base import ActionType, AgentAction
from muse_agent.models.base import BaseVisionModel


class MockMuseModel(BaseVisionModel):
    """
    Simulated Meta Muse model for offline benchmarks, deterministic testing,
    and CI environments.
    """

    def __init__(self, script: Optional[List[AgentAction]] = None):
        self.script = script or []
        self.step_idx = 0

    def set_script(self, script: List[AgentAction]) -> None:
        self.script = script
        self.step_idx = 0

    async def predict_action(
        self,
        screenshot_b64: str,
        goal: str,
        history: List[Dict[str, Any]],
        som_tags: Optional[Dict[int, Any]] = None
    ) -> AgentAction:
        await asyncio.sleep(0.05)
        if self.script and self.step_idx < len(self.script):
            action = self.script[self.step_idx]
            self.step_idx += 1
            return action

        # Default fallback: complete task
        return AgentAction(
            action=ActionType.TASK_COMPLETE,
            thought="Offline simulation complete: Goal fulfilled successfully."
        )

