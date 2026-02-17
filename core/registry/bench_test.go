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

func BenchmarkInstallLocalTypical(b *testing.B) {
	workDir := b.TempDir()
	manifestPath, publicKey := mustWriteRegistryBenchmarkManifest(b, workDir, "bench-pack", "1.0.0")
	cacheDir := filepath.Join(workDir, "cache")

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := Install(context.Background(), InstallOptions{
			Source:    manifestPath,
			CacheDir:  cacheDir,
			PublicKey: publicKey,
		})
		if err != nil {
			b.Fatalf("install local manifest: %v", err)
		}
		if result.Digest == "" {
			b.Fatalf("expected digest")
		}
	}
}

func BenchmarkVerifyInstalledTypical(b *testing.B) {
	workDir := b.TempDir()
	manifestPath, publicKey := mustWriteRegistryBenchmarkManifest(b, workDir, "bench-verify-pack", "1.0.0")
	cacheDir := filepath.Join(workDir, "cache")
	installResult, err := Install(context.Background(), InstallOptions{
		Source:    manifestPath,
		CacheDir:  cacheDir,
		PublicKey: publicKey,
	})
	if err != nil {
		b.Fatalf("install manifest for verify bench: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, verifyErr := Verify(VerifyOptions{
			MetadataPath: installResult.MetadataPath,
			CacheDir:     cacheDir,
			PublicKey:    publicKey,
		})
		if verifyErr != nil {
			b.Fatalf("verify installed manifest: %v", verifyErr)
		}
		if !result.SignatureVerified || !result.PinVerified {
			b.Fatalf("unexpected verify result: %#v", result)
		}
	}
}

func mustWriteRegistryBenchmarkManifest(b *testing.B, workDir string, packName string, packVersion string) (string, ed25519.PublicKey) {
	b.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		b.Fatalf("generate benchmark key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-bench",
		PackName:        packName,
		PackVersion:     packVersion,
		Artifacts: []schemaregistry.PackArtifact{{
			Path:   "policy.yaml",
			SHA256: strings.Repeat("a", 64),
		}},
	}
	digest, err := signableManifestDigest(manifest)
	if err != nil {
		b.Fatalf("digest benchmark manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		b.Fatalf("sign benchmark manifest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}

	path := filepath.Join(workDir, packName+"_"+packVersion+".json")
	raw, err := json.Marshal(manifest)
	if err != nil {
		b.Fatalf("marshal benchmark manifest: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		b.Fatalf("write benchmark manifest: %v", err)
	}
	return path, keyPair.Public
}
