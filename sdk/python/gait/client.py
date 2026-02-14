from __future__ import annotations

import json
import shutil
import subprocess  # nosec B404
import tempfile
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Mapping, Sequence

from .models import (
    DemoCapture,
    GateEvalResult,
    IntentArgProvenance,
    IntentContext,
    IntentRequest,
    IntentTarget,
    RegressInitResult,
    RunRecordCapture,
    TraceRecord,
)

DEFAULT_COMMAND_TIMEOUT_SECONDS = 30.0


class GaitError(RuntimeError):
    """Base SDK error."""


class GaitCommandError(GaitError):
    """Raised when a `gait` command returns an unexpected non-zero exit code."""

    def __init__(
        self, message: str, *, command: Sequence[str], exit_code: int, stdout: str, stderr: str
    ) -> None:
        super().__init__(message)
        self.command = list(command)
        self.exit_code = exit_code
        self.stdout = stdout
        self.stderr = stderr


@dataclass(slots=True, frozen=True)
class _CommandResult:
    command: list[str]
    exit_code: int
    stdout: str
    stderr: str


def capture_intent(
    *,
    tool_name: str,
    args: Mapping[str, Any],
    context: IntentContext,
    targets: Sequence[IntentTarget] | None = None,
    arg_provenance: Sequence[IntentArgProvenance] | None = None,
    created_at: datetime | None = None,
    producer_version: str = "0.0.0-dev",
) -> IntentRequest:
    return IntentRequest(
        tool_name=tool_name,
        args=dict(args),
        context=context,
        targets=list(targets or []),
        arg_provenance=list(arg_provenance or []),
        created_at=created_at or datetime.now(UTC),
        producer_version=producer_version,
    )


def evaluate_gate(
    *,
    policy_path: str | Path,
    intent: IntentRequest,
    gait_bin: str | Sequence[str] = "gait",
    cwd: str | Path | None = None,
    trace_out: str | Path | None = None,
    approval_token: str | Path | None = None,
    key_mode: str = "dev",
    private_key: str | Path | None = None,
    private_key_env: str | None = None,
    approval_public_key: str | Path | None = None,
    approval_public_key_env: str | None = None,
    approval_private_key: str | Path | None = None,
    approval_private_key_env: str | None = None,
    delegation_token: str | Path | None = None,
    delegation_token_chain: Sequence[str | Path] | None = None,
    delegation_public_key: str | Path | None = None,
    delegation_public_key_env: str | None = None,
    delegation_private_key: str | Path | None = None,
    delegation_private_key_env: str | None = None,
) -> GateEvalResult:
    with tempfile.TemporaryDirectory(prefix="gait-intent-") as tmp_dir:
        intent_path = Path(tmp_dir) / "intent.json"
        intent_path.write_text(json.dumps(intent.to_dict(), indent=2) + "\n", encoding="utf-8")

        command = _command_prefix(gait_bin) + [
            "gate",
            "eval",
            "--policy",
            str(policy_path),
            "--intent",
            str(intent_path),
            "--key-mode",
            key_mode,
            "--json",
        ]

        if trace_out is not None:
            command.extend(["--trace-out", str(trace_out)])
        if approval_token is not None:
            command.extend(["--approval-token", str(approval_token)])
        if delegation_token is not None:
            command.extend(["--delegation-token", str(delegation_token)])
        if delegation_token_chain:
            chain = ",".join(str(value) for value in delegation_token_chain)
            command.extend(["--delegation-token-chain", chain])
        if private_key is not None:
            command.extend(["--private-key", str(private_key)])
        if private_key_env:
            command.extend(["--private-key-env", private_key_env])
        if approval_public_key is not None:
            command.extend(["--approval-public-key", str(approval_public_key)])
        if approval_public_key_env:
            command.extend(["--approval-public-key-env", approval_public_key_env])
        if approval_private_key is not None:
            command.extend(["--approval-private-key", str(approval_private_key)])
        if approval_private_key_env:
            command.extend(["--approval-private-key-env", approval_private_key_env])
        if delegation_public_key is not None:
            command.extend(["--delegation-public-key", str(delegation_public_key)])
        if delegation_public_key_env:
            command.extend(["--delegation-public-key-env", delegation_public_key_env])
        if delegation_private_key is not None:
            command.extend(["--delegation-private-key", str(delegation_private_key)])
        if delegation_private_key_env:
            command.extend(["--delegation-private-key-env", delegation_private_key_env])

        result = _run_command(command, cwd=cwd)
        payload = _parse_json_stdout(result.stdout)
        if payload is None:
            raise GaitCommandError(
                "failed to parse JSON from gait gate eval",
                command=result.command,
                exit_code=result.exit_code,
                stdout=result.stdout,
                stderr=result.stderr,
            )

        if result.exit_code in (0, 4):
            return GateEvalResult.from_dict(payload, exit_code=result.exit_code)

        message = str(payload.get("error") or "gait gate eval failed")
        raise GaitCommandError(
            message,
            command=result.command,
            exit_code=result.exit_code,
            stdout=result.stdout,
            stderr=result.stderr,
        )


def write_trace(*, trace_path: str | Path, destination_path: str | Path) -> Path:
    source = Path(trace_path)
    if not source.exists():
        raise GaitError(f"trace file not found: {source}")
    payload = json.loads(source.read_text(encoding="utf-8"))
    trace = TraceRecord.from_dict(payload)
    if trace.schema_id != "gait.gate.trace":
        raise GaitError(f"unexpected trace schema_id: {trace.schema_id}")

    destination = Path(destination_path)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)
    return destination


