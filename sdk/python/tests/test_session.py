from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

from gait import GateEnforcementError, IntentContext, ToolAdapter, capture_intent, run_session

from helpers import create_fake_gait_script


def test_run_session_captures_attempts_and_emits_runpack(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    capture_input_path = tmp_path / "record_input.json"
    monkeypatch.setenv("FAKE_GAIT_RECORD_CAPTURE", str(capture_input_path))

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
    )

    allow_intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/ok.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    block_intent = capture_intent(
        tool_name="tool.block",
        args={"path": "/tmp/block.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    with run_session(
        run_id="run_sdk_session",
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
        out_dir=tmp_path / "gait-out",
    ) as session:
        outcome = adapter.execute(
            intent=allow_intent,
            executor=lambda _: {"status": "ok"},
            cwd=tmp_path,
        )
        assert outcome.executed
        with pytest.raises(GateEnforcementError):
            adapter.execute(
                intent=block_intent, executor=lambda _: {"status": "never"}, cwd=tmp_path
            )

    assert session.capture is not None
    assert session.capture.run_id == "run_sdk_session"
    assert session.capture.bundle_path.endswith("runpack_run_sdk_session.zip")
    assert len(session.attempts) == 2
    assert session.attempts[0].status == "ok"
    assert session.attempts[1].status == "block"

    record_input = json.loads(capture_input_path.read_text(encoding="utf-8"))
    assert record_input["run"]["run_id"] == "run_sdk_session"
    assert len(record_input["intents"]) == 2
    assert len(record_input["results"]) == 2
    assert record_input["capture_mode"] == "reference"


def test_run_session_records_executor_errors(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    capture_input_path = tmp_path / "record_input_error.json"
    monkeypatch.setenv("FAKE_GAIT_RECORD_CAPTURE", str(capture_input_path))

    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
    )
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/ok.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    with run_session(
        run_id="run_sdk_error",
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
        out_dir=tmp_path / "gait-out",
    ) as session:
        with pytest.raises(RuntimeError, match="boom"):
            adapter.execute(
                intent=intent,
                executor=lambda _: (_ for _ in ()).throw(RuntimeError("boom")),
                cwd=tmp_path,
            )

    assert len(session.attempts) == 1
    assert session.attempts[0].status == "error"
    record_input = json.loads(capture_input_path.read_text(encoding="utf-8"))
    assert record_input["results"][0]["status"] == "error"
