package main

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSigningSeedStrictFormats(t *testing.T) {
	seed := strings.Repeat("a", 32)
	if decoded, err := parseSigningSeed(strings.Repeat("1", 64)); err != nil || len(decoded) != 32 {
		t.Fatalf("hex seed: len=%d err=%v", len(decoded), err)
	}
	if decoded, err := parseSigningSeed(base64.StdEncoding.EncodeToString([]byte(seed))); err != nil || len(decoded) != 32 {
		t.Fatalf("base64 seed: len=%d err=%v", len(decoded), err)
	}
	for _, invalid := range []string{"", "not-hex-or-base64", "00"} {
		if _, err := parseSigningSeed(invalid); err == nil {
			t.Fatalf("invalid seed accepted: %q", invalid)
		}
	}
}

func TestAuthoritativeBundleGenerationAndVerification(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	commit := strings.Repeat("a", 40)
	seed := strings.Repeat("1", 64)
	if err := generateBundle(root, out, "v1.7.1", commit, workflowIdentity, ""); err == nil {
		t.Fatal("generation without a stable signing seed unexpectedly succeeded")
	}
	if err := generateBundle(root, out, "v1.7.1", commit, workflowIdentity, seed); err != nil {
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
	outAgain := t.TempDir()
	if err := generateBundle(root, outAgain, "v1.7.1", commit, workflowIdentity, seed); err != nil {
		t.Fatal(err)
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
			if err := verifyBundle(path, "", ""); err == nil {
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
	if err := generateBundle(root, out, "v1.7.1", commit, workflowIdentity, strings.Repeat("2", 64)); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(filepath.Join(out, "action-contract-authoritative-evidence-v1.7.1.zip"), "v1.7.2", commit); err == nil {
		t.Fatal("bundle with mismatched release tag unexpectedly verified")
	}
	if err := verifyBundle(filepath.Join(out, "action-contract-authoritative-evidence-v1.7.1.zip"), "v1.7.1", strings.Repeat("c", 40)); err == nil {
		t.Fatal("bundle with mismatched peeled commit unexpectedly verified")
	}
	if err := verifyBundle(filepath.Join(out, "missing.zip"), "v1.7.1", commit); err == nil {
		t.Fatal("missing bundle unexpectedly verified")
	}
}
