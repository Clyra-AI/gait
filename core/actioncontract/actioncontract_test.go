package actioncontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtureArtifact(t *testing.T, scenario string) Artifact {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", scenario, "pac-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("fixture %s: %v (%v)", scenario, err, paths)
	}
	artifact, _, err := ReadArtifact(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func TestActivationIsDeterministicAndSigned(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	options := ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:security-owner", AuthorityRefs: []string{"approval:security-owner", "policy:gait://release-control"}, Target: "target:deploy-control", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", ValidUntil: "2026-07-20T00:00:00Z"}
	left, validation, err := Activate(proposal, options)
	if err != nil {
		t.Fatalf("activate: %v (%v)", err, validation.Reasons)
	}
	right, _, err := Activate(proposal, options)
	if err != nil {
		t.Fatal(err)
	}
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	if string(leftBytes) != string(rightBytes) {
		t.Fatalf("activation is not deterministic\nleft=%s\nright=%s", leftBytes, rightBytes)
	}
	valid, err := VerifyActivation(left, DevelopmentPublicKey())
	if err != nil || !valid {
		t.Fatalf("verify activation: valid=%v err=%v", valid, err)
	}
	left.PolicyDigest = "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if valid, _ := VerifyActivation(left, DevelopmentPublicKey()); valid {
		t.Fatal("tampered activation verified")
	}
}

func TestValidationRejectsTamperedDigestAndVersion(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	proposal.Contract["contract_content_digest"] = "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	result := ValidateArtifact(proposal, ValidationOptions{Now: mustTime("2026-07-19T00:00:00Z")})
	if result.Valid || !contains(result.Reasons, ReasonDigestMismatch) || !contains(result.Reasons, ReasonContractDigestMismatch) {
		t.Fatalf("tampered digest accepted: %+v", result)
	}
	proposal = fixtureArtifact(t, "customer-data-to-egress")
	proposal.SchemaVersion = "2"
	result = ValidateArtifact(proposal, ValidationOptions{})
	if result.Valid || !contains(result.Reasons, ReasonUnsupportedArtifactSchema) {
		t.Fatalf("invalid schema accepted: %+v", result)
	}
}

func TestReadArtifactNeverScansRecommendations(t *testing.T) {
	if _, _, err := ReadArtifact(""); err == nil {
		t.Fatal("empty path accepted")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected")); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
