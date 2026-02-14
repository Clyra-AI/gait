#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import socket
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib import error, request


def usage() -> int:
    print(
        "usage: check_ui_budgets.py <gait_binary_path> <ui_budgets.json> <report.json>",
        file=sys.stderr,
    )
    return 2


def load_json(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError(f"{path} must be a JSON object")
    return payload


def pick_port() -> int:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return int(port)


def get_json(url: str) -> dict[str, Any]:
    with request.urlopen(url, timeout=2) as response:
        body = response.read().decode("utf-8")
    payload = json.loads(body)
    if not isinstance(payload, dict):
        raise ValueError(f"expected object response from {url}")
    return payload


def post_json(url: str, payload: dict[str, Any]) -> dict[str, Any]:
    req = request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={"content-type": "application/json"},
        method="POST",
    )
    with request.urlopen(req, timeout=120) as response:
        body = response.read().decode("utf-8")
    parsed = json.loads(body)
    if not isinstance(parsed, dict):
        raise ValueError(f"expected object response from {url}")
    return parsed


def read_rss_kib(pid: int) -> int:
    result = subprocess.run(
        ["ps", "-o", "rss=", "-p", str(pid)],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(f"failed to read rss for pid {pid}: {result.stderr.strip()}")
    value = result.stdout.strip()
    if not value:
        raise RuntimeError(f"rss output was empty for pid {pid}")
    return int(value)


def now_utc() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def main() -> int:
    if len(sys.argv) != 4:
        return usage()

    gait_path = Path(sys.argv[1]).resolve()
    budget_path = Path(sys.argv[2]).resolve()
    report_path = Path(sys.argv[3]).resolve()
    report_path.parent.mkdir(parents=True, exist_ok=True)

    if not gait_path.exists():
        print(f"gait binary not found: {gait_path}", file=sys.stderr)
        return 2

    try:
        budgets = load_json(budget_path)
    except (OSError, ValueError, json.JSONDecodeError) as err:
        print(f"load budgets: {err}", file=sys.stderr)
        return 2

    required = ("startup_tti_ms", "command_roundtrip_ms", "max_rss_kib")
    for key in required:
        value = budgets.get(key)
        if not isinstance(value, (int, float)):
            print(f"invalid budget field {key}: expected number", file=sys.stderr)
            return 2

    port = pick_port()
    base_url = f"http://127.0.0.1:{port}"
    failures: list[str] = []
    report: dict[str, Any] = {
        "schema_id": "gait.perf.ui_budget_report",
        "schema_version": "1.0.0",
        "generated_at": now_utc(),
        "budgets": {
            "startup_tti_ms": float(budgets["startup_tti_ms"]),
            "command_roundtrip_ms": float(budgets["command_roundtrip_ms"]),
            "max_rss_kib": float(budgets["max_rss_kib"]),
        },
        "metrics": {},
        "failures": failures,
    }

    with tempfile.TemporaryDirectory(prefix="gait-ui-budget-") as temp_dir:
        work_dir = Path(temp_dir)
        ui_log = work_dir / "ui_perf.log"
        with ui_log.open("w", encoding="utf-8") as log_handle:
            process = subprocess.Popen(
                [
                    str(gait_path),
                    "ui",
                    "--listen",
                    f"127.0.0.1:{port}",
                    "--open-browser=false",
                ],
                cwd=work_dir,
                stdout=log_handle,
                stderr=log_handle,
                env=os.environ.copy(),
            )
            try:
                startup_start = time.perf_counter()
                ready = False
                last_health_error: str | None = None
                for _ in range(120):
                    if process.poll() is not None:
                        break
                    try:
                        health = get_json(f"{base_url}/api/health")
                        if health.get("ok") is True:
                            ready = True
                            break
                    except (error.URLError, TimeoutError, ValueError, json.JSONDecodeError) as err:
                        last_health_error = str(err)
                    time.sleep(0.25)
                startup_tti_ms = (time.perf_counter() - startup_start) * 1000.0
                report["metrics"]["startup_tti_ms"] = startup_tti_ms
                report["metrics"]["ready"] = ready
                report["metrics"]["ui_log_path"] = str(ui_log)
                if last_health_error is not None:
                    report["metrics"]["last_health_error"] = last_health_error

                if not ready:
                    failures.append("ui server did not become healthy")
                else:
                    command_start = time.perf_counter()
                    demo_payload = post_json(
                        f"{base_url}/api/exec",
                        {"command": "demo", "args": {}},
                    )
                    command_roundtrip_ms = (time.perf_counter() - command_start) * 1000.0
                    report["metrics"]["command_roundtrip_ms"] = command_roundtrip_ms
                    report["metrics"]["demo_exit_code"] = int(demo_payload.get("exit_code", -1))
                    report["metrics"]["demo_ok"] = bool(demo_payload.get("ok", False))
                    if demo_payload.get("exit_code") != 0:
                        failures.append(f"ui demo command failed: exit_code={demo_payload.get('exit_code')}")

                    rss_kib = read_rss_kib(process.pid)
                    report["metrics"]["rss_kib"] = rss_kib

                if startup_tti_ms > float(budgets["startup_tti_ms"]):
                    failures.append(
                        f"startup_tti_ms over budget: observed={startup_tti_ms:.1f} budget={float(budgets['startup_tti_ms']):.1f}"
                    )
                if "command_roundtrip_ms" in report["metrics"]:
                    observed = float(report["metrics"]["command_roundtrip_ms"])
                    if observed > float(budgets["command_roundtrip_ms"]):
                        failures.append(
                            f"command_roundtrip_ms over budget: observed={observed:.1f} budget={float(budgets['command_roundtrip_ms']):.1f}"
                        )
                if "rss_kib" in report["metrics"]:
                    observed_rss = float(report["metrics"]["rss_kib"])
                    if observed_rss > float(budgets["max_rss_kib"]):
                        failures.append(
                            f"rss_kib over budget: observed={observed_rss:.0f} budget={float(budgets['max_rss_kib']):.0f}"
                        )
            except Exception as err:  # noqa: BLE001
                failures.append(f"runtime error: {err}")
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=10)

            log_tail = ui_log.read_text(encoding="utf-8", errors="replace").splitlines()[-30:]
            report["log_tail"] = log_tail

    report["status"] = "pass" if not failures else "fail"
    report_path.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")

    if failures:
        print("ui budget check failed:", file=sys.stderr)
        for item in failures:
            print(f"- {item}", file=sys.stderr)
        return 1

    print(
        "ui budget check passed "
        f"(startup={report['metrics']['startup_tti_ms']:.1f}ms "
        f"roundtrip={report['metrics']['command_roundtrip_ms']:.1f}ms "
        f"rss={report['metrics']['rss_kib']}KiB)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
