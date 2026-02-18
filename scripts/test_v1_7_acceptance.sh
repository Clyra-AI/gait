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

generate_private_key_file() {
  local out_path="$1"
  local helper_path="$WORK_DIR/generate_private_key.go"
  cat >"$helper_path" <<'EOF'
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: generate_private_key <out-path>")
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	encoded := base64.StdEncoding.EncodeToString(privateKey)
	if err := os.WriteFile(os.Args[1], []byte(encoded), 0o600); err != nil {
		panic(err)
	}
	fmt.Print(os.Args[1])
}
EOF
  (cd "$REPO_ROOT" && go run "$helper_path" "$out_path" >/dev/null)
}

generate_signed_skill_pack() {
  local manifest_path="$1"
  local public_key_path="$2"
  local helper_path="$WORK_DIR/generate_skill_pack.go"
  cat >"$helper_path" <<'EOF'
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: generate_skill_pack <manifest-path> <public-key-path>")
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		panic(err)
	}

	manifest := map[string]any{
		"schema_id":        "gait.registry.pack",
		"schema_version":   "1.0.0",
		"created_at":       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"producer_version": "0.0.0-v17",
		"pack_name":        "skill-guardrails",
		"pack_version":     "1.0.0",
		"pack_type":        "skill",
		"publisher":        "acme",
		"source":           "registry",
		"artifacts": []map[string]string{
			{"path": "skill.yaml", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		},
	}

	signableBytes, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}
	digest, err := jcs.DigestJCS(signableBytes)
	if err != nil {
		panic(err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		panic(err)
	}
	manifest["signatures"] = []map[string]string{{
		"alg":           signature.Alg,
		"key_id":        signature.KeyID,
		"sig":           signature.Sig,
		"signed_digest": signature.SignedDigest,
	}}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[1], manifestBytes, 0o600); err != nil {
		panic(err)
	}
}
EOF
  (cd "$REPO_ROOT" && go run "$helper_path" "$manifest_path" "$public_key_path")
}

echo "==> endpoint policy path"
"$BIN_PATH" policy test \
  "$REPO_ROOT/examples/policy/endpoint/allow_safe_endpoints.yaml" \
  "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_allow.json" \
  --json >"$WORK_DIR/endpoint_allow.json"
set +e
"$BIN_PATH" policy test \
  "$REPO_ROOT/examples/policy/endpoint/block_denied_endpoints.yaml" \
  "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_block.json" \
  --json >"$WORK_DIR/endpoint_block.json"
endpoint_block_code=$?
"$BIN_PATH" policy test \
  "$REPO_ROOT/examples/policy/endpoint/require_approval_destructive.yaml" \
  "$REPO_ROOT/examples/policy/endpoint/fixtures/intent_destructive.json" \
  --json >"$WORK_DIR/endpoint_approval.json"
endpoint_approval_code=$?
set -e
if [[ "$endpoint_block_code" -ne 3 ]]; then
  echo "endpoint block exit mismatch: $endpoint_block_code" >&2
  exit 1
fi
if [[ "$endpoint_approval_code" -ne 4 ]]; then
  echo "endpoint approval exit mismatch: $endpoint_approval_code" >&2
  exit 1
fi

echo "==> skill provenance verification path"
manifest_path="$WORK_DIR/registry_skill_pack.json"
public_key_path="$WORK_DIR/registry_skill_public.key"
cache_dir="$WORK_DIR/registry_cache"
generate_signed_skill_pack "$manifest_path" "$public_key_path"
"$BIN_PATH" registry install \
  --source "$manifest_path" \
  --cache-dir "$cache_dir" \
  --public-key "$public_key_path" \
  --json >"$WORK_DIR/registry_install.json"
metadata_path="$(python3 - "$WORK_DIR/registry_install.json" <<'PY'
import json
import sys
payload = json.loads(open(sys.argv[1], encoding="utf-8").read())
if not payload.get("ok"):
    raise SystemExit("registry install did not return ok=true")
metadata = payload.get("metadata_path", "")
if not metadata:
    raise SystemExit("registry install metadata_path missing")
