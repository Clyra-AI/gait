package runpack

import (
	"bytes"
	"crypto/ed25519"
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
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, manifestFiles)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

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
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, manifestFiles)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	for index := range runpackFiles {
		if runpackFiles[index].Path == "run.json" {
			runpackFiles[index].Data = []byte("{\"tampered\":true}\n")
		}
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

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
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, manifestFiles)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	tamperedManifest := bytes.Replace(manifestBytes, []byte("run_test"), []byte("run_bad"), 1)
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: tamperedManifest, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

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

func TestVerifyZipMissingManifest(test *testing.T) {
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "run.json", Data: []byte("hello"), Mode: 0o644},
	})
	if _, err := VerifyZip(zipPath, VerifyOptions{}); err == nil {
		test.Fatalf("expected error for missing manifest")
	}
}

func TestVerifyZipMissingFile(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, manifestFiles)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	filteredFiles := make([]zipx.File, 0, len(runpackFiles))
	for _, file := range runpackFiles {
		if file.Path == "refs.json" {
			continue
		}
		filteredFiles = append(filteredFiles, file)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, filteredFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	result, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if len(result.MissingFiles) != 1 {
		test.Fatalf("expected missing file to be reported")
	}
	if result.MissingFiles[0] != "refs.json" {
		test.Fatalf("expected refs.json missing, got %v", result.MissingFiles)
	}
}

