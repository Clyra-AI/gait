package guard

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemaguard "github.com/davidahmann/gait/core/schema/v1/guard"
	schemaregress "github.com/davidahmann/gait/core/schema/v1/regress"
	schemascout "github.com/davidahmann/gait/core/schema/v1/scout"
	"github.com/davidahmann/gait/core/zipx"
)

type BuildOptions struct {
	RunpackPath             string
	OutputPath              string
	CaseID                  string
	InventoryPaths          []string
	TracePaths              []string
	RegressPaths            []string
	ApprovalAuditPaths      []string
	CredentialEvidencePaths []string
	AutoDiscoverV12         bool
	ProducerVersion         string
}

type BuildResult struct {
	PackPath string
	Manifest schemaguard.PackManifest
}

type VerifyResult struct {
	PackID         string         `json:"pack_id,omitempty"`
	RunID          string         `json:"run_id,omitempty"`
	FilesChecked   int            `json:"files_checked"`
	MissingFiles   []string       `json:"missing_files,omitempty"`
	HashMismatches []HashMismatch `json:"hash_mismatches,omitempty"`
}

type HashMismatch struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type Builder struct {
	ProducerVersion string
}

func (builder Builder) Build(_ context.Context, request BuildRequest) (schemaguard.PackManifest, error) {
	result, err := BuildPack(BuildOptions{
		RunpackPath:     request.RunpackZip,
		OutputPath:      request.OutputPath,
		CaseID:          request.CaseID,
		ProducerVersion: builder.ProducerVersion,
	})
	if err != nil {
		return schemaguard.PackManifest{}, err
	}
	return result.Manifest, nil
}

func BuildPack(options BuildOptions) (BuildResult, error) {
	if strings.TrimSpace(options.RunpackPath) == "" {
		return BuildResult{}, fmt.Errorf("runpack path is required")
	}
	runpackData, err := runpack.ReadRunpack(options.RunpackPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read runpack: %w", err)
	}

	evidenceFiles := map[string][]byte{}
	runpackSummary, err := buildRunpackSummary(runpackData)
	if err != nil {
		return BuildResult{}, err
	}
	evidenceFiles["runpack_summary.json"] = runpackSummary

	referencedSummary, err := buildReferencedRunpackSummary(runpackData)
	if err != nil {
		return BuildResult{}, err
	}
	evidenceFiles["referenced_runpacks.json"] = referencedSummary

	inventoryPayloads, err := readInventorySnapshots(options.InventoryPaths)
	if err != nil {
		return BuildResult{}, err
	}
	for index, payload := range inventoryPayloads {
		evidenceFiles[fmt.Sprintf("inventory_snapshot_%02d.json", index+1)] = payload
	}

	traceSummary, err := buildTraceSummary(options.TracePaths)
	if err != nil {
		return BuildResult{}, err
	}
	if len(traceSummary) > 0 {
		evidenceFiles["trace_summary.json"] = traceSummary
	}

	regressSummary, err := buildRegressSummary(options.RegressPaths)
	if err != nil {
		return BuildResult{}, err
	}
	if len(regressSummary) > 0 {
		evidenceFiles["regress_summary.json"] = regressSummary
	}

	approvalAuditPaths := options.ApprovalAuditPaths
	credentialEvidencePaths := options.CredentialEvidencePaths
	if options.AutoDiscoverV12 || (len(approvalAuditPaths) == 0 && len(credentialEvidencePaths) == 0) {
		discoveredAudits, discoveredCredentials, err := discoverV12EvidencePaths(filepath.Dir(options.RunpackPath))
		if err != nil {
			return BuildResult{}, err
		}
		if len(approvalAuditPaths) == 0 {
			approvalAuditPaths = discoveredAudits
		}
		if len(credentialEvidencePaths) == 0 {
			credentialEvidencePaths = discoveredCredentials
		}
	}

	approvalAudits, err := readApprovalAuditRecords(approvalAuditPaths)
	if err != nil {
		return BuildResult{}, err
	}
	for index, payload := range approvalAudits {
		evidenceFiles[fmt.Sprintf("approval_audit_%02d.json", index+1)] = payload
	}

	credentialEvidence, err := readBrokerCredentialRecords(credentialEvidencePaths)
	if err != nil {
		return BuildResult{}, err
	}
	for index, payload := range credentialEvidence {
		evidenceFiles[fmt.Sprintf("credential_evidence_%02d.json", index+1)] = payload
	}

	contents := make([]schemaguard.PackEntry, 0, len(evidenceFiles))
	for path, data := range evidenceFiles {
		contents = append(contents, schemaguard.PackEntry{
			Path:   path,
			SHA256: sha256Hex(data),
			Type:   inferPackEntryType(path),
		})
	}
	sort.Slice(contents, func(i, j int) bool {
		return contents[i].Path < contents[j].Path
	})

	manifestTime := runpackData.Run.CreatedAt.UTC()
	if manifestTime.IsZero() {
		manifestTime = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(options.ProducerVersion)
	if producerVersion == "" {
		producerVersion = runpackData.Run.ProducerVersion
	}
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	packID, err := computePackID(runpackData.Run.RunID, contents)
	if err != nil {
		return BuildResult{}, fmt.Errorf("compute pack id: %w", err)
	}
	manifest := schemaguard.PackManifest{
		SchemaID:        "gait.guard.pack_manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       manifestTime,
		ProducerVersion: producerVersion,
		PackID:          packID,
		RunID:           runpackData.Run.RunID,
		CaseID:          strings.TrimSpace(options.CaseID),
		GeneratedAt:     manifestTime,
		Contents:        contents,
	}

	manifestBytes, err := marshalCanonicalJSON(manifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode pack_manifest.json: %w", err)
	}
	evidenceFiles["pack_manifest.json"] = manifestBytes

	zipFiles := make([]zipx.File, 0, len(evidenceFiles))
	for path, data := range evidenceFiles {
		zipFiles = append(zipFiles, zipx.File{
			Path: path,
			Data: data,
			Mode: 0o644,
		})
	}
	sort.Slice(zipFiles, func(i, j int) bool {
		return zipFiles[i].Path < zipFiles[j].Path
	})

	var zipBuffer bytes.Buffer
	if err := zipx.WriteDeterministicZip(&zipBuffer, zipFiles); err != nil {
		return BuildResult{}, fmt.Errorf("write evidence pack zip: %w", err)
	}

	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(options.RunpackPath), fmt.Sprintf("evidence_pack_%s.zip", packID))
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return BuildResult{}, fmt.Errorf("mkdir output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, zipBuffer.Bytes(), 0o600); err != nil {
		return BuildResult{}, fmt.Errorf("write evidence pack: %w", err)
	}

	return BuildResult{
		PackPath: outputPath,
		Manifest: manifest,
	}, nil
}

