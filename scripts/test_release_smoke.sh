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
