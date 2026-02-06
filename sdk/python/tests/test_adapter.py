from __future__ import annotations

import sys
from pathlib import Path

import pytest

from gait import (
    GateEnforcementError,
    GateEvalResult,
    IntentContext,
    ToolAdapter,
    capture_intent,
)

from helpers import create_fake_gait_script


def test_tool_adapter_executes_allowed_intent(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)]
    )
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    outcome = adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)
    assert outcome.executed
    assert outcome.result == {"ok": True}
    assert outcome.decision.verdict == "allow"


def test_tool_adapter_blocks_high_risk_intent(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)]
    )
    intent = capture_intent(
        tool_name="tool.block",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    with pytest.raises(GateEnforcementError):
        adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)


def test_tool_adapter_capture_and_regress_helpers(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)]
    )
    demo = adapter.capture_runpack(cwd=tmp_path)
    assert demo.run_id == "run_demo"

    fixture = adapter.create_regression_fixture(from_run=demo.run_id, cwd=tmp_path)
    assert fixture.fixture_name == "run_demo"


def test_tool_adapter_requires_approval(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)]
    )
    intent = capture_intent(
        tool_name="tool.approval",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    with pytest.raises(GateEnforcementError):
        adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)


def test_tool_adapter_dry_run_skips_execution(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)]
    )
    intent = capture_intent(
        tool_name="tool.dry",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    calls = {"count": 0}

    def _executor(_: object) -> dict[str, bool]:
        calls["count"] += 1
        return {"ok": True}

    outcome = adapter.execute(intent=intent, executor=_executor, cwd=tmp_path)
    assert not outcome.executed
    assert outcome.result is None
    assert outcome.decision.verdict == "dry_run"
    assert calls["count"] == 0


def test_tool_adapter_fails_closed_on_unexpected_verdict(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin="gait")
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    monkeypatch.setattr(
        ToolAdapter,
        "gate_intent",
        lambda self, **_: GateEvalResult(
            ok=True,
            exit_code=0,
            verdict="unknown",
            reason_codes=[],
            violations=[],
        ),
    )

    with pytest.raises(GateEnforcementError):
        adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)


def test_tool_adapter_fails_closed_when_decision_not_ok(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin="gait")
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    monkeypatch.setattr(
        ToolAdapter,
        "gate_intent",
        lambda self, **_: GateEvalResult(
            ok=False,
            exit_code=6,
            verdict="allow",
            reason_codes=["invalid_intent"],
            violations=[],
        ),
    )

    with pytest.raises(GateEnforcementError):
        adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)
