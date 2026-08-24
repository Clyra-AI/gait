package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	proofsign "github.com/Clyra-AI/proof/signing"
)

type conformanceFixturePack struct {
	Records []LifecycleRecord `json:"records"`
}

type conformanceFixtureManifest struct {
	FixtureVersion   string `json:"fixture_version"`
	FoundationCommit string `json:"foundation_commit"`
	Producer         struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Bindings map[string]string `json:"bindings"`
	Signing  struct {
		FixtureTestOnly  bool   `json:"fixture_test_only"`
		NonAuthoritative bool   `json:"non_authoritative"`
		PublicKeyPath    string `json:"public_key_path"`
		PublicKeySHA256  string `json:"public_key_sha256"`
		KeyID            string `json:"key_id"`
	} `json:"signing"`
	Scenarios []struct {
		ScenarioID     string `json:"scenario_id"`
		Path           string `json:"path"`
		SHA256         string `json:"sha256"`
		ExpectedValid  bool   `json:"expected_valid"`
		ExpectedReason string `json:"expected_reason"`
		EvaluationTime string `json:"evaluation_time"`
	} `json:"scenarios"`
}

func loadConformanceFixture(t *testing.T, scenario string) (Artifact, ActivatedArtifact, RuntimeAction, ReadinessResult, []LifecycleRecord, ed25519.PublicKey, ed25519.PublicKey) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "action-contract-evidence", "v1")
	proposal, _, err := ReadArtifact(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "pac-4b7f1402784256ce.json"))
	if err != nil {
		t.Fatal(err)
	}
	activationRaw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "compensation", "activated-action-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	activation, err := ParseActivatedArtifact(activationRaw)
	if err != nil {
		t.Fatal(err)
	}
	actionRaw, err := os.ReadFile(filepath.Join(root, "runtime-action.json"))
	if err != nil {
		t.Fatal(err)
	}
	action, err := ParseRuntimeAction(actionRaw)
	if err != nil {
		t.Fatal(err)
	}
	readinessRaw, err := os.ReadFile(filepath.Join(root, "runtime-readiness.json"))
	if err != nil {
		t.Fatal(err)
	}
	var readiness ReadinessResult
	if err := json.Unmarshal(readinessRaw, &readiness); err != nil {
		t.Fatal(err)
	}
	packRaw, err := os.ReadFile(filepath.Join(root, scenario, "lifecycle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pack conformanceFixturePack
	if err := json.Unmarshal(packRaw, &pack); err != nil {
		t.Fatal(err)
	}
	keyRaw, err := os.ReadFile(filepath.Join(root, "fixture-signing-key.public.b64"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := base64.StdEncoding.DecodeString(string(keyRaw))
	if err != nil || len(key) != ed25519.PublicKeySize {
		t.Fatal(err)
	}
	validatorRaw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "runtime-goldens", "fixture-signing-key.public.b64"))
	if err != nil {
		t.Fatal(err)
	}
	validatorKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(validatorRaw)))
	if err != nil || len(validatorKey) != ed25519.PublicKeySize {
		t.Fatal(err)
	}
	return proposal, activation, action, readiness, pack.Records, ed25519.PublicKey(key), ed25519.PublicKey(validatorKey)
}