func VerifyPack(path string) (VerifyResult, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("open evidence pack zip: %w", err)
	}
	defer func() {
		_ = zipReader.Close()
	}()

	files := make(map[string]*zip.File, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		files[zipFile.Name] = zipFile
	}

	manifestFile := files["pack_manifest.json"]
	if manifestFile == nil {
		return VerifyResult{}, fmt.Errorf("missing pack_manifest.json")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("read pack_manifest.json: %w", err)
	}
	var manifest schemaguard.PackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return VerifyResult{}, fmt.Errorf("parse pack manifest: %w", err)
	}

	result := VerifyResult{
		PackID:       manifest.PackID,
		RunID:        manifest.RunID,
		FilesChecked: len(manifest.Contents),
	}
	for _, entry := range manifest.Contents {
		zipFile := files[entry.Path]
		if zipFile == nil {
			result.MissingFiles = append(result.MissingFiles, entry.Path)
			continue
		}
		actualHash, err := hashZipFile(zipFile)
		if err != nil {
			return VerifyResult{}, fmt.Errorf("hash %s: %w", entry.Path, err)
		}
		if !strings.EqualFold(actualHash, entry.SHA256) {
			result.HashMismatches = append(result.HashMismatches, HashMismatch{
				Path:     entry.Path,
				Expected: entry.SHA256,
				Actual:   actualHash,
			})
		}
	}
	sort.Strings(result.MissingFiles)
	sort.Slice(result.HashMismatches, func(i, j int) bool {
		return result.HashMismatches[i].Path < result.HashMismatches[j].Path
	})
	return result, nil
}

func buildRunpackSummary(data runpack.Runpack) ([]byte, error) {
	summary := struct {
		RunID          string `json:"run_id"`
		ManifestDigest string `json:"manifest_digest"`
		CaptureMode    string `json:"capture_mode"`
		IntentCount    int    `json:"intent_count"`
		ResultCount    int    `json:"result_count"`
		ReceiptCount   int    `json:"receipt_count"`
	}{
		RunID:          data.Run.RunID,
		ManifestDigest: data.Manifest.ManifestDigest,
		CaptureMode:    data.Manifest.CaptureMode,
		IntentCount:    len(data.Intents),
		ResultCount:    len(data.Results),
		ReceiptCount:   len(data.Refs.Receipts),
	}
	return marshalCanonicalJSON(summary)
}

