#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Canonical Gait sidecar: evaluate IntentRequest with gait gate eval."
    )
    parser.add_argument("--policy", required=True, help="Path to gate policy yaml")
    parser.add_argument(
        "--intent-file",
        default="-",
        help='IntentRequest JSON file path. Use "-" to read from stdin.',
    )
    parser.add_argument("--gait-bin", default="gait", help="gait executable path")
    parser.add_argument("--key-mode", default="dev", choices=["dev", "prod"])
    parser.add_argument(
        "--profile", default="standard", choices=["standard", "oss-prod"]
    )
    parser.add_argument("--trace-out", default="", help="Optional trace output path")
    parser.add_argument(
        "--delegation-token",
        default="",
        help="Optional delegation token path for delegated runtime actions.",
    )
    parser.add_argument(
        "--delegation-token-chain",
        default="",
        help="Optional comma-separated delegation token chain paths.",
    )
    parser.add_argument(
        "--delegation-public-key",
        default="",
        help="Optional base64 public key path for delegation token verification.",
    )
    parser.add_argument(
        "--delegation-private-key",
        default="",
        help="Optional base64 private key path for delegation token verification (public derived).",
    )
    return parser.parse_args()


def _load_intent(intent_file: str) -> dict[str, Any]:
    if intent_file == "-":
        raw = sys.stdin.read()
    else:
        raw = Path(intent_file).read_text(encoding="utf-8")
    payload = json.loads(raw)
    if not isinstance(payload, dict):
        raise ValueError("intent payload must be a JSON object")
    if payload.get("schema_id") != "gait.gate.intent_request":
        raise ValueError("intent schema_id must be gait.gate.intent_request")
    if payload.get("schema_version") != "1.0.0":
        raise ValueError("intent schema_version must be 1.0.0")
    return payload


def main() -> int:
    args = _parse_args()
    try:
        intent = _load_intent(args.intent_file)
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"ok": False, "error": f"invalid intent request: {exc}"}))
        return 2

    with tempfile.TemporaryDirectory(prefix="gait-sidecar-") as temp_dir:
        intent_path = Path(temp_dir) / "intent.json"
        intent_path.write_text(json.dumps(intent, indent=2) + "\n", encoding="utf-8")

        command = [
            args.gait_bin,
            "gate",
            "eval",
            "--policy",
            args.policy,
            "--intent",
            str(intent_path),
            "--key-mode",
            args.key_mode,
            "--profile",
            args.profile,
            "--json",
        ]
        if args.trace_out:
            command.extend(["--trace-out", args.trace_out])
        if args.delegation_token:
            command.extend(["--delegation-token", args.delegation_token])
        if args.delegation_token_chain:
            command.extend(["--delegation-token-chain", args.delegation_token_chain])
        if args.delegation_public_key:
            command.extend(["--delegation-public-key", args.delegation_public_key])
        if args.delegation_private_key:
            command.extend(["--delegation-private-key", args.delegation_private_key])

        completed = subprocess.run(  # nosec B603
            command,
            capture_output=True,
            text=True,
            check=False,
        )

    parsed_output: dict[str, Any] | None = None
    stdout = completed.stdout.strip()
    if stdout:
        try:
            decoded = json.loads(stdout)
            if isinstance(decoded, dict):
                parsed_output = decoded
        except json.JSONDecodeError:
            parsed_output = None

    if parsed_output is None:
        response = {
            "ok": False,
            "exit_code": completed.returncode,
            "error": "failed to parse gait gate eval output",
            "stdout": completed.stdout,
            "stderr": completed.stderr,
        }
        print(json.dumps(response))
        return completed.returncode if completed.returncode != 0 else 6

    response = {
        "ok": completed.returncode in (0, 3, 4),
        "exit_code": completed.returncode,
        "gate_result": parsed_output,
        "trace_path": parsed_output.get("trace_path"),
    }
    print(json.dumps(response))
    return completed.returncode


if __name__ == "__main__":
    raise SystemExit(main())
