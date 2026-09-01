package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/fsx"
	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
)

type containmentOutput struct {
	OK             bool     `json:"ok"`
	Status         string   `json:"status"`
	ReasonCodes    []string `json:"reason_codes,omitempty"`
	StatePath      string   `json:"state_path,omitempty"`
	RevocationPath string   `json:"revocation_path,omitempty"`
	Error          string   `json:"error,omitempty"`
	JournalPath    string   `json:"journal_path,omitempty"`
	ReceiptPath    string   `json:"receipt_path,omitempty"`
}

func runContainment(args []string) int {
	if len(args) == 0 || args[0] != "stop" {
		fmt.Println("Usage: gait containment stop --proposal proposal.json --activation activated.json --trace trace.json --trace-public-key key --activation-public-key key --evaluation-time RFC3339 [--occurrence-time RFC3339] --private-key key --journal lifecycle.jsonl --receipt-out receipt.json --identity identity --boundary-id boundary --adapter-identity adapter --resource-id resource [--trusted-validators csv --trusted-validator-key producer=key] [--allow-development-signing] [--state state.json] [--revocation-registry path] [--external-command cmd] [--json]")
		return exitInvalidInput
	}
	f := flag.NewFlagSet("containment-stop", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var statePath, identity, toolName, targetValue, revocationPath, external, proposalPath, activationPath, tracePath, tracePublicPath, activationPublicPath, evaluationTime, occurrenceTime, privateKeyPath, journalPath, receiptPath, boundaryID, adapterIdentity, resourceID, tokenIDs, trustedValidators string
	var trustedValidatorKeys repeatStringFlag
	var timeout time.Duration
	var js, allowDevelopment bool
	f.StringVar(&statePath, "state", "./.gait-out/kill_switch_state.json", "persistent kill-switch state")
	f.StringVar(&proposalPath, "proposal", "", "verified Wrkr proposal")
	f.StringVar(&activationPath, "activation", "", "verified Gait activation")
	f.StringVar(&tracePath, "trace", "", "signed Gate trace")
	f.StringVar(&tracePublicPath, "trace-public-key", "", "trace verification key")
	f.StringVar(&activationPublicPath, "activation-public-key", "", "activation verification key")
	f.StringVar(&evaluationTime, "evaluation-time", "", "explicit RFC3339 evaluation time")
	f.StringVar(&occurrenceTime, "occurrence-time", "", "explicit RFC3339 containment occurrence time (default current UTC time)")
	f.StringVar(&privateKeyPath, "private-key", "", "lifecycle signing key")
	f.StringVar(&journalPath, "journal", "", "existing lifecycle journal")
	f.StringVar(&receiptPath, "receipt-out", "", "signed containment receipt output")
	f.StringVar(&identity, "identity", "", "identity scope")
	f.StringVar(&toolName, "tool-name", "", "tool scope")
	f.StringVar(&targetValue, "target-value", "", "target scope")
	f.StringVar(&boundaryID, "boundary-id", "", "explicit containment boundary")
	f.StringVar(&adapterIdentity, "adapter-identity", "", "explicit adapter identity")
	f.StringVar(&resourceID, "resource-id", "", "explicit resource identity")
	f.StringVar(&revocationPath, "revocation-registry", "", "persistent native capability revocation registry")
	f.StringVar(&tokenIDs, "token-ids", "", "comma-separated approval/JIT/delegation/descendant token IDs or digests")
	f.StringVar(&trustedValidators, "trusted-validators", "", "comma-separated policy-named trusted validator references")
	f.Var(&trustedValidatorKeys, "trusted-validator-key", "authoritative validator key as producer=public-key-path (repeatable)")
	f.StringVar(&external, "external-command", "", "optional external revocation adapter command")
	f.DurationVar(&timeout, "timeout", 5*time.Second, "external adapter timeout")
	f.BoolVar(&js, "json", false, "emit JSON")
	f.BoolVar(&allowDevelopment, "allow-development-signing", false, "TEST ONLY: permit development-signed activation")
	if err := f.Parse(args[1:]); err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	if strings.TrimSpace(identity) == "" || proposalPath == "" || activationPath == "" || tracePath == "" || tracePublicPath == "" || activationPublicPath == "" || evaluationTime == "" || privateKeyPath == "" || journalPath == "" || receiptPath == "" || boundaryID == "" || adapterIdentity == "" || resourceID == "" {
		return writeContainment(js, containmentOutput{Error: "--identity, --proposal, --activation, --trace, --trace-public-key, --activation-public-key, --evaluation-time, --private-key, --journal, --receipt-out, --boundary-id, --adapter-identity, and --resource-id are required"}, exitInvalidInput)
	}
	when, err := time.Parse(time.RFC3339, evaluationTime)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	containmentAt := time.Now().UTC()
	if strings.TrimSpace(occurrenceTime) != "" {
		containmentAt, err = time.Parse(time.RFC3339, occurrenceTime)
		if err != nil {
			return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
		}
		containmentAt = containmentAt.UTC()
	}
	trace, err := gate.ReadTraceRecord(tracePath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitVerifyFailed)
	}
	tracePublic, err := proofsign.LoadPublicKeyBase64(tracePublicPath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	if valid, verifyErr := gate.VerifyTraceRecordSignature(trace, tracePublic); verifyErr != nil || !valid {
		return writeContainment(js, containmentOutput{Error: "trace verification failed"}, exitVerifyFailed)
	}
	proposal, _, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitVerifyFailed)
	}
	activation, _, err := actioncontract.ReadActivatedArtifact(activationPath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitVerifyFailed)
	}
	activationPublic, err := proofsign.LoadPublicKeyBase64(activationPublicPath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	if valid, verifyErr := actioncontract.VerifyActivationWithOptions(activation, activationPublic, actioncontract.VerificationOptions{Proposal: &proposal, EvaluationTime: when, AllowDevelopmentSigning: allowDevelopment}); verifyErr != nil || !valid {
		return writeContainment(js, containmentOutput{Error: "activation verification failed"}, exitVerifyFailed)
	}
	activationRaw, readActivationErr := actioncontract.ReadRuntimeInput(activationPath)
	if readActivationErr != nil {
		return writeContainment(js, containmentOutput{Error: readActivationErr.Error()}, exitVerifyFailed)
	}
	if trace.ContractID != proposal.ContractID || trace.ContractFamilyID != proposal.ContractFamilyID || trace.ContractRevision != proposal.Revision || trace.ProposalDigest != strings.TrimPrefix(proposal.CanonicalContentDigest, "sha256:") || trace.ActivationDigest != strings.TrimPrefix(actioncontract.RawDigest(activationRaw), "sha256:") || strings.TrimPrefix(trace.PolicyDigest, "sha256:") != strings.TrimPrefix(activation.PolicyDigest, "sha256:") {
		return writeContainment(js, containmentOutput{Error: "trace action contract binding mismatch"}, exitVerifyFailed)
	}
	private, err := proofsign.LoadPrivateKeyBase64(privateKeyPath)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	classification := actioncontract.ClassifyArtifact(proposal)
	trusted := parseActionContractCSV(trustedValidators)
	trustedKeys, keyErr := loadTrustedValidatorKeys(trustedValidatorKeys)
	if keyErr != nil {
		return writeContainment(js, containmentOutput{Error: keyErr.Error()}, exitInvalidInput)
	}
	readiness := actioncontract.ReadinessFromArtifact(proposal, actioncontract.ReadinessInput{PolicyDigest: activation.PolicyDigest, Now: when, TrustedValidatorRefs: trusted, TrustedValidatorKeys: trustedKeys})
	readinessDigest, digestErr := actioncontract.DigestReadinessResult(readiness)
	if digestErr != nil || strings.TrimSpace(trace.ReadinessDigest) == "" || strings.TrimPrefix(trace.ReadinessDigest, "sha256:") != strings.TrimPrefix(readinessDigest, "sha256:") {
		return writeContainment(js, containmentOutput{Error: "trace readiness digest does not match verified readiness"}, exitVerifyFailed)
	}
	traceDigest := trace.Signature.SignedDigest
	if !strings.HasPrefix(traceDigest, "sha256:") {
		traceDigest = "sha256:" + traceDigest
	}
	resultDigest := containmentScopeDigest(identity, boundaryID, resourceID)
	binding, err := actioncontract.BuildContractEvidenceBinding(proposal, activation, classification.Action, readiness, trace.TraceID, traceDigest, resultDigest, actioncontract.RawDigest(activationRaw))
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitVerifyFailed)
	}
	// Evaluation time controls activation/readiness validity; occurrence time
	// records when this operational stop actually happened.
	now := containmentAt
	state, err := loadOrCreateKillSwitchState(statePath, now)
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	entry, err := gate.NewKillSwitchEntry(now, schemagate.KillSwitchEntry{Enabled: true, Identity: identity, ToolName: toolName, TargetValue: targetValue, Reason: "containment_stop", Actor: "gait-containment"})
	if err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	state.Entries = append(state.Entries, entry)
	state.UpdatedAt = now
	if err = gate.WriteKillSwitchState(statePath, state); err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInvalidInput)
	}
	status := "contained"
	reasons := []string{"kill_switch_persisted"}
	localAdapter := strings.TrimSpace(adapterIdentity) == "gait-local-kill-switch"
	if external == "" && !localAdapter {
		status = "partial"
		reasons = append(reasons, "external_revocation_unconfigured")
	}
	if revocationPath != "" {
		if err := appendRevocation(revocationPath, identity, parseCSV(tokenIDs), now); err != nil {
			status = "partial"
			reasons = append(reasons, "revocation_registry_write_failed")
		}
	}
	if external != "" {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		command := strings.Fields(external)
		if len(command) == 0 {
			status = "partial"
			reasons = append(reasons, "external_revocation_invalid")
		} else if err := exec.CommandContext(ctx, command[0], command[1:]...).Run(); err != nil { // #nosec G204 -- explicit operator-selected adapter command; executed without a shell.
			status = "partial"
			reasons = append(reasons, "external_revocation_failed")
		} else {
			reasons = append(reasons, "external_revocation_acknowledged")
		}
	}
	terminalPhase := "acknowledged"
	if status != "contained" {
		terminalPhase = "failed"
	}
	controlRef := proof.RelationshipRef{Kind: "containment_control", ID: boundaryID, Digest: resultDigest, SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}
	makeControl := func(controlBinding actioncontract.EvidenceBinding, causalRef proof.RelationshipRef, phase string, acknowledged bool) (actioncontract.ControlEventEvidence, error) {
		return actioncontract.NewControlEventEvidence(actioncontract.ControlEventEvidence{Binding: controlBinding, EventRef: proof.RelationshipRef{Kind: "control_event", ID: "stop-" + phase, Digest: resultDigest, SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}, CausalRef: causalRef, ControlRef: controlRef, Command: "stop", Phase: phase, BoundaryID: boundaryID, ResourceID: resourceID, AffectedScope: []string{identity}, AdapterIdentity: adapterIdentity, AdapterAcknowledged: acknowledged, OccurredAt: now.Format(time.RFC3339Nano), FreshUntil: now.Add(5 * time.Minute).Format(time.RFC3339Nano), ReasonCode: "containment_" + phase}, private)
	}
	requested, controlErr := makeControl(binding, binding.ContractRef, "requested", false)
	if controlErr != nil {
		return writeContainment(js, containmentOutput{Error: controlErr.Error()}, exitInternalFailure)
	}
	terminalBinding := binding
	terminalBinding.CausalRefs = []proof.RelationshipRef{{Kind: "control", ID: requested.EvidenceID, Digest: requested.CanonicalContentDigest, SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}}
	requestedRef := proof.RelationshipRef{Kind: "control", ID: requested.EvidenceID, Digest: requested.CanonicalContentDigest, SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}
	terminal, controlErr := makeControl(terminalBinding, requestedRef, terminalPhase, terminalPhase == "acknowledged" && (external != "" || localAdapter))
	if controlErr != nil {
		return writeContainment(js, containmentOutput{Error: controlErr.Error()}, exitInternalFailure)
	}
	requestedRecord, controlErr := actioncontract.NewLifecycleRecord(actioncontract.LifecycleRecordOptions{Kind: actioncontract.LifecycleStopRequested, OccurredAt: now, ContractRef: binding.ContractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ProposalRef: &binding.ContractRef, ActivationRef: &binding.ActivationRef, Control: &requested, SigningPrivateKey: private})
	if controlErr != nil {
		return writeContainment(js, containmentOutput{Error: controlErr.Error()}, exitInternalFailure)
	}
	terminalKind := actioncontract.LifecycleStopAcknowledged
	if terminalPhase == "failed" {
		terminalKind = actioncontract.LifecycleStopFailed
	}
	terminalRecord, controlErr := actioncontract.NewLifecycleRecord(actioncontract.LifecycleRecordOptions{Kind: terminalKind, OccurredAt: now.Add(time.Nanosecond), ContractRef: binding.ContractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ProposalRef: &binding.ContractRef, ActivationRef: &binding.ActivationRef, Control: &terminal, SigningPrivateKey: private})
	if controlErr != nil {
		return writeContainment(js, containmentOutput{Error: controlErr.Error()}, exitInternalFailure)
	}
	if err := actioncontract.AppendLifecycleRecord(journalPath, requestedRecord); err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInternalFailure)
	}
	if err := actioncontract.AppendLifecycleRecord(journalPath, terminalRecord); err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInternalFailure)
	}
	outcome := status
	if outcome == "contained" {
		outcome = "succeeded"
	}
	ld := func(value string) string {
		if !strings.HasPrefix(value, "sha256:") {
			return "sha256:" + value
		}
		return value
	}
	receipt := actioncontract.LifecycleReceipt{ContractFamilyID: proposal.ContractFamilyID, ContractID: proposal.ContractID, Revision: proposal.Revision, ArtifactDigests: []string{binding.ContractRef.Digest, ld(requested.CanonicalContentDigest), ld(terminal.CanonicalContentDigest), ld(requestedRecord.Signature.SignedDigest), ld(terminalRecord.Signature.SignedDigest)}, ArtifactRefs: []proof.RelationshipRef{binding.ContractRef, {Kind: "control", ID: requested.EvidenceID, Digest: ld(requested.CanonicalContentDigest), SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}, {Kind: "control", ID: terminal.EvidenceID, Digest: ld(terminal.CanonicalContentDigest), SchemaID: actioncontract.ControlEventEvidenceSchemaID, SchemaVersion: "1", SourceProduct: "gait"}, {Kind: "lifecycle_record", ID: requestedRecord.RecordID, Digest: ld(requestedRecord.Signature.SignedDigest), SchemaID: actioncontract.RuntimeLifecycleSchemaID, SchemaVersion: "1", SourceProduct: "gait"}, {Kind: "lifecycle_record", ID: terminalRecord.RecordID, Digest: ld(terminalRecord.Signature.SignedDigest), SchemaID: actioncontract.RuntimeLifecycleSchemaID, SchemaVersion: "1", SourceProduct: "gait"}}, ObservedAt: now.Format(time.RFC3339Nano), FreshUntil: now.Add(time.Hour).Format(time.RFC3339Nano), Authority: "non_authoritative", Quarantine: true, Redaction: "reference_only", Outcome: outcome, ReasonCodes: append(reasons, "effects_not_reversed")}
	receipt.Correlation = proof.ControlContainmentTelemetryProfile{ProfileVersion: "1.0", BindingMode: proof.BindingModeDigestBound, ContractRef: &binding.ContractRef, ContentDigest: binding.ContractRef.Digest}
	signedReceipt, receiptErr := receipt.Sign(private)
	if receiptErr != nil {
		return writeContainment(js, containmentOutput{Error: receiptErr.Error()}, exitInternalFailure)
	}
	receiptRaw, _ := json.Marshal(signedReceipt)
	if err := fsx.WriteFileAtomic(receiptPath, append(receiptRaw, '\n'), 0600); err != nil {
		return writeContainment(js, containmentOutput{Error: err.Error()}, exitInternalFailure)
	}
	code := exitOK
	if status != "contained" {
		code = exitVerifyFailed
	}
	return writeContainment(js, containmentOutput{OK: true, Status: status, ReasonCodes: reasons, StatePath: statePath, RevocationPath: revocationPath, JournalPath: journalPath, ReceiptPath: receiptPath}, code)
}

