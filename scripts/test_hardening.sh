#!/usr/bin/env bash
set -euo pipefail

echo "[hardening] atomic write integrity tests"
go test ./core/fsx -count=1

echo "[hardening] durable job rollback and divergence diagnostics"
go test ./core/jobruntime ./core/doctor -run 'TestMutationAppendFailureRollsBackStateAndRetrySucceeds|TestStatusRecoversPendingMutationWithoutEventByRollingBackState|TestStatusRecoversPendingMutationWithDurableEvent|TestDiagnoseDurableStateDetectsPendingAndRevisionDivergence|TestRunDetectsDurableStateDivergence' -count=1

echo "[hardening] lock contention and stale lock tests"
go test ./core/gate -run 'TestEnforceRateLimitConcurrentLocking|TestEnforceRateLimitRecoversStaleLock|TestWithRateLimitLockTimeoutCategory' -count=1

echo "[hardening] network retry/fallback classification tests"
go test ./core/registry -run 'TestInstallRemoteRetryAndFallbackBranches' -count=1

echo "[hardening] error envelope and exit-code contract tests"
go test ./cmd/gait -run 'TestMarshalOutputWithErrorEnvelope|TestMarshalOutputWithCorrelationForSuccess|TestMarshalOutputErrorEnvelopeGolden|TestExitCodeForError' -count=1

echo "[hardening] concurrent integration tests"
go test ./internal/integration -run 'TestConcurrentGateRateLimitStateIsDeterministic' -count=1

echo "[hardening] e2e exit-code contract tests"
go test ./internal/e2e -run 'TestCLIRegressExitCodes|TestCLIPolicyTestExitCodes|TestCLIDoctor' -count=1
