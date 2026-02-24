#!/usr/bin/env bash
set -euo pipefail

echo "[chaos-job] runtime transition contention and fail-closed invariants"
go test ./core/jobruntime -run 'TestDecisionNeededResumeRequiresApproval|TestResumeEnvironmentMismatchFailClosed|TestInvalidPauseTransition|TestAddCheckpointValidationAndStateTransitions' -count=5

echo "[chaos-job] emergency stop latency and post-stop side-effect invariants"
go test ./internal/integration ./internal/e2e -run 'StopLatency|EmergencyStop' -count=1

echo "[chaos-job] integration job->pack->regress loop stability"
go test ./internal/integration -run 'TestJobRuntimeToPackRoundTrip|TestRegressInitFromPackSource' -count=3
