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

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gait-context-chaos-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT
OUT_DIR="${TMP_DIR}/out"
mkdir -p "${OUT_DIR}"

cat >"${TMP_DIR}/run_record.json" <<'JSON'
{
  "run": {
    "schema_id": "gait.runpack.run",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "test-v2.5",
    "run_id": "run_ctx_chaos",
    "env": {"os":"darwin","arch":"arm64","runtime":"go"},
    "timeline": [{"event":"run_started","ts":"2026-02-14T00:00:00Z"}]
  },
  "intents": [],
  "results": [],
  "refs": {
    "schema_id": "gait.runpack.refs",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "test-v2.5",
    "run_id": "run_ctx_chaos",
    "receipts": []
  },
  "capture_mode": "reference"
}
JSON

echo "==> chaos: required mode without evidence blocks"
set +e
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-evidence-mode required --json >"${TMP_DIR}/required_missing.json"
MISSING_EXIT=$?
set -e
if [[ "${MISSING_EXIT}" -eq 0 ]]; then
  echo "error: expected required context mode without evidence to fail" >&2
  exit 1
fi

echo "==> chaos: tampered context_set_digest blocks capture"
cat >"${TMP_DIR}/env_tampered_digest.json" <<'JSON'
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_chaos_tampered",
  "context_set_digest": "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "reference",
      "immutability": "immutable"
    }
  ]
}
JSON
set +e
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-envelope "${TMP_DIR}/env_tampered_digest.json" --context-evidence-mode required --json >"${TMP_DIR}/tampered_digest.json"
TAMPERED_DIGEST_EXIT=$?
set -e
if [[ "${TAMPERED_DIGEST_EXIT}" -eq 0 ]]; then
  echo "error: expected tampered context digest to fail" >&2
  exit 1
fi

echo "==> chaos: oversized context envelope blocks capture"
python3 - <<'PY' "${TMP_DIR}/env_oversized.json"
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_text("{" + "\"junk\":\"" + ("x" * (1024 * 1024 + 16)) + "\"}", encoding="utf-8")
PY
set +e
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-envelope "${TMP_DIR}/env_oversized.json" --context-evidence-mode required --json >"${TMP_DIR}/oversized.json"
OVERSIZED_EXIT=$?
set -e
if [[ "${OVERSIZED_EXIT}" -eq 0 ]]; then
  echo "error: expected oversized context envelope to fail" >&2
  exit 1
fi

cat >"${TMP_DIR}/env_a.json" <<'JSON'
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_chaos",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "reference",
      "immutability": "immutable"
    }
  ]
}
JSON

cat >"${TMP_DIR}/env_b.json" <<'JSON'
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_chaos",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "reference",
      "immutability": "immutable"
    }
  ]
}
JSON

echo "==> chaos: concurrent context capture contention remains deterministic"
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-envelope "${TMP_DIR}/env_a.json" --context-evidence-mode required --json >"${TMP_DIR}/baseline_record.json"
(
  "${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-envelope "${TMP_DIR}/env_a.json" --context-evidence-mode required --json >/dev/null
) &
PID_A=$!
(
  "${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --context-envelope "${TMP_DIR}/env_b.json" --context-evidence-mode required --json >/dev/null
) &
PID_B=$!
wait "${PID_A}"
wait "${PID_B}"

"${BIN_PATH}" verify "${OUT_DIR}/runpack_run_ctx_chaos.zip" --json >"${TMP_DIR}/verify_after_contention.json"
python3 - <<'PY' "${TMP_DIR}/verify_after_contention.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
PY

echo "==> chaos: semantic context drift classification"
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --run-id run_ctx_chaos_a --context-envelope "${TMP_DIR}/env_a.json" --context-evidence-mode required --json >"${TMP_DIR}/record_a.json"
"${BIN_PATH}" run record --input "${TMP_DIR}/run_record.json" --out-dir "${OUT_DIR}" --run-id run_ctx_chaos_b --context-envelope "${TMP_DIR}/env_b.json" --context-evidence-mode required --json >"${TMP_DIR}/record_b.json"

