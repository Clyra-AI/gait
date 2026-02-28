package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
)

type EmitTraceOptions struct {
	ProducerVersion       string
	CorrelationID         string
	ApprovalTokenRef      string
	DelegationTokenRef    string
	DelegationReasonCodes []string
	LatencyMS             float64
	ContextSource         string
	CompositeRiskClass    string
	StepVerdicts          []schemagate.TraceStepVerdict
	PreApproved           bool
	PatternID             string
	RegistryReason        string
	SigningPrivateKey     ed25519.PrivateKey
	TracePath             string
}

type EmitTraceResult struct {
	Trace        schemagate.TraceRecord
	TracePath    string
	PolicyDigest string
	IntentDigest string
}

func EmitSignedTrace(policy Policy, intent schemagate.IntentRequest, gateResult schemagate.GateResult, opts EmitTraceOptions) (EmitTraceResult, error) {
	if len(opts.SigningPrivateKey) == 0 {
		return EmitTraceResult{}, fmt.Errorf("signing private key is required")
	}
	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("normalize intent for trace: %w", err)
	}
	policyDigest, err := PolicyDigest(policy)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("digest policy for trace: %w", err)
	}
	policyID := strings.TrimSpace(policy.SchemaID)
	policyVersion := strings.TrimSpace(policy.SchemaVersion)
	if normalizedIntent.IntentDigest == "" {
		return EmitTraceResult{}, fmt.Errorf("intent digest missing after normalization")
	}
	verdict := strings.TrimSpace(gateResult.Verdict)
	if verdict == "" {
		return EmitTraceResult{}, fmt.Errorf("gate result verdict is required for trace emission")
	}

	createdAt := gateResult.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = normalizedIntent.CreatedAt.UTC()
	}
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = strings.TrimSpace(gateResult.ProducerVersion)
	}
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	trace := schemagate.TraceRecord{
		SchemaID:            "gait.gate.trace",
		SchemaVersion:       "1.0.0",
		CreatedAt:           createdAt,
		ObservedAt:          time.Now().UTC(),
		ProducerVersion:     producerVersion,
		TraceID:             computeTraceID(policyDigest, normalizedIntent.IntentDigest, verdict),
		CorrelationID:       strings.TrimSpace(opts.CorrelationID),
		ToolName:            normalizedIntent.ToolName,
		ArgsDigest:          normalizedIntent.ArgsDigest,
		IntentDigest:        normalizedIntent.IntentDigest,
		PolicyDigest:        policyDigest,
		PolicyID:            policyID,
		PolicyVersion:       policyVersion,
		Verdict:             verdict,
		ContextSetDigest:    normalizedIntent.Context.ContextSetDigest,
		ContextEvidenceMode: normalizedIntent.Context.ContextEvidenceMode,
		ContextRefCount:     len(normalizedIntent.Context.ContextRefs),
		ContextSource:       strings.TrimSpace(opts.ContextSource),
		Script:              normalizedIntent.Script != nil,
		ScriptHash:          normalizedIntent.ScriptHash,
		CompositeRiskClass:  strings.ToLower(strings.TrimSpace(opts.CompositeRiskClass)),
		StepVerdicts:        append([]schemagate.TraceStepVerdict(nil), opts.StepVerdicts...),
		PreApproved:         opts.PreApproved,
		PatternID:           strings.TrimSpace(opts.PatternID),
		RegistryReason:      strings.TrimSpace(opts.RegistryReason),
		Violations:          uniqueSorted(gateResult.Violations),
		LatencyMS:           clampLatency(opts.LatencyMS),
		ApprovalTokenRef:    strings.TrimSpace(opts.ApprovalTokenRef),
		SkillProvenance:     normalizedIntent.SkillProvenance,
	}
	trace.MatchedRuleIDs = matchedRuleIDsFromStepVerdicts(opts.StepVerdicts)
	trace.Relationship = buildTraceRelationship(
		normalizedIntent,
		trace.TraceID,
		policyID,
		policyVersion,
		policyDigest,
		trace.MatchedRuleIDs,
	)
	if normalizedIntent.Script != nil {
		trace.StepCount = len(normalizedIntent.Script.Steps)
	}
	trace.EventID = computeTraceEventID(trace.TraceID, trace.ObservedAt)
	if normalizedIntent.Delegation != nil {
		delegationTokenRef := strings.TrimSpace(opts.DelegationTokenRef)
		if delegationTokenRef == "" && len(normalizedIntent.Delegation.TokenRefs) > 0 {
			delegationTokenRef = strings.TrimSpace(normalizedIntent.Delegation.TokenRefs[0])
		}
		delegationDigest, err := digestDelegationChain(*normalizedIntent.Delegation)
		if err != nil {
			return EmitTraceResult{}, fmt.Errorf("digest delegation chain: %w", err)
		}
		trace.DelegationRef = &schemagate.DelegationRef{
			DelegationTokenRef: delegationTokenRef,
			RequesterIdentity:  strings.TrimSpace(normalizedIntent.Delegation.RequesterIdentity),
			DelegationDepth:    len(normalizedIntent.Delegation.Chain),
			ScopeClass:         strings.TrimSpace(normalizedIntent.Delegation.ScopeClass),
			ChainDigest:        delegationDigest,
			ReasonCodes:        uniqueSorted(opts.DelegationReasonCodes),
		}
	}

	signable := trace
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("marshal signable trace: %w", err)
	}
	signature, err := sign.SignTraceRecordJSON(opts.SigningPrivateKey, signableRaw)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("sign trace record: %w", err)
	}
	trace.Signature = &schemagate.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}

	tracePath := strings.TrimSpace(opts.TracePath)
	if tracePath == "" {
		tracePath = fmt.Sprintf("trace_%s.json", trace.TraceID)
	}
	if err := WriteTraceRecord(tracePath, trace); err != nil {
		return EmitTraceResult{}, err
	}
	return EmitTraceResult{
		Trace:        trace,
		TracePath:    tracePath,
		PolicyDigest: policyDigest,
		IntentDigest: normalizedIntent.IntentDigest,
	}, nil
}

