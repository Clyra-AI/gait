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

cp "$REPO_ROOT/examples/policy-test/intent.json" "$WORK_DIR/intent.json"
cp "$REPO_ROOT/examples/policy-test/allow.yaml" "$WORK_DIR/allow.yaml"
cp "$REPO_ROOT/examples/policy-test/block.yaml" "$WORK_DIR/block.yaml"
cp "$REPO_ROOT/examples/policy-test/require_approval.yaml" "$WORK_DIR/require_approval.yaml"

cd "$WORK_DIR"

DEMO_OUT="$("$BIN_PATH" demo)"
echo "$DEMO_OUT"
[[ "$DEMO_OUT" == *"run_id=run_demo"* ]]
[[ "$DEMO_OUT" == *"ticket_footer=GAIT run_id=run_demo"* ]]
[[ "$DEMO_OUT" == *"verify=ok"* ]]

VERIFY_OUT="$("$BIN_PATH" verify run_demo)"
echo "$VERIFY_OUT"
[[ "$VERIFY_OUT" == *"verify ok"* ]]

REPLAY_A="$("$BIN_PATH" run replay --json run_demo)"
REPLAY_B="$("$BIN_PATH" run replay --json run_demo)"
python3 - "$REPLAY_A" "$REPLAY_B" <<'PY'
import json
import sys

first = json.loads(sys.argv[1])
second = json.loads(sys.argv[2])
if first != second:
    raise SystemExit("stub replay is not deterministic")
if first.get("mode") != "stub":
    raise SystemExit("expected replay mode stub")
PY

"$BIN_PATH" regress init --from run_demo --json > regress_init.json
python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("regress_init.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("regress init returned ok=false")
if payload.get("run_id") != "run_demo":
    raise SystemExit("unexpected run_id from regress init")
PY

"$BIN_PATH" regress run --json > regress_run.json
python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("regress_run.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("regress run returned ok=false")
if payload.get("status") != "pass":
    raise SystemExit(f"unexpected regress status: {payload.get('status')}")
PY

"$BIN_PATH" policy test allow.yaml intent.json --json > allow.json
ALLOW_CODE=$?
if [[ $ALLOW_CODE -ne 0 ]]; then
  echo "unexpected allow exit code: $ALLOW_CODE" >&2
  exit 1
fi

set +e
"$BIN_PATH" policy test block.yaml intent.json --json > block.json
BLOCK_CODE=$?
"$BIN_PATH" policy test require_approval.yaml intent.json --json > require_approval.json
APPROVAL_CODE=$?
set -e

if [[ $BLOCK_CODE -ne 3 ]]; then
  echo "unexpected block exit code: $BLOCK_CODE" >&2
  exit 1
fi
if [[ $APPROVAL_CODE -ne 4 ]]; then
  echo "unexpected require_approval exit code: $APPROVAL_CODE" >&2
  exit 1
fi

python3 - <<'PY'
import json
from pathlib import Path

allow = json.loads(Path("allow.json").read_text(encoding="utf-8"))
block = json.loads(Path("block.json").read_text(encoding="utf-8"))
approval = json.loads(Path("require_approval.json").read_text(encoding="utf-8"))

if allow.get("verdict") != "allow":
    raise SystemExit("allow policy verdict mismatch")
if block.get("verdict") != "block":
    raise SystemExit("block policy verdict mismatch")
if approval.get("verdict") != "require_approval":
    raise SystemExit("require_approval policy verdict mismatch")
PY

echo "v1 acceptance checks passed"
