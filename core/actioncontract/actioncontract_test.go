package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proofsign "github.com/Clyra-AI/proof/signing"
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

func fixtureSelection(t *testing.T, scenario string, artifact Artifact) *SelectionEvidence {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", scenario, "pac-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("fixture %s: %v (%v)", scenario, err, paths)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return &SelectionEvidence{ArtifactID: artifact.ArtifactID, ArtifactSHA256: RawDigest(raw), CanonicalContentDigest: artifact.CanonicalContentDigest, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID, Revision: artifact.Revision, Current: true}
}

func TestActivationIsDeterministicAndSigned(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	options := ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:security-owner", AuthorityRefs: []string{"approval:security-owner", "policy:gait://release-control"}, Target: "target:deploy-control", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", ValidUntil: "2026-07-20T00:00:00Z", Selection: fixtureSelection(t, "customer-data-to-egress", proposal)}
	keyPair, err := proofsign.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	options.SigningPrivateKey = keyPair.Private
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
	valid, err := VerifyActivation(left, keyPair.Public)
	if err != nil || !valid {
		t.Fatalf("verify activation: valid=%v err=%v", valid, err)
	}
	malformed := left
	malformed.Target = ""
	if valid, err := VerifyActivation(malformed, keyPair.Public); valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonSchemaValidationFailed) {
		t.Fatalf("activated schema drift must fail before signature verification: valid=%v err=%v", valid, err)
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

func TestDuplicateJSONKeysAndNestedSchemaDriftReject(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte(`"report_only": true`)
	index := strings.Index(string(raw), string(marker))
	if index < 0 {
		t.Fatal("fixture report_only marker missing")
	}
	duplicate := append([]byte(nil), raw[:index+len(marker)]...)
	duplicate = append(duplicate, []byte(`, "report_only": true`)...)
	duplicate = append(duplicate, raw[index+len(marker):]...)
	if _, err := ParseArtifact(duplicate); err == nil {
		t.Fatal("duplicate JSON keys must be rejected")
	}
	nested := bytesReplace(raw, []byte(`"minimum_approvals": 2`), []byte(`"minimum_approvals": "2"`))
	_, result := ValidateArtifactBytes(nested, ValidationOptions{})
	if result.Valid || !contains(result.Reasons, ReasonSchemaValidationFailed) {
		t.Fatalf("nested schema drift must fail schema validation: %+v", result)
	}
	unknown := bytesReplace(raw, []byte(`"minimum_approvals": 2`), []byte(`"minimum_approvals": 2, "unknown_field": "x"`))
	_, result = ValidateArtifactBytes(unknown, ValidationOptions{})
	if result.Valid || !contains(result.Reasons, ReasonSchemaValidationFailed) {
		t.Fatalf("unknown nested fields must fail schema validation: %+v", result)
	}
}

func TestSelectionManifestAndStaleRevisionFailClosed(t *testing.T) {
	artifactPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	manifestPath := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "fixture-manifest.json")
	artifact, raw, err := ReadArtifact(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := LoadSelectionEvidence(manifestPath, artifactPath, artifact, raw)
	if err != nil || !selection.Current {
		t.Fatalf("local fixture selection must validate: %v %+v", err, selection)
	}
	selection.Current = false
	if err := validateSelectionEvidence(selection, artifact, raw); err == nil || !contains(err.(*ValidationError).Reasons, ReasonSelectionNotCurrent) {
		t.Fatalf("stale selection must fail closed: %v", err)
	}
}

func TestActivationRequiresKeyAndDevelopmentModeIsMarked(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	options := ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", Selection: fixtureSelection(t, "customer-data-to-egress", proposal)}
	if _, _, err := Activate(proposal, options); err == nil || !contains(err.(*ValidationError).Reasons, ReasonSigningKeyRequired) {
		t.Fatalf("missing signing key must fail: %v", err)
	}
	options.AllowDevelopmentSigning = true
	if _, _, err := Activate(proposal, options); err == nil || !contains(err.(*ValidationError).Reasons, ReasonDevelopmentSigningForbidden) {
		t.Fatalf("development signing must be forbidden for production: %v", err)
	}
	options.Environment = "development"
	activated, _, err := Activate(proposal, options)
	if err != nil || !activated.DevelopmentSigning {
		t.Fatalf("explicit development signing must be marked: %v %+v", err, activated)
	}
	if valid, err := VerifyActivation(activated, DevelopmentPublicKey()); valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonDevelopmentSigningUnverified) {
		t.Fatalf("development signing must not verify by default: valid=%v err=%v", valid, err)
	}
	if valid, err := VerifyActivationWithOptions(activated, DevelopmentPublicKey(), VerificationOptions{AllowDevelopmentSigning: true, Proposal: &proposal}); err != nil || !valid {
		t.Fatalf("explicit development verification should work: valid=%v err=%v", valid, err)
	}
}

func TestActivationRejectsOffsetValidityInversionAndWrongProposalBinding(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	options := ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00-05:00", ValidUntil: "2026-07-19T01:00:00+05:00", SigningPrivateKey: mustKey(t), Selection: fixtureSelection(t, "customer-data-to-egress", proposal)}
	if _, _, err := Activate(proposal, options); err == nil || !contains(err.(*ValidationError).Reasons, ReasonValidityInvalid) {
		t.Fatalf("offset-inverted validity must fail: %v", err)
	}
	options.ValidFrom, options.ValidUntil = "2026-07-19T00:00:00Z", "2026-07-20T00:00:00Z"
	activated, _, err := Activate(proposal, options)
	if err != nil {
		t.Fatal(err)
	}
	other := fixtureArtifact(t, "workflow-to-deploy")
	if valid, err := VerifyActivationWithOptions(activated, options.SigningPrivateKey.Public().(ed25519.PublicKey), VerificationOptions{Proposal: &other}); valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonBindingMismatch) {
		t.Fatalf("wrong bound proposal must fail: valid=%v err=%v", valid, err)
	}
}

func mustKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func bytesReplace(input, old, replacement []byte) []byte {
	return []byte(strings.Replace(string(input), string(old), string(replacement), 1))
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
