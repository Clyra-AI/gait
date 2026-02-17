#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FAILURES=0

fail() {
  echo "docs consistency failure: $1" >&2
  FAILURES=$((FAILURES + 1))
}

require_file() {
  local path="$1"
  if [[ ! -f "${path}" ]]; then
    fail "missing file ${path}"
  fi
}

require_pattern() {
  local path="$1"
  local pattern="$2"
  local reason="$3"
  if ! rg -q --pcre2 "${pattern}" "${path}"; then
    fail "${reason} (${path})"
  fi
}

require_readme_intro_heading_near_top() {
  local path="$1"
  local max_lines="${2:-40}"
  local pattern="$3"
  if ! sed -n "1,${max_lines}p" "${path}" | rg -q --pcre2 "${pattern}"; then
    fail "README must include an intro heading near the top (within first ${max_lines} lines): ${pattern} (${path})"
  fi
}

for path in \
  "${REPO_ROOT}/README.md" \
  "${REPO_ROOT}/docs/README.md" \
  "${REPO_ROOT}/docs/concepts/mental_model.md" \
  "${REPO_ROOT}/docs/failure_taxonomy_exit_codes.md" \
  "${REPO_ROOT}/docs/adopt_in_one_pr.md" \
  "${REPO_ROOT}/docs/durable_jobs.md" \
  "${REPO_ROOT}/docs/slo/runtime_slo.md" \
  "${REPO_ROOT}/docs/contracts/compatibility_matrix.md" \
  "${REPO_ROOT}/docs/contracts/pack_producer_kit.md" \
  "${REPO_ROOT}/docs-site/src/lib/navigation.ts" \
  "${REPO_ROOT}/docs-site/src/app/docs/page.tsx" \
  "${REPO_ROOT}/docs-site/public/sitemap.xml" \
  "${REPO_ROOT}/docs-site/public/ai-sitemap.xml" \
  "${REPO_ROOT}/docs-site/public/robots.txt" \
  "${REPO_ROOT}/docs-site/public/llm/product.md" \
  "${REPO_ROOT}/docs-site/public/llm/contracts.md" \
  "${REPO_ROOT}/docs-site/public/llm/quickstart.md" \
  "${REPO_ROOT}/docs-site/public/llm/security.md" \
  "${REPO_ROOT}/docs-site/public/llm/faq.md" \
  "${REPO_ROOT}/docs-site/public/llms.txt" \
  "${REPO_ROOT}/docs-site/public/llms-full.txt" \
  "${REPO_ROOT}/cmd/gait/verify.go"; do
  require_file "${path}"
done

# Canonical capability surface consistency.
for path in \
  "${REPO_ROOT}/README.md" \
  "${REPO_ROOT}/docs/README.md" \
  "${REPO_ROOT}/docs-site/public/llm/product.md"; do
  for term in Jobs Packs Gate Regress Doctor; do
    if ! rg -qi --pcre2 "\\b${term}\\b" "${path}"; then
      fail "capability term '${term}' missing in ${path}"
    fi
  done
done

for cmd in "gait job" "gait pack" "gait gate eval" "gait regress" "gait doctor"; do
  if ! rg -qi --fixed-strings "${cmd}" "${REPO_ROOT}/docs-site/public/llms.txt"; then
    fail "llms command surface missing '${cmd}'"
  fi
done

# Required onboarding sections.
require_readme_intro_heading_near_top \
  "${REPO_ROOT}/README.md" \
  "40" \
  "^## (Overview|What Gait Is|Quick Context|In Brief|At a Glance)$"
require_pattern "${REPO_ROOT}/README.md" "^## When To Use Gait$" "README must include when-to-use guidance"
require_pattern "${REPO_ROOT}/README.md" "^## When Not To Use Gait$" "README must include when-not-to-use guidance"
require_pattern "${REPO_ROOT}/docs/concepts/mental_model.md" "^## Problem-First View$" "mental model must lead with problems"
require_pattern "${REPO_ROOT}/docs/concepts/mental_model.md" "^## Tool Boundary \\(Canonical Definition\\)$" "mental model must define tool boundary"
require_pattern "${REPO_ROOT}/docs/adopt_in_one_pr.md" "^# Adopt In One PR$" "one-pr adoption page missing title"
require_pattern "${REPO_ROOT}/docs/durable_jobs.md" "^# Durable Jobs$" "durable jobs page missing title"
require_pattern "${REPO_ROOT}/docs/durable_jobs.md" "^## When To Use This$" "durable jobs page missing use guidance"
require_pattern "${REPO_ROOT}/docs/durable_jobs.md" "^## When Not To Use This$" "durable jobs page missing non-fit guidance"

# Exit-code consistency against CLI constants.
EXIT_CODES=()
while IFS= read -r code; do
  EXIT_CODES+=("${code}")
done < <(rg -o --no-filename "exit[A-Za-z]+\\s*=\\s*[0-9]+" "${REPO_ROOT}/cmd/gait/verify.go" | rg -o "[0-9]+" | sort -n | uniq)
if [[ "${#EXIT_CODES[@]}" -eq 0 ]]; then
  fail "could not parse exit codes from cmd/gait/verify.go"
fi

