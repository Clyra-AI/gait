#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CALLER_PWD="$(pwd)"

usage() {
  cat <<'EOF'
Validate Claude Code hook decision contract.

Usage:
  test_claude_code_hook_contract.sh [path-to-gait-binary]
EOF
}

if [[ $# -gt 1 ]]; then
  usage >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  if [[ "$1" = /* ]]; then
    BIN_PATH="$1"
  else
    BIN_PATH="${CALLER_PWD}/$1"
  fi
else
  BIN_PATH="${REPO_ROOT}/gait"
  (
    cd "${REPO_ROOT}"
    go build -o "${BIN_PATH}" ./cmd/gait
  )
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "binary is not executable: ${BIN_PATH}" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

TRACE_DIR="${WORK_DIR}/traces"
mkdir -p "${TRACE_DIR}"

BASE_PAYLOAD='{"session_id":"sess-claude-hook","tool_name":"Write","tool_input":{"path":"/tmp/gait-claude-hook.json","content":"ok"},"hook_event_name":"PreToolUse"}'

run_hook() {
  local policy_path="$1"
  local gait_bin="$2"
  local payload="${3-}"
  shift 3

  if [[ -n "$payload" ]]; then
    printf '%s\n' "$payload" | env \
      GAIT_BIN="$gait_bin" \
      GAIT_CLAUDE_POLICY="$policy_path" \
      GAIT_CLAUDE_TRACE_DIR="$TRACE_DIR" \
      "$@" \
      "${REPO_ROOT}/examples/integrations/claude_code/gait-gate.sh"
    return
  fi

  printf '' | env \
    GAIT_BIN="$gait_bin" \
    GAIT_CLAUDE_POLICY="$policy_path" \
    GAIT_CLAUDE_TRACE_DIR="$TRACE_DIR" \
    "$@" \
    "${REPO_ROOT}/examples/integrations/claude_code/gait-gate.sh"
}

assert_response() {
  local output="$1"
  local expected_decision="$2"
  local reason_contains="$3"
  local trace_expectation="$4"
  OUTPUT="$output" EXPECTED_DECISION="$expected_decision" REASON_CONTAINS="$reason_contains" TRACE_EXPECTATION="$trace_expectation" python3 - <<'PY'
import json
import os
from pathlib import Path

payload = json.loads(os.environ["OUTPUT"])
expected_decision = os.environ["EXPECTED_DECISION"]
reason_contains = os.environ["REASON_CONTAINS"]
trace_expectation = os.environ["TRACE_EXPECTATION"]

decision = str(payload.get("permissionDecision", ""))
if decision != expected_decision:
    raise SystemExit(
        f"permissionDecision mismatch: expected={expected_decision} got={decision} payload={payload}"
    )

reason = str(payload.get("permissionDecisionReason", ""))
if reason_contains and reason_contains not in reason:
    raise SystemExit(
        f"permissionDecisionReason missing substring {reason_contains!r}: {reason}"
    )

trace_path = str(payload.get("tracePath", ""))
if trace_expectation == "required":
    if not trace_path:
        raise SystemExit(f"tracePath missing: {payload}")
    if not Path(trace_path).exists():
        raise SystemExit(f"tracePath does not exist: {trace_path}")
elif trace_expectation == "absent":
    if trace_path:
        raise SystemExit(f"tracePath expected absent, got {trace_path}")
else:
    if trace_path and not Path(trace_path).exists():
        raise SystemExit(f"tracePath does not exist: {trace_path}")
PY
}

echo "==> nominal verdict mapping"
for scenario in allow block require_approval; do
  output="$(
    run_hook \
      "${REPO_ROOT}/examples/integrations/claude_code/policy_${scenario}.yaml" \
      "${BIN_PATH}" \
      "${BASE_PAYLOAD}"
  )"
  printf '%s\n' "$output"
  expected_decision="allow"
  if [[ "$scenario" == "block" ]]; then
    expected_decision="deny"
  elif [[ "$scenario" == "require_approval" ]]; then
    expected_decision="ask"
  fi
  assert_response "$output" "$expected_decision" "gait verdict=" "required"
done

echo "==> broken gait binary fails closed by default"
output="$(
  run_hook \
    "${REPO_ROOT}/examples/integrations/claude_code/policy_allow.yaml" \
    "/definitely/missing/path" \
    "${BASE_PAYLOAD}"
)"
printf '%s\n' "$output"
assert_response "$output" "deny" "fail-closed" "absent"

echo "==> empty payload fails closed by default"
output="$(
  run_hook \
    "${REPO_ROOT}/examples/integrations/claude_code/policy_allow.yaml" \
    "${BIN_PATH}" \
    ""
  )"
printf '%s\n' "$output"
assert_response "$output" "deny" "empty payload" "absent"

echo "==> malformed payload fails closed by default"
output="$(
  run_hook \
    "${REPO_ROOT}/examples/integrations/claude_code/policy_allow.yaml" \
    "${BIN_PATH}" \
    '{"session_id":"sess-bad","tool_name":"Write"'
)"
printf '%s\n' "$output"
assert_response "$output" "deny" "fail-closed" "optional"

echo "==> invalid proxy output fails closed by default"
FAKE_GAIT="${WORK_DIR}/fake-gait"
cat > "${FAKE_GAIT}" <<'EOF'
#!/usr/bin/env bash
printf 'not-json\n'
exit 0
EOF
chmod 0755 "${FAKE_GAIT}"
output="$(
  run_hook \
    "${REPO_ROOT}/examples/integrations/claude_code/policy_allow.yaml" \
    "${FAKE_GAIT}" \
    "${BASE_PAYLOAD}"
)"
printf '%s\n' "$output"
assert_response "$output" "deny" "invalid proxy output" "absent"

echo "==> explicit unsafe fail-open override remains opt-in"
output="$(
  run_hook \
    "${REPO_ROOT}/examples/integrations/claude_code/policy_allow.yaml" \
    "/definitely/missing/path" \
    "${BASE_PAYLOAD}" \
    GAIT_CLAUDE_UNSAFE_FAIL_OPEN=1
)"
printf '%s\n' "$output"
assert_response "$output" "allow" "unsafe fail-open" "absent"

echo "claude code hook contract: pass"
