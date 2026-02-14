from __future__ import annotations

import hashlib
import json
import platform
import sys
from contextvars import ContextVar, Token
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Literal, Mapping

from .client import record_runpack
from .models import GateEvalResult, IntentRequest, RunRecordCapture

_ACTIVE_RUN_SESSION: ContextVar["RunSession | None"] = ContextVar(
    "gait_active_run_session",
    default=None,
)


def get_active_run_session() -> "RunSession | None":
    return _ACTIVE_RUN_SESSION.get()


def run_session(
    *,
    run_id: str,
    gait_bin: str | list[str] = "gait",
    out_dir: str | Path = "gait-out",
    cwd: str | Path | None = None,
    capture_mode: str = "reference",
    include_raw_payload: bool = False,
    producer_version: str = "0.0.0-dev",
    context_evidence_mode: str | None = None,
    context_envelope: str | Path | None = None,
) -> "RunSession":
    return RunSession(
        run_id=run_id,
        gait_bin=gait_bin,
        out_dir=out_dir,
        cwd=cwd,
        capture_mode=capture_mode,
        include_raw_payload=include_raw_payload,
        producer_version=producer_version,
        context_evidence_mode=context_evidence_mode,
        context_envelope=context_envelope,
    )


@dataclass(slots=True, frozen=True)
class RunAttempt:
    intent_id: str
    tool_name: str
    verdict: str
    executed: bool
    status: str


