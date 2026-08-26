package main

import (
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
)

func TestGateActionContractIntegrationLegacyAndActivation(t *testing.T) {
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(work, "policy.yaml")
	if err := os.WriteFile(policy, []byte("schema_id: gait.gate.policy\nschema_version: 1.0.0\ndefault_verdict: allow\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	intent := filepath.Join(work, "intent.json")
	writeIntentFixture(t, intent, "tool.read")
	if code := runGateEval([]string{"--policy", policy, "--intent", intent, "--trace-out", filepath.Join(work, "legacy.json"), "--json"}); code != exitOK {
		t.Fatalf("legacy gate=%d", code)
	}
	proposal := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	activation := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "activated-action-contract.json")
	pub := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "fixture-signing-key.public.b64")
	if code := runGateEval([]string{"--policy", policy, "--intent", intent, "--action-contract", proposal, "--activation", activation, "--activation-public-key", pub, "--allow-development-contract-signing", "--trace-out", filepath.Join(work, "bound.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("bound gate=%d", code)
	}
	var object map[string]any
	raw, _ := os.ReadFile(activation)
	_ = json.Unmarshal(raw, &object)
	object["artifact_id"] = "gact-0000000000000000"
	tampered := filepath.Join(work, "tampered.json")
	encoded, _ := json.Marshal(object)
	_ = os.WriteFile(tampered, encoded, 0o600)
	if code := runGateEval([]string{"--policy", policy, "--intent", intent, "--action-contract", proposal, "--activation", tampered, "--activation-public-key", pub, "--allow-development-contract-signing", "--trace-out", filepath.Join(work, "tampered.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("tampered gate=%d", code)
	}
}

func TestEvaluateGateActionContractDirectBindingBranches(t *testing.T) {
	if runtime, err := evaluateGateActionContract("", "", "", "", "", nil, false, "", schemagate.IntentRequest{}, time.Now().UTC()); err != nil || runtime.Selected {
		t.Fatalf("legacy contract selection changed: %#v %v", runtime, err)
	}
	if _, err := evaluateGateActionContract("proposal", "", "", "", "", nil, false, "", schemagate.IntentRequest{}, time.Now().UTC()); err == nil {
		t.Fatal("incomplete contract selection accepted")
	}
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	activationPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "activated-action-contract.json")
	pubPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "fixture-signing-key.public.b64")
	activation, _, err := actioncontract.ReadActivatedArtifact(activationPath)
	if err != nil {
		t.Fatal(err)
	}
	intent := schemagate.IntentRequest{ToolName: "tool.read", Context: schemagate.IntentContext{Identity: "alice", Workspace: "/repo", RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/repo/file"}}}
	runtime, err := evaluateGateActionContract(proposalPath, activationPath, pubPath, "", "", nil, true, activation.PolicyDigest, intent, time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC))
	if err != nil || !runtime.Selected || runtime.ContractID == "" || runtime.Readiness == nil || runtime.ReadinessDigest == "" {
		t.Fatalf("valid contract binding rejected: %#v %v", runtime, err)
	}
	if _, err := evaluateGateActionContract(proposalPath, activationPath, pubPath, "", "", []string{"broken=key"}, true, activation.PolicyDigest, intent, time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("invalid trusted validator key accepted")
	}
	devRuntime, err := evaluateGateActionContract(proposalPath, activationPath, "", "", "", nil, true, activation.PolicyDigest, intent, time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC))
	if err != nil || !devRuntime.Selected || devRuntime.ContractID == "" {
		t.Fatalf("development activation fallback rejected: %#v %v", devRuntime, err)
	}
	if !sameActionContractDigest("sha256:"+strings.Repeat("a", 64), strings.Repeat("A", 64)) || sameActionContractDigest("sha256:"+strings.Repeat("a", 64), strings.Repeat("b", 64)) {
		t.Fatal("contract digest normalization drift")
	}
}

func TestEvaluateGateActionContractFailureMatrix(t *testing.T) {
	root := t.TempDir()
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	activationPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "activated-action-contract.json")
	pubPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "fixture-signing-key.public.b64")
	activation, _, err := actioncontract.ReadActivatedArtifact(activationPath)
	if err != nil {
		t.Fatal(err)
	}
	intent := schemagate.IntentRequest{ToolName: "tool.read", Context: schemagate.IntentContext{Identity: "alice", Workspace: "/repo", RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/repo/file"}}}
	call := func(proposal, activationFile, public string, value schemagate.IntentRequest) error {
		_, err := evaluateGateActionContract(proposal, activationFile, public, "", "", nil, true, activation.PolicyDigest, value, time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC))
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "bad-proposal.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad-activation.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"proposal unreadable":     call(filepath.Join(root, "missing-proposal"), activationPath, pubPath, intent),
		"proposal malformed":      call(filepath.Join(root, "bad-proposal.json"), activationPath, pubPath, intent),
		"activation malformed":    call(proposalPath, filepath.Join(root, "bad-activation.json"), pubPath, intent),
		"public key unreadable":   call(proposalPath, activationPath, filepath.Join(root, "missing-key"), intent),
		"context digest mismatch": call(proposalPath, activationPath, pubPath, func() schemagate.IntentRequest { value := intent; value.Context.ContractID = "wrong"; return value }()),
	} {
		if err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestGateControlsRejectCircuitBoundToPreContractIntent(t *testing.T) {
	root := t.TempDir()
	base := schemagate.IntentRequest{ToolName: "tool.read", Context: schemagate.IntentContext{RequestID: "req-contract-rebind", Identity: "alice", Workspace: root, RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/repo/file", Operation: "read"}}}
	normalized, err := gate.NormalizeIntent(base)
	if err != nil {
		t.Fatal(err)
	}
	state := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{"req-contract-rebind"}}
	decision := actioncontract.ChainDecision{SchemaID: actioncontract.ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: state}
	circuit := actioncontract.CircuitBreakerInput{SchemaID: actioncontract.CircuitInputSchemaID, SchemaVersion: "1", Chain: decision, EffectStatus: "pass", EffectAuthoritative: true, IntentDigest: "sha256:" + normalized.IntentDigest}
	circuit.ChainStateDigest, err = actioncontract.DigestChainState(state)
	if err != nil {
		t.Fatal(err)
	}
	circuit.BindingDigest, err = actioncontract.CircuitBindingDigest(circuit)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, value any) string {
		path := filepath.Join(root, name)
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, raw, 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return path
	}
	contractIntent := normalized
	contractIntent.Context.ContractID = "contract:bound"
	contractIntent.Context.ContractFamilyID = "family:bound"
	contractIntent.Context.ContractRevision = 1
	contractIntent, err = gate.NormalizeIntent(contractIntent)
	if err != nil {
		t.Fatal(err)
	}
	if contractIntent.IntentDigest == normalized.IntentDigest {
		t.Fatal("contract enrichment did not change bound intent digest")
	}
	if err := evaluateGateControls("", "", "", "", write("circuit.json", circuit), contractIntent); err == nil {
		t.Fatal("pre-contract circuit authorized enriched contracted intent")
	}
}

func TestGateLifecycleJournalStartsWithProposal(t *testing.T) {
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
	if !strings.HasPrefix(policyDigest, "sha256:") {
		policyDigest = "sha256:" + policyDigest
	}
	activation, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{PolicyDigest: policyDigest, ActivatingPrincipal: "principal:test", AuthorityRefs: []string{"authority:test"}, Target: "target:test", Environment: "test", Mode: actioncontract.ActivationContextOnly, ValidFrom: "2026-01-01T00:00:00Z", Selection: &selection, AllowDevelopmentSigning: true, EvaluationTime: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(root, "activation.json")
	activationRaw, _ := json.Marshal(activation)
	if err := os.WriteFile(activationPath, activationRaw, 0600); err != nil {
		t.Fatal(err)
	}
	activationPubPath := filepath.Join(root, "activation.pub")
	if err := os.WriteFile(activationPubPath, []byte(base64.StdEncoding.EncodeToString(actioncontract.DevelopmentPublicKey())), 0600); err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(root, "intent.json")
	writeIntentFixture(t, intentPath, "tool.read")
	lifecyclePath := filepath.Join(root, "lifecycle.jsonl")
	if code := runGateEval([]string{"--policy", policyPath, "--intent", intentPath, "--action-contract", proposalPath, "--activation", activationPath, "--activation-public-key", activationPubPath, "--allow-development-contract-signing", "--lifecycle-out", lifecyclePath, "--trace-out", filepath.Join(root, "trace.json"), "--evaluation-time", "2026-08-26T00:00:00Z", "--json"}); code != exitOK {
		t.Fatalf("gate lifecycle=%d", code)
	}
	records, err := actioncontract.ReadLifecycleJournal(lifecyclePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Kind != actioncontract.LifecycleProposalIngested {
		t.Fatalf("unexpected initial lifecycle sequence: %#v", records)
	}
}

func TestGateActionContractRequiredReadinessAndControlBindings(t *testing.T) {
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(work, "policy.yaml")
	_ = os.WriteFile(policy, []byte("default_verdict: allow\n"), 0o600)
	intentPath := filepath.Join(work, "intent.json")
	writeIntentFixture(t, intentPath, "tool.read")
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json")
	manifest := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	proposal, raw, _ := actioncontract.ReadArtifact(proposalPath)
	selection, _ := actioncontract.LoadSelectionEvidence(manifest, proposalPath, proposal, raw)
	activation, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{PolicyDigest: "sha256:" + strings.Repeat("a", 64), ActivatingPrincipal: "principal:test", AuthorityRefs: []string{"authority:test"}, Target: "target:test", Environment: "test", Mode: actioncontract.ActivationRequired, ValidFrom: "2026-01-01T00:00:00Z", Selection: &selection, AllowDevelopmentSigning: true, EvaluationTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(work, "activation.json")
	aRaw, _ := json.Marshal(activation)
	_ = os.WriteFile(activationPath, aRaw, 0o600)
	pubPath := filepath.Join(work, "pub.b64")
	_ = os.WriteFile(pubPath, []byte(actioncontract.EncodePrivateKey(actioncontract.DevelopmentPrivateKey())), 0o600)
	if code := runGateEval([]string{"--policy", policy, "--intent", intentPath, "--action-contract", proposalPath, "--activation", activationPath, "--activation-public-key", filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "fixture-signing-key.public.b64"), "--allow-development-contract-signing", "--trace-out", filepath.Join(work, "required.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("required readiness gate=%d", code)
	}
	normalized, _ := gate.NormalizeIntent(mustReadGateIntent(t, intentPath))
	chainID := normalized.IntentDigest
	normalizedRaw, _ := json.Marshal(normalized)
	_ = os.WriteFile(intentPath, normalizedRaw, 0o600)
	chain := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{chainID}}
	decision := actioncontract.ChainDecision{SchemaID: actioncontract.ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: chain}
	circuit := actioncontract.CircuitBreakerInput{SchemaID: actioncontract.CircuitInputSchemaID, SchemaVersion: "1", Chain: decision, EffectStatus: "pass", EffectAuthoritative: true, IntentDigest: "sha256:" + normalized.IntentDigest}
	circuit.ChainStateDigest, _ = actioncontract.DigestChainState(chain)
	circuit.BindingDigest, _ = actioncontract.CircuitBindingDigest(circuit)
	circuitPath := filepath.Join(work, "circuit.json")
	cRaw, _ := json.Marshal(circuit)
	_ = os.WriteFile(circuitPath, cRaw, 0o600)
	if _, readErr := actioncontract.ReadRuntimeInput(circuitPath); readErr != nil {
		t.Fatalf("direct circuit read: %v path=%s", readErr, circuitPath)
	}
	if code := runGateEval([]string{"--policy", policy, "--intent", intentPath, "--circuit-input", circuitPath, "--trace-out", filepath.Join(work, "circuit.json"), "--json"}); code != exitOK {
		t.Fatalf("bound circuit gate=%d", code)
	}
	circuit.IntentDigest = "sha256:" + strings.Repeat("b", 64)
	cRaw, _ = json.Marshal(circuit)
	_ = os.WriteFile(circuitPath, cRaw, 0o600)
	if code := runGateEval([]string{"--policy", policy, "--intent", intentPath, "--circuit-input", circuitPath, "--trace-out", filepath.Join(work, "circuit-bad.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("mismatched circuit gate=%d", code)
	}
	if _, statErr := os.Stat(filepath.Join(work, "circuit-bad.json")); statErr != nil {
		t.Fatalf("control-block trace was not emitted: %v", statErr)
	}
}

func mustReadGateIntent(t *testing.T, path string) schemagate.IntentRequest {
	t.Helper()
	intent, err := readIntentRequest(path)
	if err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestGateControlsPersistsBoundCandidate(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(root, "intent.json")
	writeIntentFixture(t, intentPath, "tool.read")
	intent := mustReadGateIntent(t, intentPath)
	candidate := deriveGateChainCandidate(intent)
	policy := actioncontract.ChainPolicy{SchemaID: actioncontract.ChainPolicySchemaID, SchemaVersion: "1", MaxSteps: 2}
	state := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1"}
	write := func(name string, value any) string {
		path := filepath.Join(root, name)
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if err := evaluateGateControls(write("policy.json", policy), write("state.json", state), write("candidate.json", candidate), filepath.Join(root, "next.json"), "", intent); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "next.json")); err != nil {
		t.Fatal(err)
	}
}

func TestGateControlsRejectsPartialAndUnreadableInputs(t *testing.T) {
	intent := schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/a", Operation: "read"}}}
	if err := evaluateGateControls("policy.json", "", "", "", "", intent); err == nil {
		t.Fatal("partial chain controls accepted")
	}
	if err := evaluateGateControls("", "", "", "", "missing-circuit.json", intent); err == nil {
		t.Fatal("unreadable circuit accepted")
	}
}

func TestGateChainCandidateDerivationClassesAndSets(t *testing.T) {
	for _, target := range []schemagate.IntentTarget{{Value: "x", Operation: "read"}, {Value: "x", Operation: "write"}, {Value: "x", Operation: "delete"}, {Value: "x", Operation: "execute"}, {Value: "x", EndpointClass: "net.http"}} {
		candidate := deriveGateChainCandidate(schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req-1"}, Targets: []schemagate.IntentTarget{target}})
		if candidate.ID != "req-1" || candidate.Target != "x" || len(candidate.Classes) != 1 {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
	if !sameStringSet([]string{"write", "read", "write"}, []string{"read", "write"}) || sameStringSet([]string{"read"}, []string{"write"}) {
		t.Fatal("candidate class set comparison drift")
	}
	networkDelete := deriveGateChainCandidate(schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req-net"}, Targets: []schemagate.IntentTarget{{Value: "https://example.test/item", EndpointClass: "http", Operation: "delete"}}})
	if !sameStringSet(networkDelete.Classes, []string{"delete", "egress"}) {
		t.Fatalf("network operation lost additive egress class: %#v", networkDelete)
	}
}

func TestGateControlsLockedCircuitAndChainFailureBranches(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req-1", Identity: "a", Workspace: root, RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/a", Operation: "read"}}, IntentDigest: strings.Repeat("a", 64)}
	write := func(name string, value any) string {
		path := filepath.Join(root, name)
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	validPolicy := actioncontract.ChainPolicy{SchemaID: actioncontract.ChainPolicySchemaID, SchemaVersion: "1", MaxSteps: 2}
	validState := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1"}
	validCandidate := deriveGateChainCandidate(intent)
	if err := evaluateGateControlsLocked(write("p.json", validPolicy), write("s.json", validState), write("c.json", validCandidate), "", "", intent); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "missing-p"), filepath.Join(root, "missing-s"), filepath.Join(root, "missing-c")} {
		if err := evaluateGateControlsLocked(path, path, path, "", "", intent); err == nil {
			t.Fatal("unreadable chain input accepted")
		}
	}
	badCandidate := validCandidate
	badCandidate.Target = "other"
	if err := evaluateGateControlsLocked(write("p2.json", validPolicy), write("s2.json", validState), write("c2.json", badCandidate), "", "", intent); err == nil {
		t.Fatal("unbound candidate accepted")
	}
	chain := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{"req-1"}}
	chainDecision := actioncontract.ChainDecision{SchemaID: actioncontract.ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: chain}
	circuit := actioncontract.CircuitBreakerInput{SchemaID: actioncontract.CircuitInputSchemaID, SchemaVersion: "1", Chain: chainDecision, EffectStatus: "pass", EffectAuthoritative: true, IntentDigest: "sha256:" + intent.IntentDigest}
	circuit.ChainStateDigest, _ = actioncontract.DigestChainState(chain)
	circuit.BindingDigest, _ = actioncontract.CircuitBindingDigest(circuit)
	cp := write("circuit.json", circuit)
	if err := evaluateGateControlsLocked("", "", "", "", cp, intent); err != nil {
		t.Fatal(err)
	}
	circuit.IntentDigest = "sha256:" + strings.Repeat("b", 64)
	if err := evaluateGateControlsLocked("", "", "", "", write("ci.json", circuit), intent); err == nil {
		t.Fatal("bad circuit intent accepted")
	}
	circuit.IntentDigest = "sha256:" + intent.IntentDigest
	circuit.BindingDigest = "sha256:" + strings.Repeat("c", 64)
	if err := evaluateGateControlsLocked("", "", "", "", write("cb.json", circuit), intent); err == nil {
		t.Fatal("bad circuit binding accepted")
	}
}

func TestGateControlsLockedDecisionAndPersistenceFailures(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req-write", Identity: "a", Workspace: root, RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/a", Operation: "write", EndpointClass: "fs.write"}}}
	write := func(name string, value any) string {
		path := filepath.Join(root, name)
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	policy := actioncontract.ChainPolicy{SchemaID: actioncontract.ChainPolicySchemaID, SchemaVersion: "1", ForbiddenClasses: []string{"write"}}
	state := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1"}
	candidate := deriveGateChainCandidate(intent)
	if err := evaluateGateControlsLocked(write("pf.json", policy), write("sf.json", state), write("cf.json", candidate), "", "", intent); err == nil {
		t.Fatal("forbidden chain accepted")
	}
	policy.ForbiddenClasses = nil
	if err := evaluateGateControlsLocked(write("pa.json", policy), write("sa.json", state), write("ca.json", candidate), root, "", intent); err == nil {
		t.Fatal("directory state output accepted")
	}
	intent.IntentDigest = strings.Repeat("a", 64)
	chain := actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{"req-write"}}
	cd := actioncontract.ChainDecision{SchemaID: actioncontract.ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: chain}
	ci := actioncontract.CircuitBreakerInput{SchemaID: actioncontract.CircuitInputSchemaID, SchemaVersion: "1", Chain: cd, EffectStatus: "pass", EffectAuthoritative: false, IntentDigest: "sha256:" + intent.IntentDigest}
	ci.ChainStateDigest, _ = actioncontract.DigestChainState(chain)
	ci.BindingDigest, _ = actioncontract.CircuitBindingDigest(ci)
	if err := evaluateGateControlsLocked("", "", "", "", write("deny.json", ci), intent); err == nil {
		t.Fatal("denied circuit accepted")
	}
	ci.EffectAuthoritative = true
	ci.BindingDigest, _ = actioncontract.CircuitBindingDigest(ci)
	ci.Chain.State.StepIDs = []string{"other"}
	ci.ChainStateDigest, _ = actioncontract.DigestChainState(ci.Chain.State)
	ci.BindingDigest, _ = actioncontract.CircuitBindingDigest(ci)
	if err := evaluateGateControlsLocked("", "", "", "", write("scope.json", ci), intent); err == nil {
		t.Fatal("out-of-scope circuit accepted")
	}
}

func TestGateControlsRejectsMalformedRuntimeFilesAndDefaultsCandidate(t *testing.T) {
	root := t.TempDir()
	intent := schemagate.IntentRequest{Context: schemagate.IntentContext{RequestID: "req-malformed"}, Targets: []schemagate.IntentTarget{{Value: "target", Operation: "read"}}}
	writeRaw := func(name, raw string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	validPolicy := `{"schema_id":"` + actioncontract.ChainPolicySchemaID + `","schema_version":"1","max_steps":2}`
	validState := `{"schema_id":"` + actioncontract.ChainStateSchemaID + `","schema_version":"1"}`
	candidate, _ := json.Marshal(deriveGateChainCandidate(intent))
	if err := evaluateGateControlsLocked(writeRaw("policy.json", validPolicy), writeRaw("state.json", "{"), writeRaw("candidate.json", string(candidate)), "", "", intent); err == nil {
		t.Fatal("malformed state accepted")
	}
	if err := evaluateGateControlsLocked(writeRaw("policy-2.json", validPolicy), writeRaw("state-2.json", validState), writeRaw("candidate-2.json", "{"), "", "", intent); err == nil {
		t.Fatal("malformed candidate accepted")
	}
	if err := evaluateGateControlsLocked("", "", "", "", writeRaw("circuit.json", "{"), intent); err == nil {
		t.Fatal("malformed circuit accepted")
	}
	if got := deriveGateChainCandidate(schemagate.IntentRequest{}); got.ID != "request" || got.Target != "intent" || !sameStringSet(got.Classes, []string{"read"}) {
		t.Fatalf("default candidate=%#v", got)
	}
}

func TestGateControlsPersistOnlyAfterFinalAllow(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := schemagate.IntentRequest{ToolName: "tool.write", Context: schemagate.IntentContext{RequestID: "req-deferred", Identity: "alice", Workspace: root, RiskClass: "low"}, Targets: []schemagate.IntentTarget{{Kind: "path", Value: "repo", Operation: "write"}}}
	write := func(name string, value any) string {
		path := filepath.Join(root, name)
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	intentPath := write("intent.json", intent)
	statePath := write("state.json", actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1"})
	candidatePath := write("candidate.json", deriveGateChainCandidate(intent))
	chainPolicy := write("chain-policy.json", actioncontract.ChainPolicy{SchemaID: actioncontract.ChainPolicySchemaID, SchemaVersion: "1", MaxSteps: 2})
	stateOut := filepath.Join(root, "next-state.json")
	blockedPolicy := filepath.Join(root, "blocked.yaml")
	if err := os.WriteFile(blockedPolicy, []byte("default_verdict: block\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runGateEval([]string{"--policy", blockedPolicy, "--intent", intentPath, "--chain-policy", chainPolicy, "--chain-state", statePath, "--chain-candidate", candidatePath, "--chain-state-out", stateOut, "--trace-out", filepath.Join(root, "blocked-trace.json"), "--json"}); code != exitPolicyBlocked {
		t.Fatalf("blocked gate=%d", code)
	}
	if _, err := os.Stat(stateOut); !os.IsNotExist(err) {
		t.Fatalf("chain state persisted for blocked action: err=%v", err)
	}
	allowedPolicy := filepath.Join(root, "allowed.yaml")
	if err := os.WriteFile(allowedPolicy, []byte("default_verdict: allow\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runGateEval([]string{"--policy", allowedPolicy, "--intent", intentPath, "--chain-policy", chainPolicy, "--chain-state", statePath, "--chain-candidate", candidatePath, "--chain-state-out", stateOut, "--trace-out", filepath.Join(root, "allowed-trace.json"), "--json"}); code != exitOK {
		t.Fatalf("allowed gate=%d", code)
	}
	if _, err := os.Stat(stateOut); err != nil {
		t.Fatalf("allowed chain state was not persisted: %v", err)
	}
	statePath2 := write("state-retry.json", actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1"})
	stateOut2 := filepath.Join(root, "next-retry.json")
	traceDir := filepath.Join(root, "trace-directory")
	if err := os.Mkdir(traceDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(traceDir, "keep"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runGateEval([]string{"--policy", allowedPolicy, "--intent", intentPath, "--chain-policy", chainPolicy, "--chain-state", statePath2, "--chain-candidate", candidatePath, "--chain-state-out", stateOut2, "--trace-out", traceDir, "--json"}); code == exitOK {
		t.Fatal("unwritable trace output unexpectedly allowed")
	}
	if _, err := os.Stat(stateOut2); !os.IsNotExist(err) {
		t.Fatalf("chain state persisted before trace publication: err=%v", err)
	}
}
