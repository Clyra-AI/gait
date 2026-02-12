#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

go build -o ./gait ./cmd/gait
BIN_PATH="$REPO_ROOT/gait"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

mkdir -p "$WORK_DIR/.github/workflows"
cp "$REPO_ROOT/.github/workflows/adoption-regress-template.yml" "$WORK_DIR/.github/workflows/adoption-regress-template.yml"
mkdir -p "$WORK_DIR/.github/actions/gait-regress"
cp "$REPO_ROOT/.github/actions/gait-regress/action.yml" "$WORK_DIR/.github/actions/gait-regress/action.yml"

echo "==> validate reusable workflow contract fields"
python3 - "$WORK_DIR/.github/workflows/adoption-regress-template.yml" <<'PY'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required_fragments = [
    "workflow_call:",
    "inputs:",
    "outputs:",
    "regress_status:",
    "regress_exit_code:",
    "top_failure_reason:",
    "next_command:",
    "artifact_root:",
    "Publish regress summary",
    "Enforce stable regress exit code contract",
]
missing = [fragment for fragment in required_fragments if fragment not in text]
if missing:
    raise SystemExit(f"adoption-regress-template missing required fragments: {missing}")
PY

echo "==> validate gait-regress action v2 contract fields"
python3 - "$WORK_DIR/.github/actions/gait-regress/action.yml" <<'PY'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required_fragments = [
    "version:",
    "workdir:",
    "command:",
    "args:",
    "upload_artifacts:",
    "artifact_name:",
    "exit_code:",
    "summary_path:",
    "artifact_path:",
    "Download and verify gait binary",
    "Run gait command",
]
missing = [fragment for fragment in required_fragments if fragment not in text]
if missing:
    raise SystemExit(f"gait-regress action missing required fragments: {missing}")
PY

echo "==> simulate downstream regress path with deterministic fixture restore"
ARTIFACT_ROOT="$WORK_DIR/gait-out/adoption_regress"
mkdir -p "$ARTIFACT_ROOT"
(
  cd "$WORK_DIR"
  "$BIN_PATH" demo --json > demo.json
  "$BIN_PATH" regress init --from run_demo --json > "$ARTIFACT_ROOT/regress_init_result.json"
  "$BIN_PATH" regress run --json --junit "$ARTIFACT_ROOT/junit.xml" > "$ARTIFACT_ROOT/regress_result.json"
)

python3 - "$ARTIFACT_ROOT/regress_result.json" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("status") != "pass":
    raise SystemExit(f"expected regress status=pass, got {payload.get('status')}")
if payload.get("ok") is not True:
    raise SystemExit("expected regress ok=true")
PY

echo "==> endpoint policy + skill provenance assumptions"
"$BIN_PATH" policy test examples/policy/endpoint/allow_safe_endpoints.yaml examples/policy/endpoint/fixtures/intent_allow.json --json >/dev/null
set +e
"$BIN_PATH" policy test examples/policy/endpoint/block_denied_endpoints.yaml examples/policy/endpoint/fixtures/intent_block.json --json >/dev/null
block_status=$?
"$BIN_PATH" policy test examples/policy/endpoint/require_approval_destructive.yaml examples/policy/endpoint/fixtures/intent_destructive.json --json >/dev/null
approval_status=$?
set -e
if [[ "$block_status" -ne 3 ]]; then
  echo "endpoint block fixture exit mismatch: $block_status" >&2
  exit 1
fi
if [[ "$approval_status" -ne 4 ]]; then
  echo "endpoint approval fixture exit mismatch: $approval_status" >&2
  exit 1
fi

bash "$REPO_ROOT/scripts/test_skill_supply_chain.sh"

required_artifacts=(
  "$ARTIFACT_ROOT/regress_result.json"
  "$ARTIFACT_ROOT/junit.xml"
  "$ARTIFACT_ROOT/regress_init_result.json"
)
for artifact in "${required_artifacts[@]}"; do
  if [[ ! -f "$artifact" ]]; then
    echo "missing template simulation artifact: $artifact" >&2
    exit 1
  fi
done

echo "ci regress template reuse: pass"
