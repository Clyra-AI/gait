#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GAIT_BIN="${GAIT_BIN:-}"
DEMO_WORKDIR="${DEMO_WORKDIR:-${REPO_ROOT}/gait-out/demo_90s/workspace}"

if [[ -z "${GAIT_BIN}" ]]; then
  if command -v gait >/dev/null 2>&1; then
    GAIT_BIN="$(command -v gait)"
  elif [[ -x "${REPO_ROOT}/gait" ]]; then
    GAIT_BIN="${REPO_ROOT}/gait"
  else
    echo "==> building local gait binary"
    (cd "${REPO_ROOT}" && go build -o ./gait ./cmd/gait)
    GAIT_BIN="${REPO_ROOT}/gait"
  fi
fi

export PATH="$(dirname "${GAIT_BIN}"):${PATH}"
mkdir -p "${DEMO_WORKDIR}"

if [[ ! -e "${DEMO_WORKDIR}/schemas" ]]; then
  ln -s "${REPO_ROOT}/schemas" "${DEMO_WORKDIR}/schemas"
fi

echo "==> demo workspace: ${DEMO_WORKDIR}"
cd "${DEMO_WORKDIR}"

echo "==> doctor"
"${GAIT_BIN}" doctor --json

echo "==> first win"
"${GAIT_BIN}" demo
"${GAIT_BIN}" verify run_demo --json

echo "==> execution-boundary block example"
set +e
"${GAIT_BIN}" policy test \
  "${REPO_ROOT}/examples/prompt-injection/policy.yaml" \
  "${REPO_ROOT}/examples/prompt-injection/intent_injected.json" \
  --json
policy_exit=$?
set -e
if [[ "${policy_exit}" -ne 3 ]]; then
  echo "unexpected prompt-injection fixture exit code: ${policy_exit}" >&2
  exit 1
fi

echo "==> incident to regression"
"${GAIT_BIN}" regress bootstrap --from run_demo --json --junit "./junit.xml"

echo "demo_90s: pass"
