from __future__ import annotations

import sys
from dataclasses import dataclass, replace
from pathlib import Path
from types import ModuleType
from typing import Any


def create_fake_gait_script(path: Path) -> None:
    script = """#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path


def arg_value(args, name, default=None):
    if name not in args:
        return default
    index = args.index(name)
    if index + 1 >= len(args):
        return default
    return args[index + 1]


def run_gate_eval(args):
    intent_path = arg_value(args, "--intent")
    if intent_path is None:
        print(json.dumps({"ok": False, "error": "missing intent path"}))
        return 6

    payload = json.loads(Path(intent_path).read_text(encoding="utf-8"))
    tool_name = payload.get("tool_name", "")
    verdict = "allow"
    exit_code = 0
    reason_codes = ["default_allow"]

    if tool_name == "tool.block":
        verdict = "block"
        exit_code = 3
        reason_codes = ["blocked_tool"]
    elif tool_name == "tool.approval":
        verdict = "require_approval"
        exit_code = 4
        reason_codes = ["approval_required"]
    elif tool_name == "tool.dry":
        verdict = "dry_run"
        reason_codes = ["dry_run_selected"]

    default_trace_path = str(Path(intent_path).with_name("trace_fake.json"))
    trace_path = arg_value(args, "--trace-out", default_trace_path)
    trace_payload = {
        "schema_id": "gait.gate.trace",
        "schema_version": "1.0.0",
        "created_at": "2026-02-05T00:00:00Z",
        "producer_version": "0.0.0-dev",
        "trace_id": "trace_fake",
        "tool_name": tool_name,
        "args_digest": "1" * 64,
        "intent_digest": "2" * 64,
        "policy_digest": "3" * 64,
        "verdict": verdict,
    }
    Path(trace_path).write_text(json.dumps(trace_payload, indent=2) + "\\n", encoding="utf-8")

    print(
        json.dumps(
            {
                "ok": True,
                "verdict": verdict,
                "reason_codes": reason_codes,
                "violations": [],
                "trace_id": "trace_fake",
                "trace_path": trace_path,
                "policy_digest": "3" * 64,
                "intent_digest": "2" * 64,
            }
        )
    )
    return exit_code


def run_regress_init():
    print(
        json.dumps(
            {
                "ok": True,
                "run_id": "run_demo",
                "fixture_name": "run_demo",
                "fixture_dir": "fixtures/run_demo",
                "runpack_path": "fixtures/run_demo/runpack.zip",
                "config_path": "gait.yaml",
                "next_commands": ["gait regress run --json"],
            }
        )
    )
    return 0


def run_demo(args):
    if "--json" in args:
        print(
            json.dumps(
                {
                    "ok": True,
                    "mode": "standard",
                    "run_id": "run_demo",
                    "bundle": "./gait-out/runpack_run_demo.zip",
                    "ticket_footer": 'GAIT run_id=run_demo manifest=sha256:abc verify="gait verify run_demo"',
                    "verify": "ok",
                }
            )
        )
        return 0

    print("demo_run=run_demo")
    print("artifact=./gait-out/runpack_run_demo.zip")
    print('receipt=GAIT run_id=run_demo manifest=sha256:abc verify="gait verify run_demo"')
    print("verification_status=ok")
    return 0


def run_record(args):
    input_path = arg_value(args, "--input")
    out_dir = arg_value(args, "--out-dir", "gait-out")
    capture_mode = arg_value(args, "--capture-mode", "reference")
    if input_path is None:
        print(json.dumps({"ok": False, "error": "missing input path"}))
        return 6

    payload = json.loads(Path(input_path).read_text(encoding="utf-8"))
    run_id = str(payload.get("run", {}).get("run_id", "run_demo"))

    capture_out = os.environ.get("FAKE_GAIT_RECORD_CAPTURE")
    if capture_out:
        Path(capture_out).write_text(json.dumps(payload, indent=2) + "\\n", encoding="utf-8")

    out_root = Path(out_dir)
    out_root.mkdir(parents=True, exist_ok=True)
    bundle_path = out_root / f"runpack_{run_id}.zip"
    bundle_path.write_text(f"fake zip {run_id} {capture_mode}\\n", encoding="utf-8")

    print(
        json.dumps(
            {
                "ok": True,
                "run_id": run_id,
                "bundle": str(bundle_path),
                "manifest_digest": "4" * 64,
                "ticket_footer": f'GAIT run_id={run_id} manifest=sha256:{"4" * 64} verify="gait verify {run_id}"',
            }
        )
    )
    return 0


def main():
    args = sys.argv[1:]
    if args[:2] == ["gate", "eval"]:
        return run_gate_eval(args[2:])
    if args[:2] == ["regress", "init"]:
        return run_regress_init()
    if args[:2] == ["run", "record"]:
        return run_record(args[2:])
    if args[:1] == ["demo"]:
        return run_demo(args[1:])
    print(json.dumps({"ok": False, "error": "unsupported command", "argv": args}))
    return 6


if __name__ == "__main__":
    raise SystemExit(main())
"""
    path.write_text(script, encoding="utf-8")


def install_fake_langchain_modules() -> type[Any]:
    langchain = ModuleType("langchain")
    langchain.__path__ = []  # type: ignore[attr-defined]
    langchain_agents = ModuleType("langchain.agents")
    langchain_agents.__path__ = []  # type: ignore[attr-defined]
    langchain_agents_middleware = ModuleType("langchain.agents.middleware")
    langchain_tools = ModuleType("langchain.tools")
    langchain_tools.__path__ = []  # type: ignore[attr-defined]
    langchain_tools_tool_node = ModuleType("langchain.tools.tool_node")
    langchain_core = ModuleType("langchain_core")
    langchain_core.__path__ = []  # type: ignore[attr-defined]
    langchain_core_callbacks = ModuleType("langchain_core.callbacks")
    langchain_core_callbacks.__path__ = []  # type: ignore[attr-defined]
    langchain_core_callbacks_base = ModuleType("langchain_core.callbacks.base")

    class AgentMiddleware:
        pass

    @dataclass(slots=True)
    class ToolCallRequest:
        tool_call: dict[str, Any]
        tool: Any | None
        state: Any
        runtime: Any

        def override(self, **overrides: Any) -> "ToolCallRequest":
            return replace(self, **overrides)

    class BaseCallbackHandler:
        pass

    langchain_agents_middleware.AgentMiddleware = AgentMiddleware
    langchain_tools_tool_node.ToolCallRequest = ToolCallRequest
    langchain_core_callbacks_base.BaseCallbackHandler = BaseCallbackHandler

    langchain.agents = langchain_agents  # type: ignore[attr-defined]
    langchain_agents.middleware = langchain_agents_middleware  # type: ignore[attr-defined]
    langchain.tools = langchain_tools  # type: ignore[attr-defined]
    langchain_tools.tool_node = langchain_tools_tool_node  # type: ignore[attr-defined]
    langchain_core.callbacks = langchain_core_callbacks  # type: ignore[attr-defined]
    langchain_core_callbacks.base = langchain_core_callbacks_base  # type: ignore[attr-defined]

    modules = {
        "langchain": langchain,
        "langchain.agents": langchain_agents,
        "langchain.agents.middleware": langchain_agents_middleware,
        "langchain.tools": langchain_tools,
        "langchain.tools.tool_node": langchain_tools_tool_node,
        "langchain_core": langchain_core,
        "langchain_core.callbacks": langchain_core_callbacks,
        "langchain_core.callbacks.base": langchain_core_callbacks_base,
    }
    sys.modules.update(modules)
    return ToolCallRequest
