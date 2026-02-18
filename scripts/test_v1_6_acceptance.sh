#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <path-to-gait-binary>" >&2
  exit 2
fi

if [[ "$1" = /* ]]; then
  BIN_PATH="$1"
else
  BIN_PATH="$(pwd)/$1"
fi
if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

cp "$REPO_ROOT/examples/policy-test/allow.yaml" "$WORK_DIR/allow.yaml"
cp "$REPO_ROOT/examples/policy-test/block.yaml" "$WORK_DIR/block.yaml"
cp "$REPO_ROOT/core/schema/testdata/gate_intent_request_valid.json" "$WORK_DIR/intent_valid.json"

echo "==> one-path onboarding"
pushd "$WORK_DIR" >/dev/null
QUICKSTART_OUT="$(GAIT_BIN="$BIN_PATH" bash "$REPO_ROOT/scripts/quickstart.sh")"
printf '%s\n' "$QUICKSTART_OUT"
QUICKSTART_OUTPUT="$QUICKSTART_OUT" python3 - <<'PY'
import os
from pathlib import Path

raw = os.environ["QUICKSTART_OUTPUT"]
parsed: dict[str, str] = {}
for line in raw.splitlines():
    if "=" not in line:
        continue
    key, value = line.split("=", 1)
    parsed[key.strip()] = value.strip()

required = ["quickstart_status", "run_id", "runpack", "verify_json", "regress_init_json"]
missing = [field for field in required if field not in parsed]
if missing:
    raise SystemExit(f"quickstart missing required output fields: {missing}")
if parsed["quickstart_status"] != "pass":
    raise SystemExit(f"quickstart_status mismatch: {parsed['quickstart_status']}")
for field in ("runpack", "verify_json", "regress_init_json"):
    if not Path(parsed[field]).exists():
        raise SystemExit(f"quickstart artifact missing: {field}={parsed[field]}")
PY
popd >/dev/null

echo "==> ticket footer contract"
pushd "$WORK_DIR" >/dev/null
DEMO_OUT="$("$BIN_PATH" demo)"
python3 - "$DEMO_OUT" <<'PY'
import re
import sys

output = sys.argv[1]
ticket_footer = ""
for line in output.splitlines():
    if line.startswith("ticket_footer="):
        ticket_footer = line.removeprefix("ticket_footer=").strip()
        break
if not ticket_footer:
    raise SystemExit("ticket_footer line missing")

pattern = re.compile(
    r'^GAIT run_id=([A-Za-z0-9_-]+) manifest=sha256:([a-f0-9]{64}) verify="gait verify ([A-Za-z0-9_-]+)"$'
)
match = pattern.match(ticket_footer)
if match is None:
    raise SystemExit(f"ticket_footer format mismatch: {ticket_footer}")
if match.group(1) != match.group(3):
    raise SystemExit("ticket_footer run_id mismatch")
PY
popd >/dev/null

echo "==> incident to regression path"
pushd "$WORK_DIR" >/dev/null
"$BIN_PATH" regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml > regress_bootstrap.json
python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("regress_bootstrap.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("bootstrap returned ok=false")
if payload.get("status") != "pass":
    raise SystemExit(f"unexpected regress status: {payload.get('status')}")
for key in ("run_id", "fixture_dir", "output", "artifact_paths"):
    if key not in payload:
        raise SystemExit(f"missing bootstrap key: {key}")
PY
[[ -f "$WORK_DIR/gait-out/junit.xml" ]]
[[ -f "$WORK_DIR/regress_result.json" ]]
popd >/dev/null

echo "==> sidecar fail-closed behavior"
pushd "$WORK_DIR" >/dev/null
python3 "$REPO_ROOT/examples/sidecar/gate_sidecar.py" \
  --gait-bin "$BIN_PATH" \
  --policy "$WORK_DIR/allow.yaml" \
  --intent-file "$WORK_DIR/intent_valid.json" \
  --trace-out "$WORK_DIR/gait-out/trace_sidecar_allow.json" \
  > sidecar_allow.json
ALLOW_CODE=$?
set +e
python3 "$REPO_ROOT/examples/sidecar/gate_sidecar.py" \
  --gait-bin "$BIN_PATH" \
  --policy "$WORK_DIR/block.yaml" \
  --intent-file "$WORK_DIR/intent_valid.json" \
  --trace-out "$WORK_DIR/gait-out/trace_sidecar_block.json" \
  > sidecar_block.json
BLOCK_CODE=$?
printf '%s\n' '{}' > intent_invalid.json
python3 "$REPO_ROOT/examples/sidecar/gate_sidecar.py" \
  --gait-bin "$BIN_PATH" \
  --policy "$WORK_DIR/allow.yaml" \
  --intent-file "$WORK_DIR/intent_invalid.json" \
  > sidecar_invalid.json
INVALID_CODE=$?
set -e
if [[ $ALLOW_CODE -ne 0 ]]; then
  echo "unexpected allow sidecar exit code: $ALLOW_CODE" >&2
  exit 1
fi
if [[ $BLOCK_CODE -ne 3 ]]; then
  echo "unexpected block sidecar exit code: $BLOCK_CODE" >&2
  exit 1
fi
if [[ $INVALID_CODE -ne 2 ]]; then
  echo "unexpected invalid sidecar exit code: $INVALID_CODE" >&2
  exit 1
fi
python3 - <<'PY'
import json
from pathlib import Path

allow = json.loads(Path("sidecar_allow.json").read_text(encoding="utf-8"))
block = json.loads(Path("sidecar_block.json").read_text(encoding="utf-8"))
invalid = json.loads(Path("sidecar_invalid.json").read_text(encoding="utf-8"))

if allow.get("gate_result", {}).get("verdict") != "allow":
    raise SystemExit("sidecar allow verdict mismatch")
if block.get("gate_result", {}).get("verdict") != "block":
    raise SystemExit("sidecar block verdict mismatch")
if allow.get("ok") is not True:
    raise SystemExit("sidecar allow must be ok=true")
if block.get("ok") is not True:
    raise SystemExit("sidecar block command must be ok=true")
if block.get("gate_result", {}).get("verdict") == "allow":
    raise SystemExit("sidecar block flow must never bypass to allow")
if invalid.get("ok") is not False:
    raise SystemExit("invalid sidecar payload must be fail-closed")
PY
popd >/dev/null

echo "==> local signal report generation"
pushd "$WORK_DIR" >/dev/null
"$BIN_PATH" scout signal \
  --runs "$WORK_DIR/gait-out/runpack_run_demo.zip,$WORK_DIR/fixtures/run_demo/runpack.zip" \
  --regress "$WORK_DIR/regress_result.json" \
  --json \
  > signal.json
python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("signal.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("signal command returned ok=false")
report = payload.get("report")
if not isinstance(report, dict):
    raise SystemExit("signal report payload missing")
if report.get("schema_id") != "gait.scout.signal_report":
    raise SystemExit("unexpected signal report schema_id")
if report.get("schema_version") != "1.0.0":
    raise SystemExit("unexpected signal report schema_version")
if report.get("family_count", 0) < 1:
    raise SystemExit("expected at least one signal family")
if len(report.get("top_issues", [])) < 1:
    raise SystemExit("expected at least one top issue")
PY
[[ -f "$WORK_DIR/gait-out/scout_signal_report.json" ]]
popd >/dev/null

echo "v1.6 acceptance checks passed"
