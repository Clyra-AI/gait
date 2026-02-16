#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GAIT_BIN="${GAIT_BIN:-$REPO_ROOT/gait}"

if [[ "${GAIT_BIN:-}" == "$REPO_ROOT/gait" ]]; then
  (cd "$REPO_ROOT" && go build -o ./gait ./cmd/gait)
elif [[ ! -x "$GAIT_BIN" ]]; then
  echo "GAIT_BIN is not executable: $GAIT_BIN" >&2
  exit 1
fi

WORK_DIR="${1:-$REPO_ROOT/gait-out/mcp_canonical}"
mkdir -p "$WORK_DIR" "$WORK_DIR/traces" "$WORK_DIR/runpacks" "$WORK_DIR/packs"

next_free_port() {
  python3 - <<'PY'
import socket
s=socket.socket()
s.bind(("127.0.0.1",0))
print(s.getsockname()[1])
s.close()
PY
}

run_case() {
  local case_name="$1"
  local policy_path="$2"
  local expected_verdict="$3"
  local expected_exit="$4"

  local port
  port="$(next_free_port)"
  local listen_addr="127.0.0.1:${port}"
  local log_path="$WORK_DIR/${case_name}_serve.log"
  local req_path="$WORK_DIR/${case_name}_request.json"
  local resp_path="$WORK_DIR/${case_name}_response.json"

  cat > "$req_path" <<JSON
{
  "adapter": "openai",
  "run_id": "run_mcp_${case_name}",
  "emit_runpack": true,
  "emit_pack": true,
  "call": {
    "type": "function",
    "function": {
      "name": "tool.write",
      "arguments": "{\"path\":\"/tmp/${case_name}.txt\",\"content\":\"demo\"}"
    }
  }
}
JSON

  "$GAIT_BIN" mcp serve \
    --policy "$policy_path" \
    --adapter openai \
    --listen "$listen_addr" \
    --trace-dir "$WORK_DIR/traces" \
    --runpack-dir "$WORK_DIR/runpacks" \
    --pack-dir "$WORK_DIR/packs" \
    --key-mode dev >"$log_path" 2>&1 &
  local server_pid=$!

  cleanup() {
    if kill -0 "$server_pid" >/dev/null 2>&1; then
      kill "$server_pid" >/dev/null 2>&1 || true
      wait "$server_pid" >/dev/null 2>&1 || true
    fi
  }
  trap cleanup RETURN

  local ready=0
  for _ in $(seq 1 80); do
    if curl -fsS "http://${listen_addr}/healthz" >/dev/null 2>/dev/null; then
      ready=1
      break
    fi
    sleep 0.1
  done
  if [[ "$ready" -ne 1 ]]; then
    echo "mcp serve did not become ready for ${case_name}" >&2
    cat "$log_path" >&2
    exit 1
  fi

  curl -fsS \
    -H "content-type: application/json" \
    --data-binary "@$req_path" \
    "http://${listen_addr}/v1/evaluate" >"$resp_path"

  python3 - "$resp_path" "$expected_verdict" "$expected_exit" "$GAIT_BIN" <<'PY'
import json
import pathlib
import subprocess
import sys

resp_path = pathlib.Path(sys.argv[1])
expected_verdict = sys.argv[2]
expected_exit = int(sys.argv[3])
gait_bin = sys.argv[4]

payload = json.loads(resp_path.read_text(encoding="utf-8"))
if payload.get("ok") is not True:
    raise SystemExit(f"expected ok=true, got {payload}")
if payload.get("verdict") != expected_verdict:
    raise SystemExit(f"expected verdict={expected_verdict}, got {payload.get('verdict')}")
if payload.get("exit_code") != expected_exit:
    raise SystemExit(f"expected exit_code={expected_exit}, got {payload.get('exit_code')}")

trace_path = pathlib.Path(payload.get("trace_path", ""))
if not trace_path.exists():
    raise SystemExit(f"missing trace artifact: {trace_path}")

runpack_path = pathlib.Path(payload.get("runpack_path", ""))
if not runpack_path.exists():
    raise SystemExit(f"missing runpack artifact: {runpack_path}")
pack_path = pathlib.Path(payload.get("pack_path", ""))
if not pack_path.exists():
    raise SystemExit(f"missing pack artifact: {pack_path}")

verify = subprocess.run(
    [gait_bin, "verify", str(runpack_path), "--json"],
    text=True,
    capture_output=True,
    check=False,
)
if verify.returncode != 0:
    raise SystemExit(f"runpack verify failed ({verify.returncode}): {verify.stdout}\n{verify.stderr}")
verify_payload = json.loads(verify.stdout)
if verify_payload.get("ok") is not True:
    raise SystemExit(f"runpack verify payload not ok: {verify_payload}")
pack_verify = subprocess.run(
    [gait_bin, "pack", "verify", str(pack_path), "--json"],
    text=True,
    capture_output=True,
    check=False,
)
if pack_verify.returncode != 0:
    raise SystemExit(
        f"pack verify failed ({pack_verify.returncode}): {pack_verify.stdout}\n{pack_verify.stderr}"
    )
pack_verify_payload = json.loads(pack_verify.stdout)
if pack_verify_payload.get("ok") is not True:
    raise SystemExit(f"pack verify payload not ok: {pack_verify_payload}")

summary = {
    "ok": True,
    "verdict": payload["verdict"],
    "exit_code": payload["exit_code"],
    "trace_path": str(trace_path),
    "runpack_path": str(runpack_path),
    "pack_path": str(pack_path),
    "pack_id": payload.get("pack_id", ""),
    "verify_manifest_digest": verify_payload.get("manifest_digest", ""),
}
print(json.dumps(summary, separators=(",", ":"), sort_keys=True))
PY

  cp "$resp_path" "$WORK_DIR/${case_name}_response_raw.json"
  python3 - "$resp_path" "$WORK_DIR/${case_name}_summary.json" <<'PY'
import json
import pathlib
import sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
out = {
    "ok": payload.get("ok", False),
    "verdict": payload.get("verdict", ""),
    "exit_code": payload.get("exit_code", -1),
    "trace_path": payload.get("trace_path", ""),
    "runpack_path": payload.get("runpack_path", ""),
    "pack_path": payload.get("pack_path", ""),
    "pack_id": payload.get("pack_id", ""),
  }
pathlib.Path(sys.argv[2]).write_text(json.dumps(out, indent=2) + "\n", encoding="utf-8")
PY
}

run_case "allow" "$REPO_ROOT/examples/integrations/template/policy_allow.yaml" "allow" 0
run_case "block" "$REPO_ROOT/examples/integrations/template/policy_block.yaml" "block" 3
run_case "require_approval" "$REPO_ROOT/examples/integrations/template/policy_require_approval.yaml" "require_approval" 4

python3 - "$WORK_DIR" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
summary = {
    "schema_id": "gait.mcp.canonical.summary",
    "schema_version": "1.0.0",
    "cases": {},
}
for name in ("allow", "block", "require_approval"):
    summary["cases"][name] = json.loads((root / f"{name}_summary.json").read_text(encoding="utf-8"))
(root / "mcp_canonical_summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
print(json.dumps({"ok": True, "summary": str(root / "mcp_canonical_summary.json")}, separators=(",", ":")))
PY