func TestLifecycleConformanceManifestPinsExactBytes(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-evidence", "v1")
	manifestRaw, err := os.ReadFile(filepath.Join(root, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest conformanceFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FixtureVersion != "1" || manifest.FoundationCommit != "4177f1e575441975b5a8979e6350e988c2f71d70" || manifest.Producer.Name != "gait" || manifest.Producer.Version != "v1.5.0" || !manifest.Signing.FixtureTestOnly || !manifest.Signing.NonAuthoritative || len(manifest.Scenarios) != 9 {
		t.Fatalf("unexpected conformance fixture manifest: %+v", manifest)
	}
	for _, prefix := range []string{"proposal", "activation", "runtime_action", "runtime_readiness", "runtime_action_source", "runtime_readiness_source"} {
		path, digest := manifest.Bindings[prefix+"_path"], manifest.Bindings[prefix+"_sha256"]
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if RawDigest(raw) != digest {
			t.Fatalf("bound source drift for %s", prefix)
		}
	}
	publicRaw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(manifest.Signing.PublicKeyPath)))
	if err != nil {
		t.Fatal(err)
	}
	public, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(publicRaw)))
	if err != nil || len(public) != ed25519.PublicKeySize || RawDigest(publicRaw) != manifest.Signing.PublicKeySHA256 || proofsign.KeyID(ed25519.PublicKey(public)) != manifest.Signing.KeyID {
		t.Fatalf("fixture public key provenance drift: %v", err)
	}
	validatorRaw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(manifest.Bindings["readiness_validator_key_path"])))
	if err != nil || RawDigest(validatorRaw) != manifest.Bindings["readiness_validator_key_sha256"] {
		t.Fatalf("readiness validator key drift: %v", err)
	}
	allowed := map[string]bool{"fixture-manifest.json": true, "fixture-signing-key.public.b64": true, "runtime-action.json": true, "runtime-readiness.json": true}
	seen := map[string]bool{}
	for _, scenario := range manifest.Scenarios {
		if seen[scenario.ScenarioID] || scenario.Path == "" {
			t.Fatalf("duplicate or missing scenario path: %+v", scenario)
		}
		seen[scenario.ScenarioID] = true
		path := filepath.Join(root, filepath.FromSlash(scenario.Path))
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("unsafe scenario path: %q", scenario.Path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if RawDigest(raw) != scenario.SHA256 || bytes.Contains(raw, []byte("\r\n")) {
			t.Fatalf("scenario byte drift: %s", scenario.Path)
		}
		allowed[filepath.ToSlash(rel)] = true
	}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !allowed[filepath.ToSlash(rel)] || strings.Contains(strings.ToLower(info.Name()), "private") {
			return fmt.Errorf("unexpected or private fixture file: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleConformanceFixtureSuccessfulLineage(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	decisionTime, err := time.Parse(time.RFC3339Nano, records[3].OccurredAt)
	if err != nil {
		t.Fatal(err)
	}
	revalidated := EvaluateReadiness(ReadinessInput{Now: decisionTime, ContractID: readiness.ContractID, PolicyDigest: readiness.PolicyDigest, Preconditions: readiness.Preconditions, TrustedValidatorRefs: []string{"validator"}, TrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}})
	if !revalidated.Ready {
		t.Fatalf("fixture readiness failed revalidation: %+v", revalidated)
	}
	result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Expectation: LifecycleConformanceExpectation{ExecutionOutcome: "succeeded", EffectOutcome: "validated", ContainmentOutcome: "completed", RequireComplete: true}})
	if !result.Valid || !result.AuthoritativeSuccess {
		t.Fatalf("successful fixture rejected: %+v", result)
	}
}

func TestLifecycleConformanceFixtureScenarioMatrix(t *testing.T) {
	cases := []struct {
		name                 string
		expect               LifecycleConformanceExpectation
		authoritativeSuccess bool
	}{
		{"blocked-before-execution", LifecycleConformanceExpectation{ExecutionOutcome: "blocked"}, false},
		{"failed-execution-compensation", LifecycleConformanceExpectation{ExecutionOutcome: "failed", CompensationOutcome: "completed"}, false},
		{"partial-containment", LifecycleConformanceExpectation{ExecutionOutcome: "succeeded", EffectOutcome: "validated", ContainmentOutcome: "partial"}, false},
		{"unresolved-containment", LifecycleConformanceExpectation{ExecutionOutcome: "succeeded", EffectOutcome: "validated", ContainmentOutcome: "unresolved"}, false},
		{"compensation-required-started-completed", LifecycleConformanceExpectation{ExecutionOutcome: "succeeded", EffectOutcome: "validated", ContainmentOutcome: "completed", CompensationOutcome: "completed", RequireComplete: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, tc.name)
			result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Expectation: tc.expect})
			if !result.Valid {
				t.Fatalf("fixture rejected: %+v", result)
			}
			if result.AuthoritativeSuccess != tc.authoritativeSuccess {
				t.Fatalf("authoritative success mismatch: got %t want %t result=%+v", result.AuthoritativeSuccess, tc.authoritativeSuccess, result)
			}
		})
	}
}

