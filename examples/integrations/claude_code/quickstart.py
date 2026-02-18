#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess  # nosec B404
from pathlib import Path
from typing import Any

FRAMEWORK = "claude_code"
CREATED_AT = "2026-02-18T00:00:00Z"

CLAUDE_TOOL_MAP: dict[str, str] = {
    "Read": "tool.read",
    "Grep": "tool.read",
    "Glob": "tool.read",
    "WebFetch": "tool.read",
    "WebSearch": "tool.read",
    "Write": "tool.write",
    "Edit": "tool.write",
    "NotebookEdit": "tool.write",
    "Bash": "tool.exec",
    "Task": "tool.delegate",
}


def resolve_repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def resolve_gait_bin(repo_root: Path) -> str:
    configured = os.environ.get("GAIT_BIN", "")
    if configured:
        return configured
    local_binary = repo_root / "gait"
    if local_binary.exists():
        return str(local_binary)
    discovered = shutil.which("gait")
    if discovered:
        return discovered
    raise RuntimeError("unable to find gait binary; set GAIT_BIN or build ./gait")


def map_claude_tool_name(raw_name: str) -> str:
    trimmed = raw_name.strip()
    if trimmed in CLAUDE_TOOL_MAP:
        return CLAUDE_TOOL_MAP[trimmed]
    lowered = trimmed.lower().replace(" ", "_")
    return f"tool.{lowered}" if lowered else "tool.unknown"


def build_claude_hook_event(scenario: str) -> dict[str, Any]:
    return {
        "session_id": f"sess-{FRAMEWORK}-{scenario}",
        "hook_event_name": "PreToolUse",
        "tool_name": "Write",
        "tool_input": {
            "path": f"/tmp/gait-{FRAMEWORK}-{scenario}.json",
            "content": {
                "framework": FRAMEWORK,
                "scenario": scenario,
                "source_tool": "Write",
            },
        },
    }


def infer_target(mapped_tool_name: str, tool_input: dict[str, Any]) -> dict[str, Any]:
    path = str(tool_input.get("path", "")).strip()
    if mapped_tool_name == "tool.write":
        return {
            "kind": "path",
            "value": path,
            "operation": "write",
            "endpoint_class": "fs.write",
        }
    if mapped_tool_name == "tool.read":
        return {
            "kind": "path",
            "value": path,
            "operation": "read",
            "endpoint_class": "fs.read",
        }
    if mapped_tool_name == "tool.exec":
        return {
            "kind": "other",
            "value": str(tool_input.get("command", "")),
            "operation": "execute",
            "endpoint_class": "proc.exec",
        }
    if mapped_tool_name == "tool.delegate":
        return {
            "kind": "other",
            "value": str(tool_input.get("task", "delegate")),
            "operation": "delegate",
            "endpoint_class": "agent.delegate",
        }
    return {
        "kind": "other",
        "value": path,
        "operation": "write",
        "endpoint_class": "fs.write",
    }


def to_intent_payload(hook_event: dict[str, Any], scenario: str) -> dict[str, Any]:
    tool_name = str(hook_event.get("tool_name", ""))
    mapped_tool = map_claude_tool_name(tool_name)
    tool_input = hook_event.get("tool_input")
    if not isinstance(tool_input, dict):
        raise RuntimeError("hook event tool_input must be an object")
    args = dict(tool_input)
    target = infer_target(mapped_tool, args)
    return {
        "schema_id": "gait.gate.intent_request",
        "schema_version": "1.0.0",
        "created_at": CREATED_AT,
        "producer_version": "0.0.0-example",
        "tool_name": mapped_tool,
        "args": args,
        "targets": [target],
        "arg_provenance": [{"arg_path": "$.path", "source": "user"}],
        "skill_provenance": {
            "skill_name": "safe_writer",
            "skill_version": "1.0.0",
            "source": "registry",
            "publisher": "acme",
            "digest": "a" * 64,
            "signature_key_id": "acme-dev-key",
        },
        "delegation": {
            "requester_identity": "claude-code-user",
            "scope_class": "write",
            "token_refs": [f"delegation-{FRAMEWORK}-{scenario}"],
            "chain": [
                {
                    "delegator_identity": "agent.lead",
                    "delegate_identity": "claude-code-user",
                    "scope_class": "write",
                    "token_ref": f"delegation-{FRAMEWORK}-{scenario}",
                }
            ],
        },
        "context": {
            "identity": "claude-code-user",
            "workspace": "/tmp/gait-claude-code",
            "risk_class": "high",
            "session_id": str(hook_event.get("session_id", "")),
            "auth_context": {
                "framework": FRAMEWORK,
                "hook_event_name": str(hook_event.get("hook_event_name", "")),
            },
            "credential_scopes": ["files.write", "network.egress"],
            "environment_fingerprint": f"{FRAMEWORK}:local:{scenario}",
        },
    }