print(metadata)
PY
)"
set +e
"$BIN_PATH" registry verify \
  --path "$metadata_path" \
  --cache-dir "$cache_dir" \
  --public-key "$public_key_path" \
  --publisher-allowlist trusted-inc \
  --report-out "$WORK_DIR/registry_verify_report.json" \
  --json >"$WORK_DIR/registry_verify.json"
registry_verify_code=$?
set -e
if [[ "$registry_verify_code" -ne 2 ]]; then
  echo "skill registry verify exit mismatch: $registry_verify_code" >&2
  exit 1
fi
python3 - "$WORK_DIR/registry_verify_report.json" <<'PY'
import json
import sys
payload = json.loads(open(sys.argv[1], encoding="utf-8").read())
if payload.get("status") != "fail":
    raise SystemExit("expected verification report status=fail")
if payload.get("publisher_allowed", True):
    raise SystemExit("expected publisher_allowed=false")
if not payload.get("signature_verified", False):
    raise SystemExit("expected signature_verified=true")
PY

echo "==> fail-closed matrix"
invalid_intent_path="$WORK_DIR/intent_invalid.json"
printf '%s\n' '{}' >"$invalid_intent_path"
set +e
(
  cd "$WORK_DIR"
  "$BIN_PATH" gate eval \
    --policy "$REPO_ROOT/examples/policy-test/allow.yaml" \
    --intent "$invalid_intent_path" \
    --json >fail_closed_invalid_intent.json
)
invalid_intent_code=$?
set -e
if [[ "$invalid_intent_code" -ne 6 ]]; then
  echo "invalid intent fail-closed exit mismatch: $invalid_intent_code" >&2
  exit 1
fi

private_key_path="$WORK_DIR/trace_private.key"
generate_private_key_file "$private_key_path"

cat >"$WORK_DIR/high_risk_no_broker.yaml" <<'YAML'
default_verdict: allow
rules:
  - name: high-risk-allow-without-broker
    effect: allow
    match:
      tool_names: [tool.delete]
      risk_classes: [high]
