#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> gate skill provenance normalization"
go test ./core/gate -run 'TestNormalizeIntentSkillProvenance|TestEvaluatePolicySkillTrustHooks|TestEmitSignedTraceAndVerify' -count=1

echo "==> registry signature/pin/publisher verification"
go test ./core/registry -run 'TestInstallRemoteWithSignatureAndPin|TestVerifyBranchesAndHelpers|TestListAndVerifyInstalledPack' -count=1

echo "==> cli verification report writer"
go test ./cmd/gait -run 'TestGuardRegistryAndReduceWriters' -count=1

echo "==> schema validation for skill supply-chain artifacts"
go test ./core/schema/validate -run 'TestValidateSchemaFixtures' -count=1

echo "==> deterministic skill install path (codex + claude)"
tmp_root="$(mktemp -d)"
trap 'rm -rf "${tmp_root}"' EXIT
export CODEX_HOME="${tmp_root}/codex"
export CLAUDE_HOME="${tmp_root}/claude"

bash scripts/install_repo_skills.sh --provider codex
bash scripts/install_repo_skills.sh --provider claude

python3 - "${CODEX_HOME}/skills" "${CLAUDE_HOME}/skills" > "${tmp_root}/manifest_before.txt" <<'PY'
import hashlib
import os
import sys
from pathlib import Path

roots = [Path(sys.argv[1]), Path(sys.argv[2])]
for root in roots:
    if not root.exists():
        raise SystemExit(f"missing skills root: {root}")
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        rel = path.relative_to(root).as_posix()
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        mode = oct(path.stat().st_mode & 0o777)
        print(f"{root.name}:{rel}:{mode}:{digest}")
PY

bash scripts/install_repo_skills.sh --provider codex
bash scripts/install_repo_skills.sh --provider claude

python3 - "${CODEX_HOME}/skills" "${CLAUDE_HOME}/skills" > "${tmp_root}/manifest_after.txt" <<'PY'
import hashlib
import os
import sys
from pathlib import Path

roots = [Path(sys.argv[1]), Path(sys.argv[2])]
for root in roots:
    if not root.exists():
        raise SystemExit(f"missing skills root: {root}")
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        rel = path.relative_to(root).as_posix()
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        mode = oct(path.stat().st_mode & 0o777)
        print(f"{root.name}:{rel}:{mode}:{digest}")
PY

if ! cmp -s "${tmp_root}/manifest_before.txt" "${tmp_root}/manifest_after.txt"; then
  echo "skill install is not idempotent across reruns" >&2
  diff -u "${tmp_root}/manifest_before.txt" "${tmp_root}/manifest_after.txt" || true
  exit 1
fi

for provider in codex claude; do
  for skill in \
    gait-capture-runpack \
    gait-incident-to-regression \
    gait-policy-test-rollout \
    incident-to-regression \
    ci-failure-triage \
    evidence-receipt-generation; do
    if [[ ! -f "${tmp_root}/${provider}/skills/${skill}/SKILL.md" ]]; then
      echo "missing installed skill file for provider=${provider} skill=${skill}" >&2
      exit 1
    fi
  done
done

echo "skill supply chain checks: pass"