func buildReferencedRunpackSummary(data runpack.Runpack) ([]byte, error) {
	compact := make([]map[string]any, 0, len(data.Refs.Receipts))
	for _, receipt := range data.Refs.Receipts {
		compact = append(compact, map[string]any{
			"ref_id":         receipt.RefID,
			"source_type":    receipt.SourceType,
			"source_locator": receipt.SourceLocator,
			"content_digest": receipt.ContentDigest,
			"retrieved_at":   receipt.RetrievedAt.UTC(),
		})
	}
	sort.Slice(compact, func(i, j int) bool {
		left := fmt.Sprintf("%v", compact[i]["ref_id"])
		right := fmt.Sprintf("%v", compact[j]["ref_id"])
		return left < right
	})
	payload := map[string]any{
		"run_id":     data.Run.RunID,
		"receipts":   compact,
		"references": len(compact),
	}
	return marshalCanonicalJSON(payload)
}

func readInventorySnapshots(paths []string) ([][]byte, error) {
	normalized := normalizePaths(paths)
	if len(normalized) == 0 {
		return nil, nil
	}
	out := make([][]byte, 0, len(normalized))
	for _, path := range normalized {
		// #nosec G304 -- user-supplied local file path.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read inventory snapshot %s: %w", path, err)
		}
		var snapshot schemascout.InventorySnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return nil, fmt.Errorf("parse inventory snapshot %s: %w", path, err)
		}
		encoded, err := marshalCanonicalJSON(snapshot)
		if err != nil {
			return nil, fmt.Errorf("encode inventory snapshot %s: %w", path, err)
		}
		out = append(out, encoded)
	}
	return out, nil
}

func buildTraceSummary(paths []string) ([]byte, error) {
	normalized := normalizePaths(paths)
	if len(normalized) == 0 {
		return nil, nil
	}
	type traceLine struct {
		TraceID string `json:"trace_id"`
		Tool    string `json:"tool_name"`
		Verdict string `json:"verdict"`
	}
	lines := make([]traceLine, 0, len(normalized))
	verdictCounts := map[string]int{}
	for _, path := range normalized {
		record, err := readTraceRecord(path)
		if err != nil {
			return nil, fmt.Errorf("read trace %s: %w", path, err)
		}
		lines = append(lines, traceLine{
			TraceID: record.TraceID,
			Tool:    record.ToolName,
			Verdict: record.Verdict,
		})
		verdictCounts[record.Verdict]++
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].TraceID < lines[j].TraceID
	})
	payload := struct {
		Traces        []traceLine    `json:"traces"`
		VerdictCounts map[string]int `json:"verdict_counts"`
		Total         int            `json:"total"`
	}{
		Traces:        lines,
		VerdictCounts: verdictCounts,
		Total:         len(lines),
	}
	return marshalCanonicalJSON(payload)
}

func readTraceRecord(path string) (schemagate.TraceRecord, error) {
	// #nosec G304 -- user-supplied local file path.
	raw, err := os.ReadFile(path)
	if err != nil {
		return schemagate.TraceRecord{}, err
	}
	var record schemagate.TraceRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return schemagate.TraceRecord{}, err
	}
	return record, nil
}

func buildRegressSummary(paths []string) ([]byte, error) {
	normalized := normalizePaths(paths)
	if len(normalized) == 0 {
		return nil, nil
	}
	type regressLine struct {
		FixtureSet string `json:"fixture_set"`
		Status     string `json:"status"`
		Graders    int    `json:"graders"`
	}
	lines := make([]regressLine, 0, len(normalized))
	statusCounts := map[string]int{}
	for _, path := range normalized {
		// #nosec G304 -- user-supplied local file path.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read regress result %s: %w", path, err)
		}
		var result schemaregress.RegressResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("parse regress result %s: %w", path, err)
		}
		lines = append(lines, regressLine{
			FixtureSet: result.FixtureSet,
			Status:     result.Status,
			Graders:    len(result.Graders),
		})
		statusCounts[result.Status]++
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].FixtureSet != lines[j].FixtureSet {
			return lines[i].FixtureSet < lines[j].FixtureSet
		}
		return lines[i].Status < lines[j].Status
	})
	payload := struct {
		Results      []regressLine  `json:"results"`
		StatusCounts map[string]int `json:"status_counts"`
		Total        int            `json:"total"`
	}{
		Results:      lines,
		StatusCounts: statusCounts,
		Total:        len(lines),
	}
	return marshalCanonicalJSON(payload)
}

