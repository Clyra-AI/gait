package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	valid, err := VerifyActivation(left, keyPair.Public, proposal)
	if err != nil || !valid {
		t.Fatalf("verify activation: valid=%v err=%v", valid, err)
	}
	malformed := left
	malformed.Target = ""
	if valid, err := VerifyActivation(malformed, keyPair.Public, proposal); valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonSchemaValidationFailed) {
		t.Fatalf("activated schema drift must fail before signature verification: valid=%v err=%v", valid, err)
	}
	left.PolicyDigest = "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if valid, _ := VerifyActivation(left, DevelopmentPublicKey(), proposal); valid {
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
	if valid, err := VerifyActivation(activated, DevelopmentPublicKey(), proposal); valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonDevelopmentSigningUnverified) {
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

func TestEmbeddedSchemasRemainAlignedWithCheckedInSchemas(t *testing.T) {
	for _, name := range []string{"activated-action-contract-artifact.schema.json", "proposed-action-contract-artifact.schema.json", "proposed-action-contract-v3.schema.json"} {
		embedded, err := schemaAssets.ReadFile("schemaassets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		checkedIn, err := os.ReadFile(filepath.Join("..", "..", "schemas", "v1", "action-contract", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(embedded) != string(checkedIn) {
			t.Fatalf("embedded schema drift: %s", name)
		}
	}
}

func TestEmbeddedSchemaValidationIsIndependentOfWorkingDirectory(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "pac-6dcee5a6d9a65e8c.json")
	abs, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	shadow := filepath.Join(t.TempDir(), "schemas", "v1", "action-contract")
	if err := os.MkdirAll(shadow, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shadow, "proposed-action-contract-artifact.schema.json"), []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Dir(shadow))
	_, result := ValidateArtifactBytes(raw, ValidationOptions{})
	if contains(result.Reasons, ReasonSchemaValidationFailed) {
		t.Fatalf("embedded validation was shadowed by CWD schema: %+v", result)
	}
}

func TestVerifyActivationRejectsInvalidPublicKeyLength(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	options := ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", SigningPrivateKey: mustKey(t), Selection: fixtureSelection(t, "customer-data-to-egress", proposal)}
	activated, _, err := Activate(proposal, options)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := VerifyActivationWithOptions(activated, ed25519.PublicKey{1}, VerificationOptions{Proposal: &proposal})
	if valid || err == nil || !contains(err.(*ValidationError).Reasons, ReasonSigningKeyRequired) {
		t.Fatalf("invalid key length must fail safely: valid=%v err=%v", valid, err)
	}
}

func TestSelectionEvidenceRejectsSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	scenarioDir := filepath.Join(root, "scenario")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := fixturePath(t, "customer-data-to-egress")
	artifact, raw, err := ReadArtifact(fixture)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "proposal.json")
	if err := os.WriteFile(outside, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(scenarioDir, "proposal.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	manifest := selectionManifest{FixtureVersion: "1"}
	manifest.Producer.Name, manifest.Producer.Version = ProposedProducer, "v1.14.0"
	manifest.Schemas.Artifact, manifest.Schemas.Contract = "1", "3"
	manifest.Scenarios = []struct {
		ScenarioID             string `json:"scenario_id"`
		ArtifactPath           string `json:"artifact_path"`
		ArtifactSHA256         string `json:"artifact_sha256"`
		ArtifactID             string `json:"artifact_id"`
		CanonicalContentDigest string `json:"canonical_content_digest"`
		ContractID             string `json:"contract_id"`
		ContractFamilyID       string `json:"contract_family_id"`
		Revision               int    `json:"revision"`
		Current                bool   `json:"current"`
	}{{ScenarioID: "scenario", ArtifactPath: "scenario/proposal.json", ArtifactSHA256: RawDigest(raw), ArtifactID: artifact.ArtifactID, CanonicalContentDigest: artifact.CanonicalContentDigest, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID, Revision: artifact.Revision, Current: true}}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "fixture-manifest.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadSelectionEvidence(manifestPath, link, artifact, raw)
	if err == nil || !contains(err.(*ValidationError).Reasons, ReasonSelectionMismatch) {
		t.Fatalf("outside symlink must fail: %v", err)
	}
}

func TestWriteActivatedArtifactRefusesUnsafeTargets(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	activated, _, err := Activate(proposal, ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", SigningPrivateKey: mustKey(t), Selection: fixtureSelection(t, "customer-data-to-egress", proposal)})
	if err != nil {
		t.Fatal(err)
	}
	dir := outputWriterTempDir(t)
	target := filepath.Join(dir, "activated.json")
	if err := WriteActivatedArtifact(target, activated, false); err != nil {
		t.Fatal(err)
	}
	if err := WriteActivatedArtifact(target, activated, false); err == nil {
		t.Fatal("existing output must be refused")
	}
	if err := WriteActivatedArtifact(target, activated, true); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	nestedTarget := filepath.Join(nested, "activated.json")
	if err := WriteActivatedArtifact(nestedTarget, activated, false); err != nil {
		t.Fatalf("normal nested directory must remain usable: %v", err)
	}
	if _, err := os.Stat(nestedTarget); err != nil {
		t.Fatalf("nested activation output missing: %v", err)
	}
	nonRegular := filepath.Join(dir, "non-regular")
	if err := os.Mkdir(nonRegular, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteActivatedArtifact(nonRegular, activated, true); err == nil {
		t.Fatal("non-regular output must be refused")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteActivatedArtifact(link, activated, true); err == nil {
		t.Fatal("symlink output must be refused")
	}
	realParent := filepath.Join(dir, "real-parent", "sub")
	if err := os.MkdirAll(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	aliasParent := filepath.Join(dir, "alias-parent")
	if err := os.Symlink(filepath.Dir(realParent), aliasParent); err != nil {
		t.Fatal(err)
	}
	ancestorTarget := filepath.Join(aliasParent, "sub", "ancestor.json")
	if err := WriteActivatedArtifact(ancestorTarget, activated, false); err == nil {
		t.Fatal("ancestor symlink output must be refused")
	}
	if _, err := os.Stat(filepath.Join(realParent, "ancestor.json")); !os.IsNotExist(err) {
		t.Fatalf("ancestor symlink must not redirect output: %v", err)
	}
	assertNoActivationTempFiles(t, realParent)
}

func TestWriteActivatedArtifactInstallRacesAreFailClosedAndCleaned(t *testing.T) {
	proposal := fixtureArtifact(t, "customer-data-to-egress")
	activated, _, err := Activate(proposal, ActivationOptions{PolicyDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActivatingPrincipal: "principal:owner", AuthorityRefs: []string{"approval:owner"}, Target: "target:deploy", Environment: "production", Mode: ActivationContextOnly, ValidFrom: "2026-07-19T00:00:00Z", SigningPrivateKey: mustKey(t), Selection: fixtureSelection(t, "customer-data-to-egress", proposal)})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("concurrent creation refuses without overwrite", func(t *testing.T) {
		dir := outputWriterTempDir(t)
		target := filepath.Join(dir, "activated.json")
		err := writeActivatedArtifact(target, activated, false, func() {
			if writeErr := os.WriteFile(target, []byte("raced"), 0o600); writeErr != nil {
				t.Errorf("create raced target: %v", writeErr)
			}
		})
		if err == nil || !strings.Contains(err.Error(), "appeared during install") {
			t.Fatalf("concurrent target creation must fail closed: %v", err)
		}
		assertNoActivationTempFiles(t, dir)
	})

	t.Run("concurrent symlink creation refuses without overwrite", func(t *testing.T) {
		dir := outputWriterTempDir(t)
		target := filepath.Join(dir, "activated.json")
		outside := filepath.Join(t.TempDir(), "outside.json")
		if writeErr := os.WriteFile(outside, []byte("outside"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		err := writeActivatedArtifact(target, activated, false, func() {
			if linkErr := os.Symlink(outside, target); linkErr != nil {
				t.Errorf("create raced symlink: %v", linkErr)
			}
		})
		if err == nil || !strings.Contains(err.Error(), "appeared during install") {
			t.Fatalf("concurrent symlink creation must fail closed: %v", err)
		}
		assertNoActivationTempFiles(t, dir)
	})

	t.Run("concurrent replacement refuses overwrite", func(t *testing.T) {
		dir := outputWriterTempDir(t)
		target := filepath.Join(dir, "activated.json")
		if err := WriteActivatedArtifact(target, activated, false); err != nil {
			t.Fatal(err)
		}
		err := writeActivatedArtifact(target, activated, true, func() {
			if removeErr := os.Remove(target); removeErr != nil {
				t.Errorf("remove target before replacement race: %v", removeErr)
				return
			}
			if writeErr := os.WriteFile(target, []byte("replacement"), 0o600); writeErr != nil {
				t.Errorf("replace raced target: %v", writeErr)
			}
		})
		if err == nil || !strings.Contains(err.Error(), "changed before overwrite") {
			t.Fatalf("concurrent regular replacement must fail closed: %v", err)
		}
		got, readErr := os.ReadFile(target)
		if readErr != nil || string(got) != "replacement" {
			t.Fatalf("concurrent replacement must remain untouched: bytes=%q err=%v", got, readErr)
		}
		assertNoActivationTempFiles(t, dir)
	})

	t.Run("concurrent symlink replacement refuses overwrite", func(t *testing.T) {
		dir := outputWriterTempDir(t)
		target := filepath.Join(dir, "activated.json")
		if err := WriteActivatedArtifact(target, activated, false); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := writeActivatedArtifact(target, activated, true, func() {
			if removeErr := os.Remove(target); removeErr != nil {
				t.Errorf("remove target before symlink race: %v", removeErr)
				return
			}
			if linkErr := os.Symlink(outside, target); linkErr != nil {
				t.Errorf("create replacement symlink: %v", linkErr)
			}
		})
		if err == nil || !strings.Contains(err.Error(), "changed before overwrite") {
			t.Fatalf("concurrent symlink replacement must fail closed: %v", err)
		}
		assertNoActivationTempFiles(t, dir)
	})
}

func assertNoActivationTempFiles(t *testing.T, dir string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, ".gait-activated-action-contract-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("temporary activation files leaked: %v", paths)
	}
}

func outputWriterTempDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/private/tmp"); err == nil {
			dir, err := os.MkdirTemp("/private/tmp", "gait-action-contract-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
	}
	return t.TempDir()
}

func fixturePath(t *testing.T, scenario string) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", scenario, "pac-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("fixture %s: %v (%v)", scenario, err, paths)
	}
	return paths[0]
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
