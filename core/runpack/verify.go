package runpack

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/zipx"
	sign "github.com/Clyra-AI/proof/signing"
)

type VerifyOptions struct {
	PublicKey               ed25519.PublicKey
	RequireSignature        bool
	SkipManifestDigestCheck bool
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
const maxVerifyZipReadAllBytes = 8 * 1024 * 1024

var zipCopyBufferPool = sync.Pool{
	New: func() any {
		buffer := make([]byte, 32*1024)
		return &buffer
	},
}

func VerifyZip(path string, opts VerifyOptions) (VerifyResult, error) {
	zipFiles, closeZip, err := openZipFiles(path)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("open zip: %w", err)
	}
	defer func() {
		_ = closeZip()
	}()
	if err := rejectDuplicateZipEntries(zipFiles); err != nil {
		return VerifyResult{}, err
	}
	filesByPath := indexZipFiles(zipFiles)

	manifestFile, manifestFound := filesByPath["manifest.json"]
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

	hasRun := false
	hasIntents := false
	hasResults := false
	hasRefs := false
	for _, entry := range manifest.Files {
		name := filepath.ToSlash(entry.Path)
		switch name {
		case "run.json":
			hasRun = true
		case "intents.jsonl":
			hasIntents = true
		case "results.jsonl":
			hasResults = true
		case "refs.json":
			hasRefs = true
		}
		zipFile, exists := filesByPath[name]
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
	if !hasRun {
		result.MissingFiles = append(result.MissingFiles, "run.json")
	}
	if !hasIntents {
		result.MissingFiles = append(result.MissingFiles, "intents.jsonl")
	}
	if !hasResults {
		result.MissingFiles = append(result.MissingFiles, "results.jsonl")
	}
	if !hasRefs {
		result.MissingFiles = append(result.MissingFiles, "refs.json")
	}

	if !opts.SkipManifestDigestCheck {
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
		signable, err := signableManifestBytes(manifestBytes)
		if err != nil {
			return VerifyResult{}, fmt.Errorf("prepare manifest for signing: %w", err)
		}
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

func openZipFiles(path string) ([]*zip.File, func() error, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	root, err := os.OpenRoot(dir)
	if err == nil {
		defer func() {
			_ = root.Close()
		}()
		info, statErr := root.Stat(name)
		if statErr == nil && info.Size() > 0 && info.Size() <= maxVerifyZipReadAllBytes {
			file, err := root.Open(name)
			if err != nil {
				return nil, nil, err
			}
			zipBytes, err := readWithByteLimit(file, maxVerifyZipReadAllBytes)
			_ = file.Close()
			if err != nil {
				return nil, nil, err
			}
			if zipBytes == nil {
				goto fallback
			}
			reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
			if err != nil {
				return nil, nil, err
			}
			return reader.File, func() error { return nil }, nil
		}
	}

fallback:
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	return zipReader.File, zipReader.Close, nil
}

func readWithByteLimit(reader io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(reader, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, nil
	}
	return data, nil
}

func rejectDuplicateZipEntries(files []*zip.File) error {
	duplicates := zipx.DuplicatePaths(files)
	if len(duplicates) == 0 {
		return nil
	}
	return coreerrors.Wrap(
		fmt.Errorf("zip contains duplicate entries: %s", strings.Join(duplicates, ", ")),
		coreerrors.CategoryVerification,
		"runpack_duplicate_entries",
		"rebuild the artifact so each zip path is unique",
		false,
	)
}

func signableManifestBytes(manifest []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(manifest, &obj); err != nil {
		return nil, err
	}
	delete(obj, "signatures")
	return json.Marshal(obj)
}

func indexZipFiles(files []*zip.File) map[string]*zip.File {
	index := make(map[string]*zip.File, len(files))
	for _, zipFile := range files {
		index[filepath.ToSlash(zipFile.Name)] = zipFile
	}
	return index
}

func findZipFile(files []*zip.File, name string) (*zip.File, bool) {
	for _, zipFile := range files {
		if filepath.ToSlash(zipFile.Name) == name {
			return zipFile, true
		}
	}
	return nil, false
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
	copyBuffer := zipCopyBufferPool.Get().(*[]byte)
	defer zipCopyBufferPool.Put(copyBuffer)
	bytesCopied, err := io.CopyBuffer(hashWriter, limitedReader, *copyBuffer)
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
