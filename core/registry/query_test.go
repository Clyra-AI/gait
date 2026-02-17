package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemaregistry "github.com/Clyra-AI/gait/core/schema/v1/registry"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestListAndVerifyInstalledPack(t *testing.T) {
	workDir := t.TempDir()
	cacheDir := filepath.Join(workDir, "cache")
	manifestPath, publicKey := mustWriteSignedRegistryManifest(t, workDir, "baseline-highrisk", "1.1.0")

	installResult, err := Install(context.Background(), InstallOptions{
		Source:    manifestPath,
		CacheDir:  cacheDir,
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("install registry pack: %v", err)
	}

	listResult, err := List(ListOptions{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("list installed packs: %v", err)
	}
	if len(listResult) != 1 {
		t.Fatalf("expected 1 installed pack, got %d", len(listResult))
	}
	if listResult[0].PackName != installResult.PackName || listResult[0].PackVersion != installResult.PackVersion {
		t.Fatalf("unexpected list item: %#v", listResult[0])
	}
	if !listResult[0].PinVerified {
		t.Fatalf("expected pin to verify")
	}

	verifyResult, err := Verify(VerifyOptions{
		MetadataPath: installResult.MetadataPath,
		CacheDir:     cacheDir,
		PublicKey:    publicKey,
	})
	if err != nil {
		t.Fatalf("verify installed pack: %v", err)
	}
	if !verifyResult.SignatureVerified {
		t.Fatalf("expected verified signature, error=%s", verifyResult.SignatureError)
	}
	if !verifyResult.PinPresent || !verifyResult.PinVerified {
		t.Fatalf("expected verified pin, got: %#v", verifyResult)
	}
	if verifyResult.Publisher != "acme" || verifyResult.Source != "registry" || !verifyResult.PublisherAllowed {
		t.Fatalf("expected publisher/source verification metadata, got %#v", verifyResult)
	}
}

func TestVerifyBranchesAndHelpers(t *testing.T) {
	workDir := t.TempDir()
	cacheDir := filepath.Join(workDir, "cache")
	manifestPath, publicKey := mustWriteSignedRegistryManifest(t, workDir, "pack-branches", "1.0.0")
	result, err := Install(context.Background(), InstallOptions{
		Source:    manifestPath,
		CacheDir:  cacheDir,
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("install registry pack: %v", err)
	}

	if _, err := Verify(VerifyOptions{}); err == nil {
		t.Fatalf("expected verify to require metadata path")
	}
	if _, err := Verify(VerifyOptions{MetadataPath: filepath.Join(workDir, "missing.json"), PublicKey: publicKey}); err == nil {
		t.Fatalf("expected verify missing metadata file error")
	}

	pinPath := filepath.Join(cacheDir, "pins", "pack-branches.pin")
	if err := os.WriteFile(pinPath, []byte("sha256:"+strings.Repeat("f", 64)+"\n"), 0o600); err != nil {
		t.Fatalf("write mismatched pin: %v", err)
	}
	mismatchPinResult, err := Verify(VerifyOptions{
		MetadataPath: result.MetadataPath,
		CacheDir:     cacheDir,
		PublicKey:    publicKey,
	})
	if err != nil {
		t.Fatalf("verify with mismatched pin: %v", err)
	}
	if !mismatchPinResult.PinPresent || mismatchPinResult.PinVerified {
		t.Fatalf("expected mismatched pin to fail verification: %#v", mismatchPinResult)
	}

	otherKey, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alternate key: %v", err)
	}
	badSigResult, err := Verify(VerifyOptions{
		MetadataPath: result.MetadataPath,
		CacheDir:     cacheDir,
		PublicKey:    otherKey.Public,
	})
	if err != nil {
		t.Fatalf("verify with alternate key should return non-fatal signature status: %v", err)
	}
	if badSigResult.SignatureVerified {
		t.Fatalf("expected signature verification failure with alternate key")
	}
	if strings.TrimSpace(badSigResult.SignatureError) == "" {
		t.Fatalf("expected signature error string")
	}

	publisherDeniedResult, err := Verify(VerifyOptions{
		MetadataPath:       result.MetadataPath,
		CacheDir:           cacheDir,
		PublicKey:          publicKey,
		PublisherAllowlist: []string{"other"},
	})
	if err != nil {
		t.Fatalf("verify with publisher allowlist should return non-fatal status: %v", err)
	}
	if publisherDeniedResult.PublisherAllowed {
		t.Fatalf("expected publisher allowlist mismatch")
	}

	if _, ok := inferCacheDirFromMetadataPath(result.MetadataPath, "pack-branches", "1.0.0", result.Digest); !ok {
		t.Fatalf("expected inferCacheDirFromMetadataPath to succeed")
	}
	if _, ok := inferCacheDirFromMetadataPath(filepath.Join(workDir, "other.json"), "pack-branches", "1.0.0", result.Digest); ok {
		t.Fatalf("expected inferCacheDirFromMetadataPath to fail")
	}

	emptyList, err := List(ListOptions{CacheDir: filepath.Join(workDir, "missing-cache")})
	if err != nil {
		t.Fatalf("list missing cache should not fail: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected empty list for missing cache, got %d", len(emptyList))
	}
}

func mustWriteSignedRegistryManifest(t *testing.T, dir string, packName string, packVersion string) (string, ed25519.PublicKey) {
	t.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-test",
		PackName:        packName,
		PackVersion:     packVersion,
		PackType:        "skill",
		Publisher:       "acme",
		Source:          "registry",
		Artifacts: []schemaregistry.PackArtifact{{
			Path:   "policy.yaml",
			SHA256: strings.Repeat("a", 64),
		}},
	}
	digest, err := signableManifestDigest(manifest)
	if err != nil {
		t.Fatalf("digest signable manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}

	path := filepath.Join(dir, packName+"_"+packVersion+".json")
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path, keyPair.Public
}