class RunSession:
    def __init__(
        self,
        *,
        run_id: str,
        gait_bin: str | list[str] = "gait",
        out_dir: str | Path = "gait-out",
        cwd: str | Path | None = None,
        capture_mode: str = "reference",
        include_raw_payload: bool = False,
        producer_version: str = "0.0.0-dev",
        context_evidence_mode: str | None = None,
        context_envelope: str | Path | None = None,
    ) -> None:
        if not run_id.strip():
            raise ValueError("run_id is required")
        if capture_mode not in {"reference", "raw"}:
            raise ValueError("capture_mode must be 'reference' or 'raw'")

        self.run_id = run_id.strip()
        self.gait_bin = gait_bin
        self.out_dir = out_dir
        self.cwd = cwd
        self.capture_mode = capture_mode
        self.include_raw_payload = include_raw_payload
        self.producer_version = producer_version
        self.context_evidence_mode = context_evidence_mode
        self.context_envelope = context_envelope

        self._token: Token[RunSession | None] | None = None
        self._closed = False
        self._capture: RunRecordCapture | None = None
        self._record_input: dict[str, Any] | None = None
        self._attempt_count = 0

        self._started_at = _utc_now()
        self._timeline: list[dict[str, Any]] = [
            {"event": "run_started", "ts": _isoformat(self._started_at)}
        ]
        self._intents: list[dict[str, Any]] = []
        self._results: list[dict[str, Any]] = []
        self._refs: list[dict[str, Any]] = []
        self._attempts: list[RunAttempt] = []
        self._context_set_digest: str | None = None
        self._context_evidence_mode: str | None = context_evidence_mode
        self._context_refs: list[str] = []

    def __enter__(self) -> "RunSession":
        self._token = _ACTIVE_RUN_SESSION.set(self)
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> Literal[False]:
        try:
            self.finalize()
        finally:
            if self._token is not None:
                _ACTIVE_RUN_SESSION.reset(self._token)
                self._token = None
        return False

    @property
    def capture(self) -> RunRecordCapture | None:
        return self._capture

    @property
    def record_input(self) -> dict[str, Any] | None:
        return self._record_input

    @property
    def attempts(self) -> list[RunAttempt]:
        return list(self._attempts)

    def record_attempt(
        self,
        *,
        intent: IntentRequest,
        decision: GateEvalResult,
        executed: bool,
        result: Any | None = None,
        error: BaseException | None = None,
    ) -> None:
        if self._closed:
            raise RuntimeError("run session already finalized")

        self._attempt_count += 1
        intent_id = f"intent_{self._attempt_count:04d}"
        created_at = _utc_now()

        args_digest = intent.args_digest or _sha256_json(intent.args)
        intent_payload = intent.to_dict()
        intent_digest = intent.intent_digest or _sha256_json(intent_payload)
        if intent.context.context_set_digest and not self._context_set_digest:
            self._context_set_digest = str(intent.context.context_set_digest)
        if intent.context.context_evidence_mode and not self._context_evidence_mode:
            self._context_evidence_mode = str(intent.context.context_evidence_mode)
        for context_ref in intent.context.context_refs:
            value = str(context_ref).strip()
            if value and value not in self._context_refs:
                self._context_refs.append(value)

        intent_record: dict[str, Any] = {
            "schema_id": "gait.runpack.intent",
            "schema_version": "1.0.0",
            "created_at": _isoformat(created_at),
            "producer_version": self.producer_version,
            "run_id": self.run_id,
            "intent_id": intent_id,
            "tool_name": intent.tool_name,
            "args_digest": args_digest,
        }
        if self.capture_mode == "raw" and self.include_raw_payload:
            intent_record["args"] = _json_compatible(intent.args)
        self._intents.append(intent_record)

        verdict = decision.verdict or "unknown"
        status = _result_status(verdict=verdict, executed=executed, error=error)
        result_payload: dict[str, Any] = {
            "executed": executed,
            "verdict": verdict,
            "reason_codes": decision.reason_codes,
            "trace_id": decision.trace_id,
            "trace_path": decision.trace_path,
            "policy_digest": decision.policy_digest,
            "intent_digest": decision.intent_digest or intent_digest,
        }
        if error is not None:
            result_payload["error"] = str(error)
        if executed and result is not None:
            result_payload["result"] = _json_compatible(result)
        result_digest = _sha256_json(result_payload)

        result_record: dict[str, Any] = {
            "schema_id": "gait.runpack.result",
            "schema_version": "1.0.0",
            "created_at": _isoformat(created_at),
            "producer_version": self.producer_version,
            "run_id": self.run_id,
            "intent_id": intent_id,
            "status": status,
            "result_digest": result_digest,
        }
        if self.capture_mode == "raw" and self.include_raw_payload:
            result_record["result"] = _json_compatible(result_payload)
        self._results.append(result_record)

        trace_ref = decision.trace_id or intent_id
        source_locator = decision.trace_path or f"trace://{trace_ref}"
        ref_record: dict[str, Any] = {
            "ref_id": f"trace_{intent_id}",
            "source_type": "gait.trace",
            "source_locator": source_locator,
            "query_digest": _sha256_json(
                {"tool_name": intent.tool_name, "args_digest": args_digest}
            ),
            "content_digest": result_digest,
            "retrieved_at": _isoformat(created_at),
            "redaction_mode": self.capture_mode,
            "retrieval_params": {
                "verdict": verdict,
                "reason_codes": list(decision.reason_codes),
            },
        }
        self._refs.append(ref_record)

        self._timeline.append(
            {"event": "intent_captured", "ts": _isoformat(created_at), "ref": intent_id}
        )
        self._timeline.append(
            {"event": "result_captured", "ts": _isoformat(created_at), "ref": intent_id}
        )
        self._attempts.append(
            RunAttempt(
                intent_id=intent_id,
                tool_name=intent.tool_name,
                verdict=verdict,
                executed=executed,
                status=status,
            )
        )

    def finalize(self) -> RunRecordCapture:
        if self._capture is not None:
            return self._capture

        finished_at = _utc_now()
        self._timeline.append({"event": "run_finished", "ts": _isoformat(finished_at)})

        record_input: dict[str, Any] = {
            "run": {
                "schema_id": "gait.runpack.run",
                "schema_version": "1.0.0",
                "created_at": _isoformat(self._started_at),
                "producer_version": self.producer_version,
                "run_id": self.run_id,
                "env": {
                    "os": _runtime_os(),
                    "arch": platform.machine().lower(),
                    "runtime": f"python{sys.version_info.major}.{sys.version_info.minor}",
                },
                "timeline": list(self._timeline),
            },
            "intents": list(self._intents),
            "results": list(self._results),
            "refs": {
                "schema_id": "gait.runpack.refs",
                "schema_version": "1.0.0",
                "created_at": _isoformat(finished_at),
                "producer_version": self.producer_version,
                "run_id": self.run_id,
                "receipts": list(self._refs),
            },
            "capture_mode": self.capture_mode,
        }
        if self._context_set_digest:
            record_input["refs"]["context_set_digest"] = self._context_set_digest
        if self._context_evidence_mode:
            record_input["refs"]["context_evidence_mode"] = self._context_evidence_mode
        if self._context_refs:
            record_input["refs"]["context_ref_count"] = len(self._context_refs)
        self._record_input = record_input
        self._capture = record_runpack(
            record_input=record_input,
            gait_bin=self.gait_bin,
            cwd=self.cwd,
            out_dir=self.out_dir,
            capture_mode=self.capture_mode,
            context_evidence_mode=self._context_evidence_mode,
            context_envelope=self.context_envelope,
        )
        self._closed = True
        return self._capture


def _result_status(*, verdict: str, executed: bool, error: BaseException | None) -> str:
    if error is not None:
        return "error"
    if executed:
        return "ok"
    if verdict == "dry_run":
        return "dry_run"
    if verdict in {"block", "require_approval"}:
        return verdict
    if verdict:
        return verdict
    return "blocked"


def _runtime_os() -> str:
    os_name = platform.system().lower().strip()
    if os_name:
        return os_name
    return sys.platform.lower().strip() or "unknown"


def _utc_now() -> datetime:
    return datetime.now(UTC)


def _isoformat(value: datetime) -> str:
    return value.astimezone(UTC).isoformat().replace("+00:00", "Z")


def _sha256_json(value: Any) -> str:
    canonical = _canonical_json_bytes(value)
    return hashlib.sha256(canonical).hexdigest()


def _canonical_json_bytes(value: Any) -> bytes:
    text = json.dumps(_json_compatible(value), sort_keys=True, separators=(",", ":"))
    return text.encode("utf-8")


def _json_compatible(value: Any) -> Any:
    if isinstance(value, Mapping):
        return {str(key): _json_compatible(item) for key, item in value.items()}
    if isinstance(value, (list, tuple, set)):
        return [_json_compatible(item) for item in value]
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, datetime):
        return _isoformat(value)
    return str(value)
