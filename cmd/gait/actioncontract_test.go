package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func TestActionContractVerifyCLIAndExitCodes(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	proposal, raw, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := proofsign.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	selection := &actioncontract.SelectionEvidence{ArtifactID: proposal.ArtifactID, ArtifactSHA256: actioncontract.RawDigest(raw), CanonicalContentDigest: proposal.CanonicalContentDigest, ContractID: proposal.ContractID, ContractFamilyID: proposal.ContractFamilyID, Revision: proposal.Revision, Current: true}
	activated, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: actioncontract.ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", SigningPrivateKey: keyPair.Private, Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	activationPath := filepath.Join(directory, "activated.json")
	encoded, err := json.Marshal(activated)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(activationPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(directory, "public.key")
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractVerify([]string{"--activation", activationPath, "--proposal", proposalPath, "--public-key", publicPath, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("verify CLI should pass: code=%d output=%s", code, output)
	}
	_, code = captureActionContractOutput(t, func() int {
		return runActionContractVerify([]string{"--activation", activationPath, "--proposal", proposalPath, "--public-key", publicPath, "--evaluation-time", "not-a-time", "--json"})
	})
	if code != exitInvalidInput {
		t.Fatalf("malformed verification evaluation time must be invalid input: %d", code)
	}
	wrongPublic, wrongPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = wrongPrivate
	wrongPath := filepath.Join(directory, "wrong.key")
	if err := os.WriteFile(wrongPath, []byte(base64.StdEncoding.EncodeToString(wrongPublic)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, code = captureActionContractOutput(t, func() int {
		return runActionContractVerify([]string{"--activation", activationPath, "--proposal", proposalPath, "--public-key", wrongPath, "--json"})
	})
	if code != exitVerifyFailed {
		t.Fatalf("wrong public key must use verification-failure exit code: %d", code)
	}
	_, code = captureActionContractOutput(t, func() int {
		return runActionContractVerify([]string{"--activation", activationPath, "--proposal", proposalPath, "--json"})
	})
	if code != exitInvalidInput {
		t.Fatalf("missing public key must use invalid-input exit code: %d", code)
	}
}

func TestActionContractHelpParity(t *testing.T) {
	output, code := captureActionContractOutput(t, func() int { return runActionContract([]string{"--help"}) })
	if code != exitOK {
		t.Fatalf("contract help exit=%d output=%s", code, output)
	}
	for _, line := range []string{"gait contract validate", "gait contract activate", "gait contract verify", "gait contract consume"} {
		if !strings.Contains(output, line) {
			t.Fatalf("contract help missing %q: %s", line, output)
		}
	}
}

func TestActionContractValidateRejectsMalformedEvaluationTime(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractValidate([]string{"--proposal", proposalPath, "--evaluation-time", "not-a-time", "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, actioncontract.ReasonEvaluationTimeInvalid) {
		t.Fatalf("malformed evaluation time must fail as invalid input: code=%d output=%s", code, output)
	}
}

func TestActionContractValidateAndConsumeSuccess(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	selectionPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractValidate([]string{"--proposal", proposalPath, "--evaluation-time", "2026-07-19T00:00:00Z", "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"operation":"validate"`) || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("valid proposal validation should pass: code=%d output=%s", code, output)
	}

	output, code = captureActionContractOutput(t, func() int {
		return runActionContractConsume([]string{proposalPath, "--selection", selectionPath, "--scenario-id", "customer-data-to-egress"})
	})
	if code != exitOK || !strings.Contains(output, `"status":"pass"`) || !strings.Contains(output, `"self_attestation":false`) {
		t.Fatalf("valid proposal consumer should pass: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return runActionContractConsume([]string{"--artifact", proposalPath, "--selection", selectionPath})
	})
	if code != exitOK || !strings.Contains(output, `"status":"pass"`) || !strings.Contains(output, `"scenario_id":"customer-data-to-egress"`) {
		t.Fatalf("inferred scenario consumer should pass: code=%d output=%s", code, output)
	}
	output, code = captureActionContractOutput(t, func() int {
		return runActionContractConsume([]string{"--artifact", proposalPath, "--selection", filepath.Join(t.TempDir(), "missing.json")})
	})
	if code != exitVerifyFailed || !strings.Contains(output, actioncontract.ReasonSelectionEvidenceRequired) {
		t.Fatalf("invalid consumer selection must fail verification: code=%d output=%s", code, output)
	}
}

func TestActionContractKeyAndHelperBranches(t *testing.T) {
	keyPair, err := proofsign.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	privatePath := filepath.Join(directory, "private.key")
	publicPath := filepath.Join(directory, "public.key")
	if err := os.WriteFile(privatePath, []byte(base64.StdEncoding.EncodeToString(keyPair.Private)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatal(err)
	}
	if privateKey, err := loadActionContractPrivateKey(privatePath, ""); err != nil || len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key file branch: len=%d err=%v", len(privateKey), err)
	}
	if publicKey, err := loadActionContractPublicKey(publicPath, ""); err != nil || len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key file branch: len=%d err=%v", len(publicKey), err)
	}
	privateEnv, publicEnv := "GAIT_TEST_PRIVATE_KEY", "GAIT_TEST_PUBLIC_KEY"
	t.Setenv(privateEnv, base64.StdEncoding.EncodeToString(keyPair.Private))
	t.Setenv(publicEnv, base64.StdEncoding.EncodeToString(keyPair.Public))
	if privateKey, err := loadActionContractPrivateKey("", privateEnv); err != nil || len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key environment branch: len=%d err=%v", len(privateKey), err)
	}
	if publicKey, err := loadActionContractPublicKey("", publicEnv); err != nil || len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key environment branch: len=%d err=%v", len(publicKey), err)
	}
	for _, test := range []struct {
		name string
		fn   func() error
	}{
		{name: "private both", fn: func() error { _, err := loadActionContractPrivateKey(privatePath, privateEnv); return err }},
		{name: "public both", fn: func() error { _, err := loadActionContractPublicKey(publicPath, publicEnv); return err }},
		{name: "private missing env", fn: func() error { _, err := loadActionContractPrivateKey("", "GAIT_MISSING_PRIVATE"); return err }},
		{name: "public missing env", fn: func() error { _, err := loadActionContractPublicKey("", "GAIT_MISSING_PUBLIC"); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(); err == nil {
				t.Fatal("invalid key source accepted")
			}
		})
	}
	if got := parseActionContractCSV(" one, ,two,, three "); strings.Join(got, "|") != "one|two|three" {
		t.Fatalf("CSV normalization: %v", got)
	}
	if _, err := parseEvaluationTime("2026-07-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseEvaluationTime("not-a-time"); err == nil {
		t.Fatal("invalid evaluation time accepted")
	}
	if got := errorString(nil, "fallback"); got != "fallback" {
		t.Fatalf("nil error fallback: %q", got)
	}
	if got := errorString(errors.New("boom"), "fallback"); got != "boom" {
		t.Fatalf("error string: %q", got)
	}
}

func TestActionContractOutputAndSelectorHelpers(t *testing.T) {
	output, code := captureActionContractOutput(t, func() int {
		return writeActionContractOutput(false, actionContractOutput{Operation: "validate", OK: true}, exitOK)
	})
	if code != exitOK || !strings.Contains(output, "contract validate: ok=true") {
		t.Fatalf("text output branch: code=%d output=%q", code, output)
	}
	receipt := normalizeActionContractReceipt(actionContractReceipt{})
	if receipt.Consumer != "gait" || receipt.Version == "" || receipt.ScenarioID != "unknown" || receipt.ArtifactSHA256 == "" || receipt.SchemaVersions.Artifact == "" || receipt.SchemaVersions.Contract == "" || receipt.SelfAttestation {
		t.Fatalf("receipt defaults not normalized: %+v", receipt)
	}
	validationErr := &actioncontract.ValidationError{Reasons: []string{"one", "two"}}
	if got := actionContractReasonCodes(validationErr); strings.Join(got, "|") != "one|two" {
		t.Fatalf("validation reason extraction: %v", got)
	}
	if got := actionContractReasonCodes(errors.New("ordinary")); got != nil {
		t.Fatalf("ordinary errors should not have reason codes: %v", got)
	}
	if got := actionContractNamedFlagCount([]string{"--activation=x", "--activation", "--proposal", "--json"}, "--activation", "--proposal"); got != 3 {
		t.Fatalf("named flag count: %d", got)
	}
	if got := actionContractSelectionFlagCount([]string{"--proposal=x", "--artifact=y", "--from=z"}); got != 3 {
		t.Fatalf("selection flag count: %d", got)
	}
	for _, test := range []struct {
		name string
		fn   func() int
		want string
	}{
		{name: "validate help", fn: func() int { return runActionContractValidate([]string{"--help"}) }, want: "Usage: gait contract validate"},
		{name: "verify help", fn: func() int { return runActionContractVerify([]string{"--help"}) }, want: "Usage: gait contract verify"},
		{name: "consume help", fn: func() int { return runActionContractConsume([]string{"--help"}) }, want: "Usage: gait contract consume"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, gotCode := captureActionContractOutput(t, test.fn)
			if gotCode != exitOK || !strings.Contains(got, test.want) {
				t.Fatalf("help branch code=%d output=%s", gotCode, got)
			}
		})
	}
}

func TestActionContractRejectsInvalidCLISelections(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	cases := []struct {
		name string
		fn   func() int
		code int
		want string
	}{
		{name: "validate parser", fn: func() int { return runActionContractValidate([]string{"--unknown"}) }, code: exitInvalidInput, want: "contract validate error"},
		{name: "validate ambiguous", fn: func() int {
			return runActionContractValidate([]string{"--proposal", proposalPath, "--artifact", proposalPath, "--json"})
		}, code: exitInvalidInput, want: actioncontract.ReasonAmbiguousSelection},
		{name: "validate missing", fn: func() int { return runActionContractValidate([]string{"--json"}) }, code: exitInvalidInput, want: actioncontract.ReasonSelectionRequired},
		{name: "validate read", fn: func() int { return runActionContractValidate([]string{"--proposal", missingPath, "--json"}) }, code: exitInvalidInput, want: `"operation":"validate"`},
		{name: "verify parser", fn: func() int { return runActionContractVerify([]string{"--unknown"}) }, code: exitInvalidInput, want: "contract verify error"},
		{name: "verify missing", fn: func() int { return runActionContractVerify([]string{"--json"}) }, code: exitInvalidInput, want: actioncontract.ReasonSelectionRequired},
		{name: "consume parser", fn: func() int { return runActionContractConsume([]string{"--unknown"}) }, code: exitInvalidInput, want: actioncontract.ReasonMalformedArtifact},
		{name: "consume ambiguous", fn: func() int { return runActionContractConsume([]string{"--artifact", proposalPath, proposalPath}) }, code: exitInvalidInput, want: actioncontract.ReasonAmbiguousSelection},
		{name: "consume read", fn: func() int { return runActionContractConsume([]string{"--artifact", missingPath}) }, code: exitVerifyFailed, want: `"status":"reject"`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			output, code := captureActionContractOutput(t, test.fn)
			if code != test.code || !strings.Contains(output, test.want) {
				t.Fatalf("invalid CLI branch code=%d output=%s", code, output)
			}
		})
	}
	output, code := captureActionContractOutput(t, func() int { return runActionContract([]string{"--explain"}) })
	if code != exitOK || !strings.Contains(output, "explicitly selected Wrkr") {
		t.Fatalf("explain branch code=%d output=%s", code, output)
	}
}

func TestActionContractRejectReceiptHasRequiredFields(t *testing.T) {
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractConsume([]string{"--json"})
	})
	if code != exitInvalidInput {
		t.Fatalf("missing consumer artifact should be invalid input: %d", code)
	}
	var receipt struct {
		ScenarioID     string `json:"scenario_id"`
		ArtifactSHA256 string `json:"artifact_sha256"`
		SchemaVersions struct {
			Artifact string `json:"artifact"`
			Contract string `json:"contract"`
		} `json:"schema_versions"`
		SemanticResult struct {
			ExecutionClaim bool `json:"execution_claim"`
			EffectClaim    bool `json:"effect_claim"`
		} `json:"semantic_result"`
	}
	if err := json.Unmarshal([]byte(output), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.ScenarioID == "" || receipt.ArtifactSHA256 == "" || receipt.SchemaVersions.Artifact == "" || receipt.SchemaVersions.Contract == "" || receipt.SemanticResult.ExecutionClaim || receipt.SemanticResult.EffectClaim {
		t.Fatalf("reject receipt is not schema-valid: %s", output)
	}
}

