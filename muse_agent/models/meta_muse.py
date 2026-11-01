from __future__ import annotations

import json
import logging
import re
from typing import Any, Dict, List, Optional
import httpx
from muse_agent.controller.base import ActionType, AgentAction
from muse_agent.models.base import BaseVisionModel, SYSTEM_PROMPT

logger = logging.getLogger("muse_agent.models.meta_muse")


class MetaMuseModel(BaseVisionModel):
    """
    Client for Meta AI Muse Frontier Models (e.g. Muse Spark 1.3, Muse Wonder).
    Communicates via Meta Model API or OpenAI-compatible endpoint with multimodal vision capabilities.
    """

    def __init__(
        self,
        api_key: str,
        model_name: str = "meta-muse-spark-1.3",
        endpoint: str = "https://api.meta.ai/v1/models/muse",
        temperature: float = 0.1,
        max_tokens: int = 2048
    ):
        self.api_key = api_key
        self.model_name = model_name
        self.endpoint = endpoint
        self.temperature = temperature
        self.max_tokens = max_tokens

    def _clean_json_response(self, text: str) -> str:
        """Strip markdown fences, reasoning tags, and clean whitespace."""
        # Remove <think>...</think> if present
        text = re.sub(r"<think>.*?</think>", "", text, flags=re.DOTALL)
        # Match ```json ... ``` or ``` ... ```
        fence_match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", text, re.DOTALL)
        if fence_match:
            return fence_match.group(1).strip()
        # Fallback: Find outermost curly braces
        start = text.find("{")
        end = text.rfind("}")
        if start != -1 and end != -1:
            return text[start:end + 1].strip()
        return text.strip()

    async def predict_action(
        self,
        screenshot_b64: str,
        goal: str,
        history: List[Dict[str, Any]],
        som_tags: Optional[Dict[int, Any]] = None
    ) -> AgentAction:
        """Query Meta Muse with the screenshot, current goal, and recent step history."""
        # Construct message payload
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT}
        ]

        # Context summary of prior steps
        history_summary = []
        for step in history[-6:]:  # Keep sliding window of last 6 steps
            history_summary.append(
                f"Step {step.get('step')}: Action={step.get('action')} | Result={step.get('result')} | "
                f"VisualChanged={step.get('visual_change')}"
            )
        hist_text = "\n".join(history_summary) if history_summary else "No previous actions taken yet."

        som_info = ""
        if som_tags:
            som_info = f"\nDetected UI Element Marks: {list(som_tags.keys())[:25]}..."

        user_prompt = (
            f"GOAL: {goal}\n\n"
            f"RECENT ACTION HISTORY:\n{hist_text}\n"
            f"{som_info}\n\n"
            f"Analyze the current screenshot and determine the single next optimal action to achieve the goal."
        )

        messages.append({
            "role": "user",
            "content": [
                {"type": "text", "text": user_prompt},
                {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:image/png;base64,{screenshot_b64}"
                    }
                }
            ]
        })

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json"
        }

        payload = {
            "model": self.model_name,
            "messages": messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "response_format": {"type": "json_object"}
        }

        # If no API key is provided, we raise a helpful error or fallback
        if not self.api_key:
            raise ValueError(
                "MUSE_API_KEY is not configured! Please provide your Meta Model API key in .env or AgentConfig."
            )

        async with httpx.AsyncClient(timeout=45.0) as client:
            resp = await client.post(self.endpoint, headers=headers, json=payload)
            resp.raise_for_status()
            data = resp.json()

        raw_content = data["choices"][0]["message"]["content"]
        cleaned_json = self._clean_json_response(raw_content)

        try:
            parsed = json.loads(cleaned_json)
            return AgentAction(**parsed)
        except Exception as e:
            logger.error(f"Failed to parse model response into AgentAction: {e}. Raw content: {raw_content}")
            return AgentAction(
                action=ActionType.WAIT,
                seconds=1.0,
                thought=f"Encountered parsing error, waiting before retry: {e}"
            )

