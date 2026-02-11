from __future__ import annotations

import inspect
from functools import wraps
from pathlib import Path
from typing import Any, Callable, Mapping, ParamSpec, Sequence, TypeVar, cast

from .adapter import GateEnforcementError, ToolAdapter
from .client import capture_intent
from .models import IntentArgProvenance, IntentContext, IntentTarget

P = ParamSpec("P")
R = TypeVar("R")

ArgsMapper = Callable[[tuple[Any, ...], Mapping[str, Any]], Mapping[str, Any]]
ContextResolver = Callable[[tuple[Any, ...], Mapping[str, Any]], IntentContext]
TargetsResolver = Callable[[tuple[Any, ...], Mapping[str, Any]], Sequence[IntentTarget]]
ArgProvenanceResolver = Callable[[tuple[Any, ...], Mapping[str, Any]], Sequence[IntentArgProvenance]]
PathResolver = Callable[[tuple[Any, ...], Mapping[str, Any]], str | Path | None]


def gate_tool(
    *,
    adapter: ToolAdapter,
    context: IntentContext | ContextResolver,
    tool_name: str | None = None,
    args_mapper: ArgsMapper | None = None,
    targets: Sequence[IntentTarget] | TargetsResolver | None = None,
    arg_provenance: Sequence[IntentArgProvenance] | ArgProvenanceResolver | None = None,
    trace_out: str | Path | PathResolver | None = None,
    cwd: str | Path | PathResolver | None = None,
) -> Callable[[Callable[P, R]], Callable[P, R]]:
    """Decorate a tool function so every invocation is routed through Gait gate eval.

    The decorated function fails closed: any non-allow verdict raises `GateEnforcementError`
    and the wrapped function body is not executed.
    """

    def decorator(function: Callable[P, R]) -> Callable[P, R]:
        @wraps(function)
        def wrapped(*args: P.args, **kwargs: P.kwargs) -> R:
            payload = (
                dict(args_mapper(args, kwargs))
                if args_mapper is not None
                else _default_args_payload(function, args, kwargs)
            )
            resolved_context = (
                context(args, kwargs) if callable(context) else context
            )
            resolved_targets = _resolve_sequence(targets, args, kwargs)
            resolved_arg_provenance = _resolve_sequence(arg_provenance, args, kwargs)
            resolved_trace_out = _resolve_path(trace_out, args, kwargs)
            resolved_cwd = _resolve_path(cwd, args, kwargs)

            intent = capture_intent(
                tool_name=tool_name or function.__name__,
                args=payload,
                context=resolved_context,
                targets=resolved_targets,
                arg_provenance=resolved_arg_provenance,
            )
            outcome = adapter.execute(
                intent=intent,
                executor=lambda _: function(*args, **kwargs),
                cwd=resolved_cwd,
                trace_out=resolved_trace_out,
            )
            if not outcome.executed:
                raise GateEnforcementError(outcome.decision)
            return cast(R, outcome.result)

        return wrapped

    return decorator


def _default_args_payload(
    function: Callable[..., Any], args: tuple[Any, ...], kwargs: Mapping[str, Any]
) -> dict[str, Any]:
    signature = inspect.signature(function)
    bound = signature.bind_partial(*args, **kwargs)
    bound.apply_defaults()

    payload: dict[str, Any] = {}
    for name, value in bound.arguments.items():
        if name in {"self", "cls"}:
            continue
        payload[name] = value
    return payload


def _resolve_sequence(
    value: Sequence[Any] | Callable[[tuple[Any, ...], Mapping[str, Any]], Sequence[Any]] | None,
    args: tuple[Any, ...],
    kwargs: Mapping[str, Any],
) -> list[Any]:
    if value is None:
        return []
    if callable(value):
        return list(value(args, kwargs))
    return list(value)


def _resolve_path(
    value: str | Path | Callable[[tuple[Any, ...], Mapping[str, Any]], str | Path | None] | None,
    args: tuple[Any, ...],
    kwargs: Mapping[str, Any],
) -> str | Path | None:
    if value is None:
        return None
    if callable(value):
        return value(args, kwargs)
    return value
