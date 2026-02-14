#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

assert_file() {
  local path="$1"
  if [[ ! -f "${path}" ]]; then
    echo "missing required doc: ${path}" >&2
    exit 2
  fi
}

assert_pattern() {
  local path="$1"
  local pattern="$2"
  local reason="$3"
  if ! rg -q "${pattern}" "${path}"; then
    echo "missing pattern (${reason}) in ${path}: ${pattern}" >&2
    exit 2
  fi
}

assert_file "${REPO_ROOT}/README.md"
assert_file "${REPO_ROOT}/docs/architecture.md"
assert_file "${REPO_ROOT}/docs/flows.md"
assert_file "${REPO_ROOT}/docs/integration_checklist.md"
assert_file "${REPO_ROOT}/docs/ui_localhost.md"
assert_file "${REPO_ROOT}/docs/approval_runbook.md"
assert_file "${REPO_ROOT}/docs/policy_rollout.md"
assert_file "${REPO_ROOT}/docs/concepts/mental_model.md"
assert_file "${REPO_ROOT}/docs/sdk/python.md"
assert_file "${REPO_ROOT}/docs/agent_integration_boundary.md"
assert_file "${REPO_ROOT}/docs/mcp_capability_matrix.md"
assert_file "${REPO_ROOT}/docs/scenarios/simple_agent_tool_boundary.md"
assert_file "${REPO_ROOT}/docs/demo_output_legend.md"

assert_pattern "${REPO_ROOT}/README.md" "## In Plain Language" "C-01 plain language"
assert_pattern "${REPO_ROOT}/README.md" "Managed/preloaded agent note" "C-02 managed boundary"
assert_pattern "${REPO_ROOT}/README.md" "Simple End-To-End Scenario" "C-13 hero scenario"
assert_pattern "${REPO_ROOT}/README.md" "Fast 20-Second Proof" "C-13 legacy asset retained"
assert_pattern "${REPO_ROOT}/README.md" "examples/integrations/openai_agents/quickstart.py" "C-15 promoted agent context"

assert_pattern "${REPO_ROOT}/docs/architecture.md" "## Integration-First Architecture" "C-03 architecture rewrite"
assert_pattern "${REPO_ROOT}/docs/architecture.md" "Not evidence that Go Core is \"the agent\"" "C-04 C-05 actor clarification"

assert_pattern "${REPO_ROOT}/docs/flows.md" "## Actor and Plane Legend" "flow legend"
assert_pattern "${REPO_ROOT}/docs/flows.md" "What this flow is:" "flow interpretation"
assert_pattern "${REPO_ROOT}/docs/flows.md" "What this flow is not:" "flow non-goals"
assert_pattern "${REPO_ROOT}/docs/flows.md" "Trigger summary:" "C-08 trigger semantics"
assert_pattern "${REPO_ROOT}/docs/flows.md" "Not a full IdP/OIDC token exchange system" "C-11 identity boundary"

assert_pattern "${REPO_ROOT}/docs/integration_checklist.md" "Integration Boundary Decision" "C-02 decision branch"
assert_pattern "${REPO_ROOT}/docs/integration_checklist.md" "docs/scenarios/simple_agent_tool_boundary.md" "C-16 scenario link"

assert_pattern "${REPO_ROOT}/docs/ui_localhost.md" "Runtime boundary reminder" "C-14 UI relation"
assert_pattern "${REPO_ROOT}/docs/approval_runbook.md" "Trigger and response matrix" "C-08 runtime vs CI"
assert_pattern "${REPO_ROOT}/docs/policy_rollout.md" "Identity boundary note" "C-11 external IdP boundary"
assert_pattern "${REPO_ROOT}/docs/sdk/python.md" "Important boundary" "C-12 CLI authority"

assert_pattern "${REPO_ROOT}/docs/agent_integration_boundary.md" "Tier C" "managed agent tier"
assert_pattern "${REPO_ROOT}/docs/mcp_capability_matrix.md" "Capability Matrix" "C-09 mcp capability"
assert_pattern "${REPO_ROOT}/docs/scenarios/simple_agent_tool_boundary.md" "Operational Rule" "C-16 operational rule"
assert_pattern "${REPO_ROOT}/docs/demo_output_legend.md" '## `gait demo` \(Standard\)' "C-13 output legend"

echo "docs storyline acceptance: pass"