def capture_demo_runpack(
    *, gait_bin: str | Sequence[str] = "gait", cwd: str | Path | None = None
) -> DemoCapture:
    result = _run_command(_command_prefix(gait_bin) + ["demo"], cwd=cwd)
    if result.exit_code != 0:
        raise GaitCommandError(
            "gait demo failed",
            command=result.command,
            exit_code=result.exit_code,
            stdout=result.stdout,
            stderr=result.stderr,
        )

    run_id = ""
    bundle_path = ""
    ticket_footer = ""
    verified = False
    for line in result.stdout.splitlines():
        if line.startswith("run_id="):
            run_id = line.removeprefix("run_id=").strip()
        elif line.startswith("bundle="):
            bundle_path = line.removeprefix("bundle=").strip()
        elif line.startswith("ticket_footer="):
            ticket_footer = line.removeprefix("ticket_footer=").strip()
        elif line.strip() == "verify=ok":
            verified = True

    if not run_id or not bundle_path:
        raise GaitError("unable to parse gait demo output")
    return DemoCapture(
        run_id=run_id,
        bundle_path=bundle_path,
        ticket_footer=ticket_footer,
        verified=verified,
        raw_output=result.stdout,
    )


def create_regress_fixture(
    *,
    from_run: str,
    gait_bin: str | Sequence[str] = "gait",
    cwd: str | Path | None = None,
) -> RegressInitResult:
    command = _command_prefix(gait_bin) + ["regress", "init", "--from", from_run, "--json"]
    result = _run_command(command, cwd=cwd)
    payload = _parse_json_stdout(result.stdout)
    if payload is None:
        raise GaitCommandError(
            "failed to parse JSON from gait regress init",
            command=result.command,
            exit_code=result.exit_code,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    if result.exit_code != 0:
        message = str(payload.get("error") or "gait regress init failed")
        raise GaitCommandError(
            message,
            command=result.command,
            exit_code=result.exit_code,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    if not bool(payload.get("ok", False)):
        raise GaitError("gait regress init returned ok=false")
    return RegressInitResult.from_dict(payload)


def record_runpack(
    *,
    record_input: Mapping[str, Any],
    gait_bin: str | Sequence[str] = "gait",
    cwd: str | Path | None = None,
    out_dir: str | Path = "gait-out",
    capture_mode: str = "reference",
    context_evidence_mode: str | None = None,
    context_envelope: str | Path | None = None,
) -> RunRecordCapture:
    if capture_mode not in {"reference", "raw"}:
        raise GaitError("capture_mode must be 'reference' or 'raw'")

    with tempfile.TemporaryDirectory(prefix="gait-run-record-") as tmp_dir:
        input_path = Path(tmp_dir) / "run_record.json"
        input_path.write_text(json.dumps(dict(record_input), indent=2) + "\n", encoding="utf-8")

        command = _command_prefix(gait_bin) + [
            "run",
            "record",
            "--input",
            str(input_path),
            "--out-dir",
            str(out_dir),
            "--capture-mode",
            capture_mode,
            "--json",
        ]
        if context_evidence_mode:
            command.extend(["--context-evidence-mode", str(context_evidence_mode)])
        if context_envelope is not None:
            command.extend(["--context-envelope", str(context_envelope)])
        result = _run_command(command, cwd=cwd)
        payload = _parse_json_stdout(result.stdout)
        if payload is None:
            raise GaitCommandError(
                "failed to parse JSON from gait run record",
                command=result.command,
                exit_code=result.exit_code,
                stdout=result.stdout,
                stderr=result.stderr,
            )
        if result.exit_code != 0:
            message = str(payload.get("error") or "gait run record failed")
            raise GaitCommandError(
                message,
                command=result.command,
                exit_code=result.exit_code,
                stdout=result.stdout,
                stderr=result.stderr,
            )
        if not bool(payload.get("ok", False)):
            raise GaitError("gait run record returned ok=false")

        return RunRecordCapture(
            run_id=str(payload.get("run_id", "")),
            bundle_path=str(payload.get("bundle", "")),
            manifest_digest=str(payload.get("manifest_digest", "")),
            ticket_footer=str(payload.get("ticket_footer", "")),
        )


def _run_command(command: Sequence[str], *, cwd: str | Path | None) -> _CommandResult:
    command_list = list(command)
    try:
        completed = subprocess.run(  # nosec B603
            command_list,
            cwd=str(cwd) if cwd is not None else None,
            capture_output=True,
            text=True,
            check=False,
            timeout=DEFAULT_COMMAND_TIMEOUT_SECONDS,
        )
    except FileNotFoundError as not_found_error:
        binary = command_list[0] if command_list else "gait"
        raise GaitCommandError(
            (
                f"unable to execute '{binary}': binary not found. "
                "Install gait and ensure it is available on PATH, or pass gait_bin explicitly."
            ),
            command=command_list,
            exit_code=127,
            stdout="",
            stderr=str(not_found_error),
        ) from not_found_error
    except subprocess.TimeoutExpired as timeout_error:
        raise GaitCommandError(
            "gait command timed out",
            command=command_list,
            exit_code=-1,
            stdout=str(timeout_error.stdout or ""),
            stderr=str(timeout_error.stderr or ""),
        ) from timeout_error
    return _CommandResult(
        command=command_list,
        exit_code=completed.returncode,
        stdout=completed.stdout,
        stderr=completed.stderr,
    )


def _parse_json_stdout(stdout: str) -> dict[str, Any] | None:
    content = stdout.strip()
    if not content:
        return None
    try:
        parsed = json.loads(content)
    except json.JSONDecodeError:
        return None
    if not isinstance(parsed, dict):
        return None
    return parsed


def _command_prefix(gait_bin: str | Sequence[str]) -> list[str]:
    if isinstance(gait_bin, str):
        return [gait_bin]
    return [str(part) for part in gait_bin]
