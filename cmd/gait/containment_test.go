package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func TestContainmentStopRequiresVerifiedSignedContext(t *testing.T) {
	if code := runContainment([]string{"stop", "--identity", "agent:a"}); code != exitInvalidInput {
		t.Fatalf("containment accepted missing proposal/activation/trace signing context: %d", code)
	}
}

func TestContainmentOutputAndRegistryBoundaryBranches(t *testing.T) {
	if code := writeContainment(false, containmentOutput{Status: "partial"}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("text containment output=%d", code)
	}
	digest := containmentScopeDigest("alice", "workspace", "repo")
	if digest != containmentScopeDigest("alice", "workspace", "repo") {
		t.Fatal("containment digest is not deterministic")
	}
	if digest == containmentScopeDigest("alice", "other", "repo") {
		t.Fatal("containment digest ignored boundary")
	}
	registry := filepath.Join(t.TempDir(), "revocations.jsonl")
	if err := os.WriteFile(registry, []byte("plain-identity\n{\"identity\":\"alice\",\"token_ids\":[\"approval:1\"]}\nnot-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"plain-identity", "alice", "approval:1"} {
		revoked, err := identityRevokedByRegistry(registry, identity)
		if err != nil || !revoked {
			t.Fatalf("identity %q revoked=%t err=%v", identity, revoked, err)
		}
	}
	if revoked, err := identityRevokedByRegistry(registry, "unknown"); err != nil || revoked {
		t.Fatalf("unknown identity revoked=%t err=%v", revoked, err)
	}
	if !containmentContains([]string{"a", "b"}, "b") || containmentContains([]string{"a"}, "b") {
		t.Fatal("registry token membership drift")
	}
	malformedRegistry := filepath.Join(t.TempDir(), "malformed-revocations.jsonl")
	if err := os.WriteFile(malformedRegistry, []byte("{\"identity\":\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := identityRevokedByRegistry(malformedRegistry, "alice"); err == nil {
		t.Fatal("malformed structured revocation record accepted")
	}
	if code := runContainment(nil); code != exitInvalidInput {
		t.Fatalf("missing containment command=%d", code)
	}
	if code := runContainment([]string{"stop", "--json", "--unknown"}); code != exitInvalidInput {
		t.Fatalf("unknown containment flag=%d", code)
	}
}

func TestAnyRevokedByRegistryMatchesCapabilityDescendants(t *testing.T) {
	registry := filepath.Join(t.TempDir(), "revocations.jsonl")
	if err := os.WriteFile(registry, []byte(`{"identity":"root","token_ids":["approval:child","jit:digest"]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if revoked, err := anyRevokedByRegistry(registry, []string{"unrelated", "approval:child"}); err != nil || !revoked {
		t.Fatalf("descendant revocation=%t err=%v", revoked, err)
	}
	if revoked, err := anyRevokedByRegistry(registry, []string{"unrelated"}); err != nil || revoked {
		t.Fatalf("unrelated revocation=%t err=%v", revoked, err)
	}
	if revoked, err := anyRevokedByRegistry(filepath.Join(registry, "missing"), []string{"identity"}); err == nil || revoked {
		t.Fatalf("missing registry result=%t err=%v", revoked, err)
	}
}

func TestContainmentStopSignedFlowRevokesAndBindsTrace(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	manifest := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	proposal, raw, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := actioncontract.LoadSelectionEvidence(manifest, proposalPath, proposal, raw)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, "policy.yaml")
	_ = os.WriteFile(policyPath, []byte("default_verdict: allow\n"), 0600)
	policyValue, _ := gate.LoadPolicyFile(policyPath)
	policyDigest, _ := gate.PolicyDigest(policyValue)
	activation, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{PolicyDigest: "sha256:" + strings.TrimPrefix(policyDigest, "sha256:"), ActivatingPrincipal: "principal:test", AuthorityRefs: []string{"authority:test"}, Target: "target:test", Environment: "test", Mode: actioncontract.ActivationContextOnly, ValidFrom: "2026-01-01T00:00:00Z", Selection: &selection, AllowDevelopmentSigning: true, EvaluationTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(root, "activation.json")
	activationRaw, _ := json.Marshal(activation)
	_ = os.WriteFile(activationPath, activationRaw, 0600)
	activationPubPath := filepath.Join(root, "activation.pub")
	_ = os.WriteFile(activationPubPath, []byte(base64.StdEncoding.EncodeToString(actioncontract.DevelopmentPublicKey())), 0600)
	intentPath := filepath.Join(root, "intent.json")
	writeIntentFixture(t, intentPath, "tool.read")
	_, lifecyclePrivate, _ := ed25519.GenerateKey(nil)
	lifecycleKeyPath := filepath.Join(root, "lifecycle.key")
	_ = os.WriteFile(lifecycleKeyPath, []byte(base64.StdEncoding.EncodeToString(lifecyclePrivate)), 0600)
	_, tracePrivate, _ := ed25519.GenerateKey(nil)
	traceKeyPath := filepath.Join(root, "trace.key")
	_ = os.WriteFile(traceKeyPath, []byte(base64.StdEncoding.EncodeToString(tracePrivate)), 0600)
	tracePubPath := filepath.Join(root, "trace.pub")
	_ = os.WriteFile(tracePubPath, []byte(base64.StdEncoding.EncodeToString(tracePrivate.Public().(ed25519.PublicKey))), 0600)
	tracePath := filepath.Join(root, "trace.json")
	// Gate trace is contract-bound by the explicit proposal/activation inputs.
	if code := runGateEval([]string{"--policy", policyPath, "--intent", intentPath, "--action-contract", proposalPath, "--activation", activationPath, "--activation-public-key", activationPubPath, "--allow-development-contract-signing", "--key-mode", "prod", "--private-key", traceKeyPath, "--trace-out", tracePath, "--evaluation-time", "2026-01-02T00:00:00Z", "--json"}); code != exitOK {
		t.Fatalf("gate=%d", code)
	}
	traceCheck, _ := gate.ReadTraceRecord(tracePath)
	if traceCheck.ContractID != proposal.ContractID || traceCheck.ContractFamilyID != proposal.ContractFamilyID || traceCheck.ContractRevision != proposal.Revision || traceCheck.ProposalDigest != strings.TrimPrefix(proposal.CanonicalContentDigest, "sha256:") {
		t.Fatalf("trace binding: %#v", traceCheck)
	}
	statePath := filepath.Join(root, "state.json")
	journalPath := filepath.Join(root, "lifecycle.jsonl")
	receiptPath := filepath.Join(root, "receipt.json")
	revocationPath := filepath.Join(root, "revocations.jsonl")
	args := []string{"stop", "--state", statePath, "--proposal", proposalPath, "--activation", activationPath, "--trace", tracePath, "--trace-public-key", tracePubPath, "--activation-public-key", activationPubPath, "--allow-development-signing", "--evaluation-time", "2026-01-02T00:00:00Z", "--occurrence-time", "2026-01-02T00:00:06Z", "--private-key", lifecycleKeyPath, "--journal", journalPath, "--receipt-out", receiptPath, "--identity", "alice", "--boundary-id", "workspace", "--adapter-identity", "gait-local-kill-switch", "--resource-id", "repo", "--revocation-registry", revocationPath, "--token-ids", "approval:1,delegation:1", "--json"}
	withoutDevelopment := make([]string, 0, len(args))
	for _, value := range args {
		if value != "--allow-development-signing" {
			withoutDevelopment = append(withoutDevelopment, value)
		}
	}
	if code := runContainment(withoutDevelopment); code != exitVerifyFailed {
		t.Fatalf("development activation was accepted by default: %d", code)
	}
	if code := runContainment(args); code != exitOK {
		t.Fatalf("containment=%d", code)
	}
	if err := actioncontract.VerifyLifecycleJournal(journalPath, lifecyclePrivate.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	records, err := actioncontract.ReadLifecycleJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Kind != actioncontract.LifecycleStopRequested || records[1].Kind != actioncontract.LifecycleStopAcknowledged {
		t.Fatalf("unexpected containment lifecycle sequence: %#v", records)
	}
	if records[0].OccurredAt != "2026-01-02T00:00:06Z" || records[1].OccurredAt != "2026-01-02T00:00:06.000000001Z" {
		t.Fatalf("containment occurrence time was not separated from evaluation time: %#v", records)
	}
	receiptRaw, _ := os.ReadFile(receiptPath)
	var receipt actioncontract.LifecycleReceipt
	if err := json.Unmarshal(receiptRaw, &receipt); err != nil {
		t.Fatal(err)
	}
	if err := actioncontract.VerifyLifecycleReceipt(receipt, lifecyclePrivate.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	if len(receipt.ArtifactRefs) != 5 {
		t.Fatalf("expected contract plus four control/lifecycle refs, got %d", len(receipt.ArtifactRefs))
	}
	refKinds := map[string]int{}
	for _, ref := range receipt.ArtifactRefs {
		refKinds[ref.Kind]++
	}
	if refKinds["control"] != 2 || refKinds["lifecycle_record"] != 2 {
		t.Fatalf("receipt refs missing control/lifecycle artifacts: %#v", refKinds)
	}
	if revoked, err := identityRevokedByRegistry(revocationPath, "approval:1"); err != nil || !revoked {
		t.Fatalf("revocation=%t err=%v", revoked, err)
	}
	if code := runGateEval([]string{"--policy", policyPath, "--intent", intentPath, "--revocation-registry", revocationPath, "--trace-out", filepath.Join(root, "restart.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("restart revocation gate=%d", code)
	}
	// A trace with a mismatched contract identity cannot be used to authorize stop.
	trace := mustReadTrace(t, tracePath)
	trace.ContractID = "pac-wrong"
	trace.Signature = nil
	signable, _ := json.Marshal(trace)
	signature, signErr := proofsign.SignTraceRecordJSON(tracePrivate, signable)
	if signErr != nil {
		t.Fatal(signErr)
	}
	trace.Signature = &schemagate.Signature{Alg: signature.Alg, KeyID: signature.KeyID, Sig: signature.Sig, SignedDigest: signature.SignedDigest}
	tamperedTrace := filepath.Join(root, "tampered-trace.json")
	if err := gate.WriteTraceRecord(tamperedTrace, trace); err != nil {
		t.Fatal(err)
	}
	if code := runContainment(append(append([]string{}, args[:len(args)-1]...), "--trace", tamperedTrace, "--json")); code != exitVerifyFailed {
		t.Fatalf("tampered containment=%d", code)
	}
	failureArgs := append([]string{}, args...)
	failureArgs = append(failureArgs, "--state", filepath.Join(root, "state-failure.json"), "--journal", filepath.Join(root, "lifecycle-failure.jsonl"), "--receipt-out", filepath.Join(root, "receipt-failure.json"), "--external-command", filepath.Join(root, "missing-revocation-adapter"))
	if code := runContainment(failureArgs); code != exitVerifyFailed {
		t.Fatalf("external failure containment=%d", code)
	}
	replace := func(base []string, flag, value string) []string {
		copyArgs := append([]string(nil), base...)
		for i := range copyArgs {
			if copyArgs[i] == flag && i+1 < len(copyArgs) {
				copyArgs[i+1] = value
				return copyArgs
			}
		}
		return copyArgs
	}
	for name, invalidArgs := range map[string][]string{
		"trace unreadable":          replace(args, "--trace", filepath.Join(root, "missing-trace.json")),
		"trace key unreadable":      replace(args, "--trace-public-key", filepath.Join(root, "missing-trace.pub")),
		"proposal unreadable":       replace(args, "--proposal", filepath.Join(root, "missing-proposal.json")),
		"activation unreadable":     replace(args, "--activation", filepath.Join(root, "missing-activation.json")),
		"activation key unreadable": replace(args, "--activation-public-key", filepath.Join(root, "missing-activation.pub")),
		"lifecycle key unreadable":  replace(args, "--private-key", filepath.Join(root, "missing-lifecycle.key")),
		"evaluation time invalid":   replace(args, "--evaluation-time", "not-a-time"),
	} {
		t.Run(name, func(t *testing.T) {
			if code := runContainment(invalidArgs); code == exitOK {
				t.Fatalf("invalid containment input accepted: %s", name)
			}
		})
	}
	stateDirectory := filepath.Join(root, "state-directory")
	if err := os.Mkdir(stateDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if code := runContainment(replace(args, "--state", stateDirectory)); code == exitOK {
		t.Fatal("directory kill-switch state accepted")
	}
	revocationDirectory := filepath.Join(root, "revocation-directory")
	if err := os.Mkdir(revocationDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	partialArgs := replace(replace(replace(args, "--state", filepath.Join(root, "state-partial.json")), "--journal", filepath.Join(root, "journal-partial.jsonl")), "--receipt-out", filepath.Join(root, "receipt-partial.json"))
	partialArgs = replace(partialArgs, "--revocation-registry", revocationDirectory)
	if code := runContainment(partialArgs); code != exitVerifyFailed {
		t.Fatalf("revocation registry failure status=%d", code)
	}
}

func mustReadTrace(t *testing.T, path string) schemagate.TraceRecord {
	t.Helper()
	trace, err := gate.ReadTraceRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	return trace
}