func TestVerifyZipSignatureMissingRequired(test *testing.T) {
	manifestBytes, err := buildManifestBytes("run_test", nil, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "missing" {
		test.Fatalf("expected missing signature status")
	}
	if len(result.SignatureErrors) == 0 {
		test.Fatalf("expected signature error when required")
	}
}

func TestVerifyZipSignatureMissingNotRequired(test *testing.T) {
	manifestBytes, err := buildManifestBytes("run_test", nil, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "missing" {
		test.Fatalf("expected missing signature status")
	}
	if len(result.SignatureErrors) != 0 {
		test.Fatalf("expected no signature errors when not required")
	}
}

func TestVerifyZipSignatureSkippedNoPublicKey(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildSignedManifestBytes(keyPair.Private, manifestFiles)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	result, err := VerifyZip(zipPath, VerifyOptions{
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "skipped" {
		test.Fatalf("expected skipped signature status")
	}
	if len(result.SignatureErrors) == 0 {
		test.Fatalf("expected signature error when public key missing")
	}
}

func TestVerifyZipSignedDigestMismatch(test *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		test.Fatalf("generate keypair: %v", err)
	}
	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           "run_test",
		CaptureMode:     "reference",
		Files:           []schemarunpack.ManifestFile{},
		ManifestDigest:  "1111111111111111111111111111111111111111111111111111111111111111",
	}
	unsignedBytes, err := json.Marshal(manifest)
	if err != nil {
		test.Fatalf("marshal manifest: %v", err)
	}
	signable, err := signableManifestBytes(unsignedBytes)
	if err != nil {
		test.Fatalf("signable manifest: %v", err)
	}
	signature, err := sign.SignManifestJSON(keyPair.Private, signable)
	if err != nil {
		test.Fatalf("sign manifest: %v", err)
	}
	signature.SignedDigest = "2222222222222222222222222222222222222222222222222222222222222222"
	manifest.Signatures = []schemarunpack.Signature{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		test.Fatalf("marshal manifest: %v", err)
	}
	zipPath := writeRunpackZip(test, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	})

	result, err := VerifyZip(zipPath, VerifyOptions{
		PublicKey:        keyPair.Public,
		RequireSignature: true,
	})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if result.SignatureStatus != "failed" {
		test.Fatalf("expected failed signature status")
	}
}

func TestVerifyZipManifestMissingRunID(test *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("", manifestFiles, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)
	if _, err := VerifyZip(zipPath, VerifyOptions{}); err == nil {
		test.Fatalf("expected error for missing run_id")
	}
}

func TestVerifyZipManifestDigestMismatch(test *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		test.Fatalf("decode manifest: %v", err)
	}
	manifest.ManifestDigest = "deadbeef"
	tamperedManifestBytes, err := json.Marshal(manifest)
	if err != nil {
		test.Fatalf("encode manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: tamperedManifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	result, err := VerifyZip(zipPath, VerifyOptions{})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if len(result.HashMismatches) != 1 {
		test.Fatalf("expected manifest digest mismatch, got %v", result.HashMismatches)
	}
	if result.HashMismatches[0].Path != "manifest.json" {
		test.Fatalf("expected manifest.json mismatch path, got %s", result.HashMismatches[0].Path)
	}
}

func TestVerifyZipManifestSchemaValidation(test *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		test.Fatalf("decode manifest: %v", err)
	}
	manifest.SchemaID = "not.gait.manifest"
	tamperedManifestBytes, err := json.Marshal(manifest)
	if err != nil {
		test.Fatalf("encode manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: tamperedManifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	if _, err := VerifyZip(zipPath, VerifyOptions{}); err == nil {
		test.Fatalf("expected schema validation error")
	}
}

func TestVerifyZipManifestSchemaVersionValidation(test *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		test.Fatalf("decode manifest: %v", err)
	}
	manifest.SchemaVersion = "2.0.0"
	tamperedManifestBytes, err := json.Marshal(manifest)
	if err != nil {
		test.Fatalf("encode manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: tamperedManifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	if _, err := VerifyZip(zipPath, VerifyOptions{}); err == nil {
		test.Fatalf("expected schema version validation error")
	}
}

func TestVerifyZipMissingRequiredManifestEntry(test *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	filteredManifestFiles := make([]schemarunpack.ManifestFile, 0, len(manifestFiles))
	for _, file := range manifestFiles {
		if file.Path == "results.jsonl" {
			continue
		}
		filteredManifestFiles = append(filteredManifestFiles, file)
	}
	manifestBytes, err := buildManifestBytes("run_test", filteredManifestFiles, nil)
	if err != nil {
		test.Fatalf("build manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	zipPath := writeRunpackZip(test, archiveFiles)

	result, err := VerifyZip(zipPath, VerifyOptions{})
	if err != nil {
		test.Fatalf("verify zip: %v", err)
	}
	if len(result.MissingFiles) != 1 || result.MissingFiles[0] != "results.jsonl" {
		test.Fatalf("expected missing required manifest entry for results.jsonl, got %v", result.MissingFiles)
	}
}

func TestSignableManifestBytesInvalidJSON(test *testing.T) {
	if _, err := signableManifestBytes([]byte("{")); err == nil {
		test.Fatalf("expected error for invalid json")
	}
}

func buildSignedManifestBytes(privateKey ed25519.PrivateKey, files []schemarunpack.ManifestFile) ([]byte, error) {
	manifestBytes, err := buildManifestBytes("run_test", files, nil)
	if err != nil {
		return nil, err
	}
	signable, err := signableManifestBytes(manifestBytes)
	if err != nil {
		return nil, err
	}
	signature, err := sign.SignManifestJSON(privateKey, signable)
	if err != nil {
		return nil, err
	}
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
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

func buildManifestBytes(runID string, files []schemarunpack.ManifestFile, signatures []schemarunpack.Signature) ([]byte, error) {
	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           runID,
		CaptureMode:     "reference",
		Files:           files,
		Signatures:      signatures,
	}
	digest, err := computeManifestDigest(manifest)
	if err != nil {
		return nil, err
	}
	manifest.ManifestDigest = digest
	return json.Marshal(manifest)
}

func buildCompleteRunpackFixture() ([]schemarunpack.ManifestFile, []zipx.File) {
	runData := []byte("{\"run\":\"ok\"}\n")
	intentsData := []byte("{\"intent\":\"ok\"}\n")
	resultsData := []byte("{\"result\":\"ok\"}\n")
	refsData := []byte("{\"refs\":\"ok\"}\n")
	manifestFiles := []schemarunpack.ManifestFile{
		{Path: "run.json", SHA256: sha256Hex(runData)},
		{Path: "intents.jsonl", SHA256: sha256Hex(intentsData)},
		{Path: "results.jsonl", SHA256: sha256Hex(resultsData)},
		{Path: "refs.json", SHA256: sha256Hex(refsData)},
	}
	runpackFiles := []zipx.File{
		{Path: "run.json", Data: runData, Mode: 0o644},
		{Path: "intents.jsonl", Data: intentsData, Mode: 0o644},
		{Path: "results.jsonl", Data: resultsData, Mode: 0o644},
		{Path: "refs.json", Data: refsData, Mode: 0o644},
	}
	return manifestFiles, runpackFiles
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
