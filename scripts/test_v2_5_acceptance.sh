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

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gait-v2-5-acceptance-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

OUT_DIR="${TMP_DIR}/out"
mkdir -p "${OUT_DIR}"

cat >"${TMP_DIR}/context_envelope.json" <<'JSON'
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_acceptance",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "2222222222222222222222222222222222222222222222222222222222222222",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "reference",
      "immutability": "immutable",
      "freshness_sla_seconds": 600
    }
  ]
}
JSON

cat >"${TMP_DIR}/run_record.json" <<'JSON'
{
  "run": {
    "schema_id": "gait.runpack.run",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "test-v2.5",
    "run_id": "run_v25_acceptance",
    "env": {
      "os": "darwin",
      "arch": "arm64",
      "runtime": "go"
    },
    "timeline": [
      {"event":"run_started","ts":"2026-02-14T00:00:00Z"}
    ]
  },
  "intents": [
    {
      "schema_id": "gait.runpack.intent",
      "schema_version": "1.0.0",
      "created_at": "2026-02-14T00:00:00Z",
      "producer_version": "test-v2.5",
      "run_id": "run_v25_acceptance",
      "intent_id": "intent_0001",
      "tool_name": "tool.demo",
      "args_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  ],
  "results": [
    {
      "schema_id": "gait.runpack.result",
      "schema_version": "1.0.0",
      "created_at": "2026-02-14T00:00:00Z",
      "producer_version": "test-v2.5",
      "run_id": "run_v25_acceptance",
      "intent_id": "intent_0001",
      "status": "ok",
      "result_digest": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  ],
  "refs": {
    "schema_id": "gait.runpack.refs",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "test-v2.5",
    "run_id": "run_v25_acceptance",
    "receipts": []
  },
  "capture_mode": "reference"
}
JSON

echo "==> v2.5: required context evidence capture"
"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record.json" \
  --out-dir "${OUT_DIR}" \
  --context-envelope "${TMP_DIR}/context_envelope.json" \
  --context-evidence-mode required \
  --json >"${TMP_DIR}/record.json"

RUNPACK_PATH="$(python3 - <<'PY' "${TMP_DIR}/record.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
print(payload["bundle"])
PY
)"

echo "==> v2.5: best-effort mode without envelope"
"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record.json" \
  --out-dir "${OUT_DIR}" \
  --run-id run_v25_best_effort \
  --context-evidence-mode best_effort \
  --json >"${TMP_DIR}/record_best_effort.json"

python3 - <<'PY' "${TMP_DIR}/record_best_effort.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
PY

echo "==> v2.5: required mode fails when evidence missing"
set +e
"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record.json" \
  --out-dir "${OUT_DIR}" \
  --run-id run_v25_required_missing \
  --context-evidence-mode required \
  --json >"${TMP_DIR}/record_required_missing.json"
MISSING_EXIT=$?
set -e
if [[ "${MISSING_EXIT}" -eq 0 ]]; then
  echo "error: expected required mode without context evidence to fail" >&2
  exit 1
fi

echo "==> v2.5: unsafe raw context gate"
cat >"${TMP_DIR}/context_envelope_raw.json" <<'JSON'
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_acceptance_raw",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_raw_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security/raw",
      "query_digest": "3333333333333333333333333333333333333333333333333333333333333333",
      "content_digest": "4444444444444444444444444444444444444444444444444444444444444444",
      "retrieved_at": "2026-02-14T00:00:00Z",
      "redaction_mode": "raw",
      "immutability": "immutable"
    }
  ]
}
JSON

set +e
"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record.json" \
  --out-dir "${OUT_DIR}" \
  --run-id run_v25_raw_blocked \
  --context-envelope "${TMP_DIR}/context_envelope_raw.json" \
  --context-evidence-mode required \
  --json >"${TMP_DIR}/record_raw_blocked.json"
RAW_BLOCKED_EXIT=$?
set -e
if [[ "${RAW_BLOCKED_EXIT}" -eq 0 ]]; then
  echo "error: expected raw context envelope without unsafe flag to fail" >&2
  exit 1
fi
"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record.json" \
  --out-dir "${OUT_DIR}" \
  --run-id run_v25_raw_allowed \
  --context-envelope "${TMP_DIR}/context_envelope_raw.json" \
  --context-evidence-mode required \
  --unsafe-context-raw \
  --json >"${TMP_DIR}/record_raw_allowed.json"

