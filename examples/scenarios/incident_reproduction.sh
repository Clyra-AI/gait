#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

./gait demo >/dev/null

init_out="$(./gait regress init --from run_demo --json)"
if ! printf '%s' "$init_out" | grep -q '"ok":true'; then
  echo "regress init did not report ok=true"
  exit 1
fi

run_out="$(./gait regress run --json)"
if ! printf '%s' "$run_out" | grep -q '"ok":true'; then
  echo "regress run did not report ok=true"
  exit 1
fi

test -f gait.yaml
test -f fixtures/run_demo/runpack.zip
test -f regress_result.json

echo "incident reproduction scenario: pass"
