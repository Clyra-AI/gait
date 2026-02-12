#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <path-to-gait-binary>" >&2
  exit 2
fi

BIN_PATH="$1"
if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 2
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d)"
PORT=7989
SERVER_PID=""
cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -f ui.log regress_result.json
  rm -rf gait-out fixtures gait.yaml
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

cd "$REPO_ROOT"

"$BIN_PATH" ui --help >/dev/null

"$BIN_PATH" ui --listen "127.0.0.1:${PORT}" --open-browser=false >ui.log 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:${PORT}/api/health" >/tmp/gait_ui_health.json 2>/dev/null; then
    break
  fi
  sleep 0.25
done

if ! curl -fsS "http://127.0.0.1:${PORT}/api/health" >/tmp/gait_ui_health.json; then
  echo "ui health endpoint did not become ready" >&2
  cat ui.log >&2 || true
  exit 1
fi

curl -fsS "http://127.0.0.1:${PORT}/api/state" >/tmp/gait_ui_state_before.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"demo"}' >/tmp/gait_ui_demo.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"verify_demo"}' >/tmp/gait_ui_verify.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"receipt_demo"}' >/tmp/gait_ui_receipt.json
curl -fsS "http://127.0.0.1:${PORT}/api/state" >/tmp/gait_ui_state_after.json

python3 - <<'PY'
import json
from pathlib import Path

health = json.loads(Path('/tmp/gait_ui_health.json').read_text(encoding='utf-8'))
state_after = json.loads(Path('/tmp/gait_ui_state_after.json').read_text(encoding='utf-8'))
demo = json.loads(Path('/tmp/gait_ui_demo.json').read_text(encoding='utf-8'))
verify = json.loads(Path('/tmp/gait_ui_verify.json').read_text(encoding='utf-8'))
receipt = json.loads(Path('/tmp/gait_ui_receipt.json').read_text(encoding='utf-8'))

if not health.get('ok'):
    raise SystemExit('ui health not ok')
if demo.get('exit_code') != 0:
    raise SystemExit(f"demo command failed: {demo}")
if verify.get('exit_code') != 0:
    raise SystemExit(f"verify command failed: {verify}")
if receipt.get('exit_code') != 0:
    raise SystemExit(f"receipt command failed: {receipt}")
if not state_after.get('runpack_path'):
    raise SystemExit('expected runpack_path in post-demo state')
PY

echo "ui acceptance: pass"
