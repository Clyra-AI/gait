from __future__ import annotations

from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


def _utc_now() -> datetime:
    return datetime.now(UTC)


def _isoformat(value: datetime) -> str:
    return value.astimezone(UTC).isoformat().replace("+00:00", "Z")


def _parse_datetime(value: str) -> datetime:
    normalized = value
    if value.endswith("Z"):
        normalized = value[:-1] + "+00:00"
    return datetime.fromisoformat(normalized).astimezone(UTC)


@dataclass(slots=True, frozen=True)
class IntentTarget:
    kind: str
    value: str
    operation: str | None = None
    sensitivity: str | None = None

    def to_dict(self) -> dict[str, str]:
        output: dict[str, str] = {"kind": self.kind, "value": self.value}
        if self.operation:
            output["operation"] = self.operation
        if self.sensitivity:
            output["sensitivity"] = self.sensitivity
        return output


@dataclass(slots=True, frozen=True)
class IntentArgProvenance:
    arg_path: str
    source: str
    source_ref: str | None = None
    integrity_digest: str | None = None

    def to_dict(self) -> dict[str, str]:
        output: dict[str, str] = {"arg_path": self.arg_path, "source": self.source}
        if self.source_ref:
            output["source_ref"] = self.source_ref
        if self.integrity_digest:
            output["integrity_digest"] = self.integrity_digest
        return output


@dataclass(slots=True, frozen=True)
class IntentContext:
    identity: str
    workspace: str
    risk_class: str
    session_id: str | None = None
    request_id: str | None = None

    def to_dict(self) -> dict[str, str]:
        output: dict[str, str] = {
            "identity": self.identity,
            "workspace": self.workspace,
            "risk_class": self.risk_class,
        }
        if self.session_id:
            output["session_id"] = self.session_id
        if self.request_id:
            output["request_id"] = self.request_id
        return output


@dataclass(slots=True)
class IntentRequest:
    tool_name: str
    args: dict[str, Any]
    context: IntentContext
    targets: list[IntentTarget] = field(default_factory=list)
    arg_provenance: list[IntentArgProvenance] = field(default_factory=list)
    created_at: datetime = field(default_factory=_utc_now)
    producer_version: str = "0.0.0-dev"
    schema_id: str = "gait.gate.intent_request"
    schema_version: str = "1.0.0"
    args_digest: str | None = None
    intent_digest: str | None = None

    def to_dict(self) -> dict[str, Any]:
        output: dict[str, Any] = {
            "schema_id": self.schema_id,
            "schema_version": self.schema_version,
            "created_at": _isoformat(self.created_at),
            "producer_version": self.producer_version,
            "tool_name": self.tool_name,
            "args": self.args,
            "targets": [target.to_dict() for target in self.targets],
            "context": self.context.to_dict(),
        }
        if self.arg_provenance:
            output["arg_provenance"] = [entry.to_dict() for entry in self.arg_provenance]
        if self.args_digest:
            output["args_digest"] = self.args_digest
        if self.intent_digest:
            output["intent_digest"] = self.intent_digest
        return output

    @classmethod
    def from_dict(cls, payload: dict[str, Any]) -> "IntentRequest":
        return cls(
            schema_id=str(payload.get("schema_id", "gait.gate.intent_request")),
            schema_version=str(payload.get("schema_version", "1.0.0")),
            created_at=_parse_datetime(str(payload["created_at"])),
            producer_version=str(payload.get("producer_version", "0.0.0-dev")),
            tool_name=str(payload["tool_name"]),
            args=dict(payload.get("args", {})),
            args_digest=payload.get("args_digest"),
            intent_digest=payload.get("intent_digest"),
            targets=[
                IntentTarget(
                    kind=str(target["kind"]),
                    value=str(target["value"]),
                    operation=target.get("operation"),
                    sensitivity=target.get("sensitivity"),
                )
                for target in payload.get("targets", [])
            ],
            arg_provenance=[
                IntentArgProvenance(
                    arg_path=str(entry["arg_path"]),
                    source=str(entry["source"]),
                    source_ref=entry.get("source_ref"),
                    integrity_digest=entry.get("integrity_digest"),
                )
                for entry in payload.get("arg_provenance", [])
            ],
            context=IntentContext(
                identity=str(payload["context"]["identity"]),
                workspace=str(payload["context"]["workspace"]),
                risk_class=str(payload["context"]["risk_class"]),
                session_id=payload["context"].get("session_id"),
                request_id=payload["context"].get("request_id"),
            ),
        )


@dataclass(slots=True, frozen=True)
class GateEvalResult:
    ok: bool
    exit_code: int
    verdict: str | None = None
    reason_codes: list[str] = field(default_factory=list)
    violations: list[str] = field(default_factory=list)
    approval_ref: str | None = None
    trace_id: str | None = None
    trace_path: str | None = None
    policy_digest: str | None = None
    intent_digest: str | None = None
    warnings: list[str] = field(default_factory=list)
    error: str | None = None

    @classmethod
    def from_dict(cls, payload: dict[str, Any], exit_code: int) -> "GateEvalResult":
        return cls(
            ok=bool(payload.get("ok", False)),
            exit_code=exit_code,
            verdict=payload.get("verdict"),
            reason_codes=[str(value) for value in payload.get("reason_codes", [])],
            violations=[str(value) for value in payload.get("violations", [])],
            approval_ref=payload.get("approval_ref"),
            trace_id=payload.get("trace_id"),
            trace_path=payload.get("trace_path"),
            policy_digest=payload.get("policy_digest"),
            intent_digest=payload.get("intent_digest"),
            warnings=[str(value) for value in payload.get("warnings", [])],
            error=payload.get("error"),
        )


@dataclass(slots=True, frozen=True)
class TraceRecord:
    schema_id: str
    schema_version: str
    created_at: datetime
    producer_version: str
    trace_id: str
    tool_name: str
    args_digest: str
    intent_digest: str
    policy_digest: str
    verdict: str
    raw: dict[str, Any]

    @classmethod
    def from_dict(cls, payload: dict[str, Any]) -> "TraceRecord":
        return cls(
            schema_id=str(payload["schema_id"]),
            schema_version=str(payload["schema_version"]),
            created_at=_parse_datetime(str(payload["created_at"])),
            producer_version=str(payload["producer_version"]),
            trace_id=str(payload["trace_id"]),
            tool_name=str(payload["tool_name"]),
            args_digest=str(payload["args_digest"]),
            intent_digest=str(payload["intent_digest"]),
            policy_digest=str(payload["policy_digest"]),
            verdict=str(payload["verdict"]),
            raw=dict(payload),
        )


@dataclass(slots=True, frozen=True)
class DemoCapture:
    run_id: str
    bundle_path: str
    ticket_footer: str
    verified: bool
    raw_output: str


@dataclass(slots=True, frozen=True)
class RegressInitResult:
    run_id: str
    fixture_name: str
    fixture_dir: str
    runpack_path: str
    config_path: str
    next_commands: list[str]

    @classmethod
    def from_dict(cls, payload: dict[str, Any]) -> "RegressInitResult":
        return cls(
            run_id=str(payload.get("run_id", "")),
            fixture_name=str(payload.get("fixture_name", "")),
            fixture_dir=str(payload.get("fixture_dir", "")),
            runpack_path=str(payload.get("runpack_path", "")),
            config_path=str(payload.get("config_path", "")),
            next_commands=[str(value) for value in payload.get("next_commands", [])],
        )

    @property
    def fixture_path(self) -> Path:
        return Path(self.fixture_dir)
