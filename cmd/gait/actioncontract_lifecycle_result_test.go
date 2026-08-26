package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	proof "github.com/Clyra-AI/proof"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func TestLifecycleResultCLIEndToEndSignedSuccessAndFailures(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	manifestPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	proposal, raw, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := actioncontract.LoadSelectionEvidence(manifestPath, proposalPath, proposal, raw)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("default_verdict: allow\n"), 0600); err != nil {
		t.Fatal(err)
	}
	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	policyDigest, err := gate.PolicyDigest(policy)
	if err != nil {
		t.Fatal(err)
	}
	_, activationPrivate, _ := ed25519.GenerateKey(nil)
	if !strings.HasPrefix(policyDigest, "sha256:") {
		policyDigest = "sha256:" + policyDigest
	}
	activation, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{PolicyDigest: policyDigest, ActivatingPrincipal: "principal:test", AuthorityRefs: []string{"authority:test"}, Target: "target:test", Environment: "test", Mode: actioncontract.ActivationContextOnly, ValidFrom: "2026-01-01T00:00:00Z", Selection: &selection, SigningPrivateKey: activationPrivate, EvaluationTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(root, "activation.json")
	activationRaw, _ := json.Marshal(activation)
	if err := os.WriteFile(activationPath, activationRaw, 0600); err != nil {
		t.Fatal(err)
	}
	activationPubPath := filepath.Join(root, "activation.pub")
	if err := os.WriteFile(activationPubPath, []byte(base64.StdEncoding.EncodeToString(activationPrivate.Public().(ed25519.PublicKey))), 0600); err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(root, "intent.json")
	writeIntentFixture(t, intentPath, "tool.read")
	_, tracePrivate, _ := ed25519.GenerateKey(nil)
	tracePrivatePath := filepath.Join(root, "trace.key")
	if err := os.WriteFile(tracePrivatePath, []byte(base64.StdEncoding.EncodeToString(tracePrivate)), 0600); err != nil {
		t.Fatal(err)
	}
	tracePubPath := filepath.Join(root, "trace.pub")
	if err := os.WriteFile(tracePubPath, []byte(base64.StdEncoding.EncodeToString(tracePrivate.Public().(ed25519.PublicKey))), 0600); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(root, "trace.json")
	if code := runGateEval([]string{"--policy", policyPath, "--intent", intentPath, "--action-contract", proposalPath, "--activation", activationPath, "--activation-public-key", activationPubPath, "--key-mode", "prod", "--private-key", tracePrivatePath, "--trace-out", tracePath, "--evaluation-time", "2026-01-02T00:00:00Z", "--json"}); code != exitOK {
		t.Fatalf("gate=%d", code)
	}
	traceRecord, err := gate.ReadTraceRecord(tracePath)
	if err != nil || traceRecord.ReadinessDigest == "" {
		t.Fatalf("gate trace readiness digest missing: %v %#v", err, traceRecord)
	}
	if strings.HasPrefix(traceRecord.ReadinessDigest, "sha256:") {
		t.Fatalf("gate trace readiness digest must be bare hex: %q", traceRecord.ReadinessDigest)
	}
	_, lifecyclePrivate, _ := ed25519.GenerateKey(nil)
	lifecyclePrivatePath := filepath.Join(root, "lifecycle.key")
	if err := os.WriteFile(lifecyclePrivatePath, []byte(base64.StdEncoding.EncodeToString(lifecyclePrivate)), 0600); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(root, "lifecycle.jsonl")
	resultDigest := "sha256:" + strings.Repeat("a", 64)
	contractRef := proof.RelationshipRef{Kind: "action_contract", ID: proposal.ContractID, Digest: proposal.CanonicalContentDigest, SchemaID: actioncontract.ProposedContractSchemaID, SchemaVersion: actioncontract.ProposedContractVersion, SourceProduct: actioncontract.ProposedProducer}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: activation.ArtifactID, Digest: actioncontract.RawDigest(activationRaw), SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, SourceProduct: actioncontract.ActivatedProducer}
	preconditionDigest := "sha256:" + strings.Repeat("b", 64)
	preconditionRef := proof.RelationshipRef{Kind: "precondition", ID: "lifecycle-ready", Digest: preconditionDigest, SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, SourceProduct: actioncontract.EvidenceProducer}
	ready := actioncontract.ReadinessResult{SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, ContractID: proposal.ContractID, PolicyDigest: activation.PolicyDigest, Ready: true, Status: actioncontract.ReadinessSatisfied, Preconditions: []actioncontract.ReadinessPrecondition{{RequirementID: "lifecycle-ready", Required: true, Status: actioncontract.ReadinessSatisfied, ControlMode: actioncontract.ControlModeEnforced, EvidenceDigest: preconditionDigest}}}
	for _, options := range []actioncontract.LifecycleRecordOptions{
		{Kind: actioncontract.LifecycleProposalIngested, OccurredAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ContractRef: contractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ProposalRef: &contractRef, SigningPrivateKey: tracePrivate},
		{Kind: actioncontract.LifecyclePreconditionEvaluated, OccurredAt: time.Date(2026, 1, 2, 0, 0, 1, 0, time.UTC), ContractRef: contractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, PreconditionRefs: []proof.RelationshipRef{preconditionRef}, SigningPrivateKey: tracePrivate},
		{Kind: actioncontract.LifecycleDecisionReady, OccurredAt: time.Date(2026, 1, 2, 0, 0, 2, 0, time.UTC), ContractRef: contractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, PreconditionRefs: []proof.RelationshipRef{preconditionRef}, Decision: &ready, SigningPrivateKey: tracePrivate},
		{Kind: actioncontract.LifecycleActivationRequested, OccurredAt: time.Date(2026, 1, 2, 0, 0, 3, 0, time.UTC), ContractRef: contractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ProposalRef: &contractRef, SigningPrivateKey: tracePrivate},
		{Kind: actioncontract.LifecycleActivated, OccurredAt: time.Date(2026, 1, 2, 0, 0, 4, 0, time.UTC), ContractRef: contractRef, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ProposalRef: &contractRef, ActivationRef: &activationRef, SigningPrivateKey: tracePrivate},
	} {
		record, recordErr := actioncontract.NewLifecycleRecord(options)
		if recordErr != nil {
			t.Fatal(recordErr)
		}
		if recordErr = actioncontract.AppendLifecycleRecord(journalPath, record); recordErr != nil {
			t.Fatal(recordErr)
		}
	}
	prefixRecords, err := actioncontract.ReadLifecycleJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	seedJournal := func(path string) {
		t.Helper()
		for _, record := range prefixRecords {
			if seedErr := actioncontract.AppendLifecycleRecord(path, record); seedErr != nil {
				t.Fatal(seedErr)
			}
		}
	}
	resultJournalPath := filepath.Join(root, "result-file-events.jsonl")
	failureJournalPath := filepath.Join(root, "failure-events.jsonl")
	seedJournal(resultJournalPath)
	seedJournal(failureJournalPath)
	args := []string{"--trace", tracePath, "--proposal", proposalPath, "--activation", activationPath, "--trace-public-key", tracePubPath, "--public-key", activationPubPath, "--journal", journalPath, "--private-key", lifecyclePrivatePath, "--evaluation-time", "2026-01-02T00:00:00Z", "--execution-time", "2026-01-02T00:00:05Z", "--result-digest", resultDigest, "--outcome", "succeeded", "--json"}
	if code := runActionContractLifecycleResult(args); code != exitOK {
		t.Fatalf("lifecycle result=%d", code)
	}
	if code := runActionContractLifecycleResult(args); code == exitOK {
		t.Fatal("lifecycle result replay unexpectedly appended execution evidence")
	}
	resultPath := filepath.Join(root, "executor-result.json")
	if err := os.WriteFile(resultPath, []byte(`{"ok":true,"value":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	withoutFlag := func(base []string, flag string) []string {
		filtered := make([]string, 0, len(base))
		for i := 0; i < len(base); i++ {
			if base[i] == flag {
				i++
				continue
			}
			filtered = append(filtered, base[i])
		}
		return filtered
	}
	resultArgs := withoutFlag(args, "--result-digest")
	// The lifecycle-result boundary requires the signed proposal/decision/activation
	// prefix that Gate established. Reuse that journal when the result is supplied
	// through a file; a fresh journal would (correctly) fail closed.
	resultArgs = append(resultArgs, "--result-file", resultPath, "--journal", resultJournalPath)
	if code := runActionContractLifecycleResult(resultArgs); code != exitOK {
		t.Fatalf("result-file lifecycle result=%d", code)
	}
	badResultArgs := append([]string(nil), resultArgs...)
	for i := range badResultArgs {
		if badResultArgs[i] == "--result-file" && i+1 < len(badResultArgs) {
			badResultArgs[i+1] = filepath.Join(root, "missing-result.json")
			break
		}
	}
	if code := runActionContractLifecycleResult(badResultArgs); code == exitOK {
		t.Fatal("missing result file unexpectedly accepted")
	}
	mismatchedDigestArgs := append(append([]string(nil), resultArgs...), "--result-digest", "sha256:"+strings.Repeat("f", 64))
	if code := runActionContractLifecycleResult(mismatchedDigestArgs); code == exitOK {
		t.Fatal("mismatched result digest unexpectedly accepted")
	}
	journalRecords, err := actioncontract.ReadLifecycleJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range journalRecords {
		publicKey := tracePrivate.Public().(ed25519.PublicKey)
		if record.Kind == actioncontract.LifecycleExecutionStarted || record.Kind == actioncontract.LifecycleExecutionSucceeded || record.Kind == actioncontract.LifecycleExecutionFailed {
			publicKey = lifecyclePrivate.Public().(ed25519.PublicKey)
		}
		if valid, verifyErr := actioncontract.VerifyLifecycleRecord(record, publicKey); verifyErr != nil || !valid {
			t.Fatalf("lifecycle record signature: valid=%t err=%v", valid, verifyErr)
		}
	}
	failureArgs := append(args[:len(args)-3], "--outcome", "failed", "--json")
	failureArgs = append(failureArgs, "--journal", failureJournalPath)
	if code := runActionContractLifecycleResult(failureArgs); code != exitOK {
		t.Fatalf("failed lifecycle result=%d", code)
	}
	badKey := filepath.Join(root, "bad.pub")
	if err := os.WriteFile(badKey, []byte(base64.StdEncoding.EncodeToString(actioncontract.DevelopmentPublicKey())), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractLifecycleResult(append([]string{}, args[:len(args)-3]...)); code == exitOK {
		t.Fatal("missing outcome unexpectedly succeeded")
	}
	setFlag := func(base []string, flag, value string) []string {
		copyArgs := append([]string(nil), base...)
		for i := range copyArgs {
			if copyArgs[i] == flag && i+1 < len(copyArgs) {
				copyArgs[i+1] = value
				return copyArgs
			}
		}
		return copyArgs
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"trace unreadable", setFlag(args, "--trace", filepath.Join(root, "missing-trace.json"))},
		{"trace key unreadable", setFlag(args, "--trace-public-key", filepath.Join(root, "missing-trace.pub"))},
		{"activation key unreadable", setFlag(args, "--public-key", filepath.Join(root, "missing-activation.pub"))},
		{"proposal unreadable", setFlag(args, "--proposal", filepath.Join(root, "missing-proposal.json"))},
		{"activation unreadable", setFlag(args, "--activation", filepath.Join(root, "missing-activation.json"))},
		{"lifecycle key unreadable", setFlag(args, "--private-key", filepath.Join(root, "missing-lifecycle.key"))},
		{"result digest invalid", setFlag(args, "--result-digest", "bad")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code := runActionContractLifecycleResult(tc.args); code == exitOK {
				t.Fatalf("invalid lifecycle input accepted: %s", tc.name)
			}
		})
	}
	if code := runActionContractLifecycleResult([]string{"--unknown"}); code == exitOK {
		t.Fatal("unknown lifecycle flag unexpectedly accepted")
	}
	if code := runActionContractLifecycleResult(setFlag(args, "--trace-public-key", activationPubPath)); code == exitOK {
		t.Fatal("trace signed by another key unexpectedly accepted")
	}
	if code := runActionContractLifecycleResult(setFlag(args, "--public-key", badKey)); code == exitOK {
		t.Fatal("activation signed by another key unexpectedly accepted")
	}
	trace, err := gate.ReadTraceRecord(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	trace.ContractID = "pac-wrong"
	trace.Signature = nil
	signable, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := proofsign.SignTraceRecordJSON(tracePrivate, signable)
	if err != nil {
		t.Fatal(err)
	}
	trace.Signature = &schemagate.Signature{Alg: signature.Alg, KeyID: signature.KeyID, Sig: signature.Sig, SignedDigest: signature.SignedDigest}
	bindingPath := filepath.Join(root, "binding-mismatch.json")
	if err := gate.WriteTraceRecord(bindingPath, trace); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractLifecycleResult(setFlag(args, "--trace", bindingPath)); code == exitOK {
		t.Fatal("re-signed mismatched trace unexpectedly accepted")
	}
	trace.ContractID = "pac-4b7f1402784256ce"
	trace.PolicyDigest = "sha256:" + strings.Repeat("f", 64)
	trace.Signature = nil
	signable, _ = json.Marshal(trace)
	signature, err = proofsign.SignTraceRecordJSON(tracePrivate, signable)
	if err != nil {
		t.Fatal(err)
	}
	trace.Signature = &schemagate.Signature{Alg: signature.Alg, KeyID: signature.KeyID, Sig: signature.Sig, SignedDigest: signature.SignedDigest}
	policyPathMismatch := filepath.Join(root, "policy-mismatch.json")
	if err := gate.WriteTraceRecord(policyPathMismatch, trace); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractLifecycleResult(setFlag(args, "--trace", policyPathMismatch)); code == exitOK {
		t.Fatal("re-signed policy mismatch unexpectedly accepted")
	}
	if code := lifecycleResultOutput(false, nil, nil, exitOK); code != exitOK {
		t.Fatal(code)
	}
	trace, err = gate.ReadTraceRecord(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	signTrace := func(value schemagate.TraceRecord, path string) {
		t.Helper()
		value.Signature = nil
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		signature, signErr := proofsign.SignTraceRecordJSON(tracePrivate, raw)
		if signErr != nil {
			t.Fatal(signErr)
		}
		value.Signature = &schemagate.Signature{Alg: signature.Alg, KeyID: signature.KeyID, Sig: signature.Sig, SignedDigest: signature.SignedDigest}
		if writeErr := gate.WriteTraceRecord(path, value); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	blockedTrace := trace
	blockedTrace.Verdict = "block"
	blockedTracePath := filepath.Join(root, "blocked-trace.json")
	signTrace(blockedTrace, blockedTracePath)
	if code := runActionContractLifecycleResult(setFlag(setFlag(args, "--trace", blockedTracePath), "--journal", filepath.Join(root, "blocked-events.jsonl"))); code == exitOK {
		t.Fatal("non-allow trace unexpectedly emitted execution evidence")
	}
	readinessMismatch := trace
	readinessMismatch.ReadinessDigest = "sha256:" + strings.Repeat("f", 64)
	readinessMismatchPath := filepath.Join(root, "readiness-mismatch.json")
	signTrace(readinessMismatch, readinessMismatchPath)
	if code := runActionContractLifecycleResult(setFlag(setFlag(args, "--trace", readinessMismatchPath), "--journal", filepath.Join(root, "readiness-events.jsonl"))); code == exitOK {
		t.Fatal("readiness mismatch unexpectedly emitted execution evidence")
	}
}

func TestLifecycleResultCLIFailClosedInputBranches(t *testing.T) {
	cases := [][]string{nil, {"--trace", "missing"}, {"--trace", "x", "--proposal", "x", "--activation", "x", "--trace-public-key", "x", "--public-key", "x", "--journal", "x", "--private-key", "x", "--evaluation-time", "bad", "--result-digest", "sha256:" + strings.Repeat("a", 64), "--outcome", "succeeded"}}
	for _, args := range cases {
		if code := runActionContractLifecycleResult(args); code == exitOK {
			t.Fatalf("invalid lifecycle input accepted: %#v", args)
		}
	}
	if code := lifecycleResultOutput(true, nil, nil, exitOK); code != exitOK {
		t.Fatal(code)
	}
	if code := lifecycleResultOutput(false, nil, fmt.Errorf("expected"), exitInvalidInput); code != exitInvalidInput {
		t.Fatal(code)
	}
}
