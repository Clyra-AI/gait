from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import json
import tempfile
from pathlib import Path
from typing import Any, Callable, Sequence

from .client import (
    GaitError,
    _command_prefix,
    _json_ready,
    _run_command,
    capture_demo_runpack,
    create_regress_fixture,
    evaluate_gate,
)
from .models import DemoCapture, GateEvalResult, IntentRequest, RegressInitResult
from .session import get_active_run_session


class GateEnforcementError(RuntimeError):
    """Raised when a gate decision prevents execution."""

    def __init__(self, decision: GateEvalResult) -> None:
        self.decision = decision
        verdict = decision.verdict or "unknown"
        reasons = ",".join(decision.reason_codes) if decision.reason_codes else "none"
        super().__init__(f"execution blocked by gate verdict={verdict} reasons={reasons}")


class LifecycleEmissionError(GaitError):
    """Raised when an executed tool result cannot be recorded by Gait."""

    def __init__(self, message: str, *, result: Any | None) -> None:
        super().__init__(message)
        self.executed = True
        self.result = result


@dataclass(slots=True)
class AdapterOutcome:
    decision: GateEvalResult
    executed: bool
    result: Any | None = None


Executor = Callable[[IntentRequest], Any]
LifecycleCallback = Callable[
    [IntentRequest, GateEvalResult, Any | None, BaseException | None], None
]


