#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

private_key_path="$repo_root/examples/scenarios/keys/approval_private.key"
tmp_dir="$repo_root/gait-out/scenarios"
mkdir -p "$tmp_dir"

set +e
initial_out="$(./gait gate eval \
  --policy examples/policy-test/require_approval.yaml \
  --intent examples/policy-test/intent.json \
  --key-mode prod \
  --private-key "$private_key_path" \
  --approval-private-key "$private_key_path" \
  --json 2>&1)"
initial_code=$?
set -e

if [[ "$initial_code" -ne 4 ]]; then
  echo "expected initial approval-required exit 4, got $initial_code"
  echo "$initial_out"
  exit 1
fi

intent_digest="$(printf '%s' "$initial_out" | sed -n 's/.*"intent_digest":"\([^"]*\)".*/\1/p')"
policy_digest="$(printf '%s' "$initial_out" | sed -n 's/.*"policy_digest":"\([^"]*\)".*/\1/p')"
if [[ -z "$intent_digest" || -z "$policy_digest" ]]; then
  echo "failed to parse digests from gate output"
  echo "$initial_out"
  exit 1
fi

./gait approve \
  --intent-digest "$intent_digest" \
  --policy-digest "$policy_digest" \
  --ttl 1h \
  --scope tool.write \
  --approver test@example.com \
  --reason-code scenario_scope_mismatch \
  --key-mode prod \
  --private-key "$private_key_path" \
  --out "$tmp_dir/token_bad_scope.json" \
  --json >/dev/null

set +e
scope_fail_out="$(./gait gate eval \
  --policy examples/policy-test/require_approval.yaml \
  --intent examples/policy-test/intent.json \
  --approval-token "$tmp_dir/token_bad_scope.json" \
  --key-mode prod \
  --private-key "$private_key_path" \
  --approval-private-key "$private_key_path" \
  --json 2>&1)"
scope_fail_code=$?
set -e

if [[ "$scope_fail_code" -ne 4 ]]; then
  echo "expected scope mismatch to remain blocked (exit 4), got $scope_fail_code"
  echo "$scope_fail_out"
  exit 1
fi
if ! printf '%s' "$scope_fail_out" | grep -q 'approval_token_scope_mismatch'; then
  echo "expected approval_token_scope_mismatch reason"
  echo "$scope_fail_out"
  exit 1
fi

./gait approve \
  --intent-digest "$intent_digest" \
  --policy-digest "$policy_digest" \
  --ttl 1h \
  --scope tool:tool.write \
  --approver test@example.com \
  --reason-code scenario_valid_approval \
  --key-mode prod \
  --private-key "$private_key_path" \
  --out "$tmp_dir/token_valid_scope.json" \
  --json >/dev/null

set +e
success_out="$(./gait gate eval \
  --policy examples/policy-test/require_approval.yaml \
  --intent examples/policy-test/intent.json \
  --approval-token "$tmp_dir/token_valid_scope.json" \
  --key-mode prod \
  --private-key "$private_key_path" \
  --approval-private-key "$private_key_path" \
  --json 2>&1)"
success_code=$?
set -e

if [[ "$success_code" -ne 0 ]]; then
  echo "expected approval success exit 0, got $success_code"
  echo "$success_out"
  exit 1
fi
if ! printf '%s' "$success_out" | grep -q '"verdict":"allow"'; then
  echo "expected allow verdict after valid approval"
  echo "$success_out"
  exit 1
fi
if ! printf '%s' "$success_out" | grep -q 'approval_granted'; then
  echo "expected approval_granted reason"
  echo "$success_out"
  exit 1
fi

echo "approval flow scenario: pass"
