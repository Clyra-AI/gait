#!/usr/bin/env bash
set -euo pipefail

if ! command -v git >/dev/null 2>&1; then
  echo "[repo-hygiene] git is required"
  exit 1
fi

required_product_docs=(
  "product/PRD.md"
  "product/ROADMAP.md"
  "product/PLAN_v1.md"
  "product/PLAN_v1.7.md"
  "product/PLAN_ADOPTION.md"
  "product/PLAN_HARDENING.md"
)

missing_docs=()
ignored_docs=()

for path in "${required_product_docs[@]}"; do
  if ! git ls-files --error-unmatch "${path}" >/dev/null 2>&1; then
    missing_docs+=("${path}")
  fi
  if git check-ignore --no-index -q "${path}"; then
    ignored_docs+=("${path}")
  fi
done

tracked_generated=()
while IFS= read -r path; do
  case "${path}" in
    gait-out/*|coverage-go.out|coverage-go.txt|coverage-*.out|gait|perf/bench_output.txt|perf/bench_report.json|perf/command_budget_report.json|sdk/python/.coverage)
      tracked_generated+=("${path}")
      ;;
  esac
done < <(git ls-files)

if [[ "${#missing_docs[@]}" -gt 0 ]]; then
  echo "[repo-hygiene] required product docs must be tracked:"
  for path in "${missing_docs[@]}"; do
    echo "  - ${path}"
  done
  exit 1
fi

if [[ "${#ignored_docs[@]}" -gt 0 ]]; then
  echo "[repo-hygiene] product docs must not be ignored:"
  for path in "${ignored_docs[@]}"; do
    echo "  - ${path}"
  done
  exit 1
fi

if [[ "${#tracked_generated[@]}" -gt 0 ]]; then
  echo "[repo-hygiene] generated artifacts must not be tracked:"
  for path in "${tracked_generated[@]}"; do
    echo "  - ${path}"
  done
  echo "[repo-hygiene] remediation:"
  echo "  git rm --cached <path>"
  exit 1
fi