func appendRevocation(path, identity string, tokenIDs []string, at time.Time) error {
	entry, _ := json.Marshal(map[string]any{"identity": strings.TrimSpace(identity), "token_ids": tokenIDs, "revoked_at": at.UTC().Format(time.RFC3339Nano)})
	return fsx.AppendLineLocked(path, entry, 0600)
}

func containmentScopeDigest(identity, boundary, resource string) string {
	raw, _ := json.Marshal(struct {
		Identity string `json:"identity"`
		Boundary string `json:"boundary"`
		Resource string `json:"resource"`
		Command  string `json:"command"`
	}{identity, boundary, resource, "stop"})
	digest, _ := proofcanon.DigestJCS(raw)
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	return digest
}

func identityRevokedByRegistry(path, identity string) (bool, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- explicit operator-selected revocation registry.
	if err != nil {
		return false, err
	}
	wanted := strings.TrimSpace(identity)
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == wanted && wanted != "" {
			return true, nil
		}
		var entry struct {
			Identity string   `json:"identity"`
			TokenIDs []string `json:"token_ids"`
		}
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
				return false, fmt.Errorf("revocation registry contains malformed JSON: %w", err)
			}
		}
		if (strings.TrimSpace(entry.Identity) == wanted || containmentContains(entry.TokenIDs, wanted)) && wanted != "" {
			return true, nil
		}
	}
	return false, nil
}

func anyRevokedByRegistry(path string, identities []string) (bool, error) {
	for _, identity := range identities {
		if strings.TrimSpace(identity) == "" {
			continue
		}
		revoked, err := identityRevokedByRegistry(path, identity)
		if err != nil {
			return false, err
		}
		if revoked {
			return true, nil
		}
	}
	return false, nil
}

func containmentContains(values []string, wanted string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == wanted {
			return true
		}
	}
	return false
}
func writeContainment(js bool, out containmentOutput, code int) int {
	if js {
		raw, _ := json.Marshal(out)
		fmt.Println(string(raw))
	} else {
		fmt.Printf("containment: status=%s\n", out.Status)
	}
	return code
}
