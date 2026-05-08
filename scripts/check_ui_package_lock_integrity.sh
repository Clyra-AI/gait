#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCKFILE="$REPO_ROOT/ui/local/package-lock.json"

if [[ ! -f "$LOCKFILE" ]]; then
  echo "ui/local/package-lock.json not found" >&2
  exit 1
fi

node - "$LOCKFILE" <<'NODE'
const fs = require("fs");

const lockfilePath = process.argv[2];
const lockfile = JSON.parse(fs.readFileSync(lockfilePath, "utf8"));
const packages = lockfile.packages || {};
let failed = false;

for (const [packagePath, metadata] of Object.entries(packages)) {
  if (packagePath === "" || metadata.link === true) {
    continue;
  }

  if (typeof metadata.version !== "string" || metadata.version.trim() === "") {
    console.error(`invalid lockfile package metadata: ${packagePath} is missing a non-empty version`);
    failed = true;
  }
}

if (failed) {
  process.exit(1);
}
NODE

echo "ui package-lock integrity: pass"
