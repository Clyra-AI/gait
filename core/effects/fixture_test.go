package effects

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proofsign "github.com/Clyra-AI/proof/signing"
)

type effectFixtureManifest struct {
	FixtureVersion string `json:"fixture_version"`
	Producer       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Schemas struct {
		Snapshot string `json:"snapshot"`
		Contract string `json:"contract"`
		Grade    string `json:"grade"`
	} `json:"schemas"`
	Signing struct {
		Mode             string `json:"mode"`
		Development      bool   `json:"development_signing"`
		NonAuthoritative bool   `json:"non_authoritative"`
		KeyID            string `json:"key_id"`
		PublicKeyPath    string `json:"public_key_path"`
		PublicKeySHA256  string `json:"public_key_sha256"`
		Derivation       string `json:"derivation"`
	} `json:"signing"`
	Files []struct {
		Path          string `json:"path"`
		SHA256        string `json:"sha256"`
		SchemaID      string `json:"schema_id"`
		SchemaVersion string `json:"schema_version"`
	} `json:"files"`
}

func TestCommittedEffectFixturePackHasNoDigestOrGradingDrift(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "effects", "v1")
	manifestRaw, err := os.ReadFile(filepath.Join(root, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest effectFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FixtureVersion != "1" || manifest.Producer.Name != "gait" || manifest.Producer.Version != "v1.5.0" || len(manifest.Files) != 4 {
		t.Fatalf("unexpected effect fixture manifest: %+v", manifest)
	}
	if manifest.Signing.Mode != "fixture_test_only" || !manifest.Signing.Development || !manifest.Signing.NonAuthoritative || manifest.Signing.PublicKeyPath == "" || !strings.Contains(manifest.Signing.Derivation, "never used by production") {
		t.Fatalf("fixture signing provenance missing: %+v", manifest.Signing)
	}
	publicRaw, err := os.ReadFile(filepath.Join("..", "..", manifest.Signing.PublicKeyPath))
	if err != nil {
		t.Fatal(err)
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(publicRaw)))
	if err != nil || len(publicKeyBytes) != ed25519.PublicKeySize {
		t.Fatalf("fixture public key invalid: %v", err)
	}
	if rawDigest(publicRaw) != manifest.Signing.PublicKeySHA256 || proofsign.KeyID(ed25519.PublicKey(publicKeyBytes)) != manifest.Signing.KeyID {
		t.Fatalf("fixture public key provenance drift")
	}
	var snapshot Snapshot
	var contract Contract
	var expected GradeResult
	for _, file := range manifest.Files {
		raw, readErr := os.ReadFile(filepath.Join(root, file.Path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		sum := sha256.Sum256(raw)
		gotDigest := "sha256:" + hex.EncodeToString(sum[:])
		if gotDigest != file.SHA256 {
			t.Fatalf("fixture byte digest drift for %s: got %s want %s", file.Path, gotDigest, file.SHA256)
		}
		switch file.Path {
		case "effect_snapshot.json":
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
		case "effect_contract.json":
			if err := json.Unmarshal(raw, &contract); err != nil {
				t.Fatal(err)
			}
		case "effect_grading_result.json":
			if err := json.Unmarshal(raw, &expected); err != nil {
				t.Fatal(err)
			}
		}
	}
	if result := ValidateSnapshot(snapshot); !result.Valid {
		t.Fatalf("fixture snapshot invalid: %+v", result)
	}
	if result := ValidateContract(contract); !result.Valid {
		t.Fatalf("fixture contract invalid: %+v", result)
	}
	seed := sha256.Sum256([]byte("gait-effects-fixture-producer-key-v1"))
	trusted := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	actual := GradeWithOptions(snapshot, contract, GradeOptions{TrustedCollectorPublicKey: trusted, AllowFixtureTestProvenance: true, ExpectedCorrelation: &CorrelationExpectation{ActionDigest: snapshot.Correlation.ActionDigest, ActivationDigest: snapshot.Correlation.ActivationDigest, ProofDigest: snapshot.Correlation.ProofDigest}})
	if actual.Status != GradePass || actual.Status != expected.Status || actual.ContractID != expected.ContractID || actual.SnapshotID != expected.SnapshotID {
		t.Fatalf("fixture grade drift: actual=%+v expected=%+v", actual, expected)
	}
	actualRaw, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actualRaw = append(actualRaw, '\n')
	expectedRaw, err := os.ReadFile(filepath.Join(root, "effect_grading_result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actualRaw) != string(expectedRaw) {
		t.Fatalf("golden grading bytes drifted")
	}
	tampered := snapshot
	tampered.After.Digest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	if ValidateSnapshot(tampered).Valid || !strings.Contains(strings.Join(ValidateSnapshot(tampered).ReasonCodes, ","), ReasonDigestMismatch) {
		t.Fatalf("tampered fixture snapshot was accepted")
	}
	tampered = snapshot
	tampered.CanonicalContentDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if err := tampered.VerifyProvenanceAgainst(trusted); err == nil {
		t.Fatal("tampered canonical digest passed provenance verification")
	}
}

func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
