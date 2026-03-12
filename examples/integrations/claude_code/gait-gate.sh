#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

gait_bin="${GAIT_BIN:-gait}"
policy_path="${GAIT_CLAUDE_POLICY:-${repo_root}/examples/policy/base_high_risk.yaml}"
trace_dir="${GAIT_CLAUDE_TRACE_DIR:-${repo_root}/gait-out/integrations/claude_code/hooks}"
strict_mode="${GAIT_CLAUDE_STRICT:-0}"

emit_response() {
  local decision="$1"
  local reason="$2"
  local trace_path="$3"
  DECISION="$decision" REASON="$reason" TRACE_PATH="$trace_path" python3 - <<'PY'
import json
import os

inner = {
    "hookEventName": "PreToolUse",
    "permissionDecision": os.environ["DECISION"],
    "permissionDecisionReason": os.environ["REASON"],
}
trace_path = os.environ.get("TRACE_PATH", "").strip()
if trace_path:
    inner["tracePath"] = trace_path
print(json.dumps({"hookSpecificOutput": inner}))
PY
}

payload="$(cat)"
if [[ -z "${payload//[[:space:]]/}" ]]; then
  emit_response "allow" "gait hook: empty payload (fail-open)" ""
  exit 0
fi

tmp_call="$(mktemp)"
trap 'rm -f "$tmp_call"' EXIT
printf '%s\n' "$payload" > "$tmp_call"

mkdir -p "$trace_dir"
trace_path="$trace_dir/trace_$(date -u +%Y%m%dT%H%M%SZ)_$$.json"

set +e
proxy_output="$($gait_bin mcp proxy \
  --policy "$policy_path" \
  --call "$tmp_call" \
  --adapter claude_code \
  --trace-out "$trace_path" \
  --json 2>/dev/null)"
proxy_exit=$?
set -e

PROXY_EXIT="$proxy_exit" PROXY_OUTPUT="$proxy_output" STRICT_MODE="$strict_mode" TRACE_PATH="$trace_path" python3 - <<'PY'
import json
import os

proxy_exit = int(os.environ["PROXY_EXIT"])
proxy_output = os.environ.get("PROXY_OUTPUT", "")
strict_mode = os.environ.get("STRICT_MODE", "0").strip().lower() in {"1", "true", "yes", "on"}
trace_path = os.environ.get("TRACE_PATH", "").strip()

def emit(decision: str, reason: str) -> None:
    inner = {
        "hookEventName": "PreToolUse",
        "permissionDecision": decision,
        "permissionDecisionReason": reason,
    }
    if trace_path:
        inner["tracePath"] = trace_path
    print(json.dumps({"hookSpecificOutput": inner}))

try:
    decoded = json.loads(proxy_output) if proxy_output.strip() else {}
except json.JSONDecodeError:
    decoded = {}

if isinstance(decoded, dict):
    verdict = str(decoded.get("verdict", "")).strip().lower()
    reason_codes = decoded.get("reason_codes", [])
    if isinstance(reason_codes, list):
        reason_text = ", ".join(str(code) for code in reason_codes if str(code).strip())
    else:
        reason_text = ""

    if verdict == "allow" and proxy_exit == 0:
        emit("allow", "gait verdict=allow")
        raise SystemExit(0)
    if verdict == "block" and proxy_exit in (0, 3):
        detail = f" reason_codes={reason_text}" if reason_text else ""
        emit("deny", f"gait verdict=block{detail}")
        raise SystemExit(0)
    if verdict == "require_approval" and proxy_exit in (0, 4):
        detail = f" reason_codes={reason_text}" if reason_text else ""
        emit("ask", f"gait verdict=require_approval{detail}")
        raise SystemExit(0)

if strict_mode:
    emit("deny", f"gait hook error exit={proxy_exit} (strict mode)")
else:
    emit("allow", f"gait hook error exit={proxy_exit} (fail-open)")
PY
