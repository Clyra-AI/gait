package actioncontract

// Lifecycle conformance is the read-only, composite verifier for a complete
// Action Contract lineage. It binds the released Wrkr proposal, Gait
// activation, readiness projections and typed boundary evidence together. It
// never executes a tool and intentionally fails closed when a claim cannot be
// verified.

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
)

const (
	ReasonConformanceInputMissing      = "conformance_input_missing"
	ReasonConformanceProposalInvalid   = "conformance_proposal_invalid"
	ReasonConformanceActivationInvalid = "conformance_activation_invalid"
	ReasonConformanceRuntimeInvalid    = "conformance_runtime_invalid"
	ReasonConformanceReadinessInvalid  = "conformance_readiness_invalid"
	ReasonConformanceLineageMissing    = "conformance_lineage_missing"
	ReasonConformanceLineageMismatch   = "conformance_lineage_mismatch"
	ReasonConformanceIdentifierOnly    = "conformance_identifier_only"
	ReasonConformanceEvidenceMissing   = "conformance_evidence_missing"
	ReasonConformanceReplay            = "conformance_replay"
	ReasonConformanceReordered         = "conformance_reordered"
	ReasonConformanceVerification      = "conformance_verification_failed"
)

// LifecycleConformanceExpectation describes the terminal path that a fixture
// or caller expects. Empty values mean that the corresponding stage is not
// required (for example a blocked-before-execution path).
type LifecycleConformanceExpectation struct {
	ExecutionOutcome    string `json:"execution_outcome,omitempty"`
	EffectOutcome       string `json:"effect_outcome,omitempty"`
	ContainmentOutcome  string `json:"containment_outcome,omitempty"`
	CompensationOutcome string `json:"compensation_outcome,omitempty"`
	RequireComplete     bool   `json:"require_complete,omitempty"`
}

// LifecycleConformanceInput is deliberately composed of already parsed
// artifacts. File loading and execution are outside this API.
type LifecycleConformanceInput struct {
	Proposal                      Artifact
	Activation                    ActivatedArtifact
	ActivationPublicKey           ed25519.PublicKey
	RuntimeAction                 RuntimeAction
	Readiness                     ReadinessResult
	ReadinessTrustedValidatorRefs []string
	ReadinessTrustedValidatorKeys map[string]ed25519.PublicKey
	LifecycleRecords              []LifecycleRecord
	LifecyclePublicKey            ed25519.PublicKey
	EvaluationTime                time.Time
	AllowDevelopmentSign          bool
	Expectation                   LifecycleConformanceExpectation
}

type LifecycleConformanceResult struct {
	Valid                bool               `json:"valid"`
	AuthoritativeSuccess bool               `json:"authoritative_success"`
	ReasonCodes          []string           `json:"reason_codes,omitempty"`
	Snapshot             *LifecycleSnapshot `json:"snapshot,omitempty"`
}

func conformanceReason(result *LifecycleConformanceResult, reason string) {
	for _, existing := range result.ReasonCodes {
		if existing == reason {
			return
		}
	}
	result.ReasonCodes = append(result.ReasonCodes, reason)
}

