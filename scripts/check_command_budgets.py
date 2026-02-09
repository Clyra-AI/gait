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

DEFAULT_RUNTIME_SLO_BUDGETS: dict[str, Any] = {
    "schema_id": "gait.perf.runtime_slo_budgets",
    "schema_version": "1.0.0",
    "repeats": 7,
    "commands": {
        "verify": {
            "p50_ms": 1000.0,
            "p95_ms": 1800.0,
            "p99_ms": 2200.0,
            "max_error_rate": 0.0,
        },
        "regress_run": {
            "p50_ms": 2000.0,
            "p95_ms": 3000.0,
            "p99_ms": 3600.0,
            "max_error_rate": 0.0,
        },
        "guard_pack": {
            "p50_ms": 2000.0,
            "p95_ms": 3000.0,
            "p99_ms": 3600.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_fs_read": {
            "p50_ms": 800.0,
            "p95_ms": 1500.0,
            "p99_ms": 2100.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_fs_write": {
            "p50_ms": 800.0,
            "p95_ms": 1500.0,
            "p99_ms": 2100.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_fs_delete": {
            "p50_ms": 800.0,
            "p95_ms": 1500.0,
            "p99_ms": 2100.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_proc_exec": {
            "p50_ms": 800.0,
            "p95_ms": 1500.0,
            "p99_ms": 2100.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_net_http": {
            "p50_ms": 900.0,
            "p95_ms": 1600.0,
            "p99_ms": 2200.0,
            "max_error_rate": 0.0,
        },
        "gate_eval_net_dns": {
            "p50_ms": 900.0,
            "p95_ms": 1600.0,
            "p99_ms": 2200.0,
            "max_error_rate": 0.0,
        },
    },
}

DEFAULT_BUDGET_PATH = Path("perf/runtime_slo_budgets.json")
UTC = timezone.utc