func TestActionContractVerifyRejectsConflictingActivationSelectors(t *testing.T) {
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractVerify([]string{"--activation", "one.json", "--artifact", "two.json", "--proposal", "proposal.json", "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, actioncontract.ReasonAmbiguousSelection) {
		t.Fatalf("conflicting activation selectors must fail as invalid input: code=%d output=%s", code, output)
	}
}

func TestActionContractActivateAndDispatchBranches(t *testing.T) {
	proposalPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	selectionPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	base := []string{
		"--proposal", proposalPath,
		"--selection", selectionPath,
		"--policy-digest", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"--principal", "principal:owner",
		"--authority-ref", "approval:owner",
		"--target", "target:deploy",
		"--environment", "development",
		"--mode", "context_only",
		"--valid-from", "2026-07-19T00:00:00Z",
		"--allow-development-signing",
		"--json",
	}
	output, code := captureActionContractOutput(t, func() int {
		return runActionContractActivate(base)
	})
	if code != exitOK || !strings.Contains(output, `"operation":"activate"`) || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("valid activation CLI should pass: code=%d output=%s", code, output)
	}

	for _, test := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{name: "help", args: []string{"--help"}, code: exitOK, want: "Usage: gait contract activate"},
		{name: "ambiguous proposal", args: []string{"--proposal", proposalPath, "--artifact", proposalPath, "--json"}, code: exitInvalidInput, want: actioncontract.ReasonAmbiguousSelection},
		{name: "missing proposal", args: []string{"--json"}, code: exitInvalidInput, want: actioncontract.ReasonSelectionRequired},
		{name: "missing selection", args: []string{"--proposal", proposalPath, "--json"}, code: exitVerifyFailed, want: actioncontract.ReasonSelectionEvidenceRequired},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, gotCode := captureActionContractOutput(t, func() int {
				return runActionContractActivate(test.args)
			})
			if gotCode != test.code || !strings.Contains(got, test.want) {
				t.Fatalf("activation branch code=%d output=%s", gotCode, got)
			}
		})
	}

	for _, test := range [][]string{{}, {"unknown"}} {
		output, code := captureActionContractOutput(t, func() int { return runActionContract(test) })
		if code != exitInvalidInput || !strings.Contains(output, "gait contract validate") {
			t.Fatalf("dispatch branch code=%d output=%s", code, output)
		}
	}
}

func captureActionContractOutput(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	resultCh := make(chan struct {
		payload []byte
		err     error
	}, 1)
	go func() {
		payload, readErr := io.ReadAll(reader)
		resultCh <- struct {
			payload []byte
			err     error
		}{payload: payload, err: readErr}
	}()
	code := fn()
	_ = writer.Close()
	os.Stdout = original
	result := <-resultCh
	if result.err != nil {
		t.Fatal(result.err)
	}
	_ = reader.Close()
	return string(result.payload), code
}
