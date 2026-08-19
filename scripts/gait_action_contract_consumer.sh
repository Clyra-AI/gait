#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo '{"consumer":"gait","version":"unknown","scenario_id":"unknown","artifact_sha256":"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","status":"reject","self_attestation":false,"schema_versions":{"artifact":"1","contract":"3"},"supported_constraints":{},"semantic_result":{"proposal_valid":false,"activation_ready":false,"execution_claim":false,"effect_claim":false,"reason_codes":["explicit_selection_required"]}}'
  exit 6
fi

if [ -n "${GAIT_BIN:-}" ]; then
  exec "$GAIT_BIN" contract consume "$1"
fi

if command -v gait >/dev/null 2>&1; then
  exec gait contract consume "$1"
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ -x "$repo_root/gait" ]; then
  exec "$repo_root/gait" contract consume "$1"
fi

echo '{"consumer":"gait","version":"unknown","scenario_id":"unknown","artifact_sha256":"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","status":"dependency_missing","self_attestation":false,"schema_versions":{"artifact":"1","contract":"3"},"supported_constraints":{},"semantic_result":{"proposal_valid":false,"activation_ready":false,"execution_claim":false,"effect_claim":false,"reason_codes":["consumer_binary_missing"]}}'
exit 7