def run_gate_eval(
    gait_bin: str, policy_path: Path, intent_path: Path, trace_path: Path
) -> tuple[int, dict[str, Any]]:
    command = [
        gait_bin,
        "gate",
        "eval",
        "--policy",
        str(policy_path),
        "--intent",
        str(intent_path),
        "--trace-out",
        str(trace_path),
        "--key-mode",
        "dev",
        "--json",
    ]
    completed = subprocess.run(  # nosec B603
        command,
        capture_output=True,
        text=True,
        check=False,
    )
    if not completed.stdout.strip():
        raise RuntimeError(
            f"gait gate eval produced no JSON output (stderr={completed.stderr.strip()})"
        )
    payload = json.loads(completed.stdout)
    if not isinstance(payload, dict):
        raise RuntimeError("gait gate eval returned non-object JSON")
    return completed.returncode, payload


def execute_wrapped_tool(intent_payload: dict[str, Any], output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        json.dumps(intent_payload["args"], indent=2) + "\n", encoding="utf-8"
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Claude Code wrapped tool-call quickstart"
    )
    parser.add_argument(
        "--scenario",
        choices=["allow", "block", "require_approval"],
        required=True,
    )
    args = parser.parse_args()

    repo_root = resolve_repo_root()
    gait_bin = resolve_gait_bin(repo_root)
    scenario = args.scenario
    policy_path = Path(__file__).with_name(f"policy_{scenario}.yaml")
    run_dir = repo_root / "gait-out" / "integrations" / FRAMEWORK
    run_dir.mkdir(parents=True, exist_ok=True)

    hook_path = run_dir / f"hook_{scenario}.json"
    intent_path = run_dir / f"intent_{scenario}.json"
    trace_path = run_dir / f"trace_{scenario}.json"
    executor_path = run_dir / f"executor_{scenario}.json"

    hook_event = build_claude_hook_event(scenario)
    intent_payload = to_intent_payload(hook_event, scenario)

    hook_path.write_text(json.dumps(hook_event, indent=2) + "\n", encoding="utf-8")
    intent_path.write_text(
        json.dumps(intent_payload, indent=2) + "\n", encoding="utf-8"
    )

    exit_code, gate_payload = run_gate_eval(
        gait_bin, policy_path, intent_path, trace_path
    )
    verdict = str(gate_payload.get("verdict", "unknown"))
    traced = str(gate_payload.get("trace_path", trace_path))

    if scenario == "allow":
        if exit_code != 0 or verdict != "allow":
            raise RuntimeError(
                f"expected allow flow, got exit_code={exit_code} verdict={verdict}"
            )
        execute_wrapped_tool(intent_payload, executor_path)
        print(f"framework={FRAMEWORK}")
        print("scenario=allow")
        print(f"verdict={verdict}")
        print("executed=true")
        print(f"trace_path={traced}")
        print(f"executor_output={executor_path}")
        return 0

    expected_verdict = "block"
    expected_exit_codes = (0, 3)
    if scenario == "require_approval":
        expected_verdict = "require_approval"
        expected_exit_codes = (0, 4)

    if verdict != expected_verdict or exit_code not in expected_exit_codes:
        raise RuntimeError(
            f"expected {expected_verdict} flow, got exit_code={exit_code} verdict={verdict}"
        )
    print(f"framework={FRAMEWORK}")
    print(f"scenario={scenario}")
    print(f"verdict={verdict}")
    print("executed=false")
    print(f"trace_path={traced}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as error:
        print(f"error={error}")
        raise SystemExit(1)
