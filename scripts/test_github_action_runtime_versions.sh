#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_PATH="${REPO_ROOT}/scripts/check_github_action_runtime_versions.py"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "${message}" >&2
    echo "expected:" >&2
    printf '%s\n' "${expected}" >&2
    echo "actual:" >&2
    printf '%s\n' "${actual}" >&2
    exit 1
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    echo "${message}" >&2
    echo "missing fragment: ${needle}" >&2
    exit 1
  fi
}

mkdir -p "${WORK_DIR}/pass/.github/workflows" "${WORK_DIR}/pass/docs"
cat > "${WORK_DIR}/pass/.github/workflows/core.yml" <<'EOF'
name: core
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
      - uses: actions/setup-python@v6
      - uses: actions/setup-node@v5
      - uses: github/codeql-action/init@v4
      - uses: github/codeql-action/analyze@v4
      - uses: actions/download-artifact@87c55149d96e628cc2ef7e6fc2aab372015aec85 # v4.1.3
EOF
cat > "${WORK_DIR}/pass/docs/adopt_in_one_pr.md" <<'EOF'
```yaml
- uses: actions/checkout@v5
```
EOF

pass_output="$(
  python3 "${SCRIPT_PATH}" \
    "${WORK_DIR}/pass/.github/workflows" \
    "${WORK_DIR}/pass/docs/adopt_in_one_pr.md"
)"
assert_eq \
  "${pass_output}" \
  "workflow runtime guard: pass (2 files scanned)" \
  "guard should pass deterministically for allowed action majors"

mkdir -p "${WORK_DIR}/fail/.github/workflows" "${WORK_DIR}/fail/docs"
cat > "${WORK_DIR}/fail/.github/workflows/core.yml" <<'EOF'
name: core
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-python@v5
      - uses: actions/checkout@v4
      - uses: github/codeql-action/analyze@v3
      - uses: actions/setup-go@v5
      - uses: actions/setup-node@v4
      - uses: actions/download-artifact@v4
EOF
cat > "${WORK_DIR}/fail/docs/adopt_in_one_pr.md" <<'EOF'
```yaml
- uses: actions/checkout@v4
```
EOF

set +e
fail_output="$(
  python3 "${SCRIPT_PATH}" \
    "${WORK_DIR}/fail/.github/workflows" \
    "${WORK_DIR}/fail/docs/adopt_in_one_pr.md" \
    2>&1
)"
fail_status=$?
set -e
assert_eq "${fail_status}" "1" "guard should exit 1 on deprecated action majors"

expected_fail_output="$(cat <<EOF
workflow runtime guard: fail
${WORK_DIR}/fail/.github/workflows/core.yml:6: actions/setup-python@v5: deprecated major v5; require actions/setup-python@v6+
${WORK_DIR}/fail/.github/workflows/core.yml:7: actions/checkout@v4: deprecated major v4; require actions/checkout@v5+
${WORK_DIR}/fail/.github/workflows/core.yml:8: github/codeql-action/analyze@v3: deprecated major v3; require github/codeql-action/analyze@v4+
${WORK_DIR}/fail/.github/workflows/core.yml:9: actions/setup-go@v5: deprecated major v5; require actions/setup-go@v6+
${WORK_DIR}/fail/.github/workflows/core.yml:10: actions/setup-node@v4: deprecated major v4; require actions/setup-node@v5+
${WORK_DIR}/fail/.github/workflows/core.yml:11: actions/download-artifact@v4: mutable ref; require actions/download-artifact@<40-character commit SHA>
${WORK_DIR}/fail/docs/adopt_in_one_pr.md:2: actions/checkout@v4: deprecated major v4; require actions/checkout@v5+
EOF
)"
assert_eq \
  "${fail_output}" \
  "${expected_fail_output}" \
  "guard should report deterministic sorted failures"

set +e
usage_output="$(python3 "${SCRIPT_PATH}" 2>&1)"
usage_status=$?
set -e
assert_eq "${usage_status}" "2" "guard should exit 2 on missing arguments"
assert_contains "${usage_output}" "usage:" "guard should print argparse usage on missing arguments"

INTEGRATION_REPO="${WORK_DIR}/integration"
mkdir -p "${INTEGRATION_REPO}/scripts" "${INTEGRATION_REPO}/product" "${INTEGRATION_REPO}/docs" "${INTEGRATION_REPO}/.github/workflows"
cp "${REPO_ROOT}/scripts/check_repo_hygiene.sh" "${INTEGRATION_REPO}/scripts/check_repo_hygiene.sh"
cp "${REPO_ROOT}/scripts/check_github_action_runtime_versions.py" "${INTEGRATION_REPO}/scripts/check_github_action_runtime_versions.py"

cat > "${INTEGRATION_REPO}/Makefile" <<'EOF'
lint-fast:
	bash scripts/check_repo_hygiene.sh

lint:
	bash scripts/check_repo_hygiene.sh
EOF

for path in \
  "product/PRD.md" \
  "product/ROADMAP.md" \
  "product/PLAN_v1.md" \
  "product/PLAN_v1.7.md" \
  "product/PLAN_ADOPTION.md" \
  "product/PLAN_HARDENING.md"; do
  mkdir -p "${INTEGRATION_REPO}/$(dirname "${path}")"
  printf '# placeholder\n' > "${INTEGRATION_REPO}/${path}"
done

cat > "${INTEGRATION_REPO}/docs/adopt_in_one_pr.md" <<'EOF'
```yaml
- uses: actions/checkout@v5
```
EOF
cat > "${INTEGRATION_REPO}/.github/workflows/ci.yml" <<'EOF'
name: ci
jobs:
  lint:
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
      - uses: actions/setup-node@v5
EOF

(
  cd "${INTEGRATION_REPO}"
  git init -q
  git add .
  make lint-fast >/dev/null
  make lint >/dev/null
)

cat > "${INTEGRATION_REPO}/.github/workflows/ci.yml" <<'EOF'
name: ci
jobs:
  lint:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - uses: actions/setup-node@v4
EOF

(
  cd "${INTEGRATION_REPO}"
  git add .github/workflows/ci.yml
  set +e
  lint_fast_output="$(make lint-fast 2>&1)"
  lint_fast_status=$?
  lint_output="$(make lint 2>&1)"
  lint_status=$?
  set -e
  assert_eq "${lint_fast_status}" "2" "lint-fast should fail when deprecated action majors are reintroduced"
  assert_eq "${lint_status}" "2" "lint should fail when deprecated action majors are reintroduced"
  assert_contains "${lint_fast_output}" "workflow runtime guard: fail" "lint-fast should surface guard failure output"
  assert_contains "${lint_output}" "workflow runtime guard: fail" "lint should surface guard failure output"
)

echo "github action runtime guard: pass"
