#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/ui/local"

if [[ ! -d node_modules ]]; then
  npm ci
fi
npm run lint
npm run build
