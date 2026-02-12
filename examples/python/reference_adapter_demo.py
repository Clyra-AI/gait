from __future__ import annotations

import json
from pathlib import Path

from gait import (
    IntentArgProvenance,
    IntentContext,
    IntentTarget,
    ToolAdapter,
    gate_tool,
    run_session,
)


def deterministic_executor(target_path: str, payload: dict[str, str]) -> str:
    output_path = Path(target_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    return str(output_path)


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    policy_path = repo_root / "examples" / "python" / "policies" / "high_risk_tool_policy.yaml"

    adapter = ToolAdapter(policy_path=policy_path, gait_bin="gait")

    @gate_tool(
        adapter=adapter,
        tool_name="tool.write",
        context=IntentContext(
            identity="example-user",
            workspace="/tmp",
            risk_class="high",
        ),
        targets=lambda args, _: [IntentTarget(kind="path", value=str(args[0]), operation="write")],
        arg_provenance=[
            IntentArgProvenance(arg_path="$.payload", source="user"),
        ],
        cwd=repo_root,
    )
    def write_json(path: str, payload: dict[str, str]) -> str:
        return deterministic_executor(path, payload)

    with run_session(
        run_id="run_python_example",
        gait_bin="gait",
        cwd=repo_root,
        out_dir=repo_root / "gait-out",
    ) as session:
        result_path = write_json(
            "/tmp/gait-example/result.json",
            {"status": "ok"},
        )
        print(f"tool output={result_path}")

    if session.capture is None:
        print("run session failed to emit runpack")
        return 1

    fixture = adapter.create_regression_fixture(
        from_run=session.capture.run_id,
        cwd=repo_root,
    )
    print(f"runpack run_id={session.capture.run_id} bundle={session.capture.bundle_path}")
    print(f"ticket_footer={session.capture.ticket_footer}")
    print(f"regress fixture={fixture.fixture_name} config={fixture.config_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