func TestLifecycleConformanceFailsClosedForWrongKeyAndReplay(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	wrong := make(ed25519.PublicKey, ed25519.PublicKeySize)
	result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: wrong, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)})
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceActivationInvalid) {
		t.Fatalf("wrong key accepted: %+v", result)
	}
	result = VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: append(records, records[len(records)-1]), LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)})
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceReplay) {
		t.Fatalf("replayed lineage accepted: %+v", result)
	}
}

func TestLifecycleConformanceRejectsIdentifierOnlyCorrelation(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	records[5].Correlation.BindingMode = "identifier_only"
	result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)})
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceIdentifierOnly) {
		t.Fatalf("identifier-only lineage accepted: %+v", result)
	}
}

func TestLifecycleConformanceNegativeFixtureMatrix(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-evidence", "v1")
	manifestRaw, err := os.ReadFile(filepath.Join(root, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest conformanceFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, scenario := range manifest.Scenarios {
		if scenario.ExpectedValid {
			continue
		}
		scenario := scenario
		t.Run(scenario.ScenarioID, func(t *testing.T) {
			proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, scenario.ScenarioID)
			for _, record := range records {
				digest, err := lifecycleDigest(record)
				if err != nil || record.Signature.SignedDigest != strings.TrimPrefix(digest, "sha256:") {
					t.Fatalf("negative fixture outer digest is not authentic: kind=%s err=%v", record.Kind, err)
				}
				valid, err := proofsign.VerifyDigestHex(public, record.Signature)
				if err != nil || !valid {
					t.Fatalf("negative fixture outer signature is not authentic: kind=%s err=%v", record.Kind, err)
				}
			}
			evaluationTime, err := time.Parse(time.RFC3339, scenario.EvaluationTime)
			if err != nil {
				t.Fatal(err)
			}
			result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: evaluationTime})
			if result.Valid || !containsString(result.ReasonCodes, scenario.ExpectedReason) {
				t.Fatalf("negative fixture did not fail closed: expected=%s result=%+v", scenario.ExpectedReason, result)
			}
		})
		checked++
	}
	if checked != 3 {
		t.Fatalf("negative fixture coverage drift: got %d want 3", checked)
	}
}

func TestLifecycleConformanceRequiresExplicitEvaluationTime(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	result := VerifyLifecycleConformance(LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true})
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceInputMissing) {
		t.Fatalf("missing evaluation time accepted: %+v", result)
	}
}

func TestLifecycleConformanceBindsExactRuntimeAndReadinessArtifacts(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	base := LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}
	wrongAction := base
	wrongAction.RuntimeAction.ActionID = "runtime-action-mismatch"
	result := VerifyLifecycleConformance(wrongAction)
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceLineageMismatch) {
		t.Fatalf("mismatched runtime action accepted: %+v", result)
	}
	wrongTarget := base
	wrongTarget.RuntimeAction.TargetRef = "target:outside-activation"
	result = VerifyLifecycleConformance(wrongTarget)
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceLineageMismatch) {
		t.Fatalf("runtime target outside activation accepted: %+v", result)
	}
	wrongReadiness := base
	wrongReadiness.Readiness.PolicyDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	result = VerifyLifecycleConformance(wrongReadiness)
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceReadinessInvalid) {
		t.Fatalf("mismatched readiness accepted: %+v", result)
	}
	missingRuntime := base
	missingRuntime.RuntimeAction = RuntimeAction{}
	result = VerifyLifecycleConformance(missingRuntime)
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceInputMissing) {
		t.Fatalf("missing runtime action accepted: %+v", result)
	}
	wrongValidator := base
	wrongValidator.ReadinessTrustedValidatorKeys = map[string]ed25519.PublicKey{"validator": make(ed25519.PublicKey, ed25519.PublicKeySize)}
	result = VerifyLifecycleConformance(wrongValidator)
	if result.Valid || !containsString(result.ReasonCodes, ReasonConformanceReadinessInvalid) {
		t.Fatalf("readiness signed by an untrusted key accepted: %+v", result)
	}
}

