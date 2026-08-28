from __future__ import annotations

import importlib
import sys
from dataclasses import dataclass
from pathlib import Path

import pytest

from gait import (
    GaitError,
    GaitCommandError,
    GateEnforcementError,
    GateEvalResult,
    IntentContext,
    ToolAdapter,
    capture_intent,
)

from helpers import create_fake_gait_script, install_fake_langchain_modules


def load_gait_langchain_module() -> object:
    gait_langchain = importlib.import_module("gait.langchain")
    return importlib.reload(gait_langchain)


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


def test_tool_adapter_lifecycle_callback_only_after_execution(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    events: list[tuple[bool, object | None, BaseException | None]] = []
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
        lifecycle_callback=lambda _intent, _decision, result, error: events.append(
            (True, result, error)
        ),
    )
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)
    assert events == [(True, {"ok": True}, None)]
    with pytest.raises(RuntimeError):
        adapter.execute(
            intent=intent,
            executor=lambda _: (_ for _ in ()).throw(RuntimeError("boom")),
            cwd=tmp_path,
        )
    assert isinstance(events[-1][2], RuntimeError)


def test_tool_adapter_builtin_lifecycle_command_success_failure_and_block(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    calls: list[list[str]] = []

    def fake_run(command: list[str], cwd: str | Path | None = None) -> object:
        calls.append(command)
        return object()

    monkeypatch.setattr("gait.adapter._run_command", fake_run)
    decision = GateEvalResult(
        ok=True, exit_code=0, verdict="allow", trace_path=str(tmp_path / "trace.json")
    )
    monkeypatch.setattr(ToolAdapter, "gate_intent", lambda self, **kwargs: decision)
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        lifecycle_proposal="proposal.json",
        lifecycle_activation="activation.json",
        lifecycle_trace_public_key="trace.pub",
        lifecycle_activation_public_key="activation.pub",
        lifecycle_journal="events.jsonl",
        lifecycle_private_key="private.key",
        lifecycle_evaluation_time="2026-08-26T00:00:00Z",
    )
    intent = capture_intent(
        tool_name="tool.allow",
        args={"x": 1},
        context=IntentContext(identity="alice", workspace="/repo", risk_class="low"),
    )
    adapter.execute(intent=intent, executor=lambda _: {"ok": True}, cwd=tmp_path)
    assert (
        calls
        and "--outcome" in calls[-1]
        and calls[-1][calls[-1].index("--outcome") + 1] == "succeeded"
        and "--result-file" in calls[-1]
        and "--result-digest" not in calls[-1]
    )
    calls.clear()
    with pytest.raises(RuntimeError):
        adapter.execute(
            intent=intent,
            executor=lambda _: (_ for _ in ()).throw(RuntimeError("boom")),
            cwd=tmp_path,
        )
    assert (
        calls
        and calls[-1][calls[-1].index("--outcome") + 1] == "failed"
        and "--result-file" in calls[-1]
    )
    calls.clear()
    blocked = GateEvalResult(
        ok=True, exit_code=3, verdict="block", trace_path=str(tmp_path / "trace.json")
    )
    monkeypatch.setattr(ToolAdapter, "gate_intent", lambda self, **kwargs: blocked)
    with pytest.raises(GateEnforcementError):
        adapter.execute(intent=intent, executor=lambda _: {"never": True}, cwd=tmp_path)
    assert calls == []


def test_tool_adapter_lifecycle_configuration_is_complete_and_failures_observable(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    with pytest.raises(ValueError, match="lifecycle configuration"):
        ToolAdapter(policy_path=tmp_path / "policy.yaml", lifecycle_proposal="proposal.json")
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    decision = GateEvalResult(
        ok=True, exit_code=0, verdict="allow", trace_path=str(tmp_path / "trace.json")
    )
    monkeypatch.setattr(ToolAdapter, "gate_intent", lambda self, **kwargs: decision)
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
        lifecycle_proposal="proposal.json",
        lifecycle_activation="activation.json",
        lifecycle_trace_public_key="trace.pub",
        lifecycle_activation_public_key="activation.pub",
        lifecycle_journal="events.jsonl",
        lifecycle_private_key="private.key",
        lifecycle_evaluation_time="2026-08-26T00:00:00Z",
    )
    monkeypatch.setattr(
        "gait.adapter._run_command",
        lambda *args, **kwargs: (_ for _ in ()).throw(GaitError("lifecycle failed")),
    )
    with pytest.raises(GaitError, match="lifecycle failed"):
        adapter.execute(
            intent=capture_intent(
                tool_name="tool.allow",
                args={},
                context=IntentContext(identity="alice", workspace="/repo", risk_class="low"),
            ),
            executor=lambda _: {"ok": True},
            cwd=tmp_path,
        )