// VerifyLifecycleConformance verifies the complete proposal -> activation ->
// readiness -> execution -> effect -> containment/compensation chain. It
// returns a stable reason code instead of treating an incomplete or
// identifier-only correlation as a successful partial result.
func VerifyLifecycleConformance(input LifecycleConformanceInput) LifecycleConformanceResult {
	result := LifecycleConformanceResult{}
	if strings.TrimSpace(input.Proposal.ContractID) == "" || strings.TrimSpace(input.Activation.ArtifactID) == "" || strings.TrimSpace(input.RuntimeAction.ActionID) == "" || strings.TrimSpace(input.Readiness.ContractID) == "" || len(input.ReadinessTrustedValidatorRefs) == 0 || len(input.ReadinessTrustedValidatorKeys) == 0 || len(input.LifecycleRecords) == 0 {
		conformanceReason(&result, ReasonConformanceInputMissing)
		return result
	}
	if input.EvaluationTime.IsZero() {
		conformanceReason(&result, ReasonConformanceInputMissing)
		return result
	}
	proposalValidation := ValidateArtifact(input.Proposal, ValidationOptions{Now: input.EvaluationTime})
	if !proposalValidation.Valid {
		conformanceReason(&result, ReasonConformanceProposalInvalid)
		for _, reason := range proposalValidation.Reasons {
			conformanceReason(&result, reason)
		}
		return result
	}
	validActivation, activationErr := VerifyActivationWithOptions(input.Activation, input.ActivationPublicKey, VerificationOptions{
		AllowDevelopmentSigning: input.AllowDevelopmentSign,
		Proposal:                &input.Proposal,
		EvaluationTime:          input.EvaluationTime,
	})
	if activationErr != nil || !validActivation {
		conformanceReason(&result, ReasonConformanceActivationInvalid)
		if activationErr != nil {
			conformanceReason(&result, stableConformanceError(activationErr))
		}
		return result
	}
	if reasons := ValidateRuntimeAction(input.RuntimeAction); len(reasons) > 0 {
		conformanceReason(&result, ReasonConformanceRuntimeInvalid)
		for _, reason := range reasons {
			conformanceReason(&result, reason)
		}
		return result
	}
	readinessRaw, readinessMarshalErr := json.Marshal(input.Readiness)
	if readinessMarshalErr != nil || validateRuntimeSchema(readinessRaw, RuntimeReadinessSchemaID) != nil || !input.Readiness.Ready || input.Readiness.Status != ReadinessSatisfied || input.Readiness.ContractID != input.Proposal.ContractID {
		conformanceReason(&result, ReasonConformanceReadinessInvalid)
		return result
	}
	decisionTime := time.Time{}
	for _, record := range input.LifecycleRecords {
		if record.Kind != LifecycleDecisionReady {
			continue
		}
		if !decisionTime.IsZero() {
			conformanceReason(&result, ReasonConformanceReadinessInvalid)
			return result
		}
		parsed, err := time.Parse(time.RFC3339Nano, record.OccurredAt)
		if err != nil {
			conformanceReason(&result, ReasonConformanceReadinessInvalid)
			return result
		}
		decisionTime = parsed
	}
	if decisionTime.IsZero() {
		conformanceReason(&result, ReasonConformanceReadinessInvalid)
		return result
	}
	revalidatedReadiness := EvaluateReadiness(ReadinessInput{Now: decisionTime, ContractID: input.Readiness.ContractID, Preconditions: input.Readiness.Preconditions, TrustedValidatorRefs: input.ReadinessTrustedValidatorRefs, TrustedValidatorKeys: input.ReadinessTrustedValidatorKeys, PolicyDigest: input.Readiness.PolicyDigest})
	if !revalidatedReadiness.Ready || revalidatedReadiness.Status != ReadinessSatisfied {
		conformanceReason(&result, ReasonConformanceReadinessInvalid)
		for _, reason := range revalidatedReadiness.ReasonCodes {
			conformanceReason(&result, reason)
		}
		return result
	}
	for _, precondition := range revalidatedReadiness.Preconditions {
		if (precondition.Environment != "" && precondition.Environment != input.Activation.Environment) || (precondition.Target != "" && precondition.Target != input.Activation.Target) {
			conformanceReason(&result, ReasonConformanceLineageMismatch)
			return result
		}
	}
	runtimeDigest, runtimeDigestErr := conformanceDigest(input.RuntimeAction)
	readinessDigest, readinessDigestErr := conformanceDigest(input.Readiness)
	revalidatedReadinessDigest, revalidatedDigestErr := conformanceDigest(revalidatedReadiness)
	if runtimeDigestErr != nil || readinessDigestErr != nil || revalidatedDigestErr != nil || readinessDigest != revalidatedReadinessDigest {
		conformanceReason(&result, ReasonConformanceVerification)
		return result
	}
	proposalRef := relationshipForProposal(input.Proposal)
	activationRef := relationshipForActivation(input.Activation)
	for _, record := range input.LifecycleRecords {
		if !sameLifecycleRefIdentity(&record.ContractRef, &proposalRef) || record.ContractFamilyID != input.Proposal.ContractFamilyID || record.Revision != input.Proposal.Revision {
			conformanceReason(&result, ReasonConformanceLineageMismatch)
			return result
		}
		requiresActivation := record.Kind == LifecycleActivated || record.Execution != nil || record.Effect != nil || record.Containment != nil || record.Compensation != nil
		if record.ActivationRef != nil && !sameLifecycleRefIdentity(record.ActivationRef, &activationRef) || requiresActivation && record.ActivationRef == nil {
			conformanceReason(&result, ReasonConformanceLineageMissing)
			return result
		}
		if record.Correlation.BindingMode == proof.BindingModeIdentifierOnly {
			conformanceReason(&result, ReasonConformanceIdentifierOnly)
			return result
		}
		if record.Decision != nil {
			recordDecisionDigest, err := conformanceDigest(*record.Decision)
			if err != nil || recordDecisionDigest != readinessDigest {
				conformanceReason(&result, ReasonConformanceLineageMismatch)
				return result
			}
		}
		binding := lifecycleEvidenceBinding(record)
		if binding != nil && (binding.RuntimeActionRef.ID != input.RuntimeAction.ActionID || binding.RuntimeActionRef.Digest != runtimeDigest || binding.ReadinessRef.Digest != readinessDigest || binding.DecisionRef.Digest != readinessDigest || binding.PolicyRef.Kind != "policy" || binding.PolicyRef.Digest != input.Readiness.PolicyDigest || binding.PolicyRef.Digest != input.Activation.PolicyDigest || binding.TargetRef.Kind != "target" || binding.TargetRef.ID != input.Activation.Target || binding.TargetRef.Digest != conformanceTextDigest(input.Activation.Target) || binding.EnvironmentRef.Kind != "environment" || binding.EnvironmentRef.ID != input.Activation.Environment || binding.EnvironmentRef.Digest != conformanceTextDigest(input.Activation.Environment) || input.RuntimeAction.TargetRef != input.Activation.Target) {
			conformanceReason(&result, ReasonConformanceLineageMismatch)
			return result
		}
	}
	snapshot, reduceErr := ReduceVerifiedLifecycle(input.LifecycleRecords, input.LifecyclePublicKey)
	if reduceErr != nil {
		reason := ReasonConformanceVerification
		message := reduceErr.Error()
		switch {
		case strings.Contains(message, "reorder"):
			reason = ReasonConformanceReordered
		case strings.Contains(message, "replay") || strings.Contains(message, "duplicate"):
			reason = ReasonConformanceReplay
		case strings.Contains(message, "missing"):
			reason = ReasonConformanceEvidenceMissing
		case strings.Contains(message, "identifier_only"):
			reason = ReasonConformanceIdentifierOnly
		}
		conformanceReason(&result, reason)
		return result
	}
	result.Snapshot = &snapshot
	if err := checkConformanceExpectation(snapshot, input.Expectation); err != nil {
		conformanceReason(&result, ReasonConformanceEvidenceMissing)
		conformanceReason(&result, err.Error())
		return result
	}
	result.Valid = true
	result.AuthoritativeSuccess = snapshot.ExecutionStatus == "succeeded" && snapshot.EffectStatus == "validated" && snapshot.ContainmentStatus == "completed" && (snapshot.CompensationStatus == "" || snapshot.CompensationStatus == "completed")
	return result
}