for code in "${EXIT_CODES[@]}"; do
  require_pattern "${REPO_ROOT}/docs/failure_taxonomy_exit_codes.md" "\\|\\s*\\x60${code}\\x60\\s*\\|" "exit code ${code} missing in failure taxonomy"
  require_pattern "${REPO_ROOT}/README.md" "\\x60${code}\\x60" "exit code ${code} missing in README stable exit section"
  require_pattern "${REPO_ROOT}/docs-site/public/llm/contracts.md" "\\x60${code}\\x60" "exit code ${code} missing in llm contracts surface"
done

# Side-nav and docs-home discoverability checks.
for route in \
  "/docs/adopt_in_one_pr" \
  "/docs/durable_jobs" \
  "/docs/failure_taxonomy_exit_codes" \
  "/docs/threat_model" \
  "/docs/contracts/pack_producer_kit" \
  "/docs/contracts/compatibility_matrix"; do
  require_pattern "${REPO_ROOT}/docs-site/src/lib/navigation.ts" "${route}" "required docs route missing from side nav"
done

for route in \
  "/docs/adopt_in_one_pr" \
  "/docs/durable_jobs" \
  "/docs/failure_taxonomy_exit_codes" \
  "/docs/threat_model"; do
  require_pattern "${REPO_ROOT}/docs-site/src/app/docs/page.tsx" "${route}" "required docs route missing from docs home tracks"
done

for route in \
  "https://clyra-ai.github.io/gait/docs/adopt_in_one_pr/" \
  "https://clyra-ai.github.io/gait/docs/durable_jobs/" \
  "https://clyra-ai.github.io/gait/docs/failure_taxonomy_exit_codes/" \
  "https://clyra-ai.github.io/gait/docs/threat_model/" \
  "https://clyra-ai.github.io/gait/docs/contracts/pack_producer_kit/" \
  "https://clyra-ai.github.io/gait/docs/contracts/compatibility_matrix/" \
  "https://clyra-ai.github.io/gait/llms.txt" \
  "https://clyra-ai.github.io/gait/llms-full.txt"; do
  require_pattern "${REPO_ROOT}/docs-site/public/sitemap.xml" "${route}" "required URL missing from sitemap.xml"
done

# AEO discoverability checks (LLM resources + crawler policy).
require_pattern "${REPO_ROOT}/docs-site/public/llms.txt" "^## When To Use$" "llms.txt missing when-to-use section"
require_pattern "${REPO_ROOT}/docs-site/public/llms.txt" "^## When Not To Use$" "llms.txt missing when-not-to-use section"
require_pattern "${REPO_ROOT}/docs-site/public/llms.txt" "/llms-full.txt" "llms.txt missing llms-full resource"
require_pattern "${REPO_ROOT}/docs-site/public/ai-sitemap.xml" "https://clyra-ai.github.io/gait/llms.txt" "ai sitemap missing llms.txt"
require_pattern "${REPO_ROOT}/docs-site/public/ai-sitemap.xml" "https://clyra-ai.github.io/gait/llms-full.txt" "ai sitemap missing llms-full.txt"
require_pattern "${REPO_ROOT}/docs-site/public/robots.txt" "Sitemap: https://clyra-ai.github.io/gait/sitemap.xml" "robots.txt missing sitemap.xml pointer"
require_pattern "${REPO_ROOT}/docs-site/public/robots.txt" "Sitemap: https://clyra-ai.github.io/gait/ai-sitemap.xml" "robots.txt missing ai-sitemap pointer"
require_pattern "${REPO_ROOT}/docs-site/public/robots.txt" "User-agent: PerplexityBot" "robots.txt missing PerplexityBot allow rule"
require_pattern "${REPO_ROOT}/docs-site/public/robots.txt" "User-agent: ChatGPT-User" "robots.txt missing ChatGPT-User allow rule"

# Forbidden release tags in evergreen page titles/headings.
for path in \
  "${REPO_ROOT}/README.md" \
  "${REPO_ROOT}/docs/adopt_in_one_pr.md" \
  "${REPO_ROOT}/docs/architecture.md" \
  "${REPO_ROOT}/docs/ci_regress_kit.md" \
  "${REPO_ROOT}/docs/concepts/mental_model.md" \
  "${REPO_ROOT}/docs/durable_jobs.md" \
  "${REPO_ROOT}/docs/failure_taxonomy_exit_codes.md" \
  "${REPO_ROOT}/docs/flows.md" \
  "${REPO_ROOT}/docs/hardening/v2_2_contract.md" \
  "${REPO_ROOT}/docs/integration_checklist.md" \
  "${REPO_ROOT}/docs/slo/runtime_slo.md" \
  "${REPO_ROOT}/docs/threat_model.md"; do
  if rg -q --pcre2 "^(title:\\s*\".*v[0-9]+\\.[0-9]+.*\"|# .*v[0-9]+\\.[0-9]+)" "${path}"; then
    fail "evergreen title/header contains release tag in ${path}"
  fi
done

if [[ "${FAILURES}" -ne 0 ]]; then
  echo "docs consistency: fail (${FAILURES} issue(s))" >&2
  exit 1
fi

echo "docs consistency: pass"
