#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

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

echo "==> generate v1.7 artifacts for consumer compatibility"
(
  cd "$WORK_DIR"

  "$BIN_PATH" demo --json > demo.json

  "$BIN_PATH" gate eval \
    --policy "$REPO_ROOT/examples/policy-test/allow.yaml" \
    --intent "$REPO_ROOT/core/schema/testdata/gate_intent_request_skill_valid.json" \
    --trace-out "$WORK_DIR/trace_v17.json" \
    --json > gate_eval.json

  "$BIN_PATH" regress bootstrap \
    --from run_demo \
    --output "$WORK_DIR/regress_result.json" \
    --junit "$WORK_DIR/junit.xml" \
    --json > regress_bootstrap.json

  "$BIN_PATH" scout signal \
    --runs run_demo \
    --traces "$WORK_DIR/trace_v17.json" \
    --regress "$WORK_DIR/regress_result.json" \
    --out "$WORK_DIR/scout_signal_report.json" \
    --json > scout_signal.json

  "$BIN_PATH" policy test \
    "$REPO_ROOT/examples/policy/endpoint/allow_safe_endpoints.yaml" \
    "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_allow.json" \
    --json > endpoint_allow.json

  set +e
  "$BIN_PATH" policy test \
    "$REPO_ROOT/examples/policy/endpoint/block_denied_endpoints.yaml" \
    "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_block.json" \
    --json > endpoint_block.json
  endpoint_block_code=$?
  "$BIN_PATH" policy test \
    "$REPO_ROOT/examples/policy/endpoint/require_approval_destructive.yaml" \
    "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_destructive.json" \
    --json > endpoint_approval.json
  endpoint_approval_code=$?
  set -e

  if [[ "$endpoint_block_code" -ne 3 ]]; then
    echo "endpoint block fixture exit mismatch: $endpoint_block_code" >&2
    exit 1
  fi
  if [[ "$endpoint_approval_code" -ne 4 ]]; then
    echo "endpoint approval fixture exit mismatch: $endpoint_approval_code" >&2
    exit 1
  fi
)

echo "==> enterprise consumer ingest contract"
python3 - "$WORK_DIR" <<'PY'
import hashlib
import json
import sys
import zipfile
from pathlib import Path

work = Path(sys.argv[1])
runpack_path = work / "gait-out" / "runpack_run_demo.zip"
trace_path = work / "trace_v17.json"
regress_path = work / "regress_result.json"
signal_path = work / "scout_signal_report.json"
endpoint_allow_path = work / "endpoint_allow.json"
endpoint_block_path = work / "endpoint_block.json"
endpoint_approval_path = work / "endpoint_approval.json"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def load_jsonl(raw: bytes) -> list[dict]:
    rows: list[dict] = []
    for line in raw.decode("utf-8").splitlines():
        if line.strip():
            rows.append(json.loads(line))
    return rows


def parse_consumer_projection(
    manifest: dict,
    run_record: dict,
    intents: list[dict],
    results: list[dict],
    refs: dict,
    trace_record: dict,
    regress_result: dict,
    signal_report: dict,
    endpoint_allow: dict,
    endpoint_block: dict,
    endpoint_approval: dict,
) -> dict:
    skill_provenance = trace_record.get("skill_provenance") or {}
    signal_families = signal_report.get("families") or []
    top_issues = signal_report.get("top_issues") or []
    endpoint_reasons = sorted(
        {
            *endpoint_allow.get("reason_codes", []),
            *endpoint_block.get("reason_codes", []),
            *endpoint_approval.get("reason_codes", []),
        }
    )
    projection = {
        "run_id": manifest["run_id"],
        "runpack_manifest_digest": manifest["manifest_digest"],
        "capture_mode": manifest["capture_mode"],
        "intent_count": len(intents),
        "result_count": len(results),
        "receipt_count": len((refs.get("receipts") or [])),
        "trace_id": trace_record["trace_id"],
        "trace_verdict": trace_record["verdict"],
        "trace_skill_source": skill_provenance.get("source", ""),
        "trace_skill_publisher": skill_provenance.get("publisher", ""),
        "endpoint_verdicts": [
            endpoint_allow.get("verdict", ""),
            endpoint_block.get("verdict", ""),
            endpoint_approval.get("verdict", ""),
        ],
        "endpoint_reason_codes": endpoint_reasons,
        "regress_status": regress_result["status"],
        "regress_fixture_set": regress_result["fixture_set"],
        "signal_family_count": len(signal_families),
        "signal_top_issue_count": len(top_issues),
        "signal_primary_reason": (top_issues[0].get("reason_code", "") if top_issues else ""),
        "producer_versions": sorted(
            {
                manifest["producer_version"],
                run_record["producer_version"],
                trace_record["producer_version"],
                regress_result["producer_version"],
                signal_report["producer_version"],
            }
        ),
    }
    encoded = json.dumps(projection, sort_keys=True, separators=(",", ":")).encode("utf-8")
    projection["projection_digest"] = hashlib.sha256(encoded).hexdigest()
    return projection


