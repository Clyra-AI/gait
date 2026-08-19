package actioncontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestFixtureDigests(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(root, entry.Name(), "pac-*.json"))
		for _, path := range files {
			artifact, raw, err := ReadArtifact(path)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			result := ValidateArtifact(artifact, ValidationOptions{Now: mustTime("2026-07-19T00:00:00Z")})
			if result.CanonicalContentDigest != artifact.CanonicalContentDigest {
				t.Fatalf("%s: canonical digest %s != fixture %s", path, result.CanonicalContentDigest, artifact.CanonicalContentDigest)
			}
			if RawDigest(raw) == "" {
				t.Fatalf("%s: missing byte digest", path)
			}
			if entry.Name() == "approval-expiry" && result.Valid {
				t.Fatalf("%s: expired proposal unexpectedly valid", path)
			}
			if entry.Name() != "approval-expiry" && entry.Name() != "excessive-child-authority" && entry.Name() != "failed-effect-validation" && !result.Valid {
				t.Fatalf("%s: unexpected validation reasons %v", path, result.Reasons)
			}
		}
	}
}

func TestFixtureManifestPinsLocalReleasedArtifacts(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected")
	manifestPath := filepath.Join(root, "fixture-manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		FixtureVersion string `json:"fixture_version"`
		Producer       struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"producer"`
		Schemas struct {
			Artifact string `json:"artifact"`
			Contract string `json:"contract"`
		} `json:"schemas"`
		Scenarios []struct {
			ArtifactPath           string `json:"artifact_path"`
			ArtifactSHA256         string `json:"artifact_sha256"`
			ArtifactID             string `json:"artifact_id"`
			CanonicalContentDigest string `json:"canonical_content_digest"`
			ContractID             string `json:"contract_id"`
			ContractFamilyID       string `json:"contract_family_id"`
			Revision               int    `json:"revision"`
			Current                bool   `json:"current"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FixtureVersion != "1" || manifest.Producer.Name != "wrkr" || manifest.Producer.Version != "v1.14.0" || manifest.Schemas.Artifact != "1" || manifest.Schemas.Contract != "3" || len(manifest.Scenarios) != 9 {
		t.Fatalf("unexpected fixture provenance: %+v", manifest)
	}
	for _, scenario := range manifest.Scenarios {
		path := filepath.Join(root, filepath.FromSlash(scenario.ArtifactPath))
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("unsafe fixture path: %q", scenario.ArtifactPath)
		}
		artifactRaw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", scenario.ArtifactPath, err)
		}
		sum := sha256.Sum256(artifactRaw)
		if got := "sha256:" + hex.EncodeToString(sum[:]); got != scenario.ArtifactSHA256 {
			t.Fatalf("artifact digest mismatch for %s: got %s want %s", scenario.ArtifactPath, got, scenario.ArtifactSHA256)
		}
		artifact, result := ValidateArtifactBytes(artifactRaw, ValidationOptions{})
		if artifact.ArtifactID != scenario.ArtifactID || artifact.ContractID != scenario.ContractID || artifact.ContractFamilyID != scenario.ContractFamilyID || artifact.Revision != scenario.Revision || artifact.CanonicalContentDigest != scenario.CanonicalContentDigest || !scenario.Current || contains(result.Reasons, ReasonSchemaValidationFailed) {
			t.Fatalf("manifest identity mismatch for %s: artifact=%+v result=%+v", scenario.ArtifactPath, artifact, result)
		}
	}
}
