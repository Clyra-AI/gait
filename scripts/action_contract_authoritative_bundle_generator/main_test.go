package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSigningIdentityIsDeterministicAndDomainSeparated(t *testing.T) {
	first := releaseSigningSeed("v1.7.2", strings.Repeat("a", 40), workflowIdentity)
	second := releaseSigningSeed("v1.7.2", strings.Repeat("a", 40), workflowIdentity)
	if string(first) != string(second) {
		t.Fatal("same release identity produced different signing seed")
	}
	if string(first) == string(releaseSigningSeed("v1.7.3", strings.Repeat("a", 40), workflowIdentity)) || string(first) == string(releaseSigningSeed("v1.7.2", strings.Repeat("b", 40), workflowIdentity)) {
		t.Fatal("changed release identity reused signing seed")
	}
}

func TestGeneratorRejectsMissingInputs(t *testing.T) {
	if _, _, err := readRuntimeAction(filepath.Join(t.TempDir(), "missing-runtime-action.json")); err == nil {
		t.Fatal("missing runtime action unexpectedly accepted")
	}
	if _, _, err := releaseReadiness(filepath.Join(t.TempDir(), "missing-readiness.json"), "contract", "sha256:"+strings.Repeat("a", 64), nil, nil); err == nil {
		t.Fatal("missing readiness unexpectedly accepted")
	}
}

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
	checksums := trustedChecksums(t, out, bundle)
	if err := verifyBundle(bundle, "v1.7.1", commit, checksums); err != nil {
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
	if manifestValue.Signing.KeyOrigin != "deterministic_release_identity_non_secret_internal_integrity" {
		t.Fatalf("unexpected release key origin: %q", manifestValue.Signing.KeyOrigin)
	}
	outAgain := t.TempDir()
	if err := generateBundle(root, outAgain, "v1.7.1", commit, workflowIdentity); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(bundle, "v1.7.1", commit, ""); err == nil {
		t.Fatal("bundle verified without caller-trusted checksum anchor")
	}
	firstBytes, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(filepath.Join(outAgain, "action-contract-authoritative-evidence-v1.7.1.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("same release seed did not produce deterministic bundle bytes")
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

func TestAuthoritativeBundleRejectsDuplicateAndOversizedEntries(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func(*zip.Writer) error
	}{
		{name: "duplicate", write: func(writer *zip.Writer) error {
			first, err := writer.Create("manifest.json")
			if err != nil {
				return err
			}
			if _, err := first.Write([]byte("{}")); err != nil {
				return err
			}
			second, err := writer.Create("manifest.json")
			if err != nil {
				return err
			}
			_, err = second.Write([]byte("{}"))
			return err
		}},
		{name: "oversized", write: func(writer *zip.Writer) error {
			entry, err := writer.Create("oversized.bin")
			if err != nil {
				return err
			}
			_, err = entry.Write(make([]byte, maxBundleEntryBytes+1))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name+".zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			if err := test.write(writer); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			checksums := filepath.Join(t.TempDir(), "checksums.txt")
			if err := os.WriteFile(checksums, []byte(""), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := verifyBundle(path, "", "", checksums); err == nil {
				t.Fatal("unsafe ZIP unexpectedly verified")
			}
		})
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
	bundle := filepath.Join(out, "action-contract-authoritative-evidence-v1.7.1.zip")
	checksums := trustedChecksums(t, out, bundle)
	if err := verifyBundle(bundle, "v1.7.2", commit, checksums); err == nil {
		t.Fatal("bundle with mismatched release tag unexpectedly verified")
	}
	if err := verifyBundle(bundle, "v1.7.1", strings.Repeat("c", 40), checksums); err == nil {
		t.Fatal("bundle with mismatched peeled commit unexpectedly verified")
	}
	if err := verifyBundle(filepath.Join(out, "missing.zip"), "v1.7.1", commit, checksums); err == nil {
		t.Fatal("missing bundle unexpectedly verified")
	}
}

func trustedChecksums(t *testing.T, out, bundle string) string {
	t.Helper()
	manifestRaw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value manifest
	if err := json.Unmarshal(manifestRaw, &value); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(value.Artifacts)+len(value.ReferencedSchemas)+1)
	for _, entry := range append(append([]digestEntry{}, value.Artifacts...), value.ReferencedSchemas...) {
		lines = append(lines, strings.TrimPrefix(entry.SHA256, "sha256:")+"  action-contract-authoritative/"+entry.Path)
	}
	bundleRaw, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	lines = append(lines, strings.TrimPrefix(rawDigest(bundleRaw), "sha256:")+"  action-contract-authoritative/"+filepath.Base(bundle))
	path := filepath.Join(out, "trusted-checksums.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
