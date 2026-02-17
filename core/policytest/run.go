package policytest

import (
	"fmt"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemapolicytest "github.com/Clyra-AI/gait/core/schema/v1/policytest"
)

const (
	policyTestSchemaID      = "gait.policytest.result"
	policyTestSchemaVersion = "1.0.0"
	defaultSummaryLimit     = 240
)

type RunOptions struct {
	Policy          gate.Policy
	Intent          schemagate.IntentRequest
	ProducerVersion string
}

type RunResult struct {
	Result  schemapolicytest.PolicyTestResult
	Summary string
}

func Run(opts RunOptions) (RunResult, error) {
	policyDigest, err := gate.PolicyDigest(opts.Policy)
	if err != nil {
		return RunResult{}, fmt.Errorf("policy digest: %w", err)
	}

	normalizedIntent, err := gate.NormalizeIntent(opts.Intent)
	if err != nil {
		return RunResult{}, fmt.Errorf("normalize intent: %w", err)
	}

	evalOutcome, err := gate.EvaluatePolicyDetailed(opts.Policy, opts.Intent, gate.EvalOptions{
		ProducerVersion: opts.ProducerVersion,
	})
	if err != nil {
		return RunResult{}, err
	}
	gateResult := evalOutcome.Result

	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	result := schemapolicytest.PolicyTestResult{
		SchemaID:        policyTestSchemaID,
		SchemaVersion:   policyTestSchemaVersion,
		CreatedAt:       gateResult.CreatedAt,
		ProducerVersion: producerVersion,
		PolicyDigest:    policyDigest,
		IntentDigest:    normalizedIntent.IntentDigest,
		Verdict:         gateResult.Verdict,
		ReasonCodes:     gateResult.ReasonCodes,
		Violations:      gateResult.Violations,
		MatchedRule:     evalOutcome.MatchedRule,
	}
	if normalizedIntent.Delegation != nil {
		result.DelegationMatched = true
		result.DelegationDepth = len(normalizedIntent.Delegation.Chain)
		result.DelegationScope = strings.TrimSpace(normalizedIntent.Delegation.ScopeClass)
	}

	return RunResult{
		Result:  result,
		Summary: boundedSummary(result, defaultSummaryLimit),
	}, nil
}

func boundedSummary(result schemapolicytest.PolicyTestResult, maxChars int) string {
	reasons := "none"
	if len(result.ReasonCodes) > 0 {
		reasons = strings.Join(result.ReasonCodes, ",")
	}
	violations := "none"
	if len(result.Violations) > 0 {
		violations = strings.Join(result.Violations, ",")
	}
	raw := fmt.Sprintf(
		"policy test verdict=%s reasons=%s violations=%s",
		result.Verdict,
		reasons,
		violations,
	)
	if maxChars <= 0 || len(raw) <= maxChars {
		return raw
	}
	if maxChars <= 12 {
		return raw[:maxChars]
	}
	overflow := len(raw) - maxChars
	suffix := fmt.Sprintf("...(+%d)", overflow)
	keep := maxChars - len(suffix)
	if keep < 0 {
		keep = 0
	}
	return raw[:keep] + suffix
}
