from __future__ import annotations
# mypy: disable-error-code=import-not-found

import os
import re
from pathlib import Path
from typing import Any, Callable, Mapping, Protocol

from .adapter import GateEnforcementError, ToolAdapter
from .client import capture_intent
from .models import IntentContext, IntentRequest, LangChainDecisionMetadata

_AgentMiddlewareBase: type[Any]
_ToolCallRequest: Any
_CallbackHandlerBase: type[Any]

try:
    from langchain.agents.middleware import AgentMiddleware as _ImportedAgentMiddleware
    from langchain.tools.tool_node import ToolCallRequest as _ImportedToolCallRequest
    from langchain_core.callbacks.base import BaseCallbackHandler as _ImportedCallbackHandler
except ImportError as error:
    _LANGCHAIN_IMPORT_ERROR: ImportError | None = error
    _AgentMiddlewareBase = object
    _ToolCallRequest = Any
    _CallbackHandlerBase = object
else:
    _LANGCHAIN_IMPORT_ERROR = None
    _AgentMiddlewareBase = _ImportedAgentMiddleware
    _ToolCallRequest = _ImportedToolCallRequest
    _CallbackHandlerBase = _ImportedCallbackHandler


class SupportsGaitDecision(Protocol):
    def on_gait_decision(self, metadata: LangChainDecisionMetadata) -> None:
        """Capture additive correlation metadata after a gate decision."""


IntentFactory = Callable[[Any], IntentRequest]
ContextFactory = Callable[[Any], IntentContext]
TracePathFactory = Callable[[Any], str | Path | None]


class GaitLangChainCallbackHandler(_CallbackHandlerBase):  # type: ignore[misc]
    """Optional correlation-only sink for middleware decision metadata."""

    def __init__(
        self,
        *,
        sink: Callable[[LangChainDecisionMetadata], None] | None = None,
    ) -> None:
        self.events: list[LangChainDecisionMetadata] = []
        self._sink = sink

    def on_gait_decision(self, metadata: LangChainDecisionMetadata) -> None:
        self.events.append(metadata)
        if self._sink is not None:
            self._sink(metadata)


class GaitLangChainMiddleware(_AgentMiddlewareBase):  # type: ignore[misc]
    """Official Gait tool middleware for LangChain agents."""

    def __init__(
        self,
        adapter: ToolAdapter,
        *,
        context_factory: ContextFactory | None = None,
        intent_factory: IntentFactory | None = None,
        cwd: str | Path | None = None,
        trace_dir: str | Path | None = None,
        trace_path_factory: TracePathFactory | None = None,
        callback_handler: SupportsGaitDecision | None = None,
        producer_version: str = "0.0.0-dev",
    ) -> None:
        _require_langchain()
        self._adapter = adapter
        self._context_factory = context_factory
        self._intent_factory = intent_factory
        self._cwd = cwd
        self._trace_dir = Path(trace_dir) if trace_dir is not None else None
        self._trace_path_factory = trace_path_factory
        self._callback_handler = callback_handler
        self._producer_version = producer_version

    def wrap_tool_call(self, request: _ToolCallRequest, handler: Callable[[Any], Any]) -> Any:
        intent = self._build_intent(request)
        trace_out = self._resolve_trace_path(request)

        try:
            outcome = self._adapter.execute(
                intent=intent,
                executor=lambda _: handler(request),
                cwd=self._cwd,
                trace_out=trace_out,
            )
        except GateEnforcementError as error:
            self._emit_metadata(
                request=request,
                intent=intent,
                verdict=error.decision.verdict,
                executed=False,
                trace_path=error.decision.trace_path,
                policy_digest=error.decision.policy_digest,
                intent_digest=error.decision.intent_digest or intent.intent_digest,
            )
            raise

        if not outcome.executed:
            self._emit_metadata(
                request=request,
                intent=intent,
                verdict=outcome.decision.verdict,
                executed=False,
                trace_path=outcome.decision.trace_path,
                policy_digest=outcome.decision.policy_digest,
                intent_digest=outcome.decision.intent_digest or intent.intent_digest,
            )
            raise GateEnforcementError(outcome.decision)

        self._emit_metadata(
            request=request,
            intent=intent,
            verdict=outcome.decision.verdict,
            executed=True,
            trace_path=outcome.decision.trace_path,
            policy_digest=outcome.decision.policy_digest,
            intent_digest=outcome.decision.intent_digest or intent.intent_digest,
        )
        return outcome.result

    def _build_intent(self, request: _ToolCallRequest) -> IntentRequest:
        if self._intent_factory is not None:
            return self._intent_factory(request)
        context = (
            self._context_factory(request)
            if self._context_factory is not None
            else _default_intent_context(request)
        )
        tool_call = _tool_call_payload(request)
        return capture_intent(
            tool_name=_required_string(tool_call, "name"),
            args=_tool_args(tool_call),
            context=context,
            producer_version=self._producer_version,
        )

    def _resolve_trace_path(self, request: _ToolCallRequest) -> str | Path | None:
        if self._trace_path_factory is not None:
            return self._trace_path_factory(request)
        if self._trace_dir is None:
            return None
        tool_call = _tool_call_payload(request)
        stem = _sanitize_filename(_required_string(tool_call, "name"))
        call_id = _sanitize_filename(str(tool_call.get("id", "call")))
        return self._trace_dir / f"{stem}_{call_id}.json"

    def _emit_metadata(
        self,
        *,
        request: _ToolCallRequest,
        intent: IntentRequest,
        verdict: str | None,
        executed: bool,
        trace_path: str | None,
        policy_digest: str | None,
        intent_digest: str | None,
    ) -> None:
        if self._callback_handler is None:
            return
        metadata = LangChainDecisionMetadata(
            tool_name=intent.tool_name,
            tool_call_id=_tool_call_id(request),
            run_id=_context_value(_runtime_context(request), "run_id"),
            request_id=intent.context.request_id,
            auth_context=intent.context.auth_context,
            trace_path=trace_path,
            policy_digest=policy_digest,
            intent_digest=intent_digest,
            verdict=verdict,
            executed=executed,
        )
        try:
            # Correlation sinks are additive only and must not affect tool execution.
            self._callback_handler.on_gait_decision(metadata)
        except Exception:
            return


