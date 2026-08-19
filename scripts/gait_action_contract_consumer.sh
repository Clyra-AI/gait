#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo '{"consumer":"gait","version":"unknown","status":"reject","self_attestation":false,"reason_codes":["explicit_selection_required"]}'
  exit 6
fi

if [ -n "${GAIT_BIN:-}" ]; then
  exec "$GAIT_BIN" contract consume "$1"
fi

if command -v gait >/dev/null 2>&1; then
  exec gait contract consume "$1"
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ -x "$repo_root/gait" ]; then
  exec "$repo_root/gait" contract consume "$1"
fi

echo '{"consumer":"gait","version":"unknown","status":"dependency_missing","self_attestation":false,"reason_codes":["consumer_binary_missing"]}'
exit 7