with zipfile.ZipFile(runpack_path, "r") as archive:
    required_members = {"manifest.json", "run.json", "intents.jsonl", "results.jsonl", "refs.json"}
    members = set(archive.namelist())
    missing = sorted(required_members.difference(members))
    require(not missing, f"runpack missing required members: {missing}")

    manifest = json.loads(archive.read("manifest.json").decode("utf-8"))
    run_record = json.loads(archive.read("run.json").decode("utf-8"))
    intents = load_jsonl(archive.read("intents.jsonl"))
    results = load_jsonl(archive.read("results.jsonl"))
    refs = json.loads(archive.read("refs.json").decode("utf-8"))

trace_record = load_json(trace_path)
regress_result = load_json(regress_path)
signal_report = load_json(signal_path)
endpoint_allow = load_json(endpoint_allow_path)
endpoint_block = load_json(endpoint_block_path)
endpoint_approval = load_json(endpoint_approval_path)

require(manifest.get("schema_id") == "gait.runpack.manifest", "unexpected runpack manifest schema_id")
require(run_record.get("schema_id") == "gait.runpack.run", "unexpected run schema_id")
require(all(row.get("schema_id") == "gait.runpack.intent" for row in intents), "unexpected intent schema_id")
require(all(row.get("schema_id") == "gait.runpack.result" for row in results), "unexpected result schema_id")
require(refs.get("schema_id") == "gait.runpack.refs", "unexpected refs schema_id")
require(trace_record.get("schema_id") == "gait.gate.trace", "unexpected trace schema_id")
require(regress_result.get("schema_id") == "gait.regress.result", "unexpected regress schema_id")
require(signal_report.get("schema_id") == "gait.scout.signal_report", "unexpected signal schema_id")
require(endpoint_allow.get("schema_id") == "gait.policytest.result", "unexpected endpoint allow schema_id")
require(endpoint_block.get("schema_id") == "gait.policytest.result", "unexpected endpoint block schema_id")
require(endpoint_approval.get("schema_id") == "gait.policytest.result", "unexpected endpoint approval schema_id")
require(trace_record.get("skill_provenance", {}).get("source") == "registry", "expected skill provenance source=registry")
require(endpoint_allow.get("verdict") == "allow", "expected endpoint allow verdict")
require(endpoint_block.get("verdict") == "block", "expected endpoint block verdict")
require(endpoint_approval.get("verdict") == "require_approval", "expected endpoint approval verdict")
require("endpoint_path_denied" in (endpoint_block.get("reason_codes") or []), "missing endpoint_path_denied reason code")
require("endpoint_destructive_operation" in (endpoint_approval.get("reason_codes") or []), "missing endpoint_destructive_operation reason code")

# Enterprise consumers must tolerate additive unknown fields.
trace_with_extension = dict(trace_record)
trace_with_extension["enterprise_extension"] = {"opaque": True}
regress_with_extension = dict(regress_result)
regress_with_extension["enterprise_extension"] = "v2"
signal_with_extension = dict(signal_report)
signal_with_extension["enterprise_extension"] = [1, 2, 3]
endpoint_allow_with_extension = dict(endpoint_allow)
endpoint_allow_with_extension["enterprise_extension"] = True
endpoint_block_with_extension = dict(endpoint_block)
endpoint_block_with_extension["enterprise_extension"] = "opaque"
endpoint_approval_with_extension = dict(endpoint_approval)
endpoint_approval_with_extension["enterprise_extension"] = {"k": "v"}

projection_a = parse_consumer_projection(
    manifest,
    run_record,
    intents,
    results,
    refs,
    trace_record,
    regress_result,
    signal_report,
    endpoint_allow,
    endpoint_block,
    endpoint_approval,
)
projection_b = parse_consumer_projection(
    manifest,
    run_record,
    intents,
    results,
    refs,
    trace_with_extension,
    regress_with_extension,
    signal_with_extension,
    endpoint_allow_with_extension,
    endpoint_block_with_extension,
    endpoint_approval_with_extension,
)
require(projection_a == projection_b, "consumer projection changed under additive fields")

projection_path = work / "ent_consumer_projection.json"
projection_path.write_text(json.dumps(projection_a, indent=2, sort_keys=True) + "\n", encoding="utf-8")
print(f"consumer_projection={projection_path}")
print(f"projection_digest={projection_a['projection_digest']}")
PY

echo "ent consumer contract: pass"
