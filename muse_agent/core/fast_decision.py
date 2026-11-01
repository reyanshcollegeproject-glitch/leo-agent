from __future__ import annotations

import logging
import os
import re
import time
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple
import httpx

from muse_agent.controller.base import ActionType, AgentAction

logger = logging.getLogger("muse_agent.core.fast_decision")


@dataclass
class FastDecisionResult:
    action: AgentAction
    confidence: float
    is_fast_path: bool
    latency_ms: float
    evidence: str
    needs_review: bool


class FastDecisionEngine:
    """
    Jev-inspired Fast-Path Decision & Click Engine.
    Transforms open-ended GUI reasoning into a calibrated discrete choice problem.
    Enables sub-200ms click selection when UI candidates are unambiguous,
    escalating to deep vision only when visual verification or complex spatial reasoning is needed.
    """

    def __init__(
        self,
        confidence_threshold: float = 0.85,
        typesafe_api_key: Optional[str] = None,
        openrouter_api_key: Optional[str] = None
    ):
        self.confidence_threshold = confidence_threshold
        self.typesafe_api_key = typesafe_api_key or os.getenv("TYPESAFE_API_KEY", "")
        self.openrouter_api_key = openrouter_api_key or os.getenv("OPENROUTER_API_KEY", "")

    def evaluate_candidates(
        self,
        goal: str,
        candidates: List[Dict[str, Any]],
        som_coords: Dict[int, Tuple[int, int]],
        active_app: str = "",
        history: Optional[List[Dict[str, Any]]] = None
    ) -> Optional[FastDecisionResult]:
        """
        Fast local semantic scoring over candidate actions.
        Returns FastDecisionResult if confidence exceeds threshold, else None (routes to deep vision).
        """
        start_t = time.perf_counter()
        goal_lower = goal.lower()
        history = history or []

        past_actions = [h.get("action", "") for h in history]

        # Rule 1: High-confidence direct app activation
        app_match = re.search(r"(?:open|launch|switch to)\s+([a-zA-Z0-9\s]+?)(?:\s+(?:app|and|$))", goal_lower)
        if app_match:
            target_app = app_match.group(1).strip().title()
            already_launched = any(
                h.get("action") == ActionType.LAUNCH_APP.value or target_app.lower() in str(h).lower()
                for h in history
            )
            if active_app.lower() != target_app.lower() and not already_launched:
                latency = (time.perf_counter() - start_t) * 1000.0
                return FastDecisionResult(
                    action=AgentAction(
                        action=ActionType.LAUNCH_APP,
                        app_name=target_app,
                        thought=f"[FastPath-Jev] Goal demands launching application '{target_app}' directly."
                    ),
                    confidence=0.98,
                    is_fast_path=True,
                    latency_ms=round(latency, 2),
                    evidence=f"Direct app invocation for '{target_app}' bypasses visual search.",
                    needs_review=False
                )

        # Rule 2: High-confidence direct calculation
        if "calculate" in goal_lower and ("+" in goal_lower or "*" in goal_lower or "-" in goal_lower or "/" in goal_lower):
            calc_match = re.search(r"calculate\s+([0-9\s\+\-\*\/\.\=]+)", goal_lower)
            if calc_match:
                expr = calc_match.group(1).replace(" ", "")
                if not expr.endswith("="):
                    expr += "="

                already_typed = any(
                    h.get("action") == ActionType.TYPE_TEXT.value for h in history
                )
                if not already_typed:
                    latency = (time.perf_counter() - start_t) * 1000.0
                    return FastDecisionResult(
                        action=AgentAction(
                            action=ActionType.TYPE_TEXT,
                            text=expr,
                            thought=f"[FastPath-Jev] Numerical calculation expression identified: '{expr}'"
                        ),
                        confidence=0.96,
                        is_fast_path=True,
                        latency_ms=round(latency, 2),
                        evidence="Calculator input sequence recognized without coordinate ambiguity.",
                        needs_review=False
                    )
                else:
                    # Calculation expression already typed -> Complete task
                    latency = (time.perf_counter() - start_t) * 1000.0
                    return FastDecisionResult(
                        action=AgentAction(
                            action=ActionType.TASK_COMPLETE,
                            thought=f"[FastPath-Jev] Calculation expression '{expr}' executed successfully."
                        ),
                        confidence=0.99,
                        is_fast_path=True,
                        latency_ms=round(latency, 2),
                        evidence="Calculation step confirmed in history.",
                        needs_review=False
                    )

        # Rule 3: High-confidence candidate element targeting
        # When candidate marks match keywords in goal
        best_candidate = None
        best_score = 0.0

        for cand in candidates:
            label = cand.get("label", "").lower()
            role = cand.get("role", "").lower()
            elem_id = cand["id"]

            if not label:
                continue

            score = 0.0
            # Token overlap scoring
            for word in goal_lower.split():
                if len(word) > 2 and word in label:
                    score += 0.4
            if role in ("button", "link", "menuitem") and score > 0:
                score += 0.2

            if score > best_score:
                best_score = score
                best_candidate = cand

        if best_candidate and best_score >= self.confidence_threshold:
            elem_id = best_candidate["id"]
            cx, cy = som_coords.get(elem_id, (best_candidate.get("cx", 500), best_candidate.get("cy", 500)))
            latency = (time.perf_counter() - start_t) * 1000.0
            return FastDecisionResult(
                action=AgentAction(
                    action=ActionType.CLICK,
                    element_id=elem_id,
                    x=cx,
                    y=cy,
                    thought=f"[FastPath-Jev] Confident match for element #{elem_id} ('{best_candidate.get('label')}') with score {best_score:.2f}."
                ),
                confidence=best_score,
                is_fast_path=True,
                latency_ms=round(latency, 2),
                evidence=f"Element #{elem_id} matches goal semantics above calibrated threshold.",
                needs_review=False
            )

        # Below threshold: Escalate to Gear 2 (Meta Muse Deep Vision)
        return None

    async def query_jev_api(
        self,
        goal: str,
        elements: Dict[str, Any],
        allowed_actions: List[str]
    ) -> Optional[FastDecisionResult]:
        """
        External query to TypeSafe Jev or OpenRouter decision endpoint if keys are configured.
        """
        if not self.typesafe_api_key and not self.openrouter_api_key:
            return None

        endpoint = "https://api.typesafe.ai/v1/decide" if self.typesafe_api_key else "https://openrouter.ai/api/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {self.typesafe_api_key or self.openrouter_api_key}",
            "Content-Type": "application/json"
        }

        criteria = {act: f"Execute {act} toward goal '{goal}'" for act in allowed_actions}
        payload = {
            "model": "typesafe/jev-1.13",
            "state": {
                "goal": goal,
                "elements": elements,
                "allowed_actions": allowed_actions
            },
            "questions": {
                "action": {
                    "type": "choice",
                    "instructions": "Choose the single next allowed action.",
                    "criteria": criteria
                }
            }
        }

        try:
            start_t = time.perf_counter()
            async with httpx.AsyncClient(timeout=3.0) as client:
                res = await client.post(endpoint, json=payload, headers=headers)
                if res.status_code == 200:
                    data = res.json()
                    choice = data.get("answers", {}).get("action", {}).get("value")
                    prob = data.get("answers", {}).get("action", {}).get("probability", 0.9)
                    latency = (time.perf_counter() - start_t) * 1000.0

                    return FastDecisionResult(
                        action=AgentAction(action=ActionType.CLICK, thought=f"Jev chosen: {choice}"),
                        confidence=float(prob),
                        is_fast_path=True,
                        latency_ms=round(latency, 2),
                        evidence=f"Jev API returned '{choice}' with probability {prob}.",
                        needs_review=(prob < self.confidence_threshold)
                    )
        except Exception as e:
            logger.warning(f"Jev API query skipped: {e}")

        return None