@dataclass(slots=True)
class ToolAdapter:
    policy_path: str | Path
    gait_bin: str | Sequence[str] = "gait"
    key_mode: str = "dev"
    private_key: str | Path | None = None
    private_key_env: str | None = None
    approval_token: str | Path | None = None
    approval_public_key: str | Path | None = None
    approval_public_key_env: str | None = None
    approval_private_key: str | Path | None = None
    approval_private_key_env: str | None = None
    delegation_token: str | Path | None = None
    delegation_token_chain: Sequence[str | Path] | None = None
    delegation_public_key: str | Path | None = None
    delegation_public_key_env: str | None = None
    delegation_private_key: str | Path | None = None
    delegation_private_key_env: str | None = None
    lifecycle_callback: LifecycleCallback | None = None
    lifecycle_proposal: str | Path | None = None
    lifecycle_activation: str | Path | None = None
    lifecycle_trace_public_key: str | Path | None = None
    lifecycle_activation_public_key: str | Path | None = None
    lifecycle_journal: str | Path | None = None
    lifecycle_private_key: str | Path | None = None
    lifecycle_evaluation_time: str | None = None
    lifecycle_trusted_validators: str | None = None
    lifecycle_trusted_validator_keys: Sequence[str] | None = None

    def __post_init__(self) -> None:
        lifecycle_fields = (
            self.lifecycle_proposal,
            self.lifecycle_activation,
            self.lifecycle_trace_public_key,
            self.lifecycle_activation_public_key,
            self.lifecycle_journal,
            self.lifecycle_private_key,
            self.lifecycle_evaluation_time,
        )
        if any(value is not None for value in lifecycle_fields) and not all(
            value is not None and str(value).strip() for value in lifecycle_fields
        ):
            raise ValueError(
                "lifecycle configuration requires proposal, activation, trace/activation public keys, journal, private key, and evaluation time"
            )

    def gate_intent(
        self,
        *,
        intent: IntentRequest,
        cwd: str | Path | None = None,
        trace_out: str | Path | None = None,
    ) -> GateEvalResult:
        return evaluate_gate(
            policy_path=self.policy_path,
            intent=intent,
            gait_bin=self.gait_bin,
            cwd=cwd,
            trace_out=trace_out,
            approval_token=self.approval_token,
            key_mode=self.key_mode,
            private_key=self.private_key,
            private_key_env=self.private_key_env,
            approval_public_key=self.approval_public_key,
            approval_public_key_env=self.approval_public_key_env,
            approval_private_key=self.approval_private_key,
            approval_private_key_env=self.approval_private_key_env,
            delegation_token=self.delegation_token,
            delegation_token_chain=self.delegation_token_chain,
            delegation_public_key=self.delegation_public_key,
            delegation_public_key_env=self.delegation_public_key_env,
            delegation_private_key=self.delegation_private_key,
            delegation_private_key_env=self.delegation_private_key_env,
            action_contract=self.lifecycle_proposal,
            activation=self.lifecycle_activation,
            activation_public_key=self.lifecycle_activation_public_key,
            evaluation_time=self.lifecycle_evaluation_time,
            trusted_validators=self.lifecycle_trusted_validators,
            trusted_validator_keys=self.lifecycle_trusted_validator_keys,
            lifecycle_out=self.lifecycle_journal,
        )

    def execute(
        self,
        *,
        intent: IntentRequest,
        executor: Executor,
        cwd: str | Path | None = None,
        trace_out: str | Path | None = None,
    ) -> AdapterOutcome:
        active_session = get_active_run_session()
        lifecycle_callback = self.lifecycle_callback or (
            active_session.lifecycle_callback if active_session is not None else None
        )
        decision = self.gate_intent(intent=intent, cwd=cwd, trace_out=trace_out)
        resolved_trace_out = trace_out or decision.trace_path

        if not decision.ok:
            if active_session is not None:
                active_session.record_attempt(
                    intent=intent,
                    decision=decision,
                    executed=False,
                )
            raise GateEnforcementError(decision)
        if decision.verdict == "allow":
            if self.lifecycle_proposal is not None and not resolved_trace_out:
                raise GaitError(
                    "lifecycle configuration requires an explicit trace path from gate evaluation"
                )
            if self.lifecycle_proposal is not None:
                try:
                    self._verify_lifecycle_prefix(cwd=cwd)
                except Exception as lifecycle_error:
                    raise GaitError(
                        f"lifecycle activation prefix verification failed: {lifecycle_error}"
                    ) from lifecycle_error
            try:
                result = executor(intent)
            except Exception as error:
                if active_session is not None:
                    active_session.record_attempt(
                        intent=intent,
                        decision=decision,
                        executed=True,
                        error=error,
                    )
                try:
                    self._emit_lifecycle_result(intent, decision, resolved_trace_out, None, error, cwd)
                except Exception as lifecycle_error:
                    raise error from lifecycle_error
                if lifecycle_callback is not None:
                    lifecycle_callback(intent, decision, None, error)
                raise
            if active_session is not None:
                active_session.record_attempt(
                    intent=intent,
                    decision=decision,
                    executed=True,
                    result=result,
                )
            try:
                self._emit_lifecycle_result(intent, decision, resolved_trace_out, result, None, cwd)
            except Exception as lifecycle_error:
                raise LifecycleEmissionError(
                    f"lifecycle evidence emission failed: {lifecycle_error}", result=result
                ) from lifecycle_error
            if lifecycle_callback is not None:
                try:
                    lifecycle_callback(intent, decision, result, None)
                except Exception as callback_error:
                    raise LifecycleEmissionError(
                        f"lifecycle callback failed: {callback_error}", result=result
                    ) from callback_error
            return AdapterOutcome(decision=decision, executed=True, result=result)
        if decision.verdict == "dry_run":
            if active_session is not None:
                active_session.record_attempt(
                    intent=intent,
                    decision=decision,
                    executed=False,
                )
            return AdapterOutcome(decision=decision, executed=False, result=None)
        if active_session is not None:
            active_session.record_attempt(
                intent=intent,
                decision=decision,
                executed=False,
            )
        raise GateEnforcementError(decision)

    def _verify_lifecycle_prefix(self, *, cwd: str | Path | None) -> None:
        """Require authenticated activation evidence before invoking executor."""
        command = _command_prefix(self.gait_bin) + [
            "action-contract",
            "lifecycle-verify",
            "--journal",
            str(self.lifecycle_journal),
            "--public-key",
            str(self.lifecycle_trace_public_key),
            "--json",
        ]
        _run_command(command, cwd=cwd)

    def _emit_lifecycle_result(
        self,
        intent: IntentRequest,
        decision: GateEvalResult,
        trace_out: str | Path | None,
        result: Any | None,
        error: BaseException | None,
        cwd: str | Path | None,
    ) -> None:
        if not all(
            (
                self.lifecycle_proposal,
                self.lifecycle_activation,
                self.lifecycle_trace_public_key,
                self.lifecycle_activation_public_key,
                self.lifecycle_journal,
                self.lifecycle_private_key,
                self.lifecycle_evaluation_time,
                trace_out,
            )
        ):
            return
        payload = {"result": _json_ready(result)} if error is None else {"error": str(error)}
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", suffix=".json", prefix="gait-result-", delete=False
        ) as result_file:
            json.dump(payload, result_file)
            result_path = Path(result_file.name)
        try:
            command = _command_prefix(self.gait_bin) + [
                "action-contract",
                "lifecycle-result",
                "--trace",
                str(trace_out),
                "--proposal",
                str(self.lifecycle_proposal),
                "--activation",
                str(self.lifecycle_activation),
                "--trace-public-key",
                str(self.lifecycle_trace_public_key),
                "--public-key",
                str(self.lifecycle_activation_public_key),
                "--journal",
                str(self.lifecycle_journal),
                "--private-key",
                str(self.lifecycle_private_key),
                "--evaluation-time",
                str(self.lifecycle_evaluation_time),
                "--execution-time",
                datetime.now(UTC).isoformat().replace("+00:00", "Z"),
                "--result-file",
                str(result_path),
                "--outcome",
                "failed" if error is not None else "succeeded",
            ]
            if self.lifecycle_trusted_validators:
                command.extend(["--trusted-validators", self.lifecycle_trusted_validators])
            for validator_key in self.lifecycle_trusted_validator_keys or ():
                command.extend(["--trusted-validator-key", validator_key])
            _run_command(command, cwd=cwd)
        finally:
            result_path.unlink(missing_ok=True)

    def capture_runpack(self, *, cwd: str | Path | None = None) -> DemoCapture:
        return capture_demo_runpack(gait_bin=self.gait_bin, cwd=cwd)

    def create_regression_fixture(
        self, *, from_run: str, cwd: str | Path | None = None
    ) -> RegressInitResult:
        return create_regress_fixture(gait_bin=self.gait_bin, from_run=from_run, cwd=cwd)