def run_checked(command: list[str], cwd: Path) -> None:
    result = subprocess.run(
        command,
        cwd=cwd,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"command failed ({result.returncode}): {' '.join(command)}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )


def run_measured(command: list[str], cwd: Path) -> tuple[float, str | None]:
    start = time.perf_counter()
    result = subprocess.run(
        command,
        cwd=cwd,
        capture_output=True,
        text=True,
        check=False,
    )
    elapsed_ms = (time.perf_counter() - start) * 1000.0
    if result.returncode == 0:
        return elapsed_ms, None
    stderr = result.stderr.strip()
    if len(stderr) > 220:
        stderr = stderr[:220] + "..."
    return elapsed_ms, f"exit={result.returncode} stderr={stderr}"


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


def measure_command(
    command: list[str], cwd: Path, repeats: int
) -> tuple[list[float], list[str]]:
    timings_ms: list[float] = []
    failures: list[str] = []
    for attempt in range(repeats):
        elapsed_ms, error = run_measured(command, cwd)
        if error is None:
            timings_ms.append(elapsed_ms)
            continue
        failures.append(f"attempt={attempt + 1} {error}")
    return timings_ms, failures


def load_runtime_budgets(path: Path | None) -> tuple[dict[str, Any], str]:
    if path is None:
        if DEFAULT_BUDGET_PATH.exists():
            path = DEFAULT_BUDGET_PATH
        else:
            return DEFAULT_RUNTIME_SLO_BUDGETS, "built-in-defaults"

    raw = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError("runtime budget file must be a JSON object")
    commands = raw.get("commands")
    if not isinstance(commands, dict):
        raise ValueError("runtime budget file missing commands object")
    repeats = raw.get("repeats")
    if not isinstance(repeats, int) or repeats < 1:
        raise ValueError("runtime budget file repeats must be an integer >= 1")
    for command_name, budget in commands.items():
        if not isinstance(budget, dict):
            raise ValueError(f"runtime budget for {command_name} must be an object")
        for field in ("p50_ms", "p95_ms", "p99_ms", "max_error_rate"):
            value = budget.get(field)
            if not isinstance(value, (int, float)):
                raise ValueError(f"runtime budget field {field} missing for {command_name}")
    return raw, str(path)


def write_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    if len(sys.argv) not in (3, 4):
        print(
            "usage: check_command_budgets.py <gait_binary_path> <report.json> [runtime_slo_budgets.json]",
            file=sys.stderr,
        )
        return 2

    gait_path = Path(sys.argv[1]).resolve()
    report_path = Path(sys.argv[2])
    budgets_path: Path | None = None
    if len(sys.argv) == 4:
        budgets_path = Path(sys.argv[3])
    if not gait_path.exists():
        print(f"gait binary not found: {gait_path}", file=sys.stderr)
        return 2

    try:
        runtime_budgets, budget_source = load_runtime_budgets(budgets_path)
    except (ValueError, OSError, json.JSONDecodeError) as err:
        print(f"load runtime budgets: {err}", file=sys.stderr)
        return 2

    command_budgets = runtime_budgets["commands"]
    repeats = int(runtime_budgets["repeats"])

    failures: list[str] = []
    report: dict[str, Any] = {
        "schema_id": "gait.perf.command_budget_report",
        "schema_version": "2.0.0",
        "generated_at": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
        "runtime_budget_source": budget_source,
        "repeats": repeats,
        "commands": {},
        "failures": failures,
    }

    with tempfile.TemporaryDirectory(prefix="gait-budget-") as temp_dir:
        work_dir = Path(temp_dir)
        policy_path = work_dir / "policy.yaml"
        policy_path.write_text(
            "default_verdict: allow\n"
            "rules:\n"
            "  - name: allow-write\n"
            "    effect: allow\n"
            "    match:\n"
            "      tool_names: [tool.read,tool.write,tool.delete,tool.exec,tool.http,tool.dns]\n",
            encoding="utf-8",
        )
        intents_dir = work_dir / "intents"
        intents_dir.mkdir(parents=True, exist_ok=True)

        intent_specs = {
            "gate_eval_fs_read": {
                "tool_name": "tool.read",
                "target": {
                    "kind": "path",
                    "value": "/tmp/gait/slo/read.txt",
                    "operation": "read",
                    "endpoint_class": "fs.read",
                },
            },
            "gate_eval_fs_write": {
                "tool_name": "tool.write",
                "target": {
                    "kind": "path",
                    "value": "/tmp/gait/slo/write.txt",
                    "operation": "write",
                    "endpoint_class": "fs.write",
                },
            },
            "gate_eval_fs_delete": {
                "tool_name": "tool.delete",
                "target": {
                    "kind": "path",
                    "value": "/tmp/gait/slo/delete.txt",
                    "operation": "delete",
                    "endpoint_class": "fs.delete",
                    "destructive": True,
                },
            },
            "gate_eval_proc_exec": {
                "tool_name": "tool.exec",
                "target": {
                    "kind": "other",
                    "value": "local-shell",
                    "operation": "exec",
                    "endpoint_class": "proc.exec",
                    "destructive": True,
                },
            },
            "gate_eval_net_http": {
                "tool_name": "tool.http",
                "target": {
                    "kind": "url",
                    "value": "https://api.example.com/v1/health",
                    "operation": "get",
                    "endpoint_class": "net.http",
                    "endpoint_domain": "api.example.com",
                },
            },
            "gate_eval_net_dns": {
                "tool_name": "tool.dns",
                "target": {
                    "kind": "host",
                    "value": "api.example.com",
                    "operation": "dns",
                    "endpoint_class": "net.dns",
                    "endpoint_domain": "api.example.com",
                },
            },
        }
        intent_paths: dict[str, Path] = {}
        for command_name, spec in intent_specs.items():
            payload = {
                "schema_id": "gait.gate.intent_request",
                "schema_version": "1.0.0",
                "created_at": "2026-02-06T00:00:00Z",
                "producer_version": "0.0.0-bench",
                "tool_name": spec["tool_name"],
                "args": {"path": spec["target"]["value"]},
                "targets": [spec["target"]],
                "arg_provenance": [{"arg_path": "$.path", "source": "user"}],
                "context": {
                    "identity": "alice",
                    "workspace": str(work_dir),
                    "risk_class": "high",
                },
            }
            intent_path = intents_dir / f"{command_name}.json"
            write_json(intent_path, payload)
            intent_paths[command_name] = intent_path

        run_checked([str(gait_path), "demo", "--json"], work_dir)
        run_checked([str(gait_path), "regress", "init", "--from", "run_demo", "--json"], work_dir)

        command_map = {
            "verify": [str(gait_path), "verify", "run_demo", "--json"],
            "regress_run": [str(gait_path), "regress", "run", "--json"],
            "guard_pack": [
                str(gait_path),
                "guard",
                "pack",
                "--run",
                "run_demo",
                "--out",
                "guard_pack.zip",
                "--json",
            ],
        }
        for command_name, intent_path in intent_paths.items():
            command_map[command_name] = [
                str(gait_path),
                "gate",
                "eval",
                "--policy",
                str(policy_path),
                "--intent",
                str(intent_path),
                "--json",
            ]

        for budget_name in sorted(command_budgets.keys()):
            if budget_name not in command_map:
                failures.append(f"runtime budget configured for unknown command: {budget_name}")

        for name in sorted(command_map.keys()):
            if name not in command_budgets:
                failures.append(f"missing runtime budget for command: {name}")
                continue

            budget = command_budgets[name]
            samples, command_failures = measure_command(command_map[name], work_dir, repeats)
            successes = len(samples)
            attempt_count = repeats
            error_count = attempt_count - successes
            error_rate = float(error_count) / float(attempt_count)
            p50_ms = percentile_ms(samples, 50.0)
            p95_ms = percentile_ms(samples, 95.0)
            p99_ms = percentile_ms(samples, 99.0)
            status = "ok"

            if error_rate > float(budget["max_error_rate"]):
                status = "over_budget"
                failures.append(
                    f"{name} error_rate over budget: observed={error_rate:.3f} budget={float(budget['max_error_rate']):.3f}"
                )
            if p50_ms > float(budget["p50_ms"]):
                status = "over_budget"
                failures.append(
                    f"{name} p50 over budget: observed={p50_ms:.1f}ms budget={float(budget['p50_ms']):.1f}ms"
                )
            if p95_ms > float(budget["p95_ms"]):
                status = "over_budget"
                failures.append(
                    f"{name} p95 over budget: observed={p95_ms:.1f}ms budget={float(budget['p95_ms']):.1f}ms"
                )
            if p99_ms > float(budget["p99_ms"]):
                status = "over_budget"
                failures.append(
                    f"{name} p99 over budget: observed={p99_ms:.1f}ms budget={float(budget['p99_ms']):.1f}ms"
                )
            if command_failures:
                for failure in command_failures:
                    failures.append(f"{name} runtime failure: {failure}")

            report["commands"][name] = {
                "command": command_map[name],
                "samples_ms": samples,
                "p50_ms": p50_ms,
                "p95_ms": p95_ms,
                "p99_ms": p99_ms,
                "median_ms": float(statistics.median(samples)) if samples else math.inf,
                "attempts": attempt_count,
                "successes": successes,
                "failures": error_count,
                "error_rate": error_rate,
                "budget": {
                    "p50_ms": float(budget["p50_ms"]),
                    "p95_ms": float(budget["p95_ms"]),
                    "p99_ms": float(budget["p99_ms"]),
                    "max_error_rate": float(budget["max_error_rate"]),
                },
                "status": status,
            }

    report_path.parent.mkdir(parents=True, exist_ok=True)
    write_json(report_path, report)

    for name in sorted(report["commands"]):
        entry = report["commands"][name]
        print(
            f"{name}: p50={entry['p50_ms']:.1f}ms p95={entry['p95_ms']:.1f}ms p99={entry['p99_ms']:.1f}ms "
            f"error_rate={entry['error_rate']:.3f} status={entry['status']}"
        )

    if failures:
        print("runtime SLO budget check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print("runtime SLO budget check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