echo "==> v2.5: verify and pack lifecycle"
"${BIN_PATH}" verify "${RUNPACK_PATH}" --json >"${TMP_DIR}/verify.json"
python3 - <<'PY' "${TMP_DIR}/verify.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
PY

"${BIN_PATH}" pack build --type run --from "${RUNPACK_PATH}" --out "${TMP_DIR}/pack.zip" --json >"${TMP_DIR}/pack_build.json"
"${BIN_PATH}" pack inspect --path "${TMP_DIR}/pack.zip" --json >"${TMP_DIR}/pack_inspect.json"
"${BIN_PATH}" pack diff "${TMP_DIR}/pack.zip" "${TMP_DIR}/pack.zip" --json >"${TMP_DIR}/pack_diff.json"

CONTEXT_DIGEST="$(python3 - <<'PY' "${TMP_DIR}/pack_inspect.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
inspect = payload.get("inspect") or {}
run_payload = inspect.get("run_payload") or {}
assert run_payload.get("context_set_digest"), payload
manifest = inspect.get("manifest") or {}
contents = [item.get("path") for item in (manifest.get("contents") or [])]
assert "context_envelope.json" in contents, contents
print(run_payload["context_set_digest"])
PY
)"

python3 - <<'PY' "${TMP_DIR}/pack_diff.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
diff_payload = payload.get("diff") or {}
result = diff_payload.get("result") or diff_payload.get("Result") or {}
summary = result.get("summary") or {}
assert summary.get("context_drift_classification") in {"none", ""}, summary
PY

echo "==> v2.5: fail-closed policy evaluation"
cat >"${TMP_DIR}/policy.yaml" <<'YAML'
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: require_context
    priority: 1
    effect: allow
    match:
      risk_classes: [high]
    require_context_evidence: true
    required_context_evidence_mode: required
YAML

cat >"${TMP_DIR}/intent_missing.json" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "tool_name": "tool.demo",
  "args": {"x":"y"},
  "targets": [{"kind":"path","value":"/tmp/demo","operation":"write"}],
  "context": {"identity":"u_demo","workspace":"/tmp","risk_class":"high"}
}
JSON

"${BIN_PATH}" gate eval --policy "${TMP_DIR}/policy.yaml" --intent "${TMP_DIR}/intent_missing.json" --json >"${TMP_DIR}/gate_missing.json"
python3 - <<'PY' "${TMP_DIR}/gate_missing.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("verdict") == "block", payload
reasons = set(payload.get("reason_codes") or [])
assert "context_evidence_missing" in reasons, reasons
PY

cat >"${TMP_DIR}/intent_present.json" <<JSON
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
    "context_set_digest":"${CONTEXT_DIGEST}",
    "context_evidence_mode":"required",
    "context_refs":["ctx_001"]
  }
}
JSON

"${BIN_PATH}" gate eval --policy "${TMP_DIR}/policy.yaml" --intent "${TMP_DIR}/intent_present.json" --json >"${TMP_DIR}/gate_present.json"
python3 - <<'PY' "${TMP_DIR}/gate_present.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("verdict") == "allow", payload
PY

echo "==> v2.5: regression context conformance"
(cd "${TMP_DIR}" && "${BIN_PATH}" regress bootstrap --from "${RUNPACK_PATH}" --name v25_context_bootstrap --json >"${TMP_DIR}/regress_bootstrap.json")
(cd "${TMP_DIR}" && "${BIN_PATH}" regress run --context-conformance --json >"${TMP_DIR}/regress_run.json")
python3 - <<'PY' "${TMP_DIR}/regress_run.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
assert payload.get("status") == "pass", payload
PY

echo "==> v2.5: doctor diagnostics include context-proof schema checks"
"${BIN_PATH}" doctor --workdir "${REPO_ROOT}" --output-dir "${TMP_DIR}/doctor_out" --json >"${TMP_DIR}/doctor.json"
python3 - <<'PY' "${TMP_DIR}/doctor.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
checks = payload.get("checks") or []
schema_checks = [c for c in checks if c.get("name") == "schema_files"]
assert schema_checks, payload
assert schema_checks[0].get("status") == "pass", schema_checks[0]
PY

echo "v2.5 acceptance: pass"
