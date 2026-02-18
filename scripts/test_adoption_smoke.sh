#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go build -o ./gait ./cmd/gait
export PATH="$repo_root:$PATH"

mkdir -p "$repo_root/gait-out"

had_fixtures=0
if [[ -d "$repo_root/fixtures" ]]; then
  had_fixtures=1
fi

created_before="$(mktemp)"
touch "$created_before"

cleanup() {
  find "$repo_root" -maxdepth 1 -type f \
    \( -name 'trace_*.json' -o -name 'approval_audit_*.json' -o -name 'gait.yaml' -o -name 'regress_result.json' \) \
    -newer "$created_before" -delete
  if [[ "$had_fixtures" -eq 0 && -d "$repo_root/fixtures" ]]; then
    rm -rf "$repo_root/fixtures"
  fi
  rm -f "$created_before"
}
trap cleanup EXIT

echo "==> quickstart smoke"
quickstart_output="$(bash scripts/quickstart.sh)"
printf '%s\n' "$quickstart_output"
QUICKSTART_OUTPUT="$quickstart_output" python3 - <<'PY'
import os
from pathlib import Path

raw = os.environ["QUICKSTART_OUTPUT"]
parsed: dict[str, str] = {}
for line in raw.splitlines():
    if "=" not in line:
        continue
    key, value = line.split("=", 1)
    parsed[key.strip()] = value.strip()

required = [
    "quickstart_status",
    "run_id",
    "runpack",
    "verify_json",
    "regress_init_json",
    "regress_run_json",
    "junit",
]
missing = [field for field in required if field not in parsed]
if missing:
    raise SystemExit(f"quickstart missing required output fields: {missing}")
if parsed["quickstart_status"] != "pass":
    raise SystemExit(f"quickstart_status expected pass, got {parsed['quickstart_status']}")
for field in ("runpack", "verify_json", "regress_init_json", "regress_run_json", "junit"):
    if not Path(parsed[field]).exists():
        raise SystemExit(f"quickstart artifact missing: {field}={parsed[field]}")
PY

echo "==> integration adapter smoke"
bash scripts/test_adapter_parity.sh
python3 examples/integrations/template/quickstart.py --scenario allow
python3 examples/integrations/template/quickstart.py --scenario block
python3 examples/integrations/template/quickstart.py --scenario require_approval
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
for scenario in allow block require_approval; do
  voice_output="$(python3 examples/integrations/voice_reference/quickstart.py --scenario "$scenario")"
  printf '%s\n' "$voice_output"
  VOICE_SCENARIO="$scenario" VOICE_OUTPUT="$voice_output" python3 - <<'PY'
import os
from pathlib import Path

scenario = os.environ["VOICE_SCENARIO"]
output = os.environ["VOICE_OUTPUT"]
parsed: dict[str, str] = {}
for raw_line in output.splitlines():
    line = raw_line.strip()
    if not line or "=" not in line:
        continue
    key, value = line.split("=", 1)
    parsed[key.strip()] = value.strip()

for field in ("framework", "scenario", "verdict", "speak_emitted", "trace_path", "call_record", "callpack_path"):
    if field not in parsed:
        raise SystemExit(f"missing field {field} in voice adapter output: {output}")
if parsed["framework"] != "voice_reference":
    raise SystemExit(f"voice framework mismatch: {parsed['framework']}")
if parsed["scenario"] != scenario:
    raise SystemExit(f"voice scenario mismatch: expected={scenario} got={parsed['scenario']}")
for field in ("trace_path", "call_record", "callpack_path"):
    path = Path(parsed[field])
    if not path.exists():
        raise SystemExit(f"voice artifact missing: {field}={path}")
if scenario == "allow":
    if parsed["verdict"] != "allow" or parsed["speak_emitted"].lower() != "true":
        raise SystemExit(f"voice allow mismatch: {parsed}")
    token_path = parsed.get("token_path", "")
    if not token_path or not Path(token_path).exists():
        raise SystemExit(f"voice allow token missing: {parsed}")
else:
    expected = "block" if scenario == "block" else "require_approval"
    if parsed["verdict"] != expected or parsed["speak_emitted"].lower() != "false":
        raise SystemExit(f"voice {scenario} mismatch: {parsed}")
PY
done
python3 - <<'PY'
from pathlib import Path

required = [
    Path("gait-out/integrations/template/trace_allow.json"),
    Path("gait-out/integrations/template/trace_block.json"),
    Path("gait-out/integrations/template/trace_require_approval.json"),
    Path("gait-out/integrations/openai_agents/trace_require_approval.json"),
    Path("gait-out/integrations/voice_reference/callpack_allow.zip"),
    Path("gait-out/integrations/voice_reference/callpack_block.zip"),
    Path("gait-out/integrations/voice_reference/callpack_require_approval.zip"),
]
missing = [str(path) for path in required if not path.exists()]
if missing:
    raise SystemExit(f"missing adapter first-win trace artifacts: {missing}")
PY

echo "==> sidecar smoke"
set +e
python3 examples/sidecar/gate_sidecar.py \
  --policy examples/policy-test/allow.yaml \
  --intent-file core/schema/testdata/gate_intent_request_valid.json \
  --trace-out "$repo_root/gait-out/trace_sidecar_allow.json" \
  > "$repo_root/gait-out/sidecar_allow.json"
ALLOW_CODE=$?
python3 examples/sidecar/gate_sidecar.py \
  --policy examples/policy-test/block.yaml \
  --intent-file core/schema/testdata/gate_intent_request_valid.json \
  --trace-out "$repo_root/gait-out/trace_sidecar_block.json" \
  > "$repo_root/gait-out/sidecar_block.json"
BLOCK_CODE=$?
set -e
if [[ "$ALLOW_CODE" -ne 0 ]]; then
  echo "sidecar allow exit mismatch: $ALLOW_CODE" >&2
  exit 1
fi
if [[ "$BLOCK_CODE" -ne 3 ]]; then
  echo "sidecar block exit mismatch: $BLOCK_CODE" >&2
  exit 1
fi
python3 - <<'PY'
import json
from pathlib import Path

allow = json.loads(Path("gait-out/sidecar_allow.json").read_text(encoding="utf-8"))
block = json.loads(Path("gait-out/sidecar_block.json").read_text(encoding="utf-8"))

if allow.get("gate_result", {}).get("verdict") != "allow":
    raise SystemExit("sidecar allow verdict mismatch")
if block.get("gate_result", {}).get("verdict") != "block":
    raise SystemExit("sidecar block verdict mismatch")
if not allow.get("trace_path"):
    raise SystemExit("sidecar allow trace_path missing")
if not block.get("trace_path"):
    raise SystemExit("sidecar block trace_path missing")
PY

echo "==> scenario pack smoke"
bash examples/scenarios/incident_reproduction.sh
bash examples/scenarios/prompt_injection_block.sh
bash examples/scenarios/approval_flow.sh

echo "adoption smoke: pass"
