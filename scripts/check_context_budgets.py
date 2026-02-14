#!/usr/bin/env python3
from __future__ import annotations

import json
import math
import statistics
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def usage() -> int:
    print(
        "usage: check_context_budgets.py <gait_binary_path> <context_budgets.json> <report.json>",
        file=sys.stderr,
    )
    return 2


def now_utc() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def load_json(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError(f"{path} must be a JSON object")
    return payload


def run_json(
    command: list[str],
    cwd: Path,
    allowed_exit_codes: tuple[int, ...] = (0,),
    require_ok_true: bool = True,
) -> dict[str, Any]:
    result = subprocess.run(
        command,
        cwd=cwd,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode not in allowed_exit_codes:
        raise RuntimeError(
            f"command failed ({result.returncode}): {' '.join(command)}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    try:
        payload = json.loads(result.stdout)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"non-json output for {' '.join(command)}: {err}") from err
    if not isinstance(payload, dict):
        raise RuntimeError(f"expected object output for {' '.join(command)}")
    if require_ok_true and payload.get("ok") is False:
        raise RuntimeError(f"command returned ok=false: {' '.join(command)} -> {payload}")
    return payload


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def percentile_ms(samples: list[float], p: float) -> float:
    if not samples:
        return math.inf
    if p <= 0:
        return float(min(samples))
    if p >= 100:
        return float(max(samples))
    ordered = sorted(samples)
    rank = (len(ordered) - 1) * (p / 100.0)
    lower = int(math.floor(rank))
    upper = int(math.ceil(rank))
    if lower == upper:
        return float(ordered[lower])
    weight = rank - lower
    return float(ordered[lower] + (ordered[upper] - ordered[lower]) * weight)


def build_run_record_input(run_id: str) -> dict[str, Any]:
    return {
        "run": {
            "schema_id": "gait.runpack.run",
            "schema_version": "1.0.0",
            "created_at": "2026-02-14T00:00:00Z",
            "producer_version": "budget-check",
            "run_id": run_id,
            "env": {"os": "darwin", "arch": "arm64", "runtime": "go"},
            "timeline": [{"event": "run_started", "ts": "2026-02-14T00:00:00Z"}],
        },
        "intents": [],
        "results": [],
        "refs": {
            "schema_id": "gait.runpack.refs",
            "schema_version": "1.0.0",
            "created_at": "2026-02-14T00:00:00Z",
            "producer_version": "budget-check",
            "run_id": run_id,
            "receipts": [],
        },
        "capture_mode": "reference",
    }


def build_envelope(content_digest: str, retrieved_at: str) -> dict[str, Any]:
    return {
        "schema_id": "gait.context.envelope",
        "schema_version": "1.0.0",
        "created_at": "2026-02-14T00:00:00Z",
        "producer_version": "budget-check",
        "context_set_id": "ctx_budget",
        "context_set_digest": "",
        "evidence_mode": "required",
        "records": [
            {
                "ref_id": "ctx_001",
                "source_type": "doc_store",
                "source_locator": "docs://policy/security",
                "query_digest": "1" * 64,
                "content_digest": content_digest,
                "retrieved_at": retrieved_at,
                "redaction_mode": "reference",
                "immutability": "immutable",
            }
        ],
    }


def operation_context_envelope_build_verify(gait: Path, work_dir: Path, attempt: int) -> None:
    out_dir = work_dir / "out"
    out_dir.mkdir(parents=True, exist_ok=True)

    run_id = f"run_ctx_budget_{attempt}"
    run_record_path = work_dir / f"run_record_{attempt}.json"
    envelope_path = work_dir / f"context_envelope_{attempt}.json"
    run_record_path.write_text(json.dumps(build_run_record_input(run_id), indent=2) + "\n", encoding="utf-8")
    envelope_path.write_text(
        json.dumps(
            build_envelope("2" * 64, "2026-02-14T00:00:00Z"),
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    record = run_json(
        [
            str(gait),
            "run",
            "record",
            "--input",
            str(run_record_path),
            "--out-dir",
            str(out_dir),
            "--context-envelope",
            str(envelope_path),
            "--context-evidence-mode",
            "required",
            "--json",
        ],
        work_dir,
    )
    bundle = str(record.get("bundle", "")).strip()
    if not bundle:
        raise RuntimeError("run record output missing bundle path")
    run_json([str(gait), "verify", bundle, "--json"], work_dir)


def operation_gate_eval_context_required(gait: Path, work_dir: Path, _: int) -> None:
    policy_path = work_dir / "policy.yaml"
    policy_path.write_text(
        "\n".join(
            [
                "schema_id: gait.gate.policy",
                "schema_version: 1.0.0",
                "default_verdict: allow",
                "rules:",
                "  - name: require-context",
                "    priority: 1",
                "    effect: allow",
                "    match:",
                "      risk_classes: [high]",
                "    require_context_evidence: true",
                "    required_context_evidence_mode: required",
            ]
        )
        + "\n",
        encoding="utf-8",
    )
    intent_path = work_dir / "intent.json"
    intent_path.write_text(
        json.dumps(
            {
                "schema_id": "gait.gate.intent_request",
                "schema_version": "1.0.0",
                "created_at": "2026-02-14T00:00:00Z",
                "producer_version": "budget-check",
                "tool_name": "tool.demo",
                "args": {"x": "y"},
                "targets": [{"kind": "path", "value": "/tmp/demo", "operation": "write"}],
                "context": {
                    "identity": "alice",
                    "workspace": "/repo/gait",
                    "risk_class": "high",
                    "context_set_digest": "a" * 64,
                    "context_evidence_mode": "required",
                    "context_refs": ["ctx_001"],
                },
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )
    decision = run_json(
        [str(gait), "gate", "eval", "--policy", str(policy_path), "--intent", str(intent_path), "--json"],
        work_dir,
    )
    if decision.get("verdict") != "allow":
        raise RuntimeError(f"expected allow verdict, got {decision}")


def operation_pack_diff_context_classification(gait: Path, work_dir: Path, _: int) -> None:
    out_dir = work_dir / "pack_diff"
    out_dir.mkdir(parents=True, exist_ok=True)

    run_record_a = out_dir / "run_a.json"
    run_record_b = out_dir / "run_b.json"
    envelope_a = out_dir / "env_a.json"
    envelope_b = out_dir / "env_b.json"

    run_record_a.write_text(json.dumps(build_run_record_input("run_ctx_diff_a"), indent=2) + "\n", encoding="utf-8")
    run_record_b.write_text(json.dumps(build_run_record_input("run_ctx_diff_b"), indent=2) + "\n", encoding="utf-8")
    envelope_a.write_text(
        json.dumps(build_envelope("a" * 64, "2026-02-14T00:00:00Z"), indent=2) + "\n",
        encoding="utf-8",
    )
    envelope_b.write_text(
        json.dumps(build_envelope("b" * 64, "2026-02-14T00:00:00Z"), indent=2) + "\n",
        encoding="utf-8",
    )

    record_a = run_json(
        [
            str(gait),
            "run",
            "record",
            "--input",
            str(run_record_a),
            "--out-dir",
            str(out_dir),
            "--context-envelope",
            str(envelope_a),
            "--context-evidence-mode",
            "required",
            "--json",
        ],
        work_dir,
    )
    record_b = run_json(
        [
            str(gait),
            "run",
            "record",
            "--input",
            str(run_record_b),
            "--out-dir",
            str(out_dir),
            "--context-envelope",
            str(envelope_b),
            "--context-evidence-mode",
            "required",
            "--json",
        ],
        work_dir,
    )
    bundle_a = str(record_a.get("bundle", "")).strip()
    bundle_b = str(record_b.get("bundle", "")).strip()
    if not bundle_a or not bundle_b:
        raise RuntimeError("missing bundle paths for pack diff operation")

    pack_a = out_dir / "pack_a.zip"
    pack_b = out_dir / "pack_b.zip"
    run_json([str(gait), "pack", "build", "--type", "run", "--from", bundle_a, "--out", str(pack_a), "--json"], work_dir)
    run_json([str(gait), "pack", "build", "--type", "run", "--from", bundle_b, "--out", str(pack_b), "--json"], work_dir)
    diff = run_json(
        [str(gait), "pack", "diff", str(pack_a), str(pack_b), "--json"],
        work_dir,
        allowed_exit_codes=(0, 2),
        require_ok_true=False,
    )
    diff_payload = diff.get("diff") or {}
    result_payload = diff_payload.get("result") or diff_payload.get("Result") or {}
    summary = result_payload.get("summary") or {}
    if summary.get("context_drift_classification") != "semantic":
        raise RuntimeError(f"expected semantic context drift classification, got {summary}")


def operation_regress_context_grader_run(gait: Path, work_dir: Path, _: int) -> None:
    out_dir = work_dir / "regress_ctx"
    out_dir.mkdir(parents=True, exist_ok=True)

    source_input = out_dir / "run_source.json"
    candidate_input = out_dir / "run_candidate.json"
    source_env = out_dir / "env_source.json"
    candidate_env = out_dir / "env_candidate.json"

    source_input.write_text(json.dumps(build_run_record_input("run_ctx_regress_src"), indent=2) + "\n", encoding="utf-8")
    candidate_input.write_text(
        json.dumps(build_run_record_input("run_ctx_regress_src"), indent=2) + "\n",
        encoding="utf-8",
    )
    source_env.write_text(
        json.dumps(build_envelope("2" * 64, "2026-02-14T00:00:00Z"), indent=2) + "\n",
        encoding="utf-8",
    )
    candidate_env.write_text(
        json.dumps(build_envelope("2" * 64, "2026-02-14T00:05:00Z"), indent=2) + "\n",
        encoding="utf-8",
    )

    source_record = run_json(
        [
            str(gait),
            "run",
            "record",
            "--input",
            str(source_input),
            "--out-dir",
            str(out_dir / "source"),
            "--context-envelope",
            str(source_env),
            "--context-evidence-mode",
            "required",
            "--json",
        ],
        work_dir,
    )
    candidate_record = run_json(
        [
            str(gait),
            "run",
            "record",
            "--input",
            str(candidate_input),
            "--out-dir",
            str(out_dir / "candidate"),
            "--context-envelope",
            str(candidate_env),
            "--context-evidence-mode",
            "required",
            "--json",
        ],
        work_dir,
    )
    source_bundle = str(source_record.get("bundle", "")).strip()
    candidate_bundle = str(candidate_record.get("bundle", "")).strip()
    if not source_bundle or not candidate_bundle:
        raise RuntimeError("missing source/candidate runpack path")

    init = run_json([str(gait), "regress", "init", "--from", source_bundle, "--json"], out_dir)
    fixture_dir = str(init.get("fixture_dir", "")).strip()
    if not fixture_dir:
        raise RuntimeError("regress init did not return fixture_dir")
    fixture_meta_path = out_dir / fixture_dir / "fixture.json"
    fixture_meta = load_json(fixture_meta_path)
    fixture_meta["candidate_runpack"] = candidate_bundle
    fixture_meta["diff_allow_changed_files"] = ["manifest.json", "refs.json"]
    fixture_meta["context_conformance"] = "required"
    fixture_meta["allow_context_runtime_drift"] = True
    fixture_meta["expected_context_set_digest"] = ""
    write_json(fixture_meta_path, fixture_meta)

    result = run_json(
        [
            str(gait),
            "regress",
            "run",
            "--config",
            str(out_dir / "gait.yaml"),
            "--context-conformance",
            "--allow-context-runtime-drift",
            "--json",
        ],
        out_dir,
    )
    if result.get("status") != "pass":
        raise RuntimeError(f"expected regress pass status, got {result}")


def main() -> int:
    if len(sys.argv) != 4:
        return usage()

    gait_path = Path(sys.argv[1]).resolve()
    budgets_path = Path(sys.argv[2]).resolve()
    report_path = Path(sys.argv[3]).resolve()
    if not gait_path.exists():
        print(f"gait binary not found: {gait_path}", file=sys.stderr)
        return 2

    try:
        budgets = load_json(budgets_path)
    except (OSError, ValueError, json.JSONDecodeError) as err:
        print(f"load context budgets: {err}", file=sys.stderr)
        return 2

    repeats = int(budgets.get("repeats", 0))
    operations_budget = budgets.get("operations")
    if repeats < 1 or not isinstance(operations_budget, dict):
        print("context budget file must include repeats>=1 and operations object", file=sys.stderr)
        return 2

    operations = {
        "context_envelope_build_verify": operation_context_envelope_build_verify,
        "gate_eval_context_required": operation_gate_eval_context_required,
        "pack_diff_context_classification": operation_pack_diff_context_classification,
        "regress_context_grader_run": operation_regress_context_grader_run,
    }

    failures: list[str] = []
    report: dict[str, Any] = {
        "schema_id": "gait.perf.context_budget_report",
        "schema_version": "1.0.0",
        "generated_at": now_utc(),
        "budget_source": str(budgets_path),
        "repeats": repeats,
        "operations": {},
        "failures": failures,
    }

    with tempfile.TemporaryDirectory(prefix="gait-context-budget-") as temp_dir:
        work_dir = Path(temp_dir)
        for operation_name, operation_func in operations.items():
            budget = operations_budget.get(operation_name)
            if not isinstance(budget, dict):
                failures.append(f"missing budget for operation: {operation_name}")
                continue
            p95_budget = float(budget.get("p95_ms", 0.0))
            max_error_rate = float(budget.get("max_error_rate", 0.0))
            samples_ms: list[float] = []
            run_failures: list[str] = []
            for attempt in range(repeats):
                start = time.perf_counter()
                try:
                    operation_func(gait_path, work_dir, attempt)
                except Exception as err:  # noqa: BLE001
                    run_failures.append(f"attempt={attempt + 1}: {err}")
                    continue
                samples_ms.append((time.perf_counter() - start) * 1000.0)

            error_rate = 0.0
            if repeats > 0:
                error_rate = len(run_failures) / float(repeats)
            p50_ms = float(statistics.median(samples_ms)) if samples_ms else math.inf
            p95_ms = percentile_ms(samples_ms, 95.0)
            status = "pass"
            if error_rate > max_error_rate:
                status = "fail"
                failures.append(
                    f"{operation_name} error_rate over budget: observed={error_rate:.3f} budget={max_error_rate:.3f}"
                )
            if p95_ms > p95_budget:
                status = "fail"
                failures.append(
                    f"{operation_name} p95 over budget: observed={p95_ms:.1f}ms budget={p95_budget:.1f}ms"
                )

            report["operations"][operation_name] = {
                "samples_ms": samples_ms,
                "p50_ms": p50_ms,
                "p95_ms": p95_ms,
                "error_rate": error_rate,
                "budget_p95_ms": p95_budget,
                "budget_max_error_rate": max_error_rate,
                "attempt_failures": run_failures,
                "status": status,
            }

    report["status"] = "pass" if not failures else "fail"
    write_json(report_path, report)

    if failures:
        print("context budget check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print("context budget check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
