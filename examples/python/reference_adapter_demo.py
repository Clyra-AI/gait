from __future__ import annotations

import json
from pathlib import Path

from gait import (
    GateEnforcementError,
    IntentContext,
    IntentTarget,
    ToolAdapter,
    capture_intent,
)


def deterministic_executor(intent_path: str, payload: dict[str, str]) -> str:
    output_path = Path(intent_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    return str(output_path)


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    policy_path = (
        repo_root / "examples" / "python" / "policies" / "high_risk_tool_policy.yaml"
    )

    adapter = ToolAdapter(policy_path=policy_path)
    intent = capture_intent(
        tool_name="tool.write",
        args={"path": "/tmp/gait-example/result.json", "content": {"status": "ok"}},
        targets=[IntentTarget(kind="path", value="/tmp/gait-example/result.json")],
        context=IntentContext(
            identity="example-user", workspace="/tmp", risk_class="high"
        ),
    )

    try:
        outcome = adapter.execute(
            intent=intent,
            executor=lambda request: deterministic_executor(
                request.args["path"],
                request.args["content"],
            ),
        )
    except GateEnforcementError as error:
        print(f"gate blocked execution: {error}")
        return 1

    print(f"gate verdict={outcome.decision.verdict} executed={outcome.executed}")
    if outcome.executed:
        print(f"executor output={outcome.result}")

    runpack = adapter.capture_runpack(cwd=repo_root)
    fixture = adapter.create_regression_fixture(from_run=runpack.run_id, cwd=repo_root)
    print(f"runpack run_id={runpack.run_id} bundle={runpack.bundle_path}")
    print(f"regress fixture={fixture.fixture_name} config={fixture.config_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
