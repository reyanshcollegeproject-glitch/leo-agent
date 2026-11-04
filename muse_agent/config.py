from __future__ import annotations

import os
from pathlib import Path
from typing import List, Optional
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class AgentConfig(BaseSettings):
    """
    Configuration for the Meta Muse Autonomous Laptop Control Agent.
    Supports environment variables and .env file loading.
    """
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore"
    )

    # --- Model Configuration ---
    # Meta AI Muse model family settings
    model_provider: str = Field(
        default="meta_muse",
        description="Provider identifier: 'meta_muse', 'openai', 'anthropic', or 'custom'"
    )
    muse_control_model: str = Field(
        default=os.getenv("MUSE_CONTROL_MODEL", "muse-wonder-1.0"),
        description="Meta AI model for deep computer control & tool planning"
    )
    muse_talk_model: str = Field(
        default=os.getenv("MUSE_TALK_MODEL", "muse-spark-1.3"),
        description="Meta AI model for fast natural conversation & real-time voice speech"
    )
    muse_model_name: str = Field(
        default="muse-wonder-1.0",
        description="Default Meta AI Muse model version"
    )
    muse_api_key: Optional[str] = Field(
        default=os.getenv("MUSE_API_KEY", ""),
        description="API Key for Meta Model API / Muse endpoint"
    )
    muse_endpoint: str = Field(
        default=os.getenv("MUSE_ENDPOINT", "https://api.meta.ai/v1/chat/completions"),
        description="Base URL for Meta Muse API"
    )
    fallback_api_key: Optional[str] = Field(
        default=os.getenv("OPENAI_API_KEY", os.getenv("ANTHROPIC_API_KEY", "")),
        description="Fallback API key for OpenAI or Anthropic"
    )
    temperature: float = Field(default=0.1, description="Sampling temperature for deterministic agent reasoning")
    max_tokens: int = Field(default=2048, description="Maximum tokens per step reasoning")

    # --- Screen Perception & Grounding Configuration ---
    display_index: int = Field(default=0, description="Display index (0 = primary monitor)")
    screenshot_max_width: int = Field(
        default=1440,
        description="Max width to resize screenshots for VLM token efficiency while retaining sharpness"
    )
    screenshot_format: str = Field(default="PNG", description="Screenshot image encoding format")
    use_coordinate_normalization: bool = Field(
        default=True,
        description="Normalize coordinates to [0, 1000] space for model grounding accuracy"
    )
    use_grid_overlay: bool = Field(
        default=True,
        description="Annotate screenshot with visual reference coordinate grid for zero-shot grounding"
    )
    use_set_of_marks: bool = Field(
        default=True,
        description="Detect interactive UI elements and tag with numeric Set-of-Marks labels"
    )

    # --- Execution & Verification Configuration ---
    step_delay_seconds: float = Field(
        default=0.8,
        description="Time to pause after an action to allow OS animations and UI rendering to settle"
    )
    max_steps_per_task: int = Field(
        default=30,
        description="Guardrail maximum steps before prompting for guidance or aborting"
    )
    action_timeout_seconds: float = Field(default=15.0, description="Timeout for individual action execution")
    visual_verification: bool = Field(
        default=True,
        description="Verify visual state changes before and after each executed action"
    )
    diff_threshold: float = Field(
        default=0.005,
        description="Threshold of pixel difference ratio to consider screen state changed"
    )

    # --- Safety & Sentinel Guardrails ---
    enable_sentinel_guardrails: bool = Field(
        default=True,
        description="Enable Meta Muse Sentinel permission and destructive action interceptor"
    )
    require_confirmation_for_destructive: bool = Field(
        default=True,
        description="Block destructive operations (rm -rf, deleting files, sending emails) without confirmation"
    )
    blocked_commands: List[str] = Field(
        default_factory=lambda: [
            "rm -rf /",
            "rm -rf ~",
            ":(){ :|:& };:",
            "mkfs",
            "dd if=",
            "chmod -R 777 /",
            "shutdown",
            "reboot"
        ]
    )
    emergency_fail_safe_corner: bool = Field(
        default=True,
        description="Moving mouse to top-left (0, 0) immediately triggers an emergency stop"
    )

    # --- Logging & Telemetry ---
    save_trajectories: bool = Field(default=True, description="Save trajectory data, actions, and screenshots")
    trajectories_dir: Path = Field(default=Path("runs"), description="Directory to store run history")
    benchmark_dir: Path = Field(default=Path("benchmark_results"), description="Directory to store benchmark data")
    server_port: int = Field(default=8765, description="Port for real-time web observer dashboard")
    server_host: str = Field(default="127.0.0.1", description="Host address for web server")


# Global configuration instance
settings = AgentConfig()

