package main

import (
	"crypto/ed25519"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	proof "github.com/Clyra-AI/proof"
)

// gateActionContractRuntime is deliberately a projection used by gate eval;
// the signed proposal and activation remain the source of truth.
type gateActionContractRuntime struct {
	Selected         bool
	Block            bool
	ContractID       string
	ContractFamilyID string
	Revision         int
	ActivationDigest string
	Mode             string
	Action           *actioncontract.RuntimeAction
	Readiness        *actioncontract.ReadinessResult
	ReadinessDigest  string
	ReasonCodes      []string
	ProposalRef      proof.RelationshipRef
	ActivationRef    proof.RelationshipRef
}

// evaluateGateActionContract verifies the complete proposal/activation pair
// before a gate verdict reaches a tool boundary. Context digests are checked
// against verified artifacts, never used as caller-provided authority.
func evaluateGateActionContract(proposalPath, activationPath, publicPath, publicEnv, validators string, validatorKeys []string, allowDevelopment bool, evaluatedPolicyDigest string, intent schemagate.IntentRequest, now time.Time) (gateActionContractRuntime, error) {
	selected := strings.TrimSpace(proposalPath) != "" || strings.TrimSpace(activationPath) != "" || strings.TrimSpace(intent.Context.ContractID) != "" || strings.TrimSpace(intent.Context.ContractFamilyID) != "" || strings.TrimSpace(intent.Context.ProposalDigest) != "" || strings.TrimSpace(intent.Context.ActivationDigest) != ""
	if !selected {
		return gateActionContractRuntime{}, nil
	}
	fail := func(reasons ...string) (gateActionContractRuntime, error) {
		return gateActionContractRuntime{Selected: true, ReasonCodes: reasons}, &actioncontract.ValidationError{Reasons: reasons}
	}
	if strings.TrimSpace(proposalPath) == "" || strings.TrimSpace(activationPath) == "" {
		return fail(actioncontract.ReasonSelectionRequired)
	}
	proposal, raw, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		return fail(actioncontract.ReasonMalformedArtifact)
	}
	_, validation := actioncontract.ValidateArtifactBytes(raw, actioncontract.ValidationOptions{Now: now})
	if !validation.Valid {
		return fail(validation.Reasons...)
	}
	activation, activationRaw, err := actioncontract.ReadActivatedArtifact(activationPath)
	if err != nil {
		return fail(actioncontract.ReasonMalformedArtifact)
	}
	var public ed25519.PublicKey
	if strings.TrimSpace(publicPath) != "" || strings.TrimSpace(publicEnv) != "" {
		public, err = loadActionContractPublicKey(publicPath, publicEnv)
		if err != nil {
			return fail(actioncontract.ReasonSigningKeyRequired)
		}
	} else if allowDevelopment && activation.DevelopmentSigning {
		public = actioncontract.DevelopmentPublicKey()
	} else {
		return fail(actioncontract.ReasonSigningKeyRequired)
	}
	valid, verifyErr := actioncontract.VerifyActivationWithOptions(activation, public, actioncontract.VerificationOptions{AllowDevelopmentSigning: allowDevelopment, Proposal: &proposal, EvaluationTime: now})
	if verifyErr != nil || !valid {
		if ve := actionContractReasonCodes(verifyErr); len(ve) > 0 {
			return fail(ve...)
		}
		return fail(actioncontract.ReasonBindingMismatch)
	}
	if strings.TrimPrefix(strings.TrimSpace(activation.PolicyDigest), "sha256:") != strings.TrimPrefix(strings.TrimSpace(evaluatedPolicyDigest), "sha256:") {
		return fail(actioncontract.ReasonBindingMismatch)
	}
	activationDigest := actioncontract.RawDigest(activationRaw)
	if intent.Context.ContractID != "" && intent.Context.ContractID != proposal.ContractID || intent.Context.ContractFamilyID != "" && intent.Context.ContractFamilyID != proposal.ContractFamilyID || intent.Context.ProposalDigest != "" && !sameActionContractDigest(intent.Context.ProposalDigest, proposal.CanonicalContentDigest) || intent.Context.ActivationDigest != "" && !sameActionContractDigest(intent.Context.ActivationDigest, activationDigest) {
		return fail(actioncontract.ReasonBindingMismatch)
	}
	trusted := parseActionContractCSV(validators)
	keys, keyErr := loadTrustedValidatorKeys(validatorKeys)
	if keyErr != nil {
		return fail("validator_key_invalid")
	}
	classification := actioncontract.ClassifyArtifact(proposal)
	readiness := actioncontract.ReadinessFromArtifact(proposal, actioncontract.ReadinessInput{Now: now, PolicyDigest: activation.PolicyDigest, TrustedValidatorRefs: trusted, TrustedValidatorKeys: keys})
	readinessDigest, digestErr := actioncontract.DigestReadinessResult(readiness)
	if digestErr != nil {
		return fail("readiness_digest_failed")
	}
	reasons := append([]string{}, classification.ReasonCodes...)
	reasons = append(reasons, readiness.ReasonCodes...)
	mode := string(activation.ActivationMode)
	block := !classification.Valid
	if (activation.ActivationMode == actioncontract.ActivationEnforceFloor || activation.ActivationMode == actioncontract.ActivationRequired) && !readiness.Ready {
		block = true
		if len(readiness.ReasonCodes) == 0 {
			reasons = append(reasons, "contract_readiness_not_satisfied")
		}
	}
	proposalRef := proof.RelationshipRef{Kind: "action_contract", ID: proposal.ContractID, Digest: proposal.CanonicalContentDigest, SchemaID: actioncontract.ProposedContractSchemaID, SchemaVersion: actioncontract.ProposedContractVersion, SourceProduct: actioncontract.ProposedProducer}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: activation.ArtifactID, Digest: activationDigest, SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, SourceProduct: actioncontract.ActivatedProducer}
	return gateActionContractRuntime{Selected: true, Block: block, ContractID: proposal.ContractID, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ActivationDigest: activationDigest, Mode: mode, Action: &classification.Action, Readiness: &readiness, ReadinessDigest: readinessDigest, ProposalRef: proposalRef, ActivationRef: activationRef, ReasonCodes: acUniqueSortedStrings(reasons)}, nil
}

func sameActionContractDigest(left, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(left), "sha256:"), strings.TrimPrefix(strings.TrimSpace(right), "sha256:"))
}

func acUniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	// stable ordering without importing a second helper package in gate.go.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
