from __future__ import annotations

import sys
from pathlib import Path

import pytest

from gait import GateEnforcementError, IntentContext, ToolAdapter, capture_intent

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