func digestDelegationChain(delegation schemagate.IntentDelegation) (string, error) {
	raw, err := json.Marshal(delegation)
	if err != nil {
		return "", fmt.Errorf("marshal delegation: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest delegation: %w", err)
	}
	return digest, nil
}

func WriteTraceRecord(path string, trace schemagate.TraceRecord) error {
	normalizedPath, err := normalizeTracePath(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(normalizedPath)
	if dir != "." && dir != "" {
		if filepath.IsLocal(dir) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create trace directory: %w", err)
			}
		} else if strings.HasPrefix(dir, string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create trace directory: %w", err)
			}
		} else if volume := filepath.VolumeName(dir); volume != "" && strings.HasPrefix(dir, volume+string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create trace directory: %w", err)
			}
		} else {
			return fmt.Errorf("trace output directory must be local relative or absolute")
		}
	}
	encoded, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trace record: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(normalizedPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write trace record: %w", err)
	}
	return nil
}

func ReadTraceRecord(path string) (schemagate.TraceRecord, error) {
	// #nosec G304 -- trace path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemagate.TraceRecord{}, fmt.Errorf("read trace record: %w", err)
	}
	var trace schemagate.TraceRecord
	if err := json.Unmarshal(content, &trace); err != nil {
		return schemagate.TraceRecord{}, fmt.Errorf("parse trace record: %w", err)
	}
	return trace, nil
}

func VerifyTraceRecordSignature(trace schemagate.TraceRecord, publicKey ed25519.PublicKey) (bool, error) {
	if trace.Signature == nil {
		return false, fmt.Errorf("trace signature missing")
	}
	signature := sign.Signature{
		Alg:          trace.Signature.Alg,
		KeyID:        trace.Signature.KeyID,
		Sig:          trace.Signature.Sig,
		SignedDigest: trace.Signature.SignedDigest,
	}
	signable := trace
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return false, fmt.Errorf("marshal signable trace: %w", err)
	}
	return sign.VerifyTraceRecordJSON(publicKey, signature, signableRaw)
}

func computeTraceID(policyDigest, intentDigest, verdict string) string {
	sum := sha256.Sum256([]byte(policyDigest + ":" + intentDigest + ":" + verdict))
	return hex.EncodeToString(sum[:12])
}

func computeTraceEventID(traceID string, observedAt time.Time) string {
	sum := sha256.Sum256([]byte(traceID + ":" + observedAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:12])
}

func clampLatency(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func matchedRuleIDsFromStepVerdicts(stepVerdicts []schemagate.TraceStepVerdict) []string {
	if len(stepVerdicts) == 0 {
		return nil
	}
	matched := make([]string, 0, len(stepVerdicts))
	for _, step := range stepVerdicts {
		if ruleID := strings.TrimSpace(step.MatchedRule); ruleID != "" {
			matched = append(matched, ruleID)
		}
	}
	return uniqueSorted(matched)
}

func normalizeTracePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("trace path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("trace path is required")
	}
	if !filepath.IsAbs(cleaned) {
		for _, segment := range strings.Split(filepath.ToSlash(cleaned), "/") {
			if segment == ".." {
				return "", fmt.Errorf("relative trace path must not traverse parent directories")
			}
		}
	}
	return cleaned, nil
}
