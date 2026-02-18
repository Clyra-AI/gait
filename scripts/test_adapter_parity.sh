#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go build -o ./gait ./cmd/gait
export PATH="$repo_root:$PATH"

frameworks=(openai_agents langchain autogen openclaw autogpt gastown claude_code)

echo "==> adapter parity smoke"
for framework in "${frameworks[@]}"; do
  for scenario in allow block; do
    echo "--> $framework $scenario"
    adapter_output="$(python3 "examples/integrations/${framework}/quickstart.py" --scenario "$scenario")"
    printf '%s\n' "$adapter_output"
    ADAPTER_FRAMEWORK="$framework" ADAPTER_SCENARIO="$scenario" ADAPTER_OUTPUT="$adapter_output" python3 - <<'PY'
import json
import os
from pathlib import Path

framework = os.environ["ADAPTER_FRAMEWORK"]
scenario = os.environ["ADAPTER_SCENARIO"]
output = os.environ["ADAPTER_OUTPUT"]

parsed: dict[str, str] = {}
for raw_line in output.splitlines():
    line = raw_line.strip()
    if not line or "=" not in line:
        continue
    key, value = line.split("=", 1)
    parsed[key.strip()] = value.strip()

for field in ("framework", "scenario", "verdict", "executed", "trace_path"):
    if field not in parsed:
        raise SystemExit(f"missing field {field} in adapter output: {output}")

if parsed["framework"] != framework:
    raise SystemExit(
        f"framework mismatch: expected={framework} got={parsed['framework']}"
    )
if parsed["scenario"] != scenario:
    raise SystemExit(f"scenario mismatch: expected={scenario} got={parsed['scenario']}")

trace_path = Path(parsed["trace_path"])
if not trace_path.exists():
    raise SystemExit(f"trace_path missing: {trace_path}")
expected_trace_suffix = f"gait-out/integrations/{framework}/trace_{scenario}.json"
if expected_trace_suffix not in str(trace_path).replace("\\", "/"):
    raise SystemExit(
        f"trace_path format mismatch: expected suffix {expected_trace_suffix} got {trace_path}"
    )

trace_payload = json.loads(trace_path.read_text(encoding="utf-8"))
if trace_payload.get("tool_name") != "tool.write":
    raise SystemExit(f"unexpected tool_name in trace: {trace_payload.get('tool_name')}")
skill = trace_payload.get("skill_provenance")
if not isinstance(skill, dict):
    raise SystemExit("trace skill_provenance missing")
if skill.get("publisher") != "acme" or skill.get("source") != "registry":
    raise SystemExit(f"unexpected skill provenance in trace: {skill}")

intent_path = trace_path.parent / f"intent_{scenario}.json"
if not intent_path.exists():
    raise SystemExit(f"intent fixture missing: {intent_path}")
intent_payload = json.loads(intent_path.read_text(encoding="utf-8"))
targets = intent_payload.get("targets")
if not isinstance(targets, list) or not targets:
    raise SystemExit(f"intent targets missing: {intent_path}")
endpoint_class = targets[0].get("endpoint_class")
if endpoint_class != "fs.write":
    raise SystemExit(f"endpoint_class mismatch: expected fs.write got {endpoint_class}")
context = intent_payload.get("context")
if not isinstance(context, dict):
    raise SystemExit(f"intent context missing: {intent_path}")
session_id = context.get("session_id")
if not isinstance(session_id, str) or not session_id:
    raise SystemExit(f"session_id missing in context: {intent_path}")
auth_context = context.get("auth_context")
if not isinstance(auth_context, dict) or not auth_context:
    raise SystemExit(f"auth_context missing in context: {intent_path}")
credential_scopes = context.get("credential_scopes")
if not isinstance(credential_scopes, list) or not credential_scopes:
    raise SystemExit(f"credential_scopes missing in context: {intent_path}")
env_fp = context.get("environment_fingerprint")
if not isinstance(env_fp, str) or not env_fp:
    raise SystemExit(f"environment_fingerprint missing in context: {intent_path}")

delegation = intent_payload.get("delegation")
if not isinstance(delegation, dict):
    raise SystemExit(f"delegation missing from intent: {intent_path}")
if not delegation.get("requester_identity"):
    raise SystemExit(f"delegation requester_identity missing: {intent_path}")
chain = delegation.get("chain")
if not isinstance(chain, list) or not chain:
    raise SystemExit(f"delegation chain missing: {intent_path}")
first_link = chain[0]
if not isinstance(first_link, dict) or not first_link.get("delegator_identity") or not first_link.get("delegate_identity"):
    raise SystemExit(f"delegation chain link invalid: {intent_path}")

if scenario == "allow":
    if parsed["verdict"] != "allow":
        raise SystemExit(f"allow scenario verdict mismatch: {parsed['verdict']}")
    if parsed["executed"].lower() != "true":
        raise SystemExit(f"allow scenario executed mismatch: {parsed['executed']}")
    executor_output = parsed.get("executor_output")
    if not executor_output:
        raise SystemExit("allow scenario missing executor_output field")
    executor_path = Path(executor_output)
    if not executor_path.exists():
        raise SystemExit(f"allow scenario executor output missing: {executor_path}")
else:
    if parsed["verdict"] == "allow":
        raise SystemExit("block scenario unexpectedly returned allow verdict")
    if parsed["executed"].lower() != "false":
        raise SystemExit(f"block scenario executed mismatch: {parsed['executed']}")
    executor_output = parsed.get("executor_output", "")
    if executor_output and Path(executor_output).exists():
        raise SystemExit(
            f"block scenario must not materialize executor output: {executor_output}"
        )
PY
  done
done

echo "adapter parity: pass"
