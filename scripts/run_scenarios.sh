#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PRODUCT="${1:-}"

if [[ -z "$PRODUCT" ]]; then
  echo "usage: bash scripts/run_scenarios.sh <product>" >&2
  exit 1
fi
if [[ "$PRODUCT" != "gait" ]]; then
  echo "unsupported product: $PRODUCT (expected: gait)" >&2
  exit 1
fi

bash "$SCRIPT_DIR/validate_scenarios.sh"

BIN_DIR="$(mktemp -d)"
BIN_PATH="$BIN_DIR/gait"
OUT_FILE="$(mktemp)"

cleanup() {
  rm -f "$OUT_FILE"
  rm -rf "$BIN_DIR"
}
trap cleanup EXIT

(
  cd "$REPO_ROOT"
  go build -o "$BIN_PATH" ./cmd/gait
)

set +e
(
  cd "$REPO_ROOT"
  GAIT_SCENARIO_BIN="$BIN_PATH" go test ./internal/scenarios -count=1 -tags=scenario -v | tee "$OUT_FILE"
)
status=${PIPESTATUS[0]}
set -e

pass_count=$(grep -c -- '--- PASS: TestTier11Scenarios/' "$OUT_FILE" || true)
expected_count=13
if [[ "$status" -ne 0 ]]; then
  echo "scenario test execution failed for $PRODUCT" >&2
  exit "$status"
fi
if [[ "$pass_count" -ne "$expected_count" ]]; then
  echo "scenario pass-count mismatch: got=$pass_count expected=$expected_count" >&2
  exit 1
fi

echo "$PRODUCT scenarios: $pass_count/$expected_count passed"
