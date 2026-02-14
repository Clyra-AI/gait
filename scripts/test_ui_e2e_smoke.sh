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
PORT="${GAIT_UI_E2E_PORT:-7992}"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -f ui_e2e.log
  rm -rf gait-out fixtures regress_result.json gait.yaml
}
trap cleanup EXIT

cd "$REPO_ROOT"

"$BIN_PATH" ui --listen "127.0.0.1:${PORT}" --open-browser=false >ui_e2e.log 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT}/api/health" >/tmp/gait_ui_e2e_health.json 2>/dev/null; then
    break
  fi
  sleep 0.25
done

if ! curl -fsS "http://127.0.0.1:${PORT}/api/health" >/tmp/gait_ui_e2e_health.json; then
  echo "ui health endpoint did not become ready" >&2
  cat ui_e2e.log >&2 || true
  exit 1
fi

curl -fsS "http://127.0.0.1:${PORT}/" >/tmp/gait_ui_e2e_index.html
curl -fsS "http://127.0.0.1:${PORT}/api/state" >/tmp/gait_ui_e2e_state_before.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"demo"}' >/tmp/gait_ui_e2e_demo.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"verify_demo"}' >/tmp/gait_ui_e2e_verify.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"receipt_demo"}' >/tmp/gait_ui_e2e_receipt.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"regress_init","args":{"run_id":"run_demo"}}' >/tmp/gait_ui_e2e_regress_init.json
curl -fsS -X POST "http://127.0.0.1:${PORT}/api/exec" -H 'content-type: application/json' -d '{"command":"regress_run"}' >/tmp/gait_ui_e2e_regress_run.json
curl -fsS "http://127.0.0.1:${PORT}/api/state" >/tmp/gait_ui_e2e_state_after.json

python3 - <<'PY'
import json
from pathlib import Path

def load_json(path: str):
    return json.loads(Path(path).read_text(encoding="utf-8"))

index_html = Path("/tmp/gait_ui_e2e_index.html").read_text(encoding="utf-8")
if "Gait Local UI" not in index_html:
    raise SystemExit("ui page did not render expected heading")

health = load_json("/tmp/gait_ui_e2e_health.json")
if not health.get("ok"):
    raise SystemExit("ui health was not ok")

for path in (
    "/tmp/gait_ui_e2e_demo.json",
    "/tmp/gait_ui_e2e_verify.json",
    "/tmp/gait_ui_e2e_receipt.json",
    "/tmp/gait_ui_e2e_regress_init.json",
    "/tmp/gait_ui_e2e_regress_run.json",
):
    payload = load_json(path)
    if payload.get("exit_code") != 0:
        raise SystemExit(f"command failed ({path}): {payload}")

state_after = load_json("/tmp/gait_ui_e2e_state_after.json")
if not state_after.get("runpack_path"):
    raise SystemExit("expected runpack_path after e2e flow")
if not state_after.get("regress_result_path"):
    raise SystemExit("expected regress_result_path after e2e flow")
if not state_after.get("junit_path"):
    raise SystemExit("expected junit_path after e2e flow")
PY

echo "ui e2e smoke: pass"
