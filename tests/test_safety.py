import pytest
from muse_agent.controller.base import ActionType, AgentAction
from muse_agent.core.safety import SentinelGuardrail, SafetyViolationError, EmergencyStopError


def test_sentinel_blocks_destructive_rm():
    sentinel = SentinelGuardrail()
    action = AgentAction(action=ActionType.RUN_COMMAND, command="rm -rf /")
    with pytest.raises(SafetyViolationError) as exc_info:
        sentinel.validate_action(action)
    assert "Destructive command detected" in str(exc_info.value)


def test_sentinel_blocks_fork_bomb():
    sentinel = SentinelGuardrail()
    action = AgentAction(action=ActionType.RUN_COMMAND, command=":(){ :|:& };:")
    with pytest.raises(SafetyViolationError):
        sentinel.validate_action(action)


def test_sentinel_allows_safe_command():
    sentinel = SentinelGuardrail(require_confirmation=False)
    action = AgentAction(action=ActionType.RUN_COMMAND, command="ls -la")
    # Should not raise
    sentinel.validate_action(action)


def test_sentinel_emergency_stop():
    sentinel = SentinelGuardrail()
    sentinel.trigger_emergency_stop()
    action = AgentAction(action=ActionType.CLICK, x=100, y=100)
    with pytest.raises(EmergencyStopError):
        sentinel.validate_action(action)
