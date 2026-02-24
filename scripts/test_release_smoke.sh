#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: $0 [path-to-gait-binary]" >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  if [[ "$1" = /* ]]; then
    BIN_PATH="$1"
  else
    BIN_PATH="$(pwd)/$1"
  fi
else
  BIN_PATH="$REPO_ROOT/gait"
  go build -o "$BIN_PATH" ./cmd/gait
fi

if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
cd "$WORK_DIR"

echo "==> demo -> verify"
"$BIN_PATH" demo > demo.txt
grep -q '^run_id=run_demo$' demo.txt
"$BIN_PATH" verify run_demo --json > verify.json

python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("verify.json").read_text(encoding="utf-8"))
required = {"ok", "run_id", "manifest_digest", "files_checked", "signature_status"}
missing = sorted(required.difference(payload.keys()))
if missing:
    raise SystemExit(f"verify json missing keys: {missing}")
if not payload.get("ok"):
    raise SystemExit("verify json expected ok=true")
PY

echo "==> gate eval"
"$BIN_PATH" gate eval \
  --policy "$REPO_ROOT/examples/policy/base_low_risk.yaml" \
  --intent "$REPO_ROOT/examples/policy/intents/intent_read.json" \
  --trace-out "$WORK_DIR/trace_smoke.json" \
  --json > gate.json

python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("gate.json").read_text(encoding="utf-8"))
required = {"ok", "verdict", "trace_id", "trace_path", "policy_digest", "intent_digest"}
missing = sorted(required.difference(payload.keys()))
if missing:
    raise SystemExit(f"gate json missing keys: {missing}")
if payload.get("verdict") != "allow":
    raise SystemExit(f"expected allow verdict, got={payload.get('verdict')}")
PY

echo "==> emergency stop preemption"
JOBS_ROOT="$WORK_DIR/jobs"
"$BIN_PATH" job submit --id job_release_stop --root "$JOBS_ROOT" --json > job_submit.json
STOP_START_NS="$(python3 - <<'PY'
import time
print(time.time_ns())
PY
)"
"$BIN_PATH" job stop --id job_release_stop --root "$JOBS_ROOT" --actor release-gate --json > job_stop.json
STOP_END_NS="$(python3 - <<'PY'
import time
print(time.time_ns())
PY
)"

cat > "$WORK_DIR/mcp_stop_call.json" <<'JSON'
{
  "name": "tool.delete",
  "args": {"path": "/tmp/release.txt"},
  "targets": [{"kind": "path", "value": "/tmp/release.txt", "operation": "delete", "destructive": true}],
  "arg_provenance": [{"arg_path": "$.path", "source": "user"}],
  "context": {
    "identity": "release",
    "workspace": "/repo/gait",
    "risk_class": "high",
    "session_id": "release-stop",
    "job_id": "job_release_stop",
    "phase": "apply"
  }
}
JSON

set +e
"$BIN_PATH" mcp proxy \
  --policy "$REPO_ROOT/examples/policy/base_low_risk.yaml" \
  --call "$WORK_DIR/mcp_stop_call.json" \
  --job-root "$JOBS_ROOT" \
  --json > mcp_stop.json
MCP_STOP_EXIT="$?"
set -e
if [[ "$MCP_STOP_EXIT" -ne 3 ]]; then
  echo "expected mcp proxy emergency stop preemption exit 3, got $MCP_STOP_EXIT" >&2
  exit 1
fi

"$BIN_PATH" job inspect --id job_release_stop --root "$JOBS_ROOT" --json > job_inspect.json

python3 - <<'PY' "$STOP_START_NS" "$STOP_END_NS"
import json
import sys
from pathlib import Path

