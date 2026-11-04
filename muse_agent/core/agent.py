from __future__ import annotations

import asyncio
import logging
import time
from enum import Enum
from typing import Any, Callable, Dict, List, Optional
from muse_agent.config import settings
from muse_agent.controller.base import ActionType, AgentAction, ActionResult
from muse_agent.controller.system import OSController
from muse_agent.core.memory import TrajectoryMemory
from muse_agent.core.planner import HierarchicalPlanner, ExecutionPlan
from muse_agent.core.safety import SentinelGuardrail
from muse_agent.models.base import BaseVisionModel
from muse_agent.perception.grounding import VisualGrounder
from muse_agent.perception.screen import ScreenPerceiver
from muse_agent.perception.visual_diff import VisualVerifier
from muse_agent.core.fast_decision import FastDecisionEngine

logger = logging.getLogger("muse_agent.core.agent")


class AgentState(str, Enum):
    IDLE = "idle"
    RUNNING = "running"
    PAUSED = "paused"
    STOPPED = "stopped"
    COMPLETED = "completed"
    FAILED = "failed"


class MuseAgent:
    """
    Production-grade Autonomous OS / Laptop Control Agent.
    Implements a closed-loop Observe-Orient-Decide-Act-Verify cycle powered by
    Meta AI Muse and sub-pixel visual grounding.
    """

    def __init__(
        self,
        model: BaseVisionModel,
        talk_model: Optional[Any] = None,
        perceiver: Optional[ScreenPerceiver] = None,
        grounder: Optional[VisualGrounder] = None,
        verifier: Optional[VisualVerifier] = None,
        controller: Optional[OSController] = None,
        sentinel: Optional[SentinelGuardrail] = None,
        run_id: Optional[str] = None
    ):
        self.model = model
        self.control_model = model
        self.talk_model = talk_model
        self.perceiver = perceiver or ScreenPerceiver(display_index=settings.display_index)
        self.grounder = grounder or VisualGrounder()
        self.verifier = verifier or VisualVerifier(threshold=settings.diff_threshold)
        self.sentinel = sentinel or SentinelGuardrail(
            blocked_commands=settings.blocked_commands,
            require_confirmation=settings.require_confirmation_for_destructive
        )
        self.controller = controller or OSController(perceiver=self.perceiver, sentinel=self.sentinel)
        self.planner = HierarchicalPlanner()
        self.fast_engine = FastDecisionEngine()
        self.memory = TrajectoryMemory(run_id=run_id, base_dir=settings.trajectories_dir)

        self.state = AgentState.IDLE
        self.current_goal: Optional[str] = None
        self.current_plan: Optional[ExecutionPlan] = None
        self.step_callbacks: List[Callable[[Dict[str, Any]], None]] = []
        self._pause_event = asyncio.Event()
        self._pause_event.set()  # Unpaused initially

    def add_step_callback(self, callback: Callable[[Dict[str, Any]], None]) -> None:
        """Register a callback for real-time telemetry streaming (WebSockets, CLI, etc.)."""
        self.step_callbacks.append(callback)

    def pause(self) -> None:
        """Pause agent execution after current step."""
        self.state = AgentState.PAUSED
        self._pause_event.clear()
        logger.info("Agent execution paused.")

    def resume(self) -> None:
        """Resume paused agent execution."""
        if self.state == AgentState.PAUSED:
            self.state = AgentState.RUNNING
            self._pause_event.set()
            logger.info("Agent execution resumed.")

    def stop(self) -> None:
        """Halt agent execution immediately."""
        self.state = AgentState.STOPPED
        self.sentinel.trigger_emergency_stop()
        self._pause_event.set()
        logger.warning("Agent execution stopped by user or kill switch.")

    async def run(self, goal: str, max_steps: Optional[int] = None) -> Dict[str, Any]:
        """
        Execute an autonomous task on the laptop from start to finish.
        Returns final execution summary and metrics.
        """
        self.current_goal = goal
        self.current_plan = self.planner.decompose(goal)
        self.state = AgentState.RUNNING
        self.sentinel.reset()
        limit = max_steps or settings.max_steps_per_task
        step_number = 1
        start_time = time.time()

        logger.info(f"Starting autonomous agent run for goal: '{goal}' (limit: {limit} steps)")

        try:
            while step_number <= limit and self.state == AgentState.RUNNING:
                await self._pause_event.wait()
                if self.state != AgentState.RUNNING:
                    break

                # 1. OBSERVE: Capture before screenshot
                raw_before = self.perceiver.capture()

                # 2. GROUND: Generate Set-of-Marks and reference grid overlay
                som_img, mark_coords = self.grounder.apply_set_of_marks(raw_before)
                if settings.use_grid_overlay:
                    visual_input = self.grounder.apply_grid_overlay(som_img)
                else:
                    visual_input = som_img

                # Downscale & encode base64 for token-efficient model reasoning
                b64_str, resized_img = self.perceiver.capture_base64(
                    max_width=settings.screenshot_max_width
                )

                # 3. DECIDE: Dual-Gear Decision Routing (Gear 1: Jev Fast-Path vs Gear 2: Meta Muse Vision)
                history_ctx = self.memory.get_context_history()
                front_app = self.controller.macos.get_frontmost_app().get("app", "") if self.controller.macos.is_macos() else ""
                candidates_list = self.grounder.detect_ui_candidates(raw_before)
                fast_res = self.fast_engine.evaluate_candidates(
                    goal=goal,
                    candidates=candidates_list,
                    som_coords=mark_coords,
                    active_app=front_app,
                    history=history_ctx
                )

                if fast_res and fast_res.is_fast_path:
                    action = fast_res.action
                    logger.info(f"Gear 1 (Jev Fast-Path) triggered ({fast_res.latency_ms}ms, confidence: {fast_res.confidence:.2f}): {action.action}")
                else:
                    # Gear 2: Full Multimodal Meta Muse Frontier Model
                    history_ctx = self.memory.get_context_history()
                    action = await self.model.predict_action(
                        screenshot_b64=b64_str,
                        goal=goal,
                        history=history_ctx,
                        som_tags=mark_coords
                    )

                # Annotate action on image for visual telemetry
                if action.x is not None and action.y is not None:
                    annotated_img = self.grounder.annotate_action(
                        visual_input, action.action.value, action.x, action.y
                    )
                else:
                    annotated_img = visual_input

                # Check termination conditions
                if action.action == ActionType.TASK_COMPLETE:
                    self.state = AgentState.COMPLETED
                    result = ActionResult(
                        success=True,
                        action=action,
                        message=action.thought or "Goal completed successfully."
                    )
                    self.memory.record_step(
                        step_number=step_number,
                        action=action,
                        result=result,
                        raw_image=raw_before,
                        annotated_image=annotated_img
                    )
                    self._emit_telemetry(step_number, action, result, None)
                    break

                if action.action == ActionType.FAIL:
                    self.state = AgentState.FAILED
                    result = ActionResult(
                        success=False,
                        action=action,
                        message=action.thought or "Agent declared task unachievable."
                    )
                    self.memory.record_step(
                        step_number=step_number,
                        action=action,
                        result=result,
                        raw_image=raw_before,
                        annotated_image=annotated_img
                    )
                    self._emit_telemetry(step_number, action, result, None)
                    break

                # 4. ACT: Execute the action across OS
                action_coord = (action.x, action.y) if (action.x is not None and action.y is not None) else None
                result = self.controller.execute(action, som_element_map=mark_coords)

                # OS Animation settle delay
                await asyncio.sleep(settings.step_delay_seconds)

                # 5. VERIFY: Closed-loop visual change detection
                diff_result = None
                if settings.visual_verification:
                    raw_after = self.perceiver.capture()
                    diff_result = self.verifier.compare(
                        before=raw_before,
                        after=raw_after,
                        action_coord=action_coord
                    )
                    result.visual_change_detected = diff_result.has_changed

                # Record step in episodic memory
                step_record = self.memory.record_step(
                    step_number=step_number,
                    action=action,
                    result=result,
                    diff=diff_result,
                    raw_image=raw_before,
                    annotated_image=annotated_img
                )

                # Emit telemetry to listeners (Web UI, CLI)
                self._emit_telemetry(step_number, action, result, diff_result)

                step_number += 1

            if step_number > limit and self.state == AgentState.RUNNING:
                self.state = AgentState.FAILED
                logger.warning(f"Task reached maximum step limit ({limit}).")

        except Exception as e:
            self.state = AgentState.FAILED
            logger.error(f"Agent runtime exception: {e}", exc_info=True)
            return {
                "success": False,
                "state": self.state.value,
                "error": str(e),
                "total_steps": step_number - 1,
                "elapsed_seconds": round(time.time() - start_time, 2)
            }

        total_time = round(time.time() - start_time, 2)
        success = (self.state == AgentState.COMPLETED)
        return {
            "success": success,
            "state": self.state.value,
            "total_steps": len(self.memory.steps),
            "elapsed_seconds": total_time,
            "run_id": self.memory.run_id,
            "trajectory_file": str(self.memory.run_dir / "trajectory.json")
        }

    def _emit_telemetry(
        self,
        step_number: int,
        action: AgentAction,
        result: ActionResult,
        diff: Optional[Any]
    ) -> None:
        """Broadcast live step payload to all registered observers."""
        payload = {
            "step": step_number,
            "action": action.model_dump(),
            "result": result.model_dump(),
            "visual_diff": {
                "has_changed": diff.has_changed if diff else False,
                "diff_ratio": diff.diff_ratio if diff else 0.0,
                "description": diff.description if diff else ""
            } if diff else None,
            "timestamp": time.time(),
            "state": self.state.value
        }
        for cb in self.step_callbacks:
            try:
                cb(payload)
            except Exception as e:
                logger.error(f"Error in telemetry callback: {e}")
