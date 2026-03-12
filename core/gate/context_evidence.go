package gate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/contextproof"
	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const verifiedContextSource = "context_envelope"

func prepareIntentForEvaluation(intent schemagate.IntentRequest, opts EvalOptions) (schemagate.IntentRequest, string, error) {
	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		return schemagate.IntentRequest{}, "", err
	}

	contextSource := ""
	if opts.VerifiedContextEnvelope != nil {
		normalizedIntent, err = bindVerifiedContextEnvelope(normalizedIntent, *opts.VerifiedContextEnvelope, opts.ContextEvidenceNow)
		if err != nil {
			return schemagate.IntentRequest{}, "", err
		}
		contextSource = verifiedContextSource
	} else {
		normalizedIntent = stripContextEvidenceClaims(normalizedIntent)
	}
	return normalizedIntent, contextSource, nil
}

func stripContextEvidenceClaims(intent schemagate.IntentRequest) schemagate.IntentRequest {
	output := intent
	output.Context.ContextSetDigest = ""
	output.Context.ContextEvidenceMode = ""
	output.Context.ContextRefs = nil
	if len(output.Context.AuthContext) == 0 {
		return output
	}

	authContext := cloneAuthContext(output.Context.AuthContext)
	delete(authContext, "context_age_seconds")
	if len(authContext) == 0 {
		output.Context.AuthContext = nil
		return output
	}
	output.Context.AuthContext = authContext
	return output
}

func bindVerifiedContextEnvelope(intent schemagate.IntentRequest, envelope schemacontext.Envelope, now time.Time) (schemagate.IntentRequest, error) {
	normalizedEnvelope, err := contextproof.NormalizeEnvelope(envelope)
	if err != nil {
		return schemagate.IntentRequest{}, fmt.Errorf("verify context envelope: %w", err)
	}
	resolvedNow := now.UTC()
	if resolvedNow.IsZero() {
		resolvedNow = time.Now().UTC()
	}

	output := intent
	if digestClaim := strings.TrimSpace(output.Context.ContextSetDigest); digestClaim != "" && !strings.EqualFold(digestClaim, normalizedEnvelope.ContextSetDigest) {
		return schemagate.IntentRequest{}, fmt.Errorf("context envelope digest does not match context.context_set_digest")
	}
	if modeClaim := strings.TrimSpace(output.Context.ContextEvidenceMode); modeClaim != "" && !strings.EqualFold(modeClaim, normalizedEnvelope.EvidenceMode) {
		return schemagate.IntentRequest{}, fmt.Errorf("context envelope evidence mode does not match context.context_evidence_mode")
	}

	output = stripContextEvidenceClaims(output)
	output.Context.ContextSetDigest = normalizedEnvelope.ContextSetDigest
	output.Context.ContextEvidenceMode = normalizedEnvelope.EvidenceMode
	output.Context.ContextRefs = envelopeContextRefs(normalizedEnvelope)
	authContext := cloneAuthContext(output.Context.AuthContext)
	if authContext == nil {
		authContext = map[string]any{}
	}
	authContext["context_age_seconds"] = envelopeContextAgeSeconds(normalizedEnvelope, resolvedNow)
	output.Context.AuthContext = authContext
	return output, nil
}

func envelopeContextRefs(envelope schemacontext.Envelope) []string {
	if len(envelope.Records) == 0 {
		return nil
	}
	refs := make([]string, 0, len(envelope.Records))
	seen := make(map[string]struct{}, len(envelope.Records))
	for _, record := range envelope.Records {
		refID := strings.TrimSpace(record.RefID)
		if refID == "" {
			continue
		}
		if _, ok := seen[refID]; ok {
			continue
		}
		seen[refID] = struct{}{}
		refs = append(refs, refID)
	}
	sort.Strings(refs)
	if len(refs) == 0 {
		return nil
	}
	return refs
}

func envelopeContextAgeSeconds(envelope schemacontext.Envelope, now time.Time) int64 {
	if len(envelope.Records) == 0 {
		return contextAgeSecondsFromTime(envelope.CreatedAt.UTC(), now)
	}
	var maxAge int64
	for _, record := range envelope.Records {
		age := contextAgeSecondsFromTime(record.RetrievedAt.UTC(), now)
		if age > maxAge {
			maxAge = age
		}
	}
	return maxAge
}

func contextAgeSecondsFromTime(value time.Time, now time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	if value.After(now) {
		return 0
	}
	return int64(now.Sub(value) / time.Second)
}

func mergeContextSource(current string, extra string) string {
	parts := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, value := range []string{current, extra} {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		parts = append(parts, trimmed)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func PolicyRequiresContextEvidence(policy Policy) bool {
	normalizedPolicy, err := normalizePolicy(policy)
	if err != nil {
		return false
	}
	if contains(normalizedPolicy.FailClosed.RequiredFields, "context_evidence") {
		return true
	}
	for _, rule := range normalizedPolicy.Rules {
		if rule.RequireContextEvidence || rule.RequiredContextEvidenceMode == "required" || rule.MaxContextAgeSeconds > 0 {
			return true
		}
	}
	return false
}