func TestLifecycleConformanceStableFailureBranchesAndAlias(t *testing.T) {
	proposal, activation, action, readiness, records, public, validator := loadConformanceFixture(t, "successful-execution-effect-containment")
	base := LifecycleConformanceInput{Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, ReadinessTrustedValidatorRefs: []string{"validator"}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{"validator": validator}, ActivationPublicKey: public, LifecycleRecords: records, LifecyclePublicKey: public, AllowDevelopmentSign: true, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}
	if result := GradeLifecycleConformance(base); !result.Valid || !result.AuthoritativeSuccess {
		t.Fatalf("grader alias rejected valid lineage: %+v", result)
	}
	wrongExpectation := base
	wrongExpectation.Expectation.ExecutionOutcome = "blocked"
	if result := VerifyLifecycleConformance(wrongExpectation); result.Valid || !containsString(result.ReasonCodes, "execution_outcome_mismatch") {
		t.Fatalf("expectation mismatch accepted: %+v", result)
	}
	invalidProposal := base
	invalidProposal.Proposal.CanonicalContentDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if result := VerifyLifecycleConformance(invalidProposal); result.Valid || !containsString(result.ReasonCodes, ReasonConformanceProposalInvalid) {
		t.Fatalf("invalid proposal accepted: %+v", result)
	}
	invalidRuntime := base
	invalidRuntime.RuntimeAction.SchemaID = "wrong"
	if result := VerifyLifecycleConformance(invalidRuntime); result.Valid || !containsString(result.ReasonCodes, ReasonConformanceRuntimeInvalid) {
		t.Fatalf("invalid runtime action accepted: %+v", result)
	}
	invalidReadiness := base
	invalidReadiness.Readiness.Ready = false
	if result := VerifyLifecycleConformance(invalidReadiness); result.Valid || !containsString(result.ReasonCodes, ReasonConformanceReadinessInvalid) {
		t.Fatalf("invalid readiness accepted: %+v", result)
	}
	wrongLifecycleKey := base
	wrongLifecycleKey.LifecyclePublicKey = make(ed25519.PublicKey, ed25519.PublicKeySize)
	if result := VerifyLifecycleConformance(wrongLifecycleKey); result.Valid || !containsString(result.ReasonCodes, ReasonConformanceVerification) {
		t.Fatalf("wrong lifecycle key accepted: %+v", result)
	}
	activationOnly := base
	activationOnly.LifecycleRecords = records[:5]
	if result := VerifyLifecycleConformance(activationOnly); result.Valid || !containsString(result.ReasonCodes, "terminal_execution_required") {
		t.Fatalf("activation-only lineage accepted: %+v", result)
	}
	successWithoutEffect := base
	successWithoutEffect.LifecycleRecords = records[:7]
	if result := VerifyLifecycleConformance(successWithoutEffect); result.Valid || !containsString(result.ReasonCodes, "validated_effect_required") {
		t.Fatalf("successful execution without effect/containment accepted: %+v", result)
	}
	if _, err := conformanceDigest(make(chan int)); err == nil {
		t.Fatal("unsupported conformance digest input accepted")
	}
	result := LifecycleConformanceResult{}
	conformanceReason(&result, ReasonConformanceInputMissing)
	conformanceReason(&result, ReasonConformanceInputMissing)
	if len(result.ReasonCodes) != 1 {
		t.Fatalf("duplicate conformance reason was not collapsed: %+v", result)
	}
	for _, expectation := range []LifecycleConformanceExpectation{
		{EffectOutcome: "recorded"}, {ContainmentOutcome: "partial"}, {CompensationOutcome: "completed"}, {RequireComplete: true},
	} {
		if err := checkConformanceExpectation(LifecycleSnapshot{}, expectation); err == nil {
			t.Fatalf("incomplete snapshot met expectation: %+v", expectation)
		}
	}
}
