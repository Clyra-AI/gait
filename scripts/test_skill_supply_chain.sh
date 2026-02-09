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

echo "skill supply chain checks: pass"