RUNPACK_A="$(python3 - <<'PY' "${TMP_DIR}/record_a.json"
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["bundle"])
PY
)"
RUNPACK_B="$(python3 - <<'PY' "${TMP_DIR}/record_b.json"
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["bundle"])
PY
)"

"${BIN_PATH}" pack build --type run --from "${RUNPACK_A}" --out "${TMP_DIR}/pack_a.zip" --json >/dev/null
"${BIN_PATH}" pack build --type run --from "${RUNPACK_B}" --out "${TMP_DIR}/pack_b.zip" --json >/dev/null
set +e
"${BIN_PATH}" pack diff "${TMP_DIR}/pack_a.zip" "${TMP_DIR}/pack_b.zip" --json >"${TMP_DIR}/diff_semantic.json"
DIFF_EXIT=$?
set -e
if [[ "${DIFF_EXIT}" -ne 0 && "${DIFF_EXIT}" -ne 2 ]]; then
  echo "error: unexpected pack diff exit code: ${DIFF_EXIT}" >&2
  exit 1
fi

python3 - <<'PY' "${TMP_DIR}/diff_semantic.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
diff_payload = payload.get("diff") or {}
result = diff_payload.get("result") or diff_payload.get("Result") or {}
summary = result.get("summary") or {}
assert summary.get("context_drift_classification") == "semantic", summary
assert summary.get("context_changed") is True, summary
PY

echo "==> chaos: context_set_digest tamper breaks trace verification"
cat >"${TMP_DIR}/policy_trace.yaml" <<'YAML'
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: allow-context-trace
    priority: 1
    effect: allow
    match:
      tool_names: [tool.demo]
YAML

cat >"${TMP_DIR}/intent_trace.json" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "tool_name": "tool.demo",
  "args": {"x":"y"},
  "targets": [{"kind":"path","value":"/tmp/demo","operation":"write"}],
  "context": {
    "identity":"u_demo",
    "workspace":"/tmp",
    "risk_class":"high",
    "context_set_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "context_evidence_mode":"required",
    "context_refs":["ctx_001"]
  }
}
JSON

"${BIN_PATH}" keys init --out-dir "${TMP_DIR}" --prefix trace --force --json >"${TMP_DIR}/keys_init.json"
TRACE_PRIVATE_KEY="$(python3 - <<'PY' "${TMP_DIR}/keys_init.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
print(payload["private_key_path"])
PY
)"
TRACE_PUBLIC_KEY="$(python3 - <<'PY' "${TMP_DIR}/keys_init.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
print(payload["public_key_path"])
PY
)"
"${BIN_PATH}" gate eval \
  --policy "${TMP_DIR}/policy_trace.yaml" \
  --intent "${TMP_DIR}/intent_trace.json" \
  --key-mode prod \
  --private-key "${TRACE_PRIVATE_KEY}" \
  --trace-out "${TMP_DIR}/trace_original.json" \
  --json >"${TMP_DIR}/gate_trace.json"

python3 - <<'PY' "${TMP_DIR}/trace_original.json" "${TMP_DIR}/trace_tampered.json"
import json, sys
src = json.load(open(sys.argv[1], encoding="utf-8"))
src["context_set_digest"] = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
with open(sys.argv[2], "w", encoding="utf-8") as fh:
    json.dump(src, fh, indent=2)
    fh.write("\n")
PY

set +e
"${BIN_PATH}" trace verify "${TMP_DIR}/trace_tampered.json" --public-key "${TRACE_PUBLIC_KEY}" --json >"${TMP_DIR}/trace_verify_tampered.json"
TRACE_VERIFY_EXIT=$?
set -e
if [[ "${TRACE_VERIFY_EXIT}" -eq 0 ]]; then
  echo "error: expected tampered trace verification to fail" >&2
  exit 1
fi

echo "context chaos: pass"
