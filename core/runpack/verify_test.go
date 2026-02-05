package runpack

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
	"github.com/davidahmann/gait/core/zipx"
)

func TestVerifyZipSuccess(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	fileData := []byte("hello")
	fileHash := sha256.Sum256(fileData)
	fileHashHex := hex.EncodeToString(fileHash[:])
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, []schemarunpack.ManifestFile{
		{Path: "run.json", SHA256: fileHashHex},
	})
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
		{Path: "run.json", Data: fileData, Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "verified" {
		test.Fatalf("expected verified signature, got %s", result.SignatureStatus)
	}
	if len(result.HashMismatches) != 0 || len(result.MissingFiles) != 0 {
		test.Fatalf("expected no hash issues")
	}
}

func TestVerifyZipHashMismatch(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	expectedHash := sha256.Sum256([]byte("hello"))
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, []schemarunpack.ManifestFile{
		{Path: "run.json", SHA256: hex.EncodeToString(expectedHash[:])},
	})
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
		{Path: "run.json", Data: []byte("tampered"), Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if len(result.HashMismatches) != 1 {
		test.Fatalf("expected hash mismatch")
	}
	if result.SignatureStatus != "verified" {
		test.Fatalf("expected verified signature, got %s", result.SignatureStatus)
	}
}

func TestVerifyZipSignatureFailure(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	fileData := []byte("hello")
	fileHash := sha256.Sum256(fileData)
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, []schemarunpack.ManifestFile{
		{Path: "run.json", SHA256: hex.EncodeToString(fileHash[:])},
	})
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	tamperedManifest := bytes.Replace(manifestBytes, []byte("run_test"), []byte("run_bad"), 1)
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: tamperedManifest, Mode: 0o644},
		{Path: "run.json", Data: fileData, Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "failed" {
		test.Fatalf("expected failed signature, got %s", result.SignatureStatus)
	}
	if len(result.SignatureErrors) == 0 {
		test.Fatalf("expected signature errors")
	}
}

func buildSignedManifestBytes(privateKey ed25519.PrivateKey, files []schemarunpack.ManifestFile) ([]byte, error) {
	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           "run_test",
		CaptureMode:     "reference",
		Files:           files,
		ManifestDigest:  "1111111111111111111111111111111111111111111111111111111111111111",
	}
	unsignedBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	signable, err := signableManifestBytes(unsignedBytes)
	if err != nil {
		return nil, err
	}
	signature, err := sign.SignManifestJSON(privateKey, signable)
	if err != nil {
		return nil, err
	}
	manifest.Signatures = []schemarunpack.Signature{
		{
			Alg:          signature.Alg,
			KeyID:        signature.KeyID,
			Sig:          signature.Sig,
			SignedDigest: signature.SignedDigest,
		},
	}
	return json.Marshal(manifest)
}

func writeRunpackZip(test *testing.T, files []zipx.File) string {
	test.Helper()
	var buffer bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buffer, files); err != nil {
		test.Fatalf("write zip: %v", err)
	}
	path := filepath.Join(test.TempDir(), "runpack_test.zip")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		test.Fatalf("write zip file: %v", err)
	}
	return path
}
