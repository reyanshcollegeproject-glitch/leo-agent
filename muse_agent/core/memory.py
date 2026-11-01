from __future__ import annotations

import json
import os
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Optional
from PIL import Image
from muse_agent.controller.base import AgentAction, ActionResult
from muse_agent.perception.visual_diff import VisualDiffResult


@dataclass
class TrajectoryStep:
    step_number: int
    timestamp: float
    action: Dict[str, Any]
    result: Dict[str, Any]
    visual_change: bool
    diff_ratio: float
    thought: Optional[str] = None
    expected_outcome: Optional[str] = None
    screenshot_filename: Optional[str] = None
    annotated_filename: Optional[str] = None


class TrajectoryMemory:
    """
    Episodic memory recorder for autonomous agent runs.
    Saves step-by-step telemetry, screenshots, and visual diffs for replay and benchmarking.
    """

    def __init__(self, run_id: Optional[str] = None, base_dir: Path = Path("runs")):
        self.run_id = run_id or f"run_{int(time.time())}"
        self.run_dir = base_dir / self.run_id
        self.screenshots_dir = self.run_dir / "screenshots"
        self.steps: List[TrajectoryStep] = []
        self._initialized = False

    def _ensure_dirs(self) -> None:
        if not self._initialized:
            self.screenshots_dir.mkdir(parents=True, exist_ok=True)
            self._initialized = True

    def record_step(
        self,
        step_number: int,
        action: AgentAction,
        result: ActionResult,
        diff: Optional[VisualDiffResult] = None,
        raw_image: Optional[Image.Image] = None,
        annotated_image: Optional[Image.Image] = None
    ) -> TrajectoryStep:
        self._ensure_dirs()
        ts = time.time()

        raw_filename = None
        if raw_image:
            raw_filename = f"step_{step_number:03d}_raw.png"
            raw_image.save(self.screenshots_dir / raw_filename)

        ann_filename = None
        if annotated_image:
            ann_filename = f"step_{step_number:03d}_annotated.png"
            annotated_image.save(self.screenshots_dir / ann_filename)

        step = TrajectoryStep(
            step_number=step_number,
            timestamp=ts,
            action=action.model_dump(),
            result=result.model_dump(),
            visual_change=diff.has_changed if diff else False,
            diff_ratio=round(diff.diff_ratio, 4) if diff else 0.0,
            thought=action.thought,
            expected_outcome=action.expected_outcome,
            screenshot_filename=raw_filename,
            annotated_filename=ann_filename
        )
        self.steps.append(step)
        self.save_manifest()
        return step

    def save_manifest(self) -> None:
        self._ensure_dirs()
        manifest_path = self.run_dir / "trajectory.json"
        data = {
            "run_id": self.run_id,
            "total_steps": len(self.steps),
            "steps": [asdict(s) for s in self.steps]
        }
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2)

    def get_context_history(self, max_recent: int = 6) -> List[Dict[str, Any]]:
        """Return compact summary of recent steps for LLM prompt context."""
        summary = []
        for s in self.steps[-max_recent:]:
            act = s.action.get("action", "")
            summary.append({
                "step": s.step_number,
                "action": act,
                "result": s.result.get("message", ""),
                "visual_change": s.visual_change
            })
        return summary

