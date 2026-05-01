#!/usr/bin/env bash
set -euo pipefail

mkdir -p gait-out
summary_path="gait-out/policy_compliance_summary.md"
results_path="gait-out/policy_compliance_results.jsonl"

cat >"$summary_path" <<'EOF'
# Policy Compliance Summary

| Case | Expected Exit | Actual Exit | Signal | Detail | Result |
| --- | --- | --- | --- | --- | --- |
EOF

: >"$results_path"

run_case() {
  local case_name="$1"
  local policy_path="$2"
  local intent_path="$3"
  local expected_exit="$4"

  local output
  local actual_exit
  set +e
  output="$(./gait policy test "$policy_path" "$intent_path" --json 2>&1)"
  actual_exit=$?
  set -e

  printf '%s\n' "$output" >>"$results_path"

  local verdict
  verdict="$(printf '%s' "$output" | sed -n 's/.*"verdict":"\([^"]*\)".*/\1/p')"
  if [[ -z "$verdict" ]]; then
    verdict="unknown"
  fi

  local detail
  detail="$(printf '%s' "$output" | sed -n 's/.*"reason_codes":\[\([^]]*\)\].*/\1/p')"
  if [[ -z "$detail" ]]; then
    detail="none"
  fi

  local result
  if [[ "$actual_exit" -eq "$expected_exit" ]]; then
    result="pass"
  else
    result="fail"
    failed_cases=1
  fi

  printf '| %s | `%s` | `%s` | `%s` | `%s` | **%s** |\n' \
    "$case_name" "$expected_exit" "$actual_exit" "$verdict" "$detail" "$result" >>"$summary_path"
}

run_validate_case() {
  local case_name="$1"
  local policy_path="$2"
  local expected_exit="$3"

  local output
  local actual_exit
  set +e
  output="$(./gait policy validate "$policy_path" --json 2>&1)"
  actual_exit=$?
  set -e

  printf '%s\n' "$output" >>"$results_path"

  local digest
  digest="$(printf '%s' "$output" | sed -n 's/.*"policy_digest":"\([^"]*\)".*/\1/p')"
  if [[ -z "$digest" ]]; then
    digest="none"
  fi

  local result
  if [[ "$actual_exit" -eq "$expected_exit" ]]; then
    result="pass"
  else
    result="fail"
    failed_cases=1
  fi

  printf '| %s | `%s` | `%s` | `%s` | `%s` | **%s** |\n' \
    "$case_name" "$expected_exit" "$actual_exit" "validate" "$digest" "$result" >>"$summary_path"
}

run_fmt_idempotence_case() {
  local case_name="$1"
  local source_policy="$2"

  local tmp_policy="gait-out/policy_compliance_fmt.yaml"
  cp "$source_policy" "$tmp_policy"

  local first_output second_output
  local first_exit second_exit
  set +e
  first_output="$(./gait policy fmt "$tmp_policy" --write --json 2>&1)"
  first_exit=$?
  second_output="$(./gait policy fmt "$tmp_policy" --write --json 2>&1)"
  second_exit=$?
  set -e

  printf '%s\n' "$first_output" >>"$results_path"
  printf '%s\n' "$second_output" >>"$results_path"

  local second_changed
  second_changed="$(printf '%s' "$second_output" | sed -n 's/.*"changed":\([^,}]*\).*/\1/p')"
  if [[ -z "$second_changed" ]]; then
    second_changed="false"
  fi

  local result="pass"
  if [[ "$first_exit" -ne 0 || "$second_exit" -ne 0 || "$second_changed" != "false" ]]; then
    result="fail"
    failed_cases=1
  fi

  local actual_exit="$second_exit"
  if [[ "$result" == "fail" && "$first_exit" -ne 0 ]]; then
    actual_exit="$first_exit"
  fi

  printf '| %s | `%s` | `%s` | `%s` | `%s` | **%s** |\n' \
    "$case_name" "0" "$actual_exit" "fmt" "second_changed=${second_changed}" "$result" >>"$summary_path"
}

failed_cases=0

go build -o ./gait ./cmd/gait

run_case "baseline-allow" "examples/policy-test/allow.yaml" "examples/policy-test/intent.json" 0
run_case "baseline-block" "examples/policy-test/block.yaml" "examples/policy-test/intent.json" 3
run_case "baseline-require-approval" "examples/policy-test/require_approval.yaml" "examples/policy-test/intent.json" 4
run_case "prompt-injection-block" "examples/prompt-injection/policy.yaml" "examples/prompt-injection/intent_injected.json" 3
run_case "template-low-read" "examples/policy/base_low_risk.yaml" "examples/policy/intents/intent_read.json" 0
run_case "template-medium-write" "examples/policy/base_medium_risk.yaml" "examples/policy/intents/intent_write.json" 4
run_case "template-high-delete" "examples/policy/base_high_risk.yaml" "examples/policy/intents/intent_delete.json" 3
run_case "template-high-tainted-egress" "examples/policy/base_high_risk.yaml" "examples/policy/intents/intent_tainted_egress.json" 3
run_case "template-high-delegated-egress-valid" "examples/policy/base_high_risk.yaml" "examples/policy/intents/intent_delegated_egress_valid.json" 0
run_case "template-high-delegated-egress-invalid" "examples/policy/base_high_risk.yaml" "examples/policy/intents/intent_delegated_egress_invalid.json" 3
run_case "ci-cd-read-artifact" "examples/policy/ci_cd_pipeline.yaml" "examples/policy/intents/intent_ci_read_artifact.json" 0
run_case "ci-cd-deploy-staging" "examples/policy/ci_cd_pipeline.yaml" "examples/policy/intents/intent_ci_deploy_staging.json" 0
run_case "ci-cd-deploy-production" "examples/policy/ci_cd_pipeline.yaml" "examples/policy/intents/intent_ci_deploy_production.json" 3
run_case "ci-cd-secret-access" "examples/policy/ci_cd_pipeline.yaml" "examples/policy/intents/intent_ci_secret_access.json" 4
run_case "ci-cd-infra-destroy" "examples/policy/ci_cd_pipeline.yaml" "examples/policy/intents/intent_ci_infra_destroy.json" 3
run_validate_case "validate-template-low" "examples/policy/base_low_risk.yaml" 0
run_validate_case "validate-template-medium" "examples/policy/base_medium_risk.yaml" 0
run_validate_case "validate-template-high" "examples/policy/base_high_risk.yaml" 0
run_validate_case "validate-ci-cd-pipeline" "examples/policy/ci_cd_pipeline.yaml" 0

invalid_policy_path="gait-out/policy_invalid.yaml"
cat >"$invalid_policy_path" <<'EOF'
default_verdit: allow
EOF
run_validate_case "validate-invalid-unknown-field" "$invalid_policy_path" 6
run_fmt_idempotence_case "fmt-idempotence-template-medium" "examples/policy/base_medium_risk.yaml"

cat "$summary_path"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  cat "$summary_path" >>"$GITHUB_STEP_SUMMARY"
fi

if [[ "$failed_cases" -ne 0 ]]; then
  echo "policy compliance checks failed"
  exit 1
fi

echo "policy compliance checks passed"
