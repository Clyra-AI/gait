package gate

import (
	"strings"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

type BuildPolicyExplainOptions struct {
	ProducerVersion        string
	CreatedAt              time.Time
	RequiredApprovals      int
	ValidApprovals         int
	ApprovalAuditPath      string
	DelegationAuditPath    string
	CredentialEvidencePath string
	TraceID                string
	TracePath              string
}

func BuildPolicyExplain(policy Policy, outcome EvalOutcome, opts BuildPolicyExplainOptions) schemagate.PolicyExplain {
	createdAt := opts.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = outcome.Result.CreatedAt.UTC()
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = strings.TrimSpace(outcome.Result.ProducerVersion)
	}
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	matchedRules := matchedRulesForOutcome(policy, outcome.MatchedRule)
	return schemagate.PolicyExplain{
		OK:                       true,
		SchemaID:                 "gait.gate.policy_explain",
		SchemaVersion:            "1.0.0",
		CreatedAt:                createdAt,
		ProducerVersion:          producerVersion,
		Verdict:                  outcome.Result.Verdict,
		MatchedRule:              outcome.MatchedRule,
		MatchedRules:             toExplainRules(matchedRules),
		ReasonCodes:              mergeUniqueSorted(nil, outcome.Result.ReasonCodes),
		Violations:               mergeUniqueSorted(nil, outcome.Result.Violations),
		MissingFields:            explainMissingFields(outcome.Result.ReasonCodes),
		FailClosedReasonCodes:    filterReasonCodesByPrefix(outcome.Result.ReasonCodes, "fail_closed_"),
		ApprovalRequired:         outcome.Result.Verdict == "require_approval",
		RequiredApprovals:        opts.RequiredApprovals,
		ValidApprovals:           opts.ValidApprovals,
		RequireBrokerCredential:  outcome.RequireBrokerCredential,
		BrokerReference:          strings.TrimSpace(outcome.BrokerReference),
		BrokerScopes:             mergeUniqueSorted(nil, outcome.BrokerScopes),
		RequireDelegation:        outcome.RequireDelegation,
		RequiredDelegationScopes: mergeUniqueSorted(nil, outcome.RequiredDelegationScopes),
		CredentialPosture:        explainCredentialPosture(outcome.PreparedIntent),
		ContextEvidenceStatus:    explainContextEvidenceStatus(outcome.PreparedIntent),
		FreezeWindow:             outcome.FreezeWindow,
		KillSwitch:               outcome.KillSwitch,
		Sandbox:                  outcome.Sandbox,
		ProofRefs: &schemagate.PolicyExplainProofRefs{
			TraceID:                strings.TrimSpace(opts.TraceID),
			TracePath:              strings.TrimSpace(opts.TracePath),
			ApprovalAuditPath:      strings.TrimSpace(opts.ApprovalAuditPath),
			DelegationAuditPath:    strings.TrimSpace(opts.DelegationAuditPath),
			CredentialEvidencePath: strings.TrimSpace(opts.CredentialEvidencePath),
		},
	}
}

func matchedRulesForOutcome(policy Policy, matchedRule string) []PolicyRule {
	names := map[string]struct{}{}
	for _, name := range strings.Split(matchedRule, ",") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names[trimmed] = struct{}{}
		}
	}
	rules := make([]PolicyRule, 0, len(names))
	for _, rule := range policy.Rules {
		if _, ok := names[rule.Name]; ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func toExplainRules(rules []PolicyRule) []schemagate.PolicyExplainRule {
	out := make([]schemagate.PolicyExplainRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, schemagate.PolicyExplainRule{
			Name:     rule.Name,
			Priority: rule.Priority,
			Effect:   rule.Effect,
		})
	}
	return out
}

func explainMissingFields(reasonCodes []string) []string {
	missing := []string{}
	for _, reasonCode := range reasonCodes {
		if strings.HasPrefix(reasonCode, "fail_closed_missing_") {
			missing = append(missing, strings.TrimPrefix(reasonCode, "fail_closed_missing_"))
			continue
		}
		switch reasonCode {
		case "credential_evidence_missing":
			missing = append(missing, "credential_evidence")
		case "sandbox_metadata_missing":
			missing = append(missing, "sandbox")
		case "sandbox_evidence_missing":
			missing = append(missing, "sandbox_evidence")
		case "kill_switch_state_unavailable":
			missing = append(missing, "kill_switch_state")
		case "broker_credential_missing":
			missing = append(missing, "broker_credential")
		}
	}
	return uniqueSorted(missing)
}

func filterReasonCodesByPrefix(reasonCodes []string, prefix string) []string {
	filtered := []string{}
	for _, reasonCode := range reasonCodes {
		if strings.HasPrefix(reasonCode, prefix) {
			filtered = append(filtered, reasonCode)
		}
	}
	return uniqueSorted(filtered)
}

func explainCredentialPosture(intent schemagate.IntentRequest) *schemagate.PolicyCredentialState {
	context := intent.Context
	if strings.TrimSpace(context.CredentialRef) == "" &&
		strings.TrimSpace(context.CredentialSource) == "" &&
		strings.TrimSpace(context.CredentialAccessType) == "" &&
		strings.TrimSpace(context.CredentialIssuer) == "" &&
		context.CredentialTTLSeconds == 0 {
		return &schemagate.PolicyCredentialState{Present: false}
	}
	return &schemagate.PolicyCredentialState{
		Present:       strings.TrimSpace(context.CredentialRef) != "",
		Source:        strings.TrimSpace(context.CredentialSource),
		AccessType:    strings.TrimSpace(context.CredentialAccessType),
		Issuer:        strings.TrimSpace(context.CredentialIssuer),
		TTLSeconds:    context.CredentialTTLSeconds,
		CredentialRef: strings.TrimSpace(context.CredentialRef),
	}
}

func explainContextEvidenceStatus(intent schemagate.IntentRequest) string {
	if strings.TrimSpace(intent.Context.ContextEvidenceMode) == "required" {
		if strings.TrimSpace(intent.Context.ContextSetDigest) == "" || len(intent.Context.ContextRefs) == 0 {
			return "missing"
		}
		return "verified"
	}
	if strings.TrimSpace(intent.Context.ContextSetDigest) != "" || len(intent.Context.ContextRefs) > 0 {
		return "present"
	}
	return "not_required"
}
