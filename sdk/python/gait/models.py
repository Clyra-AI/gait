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
    auth_context: dict[str, Any] | None = None
    credential_scopes: list[str] = field(default_factory=list)
    environment_fingerprint: str | None = None
    context_set_digest: str | None = None
    context_evidence_mode: str | None = None
    context_refs: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        output: dict[str, Any] = {
            "identity": self.identity,
            "workspace": self.workspace,
            "risk_class": self.risk_class,
        }
        if self.session_id:
            output["session_id"] = self.session_id
        if self.request_id:
            output["request_id"] = self.request_id
        if self.auth_context:
            output["auth_context"] = dict(self.auth_context)
        if self.credential_scopes:
            output["credential_scopes"] = list(self.credential_scopes)
        if self.environment_fingerprint:
            output["environment_fingerprint"] = self.environment_fingerprint
        if self.context_set_digest:
            output["context_set_digest"] = self.context_set_digest
        if self.context_evidence_mode:
            output["context_evidence_mode"] = self.context_evidence_mode
        if self.context_refs:
            output["context_refs"] = list(self.context_refs)
        return output


@dataclass(slots=True, frozen=True)
class DelegationLink:
    delegator_identity: str
    delegate_identity: str
    scope_class: str | None = None
    token_ref: str | None = None

    def to_dict(self) -> dict[str, str]:
        output: dict[str, str] = {
            "delegator_identity": self.delegator_identity,
            "delegate_identity": self.delegate_identity,
        }
        if self.scope_class:
            output["scope_class"] = self.scope_class
        if self.token_ref:
            output["token_ref"] = self.token_ref
        return output


@dataclass(slots=True, frozen=True)
class IntentDelegation:
    requester_identity: str
    scope_class: str | None = None
    token_refs: list[str] = field(default_factory=list)
    chain: list[DelegationLink] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        output: dict[str, Any] = {
            "requester_identity": self.requester_identity,
        }
        if self.scope_class:
            output["scope_class"] = self.scope_class
        if self.token_refs:
            output["token_refs"] = list(self.token_refs)
        if self.chain:
            output["chain"] = [link.to_dict() for link in self.chain]
        return output


@dataclass(slots=True, frozen=True)
class IntentScriptStep:
    tool_name: str
    args: dict[str, Any]
    targets: list[IntentTarget] = field(default_factory=list)
    arg_provenance: list[IntentArgProvenance] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        output: dict[str, Any] = {"tool_name": self.tool_name, "args": self.args}
        if self.targets:
            output["targets"] = [target.to_dict() for target in self.targets]
        if self.arg_provenance:
            output["arg_provenance"] = [entry.to_dict() for entry in self.arg_provenance]
        return output


@dataclass(slots=True, frozen=True)
class IntentScript:
    steps: list[IntentScriptStep]

    def to_dict(self) -> dict[str, Any]:
        return {"steps": [step.to_dict() for step in self.steps]}


@dataclass(slots=True)
class IntentRequest:
    tool_name: str
    args: dict[str, Any]
    context: IntentContext
    targets: list[IntentTarget] = field(default_factory=list)
    arg_provenance: list[IntentArgProvenance] = field(default_factory=list)
    delegation: IntentDelegation | None = None
    script: IntentScript | None = None
    created_at: datetime = field(default_factory=_utc_now)
    producer_version: str = "0.0.0-dev"
    schema_id: str = "gait.gate.intent_request"
    schema_version: str = "1.0.0"
    args_digest: str | None = None
    intent_digest: str | None = None
    script_hash: str | None = None

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
        if self.delegation is not None:
            output["delegation"] = self.delegation.to_dict()
        if self.script is not None:
            output["script"] = self.script.to_dict()
        if self.args_digest:
            output["args_digest"] = self.args_digest
        if self.intent_digest:
            output["intent_digest"] = self.intent_digest
        if self.script_hash:
            output["script_hash"] = self.script_hash
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
            script_hash=payload.get("script_hash"),
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
                auth_context=payload["context"].get("auth_context"),
                credential_scopes=[
                    str(value) for value in payload["context"].get("credential_scopes", [])
                ],
                environment_fingerprint=payload["context"].get("environment_fingerprint"),
                context_set_digest=payload["context"].get("context_set_digest"),
                context_evidence_mode=payload["context"].get("context_evidence_mode"),
                context_refs=[str(value) for value in payload["context"].get("context_refs", [])],
            ),
            delegation=(
                IntentDelegation(
                    requester_identity=str(payload["delegation"]["requester_identity"]),
                    scope_class=payload["delegation"].get("scope_class"),
                    token_refs=[
                        str(value) for value in payload["delegation"].get("token_refs", [])
                    ],
                    chain=[
                        DelegationLink(
                            delegator_identity=str(link["delegator_identity"]),
                            delegate_identity=str(link["delegate_identity"]),
                            scope_class=link.get("scope_class"),
                            token_ref=link.get("token_ref"),
                        )
                        for link in payload["delegation"].get("chain", [])
                    ],
                )
                if "delegation" in payload and isinstance(payload.get("delegation"), dict)
                else None
            ),
            script=(
                IntentScript(
                    steps=[
                        IntentScriptStep(
                            tool_name=str(step["tool_name"]),
                            args=dict(step.get("args", {})),
                            targets=[
                                IntentTarget(
                                    kind=str(target["kind"]),
                                    value=str(target["value"]),
                                    operation=target.get("operation"),
                                    sensitivity=target.get("sensitivity"),
                                )
                                for target in step.get("targets", [])
                            ],
                            arg_provenance=[
                                IntentArgProvenance(
                                    arg_path=str(entry["arg_path"]),
                                    source=str(entry["source"]),
                                    source_ref=entry.get("source_ref"),
                                    integrity_digest=entry.get("integrity_digest"),
                                )
                                for entry in step.get("arg_provenance", [])
                            ],
                        )
                        for step in payload["script"].get("steps", [])
                    ]
                )
                if "script" in payload and isinstance(payload.get("script"), dict)
                else None
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
    script: bool = False
    step_count: int = 0
    script_hash: str | None = None
    composite_risk_class: str | None = None
    pre_approved: bool = False
    pattern_id: str | None = None
    registry_reason: str | None = None
    step_verdicts: list[dict[str, Any]] = field(default_factory=list)
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
            script=bool(payload.get("script", False)),
            step_count=int(payload.get("step_count", 0)),
            script_hash=payload.get("script_hash"),
            composite_risk_class=payload.get("composite_risk_class"),
            pre_approved=bool(payload.get("pre_approved", False)),
            pattern_id=payload.get("pattern_id"),
            registry_reason=payload.get("registry_reason"),
            step_verdicts=[
                dict(value) for value in payload.get("step_verdicts", []) if isinstance(value, dict)
            ],
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


@dataclass(slots=True, frozen=True)
class RunRecordCapture:
    run_id: str
    bundle_path: str
    manifest_digest: str
    ticket_footer: str
