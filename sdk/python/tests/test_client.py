from __future__ import annotations

import json
import sys
from pathlib import Path

from gait import (
    IntentContext,
    IntentRequest,
    IntentTarget,
    capture_demo_runpack,
    capture_intent,
    create_regress_fixture,
    evaluate_gate,
    write_trace,
)

from helpers import create_fake_gait_script


def test_capture_intent_and_evaluate_gate_allow(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        targets=[IntentTarget(kind="path", value="/tmp/out.txt")],
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    result = evaluate_gate(
        policy_path=tmp_path / "policy.yaml",
        intent=intent,
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
    )

    assert isinstance(intent, IntentRequest)
    assert result.exit_code == 0
    assert result.ok
    assert result.verdict == "allow"
    assert result.reason_codes == ["default_allow"]
    assert result.trace_path == "trace_fake.json"


def test_evaluate_gate_require_approval_exit_code(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    intent = capture_intent(
        tool_name="tool.approval",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    result = evaluate_gate(
        policy_path=tmp_path / "policy.yaml",
        intent=intent,
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
    )

    assert result.exit_code == 4
    assert result.verdict == "require_approval"
    assert result.reason_codes == ["approval_required"]


def test_write_trace_copies_source_record(tmp_path: Path) -> None:
    source = tmp_path / "trace.json"
    trace_payload = {
        "schema_id": "gait.gate.trace",
        "schema_version": "1.0.0",
        "created_at": "2026-02-05T00:00:00Z",
        "producer_version": "0.0.0-dev",
        "trace_id": "trace_123",
        "tool_name": "tool.write",
        "args_digest": "1" * 64,
        "intent_digest": "2" * 64,
        "policy_digest": "3" * 64,
        "verdict": "allow",
    }
    source.write_text(json.dumps(trace_payload, indent=2) + "\n", encoding="utf-8")

    destination = tmp_path / "out" / "trace.json"
    written = write_trace(trace_path=source, destination_path=destination)

    assert written == destination
    assert destination.read_text(encoding="utf-8") == source.read_text(encoding="utf-8")


def test_capture_demo_and_create_regress_fixture(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    demo = capture_demo_runpack(gait_bin=[sys.executable, str(fake_gait)], cwd=tmp_path)
    assert demo.run_id == "run_demo"
    assert demo.verified
    assert demo.bundle_path == "./gait-out/runpack_run_demo.zip"

    fixture = create_regress_fixture(
        from_run=demo.run_id,
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
    )
    assert fixture.run_id == "run_demo"
    assert fixture.fixture_name == "run_demo"
    assert fixture.runpack_path == "fixtures/run_demo/runpack.zip"
