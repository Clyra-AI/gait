#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SCENARIO_ROOT="$REPO_ROOT/scenarios/gait"

required_scenarios=(
  "approval-expiry-1s-past"
  "approval-token-valid"
  "approved-registry-signature-mismatch-high-risk"
  "concurrent-evaluation-10"
  "delegation-chain-depth-3"
  "dry-run-no-side-effects"
  "pack-integrity-round-trip"
  "policy-allow-safe-tools"
  "policy-block-destructive"
  "script-max-steps-exceeded"
  "script-mixed-risk-block"
  "script-threshold-approval-determinism"
  "wrkr-missing-fail-closed-high-risk"
)

if [[ ! -d "$SCENARIO_ROOT" ]]; then
  echo "scenario root missing: $SCENARIO_ROOT" >&2
  exit 1
fi

for scenario in "${required_scenarios[@]}"; do
  if [[ ! -d "$SCENARIO_ROOT/$scenario" ]]; then
    echo "missing scenario directory: $SCENARIO_ROOT/$scenario" >&2
    exit 1
  fi
done

(
  cd "$REPO_ROOT"
  go test ./internal/scenarios -run TestValidateScenarioFixtures -count=1
)

echo "validated ${#required_scenarios[@]} scenarios in $SCENARIO_ROOT"
