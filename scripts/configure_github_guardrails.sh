#!/usr/bin/env bash
set -euo pipefail

if ! command -v gh >/dev/null 2>&1; then
  echo "[guardrails] gh CLI is required" >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "[guardrails] gh auth status failed; run: gh auth login" >&2
  exit 1
fi

repo_default="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
branch_default="$(gh repo view --json defaultBranchRef --jq .defaultBranchRef.name)"

repo="${GAIT_GH_REPO:-$repo_default}"
branch="${GAIT_GH_BRANCH:-$branch_default}"
checks_csv="${GAIT_REQUIRED_CHECKS:-pr-fast-lint,pr-fast-test,pr-fast-windows,codeql-scan}"
required_reviews="${GAIT_REQUIRED_REVIEWS:-0}"
require_codeowners="${GAIT_REQUIRE_CODEOWNER_REVIEWS:-false}"
dismiss_stale="${GAIT_DISMISS_STALE_REVIEWS:-true}"
require_last_push_approval="${GAIT_REQUIRE_LAST_PUSH_APPROVAL:-false}"
enforce_admins="${GAIT_ENFORCE_ADMINS:-true}"
strict_status_checks="${GAIT_STRICT_STATUS_CHECKS:-true}"
required_conversation_resolution="${GAIT_REQUIRED_CONVERSATION_RESOLUTION:-true}"
required_linear_history="${GAIT_REQUIRED_LINEAR_HISTORY:-true}"
allow_force_pushes="${GAIT_ALLOW_FORCE_PUSHES:-false}"
allow_deletions="${GAIT_ALLOW_DELETIONS:-false}"
dry_run="${GAIT_GUARDRAILS_DRY_RUN:-0}"

python3 - "$required_reviews" <<'PY'
import sys
try:
    value = int(sys.argv[1])
except Exception:
    raise SystemExit("[guardrails] GAIT_REQUIRED_REVIEWS must be an integer")
if value < 0 or value > 6:
    raise SystemExit("[guardrails] GAIT_REQUIRED_REVIEWS must be between 0 and 6")
PY

tmp_payload="$(mktemp "${TMPDIR:-/tmp}/gait-guardrails-XXXXXX.json")"
cleanup() {
  rm -f "$tmp_payload"
}
trap cleanup EXIT

python3 - "$tmp_payload" "$checks_csv" "$required_reviews" \
  "$require_codeowners" "$dismiss_stale" "$require_last_push_approval" \
  "$enforce_admins" "$strict_status_checks" "$required_conversation_resolution" \
  "$required_linear_history" "$allow_force_pushes" "$allow_deletions" <<'PY'
import json
import sys

(
    out_path,
    checks_csv,
    required_reviews,
    require_codeowners,
    dismiss_stale,
    require_last_push_approval,
    enforce_admins,
    strict_status_checks,
    required_conversation_resolution,
    required_linear_history,
    allow_force_pushes,
    allow_deletions,
) = sys.argv[1:13]


def parse_bool(value: str) -> bool:
    normalized = value.strip().lower()
    if normalized in {"1", "true", "yes", "on"}:
        return True
    if normalized in {"0", "false", "no", "off"}:
        return False
    raise SystemExit(f"[guardrails] invalid boolean value: {value}")

checks = [item.strip() for item in checks_csv.split(",") if item.strip()]
if not checks:
    raise SystemExit("[guardrails] GAIT_REQUIRED_CHECKS must include at least one status check")

payload = {
    "required_status_checks": {
        "strict": parse_bool(strict_status_checks),
        "contexts": checks,
    },
    "enforce_admins": parse_bool(enforce_admins),
    "required_pull_request_reviews": {
        "dismiss_stale_reviews": parse_bool(dismiss_stale),
        "require_code_owner_reviews": parse_bool(require_codeowners),
        "required_approving_review_count": int(required_reviews),
        "require_last_push_approval": parse_bool(require_last_push_approval),
    },
    "restrictions": None,
    "required_linear_history": parse_bool(required_linear_history),
    "allow_force_pushes": parse_bool(allow_force_pushes),
    "allow_deletions": parse_bool(allow_deletions),
    "required_conversation_resolution": parse_bool(required_conversation_resolution),
}

with open(out_path, "w", encoding="utf-8") as handle:
    json.dump(payload, handle)
PY

echo "[guardrails] repo=${repo} branch=${branch}"
echo "[guardrails] required checks=${checks_csv}"
echo "[guardrails] required approving reviews=${required_reviews}"
echo "[guardrails] require CODEOWNERS review=${require_codeowners}"

if [[ "$dry_run" == "1" ]]; then
  echo "[guardrails] dry run payload:"
  cat "$tmp_payload"
  exit 0
fi

gh api \
  --method PUT \
  --header "Accept: application/vnd.github+json" \
  "repos/${repo}/branches/${branch}/protection" \
  --input "$tmp_payload" >/dev/null

echo "[guardrails] branch protection updated"
gh api "repos/${repo}/branches/${branch}/protection" --jq '{required_checks: .required_status_checks.contexts, strict: .required_status_checks.strict, required_reviews: .required_pull_request_reviews.required_approving_review_count, require_codeowners: .required_pull_request_reviews.require_code_owner_reviews, enforce_admins: .enforce_admins.enabled, required_conversation_resolution: .required_conversation_resolution.enabled, allow_force_pushes: .allow_force_pushes.enabled, allow_deletions: .allow_deletions.enabled}'
