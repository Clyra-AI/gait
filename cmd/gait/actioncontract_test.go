package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
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

func captureActionContractOutput(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	code := fn()
	_ = writer.Close()
	os.Stdout = original
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	return string(payload), code
}
