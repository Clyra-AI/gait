from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import pytest

from gait import client as client_module
from gait import (
    IntentContext,
    IntentRequest,
    IntentTarget,
    capture_demo_runpack,
    capture_intent,
    create_regress_fixture,
    evaluate_gate,
    record_runpack,
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
    assert result.trace_path is not None
    assert result.trace_path.endswith("trace_fake.json")


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


def test_record_runpack_round_trip(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    capture_path = tmp_path / "captured_record_input.json"

    result = record_runpack(
        record_input={
            "run": {
                "schema_id": "gait.runpack.run",
                "schema_version": "1.0.0",
                "created_at": "2026-02-12T00:00:00Z",
                "producer_version": "0.0.0-dev",
                "run_id": "run_sdk",
                "env": {"os": "darwin", "arch": "arm64", "runtime": "python3.13"},
                "timeline": [{"event": "run_started", "ts": "2026-02-12T00:00:00Z"}],
            },
            "intents": [],
            "results": [],
            "refs": {
                "schema_id": "gait.runpack.refs",
                "schema_version": "1.0.0",
                "created_at": "2026-02-12T00:00:00Z",
                "producer_version": "0.0.0-dev",
                "run_id": "run_sdk",
                "receipts": [],
            },
            "capture_mode": "reference",
        },
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
        out_dir=tmp_path / "gait-out",
    )

    assert result.run_id == "run_sdk"
    assert result.bundle_path.endswith("runpack_run_sdk.zip")
    assert result.manifest_digest == "4" * 64
    assert result.ticket_footer.startswith("GAIT run_id=run_sdk")

    with pytest.MonkeyPatch.context() as monkeypatch:
        monkeypatch.setenv("FAKE_GAIT_RECORD_CAPTURE", str(capture_path))
        record_runpack(
            record_input={
                "run": {
                    "schema_id": "gait.runpack.run",
                    "schema_version": "1.0.0",
                    "created_at": "2026-02-12T00:00:00Z",
                    "producer_version": "0.0.0-dev",
                    "run_id": "run_capture",
                    "env": {"os": "darwin", "arch": "arm64", "runtime": "python3.13"},
                    "timeline": [{"event": "run_started", "ts": "2026-02-12T00:00:00Z"}],
                },
                "intents": [],
                "results": [],
                "refs": {
                    "schema_id": "gait.runpack.refs",
                    "schema_version": "1.0.0",
                    "created_at": "2026-02-12T00:00:00Z",
                    "producer_version": "0.0.0-dev",
                    "run_id": "run_capture",
                    "receipts": [],
                },
                "capture_mode": "reference",
            },
            gait_bin=[sys.executable, str(fake_gait)],
            cwd=tmp_path,
            out_dir=tmp_path / "gait-out",
        )

    captured = json.loads(capture_path.read_text(encoding="utf-8"))
    assert captured["run"]["run_id"] == "run_capture"


def test_evaluate_gate_with_all_optional_key_flags(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)

    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    result = evaluate_gate(
        policy_path=tmp_path / "policy.yaml",
        intent=intent,
        gait_bin=[sys.executable, str(fake_gait)],
        cwd=tmp_path,
        trace_out=tmp_path / "trace.json",
        approval_token=tmp_path / "approval.json",
        private_key=tmp_path / "private.key",
        private_key_env="GAIT_PRIVATE_KEY",
        approval_public_key=tmp_path / "approval.pub",
        approval_public_key_env="GAIT_APPROVAL_PUBLIC_KEY",
        approval_private_key=tmp_path / "approval.key",
        approval_private_key_env="GAIT_APPROVAL_PRIVATE_KEY",
    )

    assert result.ok
    assert result.exit_code == 0


def test_internal_helpers_parse_json_and_prefix() -> None:
    assert client_module._parse_json_stdout("") is None
    assert client_module._parse_json_stdout("[]") is None
    assert client_module._parse_json_stdout('{"ok": true}') == {"ok": True}
    assert client_module._command_prefix("gait") == ["gait"]
    assert client_module._command_prefix(["python", "script.py"]) == ["python", "script.py"]


def test_run_command_timeout_raises_command_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def timeout_run(*args: object, **kwargs: object) -> object:
        raise subprocess.TimeoutExpired(cmd=["gait", "demo"], timeout=0.01)

    monkeypatch.setattr(client_module.subprocess, "run", timeout_run)
    with pytest.raises(client_module.GaitCommandError) as raised:
        client_module._run_command(["gait", "demo"], cwd=None)
    assert "timed out" in str(raised.value)


def test_evaluate_gate_malformed_json_raises_command_error(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )

    monkeypatch.setattr(
        client_module,
        "_run_command",
        lambda command, cwd=None: client_module._CommandResult(
            command=list(command),
            exit_code=0,
            stdout="not-json",
            stderr="",
        ),
    )

    with pytest.raises(client_module.GaitCommandError) as raised:
        evaluate_gate(policy_path=tmp_path / "policy.yaml", intent=intent, gait_bin="gait")
    assert "failed to parse JSON" in str(raised.value)


def test_create_regress_fixture_malformed_json_raises_command_error(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(
        client_module,
        "_run_command",
        lambda command, cwd=None: client_module._CommandResult(
            command=list(command),
            exit_code=0,
            stdout="{}[]",
            stderr="",
        ),
    )

    with pytest.raises(client_module.GaitCommandError) as raised:
        create_regress_fixture(from_run="run_demo", gait_bin="gait", cwd=tmp_path)
    assert "failed to parse JSON" in str(raised.value)


def test_record_runpack_invalid_capture_mode_raises(tmp_path: Path) -> None:
    with pytest.raises(client_module.GaitError):
        record_runpack(
            record_input={},
            gait_bin="gait",
            cwd=tmp_path,
            capture_mode="invalid",
        )
