#!/usr/bin/env bash
set -euo pipefail

echo "[hardening-acceptance] hooks configuration enforcement"
bash scripts/check_hooks_config.sh

echo "[hardening-acceptance] error taxonomy coverage"
go test ./core/errors -count=1

echo "[hardening-acceptance] deterministic error envelopes and exit contract"
go test ./cmd/gait -run 'TestMarshalOutputWithErrorEnvelope|TestMarshalOutputWithCorrelationForSuccess|TestMarshalOutputErrorEnvelopeGolden|TestExitCodeForError' -count=1

echo "[hardening-acceptance] atomic write integrity under failure simulation"
go test ./core/fsx -count=1

echo "[hardening-acceptance] lock contention behavior"
go test ./core/gate -run 'TestEnforceRateLimitConcurrentLocking|TestEnforceRateLimitRecoversStaleLock|TestWithRateLimitLockTimeoutCategory' -count=1

echo "[hardening-acceptance] network retry behavior classification"
go test ./core/registry -run 'TestInstallRemoteRetryAndFallbackBranches' -count=1

echo "[hardening-acceptance] hardening integration and e2e checks"
go test ./internal/integration -run 'TestConcurrentGateRateLimitStateIsDeterministic' -count=1
go test ./internal/e2e -run 'TestCLIRegressExitCodes|TestCLIPolicyTestExitCodes|TestCLIDoctor' -count=1
