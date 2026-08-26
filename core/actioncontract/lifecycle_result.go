package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
)

type LifecycleResultOptions struct {
	JournalPath         string
	Proposal            Artifact
	Activation          ActivatedArtifact
	RuntimeAction       RuntimeAction
	Readiness           ReadinessResult
	PrivateKey          ed25519.PrivateKey
	Outcome             string
	EffectOutcome       string
	CompensationOutcome string
	TraceDigest         string
	TraceID             string
	ResultDigest        string
	ActivationRawDigest string
	Now                 time.Time
}

// BuildContractEvidenceBinding creates the common digest-bound lineage used by
// execution and containment evidence producers.
func BuildContractEvidenceBinding(proposal Artifact, activation ActivatedArtifact, action RuntimeAction, readiness ReadinessResult, traceID, traceDigest, resultDigest, activationRawDigest string) (EvidenceBinding, error) {
	if proposal.ContractID == "" || activation.ArtifactID == "" || action.ActionID == "" {
		return EvidenceBinding{}, errors.New("contract evidence context incomplete")
	}
	if !validCanonicalDigest(traceDigest) || !validCanonicalDigest(resultDigest) {
		return EvidenceBinding{}, errors.New("trace/result digest required")
	}
	activationRaw, err := json.Marshal(activation)
	if err != nil {
		return EvidenceBinding{}, err
	}
	if activationRawDigest == "" {
		activationRawDigest = RawDigest(activationRaw)
	}
	if !validCanonicalDigest(activationRawDigest) {
		return EvidenceBinding{}, errors.New("activation digest required")
	}
	actionDigest, err := digestResult(action)
	if err != nil {
		return EvidenceBinding{}, err
	}
	readinessDigest, err := digestResult(readiness)
	if err != nil {
		return EvidenceBinding{}, err
	}
	contract := refResult("action_contract", proposal.ContractID, proposal.CanonicalContentDigest, ProposedContractSchemaID, ProposedContractVersion, ProposedProducer)
	activationRef := refResult("activated_action_contract", activation.ArtifactID, activationRawDigest, ActivatedSchemaID, ActivatedSchemaVersion, ActivatedProducer)
	resultRef := refResult("executor_result", "result", resultDigest, "gait.executor.result", "1", EvidenceProducer)
	binding := EvidenceBinding{ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ContractRef: contract, ActivationRef: activationRef, RuntimeActionRef: refResult("runtime_action", action.ActionID, actionDigest, RuntimeActionSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), ReadinessRef: refResult("readiness", "runtime-readiness", readinessDigest, RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), DecisionRef: refResult("decision", "runtime-readiness", readinessDigest, RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), PolicyRef: refResult("policy", "policy", activation.PolicyDigest, "gait.policy", "1", EvidenceProducer), TargetRef: refResult("target", activation.Target, textDigest(activation.Target), "gait.target", "1", EvidenceProducer), EnvironmentRef: refResult("environment", activation.Environment, textDigest(activation.Environment), "gait.environment", "1", EvidenceProducer), ProofRefs: []proof.RelationshipRef{refResult("trace", traceID, traceDigest, "gait.gate.trace", "1.0.0", EvidenceProducer), resultRef}, CausalRefs: []proof.RelationshipRef{contract, resultRef}}
	binding.Correlation = proof.ControlContainmentTelemetryProfile{ProfileVersion: CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &binding.ContractRef, ContentDigest: contract.Digest}
	return binding, binding.Validate()
}