def _require_langchain() -> None:
    if _LANGCHAIN_IMPORT_ERROR is None:
        return
    raise ImportError(
        "LangChain integration requires optional dependencies; install with "
        "`uv sync --extra langchain --extra dev` or add `langchain` to your environment."
    ) from _LANGCHAIN_IMPORT_ERROR


def _default_intent_context(request: _ToolCallRequest) -> IntentContext:
    raw_context = _runtime_context(request)
    session_id = _context_value(raw_context, "session_id")
    run_id = _context_value(raw_context, "run_id")
    request_id = _context_value(raw_context, "request_id") or _tool_call_id(request)
    auth_context = _context_dict(raw_context, "auth_context")
    credential_scopes = _context_list(raw_context, "credential_scopes")
    context_refs = _context_list(raw_context, "context_refs")
    return IntentContext(
        identity=_context_string(raw_context, "identity", default="langchain-agent")
        or "langchain-agent",
        workspace=_context_string(raw_context, "workspace", default=os.getcwd()) or os.getcwd(),
        risk_class=_context_string(raw_context, "risk_class", default="high") or "high",
        session_id=session_id or run_id,
        request_id=request_id,
        auth_context=auth_context,
        credential_scopes=credential_scopes,
        environment_fingerprint=_context_string(raw_context, "environment_fingerprint"),
        context_set_digest=_context_string(raw_context, "context_set_digest"),
        context_evidence_mode=_context_string(raw_context, "context_evidence_mode"),
        context_refs=context_refs,
    )


def _runtime_context(request: _ToolCallRequest) -> Any | None:
    runtime = getattr(request, "runtime", None)
    if runtime is None:
        return None
    return getattr(runtime, "context", None)


def _tool_call_payload(request: _ToolCallRequest) -> Mapping[str, Any]:
    payload = getattr(request, "tool_call", None)
    if not isinstance(payload, Mapping):
        raise TypeError("LangChain tool middleware expected request.tool_call to be a mapping")
    return payload


def _tool_args(payload: Mapping[str, Any]) -> dict[str, Any]:
    args = payload.get("args", {})
    if not isinstance(args, Mapping):
        raise TypeError("LangChain tool middleware expected tool_call args to be a mapping")
    return dict(args)


def _tool_call_id(request: _ToolCallRequest) -> str | None:
    payload = _tool_call_payload(request)
    value = payload.get("id")
    if value is None:
        return None
    return str(value)


def _required_string(payload: Mapping[str, Any], key: str) -> str:
    value = payload.get(key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"LangChain tool middleware expected non-empty string for {key}")
    return value


def _context_value(context: Any | None, key: str) -> Any | None:
    if context is None:
        return None
    if isinstance(context, Mapping):
        return context.get(key)
    return getattr(context, key, None)


def _context_string(
    context: Any | None,
    key: str,
    *,
    default: str | None = None,
) -> str | None:
    value = _context_value(context, key)
    if value is None:
        return default
    rendered = str(value).strip()
    if not rendered:
        return default
    return rendered


def _context_dict(context: Any | None, key: str) -> dict[str, Any] | None:
    value = _context_value(context, key)
    if value is None:
        return None
    if not isinstance(value, Mapping):
        raise TypeError(f"expected {key} to be a mapping")
    return {str(map_key): map_value for map_key, map_value in value.items()}


def _context_list(context: Any | None, key: str) -> list[str]:
    value = _context_value(context, key)
    if value is None:
        return []
    if not isinstance(value, list):
        raise TypeError(f"expected {key} to be a list")
    return [str(item) for item in value]


def _sanitize_filename(value: str) -> str:
    normalized = re.sub(r"[^A-Za-z0-9._-]+", "_", value).strip("._")
    if normalized:
        return normalized
    return "trace"
