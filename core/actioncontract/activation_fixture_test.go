package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type activationFixtureManifest struct {
	FixtureVersion string `json:"fixture_version"`
	Producer       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Schemas struct {
		Artifact string `json:"artifact"`
		Contract string `json:"contract"`
	} `json:"schemas"`
	Signing struct {
		Mode          string `json:"mode"`
		Development   bool   `json:"development_signing"`
		KeyID         string `json:"key_id"`
		PublicKeyPath string `json:"public_key_path"`
		Derivation    string `json:"derivation"`
	} `json:"signing"`
	Scenarios []struct {
		ScenarioID            string   `json:"scenario_id"`
		ProposalPath          string   `json:"proposal_path"`
		ProposalSHA256        string   `json:"proposal_sha256"`
		CanonicalDigest       string   `json:"canonical_content_digest"`
		ProposalArtifactID    string   `json:"proposal_artifact_id"`
		ContractID            string   `json:"contract_id"`
		ContractFamilyID      string   `json:"contract_family_id"`
		Revision              int      `json:"revision"`
		Current               bool     `json:"current"`
		ActivationPath        string   `json:"activation_path"`
		ActivationSHA256      string   `json:"activation_sha256"`
		ActivationArtifactID  string   `json:"activation_artifact_id"`
		ActivationSchema      string   `json:"activation_schema_version"`
		ContractSchema        string   `json:"contract_schema_version"`
		DevelopmentSigning    bool     `json:"development_signing"`
		ActivationStatus      string   `json:"activation_status"`
		ActivationReasonCodes []string `json:"activation_reason_codes"`
	} `json:"scenarios"`
}

func TestActivationFixturePackIsReleasedProducerBoundAndVerifiable(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1")
	manifestRaw, err := os.ReadFile(filepath.Join(root, "expected", "activation-fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest activationFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FixtureVersion != "1" || manifest.Producer.Name != "gait" || manifest.Producer.Version != "v1.4.0" || manifest.Schemas.Artifact != "1" || manifest.Schemas.Contract != "1" || len(manifest.Scenarios) != 9 {
		t.Fatalf("unexpected activation fixture provenance: %+v", manifest)
	}
	if !manifest.Signing.Development || manifest.Signing.KeyID == "" || !strings.Contains(manifest.Signing.Derivation, "never used by production/default activation") {
		t.Fatalf("activation fixture signing provenance is not explicitly test-only: %+v", manifest.Signing)
	}
	publicRaw, err := os.ReadFile(filepath.Join("..", "..", manifest.Signing.PublicKeyPath))
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(publicRaw)))
	if err != nil || len(publicBytes) != ed25519.PublicKeySize {
		t.Fatalf("invalid fixture public key: %v", err)
	}
	publicKey := ed25519.PublicKey(publicBytes)
	if got := keyID(publicKey); got != manifest.Signing.KeyID {
		t.Fatalf("fixture key id mismatch: got %s want %s", got, manifest.Signing.KeyID)
	}

	for _, scenario := range manifest.Scenarios {
		proposal, rawProposal, err := ReadArtifact(filepath.Join(root, scenario.ProposalPath))
		if err != nil {
			t.Fatalf("read proposal %s: %v", scenario.ScenarioID, err)
		}
		if RawDigest(rawProposal) != scenario.ProposalSHA256 || proposal.CanonicalContentDigest != scenario.CanonicalDigest {
			t.Fatalf("proposal provenance mismatch for %s", scenario.ScenarioID)
		}
		if scenario.ActivationStatus != "activated" {
			if scenario.ActivationPath != "" || len(scenario.ActivationReasonCodes) == 0 {
				t.Fatalf("non-activated scenario lacks explicit reason: %+v", scenario)
			}
			continue
		}
		activationRaw, err := os.ReadFile(filepath.Join(root, scenario.ActivationPath))
		if err != nil {
			t.Fatalf("read activation %s: %v", scenario.ScenarioID, err)
		}
		if RawDigest(activationRaw) != scenario.ActivationSHA256 {
			t.Fatalf("activation byte digest mismatch for %s", scenario.ScenarioID)
		}
		activation, err := ParseActivatedArtifact(activationRaw)
		if err != nil {
			t.Fatalf("parse activation %s: %v", scenario.ScenarioID, err)
		}
		if !activation.DevelopmentSigning || activation.ArtifactID != scenario.ActivationArtifactID || activation.ContractID != scenario.ContractID || activation.ContractFamilyID != scenario.ContractFamilyID || activation.Revision != scenario.Revision {
			t.Fatalf("activation identity/provenance mismatch for %s: %+v", scenario.ScenarioID, activation)
		}
		valid, err := VerifyActivationWithOptions(activation, publicKey, VerificationOptions{AllowDevelopmentSigning: true, Proposal: &proposal, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)})
		if err != nil || !valid {
			t.Fatalf("activation verification failed for %s: valid=%v err=%v", scenario.ScenarioID, valid, err)
		}
		if valid, err := VerifyActivationWithOptions(activation, publicKey, VerificationOptions{Proposal: &proposal, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}); valid || err == nil {
			t.Fatalf("development activation was accepted without explicit test opt-in for %s", scenario.ScenarioID)
		}
		var tampered map[string]any
		if err := json.Unmarshal(activationRaw, &tampered); err != nil {
			t.Fatal(err)
		}
		tampered["target"] = "target:tampered"
		tamperedRaw, err := json.Marshal(tampered)
		if err != nil {
			t.Fatal(err)
		}
		tamperedActivation, err := ParseActivatedArtifact(tamperedRaw)
		if err != nil {
			t.Fatal(err)
		}
		if valid, _ := VerifyActivationWithOptions(tamperedActivation, publicKey, VerificationOptions{AllowDevelopmentSigning: true, Proposal: &proposal, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}); valid {
			t.Fatalf("tampered activation verified for %s", scenario.ScenarioID)
		}
	}
}

func TestActivationFixtureParserRejectsDuplicateAndUnknownFields(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected", "customer-data-to-egress", "activated-action-contract.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	needle := []byte(`"target": "target:fixture",`)
	if !bytes.Contains(raw, needle) {
		t.Fatal("activation target marker missing")
	}
	duplicate := bytes.Replace(raw, needle, append(append([]byte{}, needle...), []byte(`"target": "target:duplicate",`)...), 1)
	if _, err := ParseActivatedArtifact(duplicate); err == nil {
		t.Fatal("duplicate activation key accepted")
	}
	unknown := bytes.Replace(raw, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)
	if _, err := ParseActivatedArtifact(unknown); err == nil {
		t.Fatal("unknown activation field accepted")
	}
}

func keyID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:])
}
