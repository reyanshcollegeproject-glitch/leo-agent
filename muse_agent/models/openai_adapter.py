from __future__ import annotations

import json
import logging
import re
from typing import Any, Dict, List, Optional
import httpx
from muse_agent.controller.base import ActionType, AgentAction
from muse_agent.models.base import BaseVisionModel, SYSTEM_PROMPT

logger = logging.getLogger("muse_agent.models.openai_adapter")


class OpenAICompatibleModel(BaseVisionModel):
    """
    Adapter for standard OpenAI-compatible Vision API endpoints (e.g. OpenAI, vLLM, Groq, Ollama).
    """

    def __init__(
        self,
        api_key: str,
        model_name: str = "gpt-4o",
        endpoint: str = "https://api.openai.com/v1/chat/completions",
        temperature: float = 0.1,
        max_tokens: int = 2048
    ):
        self.api_key = api_key
        self.model_name = model_name
        self.endpoint = endpoint
        self.temperature = temperature
        self.max_tokens = max_tokens

    def _clean_json_response(self, text: str) -> str:
        text = re.sub(r"<think>.*?</think>", "", text, flags=re.DOTALL)
        fence_match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", text, re.DOTALL)
        if fence_match:
            return fence_match.group(1).strip()
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
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT}
        ]

        history_summary = []
        for step in history[-6:]:
            history_summary.append(
                f"Step {step.get('step')}: Action={step.get('action')} | Result={step.get('result')}"
            )
        hist_text = "\n".join(history_summary) if history_summary else "No previous actions taken yet."

        user_prompt = (
            f"GOAL: {goal}\n\n"
            f"ACTION HISTORY:\n{hist_text}\n\n"
            f"Determine the single next action to take in JSON format."
        )

        messages.append({
            "role": "user",
            "content": [
                {"type": "text", "text": user_prompt},
                {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:image/png;base64,{screenshot_b64}",
                        "detail": "high"
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

        async with httpx.AsyncClient(timeout=45.0) as client:
            resp = await client.post(self.endpoint, headers=headers, json=payload)
            resp.raise_for_status()
            data = resp.json()

        raw_content = data["choices"][0]["message"]["content"]
        cleaned_json = self._clean_json_response(raw_content)

        parsed = json.loads(cleaned_json)
        return AgentAction(**parsed)
