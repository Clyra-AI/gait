from __future__ import annotations

import json
from pathlib import Path

from gait import (
    GateEvalResult,
    IntentContext,
    IntentRequest,
    IntentScript,
    IntentScriptStep,
    IntentTarget,
    TraceRecord,
)


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def _fixture(name: str) -> dict[str, object]:
    path = _repo_root() / "core" / "schema" / "testdata" / name
    return json.loads(path.read_text(encoding="utf-8"))


def test_intent_request_fixture_parses_with_sdk_model() -> None:
    payload = _fixture("gate_intent_request_valid.json")
    intent = IntentRequest.from_dict(payload)

    assert intent.schema_id == "gait.gate.intent_request"
    assert intent.tool_name == "tool.demo"
    assert intent.context.identity == "user"
    assert len(intent.targets) == 1
    assert intent.to_dict()["schema_version"] == "1.0.0"


def test_gate_result_fixture_parses_with_sdk_model() -> None:
    payload = _fixture("gate_result_valid.json")
    gate_result = GateEvalResult.from_dict(payload, exit_code=0)

    assert gate_result.ok is False
    assert gate_result.verdict == "allow"
    assert gate_result.reason_codes == []
    assert gate_result.violations == []
    assert gate_result.exit_code == 0


def test_trace_record_fixture_parses_with_sdk_model() -> None:
    payload = _fixture("gate_trace_record_valid.json")
    trace_record = TraceRecord.from_dict(payload)

    assert trace_record.schema_id == "gait.gate.trace"
    assert trace_record.trace_id == "trace_1"
    assert trace_record.verdict == "allow"
    assert trace_record.policy_digest == "3" * 64


def test_intent_request_script_round_trip() -> None:
    intent = IntentRequest(
        tool_name="script",
        args={},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
        script=IntentScript(
            steps=[
                IntentScriptStep(
                    tool_name="tool.read",
                    args={"path": "/tmp/in.txt"},
                    targets=[IntentTarget(kind="path", value="/tmp/in.txt", operation="read")],
                ),
                IntentScriptStep(
                    tool_name="tool.write",
                    args={"path": "/tmp/out.txt"},
                    targets=[IntentTarget(kind="path", value="/tmp/out.txt", operation="write")],
                ),
            ]
        ),
    )
    payload = intent.to_dict()
    restored = IntentRequest.from_dict(payload)

    assert restored.script is not None
    assert len(restored.script.steps) == 2
    assert restored.script.steps[0].tool_name == "tool.read"
    assert restored.script.steps[1].tool_name == "tool.write"


def test_gate_result_script_fields_parse() -> None:
    gate_result = GateEvalResult.from_dict(
        {
            "ok": True,
            "verdict": "allow",
            "reason_codes": ["approved_script_match"],
            "script": True,
            "step_count": 2,
            "script_hash": "a" * 64,
            "composite_risk_class": "high",
            "pre_approved": True,
            "pattern_id": "pattern_test",
            "registry_reason": "approved_script_match",
            "step_verdicts": [
                {"index": 0, "tool_name": "tool.read", "verdict": "allow"},
                {"index": 1, "tool_name": "tool.write", "verdict": "require_approval"},
            ],
        },
        exit_code=0,
    )

    assert gate_result.ok is True
    assert gate_result.script is True
    assert gate_result.step_count == 2
    assert gate_result.script_hash == "a" * 64
    assert gate_result.pre_approved is True
    assert gate_result.pattern_id == "pattern_test"
    assert len(gate_result.step_verdicts) == 2