func lifecycleEvidenceBinding(record LifecycleRecord) *EvidenceBinding {
	switch {
	case record.Execution != nil:
		return &record.Execution.Binding
	case record.Effect != nil:
		return &record.Effect.Binding
	case record.Containment != nil:
		return &record.Containment.Binding
	case record.Compensation != nil:
		return &record.Compensation.Binding
	default:
		return nil
	}
}

func conformanceDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	return "sha256:" + strings.TrimPrefix(digest, "sha256:"), nil
}

func conformanceTextDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// GradeLifecycleConformance is the verb-oriented alias used by regression
// adapters. It is intentionally pure and has no filesystem or tool access.
func GradeLifecycleConformance(input LifecycleConformanceInput) LifecycleConformanceResult {
	return VerifyLifecycleConformance(input)
}

func checkConformanceExpectation(snapshot LifecycleSnapshot, expected LifecycleConformanceExpectation) error {
	if expected.ExecutionOutcome != "" && snapshot.ExecutionStatus != expected.ExecutionOutcome {
		return fmt.Errorf("execution_outcome_mismatch")
	}
	if expected.EffectOutcome != "" && snapshot.EffectStatus != expected.EffectOutcome {
		return fmt.Errorf("effect_outcome_mismatch")
	}
	if expected.ContainmentOutcome != "" && snapshot.ContainmentStatus != expected.ContainmentOutcome {
		return fmt.Errorf("containment_outcome_mismatch")
	}
	if expected.CompensationOutcome != "" && snapshot.CompensationStatus != expected.CompensationOutcome {
		return fmt.Errorf("compensation_outcome_mismatch")
	}
	if expected.RequireComplete && (snapshot.ExecutionStatus == "" || snapshot.EffectStatus == "" || snapshot.ContainmentStatus == "") {
		return errors.New("complete_lineage_required")
	}
	return nil
}

func relationshipForProposal(proposal Artifact) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: "action_contract", ID: proposal.ContractID, Digest: proposal.CanonicalContentDigest, SchemaID: ProposedContractSchemaID, SchemaVersion: ProposedContractVersion, SourceProduct: ProposedProducer}
}

func relationshipForActivation(activation ActivatedArtifact) proof.RelationshipRef {
	digest, err := activatedSignableDigest(activation)
	if err != nil {
		return proof.RelationshipRef{}
	}
	digest = "sha256:" + strings.TrimPrefix(digest, "sha256:")
	return proof.RelationshipRef{Kind: "activated_action_contract", ID: activation.ArtifactID, Digest: digest, SchemaID: ActivatedSchemaID, SchemaVersion: ActivatedSchemaVersion, SourceProduct: ActivatedProducer}
}

func stableConformanceError(err error) string {
	if err == nil {
		return ReasonConformanceVerification
	}
	if validationErr := new(ValidationError); errors.As(err, &validationErr) && len(validationErr.Reasons) > 0 {
		return validationErr.Reasons[0]
	}
	return ReasonConformanceVerification
}