func discoverV12EvidencePaths(rootDir string) ([]string, []string, error) {
	approvalAuditPaths, err := filepath.Glob(filepath.Join(rootDir, "approval_audit_*.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("discover approval audits: %w", err)
	}
	credentialEvidencePaths, err := filepath.Glob(filepath.Join(rootDir, "credential_evidence_*.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("discover credential evidence: %w", err)
	}
	return uniqueSortedStrings(approvalAuditPaths), uniqueSortedStrings(credentialEvidencePaths), nil
}

func readApprovalAuditRecords(paths []string) ([][]byte, error) {
	normalized := normalizePaths(paths)
	if len(normalized) == 0 {
		return nil, nil
	}
	out := make([][]byte, 0, len(normalized))
	for _, path := range normalized {
		// #nosec G304 -- user-supplied local file path.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read approval audit %s: %w", path, err)
		}
		var record schemagate.ApprovalAuditRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("parse approval audit %s: %w", path, err)
		}
		encoded, err := marshalCanonicalJSON(record)
		if err != nil {
			return nil, fmt.Errorf("encode approval audit %s: %w", path, err)
		}
		out = append(out, encoded)
	}
	return out, nil
}

func readBrokerCredentialRecords(paths []string) ([][]byte, error) {
	normalized := normalizePaths(paths)
	if len(normalized) == 0 {
		return nil, nil
	}
	out := make([][]byte, 0, len(normalized))
	for _, path := range normalized {
		// #nosec G304 -- user-supplied local file path.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read broker credential record %s: %w", path, err)
		}
		var record schemagate.BrokerCredentialRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("parse broker credential record %s: %w", path, err)
		}
		encoded, err := marshalCanonicalJSON(record)
		if err != nil {
			return nil, fmt.Errorf("encode broker credential record %s: %w", path, err)
		}
		out = append(out, encoded)
	}
	return out, nil
}

func normalizePaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		for _, segment := range strings.Split(path, ",") {
			trimmed := strings.TrimSpace(segment)
			if trimmed == "" {
				continue
			}
			normalized = append(normalized, trimmed)
		}
	}
	return uniqueSortedStrings(normalized)
}

func inferPackEntryType(path string) string {
	switch path {
	case "runpack_summary.json":
		return "runpack"
	case "trace_summary.json":
		return "trace"
	case "regress_summary.json":
		return "report"
	default:
		return "evidence"
	}
}

func uniqueSortedStrings(values []string) []string {
	sort.Strings(values)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == value {
			continue
		}
		out = append(out, value)
	}
	return out
}

func computePackID(runID string, contents []schemaguard.PackEntry) (string, error) {
	payload := struct {
		RunID    string                  `json:"run_id"`
		Contents []schemaguard.PackEntry `json:"contents"`
	}{
		RunID:    runID,
		Contents: contents,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	return "pack_" + digest[:16], nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jcs.CanonicalizeJSON(raw)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const maxEvidenceZipEntryBytes = 100 * 1024 * 1024

func readZipFile(zipFile *zip.File) ([]byte, error) {
	if zipFile.UncompressedSize64 > maxEvidenceZipEntryBytes {
		return nil, fmt.Errorf("zip entry too large: %d", zipFile.UncompressedSize64)
	}
	reader, err := zipFile.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	limitedReader := io.LimitReader(reader, maxEvidenceZipEntryBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxEvidenceZipEntryBytes {
		return nil, fmt.Errorf("zip entry exceeds max size")
	}
	return data, nil
}

func hashZipFile(zipFile *zip.File) (string, error) {
	if zipFile.UncompressedSize64 > maxEvidenceZipEntryBytes {
		return "", fmt.Errorf("zip entry too large: %d", zipFile.UncompressedSize64)
	}
	reader, err := zipFile.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()
	limitedReader := io.LimitReader(reader, maxEvidenceZipEntryBytes+1)
	hashWriter := sha256.New()
	copied, err := io.Copy(hashWriter, limitedReader)
	if err != nil {
		return "", err
	}
	if copied > maxEvidenceZipEntryBytes {
		return "", fmt.Errorf("zip entry exceeds max size")
	}
	return hex.EncodeToString(hashWriter.Sum(nil)), nil
}
