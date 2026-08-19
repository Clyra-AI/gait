package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/actioncontract"
)

func TestRuntimeClassifyCLI(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actionPath := filepath.Join(root, "action.json")
	action := actioncontract.ClassifyAction(actioncontract.ClassificationInput{ActionID: "cli-action", ActionClass: "read", CompositionRole: "source", TargetTrustClass: "external", TransitionClass: "read", ExpectedOutcomeClass: "read", TargetRef: "target:cli"}).Action
	actionRaw, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actionPath, actionRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	unknownPath := filepath.Join(root, "unknown.json")
	if err := os.WriteFile(unknownPath, []byte(`{"action_id":"cli-action","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(root, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"schema_id":"https://gait.dev/schemas/v1/runtime-action.schema.json","schema_version":"1","action_id":"cli-action","action_class":"bogus","composition_role":"source","target_trust_class":"external","transition_class":"read","expected_outcome_class":"read","boundary":{"source_trust_class":"","target_trust_class":"external","transition_class":"read"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "input.json")
	if err := os.WriteFile(inputPath, []byte(`{"schema_id":"https://gait.dev/schemas/v1/runtime-classification-input.schema.json","schema_version":"1","action_id":"cli-action","action_class":"bogus","composition_role":"source","target_trust_class":"external","transition_class":"read","expected_outcome_class":"read"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	goodInputPath := filepath.Join(root, "good-input.json")
	if err := os.WriteFile(goodInputPath, []byte(`{"schema_id":"https://gait.dev/schemas/v1/runtime-classification-input.schema.json","schema_version":"1","action_id":"cli-input","action_class":"read","composition_role":"source","target_trust_class":"external","transition_class":"read","expected_outcome_class":"read"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{name: "action json", args: []string{"--action", actionPath, "--json"}, code: exitOK, want: `"operation":"classify"`},
		{name: "action text", args: []string{"--action", actionPath}, code: exitOK, want: "contract classify: ok=true"},
		{name: "raw input", args: []string{"--input", goodInputPath, "--json"}, code: exitOK, want: `"action_id":"cli-input"`},
		{name: "proposal", args: []string{"--proposal", proposalPath, "--json"}, code: exitOK, want: `"expected_outcome_class":"data_egress"`},
		{name: "help", args: []string{"--help"}, code: exitOK, want: "Usage: gait contract classify"},
		{name: "missing selector", args: []string{"--json"}, code: exitInvalidInput, want: actioncontract.ReasonSelectionRequired},
		{name: "parser error", args: []string{"--unknown"}, code: exitInvalidInput, want: "classify error"},
		{name: "unknown field", args: []string{"--action", unknownPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonMalformedArtifact},
		{name: "invalid schema", args: []string{"--action", invalidPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonMalformedArtifact},
		{name: "invalid raw input", args: []string{"--input", inputPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonMalformedArtifact},
		{name: "unreadable", args: []string{"--action", filepath.Join(root, "missing.json"), "--json"}, code: exitInvalidInput, want: "runtime_input_unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, code := captureActionContractOutput(t, func() int { return runActionContractClassify(tc.args) })
			if code != tc.code || !strings.Contains(output, tc.want) {
				t.Fatalf("classify code=%d output=%s", code, output)
			}
		})
	}
}

func TestRuntimeExplainCLI(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actionPath := filepath.Join(root, "action.json")
	action := actioncontract.ClassifyAction(actioncontract.ClassificationInput{ActionID: "explain-action", ActionClass: "read", CompositionRole: "source", TargetTrustClass: "external", TransitionClass: "read", ExpectedOutcomeClass: "read"}).Action
	actionRaw, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actionPath, actionRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(root, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"action_id":"bad","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{name: "default text", args: nil, code: exitOK, want: "pre-execution"},
		{name: "default json", args: []string{"--json"}, code: exitOK, want: `"operation":"explain"`},
		{name: "parser error", args: []string{"--unknown"}, code: exitInvalidInput, want: "explain error"},
		{name: "action json", args: []string{"--action", actionPath, "--json"}, code: exitOK, want: `"operation":"explain"`},
		{name: "action text", args: []string{"--action", actionPath}, code: exitOK, want: "contract explain: ok=true"},
		{name: "ambiguous inputs", args: []string{"--action", actionPath, "--input", actionPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonAmbiguousSelection},
		{name: "help", args: []string{"--help"}, code: exitOK, want: "Usage: gait contract explain"},
		{name: "bad action", args: []string{"--action", badPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonMalformedArtifact},
		{name: "unreadable", args: []string{"--action", filepath.Join(root, "missing.json"), "--json"}, code: exitInvalidInput, want: "runtime_input_unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, code := captureActionContractOutput(t, func() int { return runActionContractExplain(tc.args) })
			if code != tc.code || !strings.Contains(output, tc.want) {
				t.Fatalf("explain code=%d output=%s", code, output)
			}
		})
	}
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	output, code := captureActionContractOutput(t, func() int { return runActionContractExplain([]string{"--proposal", proposalPath, "--json"}) })
	if code != exitVerifyFailed || !strings.Contains(output, `"operation":"explain"`) || !strings.Contains(output, `"readiness"`) {
		t.Fatalf("proposal explanation should expose fail-closed readiness: code=%d output=%s", code, output)
	}
}

func TestRuntimeReadinessCLISuccessAndErrors(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(root, "validator.pub.b64")
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(public)), 0o600); err != nil {
		t.Fatal(err)
	}
	item := actioncontract.ReadinessPrecondition{RequirementID: "p1", Kind: "environment", Required: true, Producer: "validator", ControlMode: actioncontract.ControlModeEnforced, FreshnessState: "fresh", ObservedResult: "pass", ObservedAt: "2026-07-19T00:30:00Z", MaxAgeSeconds: 3600, EvidenceState: "verified", EvidenceRefs: []string{"evidence:p1"}, Environment: "production"}
	policyDigest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	claimInput := actioncontract.ReadinessInput{ContractID: "cli-contract", PolicyDigest: policyDigest}
	digest, err := actioncontract.CanonicalReadinessClaimDigest(claimInput, item)
	if err != nil {
		t.Fatal(err)
	}
	item.EvidenceDigest = digest
	item.ValidatorSignature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, mustDecodeDigest(t, digest)))
	inputPath := filepath.Join(root, "readiness.json")
	input, err := json.Marshal(actioncontract.ReadinessInput{ContractID: "cli-contract", PolicyDigest: policyDigest, Preconditions: []actioncontract.ReadinessPrecondition{item}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	validArgs := []string{"--input", inputPath, "--trusted-validators", "validator", "--trusted-validator-key", "validator=" + publicPath, "--policy-digest", policyDigest, "--evaluation-time", "2026-07-19T01:00:00Z", "--json"}
	output, code := captureActionContractOutput(t, func() int { return runActionContractReadiness(validArgs) })
	if code != exitOK || !strings.Contains(output, `"ready":true`) || !strings.Contains(output, `"status":"satisfied"`) {
		t.Fatalf("valid readiness should pass: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int { return runActionContractReadiness(append([]string(nil), validArgs[:len(validArgs)-1]...)) })
	if code != exitOK || !strings.Contains(output, "contract readiness: ok=true") {
		t.Fatalf("text readiness should pass: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		args := append([]string(nil), validArgs...)
		args[9] = "not-json"
		return runActionContractReadiness(args)
	})
	if code != exitInvalidInput || !strings.Contains(output, actioncontract.ReasonEvaluationTimeInvalid) {
		t.Fatalf("invalid evaluation time should fail closed: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		args := append([]string(nil), validArgs...)
		args[5] = "validator=" + filepath.Join(root, "missing.pub")
		return runActionContractReadiness(args)
	})
	if code != exitInvalidInput || !strings.Contains(output, "validator_key_invalid") {
		t.Fatalf("missing validator key should be invalid input: code=%d output=%s", code, output)
	}
	for _, value := range []string{"", "validator", "validator="} {
		if _, err := loadTrustedValidatorKeys([]string{value}); err == nil {
			t.Fatalf("invalid validator key selector accepted: %q", value)
		}
	}
	var repeat repeatStringFlag
	if err := repeat.Set("one"); err != nil {
		t.Fatal(err)
	}
	if err := repeat.Set("two"); err != nil || repeat.String() != "one,two" {
		t.Fatalf("repeat flag normalization: %q", repeat.String())
	}
	if normalizeRuntimeKey(" Validator ") != "validator" {
		t.Fatal("validator key producer was not normalized")
	}
}

func TestRuntimeReadinessCLIHelpAndSelectionErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{name: "help", args: []string{"--help"}, code: exitOK, want: "Usage: gait contract readiness"},
		{name: "missing selector", args: []string{"--evaluation-time", "2026-07-19T01:00:00Z", "--json"}, code: exitInvalidInput, want: actioncontract.ReasonAmbiguousSelection},
		{name: "invalid key shape", args: []string{"--evaluation-time", "2026-07-19T01:00:00Z", "--input", "missing.json", "--trusted-validator-key", "bad", "--json"}, code: exitInvalidInput, want: "validator_key_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, code := captureActionContractOutput(t, func() int { return runActionContractReadiness(tc.args) })
			if code != tc.code || !strings.Contains(output, tc.want) {
				t.Fatalf("readiness code=%d output=%s", code, output)
			}
		})
	}
}

func TestRuntimeOutputAndReasonHelpers(t *testing.T) {
	if got := sortedRuntimeReasonCodes([]string{"b", "", "a", "b"}); strings.Join(got, "|") != "a|b" {
		t.Fatalf("reason code sorting: %v", got)
	}
	output, code := captureActionContractOutput(t, func() int {
		return writeRuntimeOutput(false, actionContractRuntimeOutput{Operation: "readiness", OK: true}, exitOK)
	})
	if code != exitOK || !strings.Contains(output, "contract readiness: ok=true") {
		t.Fatalf("text output: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return writeRuntimeOutput(false, actionContractRuntimeOutput{Operation: "readiness", Error: "bad input"}, exitInvalidInput)
	})
	if code != exitInvalidInput || !strings.Contains(output, "contract readiness error: bad input") {
		t.Fatalf("text error output: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return writeRuntimeOutput(true, actionContractRuntimeOutput{Operation: "readiness", OK: true}, exitOK)
	})
	if code != exitOK || !strings.Contains(output, `"operation":"readiness"`) {
		t.Fatalf("JSON output: code=%d output=%s", code, output)
	}
}

func mustDecodeDigest(t *testing.T, digest string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestRuntimeReadinessCLIRequiresEvaluationTimeAndAuthoritativeKey(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractReadiness([]string{"--proposal", proposalPath, "--trusted-validators", "validator", "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, actioncontract.ReasonEvaluationTimeInvalid) {
		t.Fatalf("missing evaluation time must be invalid input: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return runActionContractReadiness([]string{"--proposal", proposalPath, "--trusted-validators", "validator", "--evaluation-time", "2026-07-19T01:00:00Z", "--json"})
	})
	if code != exitVerifyFailed || !strings.Contains(output, "validator") {
		t.Fatalf("missing authoritative validator key must remain fail-closed: code=%d output=%s", code, output)
	}
	inputRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(inputRoot, "empty-readiness.json")
	if err := os.WriteFile(inputPath, []byte(`{"preconditions":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	output, code = captureActionContractOutput(t, func() int {
		return runActionContractReadiness([]string{"--input", inputPath, "--evaluation-time", "2026-07-19T01:00:00Z", "--json"})
	})
	if code != exitVerifyFailed || !strings.Contains(output, `"ready":false`) || !strings.Contains(output, `"status":"not_required"`) {
		t.Fatalf("empty readiness input became authoritative: code=%d output=%s", code, output)
	}
}
