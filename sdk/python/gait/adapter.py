from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Sequence

from .client import capture_demo_runpack, create_regress_fixture, evaluate_gate
from .models import DemoCapture, GateEvalResult, IntentRequest, RegressInitResult
from .session import get_active_run_session


class GateEnforcementError(RuntimeError):
    """Raised when a gate decision prevents execution."""

    def __init__(self, decision: GateEvalResult) -> None:
        self.decision = decision
        verdict = decision.verdict or "unknown"
        reasons = ",".join(decision.reason_codes) if decision.reason_codes else "none"
        super().__init__(f"execution blocked by gate verdict={verdict} reasons={reasons}")


@dataclass(slots=True)
class AdapterOutcome:
    decision: GateEvalResult
    executed: bool
    result: Any | None = None


Executor = Callable[[IntentRequest], Any]


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
        decision = self.gate_intent(intent=intent, cwd=cwd, trace_out=trace_out)

        if not decision.ok:
            if active_session is not None:
                active_session.record_attempt(
                    intent=intent,
                    decision=decision,
                    executed=False,
                )
            raise GateEnforcementError(decision)
        if decision.verdict == "allow":
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
                raise
            if active_session is not None:
                active_session.record_attempt(
                    intent=intent,
                    decision=decision,
                    executed=True,
                    result=result,
                )
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

    def capture_runpack(self, *, cwd: str | Path | None = None) -> DemoCapture:
        return capture_demo_runpack(gait_bin=self.gait_bin, cwd=cwd)

    def create_regression_fixture(
        self, *, from_run: str, cwd: str | Path | None = None
    ) -> RegressInitResult:
        return create_regress_fixture(gait_bin=self.gait_bin, from_run=from_run, cwd=cwd)
