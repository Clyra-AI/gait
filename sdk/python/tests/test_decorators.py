from __future__ import annotations

import sys
from pathlib import Path

import pytest

from gait import GateEnforcementError, IntentContext, ToolAdapter, gate_tool

from helpers import create_fake_gait_script


def test_gate_tool_executes_allow_and_writes_trace(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)])
    trace_path = tmp_path / "trace_allow.json"

    @gate_tool(
        adapter=adapter,
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
        tool_name="tool.allow",
        trace_out=trace_path,
    )
    def write_file(path: str, content: str) -> dict[str, str]:
        return {"path": path, "content": content}

    output = write_file("/tmp/out.txt", "hello")
    assert output == {"path": "/tmp/out.txt", "content": "hello"}
    assert trace_path.exists()


def test_gate_tool_blocks_non_allow_without_execution(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)])
    calls = {"count": 0}

    @gate_tool(
        adapter=adapter,
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
        tool_name="tool.block",
    )
    def delete_file(path: str) -> dict[str, bool]:
        calls["count"] += 1
        return {"deleted": True}

    with pytest.raises(GateEnforcementError):
        delete_file("/tmp/out.txt")
    assert calls["count"] == 0


def test_gate_tool_fails_closed_for_dry_run(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)])
    calls = {"count": 0}

    @gate_tool(
        adapter=adapter,
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
        tool_name="tool.dry",
    )
    def write_file(path: str) -> dict[str, bool]:
        calls["count"] += 1
        return {"ok": True}

    with pytest.raises(GateEnforcementError):
        write_file("/tmp/out.txt")
    assert calls["count"] == 0


def test_gate_tool_supports_context_and_trace_resolvers(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin=[sys.executable, str(fake_gait)])

    @gate_tool(
        adapter=adapter,
        context=lambda args, kwargs: IntentContext(
            identity=str(kwargs.get("actor", "unknown")),
            workspace="/repo/gait",
            risk_class="high",
        ),
        tool_name="tool.allow",
        trace_out=lambda args, kwargs: tmp_path / f"trace_{kwargs['actor']}.json",
    )
    def export_report(path: str, actor: str) -> str:
        return f"exported:{path}:{actor}"

    output = export_report("/tmp/report.json", actor="bob")
    assert output == "exported:/tmp/report.json:bob"
    assert (tmp_path / "trace_bob.json").exists()
