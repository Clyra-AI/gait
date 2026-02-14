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

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

mkdir -p "$WORK_DIR/jobs" "$WORK_DIR/gait-out"
cp "$REPO_ROOT/fixtures/packspec_tck/v1/run_record_input.json" "$WORK_DIR/run_record_input.json"

echo "==> v2.4: signed capture path"
"$BIN_PATH" run record --input "$WORK_DIR/run_record_input.json" --out-dir "$WORK_DIR/gait-out" --key-mode dev --json > "$WORK_DIR/run_record.json"

python3 - "$WORK_DIR/run_record.json" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("run record failed")
if payload.get("signature_status") != "signed":
    raise SystemExit(f"expected signature_status=signed, got {payload.get('signature_status')}")
if not payload.get("signature_key_id"):
    raise SystemExit("expected signature_key_id in run record output")
PY

echo "==> v2.4: job lifecycle"
"$BIN_PATH" job submit --id job_v24 --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_submit.json"
"$BIN_PATH" job checkpoint add --id job_v24 --root "$WORK_DIR/jobs" --type decision-needed --summary "need approval" --required-action "approve" --json > "$WORK_DIR/job_checkpoint.json"
"$BIN_PATH" job pause --id job_v24 --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_pause.json"
"$BIN_PATH" job approve --id job_v24 --root "$WORK_DIR/jobs" --actor alice --reason "approved" --json > "$WORK_DIR/job_approve.json"
"$BIN_PATH" job resume --id job_v24 --root "$WORK_DIR/jobs" --allow-env-mismatch --env-fingerprint "envfp:override" --reason "approved" --json > "$WORK_DIR/job_resume.json"
"$BIN_PATH" job cancel --id job_v24 --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_cancel.json"
"$BIN_PATH" job status --id job_v24 --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_status.json"

python3 - "$WORK_DIR/job_status.json" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("job status failed")
job = payload.get("job") or {}
if job.get("status") != "cancelled":
    raise SystemExit(f"expected cancelled status, got {job.get('status')}")
if job.get("stop_reason") != "cancelled_by_user":
    raise SystemExit(f"expected cancelled_by_user stop reason, got {job.get('stop_reason')}")
PY

echo "==> v2.4: pack lifecycle"
"$BIN_PATH" pack build --type run --from "$WORK_DIR/gait-out/runpack_run_tck.zip" --out "$WORK_DIR/pack_run.zip" --json > "$WORK_DIR/pack_build_run.json"
"$BIN_PATH" pack build --type job --from job_v24 --job-root "$WORK_DIR/jobs" --out "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_build_job.json"
"$BIN_PATH" pack verify "$WORK_DIR/pack_run.zip" --json > "$WORK_DIR/pack_verify_run.json"
"$BIN_PATH" pack verify "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_verify_job.json"
"$BIN_PATH" pack inspect "$WORK_DIR/pack_run.zip" --json > "$WORK_DIR/pack_inspect_run.json"
"$BIN_PATH" pack inspect "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_inspect_job.json"
set +e
"$BIN_PATH" pack diff "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_diff.json"
DIFF_CODE=$?
set -e
if [[ $DIFF_CODE -ne 0 && $DIFF_CODE -ne 2 ]]; then
  echo "unexpected pack diff exit code: $DIFF_CODE" >&2
  exit 1
fi

python3 - "$WORK_DIR" <<'PY'
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
for name in ("pack_verify_run.json", "pack_verify_job.json"):
    payload = json.loads((work / name).read_text(encoding="utf-8"))
    if not payload.get("ok"):
        raise SystemExit(f"{name} expected ok=true")
for name in ("pack_inspect_run.json", "pack_inspect_job.json"):
    payload = json.loads((work / name).read_text(encoding="utf-8"))
    if not payload.get("ok"):
        raise SystemExit(f"{name} expected ok=true")
PY

echo "==> v2.4: replay interlocks + real execution"
set +e
"$BIN_PATH" run replay "$WORK_DIR/gait-out/runpack_run_tck.zip" --real-tools --json > "$WORK_DIR/replay_blocked.json"
BLOCKED_CODE=$?
set -e
if [[ $BLOCKED_CODE -eq 0 ]]; then
  echo "expected replay with --real-tools only to fail" >&2
  exit 1
fi
GAIT_ALLOW_REAL_REPLAY=1 "$BIN_PATH" run replay "$WORK_DIR/gait-out/runpack_run_tck.zip" --real-tools --unsafe-real-tools --allow-tools echo --json > "$WORK_DIR/replay_real.json"

python3 - "$WORK_DIR/replay_real.json" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("real replay expected ok=true")
if payload.get("mode") != "real":
    raise SystemExit(f"expected mode=real, got {payload.get('mode')}")
steps = payload.get("steps") or []
if not any(step.get("execution") == "executed" for step in steps):
    raise SystemExit("expected at least one executed replay step")
PY

echo "==> v2.4: credential evidence ttl compatibility"
cat > "$WORK_DIR/policy_broker.yaml" <<'YAML'
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: block
rules:
  - name: allow-echo-with-broker
    priority: 10
    effect: allow
    require_broker_credential: true
    broker_reference: echo
    broker_scopes: ["execute"]
    match:
      tool_names: [echo]
    reason_codes: [allow_echo]
YAML

cat > "$WORK_DIR/intent_echo.json" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "0.0.0-dev",
  "tool_name": "echo",
  "args": {"message": "hello"},
  "targets": [{"kind": "path", "value": "/tmp/echo"}],
  "context": {
    "identity": "alice",
    "workspace": "/tmp",
    "risk_class": "high",
    "session_id": "sess-1",
    "request_id": "req-1"
  }
}
JSON

"$BIN_PATH" gate eval --policy "$WORK_DIR/policy_broker.yaml" --intent "$WORK_DIR/intent_echo.json" --credential-broker stub --credential-evidence-out "$WORK_DIR/credential_stub.json" --trace-out "$WORK_DIR/trace_stub.json" --json > "$WORK_DIR/gate_stub.json"

cat > "$WORK_DIR/credential_command.sh" <<'SH'
#!/usr/bin/env sh
cat <<'JSON'
{"issued_by":"command","credential_ref":"cmd:no-ttl"}
JSON
SH
chmod +x "$WORK_DIR/credential_command.sh"
"$BIN_PATH" gate eval --policy "$WORK_DIR/policy_broker.yaml" --intent "$WORK_DIR/intent_echo.json" --credential-broker command --credential-command "$WORK_DIR/credential_command.sh" --credential-evidence-out "$WORK_DIR/credential_command.json" --trace-out "$WORK_DIR/trace_command.json" --json > "$WORK_DIR/gate_command.json"

python3 - "$WORK_DIR" <<'PY'
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
stub = json.loads((work / "credential_stub.json").read_text(encoding="utf-8"))
if "issued_at" not in stub or "expires_at" not in stub or "ttl_seconds" not in stub:
    raise SystemExit("stub credential evidence missing ttl fields")
if int(stub.get("ttl_seconds", 0)) <= 0:
    raise SystemExit("stub ttl_seconds must be > 0")

command = json.loads((work / "credential_command.json").read_text(encoding="utf-8"))
# compatibility: command broker response omitted ttl fields but record still valid.
if command.get("credential_ref") != "cmd:no-ttl":
    raise SystemExit("command credential_ref mismatch")
PY

echo "v2.4 acceptance: pass"
