#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="${GAIT_OUT_DIR:-./gait-out}"
GAIT_BIN="${GAIT_BIN:-}"

if [ -z "${GAIT_BIN}" ]; then
  if command -v gait >/dev/null 2>&1; then
    GAIT_BIN="$(command -v gait)"
  elif [ -x "./gait" ]; then
    GAIT_BIN="./gait"
  elif [ -f "./cmd/gait/main.go" ] && command -v go >/dev/null 2>&1; then
    echo "==> Building local gait binary"
    go build -o ./gait ./cmd/gait
    GAIT_BIN="./gait"
  fi
fi

if [ -z "${GAIT_BIN}" ]; then
  echo "error: gait binary not found"
  echo "set GAIT_BIN, put gait on PATH, or run from repo root with Go installed"
  exit 1
fi

if ! mkdir -p "${OUTPUT_DIR}"; then
  echo "error: output directory is not writable: ${OUTPUT_DIR}"
  exit 1
fi

WRITE_CHECK_PATH="${OUTPUT_DIR}/.gait-quickstart-writecheck"
if ! : >"${WRITE_CHECK_PATH}"; then
  echo "error: output directory is not writable: ${OUTPUT_DIR}"
  exit 1
fi
rm -f "${WRITE_CHECK_PATH}"

echo "==> Running gait demo"
DEMO_OUTPUT="$("${GAIT_BIN}" demo)"
printf '%s\n' "${DEMO_OUTPUT}"

RUN_ID="$(printf '%s\n' "${DEMO_OUTPUT}" | awk -F'=' '/^run_id=/{print $2; exit}')"
if [ -z "${RUN_ID}" ]; then
  echo "error: unable to parse run_id from gait demo output"
  exit 1
fi

echo "==> Running gait verify ${RUN_ID}"
"${GAIT_BIN}" verify "${RUN_ID}"

echo "next: ${GAIT_BIN} regress init --from ${RUN_ID}"
