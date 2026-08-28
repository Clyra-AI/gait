package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/gate"
	proof "github.com/Clyra-AI/proof"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func runActionContractLifecycleResult(args []string) int {
	f := flag.NewFlagSet("contract-lifecycle-result", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var tracePath, proposalPath, activationPath, publicKeyPath, tracePublicKeyPath, journalPath, privateKeyPath, outcome, effect, compensation, resultDigest, resultPath, evaluationTime, executionTime, trustedValidators string
	var trustedValidatorKeys repeatStringFlag
	var js, allowDevelopment bool
	f.StringVar(&tracePath, "trace", "", "signed Gate trace")
	f.StringVar(&proposalPath, "proposal", "", "verified Wrkr proposal")
	f.StringVar(&activationPath, "activation", "", "verified Gait activation")
	f.StringVar(&publicKeyPath, "public-key", "", "activation/trace public key")
	f.StringVar(&tracePublicKeyPath, "trace-public-key", "", "Gate trace public key (required separately)")
	f.StringVar(&journalPath, "journal", "", "existing lifecycle JSONL journal")
	f.StringVar(&privateKeyPath, "private-key", "", "lifecycle signing private key")
	f.StringVar(&outcome, "outcome", "", "executor outcome: succeeded|failed")
	f.StringVar(&effect, "effect-outcome", "", "optional observed effect outcome (reference-only)")
	f.StringVar(&compensation, "compensation-outcome", "", "optional compensation outcome (reference-only)")
	f.StringVar(&resultDigest, "result-digest", "", "digest of executor result (required)")
	f.StringVar(&resultPath, "result-file", "", "structured executor result/error JSON; digest is computed by Go")
	f.StringVar(&trustedValidators, "trusted-validators", "", "comma-separated policy-named trusted validator references")
	f.Var(&trustedValidatorKeys, "trusted-validator-key", "authoritative validator key as producer=public-key-path (repeatable)")
	f.StringVar(&evaluationTime, "evaluation-time", "", "explicit RFC3339 evaluation time (required)")
	f.StringVar(&executionTime, "execution-time", "", "RFC3339 execution occurrence time (default current UTC time)")
	f.BoolVar(&js, "json", false, "emit JSON")
	f.BoolVar(&allowDevelopment, "allow-development-signing", false, "TEST ONLY: allow development-signed activation")
	if err := f.Parse(args); err != nil {
		return lifecycleResultOutput(js, nil, err, exitInvalidInput)
	}
	if tracePath == "" || proposalPath == "" || activationPath == "" || publicKeyPath == "" || tracePublicKeyPath == "" || journalPath == "" || privateKeyPath == "" || (resultDigest == "" && resultPath == "") || evaluationTime == "" || (outcome != "succeeded" && outcome != "failed") {
		return lifecycleResultOutput(js, nil, fmt.Errorf("--trace, --proposal, --activation, --public-key, --trace-public-key, --journal, --private-key, --result-digest or --result-file, --evaluation-time, and valid --outcome are required"), exitInvalidInput)
	}
	when, err := time.Parse(time.RFC3339, evaluationTime)
	if err != nil {
		return lifecycleResultOutput(js, nil, fmt.Errorf("invalid --evaluation-time: %w", err), exitInvalidInput)
	}
	executionAt := time.Now().UTC()
	if strings.TrimSpace(executionTime) != "" {
		executionAt, err = time.Parse(time.RFC3339, executionTime)
		if err != nil {
			return lifecycleResultOutput(js, nil, fmt.Errorf("invalid --execution-time: %w", err), exitInvalidInput)
		}
		executionAt = executionAt.UTC()
	}
	trace, err := gate.ReadTraceRecord(tracePath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitVerifyFailed)
	}
	if strings.TrimSpace(trace.Verdict) != "allow" {
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle result requires an allow Gate verdict"), exitVerifyFailed)
	}
	tracePublic, err := proofsign.LoadPublicKeyBase64(tracePublicKeyPath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitInvalidInput)
	}
	public, err := proofsign.LoadPublicKeyBase64(publicKeyPath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitInvalidInput)
	}
	valid, err := gate.VerifyTraceRecordSignature(trace, tracePublic)
	if err != nil || !valid {
		return lifecycleResultOutput(js, nil, fmt.Errorf("gate trace verification failed: %w", err), exitVerifyFailed)
	}
	proposal, _, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitVerifyFailed)
	}
	activation, activationRaw, err := actioncontract.ReadActivatedArtifact(activationPath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitVerifyFailed)
	}
	if ok, verifyErr := actioncontract.VerifyActivationWithOptions(activation, public, actioncontract.VerificationOptions{AllowDevelopmentSigning: allowDevelopment, Proposal: &proposal, EvaluationTime: when}); verifyErr != nil || !ok {
		if verifyErr == nil {
			verifyErr = fmt.Errorf("signature invalid")
		}
		return lifecycleResultOutput(js, nil, fmt.Errorf("activation verification failed: %w", verifyErr), exitVerifyFailed)
	}
	proposalDigest := proposal.CanonicalContentDigest
	activationDigest := actioncontract.RawDigest(activationRaw)
	if trace.ContractID != proposal.ContractID || trace.ContractFamilyID != proposal.ContractFamilyID || trace.ContractRevision != proposal.Revision || trace.ProposalDigest != strings.TrimPrefix(proposalDigest, "sha256:") || trace.ActivationDigest != strings.TrimPrefix(activationDigest, "sha256:") {
		return lifecycleResultOutput(js, nil, fmt.Errorf("trace action contract binding mismatch"), exitVerifyFailed)
	}
	if strings.TrimPrefix(trace.PolicyDigest, "sha256:") != strings.TrimPrefix(activation.PolicyDigest, "sha256:") {
		return lifecycleResultOutput(js, nil, fmt.Errorf("trace policy digest does not match activation"), exitVerifyFailed)
	}
	proposalRef := proof.RelationshipRef{Kind: "action_contract", ID: proposal.ContractID, Digest: proposal.CanonicalContentDigest, SchemaID: actioncontract.ProposedContractSchemaID, SchemaVersion: actioncontract.ProposedContractVersion, SourceProduct: actioncontract.ProposedProducer}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: activation.ArtifactID, Digest: activationDigest, SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, SourceProduct: actioncontract.ActivatedProducer}
	trusted := parseActionContractCSV(trustedValidators)
	keys, keyErr := loadTrustedValidatorKeys(trustedValidatorKeys)
	if keyErr != nil {
		return lifecycleResultOutput(js, nil, keyErr, exitInvalidInput)
	}
	readiness := actioncontract.ReadinessFromArtifact(proposal, actioncontract.ReadinessInput{PolicyDigest: activation.PolicyDigest, Now: when, TrustedValidatorRefs: trusted, TrustedValidatorKeys: keys})
	readinessDigest, digestErr := actioncontract.DigestReadinessResult(readiness)
	if digestErr != nil || strings.TrimSpace(trace.ReadinessDigest) == "" || strings.TrimPrefix(trace.ReadinessDigest, "sha256:") != strings.TrimPrefix(readinessDigest, "sha256:") {
		return lifecycleResultOutput(js, nil, fmt.Errorf("trace readiness digest does not match verified readiness"), exitVerifyFailed)
	}
	private, err := proofsign.LoadPrivateKeyBase64(privateKeyPath)
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitInvalidInput)
	}
	// Gate owns the pre-execution prefix and signs it with the trace key. The
	// lifecycle key is intentionally separate and is only used for records this
	// command appends after verifying that prefix.
	if verifyErr := actioncontract.VerifyLifecycleJournal(journalPath, tracePublic); verifyErr != nil {
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal verification failed: %w", verifyErr), exitVerifyFailed)
	}
	prefix, prefixErr := actioncontract.ReadLifecycleJournal(journalPath)
	if prefixErr != nil {
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal prefix verification failed: %w", prefixErr), exitVerifyFailed)
	}
	prefixState, prefixErr := actioncontract.ReduceLifecycleChecked(prefix)
	if prefixErr != nil || !prefixState.Activated || !prefixState.DecisionReady {
		if prefixErr == nil {
			prefixErr = fmt.Errorf("journal is not activated and decision-ready")
		}
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal prefix verification failed: %w", prefixErr), exitVerifyFailed)
	}
	if prefixErr = lifecyclePrefixContractError(prefix, proposalRef, proposal.ContractFamilyID, proposal.Revision); prefixErr != nil {
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal prefix contract mismatch: %w", prefixErr), exitVerifyFailed)
	}
	for _, record := range prefix {
		if record.ActivationRef != nil && !sameLifecycleActivationRef(*record.ActivationRef, activationRef) {
			return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal prefix activation mismatch"), exitVerifyFailed)
		}
	}
	if prefixState.ExecutionStatus != "" {
		return lifecycleResultOutput(js, nil, fmt.Errorf("lifecycle journal already contains execution evidence"), exitVerifyFailed)
	}
	classification := actioncontract.ClassifyArtifact(proposal)
	traceDigest := trace.Signature.SignedDigest
	if !strings.HasPrefix(traceDigest, "sha256:") {
		traceDigest = "sha256:" + traceDigest
	}
	if strings.TrimSpace(resultPath) != "" {
		resultRaw, readErr := os.ReadFile(resultPath)
		if readErr != nil {
			return lifecycleResultOutput(js, nil, readErr, exitInvalidInput)
		}
		computedDigest, computeErr := actioncontract.DigestCanonicalJSON(resultRaw)
		if computeErr != nil {
			return lifecycleResultOutput(js, nil, fmt.Errorf("result file is not strict JSON: %w", computeErr), exitInvalidInput)
		}
		if strings.TrimSpace(resultDigest) != "" && strings.TrimPrefix(strings.TrimSpace(resultDigest), "sha256:") != strings.TrimPrefix(computedDigest, "sha256:") {
			return lifecycleResultOutput(js, nil, fmt.Errorf("result digest does not match result file"), exitInvalidInput)
		}
		resultDigest = computedDigest
	}
	resultDigest = strings.TrimSpace(resultDigest)
	if !strings.HasPrefix(resultDigest, "sha256:") {
		resultDigest = "sha256:" + resultDigest
	}
	records, err := actioncontract.AppendLifecycleResult(actioncontract.LifecycleResultOptions{JournalPath: journalPath, Proposal: proposal, Activation: activation, RuntimeAction: classification.Action, Readiness: readiness, PrivateKey: private, Outcome: outcome, EffectOutcome: effect, CompensationOutcome: compensation, TraceDigest: traceDigest, TraceID: trace.TraceID, ResultDigest: resultDigest, ActivationRawDigest: activationDigest, Now: executionAt})
	if err != nil {
		return lifecycleResultOutput(js, nil, err, exitInternalFailure)
	}
	return lifecycleResultOutput(js, records, nil, exitOK)
}

func sameLifecycleActivationRef(actual, expected proof.RelationshipRef) bool {
	return actual.Kind == expected.Kind && actual.ID == expected.ID && actual.Digest == expected.Digest && actual.SchemaID == expected.SchemaID && actual.SchemaVersion == expected.SchemaVersion && actual.SourceProduct == expected.SourceProduct
}

func lifecycleResultOutput(js bool, records []actioncontract.LifecycleRecord, err error, code int) int {
	if js {
		raw, _ := json.Marshal(map[string]any{"ok": err == nil, "records": records, "error": errorString(err, "")})
		fmt.Println(string(raw))
	} else if err != nil {
		fmt.Println("lifecycle result error: " + err.Error())
	} else {
		fmt.Printf("lifecycle result: records=%d\n", len(records))
	}
	return code
}
