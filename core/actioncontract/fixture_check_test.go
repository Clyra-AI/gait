package actioncontract

import (
	"os"
	"path/filepath"
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
