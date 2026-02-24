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

echo "[hardening-acceptance] session lock and checkpoint contention behavior"
go test ./core/runpack -run 'TestSessionConcurrentAppendsAreDeterministic|TestSessionLockRecoveryAndHelpers|TestSessionChainVerifyDetectsTamper' -count=1

echo "[hardening-acceptance] network retry behavior classification"
go test ./core/registry -run 'TestInstallRemoteRetryAndFallbackBranches' -count=1

echo "[hardening-acceptance] chaos exporter integrity gate"
bash scripts/test_chaos_exporters.sh

echo "[hardening-acceptance] chaos service boundary gate"
bash scripts/test_chaos_service_boundary.sh

echo "[hardening-acceptance] chaos payload limit gate"
bash scripts/test_chaos_payload_limits.sh

echo "[hardening-acceptance] chaos session swarm/latency gate"
bash scripts/test_chaos_sessions.sh

echo "[hardening-acceptance] chaos trace uniqueness gate"
bash scripts/test_chaos_trace_uniqueness.sh

echo "[hardening-acceptance] hardening integration and e2e checks"
go test ./internal/integration -run 'TestConcurrentGateRateLimitStateIsDeterministic|TestConcurrentSessionAppendStateIsDeterministic|TestSessionSwarmContentionBudget|TestStopLatencySLOForEmergencyStopAcknowledgment|TestEmergencyStopBackpressureHasZeroPostStopSideEffects' -count=1
go test ./internal/e2e -run 'TestCLIRegressExitCodes|TestCLIPolicyTestExitCodes|TestCLIDoctor|TestCLIDelegateAndGateRequireDelegationFlow|TestCLIStopLatencyAndEmergencyStopPreemption' -count=1
