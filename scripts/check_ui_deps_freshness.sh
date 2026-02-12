#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_JSON="$REPO_ROOT/ui/local/package.json"

if [[ ! -f "$PACKAGE_JSON" ]]; then
  echo "ui/local/package.json not found"
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required"
  exit 1
fi

PACKAGES=(next react react-dom typescript eslint-config-next)
failed=0
for pkg in "${PACKAGES[@]}"; do
  current="$(node -e "const p=require(process.argv[1]);const v=(p.dependencies&&p.dependencies[process.argv[2]])||(p.devDependencies&&p.devDependencies[process.argv[2]])||'';process.stdout.write(v);" "$PACKAGE_JSON" "$pkg")"
  latest="$(npm view "$pkg" version)"
  if [[ "$current" != "$latest" ]]; then
    echo "stale dependency: $pkg current=$current latest=$latest"
    failed=1
  else
    echo "ok: $pkg $current"
  fi
done

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

echo "ui dependency freshness: pass"
