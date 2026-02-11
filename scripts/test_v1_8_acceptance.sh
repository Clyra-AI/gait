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

export GAIT_BIN="$BIN_PATH"

echo "==> v1.8 openclaw install path"
bash "$REPO_ROOT/scripts/install_openclaw_skill.sh" \
  --target-dir "$WORK_DIR/openclaw/skills/gait-gate" \
  --policy "$REPO_ROOT/examples/policy/base_high_risk.yaml" \
  --json >"$WORK_DIR/openclaw_install.json"
python3 - "$WORK_DIR/openclaw_install.json" "$WORK_DIR/openclaw/skills/gait-gate" <<'PY'
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
target = pathlib.Path(sys.argv[2])

if not payload.get("ok"):
    raise SystemExit("installer did not return ok=true")
for name in ("gait_openclaw_gate.py", "skill_manifest.json", "skill_config.json"):
    if not (target / name).exists():
        raise SystemExit(f"missing installed file: {name}")
PY

echo "==> v1.8 mcp serve runtime path"
listen_addr="127.0.0.1:8788"
"$BIN_PATH" mcp serve \
  --policy "$REPO_ROOT/examples/policy-test/allow.yaml" \
  --listen "$listen_addr" \
  --trace-dir "$WORK_DIR/mcp_traces" \
  --key-mode dev >"$WORK_DIR/mcp_serve.log" 2>&1 &
server_pid=$!
cleanup_server() {
  if kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
}
trap 'cleanup_server; rm -rf "$WORK_DIR"' EXIT

ready=0
for _ in $(seq 1 60); do
  if curl -fsS "http://${listen_addr}/healthz" >/dev/null 2>/dev/null; then
    ready=1
    break
  fi
  sleep 0.2
done
if [[ "$ready" -ne 1 ]]; then
  echo "mcp serve did not become ready" >&2
  cat "$WORK_DIR/mcp_serve.log" >&2
  exit 1
fi

cat >"$WORK_DIR/mcp_request.json" <<'JSON'
{
  "adapter": "openai",
  "run_id": "run_v1_8_acceptance",
  "call": {
    "type": "function",
    "function": {
      "name": "tool.search",
      "arguments": "{\"query\":\"gait\"}"
    }
  }
}
JSON

curl -fsS \
  -H "content-type: application/json" \
  --data-binary "@$WORK_DIR/mcp_request.json" \
  "http://${listen_addr}/v1/evaluate" >"$WORK_DIR/mcp_response.json"

curl -fsS -N \
  -H "content-type: application/json" \
  --data-binary "@$WORK_DIR/mcp_request.json" \
  "http://${listen_addr}/v1/evaluate/sse" >"$WORK_DIR/mcp_response_sse.txt"

curl -fsS -N \
  -H "content-type: application/json" \
  --data-binary "@$WORK_DIR/mcp_request.json" \
  "http://${listen_addr}/v1/evaluate/stream" >"$WORK_DIR/mcp_response_stream.jsonl"

python3 - "$WORK_DIR/mcp_response.json" "$WORK_DIR/mcp_response_sse.txt" "$WORK_DIR/mcp_response_stream.jsonl" <<'PY'
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit(f"mcp serve response not ok: {payload}")
if payload.get("verdict") != "allow":
    raise SystemExit(f"mcp serve verdict mismatch: {payload.get('verdict')}")
if payload.get("exit_code") != 0:
    raise SystemExit(f"mcp serve exit code mismatch: {payload.get('exit_code')}")
trace_path = payload.get("trace_path", "")
if not trace_path:
    raise SystemExit("mcp serve response missing trace_path")
if not pathlib.Path(trace_path).exists():
    raise SystemExit(f"mcp serve trace_path does not exist: {trace_path}")

sse_raw = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
sse_data = None
for line in sse_raw.splitlines():
    if line.startswith("data: "):
        sse_data = line[len("data: "):]
        break
if sse_data is None:
    raise SystemExit(f"mcp serve sse response missing data line: {sse_raw!r}")
sse_payload = json.loads(sse_data)
if sse_payload.get("verdict") != "allow":
    raise SystemExit(f"mcp serve sse verdict mismatch: {sse_payload.get('verdict')}")
