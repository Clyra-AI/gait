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

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gait-script-intent-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

WRKR_POLICY_PATH="${TMP_DIR}/policy_wrkr.yaml"
APPROVAL_POLICY_PATH="${TMP_DIR}/policy_approval.yaml"
SCRIPT_INTENT_PATH="${TMP_DIR}/intent_script.json"
WRKR_INVENTORY_PATH="${TMP_DIR}/wrkr_inventory.json"
REGISTRY_PATH="${TMP_DIR}/approved_scripts.json"
TAMPERED_REGISTRY_PATH="${TMP_DIR}/approved_scripts_tampered.json"
PRIVATE_KEY_PATH="${REPO_ROOT}/examples/scenarios/keys/approval_private.key"
PUBLIC_KEY_PATH="${REPO_ROOT}/examples/scenarios/keys/approval_public.key"

run_eval() {
  local output_path="$1"
  shift
  set +e
  "$BIN_PATH" gate eval "$@" --json >"${output_path}"
  local exit_code=$?
  set -e
  echo "${exit_code}"
}

cat >"${WRKR_POLICY_PATH}" <<'YAML'
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
scripts:
  max_steps: 8
rules:
  - name: block_wrkr_secret_write
    priority: 1
    effect: block
    match:
      tool_names: [tool.write]
      context_data_classes: [secret]
    reason_codes: [wrkr_secret_write_blocked]
    violations: [wrkr_secret_write_blocked]
  - name: require_write_approval
    priority: 10
    effect: require_approval
    match:
      tool_names: [tool.write]
    reason_codes: [approval_required_write]
YAML

cat >"${APPROVAL_POLICY_PATH}" <<'YAML'
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: require_write_approval
    priority: 10
    effect: require_approval
    match:
      tool_names: [tool.write]
    reason_codes: [approval_required_write]
YAML

cat >"${SCRIPT_INTENT_PATH}" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-20T00:00:00Z",
  "producer_version": "test-script-acceptance",
  "tool_name": "script",
  "args": {},
  "targets": [],
  "context": {
    "identity": "alice",
    "workspace": "/repo/gait",
    "risk_class": "high"
  },
  "script": {
    "steps": [
      {
        "tool_name": "tool.read",
        "args": {
          "path": "/tmp/input.txt"
        },
        "targets": [
          {
            "kind": "path",
            "value": "/tmp/input.txt",
            "operation": "read"
          }
        ]
      },
      {
        "tool_name": "tool.write",
        "args": {
          "path": "/tmp/output.txt"
        },
        "targets": [
          {
            "kind": "path",
            "value": "/tmp/output.txt",
            "operation": "write",
            "endpoint_class": "fs.write"
          }
        ]
      }
    ]
  }
}
JSON

cat >"${WRKR_INVENTORY_PATH}" <<'JSON'
[
  {
    "tool_name": "tool.read",
    "data_class": "public",
    "endpoint_class": "fs.read",
    "autonomy_level": "assist"
  },
  {
    "tool_name": "tool.write",
    "data_class": "secret",
    "endpoint_class": "fs.write",
    "autonomy_level": "assist"
  }
]
JSON

echo "==> script determinism and baseline require_approval"
EVAL_A_EXIT="$(run_eval "${TMP_DIR}/eval_a.json" --policy "${WRKR_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}")"
EVAL_B_EXIT="$(run_eval "${TMP_DIR}/eval_b.json" --policy "${WRKR_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}")"
if [[ "${EVAL_A_EXIT}" -ne 4 || "${EVAL_B_EXIT}" -ne 4 ]]; then
  echo "expected deterministic require_approval exits (4), got ${EVAL_A_EXIT} and ${EVAL_B_EXIT}" >&2
  exit 1
fi
python3 - <<'PY' "${TMP_DIR}/eval_a.json" "${TMP_DIR}/eval_b.json"
from __future__ import annotations

import json
import pathlib
import sys

first = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
second = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))

for payload in (first, second):
    assert payload.get("ok") is True, payload
    assert payload.get("script") is True, payload
    assert payload.get("step_count") == 2, payload
    assert payload.get("verdict") == "require_approval", payload
    assert "approval_required_write" in payload.get("reason_codes", []), payload
    assert payload.get("script_hash"), payload
    assert len(payload.get("step_verdicts", [])) == 2, payload

