#!/bin/sh
set -eu

receipt_path="${1:-}"
if [ -z "$receipt_path" ]; then
  echo "receipt path is required" >&2
  exit 2
fi

cat "$receipt_path"
