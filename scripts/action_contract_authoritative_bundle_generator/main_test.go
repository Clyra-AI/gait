package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthoritativeBundleGenerationAndVerification(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	commit := strings.Repeat("a", 40)
	if err := generateBundle(root, out, "v1.7.1", commit, workflowIdentity); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(out, "action-contract-authoritative-evidence-v1.7.1.zip")
	if err := verifyBundle(bundle, "v1.7.1", commit); err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifestValue manifest
	if err := json.Unmarshal(manifestRaw, &manifestValue); err != nil {
		t.Fatal(err)
	}
	if !manifestValue.Authoritative || manifestValue.FixtureOnly || manifestValue.DevelopmentSign || manifestValue.Quarantine {
		t.Fatalf("unsafe manifest markers: %#v", manifestValue)
	}
	reader, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	for _, required := range []string{"manifest.json", "public-key.b64", "proposal.json", "activation.json", "runtime-action.json", "runtime-readiness.json", "lifecycle.json"} {
		found := false
		for _, file := range reader.File {
			if file.Name == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("bundle missing %s", required)
		}
	}
}

func TestAuthoritativeBundleRejectsIdentityMismatch(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	commit := strings.Repeat("b", 40)
	if err := generateBundle(root, out, "v1.7.1", commit, workflowIdentity); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(filepath.Join(out, "action-contract-authoritative-evidence-v1.7.1.zip"), "v1.7.2", commit); err == nil {
		t.Fatal("bundle with mismatched release tag unexpectedly verified")
	}
}
