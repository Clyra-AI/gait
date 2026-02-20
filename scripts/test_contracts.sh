#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: $0 [path-to-gait-binary]" >&2
  exit 2
fi

resolve_bin_path() {
  local candidate="$1"
  if [[ -x "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  if [[ -x "${candidate}.exe" ]]; then
    printf '%s\n' "${candidate}.exe"
    return 0
  fi
  return 1
}

if [[ $# -eq 1 ]]; then
  if [[ "$1" = /* ]]; then
    BIN_CANDIDATE="$1"
  else
    BIN_CANDIDATE="$(pwd)/$1"
  fi
else
  BIN_CANDIDATE="$REPO_ROOT/gait"
  go build -o "$BIN_CANDIDATE" ./cmd/gait
fi

if ! BIN_PATH="$(resolve_bin_path "$BIN_CANDIDATE")"; then
  echo "binary is not executable: $BIN_CANDIDATE" >&2
  exit 2
fi

WORK_A="$(mktemp -d)"
WORK_B="$(mktemp -d)"
trap 'rm -rf "$WORK_A" "$WORK_B"' EXIT

echo "==> deterministic artifact bytes"
(
  cd "$WORK_A"
  "$BIN_PATH" demo >/dev/null
)
(
  cd "$WORK_B"
  "$BIN_PATH" demo >/dev/null
)
SUM_B="$(python3 - "$WORK_B/gait-out/runpack_run_demo.zip" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
print(hashlib.sha256(path.read_bytes()).hexdigest())
PY
)"
SUM_A="$(python3 - "$WORK_A/gait-out/runpack_run_demo.zip" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
print(hashlib.sha256(path.read_bytes()).hexdigest())
PY
)"
if [[ "$SUM_A" != "$SUM_B" ]]; then
  echo "determinism failure: runpack bytes differ" >&2
  exit 1
fi

echo "==> stable exit-code contract"
cp "$REPO_ROOT/examples/policy-test/intent.json" "$WORK_A/intent.json"
cp "$REPO_ROOT/examples/policy-test/allow.yaml" "$WORK_A/allow.yaml"
cp "$REPO_ROOT/examples/policy-test/block.yaml" "$WORK_A/block.yaml"
cp "$REPO_ROOT/examples/policy-test/require_approval.yaml" "$WORK_A/require_approval.yaml"

(
  cd "$WORK_A"
  "$BIN_PATH" policy test allow.yaml intent.json --json > allow.json
)

set +e
(
  cd "$WORK_A"
  "$BIN_PATH" policy test block.yaml intent.json --json > block.json
)
BLOCK_CODE=$?
(
  cd "$WORK_A"
  "$BIN_PATH" policy test require_approval.yaml intent.json --json > require_approval.json
)
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

echo "==> json shape contract"
(
  cd "$WORK_A"
  "$BIN_PATH" verify run_demo --json > verify.json
  "$BIN_PATH" gate eval \
    --policy "$REPO_ROOT/examples/policy/base_low_risk.yaml" \
    --intent "$REPO_ROOT/examples/policy/intents/intent_read.json" \
    --trace-out "$WORK_A/trace_contract.json" \
    --json > gate.json
)

python3 - "$WORK_A" <<'PY'
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
verify = json.loads((work / "verify.json").read_text(encoding="utf-8"))
gate = json.loads((work / "gate.json").read_text(encoding="utf-8"))

verify_required = {"ok", "run_id", "manifest_digest", "files_checked", "signature_status"}
gate_required = {"ok", "verdict", "reason_codes", "trace_id", "trace_path", "policy_digest", "intent_digest"}

missing_verify = sorted(verify_required.difference(verify.keys()))
missing_gate = sorted(gate_required.difference(gate.keys()))

if missing_verify:
    raise SystemExit(f"verify shape missing keys: {missing_verify}")
if missing_gate:
    raise SystemExit(f"gate shape missing keys: {missing_gate}")
if verify.get("ok") is not True:
    raise SystemExit("verify expected ok=true")
if gate.get("verdict") != "allow":
    raise SystemExit(f"gate expected allow verdict, got={gate.get('verdict')}")
if not isinstance(gate.get("reason_codes"), list):
    raise SystemExit("gate reason_codes must be an array")
PY

echo "==> primitive schema compatibility guard"
python3 - "$REPO_ROOT" "$WORK_A" <<'PY'
import json
import sys
import zipfile
from pathlib import Path

repo = Path(sys.argv[1])
work = Path(sys.argv[2])

expected = {
    "schemas/v1/gate/intent_request.schema.json": {
        "schema_id": "gait.gate.intent_request",
        "schema_version_pattern": r"^1\.0\.0$",
        "required": [
            "schema_id",
            "schema_version",
            "created_at",
            "producer_version",
            "tool_name",
            "args",
            "targets",
            "context",
        ],
    },
    "schemas/v1/gate/gate_result.schema.json": {
        "schema_id": "gait.gate.result",
        "schema_version_pattern": r"^1\.0\.0$",
        "required": [
            "schema_id",
            "schema_version",
            "created_at",
            "producer_version",
            "verdict",
            "reason_codes",
            "violations",
        ],
        "verdict_enum": ["allow", "block", "dry_run", "require_approval"],
    },
    "schemas/v1/gate/trace_record.schema.json": {
        "schema_id": "gait.gate.trace",
        "schema_version_pattern": r"^1\.0\.0$",
        "required": [
            "schema_id",
            "schema_version",
            "created_at",
            "producer_version",
            "trace_id",
            "tool_name",
            "args_digest",
            "intent_digest",
            "policy_digest",
            "verdict",
        ],
        "verdict_enum": ["allow", "block", "dry_run", "require_approval"],
    },
    "schemas/v1/gate/approved_script_entry.schema.json": {
        "schema_id": "gait.gate.approved_script_entry",
        "schema_version_pattern": r"^1\.0\.0$",
        "required": [
            "schema_id",
            "schema_version",
            "created_at",
            "producer_version",
            "pattern_id",
            "policy_digest",
            "script_hash",
            "tool_sequence",
            "approver_identity",
            "expires_at",
        ],
    },
    "schemas/v1/runpack/manifest.schema.json": {
        "schema_id": "gait.runpack.manifest",
        "schema_version_pattern": r"^1\.0\.0$",
        "required": [
            "schema_id",
            "schema_version",
            "created_at",
            "producer_version",
            "run_id",
            "capture_mode",
            "files",
            "manifest_digest",
        ],
        "capture_mode_enum": ["reference", "raw"],
    },
}

for rel_path, spec in expected.items():
    schema = json.loads((repo / rel_path).read_text(encoding="utf-8"))
    required = schema.get("required", [])
    if sorted(required) != sorted(spec["required"]):
        raise SystemExit(f"{rel_path} required fields changed: {required}")
    properties = schema.get("properties", {})
    if properties.get("schema_id", {}).get("const") != spec["schema_id"]:
        raise SystemExit(f"{rel_path} schema_id const mismatch")
    if properties.get("schema_version", {}).get("pattern") != spec["schema_version_pattern"]:
        raise SystemExit(f"{rel_path} schema_version pattern mismatch")
    verdict_enum = spec.get("verdict_enum")
    if verdict_enum is not None:
        actual = properties.get("verdict", {}).get("enum")
        if actual != verdict_enum:
            raise SystemExit(f"{rel_path} verdict enum changed: {actual}")
    capture_mode_enum = spec.get("capture_mode_enum")
    if capture_mode_enum is not None:
        actual = properties.get("capture_mode", {}).get("enum")
        if actual != capture_mode_enum:
            raise SystemExit(f"{rel_path} capture_mode enum changed: {actual}")

runpack_path = work / "gait-out" / "runpack_run_demo.zip"
required_runpack_files = {
    "manifest.json",
    "run.json",
    "intents.jsonl",
    "results.jsonl",
    "refs.json",
}
with zipfile.ZipFile(runpack_path, "r") as archive:
    members = set(archive.namelist())
missing = sorted(required_runpack_files.difference(members))
if missing:
    raise SystemExit(f"runpack missing required files: {missing}")
PY

echo "==> compatibility roundtrip from legacy fixture"
cp "$REPO_ROOT/internal/integration/testdata/legacy_run_record_v1.json" "$WORK_A/legacy_run_record_v1.json"
(
  cd "$WORK_A"
  "$BIN_PATH" run record --input legacy_run_record_v1.json --out-dir "$WORK_A/gait-out" --json > legacy_record.json
  "$BIN_PATH" migrate --input "$WORK_A/gait-out/runpack_run_legacy_fixture.zip" --json > legacy_migrate.json
  "$BIN_PATH" verify "$WORK_A/gait-out/runpack_run_legacy_fixture_migrated.zip" --json > legacy_verify.json
)

python3 - "$WORK_A" <<'PY'
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
record = json.loads((work / "legacy_record.json").read_text(encoding="utf-8"))
migrate = json.loads((work / "legacy_migrate.json").read_text(encoding="utf-8"))
verify = json.loads((work / "legacy_verify.json").read_text(encoding="utf-8"))

if not record.get("ok"):
    raise SystemExit("legacy record failed")
if not migrate.get("ok"):
    raise SystemExit("legacy migrate failed")
if migrate.get("status") not in {"migrated", "up_to_date", "no_change"}:
    raise SystemExit(f"unexpected migrate status: {migrate.get('status')}")
if not verify.get("ok"):
    raise SystemExit("legacy migrated verify failed")
PY

echo "==> intent + receipt conformance gate"
bash "$REPO_ROOT/scripts/test_intent_receipt_conformance.sh" "$BIN_PATH"

echo "==> pack producer kit compatibility"
bash "$REPO_ROOT/scripts/test_pack_producer_kit.sh" "$BIN_PATH"

echo "contracts: pass"