def test_tool_adapter_lifecycle_requires_trace_before_executor(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    monkeypatch.setattr(
        ToolAdapter,
        "gate_intent",
        lambda self, **kwargs: GateEvalResult(ok=True, exit_code=0, verdict="allow"),
    )
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
        lifecycle_proposal="proposal.json",
        lifecycle_activation="activation.json",
        lifecycle_trace_public_key="trace.pub",
        lifecycle_activation_public_key="activation.pub",
        lifecycle_journal="events.jsonl",
        lifecycle_private_key="private.key",
        lifecycle_evaluation_time="2026-08-26T00:00:00Z",
    )
    executed = False

    def executor(_: object) -> object:
        nonlocal executed
        executed = True
        return {"ok": True}

    with pytest.raises(GaitError, match="requires an explicit trace path"):
        adapter.execute(
            intent=capture_intent(
                tool_name="tool.allow",
                args={},
                context=IntentContext(identity="alice", workspace="/repo", risk_class="low"),
            ),
            executor=executor,
            cwd=tmp_path,
        )
    assert not executed


def test_tool_adapter_requires_authenticated_activation_prefix_before_executor(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    decision = GateEvalResult(
        ok=True, exit_code=0, verdict="allow", trace_path=str(tmp_path / "trace.json")
    )
    monkeypatch.setattr(ToolAdapter, "gate_intent", lambda self, **kwargs: decision)

    def reject_prefix(command: list[str], cwd: str | Path | None = None) -> object:
        if "lifecycle-verify" in command:
            raise GaitError("prefix is not activated")
        return object()

    monkeypatch.setattr("gait.adapter._run_command", reject_prefix)
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        lifecycle_proposal="proposal.json",
        lifecycle_activation="activation.json",
        lifecycle_trace_public_key="trace.pub",
        lifecycle_activation_public_key="activation.pub",
        lifecycle_journal="events.jsonl",
        lifecycle_private_key="private.key",
        lifecycle_evaluation_time="2026-08-26T00:00:00Z",
    )
    executed = False

    def executor(_: object) -> object:
        nonlocal executed
        executed = True
        return {"ok": True}

    with pytest.raises(GaitError, match="activation prefix verification"):
        adapter.execute(
            intent=capture_intent(
                tool_name="tool.allow",
                args={},
                context=IntentContext(identity="alice", workspace="/repo", risk_class="low"),
            ),
            executor=executor,
            cwd=tmp_path,
        )
    assert not executed


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


def test_tool_adapter_propagates_gate_command_failure_without_execution(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin="gait")
    intent = capture_intent(
        tool_name="tool.allow",
        args={"path": "/tmp/out.txt"},
        context=IntentContext(identity="alice", workspace="/repo/gait", risk_class="high"),
    )
    calls = {"count": 0}

    def _executor(_: object) -> dict[str, bool]:
        calls["count"] += 1
        return {"ok": True}

    def _failing_gate_intent(self: ToolAdapter, **_: object) -> GateEvalResult:
        raise GaitCommandError(
            "gate eval failed",
            command=["gait", "gate", "eval"],
            exit_code=6,
            stdout='{"ok":false}',
            stderr="intent parse error",
        )

    monkeypatch.setattr(ToolAdapter, "gate_intent", _failing_gate_intent)

    with pytest.raises(GaitCommandError):
        adapter.execute(intent=intent, executor=_executor, cwd=tmp_path)
    assert calls["count"] == 0


