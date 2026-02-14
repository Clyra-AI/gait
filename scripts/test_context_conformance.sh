#!/usr/bin/env bash
set -euo pipefail

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
  BIN_PATH="$(pwd)/gait"
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "error: gait binary not executable: ${BIN_PATH}" >&2
  exit 2
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gait-context-conformance-XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT
OUT_DIR="${TMP_DIR}/out"
mkdir -p "${OUT_DIR}"
SOURCE_OUT_DIR="${OUT_DIR}/source"
CANDIDATE_OUT_DIR="${OUT_DIR}/candidate"
mkdir -p "${SOURCE_OUT_DIR}" "${CANDIDATE_OUT_DIR}"

create_inputs() {
  local envelope_path="$1"
  local retrieved_at="$2"
  local content_digest="$3"
  local run_id="$4"
  cat >"${envelope_path}" <<JSON
{
  "schema_id": "gait.context.envelope",
  "schema_version": "1.0.0",
  "created_at": "2026-02-14T00:00:00Z",
  "producer_version": "test-v2.5",
  "context_set_id": "ctx_set_conformance",
  "context_set_digest": "",
  "evidence_mode": "required",
  "records": [
    {
      "ref_id": "ctx_001",
      "source_type": "doc_store",
      "source_locator": "docs://policy/security",
      "query_digest": "1111111111111111111111111111111111111111111111111111111111111111",
      "content_digest": "${content_digest}",
      "retrieved_at": "${retrieved_at}",
      "redaction_mode": "reference",
      "immutability": "immutable",
      "freshness_sla_seconds": 600
    }
  ]
}
JSON
  cat >"${TMP_DIR}/run_record_${run_id}.json" <<JSON
{
  "run": {
    "schema_id": "gait.runpack.run",
    "schema_version": "1.0.0",
    "created_at": "2026-02-14T00:00:00Z",
    "producer_version": "test-v2.5",
    "run_id": "${run_id}",
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
    "run_id": "${run_id}",
    "receipts": []
  },
  "capture_mode": "reference"
}
JSON
}

create_inputs "${TMP_DIR}/env_source.json" "2026-02-14T00:00:00Z" "2222222222222222222222222222222222222222222222222222222222222222" "run_ctx_src"
create_inputs "${TMP_DIR}/env_candidate.json" "2026-02-14T00:05:00Z" "2222222222222222222222222222222222222222222222222222222222222222" "run_ctx_src"

"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record_run_ctx_src.json" \
  --out-dir "${SOURCE_OUT_DIR}" \
  --context-envelope "${TMP_DIR}/env_source.json" \
  --context-evidence-mode required \
  --json >"${TMP_DIR}/source_record.json"
SOURCE_RUNPACK="$(python3 - <<'PY' "${TMP_DIR}/source_record.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
print(payload["bundle"])
PY
)"

"${BIN_PATH}" run record \
  --input "${TMP_DIR}/run_record_run_ctx_src.json" \
  --out-dir "${CANDIDATE_OUT_DIR}" \
  --context-envelope "${TMP_DIR}/env_candidate.json" \
  --context-evidence-mode required \
  --json >"${TMP_DIR}/candidate_record.json"
CANDIDATE_RUNPACK="$(python3 - <<'PY' "${TMP_DIR}/candidate_record.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
print(payload["bundle"])
PY
)"

(cd "${TMP_DIR}" && "${BIN_PATH}" regress init --from "${SOURCE_RUNPACK}" --json >"${TMP_DIR}/regress_init.json")

python3 - <<'PY' "${TMP_DIR}/regress_init.json" "${CANDIDATE_RUNPACK}" "${TMP_DIR}"
import json, pathlib, sys
init_payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert init_payload.get("ok") is True, init_payload
fixture_dir = pathlib.Path(sys.argv[3]) / init_payload["fixture_dir"]
meta_path = fixture_dir / "fixture.json"
meta = json.loads(meta_path.read_text(encoding="utf-8"))
meta["candidate_runpack"] = sys.argv[2]
meta["diff_allow_changed_files"] = ["manifest.json", "refs.json"]
meta["context_conformance"] = "required"
meta["allow_context_runtime_drift"] = False
meta["expected_context_set_digest"] = ""
meta_path.write_text(json.dumps(meta, indent=2) + "\n", encoding="utf-8")
PY

set +e
(cd "${TMP_DIR}" && "${BIN_PATH}" regress run --config "${TMP_DIR}/gait.yaml" --output "${TMP_DIR}/regress_blocked.json" --context-conformance --json >"${TMP_DIR}/regress_blocked_stdout.json")
BLOCKED_EXIT=$?
set -e
if [[ "${BLOCKED_EXIT}" -eq 0 ]]; then
  echo "error: expected context conformance run to fail without allow-context-runtime-drift" >&2
  exit 1
fi

python3 - <<'PY' "${TMP_DIR}/regress_blocked_stdout.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is False, payload
assert payload.get("top_failure_reason") in {"context_runtime_drift_blocked", "context_semantic_drift"}, payload
PY

(cd "${TMP_DIR}" && "${BIN_PATH}" regress run --config "${TMP_DIR}/gait.yaml" --output "${TMP_DIR}/regress_allowed.json" --context-conformance --allow-context-runtime-drift --json >"${TMP_DIR}/regress_allowed_stdout.json")
python3 - <<'PY' "${TMP_DIR}/regress_allowed_stdout.json"
import json, sys
payload = json.load(open(sys.argv[1], encoding="utf-8"))
assert payload.get("ok") is True, payload
PY

echo "context conformance checks passed"
