#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gait-v2-6-acceptance-XXXXXX")"
trap 'rm -rf "${WORK_DIR}"' EXIT

cd "$WORK_DIR"

echo "==> v2.6: guided demo output"
"$BIN_PATH" demo --json > "$WORK_DIR/demo.json"
python3 - <<'PY' "$WORK_DIR/demo.json"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("mode") == "standard", payload
assert payload.get("run_id") == "run_demo", payload
assert payload.get("metrics_opt_in"), payload
assert payload.get("next_commands"), payload
PY

echo "==> v2.6: verify guidance and signature note"
"$BIN_PATH" verify run_demo --json > "$WORK_DIR/verify.json"
python3 - <<'PY' "$WORK_DIR/verify.json"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("signature_status"), payload
assert payload.get("signature_note"), payload
assert payload.get("next_commands"), payload
PY

echo "==> v2.6: durable demo branch"
"$BIN_PATH" demo --durable --json > "$WORK_DIR/demo_durable.json"
python3 - <<'PY' "$WORK_DIR/demo_durable.json"
import json
import os
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("mode") == "durable", payload
assert payload.get("job_id") == "job_demo_durable", payload
assert payload.get("job_status") == "completed", payload
pack_path = payload.get("pack_path")
assert pack_path, payload
assert os.path.exists(pack_path), pack_path
PY

echo "==> v2.6: policy demo branch"
"$BIN_PATH" demo --policy --json > "$WORK_DIR/demo_policy.json"
python3 - <<'PY' "$WORK_DIR/demo_policy.json"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("mode") == "policy", payload
assert payload.get("policy_verdict") == "block", payload
assert payload.get("matched_rule") == "block-destructive-tool-delete", payload
reasons = payload.get("reason_codes") or []
assert "destructive_tool_blocked" in reasons, reasons
PY

echo "==> v2.6: activation tour"
"$BIN_PATH" tour --json > "$WORK_DIR/tour.json"
python3 - <<'PY' "$WORK_DIR/tour.json"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("mode") == "activation", payload
assert payload.get("run_id") == "run_demo", payload
assert payload.get("regress_status") == "pass", payload
assert payload.get("next_commands"), payload
PY

echo "==> v2.6: doctor summary mode"
"$BIN_PATH" doctor --workdir "$REPO_ROOT" --output-dir "$WORK_DIR/gait-out" --summary --json > "$WORK_DIR/doctor_summary.json"
python3 - <<'PY' "$WORK_DIR/doctor_summary.json"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("summary_mode") is True, payload
assert payload.get("summary"), payload
PY

echo "v2.6 acceptance: pass"