def test_langchain_middleware_requires_optional_dependency(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    gait_langchain = load_gait_langchain_module()
    monkeypatch.setattr(gait_langchain, "_LANGCHAIN_IMPORT_ERROR", ImportError("missing langchain"))
    adapter = ToolAdapter(policy_path=tmp_path / "policy.yaml", gait_bin="gait")

    with pytest.raises(ImportError, match="LangChain integration requires optional dependencies"):
        gait_langchain.GaitLangChainMiddleware(adapter)


def test_langchain_middleware_wraps_tool_execution_and_emits_metadata(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    tool_call_request = install_fake_langchain_modules()
    gait_langchain = load_gait_langchain_module()

    @dataclass(slots=True)
    class RuntimeContext:
        identity: str
        workspace: str
        risk_class: str
        run_id: str
        request_id: str
        auth_context: dict[str, str]

    @dataclass(slots=True)
    class Runtime:
        context: RuntimeContext

    callback = gait_langchain.GaitLangChainCallbackHandler()
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
    )
    middleware = gait_langchain.GaitLangChainMiddleware(
        adapter,
        trace_dir=tmp_path,
        callback_handler=callback,
    )

    request = tool_call_request(
        tool_call={
            "name": "tool.allow",
            "args": {"path": "/tmp/out.txt", "content": "ok"},
            "id": "call_langchain_allow",
        },
        tool=None,
        state={},
        runtime=Runtime(
            context=RuntimeContext(
                identity="agent-langchain",
                workspace="/repo/gait",
                risk_class="high",
                run_id="run_langchain",
                request_id="req_langchain",
                auth_context={"framework": "langchain"},
            )
        ),
    )
    calls = {"count": 0}

    def _handler(_: object) -> dict[str, bool]:
        calls["count"] += 1
        return {"ok": True}

    result = middleware.wrap_tool_call(request, _handler)

    assert result == {"ok": True}
    assert calls["count"] == 1
    assert len(callback.events) == 1
    metadata = callback.events[0]
    assert metadata.tool_name == "tool.allow"
    assert metadata.tool_call_id == "call_langchain_allow"
    assert metadata.run_id == "run_langchain"
    assert metadata.request_id == "req_langchain"
    assert metadata.auth_context == {"framework": "langchain"}
    assert metadata.executed is True
    assert metadata.verdict == "allow"
    assert metadata.trace_path is not None
    assert Path(metadata.trace_path).exists()
    assert metadata.policy_digest == "3" * 64
    assert metadata.intent_digest == "2" * 64


def test_langchain_middleware_blocks_without_running_handler(tmp_path: Path) -> None:
    fake_gait = tmp_path / "fake_gait.py"
    create_fake_gait_script(fake_gait)
    tool_call_request = install_fake_langchain_modules()
    gait_langchain = load_gait_langchain_module()

    @dataclass(slots=True)
    class Runtime:
        context: dict[str, object]

    callback = gait_langchain.GaitLangChainCallbackHandler()
    adapter = ToolAdapter(
        policy_path=tmp_path / "policy.yaml",
        gait_bin=[sys.executable, str(fake_gait)],
    )
    middleware = gait_langchain.GaitLangChainMiddleware(
        adapter,
        trace_dir=tmp_path,
        callback_handler=callback,
    )

    request = tool_call_request(
        tool_call={
            "name": "tool.approval",
            "args": {"path": "/tmp/out.txt"},
            "id": "call_langchain_approval",
        },
        tool=None,
        state={},
        runtime=Runtime(
            context={
                "identity": "agent-langchain",
                "workspace": "/repo/gait",
                "risk_class": "high",
                "run_id": "run_langchain",
                "request_id": "req_langchain",
                "auth_context": {"framework": "langchain"},
            }
        ),
    )
    calls = {"count": 0}

    def _handler(_: object) -> dict[str, bool]:
        calls["count"] += 1
        return {"ok": True}

    with pytest.raises(GateEnforcementError) as error:
        middleware.wrap_tool_call(request, _handler)

    assert error.value.decision.verdict == "require_approval"
    assert calls["count"] == 0
    assert len(callback.events) == 1
    assert callback.events[0].executed is False
    assert callback.events[0].verdict == "require_approval"
    assert callback.events[0].trace_path is not None
