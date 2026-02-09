#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

index_path="docs/ecosystem/community_index.json"
output_path="gait-out/ecosystem_release_notes.md"

python3 scripts/validate_community_index.py "$index_path"
python3 scripts/render_ecosystem_release_notes.py "$index_path" "$output_path"

if [[ ! -s "$output_path" ]]; then
  echo "ecosystem release notes output missing or empty: $output_path" >&2
  exit 1
fi
if ! rg -q '^# Ecosystem Release Notes$' "$output_path"; then
  echo "ecosystem release notes heading missing in $output_path" >&2
  exit 1
fi
if ! rg -q 'adapter-openai-agents-official' "$output_path"; then
  echo "expected adapter entry missing from ecosystem release notes" >&2
  exit 1
fi

echo "ecosystem release automation: pass"
