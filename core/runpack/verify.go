package runpack

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
)

type VerifyOptions struct {
	PublicKey        ed25519.PublicKey
	RequireSignature bool
}

type VerifyResult struct {
	RunID           string         `json:"run_id,omitempty"`
	ManifestDigest  string         `json:"manifest_digest,omitempty"`
	FilesChecked    int            `json:"files_checked"`
	MissingFiles    []string       `json:"missing_files,omitempty"`
	HashMismatches  []HashMismatch `json:"hash_mismatches,omitempty"`
	SignatureStatus string         `json:"signature_status"`
	SignatureErrors []string       `json:"signature_errors,omitempty"`
	SignaturesTotal int            `json:"signatures_total"`
	SignaturesValid int            `json:"signatures_valid"`
}

type HashMismatch struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

const maxZipFileBytes = 100 * 1024 * 1024

var requiredRunpackFiles = []string{
	"run.json",
	"intents.jsonl",
	"results.jsonl",
	"refs.json",
}

func VerifyZip(path string, opts VerifyOptions) (VerifyResult, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("open zip: %w", err)
	}
	defer func() {
		_ = zipReader.Close()
	}()

	files := make(map[string]*zip.File, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		files[zipFile.Name] = zipFile
	}

	manifestFile, manifestFound := files["manifest.json"]
	if !manifestFound {
		return VerifyResult{}, fmt.Errorf("missing manifest.json")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("read manifest.json: %w", err)
	}

	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return VerifyResult{}, fmt.Errorf("parse manifest.json: %w", err)
	}
	if manifest.SchemaID != "gait.runpack.manifest" {
		return VerifyResult{}, fmt.Errorf("manifest schema_id must be gait.runpack.manifest")
	}
	if manifest.SchemaVersion != "1.0.0" {
		return VerifyResult{}, fmt.Errorf("manifest schema_version must be 1.0.0")
	}
	if manifest.RunID == "" {
		return VerifyResult{}, fmt.Errorf("manifest missing run_id")
	}

	result := VerifyResult{
		RunID:           manifest.RunID,
		ManifestDigest:  manifest.ManifestDigest,
		FilesChecked:    len(manifest.Files),
		SignatureStatus: "missing",
		SignaturesTotal: len(manifest.Signatures),
	}

	declared := make(map[string]struct{}, len(manifest.Files))
	for _, entry := range manifest.Files {
		name := filepath.ToSlash(entry.Path)
		declared[name] = struct{}{}
		zipFile, exists := files[name]
		if !exists {
			result.MissingFiles = append(result.MissingFiles, name)
			continue
		}
		actual, err := hashZipFile(zipFile)
		if err != nil {
			return VerifyResult{}, fmt.Errorf("hash %s: %w", name, err)
		}
		if !equalHex(actual, entry.SHA256) {
			result.HashMismatches = append(result.HashMismatches, HashMismatch{
				Path:     name,
				Expected: entry.SHA256,
				Actual:   actual,
			})
		}
	}
	for _, requiredPath := range requiredRunpackFiles {
		if _, ok := declared[requiredPath]; !ok {
			result.MissingFiles = append(result.MissingFiles, requiredPath)
		}
	}

	computedManifestDigest, err := computeManifestDigest(manifest)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("compute manifest digest: %w", err)
	}
	if !equalHex(manifest.ManifestDigest, computedManifestDigest) {
		result.HashMismatches = append(result.HashMismatches, HashMismatch{
			Path:     "manifest.json",
			Expected: manifest.ManifestDigest,
			Actual:   computedManifestDigest,
		})
	}

	signable, err := signableManifestBytes(manifestBytes)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("prepare manifest for signing: %w", err)
	}

	if len(manifest.Signatures) == 0 {
		result.SignatureStatus = "missing"
		if opts.RequireSignature {
			result.SignatureErrors = append(result.SignatureErrors, "manifest has no signatures")
		}
	} else if opts.PublicKey == nil {
		result.SignatureStatus = "skipped"
		result.SignatureErrors = append(result.SignatureErrors, "public key not configured")
	} else {
		valid := 0
		for _, sig := range manifest.Signatures {
			converted := sign.Signature{
				Alg:          sig.Alg,
				KeyID:        sig.KeyID,
				Sig:          sig.Sig,
				SignedDigest: sig.SignedDigest,
			}
			ok, err := sign.VerifyManifestJSON(opts.PublicKey, converted, signable)
			if err != nil {
				result.SignatureErrors = append(result.SignatureErrors, err.Error())
				continue
			}
			if ok {
				valid++
			} else {
				result.SignatureErrors = append(result.SignatureErrors, "signature verification failed")
			}
		}
		result.SignaturesValid = valid
		if valid > 0 {
			result.SignatureStatus = "verified"
		} else {
			result.SignatureStatus = "failed"
		}
	}

	sort.Strings(result.MissingFiles)
	sort.Slice(result.HashMismatches, func(leftIndex, rightIndex int) bool {
		return result.HashMismatches[leftIndex].Path < result.HashMismatches[rightIndex].Path
	})
	sort.Strings(result.SignatureErrors)

	return result, nil
}

func signableManifestBytes(manifest []byte) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(manifest, &obj); err != nil {
		return nil, err
	}
	delete(obj, "signatures")
	return json.Marshal(obj)
}

func readZipFile(zipFile *zip.File) ([]byte, error) {
	if zipFile.UncompressedSize64 > maxZipFileBytes {
		return nil, fmt.Errorf("zip entry too large: %d", zipFile.UncompressedSize64)
	}
	reader, err := zipFile.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	limitedReader := io.LimitReader(reader, maxZipFileBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxZipFileBytes {
		return nil, fmt.Errorf("zip entry exceeds max size")
	}
	return data, nil
}

func hashZipFile(zipFile *zip.File) (string, error) {
	if zipFile.UncompressedSize64 > maxZipFileBytes {
		return "", fmt.Errorf("zip entry too large: %d", zipFile.UncompressedSize64)
	}
	reader, err := zipFile.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()
	hashWriter := sha256.New()
	limitedReader := io.LimitReader(reader, maxZipFileBytes+1)
	bytesCopied, err := io.Copy(hashWriter, limitedReader)
	if err != nil {
		return "", err
	}
	if bytesCopied > maxZipFileBytes {
		return "", fmt.Errorf("zip entry exceeds max size")
	}
	return hex.EncodeToString(hashWriter.Sum(nil)), nil
}

func equalHex(first, second string) bool {
	return strings.EqualFold(first, second)
}
