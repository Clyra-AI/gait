#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

gait_bin="${GAIT_BIN:-gait}"
policy_path="${GAIT_CLAUDE_POLICY:-${repo_root}/examples/policy/base_high_risk.yaml}"
trace_dir="${GAIT_CLAUDE_TRACE_DIR:-${repo_root}/gait-out/integrations/claude_code/hooks}"
unsafe_fail_open="${GAIT_CLAUDE_UNSAFE_FAIL_OPEN:-0}"

is_truthy() {
  local raw="${1:-}"
  local normalized
  normalized="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
  case "$normalized" in
    1|true|yes|on)
      return 0
      ;;
  esac
  return 1
}

if is_truthy "$unsafe_fail_open"; then
  unsafe_fail_open=1
else
  unsafe_fail_open=0
fi

emit_response() {
  local decision="$1"
  local reason="$2"
  local trace_path="$3"
  DECISION="$decision" REASON="$reason" TRACE_PATH="$trace_path" python3 - <<'PY'
import json
import os
from pathlib import Path

payload = {
    "permissionDecision": os.environ["DECISION"],
    "permissionDecisionReason": os.environ["REASON"],
}
trace_path = os.environ.get("TRACE_PATH", "").strip()
if trace_path and Path(trace_path).exists():
    payload["tracePath"] = trace_path
print(json.dumps(payload))
PY
}

payload="$(cat)"
if [[ -z "${payload//[[:space:]]/}" ]]; then
  if [[ "$unsafe_fail_open" -eq 1 ]]; then
    emit_response "allow" "gait hook: empty payload (unsafe fail-open)" ""
  else
    emit_response "deny" "gait hook: empty payload (fail-closed)" ""
  fi
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

PROXY_EXIT="$proxy_exit" PROXY_OUTPUT="$proxy_output" UNSAFE_FAIL_OPEN="$unsafe_fail_open" TRACE_PATH="$trace_path" python3 - <<'PY'
import json
import os
from pathlib import Path

proxy_exit = int(os.environ["PROXY_EXIT"])
proxy_output = os.environ.get("PROXY_OUTPUT", "")
unsafe_fail_open = os.environ.get("UNSAFE_FAIL_OPEN", "0").strip().lower() in {"1", "true", "yes", "on"}
trace_path = os.environ.get("TRACE_PATH", "").strip()

def emit(decision: str, reason: str) -> None:
    payload = {
        "permissionDecision": decision,
        "permissionDecisionReason": reason,
    }
    if trace_path and Path(trace_path).exists():
        payload["tracePath"] = trace_path
    print(json.dumps(payload))

failure_reason = f"gait hook proxy failure exit={proxy_exit}"
decoded_valid = True
try:
    decoded = json.loads(proxy_output) if proxy_output.strip() else {}
except json.JSONDecodeError:
    decoded = {}
    decoded_valid = False
    failure_reason = f"gait hook invalid proxy output exit={proxy_exit}"

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
    if decoded_valid and proxy_output.strip() and proxy_exit == 0:
        failure_reason = f"gait hook undecidable verdict={verdict or 'missing'} exit={proxy_exit}"

if unsafe_fail_open:
    emit("allow", f"{failure_reason} (unsafe fail-open)")
else:
    emit("deny", f"{failure_reason} (fail-closed)")
PY
