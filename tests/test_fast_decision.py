from muse_agent.controller.base import ActionType
from muse_agent.core.fast_decision import FastDecisionEngine


def test_fast_decision_app_launch():
    engine = FastDecisionEngine()
    res = engine.evaluate_candidates(
        goal="Open Notes and write a poem",
        candidates=[],
        som_coords={},
        active_app="Finder"
    )
    assert res is not None
    assert res.is_fast_path
    assert res.action.action == ActionType.LAUNCH_APP
    assert res.action.app_name == "Notes"
    assert res.confidence >= 0.9
    assert res.latency_ms < 50.0  # Must be lightning fast (<50ms)


def test_fast_decision_calculation():
    engine = FastDecisionEngine()
    res = engine.evaluate_candidates(
        goal="Calculate 25 * 40",
        candidates=[],
        som_coords={},
        active_app="Calculator"
    )
    assert res is not None
    assert res.is_fast_path
    assert res.action.action == ActionType.TYPE_TEXT
    assert "25*40=" in res.action.text
    assert res.confidence >= 0.9


def test_fast_decision_fallback_to_deep_vision():
    engine = FastDecisionEngine()
    # Ambiguous/exploratory goal without obvious shortcut
    res = engine.evaluate_candidates(
        goal="Find the blue button with the turtle logo on the page",
        candidates=[{"id": 1, "label": "Search", "role": "button"}],
        som_coords={1: (200, 300)},
        active_app="Safari"
    )
    # Should be None so agent escalates to Meta Muse Vision
    assert res is None


def test_fast_decision_media_playback():
    engine = FastDecisionEngine()
    res = engine.evaluate_candidates(
        goal="open chrome and play weekend song",
        candidates=[],
        som_coords={},
        active_app=""
    )
    assert res is not None
    assert res.is_fast_path
    assert res.action.action == ActionType.RUN_COMMAND
    assert "youtube.com" in res.action.command
    assert "weeknd" in res.action.command
    assert res.confidence >= 0.9


