#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

bash "$REPO_ROOT/scripts/ui_build.sh"

rm -rf "$REPO_ROOT/internal/uiassets/dist"
mkdir -p "$REPO_ROOT/internal/uiassets/dist"
cp -R "$REPO_ROOT/ui/local/out/." "$REPO_ROOT/internal/uiassets/dist/"

echo "ui assets synced: internal/uiassets/dist"
