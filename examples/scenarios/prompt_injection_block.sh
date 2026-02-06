#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

set +e
out="$(./gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json 2>&1)"
code=$?
set -e

if [[ "$code" -ne 3 ]]; then
  echo "expected exit 3 for prompt injection block, got $code"
  echo "$out"
  exit 1
fi
if ! printf '%s' "$out" | grep -q '"verdict":"block"'; then
  echo "expected block verdict"
  echo "$out"
  exit 1
fi
if ! printf '%s' "$out" | grep -q 'blocked_prompt_injection'; then
  echo "expected blocked_prompt_injection reason"
  echo "$out"
  exit 1
fi

echo "prompt injection scenario: pass"