stop_start_ns = int(sys.argv[1])
stop_end_ns = int(sys.argv[2])
stop_ack_ms = max(0, (stop_end_ns - stop_start_ns) // 1_000_000)

stop_payload = json.loads(Path("job_stop.json").read_text(encoding="utf-8"))
if not stop_payload.get("ok"):
    raise SystemExit(f"job stop failed: {stop_payload}")
job = stop_payload.get("job", {})
if job.get("status") != "emergency_stopped" or job.get("stop_reason") != "emergency_stopped":
    raise SystemExit(f"unexpected stop status payload: {stop_payload}")
if stop_ack_ms > 15000:
    raise SystemExit(f"stop_ack_ms over release threshold: {stop_ack_ms}")

mcp_payload = json.loads(Path("mcp_stop.json").read_text(encoding="utf-8"))
if not mcp_payload.get("ok"):
    raise SystemExit(f"mcp stop payload not ok: {mcp_payload}")
if mcp_payload.get("verdict") != "block":
    raise SystemExit(f"expected block verdict, got {mcp_payload.get('verdict')}")
if mcp_payload.get("executed"):
    raise SystemExit("expected executed=false for emergency-stop preemption")
reasons = mcp_payload.get("reason_codes", [])
if "emergency_stop_preempted" not in reasons:
    raise SystemExit(f"missing emergency_stop_preempted reason code: {reasons}")

inspect_payload = json.loads(Path("job_inspect.json").read_text(encoding="utf-8"))
events = inspect_payload.get("events", [])
ack_index = next((idx for idx, event in enumerate(events) if event.get("type") == "emergency_stop_acknowledged"), -1)
if ack_index < 0:
    raise SystemExit("missing emergency_stop_acknowledged event")
post_stop_events = events[ack_index + 1 :]
blocked = sum(1 for event in post_stop_events if event.get("type") == "dispatch_blocked")
post_stop_side_effects = sum(1 for event in post_stop_events if event.get("type") != "dispatch_blocked")
if blocked < 1:
    raise SystemExit("expected at least one dispatch_blocked event after stop")
if post_stop_side_effects != 0:
    raise SystemExit(f"post_stop_side_effects must be 0, got={post_stop_side_effects}")
PY

echo "==> regress init -> run"
"$BIN_PATH" regress init --from run_demo --json > regress_init.json
"$BIN_PATH" regress run --json > regress_run.json

python3 - <<'PY'
import json
from pathlib import Path

result = json.loads(Path("regress_run.json").read_text(encoding="utf-8"))
if not result.get("ok") or result.get("status") != "pass":
    raise SystemExit(f"unexpected regress result: {result}")
PY

echo "==> guard pack -> verify"
"$BIN_PATH" guard pack --run run_demo --out "$WORK_DIR/evidence_smoke.zip" --json > guard_pack.json
"$BIN_PATH" guard verify "$WORK_DIR/evidence_smoke.zip" --json > guard_verify.json

python3 - <<'PY'
import json
from pathlib import Path

pack = json.loads(Path("guard_pack.json").read_text(encoding="utf-8"))
verify = json.loads(Path("guard_verify.json").read_text(encoding="utf-8"))
if not pack.get("ok"):
    raise SystemExit("guard pack failed")
if not verify.get("ok"):
    raise SystemExit("guard verify failed")
PY

echo "==> render Homebrew formula from checksums"
cat > "$WORK_DIR/checksums.txt" <<'EOF'
1111111111111111111111111111111111111111111111111111111111111111  gait_v0.0.0_darwin_amd64.tar.gz
2222222222222222222222222222222222222222222222222222222222222222  gait_v0.0.0_darwin_arm64.tar.gz
EOF

bash "$REPO_ROOT/scripts/render_homebrew_formula.sh" \
  --repo "Clyra-AI/gait" \
  --version "v0.0.0" \
  --checksums "$WORK_DIR/checksums.txt" \
  --out "$WORK_DIR/gait.rb"

grep -q '^class Gait < Formula$' "$WORK_DIR/gait.rb"
grep -q '^  version "0.0.0"$' "$WORK_DIR/gait.rb"
grep -q 'gait_v0.0.0_darwin_amd64.tar.gz' "$WORK_DIR/gait.rb"
grep -q 'gait_v0.0.0_darwin_arm64.tar.gz' "$WORK_DIR/gait.rb"
grep -q '1111111111111111111111111111111111111111111111111111111111111111' "$WORK_DIR/gait.rb"
grep -q '2222222222222222222222222222222222222222222222222222222222222222' "$WORK_DIR/gait.rb"

echo "release smoke: pass"