assert first["script_hash"] == second["script_hash"], (first, second)
assert first["reason_codes"] == second["reason_codes"], (first, second)
assert first["step_verdicts"] == second["step_verdicts"], (first, second)
PY

echo "==> wrkr context enrichment policy match"
EVAL_WRKR_EXIT="$(run_eval "${TMP_DIR}/eval_wrkr.json" --policy "${WRKR_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}" --wrkr-inventory "${WRKR_INVENTORY_PATH}")"
if [[ "${EVAL_WRKR_EXIT}" -ne 3 ]]; then
  echo "expected wrkr-enriched block exit (3), got ${EVAL_WRKR_EXIT}" >&2
  exit 1
fi
python3 - <<'PY' "${TMP_DIR}/eval_wrkr.json"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("verdict") == "block", payload
assert "wrkr_secret_write_blocked" in payload.get("reason_codes", []), payload
assert payload.get("context_source"), payload
PY

echo "==> wrkr fail-closed behavior when inventory is missing"
EVAL_WRKR_MISSING_EXIT="$(run_eval "${TMP_DIR}/eval_wrkr_missing.json" --policy "${WRKR_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}" --wrkr-inventory "${TMP_DIR}/missing_wrkr.json")"
if [[ "${EVAL_WRKR_MISSING_EXIT}" -ne 3 ]]; then
  echo "expected wrkr fail-closed exit (3), got ${EVAL_WRKR_MISSING_EXIT}" >&2
  exit 1
fi
python3 - <<'PY' "${TMP_DIR}/eval_wrkr_missing.json"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is False, payload
assert "wrkr inventory unavailable in fail-closed mode" in str(payload.get("error", "")), payload
PY

echo "==> approved-script fast-path allow"
"${BIN_PATH}" approve-script \
  --policy "${APPROVAL_POLICY_PATH}" \
  --intent "${SCRIPT_INTENT_PATH}" \
  --registry "${REGISTRY_PATH}" \
  --approver "secops" \
  --key-mode prod \
  --private-key "${PRIVATE_KEY_PATH}" \
  --json >"${TMP_DIR}/approve_script.json"
python3 - <<'PY' "${TMP_DIR}/approve_script.json"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("pattern_id"), payload
assert payload.get("script_hash"), payload
PY

EVAL_APPROVED_EXIT="$(run_eval "${TMP_DIR}/eval_approved.json" --policy "${APPROVAL_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}" --approved-script-registry "${REGISTRY_PATH}" --approved-script-public-key "${PUBLIC_KEY_PATH}")"
if [[ "${EVAL_APPROVED_EXIT}" -ne 0 ]]; then
  echo "expected approved-script fast-path allow exit (0), got ${EVAL_APPROVED_EXIT}" >&2
  exit 1
fi
python3 - <<'PY' "${TMP_DIR}/eval_approved.json"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("verdict") == "allow", payload
assert payload.get("pre_approved") is True, payload
assert payload.get("pattern_id"), payload
assert payload.get("registry_reason") == "approved_script_match", payload
PY

echo "==> approved-script fail-closed on signature mismatch"
python3 - <<'PY' "${REGISTRY_PATH}" "${TAMPERED_REGISTRY_PATH}"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
entries = payload.get("entries", [])
assert entries, payload
entries[0]["script_hash"] = "f" * 64
pathlib.Path(sys.argv[2]).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY

EVAL_TAMPERED_EXIT="$(run_eval "${TMP_DIR}/eval_tampered.json" --policy "${APPROVAL_POLICY_PATH}" --intent "${SCRIPT_INTENT_PATH}" --approved-script-registry "${TAMPERED_REGISTRY_PATH}" --approved-script-public-key "${PUBLIC_KEY_PATH}")"
if [[ "${EVAL_TAMPERED_EXIT}" -ne 3 ]]; then
  echo "expected tampered registry fail-closed exit (3), got ${EVAL_TAMPERED_EXIT}" >&2
  exit 1
fi
python3 - <<'PY' "${TMP_DIR}/eval_tampered.json"
from __future__ import annotations

import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("ok") is False, payload
assert "approved script registry verification failed" in str(payload.get("error", "")), payload
PY

echo "script intent acceptance: pass"