func AppendLifecycleResult(options LifecycleResultOptions) ([]LifecycleRecord, error) {
	if strings.TrimSpace(options.JournalPath) == "" {
		return nil, errors.New("lifecycle journal path required")
	}
	if options.Outcome != "succeeded" && options.Outcome != "failed" {
		return nil, errors.New("execution outcome must be succeeded or failed")
	}
	if options.EffectOutcome != "" && options.Outcome != "succeeded" {
		return nil, errors.New("effect outcome requires successful execution")
	}
	if options.EffectOutcome != "" && options.EffectOutcome != "recorded" && options.EffectOutcome != "validated" {
		return nil, errors.New("effect outcome invalid")
	}
	if options.CompensationOutcome != "" && options.CompensationOutcome != "required" && options.CompensationOutcome != "started" && options.CompensationOutcome != "completed" {
		return nil, errors.New("compensation outcome invalid")
	}
	if len(options.PrivateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("lifecycle signing key required")
	}
	if options.RuntimeAction.ActionID == "" {
		return nil, errors.New("runtime action required")
	}
	if !validCanonicalDigest(options.TraceDigest) {
		return nil, errors.New("verified gate trace digest required")
	}
	if strings.TrimSpace(options.TraceID) == "" {
		return nil, errors.New("verified gate trace identity required")
	}
	if !validCanonicalDigest(options.ResultDigest) {
		return nil, errors.New("executor result digest required")
	}
	if options.Proposal.ContractID == "" || options.Activation.ArtifactID == "" {
		return nil, errors.New("proposal and activation required")
	}
	contract := refResult("action_contract", options.Proposal.ContractID, options.Proposal.CanonicalContentDigest, ProposedContractSchemaID, ProposedContractVersion, ProposedProducer)
	activationRaw, _ := json.Marshal(options.Activation)
	activationDigest := options.ActivationRawDigest
	if activationDigest == "" {
		activationDigest = RawDigest(activationRaw)
	}
	activation := refResult("activated_action_contract", options.Activation.ArtifactID, activationDigest, ActivatedSchemaID, ActivatedSchemaVersion, ActivatedProducer)
	actionDigest, err := digestResult(options.RuntimeAction)
	if err != nil {
		return nil, err
	}
	readinessDigest, err := digestResult(options.Readiness)
	if err != nil {
		return nil, err
	}
	resultRef := refResult("executor_result", "result", options.ResultDigest, "gait.executor.result", "1", EvidenceProducer)
	binding := EvidenceBinding{ContractFamilyID: options.Proposal.ContractFamilyID, Revision: options.Proposal.Revision, ContractRef: contract, ActivationRef: activation, RuntimeActionRef: refResult("runtime_action", options.RuntimeAction.ActionID, actionDigest, RuntimeActionSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), ReadinessRef: refResult("readiness", "runtime-readiness", readinessDigest, RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), DecisionRef: refResult("decision", "runtime-readiness", readinessDigest, RuntimeReadinessSchemaID, RuntimeActionSchemaVersion, EvidenceProducer), PolicyRef: refResult("policy", "policy", options.Activation.PolicyDigest, "gait.policy", "1", EvidenceProducer), TargetRef: refResult("target", options.Activation.Target, textDigest(options.Activation.Target), "gait.target", "1", EvidenceProducer), EnvironmentRef: refResult("environment", options.Activation.Environment, textDigest(options.Activation.Environment), "gait.environment", "1", EvidenceProducer), ProofRefs: []proof.RelationshipRef{refResult("trace", options.TraceID, options.TraceDigest, "gait.gate.trace", "1.0.0", EvidenceProducer), resultRef}, CausalRefs: []proof.RelationshipRef{contract, resultRef}}
	binding.Correlation = proof.ControlContainmentTelemetryProfile{ProfileVersion: CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &binding.ContractRef, ContentDigest: contract.Digest}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	fresh := now.Add(5 * time.Minute).Format(time.RFC3339Nano)
	startItem, err := NewExecutionEvidence(ExecutionEvidence{Binding: binding, EventRef: refResult("execution_event", "started", textDigest("started"), ExecutionEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), OccurredAt: now.Format(time.RFC3339Nano), FreshUntil: fresh, Outcome: "started", ReasonCode: "execution_started"}, options.PrivateKey)
	if err != nil {
		return nil, err
	}
	terminalBinding := binding
	terminalBinding.CausalRefs = []proof.RelationshipRef{evidenceRefForExecution(startItem)}
	terminalItem, err := NewExecutionEvidence(ExecutionEvidence{Binding: terminalBinding, EventRef: refResult("execution_event", options.Outcome, textDigest(options.Outcome), ExecutionEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), OccurredAt: now.Add(time.Nanosecond).Format(time.RFC3339Nano), FreshUntil: fresh, Outcome: options.Outcome, ReasonCode: "execution_" + options.Outcome, CompensationRequired: strings.TrimSpace(options.CompensationOutcome) != ""}, options.PrivateKey)
	if err != nil {
		return nil, err
	}
	startKind := LifecycleExecutionStarted
	terminalKind := LifecycleExecutionSucceeded
	if options.Outcome == "failed" {
		terminalKind = LifecycleExecutionFailed
	}
	first, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: startKind, OccurredAt: now, ContractRef: contract, ContractFamilyID: options.Proposal.ContractFamilyID, Revision: options.Proposal.Revision, ProposalRef: &contract, ActivationRef: &activation, Execution: &startItem, SigningPrivateKey: options.PrivateKey})
	if err != nil {
		return nil, err
	}
	second, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: terminalKind, OccurredAt: now.Add(time.Nanosecond), ContractRef: contract, ContractFamilyID: options.Proposal.ContractFamilyID, Revision: options.Proposal.Revision, ProposalRef: &contract, ActivationRef: &activation, Execution: &terminalItem, SigningPrivateKey: options.PrivateKey})
	if err != nil {
		return nil, err
	}
	if err := AppendLifecycleRecord(options.JournalPath, first); err != nil {
		return nil, err
	}
	if err := AppendLifecycleRecord(options.JournalPath, second); err != nil {
		return nil, err
	}
	records := []LifecycleRecord{first, second}
	effectTransitionCount := 0
	if options.EffectOutcome != "" {
		// Validation is a second lifecycle transition. Always establish the
		// recorded predecessor first so the reducer cannot be bypassed by a
		// caller asking directly for a validated effect.
		effectOutcomes := []string{options.EffectOutcome}
		if options.EffectOutcome == "validated" {
			effectOutcomes = []string{"recorded", "validated"}
		}
		effectTransitionCount = len(effectOutcomes)
		effectRef := refResult("effect", "effect", textDigest("recorded"), EffectEventSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer)
		causal := evidenceRefForExecution(terminalItem)
		for index, effectOutcome := range effectOutcomes {
			effectAt := now.Add((2 + time.Duration(index)) * time.Nanosecond)
			effectBinding := binding
			effectBinding.CausalRefs = []proof.RelationshipRef{causal}
			effectItem, effectErr := NewEffectEvent(EffectEvent{Binding: effectBinding, EventRef: refResult("effect_event", effectOutcome, textDigest(effectOutcome), EffectEventSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), ExecutionRef: refResult("execution", terminalItem.EvidenceID, terminalItem.CanonicalContentDigest, ExecutionEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), EffectRef: effectRef, OccurredAt: effectAt.Format(time.RFC3339Nano), FreshUntil: fresh, Outcome: effectOutcome, ReasonCode: "effect_" + effectOutcome}, options.PrivateKey)
			if effectErr != nil {
				return nil, effectErr
			}
			effectKind := LifecycleEffectRecorded
			if effectOutcome == "validated" {
				effectKind = LifecycleEffectValidated
			}
			effectRecord, effectErr := NewLifecycleRecord(LifecycleRecordOptions{Kind: effectKind, OccurredAt: effectAt, ContractRef: contract, ContractFamilyID: options.Proposal.ContractFamilyID, Revision: options.Proposal.Revision, ProposalRef: &contract, ActivationRef: &activation, Effect: &effectItem, SigningPrivateKey: options.PrivateKey})
			if effectErr != nil {
				return nil, effectErr
			}
			if effectErr = AppendLifecycleRecord(options.JournalPath, effectRecord); effectErr != nil {
				return nil, effectErr
			}
			records = append(records, effectRecord)
			causal = evidenceRefForEffect(effectItem)
		}
	}
	if options.CompensationOutcome != "" {
		requirementRef := refResult("compensation_requirement", "requirement", textDigest("requirement"), CompensationEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer)
		// Compensation follows the final effect transition. A validated effect
		// emits both recorded and validated records, so its requirement must not
		// share the validation timestamp.
		compensationBase := 2 + effectTransitionCount
		compensationSteps := []struct {
			outcome string
			kind    LifecycleEventKind
			at      time.Duration
		}{{"required", LifecycleCompensationRequired, time.Duration(compensationBase) * time.Nanosecond}}
		if options.CompensationOutcome == "started" || options.CompensationOutcome == "completed" {
			compensationSteps = append(compensationSteps, struct {
				outcome string
				kind    LifecycleEventKind
				at      time.Duration
			}{"started", LifecycleCompensationStarted, time.Duration(compensationBase+1) * time.Nanosecond})
		}
		if options.CompensationOutcome == "completed" {
			compensationSteps = append(compensationSteps, struct {
				outcome string
				kind    LifecycleEventKind
				at      time.Duration
			}{"completed", LifecycleCompensationCompleted, time.Duration(compensationBase+2) * time.Nanosecond})
		}
		causal := evidenceRefForExecution(terminalItem)
		for _, step := range compensationSteps {
			compBinding := binding
			compBinding.CausalRefs = []proof.RelationshipRef{causal}
			compItem, compErr := NewCompensationEvidence(CompensationEvidence{Binding: compBinding, EventRef: refResult("compensation_event", step.outcome, textDigest(step.outcome), CompensationEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), RequirementRef: requirementRef, ExecutionRef: refResult("execution", terminalItem.EvidenceID, terminalItem.CanonicalContentDigest, ExecutionEvidenceSchemaID, ExecutionEvidenceSchemaVersion, EvidenceProducer), OccurredAt: now.Add(step.at).Format(time.RFC3339Nano), FreshUntil: fresh, Outcome: step.outcome, ReasonCode: "compensation_" + step.outcome}, options.PrivateKey)
			if compErr != nil {
				return nil, compErr
			}
			compRecord, compErr := NewLifecycleRecord(LifecycleRecordOptions{Kind: step.kind, OccurredAt: now.Add(step.at), ContractRef: contract, ContractFamilyID: options.Proposal.ContractFamilyID, Revision: options.Proposal.Revision, ProposalRef: &contract, ActivationRef: &activation, Compensation: &compItem, SigningPrivateKey: options.PrivateKey})
			if compErr != nil {
				return nil, compErr
			}
			if compErr = AppendLifecycleRecord(options.JournalPath, compRecord); compErr != nil {
				return nil, compErr
			}
			records = append(records, compRecord)
			causal = evidenceRefForCompensation(compItem)
		}
	}
	return records, nil
}

func refResult(kind, id, digest, schema, version, product string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: digest, SchemaID: schema, SchemaVersion: version, SourceProduct: product}
}
func textDigest(value string) string {
	d, _ := proofcanon.DigestJCS([]byte(fmt.Sprintf("%q", value)))
	if !strings.HasPrefix(d, "sha256:") {
		d = "sha256:" + d
	}
	return d
}
func digestResult(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	d, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(d, "sha256:") {
		d = "sha256:" + d
	}
	return d, nil
}

// DigestReadinessResult returns the canonical JCS digest of the exact
// readiness decision used by a Gate evaluation.
func DigestReadinessResult(readiness ReadinessResult) (string, error) {
	return digestResult(readiness)
}

// DigestCanonicalJSON validates and hashes a structured JSON result using the
// same JCS implementation as all Gait evidence digests.
func DigestCanonicalJSON(raw []byte) (string, error) {
	var value any
	if err := DecodeStrictRuntimeJSON(raw, &value); err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	return digest, nil
}