if sse_payload.get("exit_code") != 0:
    raise SystemExit(f"mcp serve sse exit code mismatch: {sse_payload.get('exit_code')}")

stream_lines = [line for line in pathlib.Path(sys.argv[3]).read_text(encoding="utf-8").splitlines() if line.strip()]
if len(stream_lines) != 1:
    raise SystemExit(f"mcp serve stream expected one line, got {len(stream_lines)}")
stream_payload = json.loads(stream_lines[0])
if stream_payload.get("verdict") != "allow":
    raise SystemExit(f"mcp serve stream verdict mismatch: {stream_payload.get('verdict')}")
if stream_payload.get("exit_code") != 0:
    raise SystemExit(f"mcp serve stream exit code mismatch: {stream_payload.get('exit_code')}")
PY
cleanup_server

echo "==> v1.8 run inspect timeline path"
"$BIN_PATH" demo --json >"$WORK_DIR/demo.json"
"$BIN_PATH" run inspect --from run_demo --json >"$WORK_DIR/run_inspect.json"
python3 - "$WORK_DIR/run_inspect.json" <<'PY'
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit(f"run inspect returned ok=false: {payload}")
if payload.get("run_id") != "run_demo":
    raise SystemExit(f"run inspect run_id mismatch: {payload.get('run_id')}")
if payload.get("intents_total", 0) <= 0:
    raise SystemExit("run inspect expected intents_total > 0")
if not payload.get("entries"):
    raise SystemExit("run inspect expected non-empty entries")
PY

echo "==> v1.8 gastown integration path"
python3 "$REPO_ROOT/examples/integrations/gastown/quickstart.py" --scenario allow >"$WORK_DIR/gastown_allow.txt"
python3 "$REPO_ROOT/examples/integrations/gastown/quickstart.py" --scenario block >"$WORK_DIR/gastown_block.txt"

python3 - "$WORK_DIR/gastown_allow.txt" "$WORK_DIR/gastown_block.txt" <<'PY'
from __future__ import annotations

import pathlib
import sys

allow_lines = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines()
block_lines = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8").splitlines()
allow = {k: v for line in allow_lines if "=" in line for k, v in [line.split("=", 1)]}
block = {k: v for line in block_lines if "=" in line for k, v in [line.split("=", 1)]}

if allow.get("framework") != "gastown":
    raise SystemExit(f"gastown allow framework mismatch: {allow}")
if allow.get("executed") != "true":
    raise SystemExit(f"gastown allow executed mismatch: {allow}")
if block.get("framework") != "gastown":
    raise SystemExit(f"gastown block framework mismatch: {block}")
if block.get("executed") != "false":
    raise SystemExit(f"gastown block executed mismatch: {block}")
PY

echo "==> v1.8 beads bridge dry-run"
trace_path="$REPO_ROOT/gait-out/integrations/gastown/trace_block.json"
bash "$REPO_ROOT/scripts/bridge_trace_to_beads.sh" \
  --trace "$trace_path" \
  --dry-run \
  --json >"$WORK_DIR/beads_bridge.json"
python3 - "$WORK_DIR/beads_bridge.json" <<'PY'
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("mode") != "dry_run":
    raise SystemExit(f"unexpected beads bridge mode: {payload.get('mode')}")
if payload.get("trace_id", "") == "":
    raise SystemExit("beads bridge missing trace_id")
if payload.get("title", "") == "":
    raise SystemExit("beads bridge missing title")
PY

echo "==> v1.8 distribution artifacts"
required_files=(
  "$REPO_ROOT/docs/launch/rfc_openclaw.md"
  "$REPO_ROOT/docs/launch/rfc_gastown.md"
  "$REPO_ROOT/docs/launch/rfc_agent_framework_template.md"
  "$REPO_ROOT/docs/launch/secure_deployment_openclaw.md"
  "$REPO_ROOT/docs/launch/secure_deployment_gastown.md"
  "$REPO_ROOT/docs/zero_trust_stack.md"
  "$REPO_ROOT/docs/external_tool_registry_policy.md"
  "$REPO_ROOT/docs/siem_ingestion_recipes.md"
)
for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing distribution artifact: $file" >&2
    exit 1
  fi
done

python3 "$REPO_ROOT/scripts/validate_community_index.py"

echo "v1.8 acceptance: pass"
