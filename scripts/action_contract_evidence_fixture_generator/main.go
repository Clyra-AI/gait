// Command action_contract_evidence_fixture_generator creates the checked-in,
// non-authoritative lifecycle conformance fixtures. The deterministic key is
// fixture-only and its private half is never written to the repository.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	fixtureRoot      = "testdata/action-contract-evidence/v1"
	sourceProposal   = "testdata/action-contract-interop/v1/expected/compensation/pac-4b7f1402784256ce.json"
	sourceActivation = "testdata/action-contract-interop/v1/expected/compensation/activated-action-contract.json"
	runtimeAction    = "testdata/runtime-goldens/runtime-action.json"
	runtimeReadiness = "testdata/runtime-goldens/runtime-readiness.json"
	seedPhrase       = "gait-action-contract-activation-development-key-v1"
	foundationCommit = "4177f1e575441975b5a8979e6350e988c2f71d70"
	sourceCommit     = "eb4c599a5c1a24dbb270c39a5a513d78f253506d"
)

type manifest struct {
	FixtureVersion   string `json:"fixture_version"`
	FoundationCommit string `json:"foundation_commit"`
	SourceCommit     string `json:"source_commit"`
	Producer         struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Signing struct {
		Mode               string `json:"mode"`
		FixtureTestOnly    bool   `json:"fixture_test_only"`
		DevelopmentSigning bool   `json:"development_signing"`
		NonAuthoritative   bool   `json:"non_authoritative"`
		PublicKeyPath      string `json:"public_key_path"`
		PublicKeySHA256    string `json:"public_key_sha256"`
		KeyID              string `json:"key_id"`
		Derivation         string `json:"derivation"`
	} `json:"signing"`
	Bindings  map[string]string  `json:"bindings"`
	Scenarios []scenarioManifest `json:"scenarios"`
}
type scenarioManifest struct {
	ScenarioID     string `json:"scenario_id"`
	Path           string `json:"path,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	ExpectedValid  bool   `json:"expected_valid"`
	ExpectedReason string `json:"expected_reason,omitempty"`
	EvaluationTime string `json:"evaluation_time,omitempty"`
	Execution      string `json:"execution,omitempty"`
	Effect         string `json:"effect,omitempty"`
	Containment    string `json:"containment,omitempty"`
	Compensation   string `json:"compensation,omitempty"`
}
type fixturePack struct {
	Records []actioncontract.LifecycleRecord `json:"records"`
}

func cloneFixturePack(input fixturePack) fixturePack {
	raw, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	var output fixturePack
	if err := json.Unmarshal(raw, &output); err != nil {
		panic(err)
	}
	return output
}

func main() {
	check := flag.Bool("check", false, "verify exact fixture bytes")
	update := flag.Bool("update", false, "write exact fixture bytes")
	root := flag.String("repo-root", ".", "repository root")
	flag.Parse()
	if *check == *update {
		fatal("exactly one of --check or --update is required")
	}
	if err := run(*root, *update); err != nil {
		fatal("%v", err)
	}
}
func fatal(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }

func run(repoRoot string, update bool) error {
	root := filepath.Join(repoRoot, fixtureRoot)
	proposal, _, err := actioncontract.ReadArtifact(filepath.Join(repoRoot, sourceProposal))
	if err != nil {
		return err
	}
	activationRaw, err := os.ReadFile(filepath.Join(repoRoot, sourceActivation)) // #nosec G304 -- fixed generator-owned activation fixture path.
	if err != nil {
		return err
	}
	activation, err := actioncontract.ParseActivatedArtifact(activationRaw)
	if err != nil {
		return err
	}
	private := fixturePrivateKey()
	public := private.Public().(ed25519.PublicKey)
	publicRaw := []byte(base64.StdEncoding.EncodeToString(public) + "\n")
	if valid, err := actioncontract.VerifyActivationWithOptions(activation, public, actioncontract.VerificationOptions{AllowDevelopmentSigning: true, Proposal: &proposal, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}); err != nil || !valid {
		return fmt.Errorf("source activation verification failed: %v", err)
	}
	readinessRaw, err := os.ReadFile(filepath.Join(repoRoot, runtimeReadiness)) // #nosec G304 -- fixed generator-owned readiness source path.
	if err != nil {
		return err
	}
	var readiness actioncontract.ReadinessResult
	if err := json.Unmarshal(readinessRaw, &readiness); err != nil {
		return err
	}
	actionRaw, err := os.ReadFile(filepath.Join(repoRoot, runtimeAction)) // #nosec G304 -- fixed generator-owned runtime action source path.
	if err != nil {
		return err
	}
	var actionWrapper struct {
		Action json.RawMessage `json:"action"`
	}
	if err := json.Unmarshal(actionRaw, &actionWrapper); err != nil {
		return err
	}
	var action actioncontract.RuntimeAction
	if err := json.Unmarshal(actionWrapper.Action, &action); err != nil {
		return err
	}
	sourceReadinessRaw, sourceActionRaw := readinessRaw, actionRaw
	action.TargetRef = activation.Target
	projectedActionRaw, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	projectedActionRaw = append(projectedActionRaw, '\n')
	validatorPrivate := runtimeValidatorPrivateKey()
	validatorPublic := validatorPrivate.Public().(ed25519.PublicKey)
	precondition := readiness.Preconditions[0]
	precondition.Environment = activation.Environment
	precondition.Target = activation.Target
	precondition.Status = ""
	precondition.ReasonCodes = nil
	precondition.EvidenceDigest = ""
	precondition.ValidatorSignature = ""
	readinessInput := actioncontract.ReadinessInput{ContractID: proposal.ContractID, PolicyDigest: activation.PolicyDigest}
	claimDigest, err := actioncontract.CanonicalReadinessClaimDigest(readinessInput, precondition)
	if err != nil {
		return err
	}
	precondition.EvidenceDigest = claimDigest
	precondition.ValidatorSignature = signReadinessDigest(validatorPrivate, claimDigest)
	readiness = actioncontract.EvaluateReadiness(actioncontract.ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 3, 0, time.UTC), ContractID: proposal.ContractID, PolicyDigest: activation.PolicyDigest, TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validatorPublic}, Preconditions: []actioncontract.ReadinessPrecondition{precondition}})
	if !readiness.Ready || readiness.Status != actioncontract.ReadinessSatisfied {
		return fmt.Errorf("projected readiness is not authoritative: %+v", readiness)
	}
	projectedReadinessRaw, err := json.MarshalIndent(readiness, "", "  ")
	if err != nil {
		return err
	}
	projectedReadinessRaw = append(projectedReadinessRaw, '\n')
	readinessRaw, actionWrapper.Action = projectedReadinessRaw, projectedActionRaw
	proposalRef := proof.RelationshipRef{Kind: "action_contract", ID: proposal.ContractID, Digest: proposal.CanonicalContentDigest, SchemaID: actioncontract.ProposedContractSchemaID, SchemaVersion: actioncontract.ProposedContractVersion, SourceProduct: "wrkr"}
	activationRef := proof.RelationshipRef{Kind: "activated_action_contract", ID: activation.ArtifactID, Digest: activationDigest(activation), SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, SourceProduct: "gait"}
	readinessDigest, err := digestJCS(readinessRaw)
	if err != nil {
		return err
	}
	actionDigest, err := digestJCS(actionWrapper.Action)
	if err != nil {
		return err
	}
	policyRef := ref("policy", "policy-fixture", readiness.PolicyDigest, actioncontract.RuntimeReadinessSchemaID, "1", "gait")
	binding := actioncontract.EvidenceBinding{ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, ContractRef: proposalRef, ActivationRef: activationRef,
		RuntimeActionRef: ref("runtime_action", action.ActionID, actionDigest, actioncontract.RuntimeActionSchemaID, "1", "gait"),
		ReadinessRef:     ref("readiness", "runtime-readiness", readinessDigest, actioncontract.RuntimeReadinessSchemaID, "1", "gait"),
		DecisionRef:      ref("decision", "runtime-decision", readinessDigest, actioncontract.RuntimeReadinessSchemaID, "1", "gait"), PolicyRef: policyRef,
		TargetRef:      ref("target", activation.Target, digestText(activation.Target), "https://gait.dev/schemas/v1/target-ref.schema.json", "1", "gait"),
		EnvironmentRef: ref("environment", activation.Environment, digestText(activation.Environment), "https://gait.dev/schemas/v1/environment-ref.schema.json", "1", "gait"),
		ProofRefs:      []proof.RelationshipRef{ref("proof", "proof-fixture", digestText("proof-fixture"), "https://proof.dev/schemas/v1/proof-ref.schema.json", "1", "proof")},
		CausalRefs:     []proof.RelationshipRef{proposalRef},
		Correlation:    proof.ControlContainmentTelemetryProfile{ProfileVersion: actioncontract.CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &proposalRef, ContentDigest: proposalRef.Digest}}
	if err := binding.Validate(); err != nil {
		return fmt.Errorf("binding: %w", err)
	}
	base := fixtureBase{proposalRef: proposalRef, activationRef: activationRef, binding: binding, readiness: &readiness, private: private}
	validPacks := map[string]fixturePack{}
	validPacks["successful-execution-effect-containment"] = base.successful("completed", false)
	validPacks["blocked-before-execution"] = base.blocked()
	validPacks["failed-execution-compensation"] = base.failed()
	validPacks["partial-containment"] = base.successful("partial", false)
	validPacks["unresolved-containment"] = base.successful("unresolved", false)
	validPacks["compensation-required-started-completed"] = base.successful("completed", true)
	files := map[string][]byte{"runtime-action.json": projectedActionRaw, "runtime-readiness.json": projectedReadinessRaw}
	for id, pack := range validPacks {
		raw, err := json.MarshalIndent(pack, "", "  ")
		if err != nil {
			return err
		}
		files[id+"/lifecycle.json"] = append(raw, '\n')
	}
	negatives := []scenarioManifest{
		{ScenarioID: "stale-activation", Path: "stale-activation/lifecycle.json", ExpectedValid: false, ExpectedReason: "activation_expired", EvaluationTime: "2036-07-20T00:00:00Z"},
		{ScenarioID: "replay-mismatched-lineage", Path: "replay-mismatched-lineage/lifecycle.json", ExpectedValid: false, ExpectedReason: actioncontract.ReasonConformanceReplay, EvaluationTime: "2026-07-20T00:00:00Z"},
		{ScenarioID: "identifier-only-correlation", Path: "identifier-only-correlation/lifecycle.json", ExpectedValid: false, ExpectedReason: actioncontract.ReasonConformanceIdentifierOnly, EvaluationTime: "2026-07-20T00:00:00Z"},
	}
	for _, n := range negatives {
		negativePack := cloneFixturePack(validPacks["successful-execution-effect-containment"])
		switch n.ScenarioID {
		case "replay-mismatched-lineage":
			negativePack.Records = append(negativePack.Records, negativePack.Records[len(negativePack.Records)-1])
		case "identifier-only-correlation":
			negativePack.Records[5].Correlation.BindingMode = proof.BindingModeIdentifierOnly
			negativePack.Records[5] = resignLifecycleFixture(negativePack.Records[5], private)
		}
		negativeRaw, marshalErr := json.MarshalIndent(negativePack, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		files[n.ScenarioID+"/lifecycle.json"] = append(negativeRaw, '\n')
	}
	proposalRaw, err := os.ReadFile(filepath.Join(repoRoot, sourceProposal)) // #nosec G304 -- fixed generator-owned proposal fixture path.
	if err != nil {
		return err
	}
	manifestValue := manifest{FixtureVersion: "1", FoundationCommit: foundationCommit, SourceCommit: sourceCommit, Bindings: map[string]string{
		"proposal_path": sourceProposal, "proposal_sha256": rawDigest(proposalRaw),
		"activation_path": sourceActivation, "activation_sha256": rawDigest(activationRaw),
		"runtime_action_path": fixtureRoot + "/runtime-action.json", "runtime_action_sha256": rawDigest(projectedActionRaw),
		"runtime_readiness_path": fixtureRoot + "/runtime-readiness.json", "runtime_readiness_sha256": rawDigest(projectedReadinessRaw),
		"runtime_action_source_path": runtimeAction, "runtime_action_source_sha256": rawDigest(sourceActionRaw),
		"runtime_readiness_source_path": runtimeReadiness, "runtime_readiness_source_sha256": rawDigest(sourceReadinessRaw),
		"readiness_validator_key_path": "testdata/runtime-goldens/fixture-signing-key.public.b64", "readiness_validator_key_sha256": rawDigest([]byte(base64.StdEncoding.EncodeToString(validatorPublic) + "\n")),
	}}
	manifestValue.Producer.Name, manifestValue.Producer.Version = "gait", "v1.5.0"
	manifestValue.Signing.Mode, manifestValue.Signing.FixtureTestOnly, manifestValue.Signing.DevelopmentSigning, manifestValue.Signing.NonAuthoritative = "fixture_only_deterministic_development_key", true, true, true
	manifestValue.Signing.PublicKeyPath, manifestValue.Signing.PublicKeySHA256, manifestValue.Signing.KeyID, manifestValue.Signing.Derivation = fixtureRoot+"/fixture-signing-key.public.b64", rawDigest(publicRaw), proofsign.KeyID(public), "sha256("+seedPhrase+") as Ed25519 seed; never used by production/default lifecycle signing"
	validIDs := make([]string, 0, len(validPacks))
	for id := range validPacks {
		validIDs = append(validIDs, id)
	}
	sort.Strings(validIDs)
	for _, id := range validIDs {
		s := scenarioManifest{ScenarioID: id, Path: id + "/lifecycle.json", ExpectedValid: true}
		assignExpected(&s, id)
		manifestValue.Scenarios = append(manifestValue.Scenarios, s)
	}
	manifestValue.Scenarios = append(manifestValue.Scenarios, negatives...)
	for i := range manifestValue.Scenarios {
		if manifestValue.Scenarios[i].Path != "" {
			manifestValue.Scenarios[i].SHA256 = rawDigest(files[manifestValue.Scenarios[i].Path])
		}
	}
	files["fixture-signing-key.public.b64"] = publicRaw
	manifestRaw, err := json.MarshalIndent(manifestValue, "", "  ")
	if err != nil {
		return err
	}
	files["fixture-manifest.json"] = append(manifestRaw, '\n')
	if update {
		if err := os.MkdirAll(root, 0o750); err != nil {
			return err
		}
		for name, raw := range files {
			dir := filepath.Dir(filepath.Join(root, name))
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
				return err
			}
		}
		return nil
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(root, name)) // #nosec G304 -- name is restricted to the generator-owned fixture allowlist.
		if err != nil {
			return err
		}
		if string(got) != string(want) {
			return fmt.Errorf("fixture drift: %s", name)
		}
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, ok := files[filepath.ToSlash(rel)]; !ok {
			return fmt.Errorf("fixture orphan: %s", rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

type fixtureBase struct {
	proposalRef, activationRef proof.RelationshipRef
	binding                    actioncontract.EvidenceBinding
	readiness                  *actioncontract.ReadinessResult
	private                    ed25519.PrivateKey
}

func (b fixtureBase) corr(event, causal *proof.RelationshipRef) proof.ControlContainmentTelemetryProfile {
	return proof.ControlContainmentTelemetryProfile{ProfileVersion: actioncontract.CorrelationProfileVersion, BindingMode: proof.BindingModeDigestBound, ContractRef: &b.proposalRef, EventRef: event, CausalRef: &b.proposalRef, ContentDigest: b.proposalRef.Digest}
}
func (b fixtureBase) record(kind actioncontract.LifecycleEventKind, at int, proposal, activation bool, evidence any, decision *actioncontract.ReadinessResult, pre []proof.RelationshipRef, causal *proof.RelationshipRef) actioncontract.LifecycleRecord {
	var p, a *proof.RelationshipRef
	if proposal {
		x := b.proposalRef
		p = &x
	}
	if activation {
		x := b.activationRef
		a = &x
	}
	binding := b.binding
	if causal != nil {
		binding.CausalRefs = []proof.RelationshipRef{*causal}
	}
	var exec *actioncontract.ExecutionEvidence
	var effect *actioncontract.EffectEvent
	var containment *actioncontract.ContainmentEvidence
	var compensation *actioncontract.CompensationEvidence
	switch v := evidence.(type) {
	case actioncontract.ExecutionEvidence:
		exec = &v
	case actioncontract.EffectEvent:
		effect = &v
	case actioncontract.ContainmentEvidence:
		containment = &v
	case actioncontract.CompensationEvidence:
		compensation = &v
	}
	r, err := actioncontract.NewLifecycleRecord(actioncontract.LifecycleRecordOptions{Kind: kind, OccurredAt: time.Date(2026, 7, 19, 1, 0, at, 0, time.UTC), ContractRef: b.proposalRef, ContractFamilyID: b.binding.ContractFamilyID, Revision: 1, ProposalRef: p, ActivationRef: a, Decision: decision, PreconditionRefs: pre, Execution: exec, Effect: effect, Containment: containment, Compensation: compensation, Correlation: b.corr(a, causal), SigningPrivateKey: b.private})
	if err != nil {
		panic(fmt.Sprintf("%s: %v binding=%+v evidence=%#v", kind, err, binding, evidence))
	}
	return r
}
func (b fixtureBase) successful(containmentOutcome string, compensation bool) fixturePack {
	at := 0
	preRef := proof.RelationshipRef{Kind: "precondition", ID: "runtime-p1", Digest: b.readiness.Preconditions[0].EvidenceDigest, SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: "1", SourceProduct: "gait"}
	records := []actioncontract.LifecycleRecord{b.record(actioncontract.LifecycleProposalIngested, at, true, false, nil, nil, nil, nil)}
	at++
	records = append(records, b.record(actioncontract.LifecycleActivationRequested, at, true, true, nil, nil, nil, nil))
	at++
	records = append(records, b.record(actioncontract.LifecyclePreconditionEvaluated, at, true, true, nil, nil, []proof.RelationshipRef{preRef}, nil))
	at++
	records = append(records, b.record(actioncontract.LifecycleDecisionReady, at, true, true, nil, b.readiness, nil, nil))
	at++
	records = append(records, b.record(actioncontract.LifecycleActivated, at, true, true, nil, nil, nil, nil))
	at++
	start, _ := actioncontract.NewExecutionEvidence(actioncontract.ExecutionEvidence{Binding: b.binding, EventRef: ref("event", "execution-start", digestText("execution-start"), actioncontract.ExecutionEvidenceSchemaID, "1", "gait"), OccurredAt: "2026-07-19T01:00:05Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "started", CompensationRequired: compensation}, b.private)
	records = append(records, b.record(actioncontract.LifecycleExecutionStarted, at, true, true, start, nil, nil, &b.proposalRef))
	at++
	startRef := evidenceRef("execution", start)
	done, _ := actioncontract.NewExecutionEvidence(actioncontract.ExecutionEvidence{Binding: withCausal(b.binding, startRef), EventRef: ref("event", "execution-done", digestText("execution-done"), actioncontract.ExecutionEvidenceSchemaID, "1", "gait"), OccurredAt: "2026-07-19T01:00:06Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "succeeded", CompensationRequired: compensation}, b.private)
	records = append(records, b.record(actioncontract.LifecycleExecutionSucceeded, at, true, true, done, nil, nil, &startRef))
	at++
	doneRef := evidenceRef("execution", done)
	effectArtifact := ref("effect", "effect-fixture", digestText("effect-fixture"), "https://gait.dev/schemas/v1/effect-event.schema.json", "1", "gait")
	effect, _ := actioncontract.NewEffectEvent(actioncontract.EffectEvent{Binding: withCausal(b.binding, doneRef), EventRef: ref("event", "effect-recorded", digestText("effect-recorded"), actioncontract.EffectEventSchemaID, "1", "gait"), ExecutionRef: doneRef, EffectRef: effectArtifact, OccurredAt: "2026-07-19T01:00:07Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "recorded"}, b.private)
	records = append(records, b.record(actioncontract.LifecycleEffectRecorded, at, true, true, effect, nil, nil, &doneRef))
	at++
	effectRef := evidenceRef("effect_event", effect)
	effectValidated, _ := actioncontract.NewEffectEvent(actioncontract.EffectEvent{Binding: withCausal(b.binding, effectRef), EventRef: ref("event", "effect-validated", digestText("effect-validated"), actioncontract.EffectEventSchemaID, "1", "gait"), ExecutionRef: doneRef, EffectRef: effectArtifact, OccurredAt: "2026-07-19T01:00:08Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "validated"}, b.private)
	records = append(records, b.record(actioncontract.LifecycleEffectValidated, at, true, true, effectValidated, nil, nil, &effectRef))
	at++
	effectValidatedRef := evidenceRef("effect_event", effectValidated)
	scope := ref("containment", "containment-scope", digestText("containment-scope"), "https://gait.dev/schemas/v1/containment-ref.schema.json", "1", "gait")
	requested, _ := actioncontract.NewContainmentEvidence(actioncontract.ContainmentEvidence{Binding: withCausal(b.binding, effectValidatedRef), EventRef: ref("event", "containment-request", digestText("containment-request"), actioncontract.ContainmentEvidenceSchemaID, "1", "gait"), ExecutionRef: doneRef, EffectRef: effectValidatedRef, ContainmentRef: scope, OccurredAt: "2026-07-19T01:00:09Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "requested"}, b.private)
	records = append(records, b.record(actioncontract.LifecycleContainmentRequested, at, true, true, requested, nil, nil, &effectValidatedRef))
	at++
	requestRef := evidenceRef("containment", requested)
	terminal, _ := actioncontract.NewContainmentEvidence(actioncontract.ContainmentEvidence{Binding: withCausal(b.binding, requestRef), EventRef: ref("event", "containment-terminal", digestText("containment-terminal"), actioncontract.ContainmentEvidenceSchemaID, "1", "gait"), ExecutionRef: doneRef, EffectRef: effectValidatedRef, ContainmentRef: scope, OccurredAt: "2026-07-19T01:00:10Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: containmentOutcome}, b.private)
	records = append(records, b.record(containmentKind(containmentOutcome), at, true, true, terminal, nil, nil, &requestRef))
	at++
	if compensation {
		req := ref("compensation_requirement", "compensation-fixture", digestText("compensation-fixture"), "https://gait.dev/schemas/v1/compensation-ref.schema.json", "1", "gait")
		required, _ := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, doneRef), EventRef: ref("event", "compensation-required", digestText("compensation-required"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: req, ExecutionRef: doneRef, OccurredAt: "2026-07-19T01:00:11Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "required"}, b.private)
		records = append(records, b.record(actioncontract.LifecycleCompensationRequired, at, true, true, required, nil, nil, &doneRef))
		at++
		requiredRef := evidenceRef("compensation", required)
		started, _ := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, requiredRef), EventRef: ref("event", "compensation-started", digestText("compensation-started"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: req, ExecutionRef: doneRef, OccurredAt: "2026-07-19T01:00:12Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "started"}, b.private)
		records = append(records, b.record(actioncontract.LifecycleCompensationStarted, at, true, true, started, nil, nil, &requiredRef))
		at++
		startedRef := evidenceRef("compensation", started)
		completed, _ := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, startedRef), EventRef: ref("event", "compensation-completed", digestText("compensation-completed"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: req, ExecutionRef: doneRef, OccurredAt: "2026-07-19T01:00:13Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "completed"}, b.private)
		records = append(records, b.record(actioncontract.LifecycleCompensationCompleted, at, true, true, completed, nil, nil, &startedRef))
	}
	return fixturePack{Records: records}
}
func (b fixtureBase) blocked() fixturePack {
	at := 0
	records := []actioncontract.LifecycleRecord{b.record(actioncontract.LifecycleProposalIngested, at, true, false, nil, nil, nil, nil)}
	at++
	records = append(records, b.record(actioncontract.LifecycleActivationRequested, at, true, true, nil, nil, nil, nil))
	at++
	records = append(records, b.record(actioncontract.LifecyclePreconditionEvaluated, at, true, true, nil, nil, []proof.RelationshipRef{{Kind: "precondition", ID: "runtime-p1", Digest: b.readiness.Preconditions[0].EvidenceDigest, SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: "1", SourceProduct: "gait"}}, nil))
	at++
	records = append(records, b.record(actioncontract.LifecycleDecisionReady, at, true, true, nil, b.readiness, nil, nil))
	at++
	records = append(records, b.record(actioncontract.LifecycleActivated, at, true, true, nil, nil, nil, nil))
	at++
	blocked, _ := actioncontract.NewExecutionEvidence(actioncontract.ExecutionEvidence{Binding: withCausal(b.binding, b.activationRef), EventRef: ref("event", "execution-blocked", digestText("execution-blocked"), actioncontract.ExecutionEvidenceSchemaID, "1", "gait"), OccurredAt: "2026-07-19T01:00:05Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "blocked"}, b.private)
	records = append(records, b.record(actioncontract.LifecycleExecutionBlocked, at, true, true, blocked, nil, nil, &b.activationRef))
	return fixturePack{Records: records}
}
func (b fixtureBase) failed() fixturePack {
	// A failed execution cannot produce an authoritative effect. It can still
	// require and complete compensation, which is the fail-closed recovery path.
	pack := b.blocked()
	records := append([]actioncontract.LifecycleRecord(nil), pack.Records[:5]...)
	started, err := actioncontract.NewExecutionEvidence(actioncontract.ExecutionEvidence{Binding: b.binding, EventRef: ref("event", "execution-started-failed", digestText("execution-started-failed"), actioncontract.ExecutionEvidenceSchemaID, "1", "gait"), OccurredAt: "2026-07-19T01:00:05Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "started", CompensationRequired: true}, b.private)
	if err != nil {
		panic(err)
	}
	records = append(records, b.record(actioncontract.LifecycleExecutionStarted, 5, true, true, started, nil, nil, &b.proposalRef))
	startedRef := evidenceRef("execution", started)
	failed, err := actioncontract.NewExecutionEvidence(actioncontract.ExecutionEvidence{Binding: withCausal(b.binding, startedRef), EventRef: ref("event", "execution-failed", digestText("execution-failed"), actioncontract.ExecutionEvidenceSchemaID, "1", "gait"), OccurredAt: "2026-07-19T01:00:06Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "failed", CompensationRequired: true}, b.private)
	if err != nil {
		panic(err)
	}
	records = append(records, b.record(actioncontract.LifecycleExecutionFailed, 6, true, true, failed, nil, nil, &startedRef))
	failedRef := evidenceRef("execution", failed)
	requiredRef := ref("compensation_requirement", "compensation-failed", digestText("compensation-failed"), "https://gait.dev/schemas/v1/compensation-ref.schema.json", "1", "gait")
	required, err := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, failedRef), EventRef: ref("event", "compensation-required-failed", digestText("compensation-required-failed"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: requiredRef, ExecutionRef: failedRef, OccurredAt: "2026-07-19T01:00:06Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "required"}, b.private)
	if err != nil {
		panic(err)
	}
	records = append(records, b.record(actioncontract.LifecycleCompensationRequired, 7, true, true, required, nil, nil, &failedRef))
	requiredEvidenceRef := evidenceRef("compensation", required)
	compensationStarted, err := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, requiredEvidenceRef), EventRef: ref("event", "compensation-started-failed", digestText("compensation-started-failed"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: requiredRef, ExecutionRef: failedRef, OccurredAt: "2026-07-19T01:00:08Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "started"}, b.private)
	if err != nil {
		panic(err)
	}
	records = append(records, b.record(actioncontract.LifecycleCompensationStarted, 8, true, true, compensationStarted, nil, nil, &requiredEvidenceRef))
	startedEvidenceRef := evidenceRef("compensation", compensationStarted)
	completed, err := actioncontract.NewCompensationEvidence(actioncontract.CompensationEvidence{Binding: withCausal(b.binding, startedEvidenceRef), EventRef: ref("event", "compensation-completed-failed", digestText("compensation-completed-failed"), actioncontract.CompensationEvidenceSchemaID, "1", "gait"), RequirementRef: requiredRef, ExecutionRef: failedRef, OccurredAt: "2026-07-19T01:00:09Z", FreshUntil: "2026-07-19T03:00:00Z", Outcome: "completed"}, b.private)
	if err != nil {
		panic(err)
	}
	records = append(records, b.record(actioncontract.LifecycleCompensationCompleted, 9, true, true, completed, nil, nil, &startedEvidenceRef))
	return fixturePack{Records: records}
}
func withCausal(b actioncontract.EvidenceBinding, causal proof.RelationshipRef) actioncontract.EvidenceBinding {
	b.CausalRefs = []proof.RelationshipRef{causal}
	return b
}
func evidenceRef(kind string, item any) proof.RelationshipRef {
	switch v := item.(type) {
	case actioncontract.ExecutionEvidence:
		return ref(kind, v.EvidenceID, v.CanonicalContentDigest, actioncontract.ExecutionEvidenceSchemaID, "1", "gait")
	case actioncontract.EffectEvent:
		return ref(kind, v.EvidenceID, v.CanonicalContentDigest, actioncontract.EffectEventSchemaID, "1", "gait")
	case actioncontract.ContainmentEvidence:
		return ref(kind, v.EvidenceID, v.CanonicalContentDigest, actioncontract.ContainmentEvidenceSchemaID, "1", "gait")
	case actioncontract.CompensationEvidence:
		return ref(kind, v.EvidenceID, v.CanonicalContentDigest, actioncontract.CompensationEvidenceSchemaID, "1", "gait")
	default:
		return proof.RelationshipRef{}
	}
}
func ref(kind, id, digest, schema, version, source string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: digest, SchemaID: schema, SchemaVersion: version, SourceProduct: source}
}
func digestText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func fixturePrivateKey() ed25519.PrivateKey {
	sum := sha256.Sum256([]byte(seedPhrase))
	return ed25519.NewKeyFromSeed(sum[:])
}
func runtimeValidatorPrivateKey() ed25519.PrivateKey {
	sum := sha256.Sum256([]byte("runtime-golden-key"))
	return ed25519.NewKeyFromSeed(sum[:])
}
func signReadinessDigest(private ed25519.PrivateKey, digest string) string {
	raw, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(private, raw))
}
func resignLifecycleFixture(record actioncontract.LifecycleRecord, private ed25519.PrivateKey) actioncontract.LifecycleRecord {
	record.RecordID = ""
	record.Signature = proofsign.Signature{}
	raw, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	digest, err := digestJCS(raw)
	if err != nil {
		panic(err)
	}
	record.RecordID = "gait-lr-" + strings.TrimPrefix(digest, "sha256:")[:16]
	record.Signature, err = proofsign.SignDigestHex(private, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		panic(err)
	}
	return record
}
func activationDigest(a actioncontract.ActivatedArtifact) string {
	copy := a
	copy.ArtifactID = ""
	copy.Signature = proofsign.Signature{}
	raw, _ := json.Marshal(copy)
	d, _ := digestJCS(raw)
	return d
}

func digestJCS(raw []byte) (string, error) {
	digest, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	return "sha256:" + strings.TrimPrefix(digest, "sha256:"), nil
}
func assignExpected(s *scenarioManifest, id string) {
	switch id {
	case "successful-execution-effect-containment":
		s.Execution, s.Effect, s.Containment = "succeeded", "validated", "completed"
	case "blocked-before-execution":
		s.Execution = "blocked"
	case "failed-execution-compensation":
		s.Execution, s.Compensation = "failed", "completed"
	case "partial-containment":
		s.Execution, s.Effect, s.Containment = "succeeded", "validated", "partial"
	case "unresolved-containment":
		s.Execution, s.Effect, s.Containment = "succeeded", "validated", "unresolved"
	case "compensation-required-started-completed":
		s.Execution, s.Effect, s.Containment, s.Compensation = "succeeded", "validated", "completed", "completed"
	}
}
func containmentKind(outcome string) actioncontract.LifecycleEventKind {
	switch outcome {
	case "completed":
		return actioncontract.LifecycleContainmentCompleted
	case "partial":
		return actioncontract.LifecycleContainmentPartial
	default:
		return actioncontract.LifecycleContainmentUnresolved
	}
}