YAML
cat >"$WORK_DIR/high_risk_intent.json" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-09T00:00:00Z",
  "producer_version": "0.0.0-v17",
  "tool_name": "tool.delete",
  "args": {"path": "/tmp/v17-delete.txt"},
  "targets": [
    {
      "kind": "path",
      "value": "/tmp/v17-delete.txt",
      "operation": "delete",
      "endpoint_class": "fs.delete",
      "destructive": true
    }
  ],
  "arg_provenance": [{"arg_path":"$.path","source":"user"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}
JSON
set +e
(
  cd "$WORK_DIR"
  "$BIN_PATH" gate eval \
    --policy "$WORK_DIR/high_risk_no_broker.yaml" \
    --intent "$WORK_DIR/high_risk_intent.json" \
    --profile oss-prod \
    --key-mode prod \
    --private-key "$private_key_path" \
    --json >fail_closed_missing_signals.json
)
missing_signals_code=$?
set -e
if [[ "$missing_signals_code" -ne 6 ]]; then
  echo "high-risk missing policy signal exit mismatch: $missing_signals_code" >&2
  exit 1
fi
if ! grep -q "require_broker_credential" "$WORK_DIR/fail_closed_missing_signals.json"; then
  echo "missing expected broker precondition reason in high-risk output" >&2
  exit 1
fi

cat >"$WORK_DIR/high_risk_with_broker.yaml" <<'YAML'
default_verdict: allow
rules:
  - name: high-risk-allow-with-broker
    effect: allow
    require_broker_credential: true
    broker_reference: egress
    broker_scopes: [export]
    match:
      tool_names: [tool.write]
      risk_classes: [high]
YAML
cat >"$WORK_DIR/high_risk_write_intent.json" <<'JSON'
{
  "schema_id": "gait.gate.intent_request",
  "schema_version": "1.0.0",
  "created_at": "2026-02-09T00:00:00Z",
  "producer_version": "0.0.0-v17",
  "tool_name": "tool.write",
  "args": {"path": "/tmp/v17-write.txt"},
  "targets": [
    {
      "kind": "path",
      "value": "/tmp/v17-write.txt",
      "operation": "write",
      "endpoint_class": "fs.write"
    }
  ],
  "arg_provenance": [{"arg_path":"$.path","source":"user"}],
  "context": {"identity":"alice","workspace":"/repo/gait","risk_class":"high"}
}
JSON
broker_fail_path="$WORK_DIR/broker_fail.sh"
broker_fail_script="#!/bin/sh\necho 'forced failure token=secret-broker-token' 1>&2\nexit 2\n"
if [[ "$(uname -s)" = "MINGW"* || "$(uname -s)" = "MSYS"* || "$(uname -s)" = "CYGWIN"* ]]; then
  broker_fail_path="$WORK_DIR/broker_fail.cmd"
  broker_fail_script="@echo forced failure token=secret-broker-token 1>&2\r\n@exit /b 2\r\n"
fi
printf "%b" "$broker_fail_script" >"$broker_fail_path"
chmod 700 "$broker_fail_path" 2>/dev/null || true
(
  set +e
  cd "$WORK_DIR"
  "$BIN_PATH" gate eval \
    --policy "$WORK_DIR/high_risk_with_broker.yaml" \
    --intent "$WORK_DIR/high_risk_write_intent.json" \
    --profile oss-prod \
    --key-mode prod \
    --private-key "$private_key_path" \
    --credential-broker command \
    --credential-command "$broker_fail_path" \
    --credential-evidence-out "$WORK_DIR/credential_evidence.json" \
    --json >fail_closed_broker.json
  broker_fail_code=$?
  set -e
  if [[ "$broker_fail_code" -ne 3 ]]; then
    echo "broker fail-closed exit mismatch: $broker_fail_code" >&2
    exit 1
  fi
)
if grep -q "secret-broker-token" "$WORK_DIR/fail_closed_broker.json"; then
  echo "broker failure output leaked secret token" >&2
  exit 1
fi
if ! grep -q '"verdict":"block"' "$WORK_DIR/fail_closed_broker.json"; then
  echo "broker failure expected block verdict" >&2
  exit 1
fi
if ! grep -q "broker_credential_missing" "$WORK_DIR/fail_closed_broker.json"; then
  echo "broker failure expected broker_credential_missing reason" >&2
  exit 1
fi

echo "==> local signal + receipt continuity"
pushd "$WORK_DIR" >/dev/null
"$BIN_PATH" demo >demo.txt
"$BIN_PATH" verify run_demo --json >verify.json
"$BIN_PATH" run receipt --from run_demo --json >receipt.json
python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("receipt.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("receipt output not ok")
footer = payload.get("ticket_footer", "")
if not footer.startswith("GAIT run_id="):
    raise SystemExit(f"unexpected ticket footer: {footer}")
PY
"$BIN_PATH" regress bootstrap --from run_demo --json >regress_bootstrap.json
"$BIN_PATH" scout signal \
  --runs "$WORK_DIR/gait-out/runpack_run_demo.zip,$WORK_DIR/fixtures/run_demo/runpack.zip" \
  --regress "$WORK_DIR/regress_result.json" \
  --json >signal.json
python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("signal.json").read_text(encoding="utf-8"))
if not payload.get("ok"):
    raise SystemExit("signal output not ok")
report = payload.get("report")
if not isinstance(report, dict):
    raise SystemExit("signal report missing")
if report.get("schema_id") != "gait.scout.signal_report":
    raise SystemExit("unexpected signal schema_id")
if report.get("family_count", 0) < 1:
    raise SystemExit("signal family_count must be >= 1")
PY
popd >/dev/null

echo "==> runtime SLO budget gate"
python3 "$REPO_ROOT/scripts/check_command_budgets.py" \
  "$BIN_PATH" \
  "$WORK_DIR/command_budget_report.json" \
  "$REPO_ROOT/perf/runtime_slo_budgets.json"

echo "v1.7 acceptance checks passed"
