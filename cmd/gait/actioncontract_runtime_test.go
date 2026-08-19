package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/actioncontract"
)

func TestRuntimeReadinessCLIRequiresEvaluationTimeAndAuthoritativeKey(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractReadiness([]string{"--proposal", proposalPath, "--trusted-validators", "validator", "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, actioncontract.ReasonEvaluationTimeInvalid) {
		t.Fatalf("missing evaluation time must be invalid input: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return runActionContractReadiness([]string{"--proposal", proposalPath, "--trusted-validators", "validator", "--evaluation-time", "2026-07-19T01:00:00Z", "--json"})
	})
	if code != exitVerifyFailed || !strings.Contains(output, "validator") {
		t.Fatalf("missing authoritative validator key must remain fail-closed: code=%d output=%s", code, output)
	}
}
