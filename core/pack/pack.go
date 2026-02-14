package pack

import (
	"archive/zip"
	"bufio"
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
	"time"

	"github.com/davidahmann/gait/core/contextproof"
	coreerrors "github.com/davidahmann/gait/core/errors"
	"github.com/davidahmann/gait/core/guard"
	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/jobruntime"
	"github.com/davidahmann/gait/core/runpack"
	schemaguard "github.com/davidahmann/gait/core/schema/v1/guard"
	schemapack "github.com/davidahmann/gait/core/schema/v1/pack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
	"github.com/davidahmann/gait/core/zipx"
)

const (
	manifestSchemaID      = "gait.pack.manifest"
	manifestSchemaVersion = "1.0.0"
	diffSchemaID          = "gait.pack.diff"
	diffSchemaVersion     = "1.0.0"
	manifestFileName      = "pack_manifest.json"
	maxZipEntryBytes      = int64(100 * 1024 * 1024)
)

var deterministicTimestamp = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

type BuildType string

const (
	BuildTypeRun BuildType = "run"
	BuildTypeJob BuildType = "job"
)

type BuildRunOptions struct {
	RunpackPath       string
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
}

type BuildJobOptions struct {
	State             jobruntime.JobState
	Events            []jobruntime.Event
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
}

type BuildResult struct {
	Path     string
	Manifest schemapack.Manifest
}

type VerifyOptions struct {
	PublicKey        ed25519.PublicKey
	RequireSignature bool
}

type HashMismatch struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type VerifyResult struct {
	PackID          string         `json:"pack_id,omitempty"`
	PackType        string         `json:"pack_type,omitempty"`
	SourceRef       string         `json:"source_ref,omitempty"`
	FilesChecked    int            `json:"files_checked"`
	MissingFiles    []string       `json:"missing_files,omitempty"`
	HashMismatches  []HashMismatch `json:"hash_mismatches,omitempty"`
	UndeclaredFiles []string       `json:"undeclared_files,omitempty"`
	SignatureStatus string         `json:"signature_status,omitempty"`
	SignatureErrors []string       `json:"signature_errors,omitempty"`
	SignaturesTotal int            `json:"signatures_total,omitempty"`
	SignaturesValid int            `json:"signatures_valid,omitempty"`
	LegacyType      string         `json:"legacy_type,omitempty"`
}

type InspectResult struct {
	PackID     string                 `json:"pack_id,omitempty"`
	PackType   string                 `json:"pack_type,omitempty"`
	SourceRef  string                 `json:"source_ref,omitempty"`
	Manifest   *schemapack.Manifest   `json:"manifest,omitempty"`
	RunPayload *schemapack.RunPayload `json:"run_payload,omitempty"`
	JobPayload *schemapack.JobPayload `json:"job_payload,omitempty"`
	RunLineage *RunLineage            `json:"run_lineage,omitempty"`
	JobLineage *JobLineage            `json:"job_lineage,omitempty"`
	LegacyType string                 `json:"legacy_type,omitempty"`
}

type DiffResult struct {
	Result schemapack.DiffResult
}

type RunLineage struct {
	TimelineEvents int                   `json:"timeline_events"`
	ReceiptCount   int                   `json:"receipt_count"`
	IntentResults  []RunIntentResultLink `json:"intent_results,omitempty"`
}

type RunIntentResultLink struct {
	IntentID string `json:"intent_id"`
	ToolName string `json:"tool_name,omitempty"`
	Status   string `json:"status,omitempty"`
}

type JobLineage struct {
	EventCount     int                `json:"event_count"`
	LastEventType  string             `json:"last_event_type,omitempty"`
	CheckpointRefs []JobCheckpointRef `json:"checkpoint_refs,omitempty"`
}

type JobCheckpointRef struct {
	CheckpointID string `json:"checkpoint_id"`
	Type         string `json:"type"`
	ReasonCode   string `json:"reason_code"`
}

func BuildRunPack(options BuildRunOptions) (BuildResult, error) {
	runpackPath := strings.TrimSpace(options.RunpackPath)
	if runpackPath == "" {
		return BuildResult{}, fmt.Errorf("runpack path is required")
	}
	data, err := runpack.ReadRunpack(runpackPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read runpack: %w", err)
	}
	// #nosec G304 -- caller provides local path.
	rawRunpack, err := os.ReadFile(runpackPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read runpack bytes: %w", err)
	}

	payload := schemapack.RunPayload{
		SchemaID:            "gait.pack.run",
		SchemaVersion:       "1.0.0",
		CreatedAt:           normalizeTime(data.Run.CreatedAt),
		RunID:               data.Run.RunID,
		CaptureMode:         data.Manifest.CaptureMode,
		ManifestDigest:      data.Manifest.ManifestDigest,
		IntentsCount:        len(data.Intents),
		ResultsCount:        len(data.Results),
		RefsCount:           len(data.Refs.Receipts),
		ContextSetDigest:    data.Refs.ContextSetDigest,
		ContextRefCount:     len(data.Refs.Receipts),
		ContextEvidenceMode: data.Refs.ContextEvidenceMode,
		ContextPrivacyMode:  detectContextPrivacyMode(data.Refs.Receipts),
	}
	payloadBytes, err := canonicalJSON(payload)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode run payload: %w", err)
	}

	files := []zipx.File{
		{Path: "run_payload.json", Data: payloadBytes, Mode: 0o644},
		{Path: "source/runpack.zip", Data: rawRunpack, Mode: 0o644},
	}
	if envelope, ok, envelopeErr := contextproof.EnvelopeFromRefs(data.Refs); envelopeErr != nil {
		return BuildResult{}, fmt.Errorf("build context envelope: %w", envelopeErr)
	} else if ok {
		envelopeBytes, encodeErr := canonicalJSON(envelope)
		if encodeErr != nil {
			return BuildResult{}, fmt.Errorf("encode context envelope: %w", encodeErr)
		}
		files = append(files, zipx.File{Path: "context_envelope.json", Data: envelopeBytes, Mode: 0o644})
	}

	return buildPackWithFiles(buildPackOptions{
		PackType:          string(BuildTypeRun),
		SourceRef:         data.Run.RunID,
		OutputPath:        options.OutputPath,
		ProducerVersion:   options.ProducerVersion,
		SigningPrivateKey: options.SigningPrivateKey,
		Files:             files,
		OutputDirFallback: filepath.Dir(runpackPath),
	})
}

func BuildJobPack(options BuildJobOptions) (BuildResult, error) {
	state := options.State
	if strings.TrimSpace(state.JobID) == "" {
		return BuildResult{}, fmt.Errorf("job state job_id is required")
	}
	if strings.TrimSpace(state.Status) == "" {
		state.Status = jobruntime.StatusRunning
	}
	if strings.TrimSpace(state.StopReason) == "" {
		state.StopReason = jobruntime.StopReasonNone
	}
	if strings.TrimSpace(state.StatusReasonCode) == "" {
		state.StatusReasonCode = "submitted"
	}
	if strings.TrimSpace(state.EnvironmentFingerprint) == "" {
		state.EnvironmentFingerprint = jobruntime.EnvironmentFingerprint("")
	}
	payload := schemapack.JobPayload{
		SchemaID:               "gait.pack.job",
		SchemaVersion:          "1.0.0",
		CreatedAt:              normalizeTime(state.CreatedAt),
		JobID:                  state.JobID,
		Status:                 state.Status,
		StopReason:             state.StopReason,
		StatusReasonCode:       state.StatusReasonCode,
		EnvironmentFingerprint: state.EnvironmentFingerprint,
		CheckpointCount:        len(state.Checkpoints),
		ApprovalCount:          len(state.Approvals),
	}
	payloadBytes, err := canonicalJSON(payload)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode job payload: %w", err)
	}
	stateBytes, err := canonicalJSON(state)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode job state: %w", err)
	}
	eventsBytes, err := canonicalJSONL(options.Events)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode job events: %w", err)
	}

	files := []zipx.File{
		{Path: "job_payload.json", Data: payloadBytes, Mode: 0o644},
		{Path: "job_state.json", Data: stateBytes, Mode: 0o644},
		{Path: "job_events.jsonl", Data: eventsBytes, Mode: 0o644},
	}

	return buildPackWithFiles(buildPackOptions{
		PackType:          string(BuildTypeJob),
		SourceRef:         state.JobID,
		OutputPath:        options.OutputPath,
		ProducerVersion:   options.ProducerVersion,
		SigningPrivateKey: options.SigningPrivateKey,
		Files:             files,
		OutputDirFallback: filepath.Join(".", "gait-out"),
	})
}

type buildPackOptions struct {
	PackType          string
	SourceRef         string
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
	Files             []zipx.File
	OutputDirFallback string
}

func buildPackWithFiles(options buildPackOptions) (BuildResult, error) {
	if options.PackType != string(BuildTypeRun) && options.PackType != string(BuildTypeJob) {
		return BuildResult{}, fmt.Errorf("unsupported pack type: %s", options.PackType)
	}
	createdAt := deterministicTimestamp
	producerVersion := strings.TrimSpace(options.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	contents := make([]schemapack.PackEntry, 0, len(options.Files))
	for _, file := range options.Files {
		contents = append(contents, schemapack.PackEntry{
			Path:   file.Path,
			SHA256: sha256Hex(file.Data),
			Type:   detectEntryType(file.Path),
		})
	}
	sort.Slice(contents, func(i, j int) bool { return contents[i].Path < contents[j].Path })

	manifest := schemapack.Manifest{
		SchemaID:        manifestSchemaID,
		SchemaVersion:   manifestSchemaVersion,
		CreatedAt:       createdAt,
		ProducerVersion: producerVersion,
		PackID:          "",
		PackType:        options.PackType,
		SourceRef:       strings.TrimSpace(options.SourceRef),
		Contents:        contents,
	}

	packID, err := computePackID(manifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("compute pack id: %w", err)
	}
	manifest.PackID = packID

	if len(options.SigningPrivateKey) > 0 {
		signable := manifest
		signable.Signatures = nil
		signableBytes, err := canonicalJSON(signable)
		if err != nil {
			return BuildResult{}, fmt.Errorf("encode signable manifest: %w", err)
		}
		sig, err := sign.SignJSON(options.SigningPrivateKey, signableBytes)
		if err != nil {
			return BuildResult{}, fmt.Errorf("sign manifest: %w", err)
		}
		manifest.Signatures = []schemapack.Signature{{
			Alg:          sig.Alg,
			KeyID:        sig.KeyID,
			Sig:          sig.Sig,
			SignedDigest: sig.SignedDigest,
		}}
	}

	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode manifest: %w", err)
	}

	files := append([]zipx.File{}, options.Files...)
	files = append(files, zipx.File{Path: manifestFileName, Data: manifestBytes, Mode: 0o644})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	var buffer bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buffer, files); err != nil {
		return BuildResult{}, fmt.Errorf("write pack zip: %w", err)
	}

	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		baseDir := strings.TrimSpace(options.OutputDirFallback)
		if baseDir == "" {
			baseDir = filepath.Join(".", "gait-out")
		}
		outputPath = filepath.Join(baseDir, "pack_"+manifest.PackID+".zip")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return BuildResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, buffer.Bytes(), 0o600); err != nil {
		return BuildResult{}, fmt.Errorf("write pack: %w", err)
	}
	return BuildResult{Path: outputPath, Manifest: manifest}, nil
}

func Verify(path string, options VerifyOptions) (VerifyResult, error) {
	bundle, err := openZip(path)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if _, ok := bundle.Files[manifestFileName]; !ok {
		if _, runpackManifest := bundle.Files["manifest.json"]; runpackManifest {
			legacy, err := runpack.VerifyZip(path, runpack.VerifyOptions{PublicKey: options.PublicKey, RequireSignature: options.RequireSignature})
			if err != nil {
				return VerifyResult{}, err
			}
			return VerifyResult{
				PackID:          legacy.ManifestDigest,
				PackType:        string(BuildTypeRun),
				SourceRef:       legacy.RunID,
				FilesChecked:    legacy.FilesChecked,
				MissingFiles:    legacy.MissingFiles,
				HashMismatches:  convertRunpackMismatches(legacy.HashMismatches),
				SignatureStatus: legacy.SignatureStatus,
				SignatureErrors: legacy.SignatureErrors,
				SignaturesTotal: legacy.SignaturesTotal,
				SignaturesValid: legacy.SignaturesValid,
				LegacyType:      "runpack",
			}, nil
		}
		return VerifyResult{}, fmt.Errorf("missing %s", manifestFileName)
	}

	manifestBytes, err := readZipFile(bundle.Files[manifestFileName])
	if err != nil {
		return VerifyResult{}, fmt.Errorf("read %s: %w", manifestFileName, err)
	}
	manifest, err := parsePackManifest(manifestBytes)
	if err != nil {
		var guardManifest schemaguard.PackManifest
		if json.Unmarshal(manifestBytes, &guardManifest) == nil && guardManifest.SchemaID == "gait.guard.pack_manifest" {
			legacy, verifyErr := guard.VerifyPackWithOptions(path, guard.VerifyOptions{PublicKey: options.PublicKey, RequireSignature: options.RequireSignature})
			if verifyErr != nil {
				return VerifyResult{}, verifyErr
			}
			return VerifyResult{
				PackID:          legacy.PackID,
				PackType:        "guard",
				SourceRef:       legacy.RunID,
				FilesChecked:    legacy.FilesChecked,
				MissingFiles:    legacy.MissingFiles,
				HashMismatches:  convertGuardMismatches(legacy.HashMismatches),
				SignatureStatus: legacy.SignatureStatus,
				SignatureErrors: legacy.SignatureErrors,
				SignaturesTotal: legacy.SignaturesTotal,
				SignaturesValid: legacy.SignaturesValid,
				LegacyType:      "guard",
			}, nil
		}
		return VerifyResult{}, verificationError(fmt.Errorf("parse manifest: %w", err))
	}
	expectedPackID, err := computePackID(manifest)
	if err != nil {
		return VerifyResult{}, verificationError(fmt.Errorf("compute pack id: %w", err))
	}
	if !strings.EqualFold(expectedPackID, manifest.PackID) {
		return VerifyResult{}, verificationError(fmt.Errorf("pack_id mismatch: expected=%s actual=%s", expectedPackID, manifest.PackID))
	}

	result := VerifyResult{
		PackID:          manifest.PackID,
		PackType:        manifest.PackType,
		SourceRef:       manifest.SourceRef,
		FilesChecked:    len(manifest.Contents),
		SignatureStatus: "missing",
		SignaturesTotal: len(manifest.Signatures),
	}

	declared := make(map[string]schemapack.PackEntry, len(manifest.Contents))
	for _, entry := range manifest.Contents {
		declared[entry.Path] = entry
		zipFile, ok := bundle.Files[entry.Path]
		if !ok {
			result.MissingFiles = append(result.MissingFiles, entry.Path)
			continue
		}
		actual, hashErr := hashZipFile(zipFile)
		if hashErr != nil {
			return VerifyResult{}, fmt.Errorf("hash %s: %w", entry.Path, hashErr)
		}
		if !strings.EqualFold(actual, entry.SHA256) {
			result.HashMismatches = append(result.HashMismatches, HashMismatch{Path: entry.Path, Expected: entry.SHA256, Actual: actual})
		}
	}

	for path := range bundle.Files {
		if path == manifestFileName {
			continue
		}
		if _, ok := declared[path]; !ok {
			result.UndeclaredFiles = append(result.UndeclaredFiles, path)
		}
	}

	if len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0 && len(result.UndeclaredFiles) == 0 {
		if err := verifyPayloadContracts(bundle, manifest); err != nil {
			return VerifyResult{}, verificationError(err)
		}
	}

	signable := manifest
	signable.Signatures = nil
	signableBytes, err := canonicalJSON(signable)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("encode signable manifest: %w", err)
	}
	if len(manifest.Signatures) == 0 {
		result.SignatureStatus = "missing"
		if options.RequireSignature {
			result.SignatureErrors = append(result.SignatureErrors, "manifest has no signatures")
		}
	} else if options.PublicKey == nil {
		result.SignatureStatus = "skipped"
		result.SignatureErrors = append(result.SignatureErrors, "public key not configured")
	} else {
		valid := 0
		for _, sig := range manifest.Signatures {
			ok, verifyErr := sign.VerifyJSON(options.PublicKey, sign.Signature{Alg: sig.Alg, KeyID: sig.KeyID, Sig: sig.Sig, SignedDigest: sig.SignedDigest}, signableBytes)
			if verifyErr != nil {
				result.SignatureErrors = append(result.SignatureErrors, verifyErr.Error())
				continue
			}
			if ok {
				valid++
			}
		}
		result.SignaturesValid = valid
		if valid > 0 {
			result.SignatureStatus = "verified"
		} else {
			result.SignatureStatus = "failed"
			result.SignatureErrors = append(result.SignatureErrors, "signature verification failed")
		}
	}

	sort.Strings(result.MissingFiles)
	sort.Strings(result.UndeclaredFiles)
	sort.Slice(result.HashMismatches, func(i, j int) bool { return result.HashMismatches[i].Path < result.HashMismatches[j].Path })
	sort.Strings(result.SignatureErrors)
	return result, nil
}

func Diff(leftPath string, rightPath string) (DiffResult, error) {
	leftMeta, err := collectArtifactInfo(leftPath)
	if err != nil {
		return DiffResult{}, err
	}
	rightMeta, err := collectArtifactInfo(rightPath)
	if err != nil {
		return DiffResult{}, err
	}

	leftSet := make(map[string]struct{}, len(leftMeta.Files))
	for key := range leftMeta.Files {
		leftSet[key] = struct{}{}
	}
	rightSet := make(map[string]struct{}, len(rightMeta.Files))
	for key := range rightMeta.Files {
		rightSet[key] = struct{}{}
	}

	added := make([]string, 0)
	for _, key := range sortedKeys(rightMeta.Files) {
		if _, ok := leftSet[key]; !ok {
			added = append(added, key)
		}
	}
	removed := make([]string, 0)
	changed := make([]string, 0)
	for _, key := range sortedKeys(leftMeta.Files) {
		if _, ok := rightSet[key]; !ok {
			removed = append(removed, key)
			continue
		}
		if leftMeta.Files[key] != rightMeta.Files[key] {
			changed = append(changed, key)
		}
	}

	manifestDelta := leftMeta.ManifestDigest != rightMeta.ManifestDigest || leftMeta.PackType != rightMeta.PackType
	contextDriftClass := "none"
	contextChanged := false
	contextRuntimeOnly := false
	if leftMeta.PackType == string(BuildTypeRun) && rightMeta.PackType == string(BuildTypeRun) {
		switch {
		case leftMeta.ContextRefs != nil && rightMeta.ContextRefs != nil:
			classification, changedFlag, runtimeOnlyFlag, classifyErr := contextproof.ClassifyRefsDrift(*leftMeta.ContextRefs, *rightMeta.ContextRefs)
			if classifyErr != nil {
				return DiffResult{}, fmt.Errorf("classify context drift: %w", classifyErr)
			}
			contextDriftClass = classification
			contextChanged = changedFlag
			contextRuntimeOnly = runtimeOnlyFlag
		case leftMeta.ContextSetDigest != "" || rightMeta.ContextSetDigest != "" || leftMeta.ContextRefCount > 0 || rightMeta.ContextRefCount > 0:
			if leftMeta.ContextSetDigest == rightMeta.ContextSetDigest &&
				leftMeta.ContextEvidenceMode == rightMeta.ContextEvidenceMode &&
				leftMeta.ContextRefCount == rightMeta.ContextRefCount {
				contextDriftClass = "none"
			} else {
				contextDriftClass = "semantic"
				contextChanged = true
			}
		}
	}
	result := schemapack.DiffResult{
		SchemaID:      diffSchemaID,
		SchemaVersion: diffSchemaVersion,
		CreatedAt:     time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC),
		LeftPackID:    leftMeta.PackID,
		RightPackID:   rightMeta.PackID,
		LeftPackType:  leftMeta.PackType,
		RightPackType: rightMeta.PackType,
		Summary: schemapack.DiffSummary{
			Changed:                    manifestDelta || len(added) > 0 || len(removed) > 0 || len(changed) > 0,
			AddedFiles:                 added,
			RemovedFiles:               removed,
			ChangedFiles:               changed,
			ManifestDelta:              manifestDelta,
			ContextChanged:             contextChanged,
			ContextRuntimeOnlyChanges:  contextRuntimeOnly,
			ContextDriftClassification: contextDriftClass,
		},
	}
	if contextChanged {
		result.Summary.Changed = true
	}
	return DiffResult{Result: result}, nil
}

func Inspect(path string) (InspectResult, error) {
	bundle, err := openZip(path)
	if err != nil {
		return InspectResult{}, err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if _, ok := bundle.Files[manifestFileName]; !ok {
		if _, runManifest := bundle.Files["manifest.json"]; runManifest {
			legacy, readErr := runpack.ReadRunpack(path)
			if readErr != nil {
				return InspectResult{}, readErr
			}
			payload := schemapack.RunPayload{
				SchemaID:            "gait.pack.run",
				SchemaVersion:       "1.0.0",
				CreatedAt:           legacy.Run.CreatedAt,
				RunID:               legacy.Run.RunID,
				CaptureMode:         legacy.Manifest.CaptureMode,
				ManifestDigest:      legacy.Manifest.ManifestDigest,
				IntentsCount:        len(legacy.Intents),
				ResultsCount:        len(legacy.Results),
				RefsCount:           len(legacy.Refs.Receipts),
				ContextSetDigest:    legacy.Refs.ContextSetDigest,
				ContextRefCount:     len(legacy.Refs.Receipts),
				ContextEvidenceMode: legacy.Refs.ContextEvidenceMode,
				ContextPrivacyMode:  detectContextPrivacyMode(legacy.Refs.Receipts),
			}
			return InspectResult{
				PackID:     legacy.Manifest.ManifestDigest,
				PackType:   string(BuildTypeRun),
				SourceRef:  legacy.Run.RunID,
				RunPayload: &payload,
				RunLineage: buildRunLineage(legacy),
				LegacyType: "runpack",
			}, nil
		}
		return InspectResult{}, fmt.Errorf("missing %s", manifestFileName)
	}

	manifestBytes, err := readZipFile(bundle.Files[manifestFileName])
	if err != nil {
		return InspectResult{}, fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := parsePackManifest(manifestBytes)
	if err != nil {
		return InspectResult{}, err
	}

	result := InspectResult{PackID: manifest.PackID, PackType: manifest.PackType, SourceRef: manifest.SourceRef, Manifest: &manifest}
	switch manifest.PackType {
	case string(BuildTypeRun):
		if payloadFile, ok := bundle.Files["run_payload.json"]; ok {
			payloadBytes, readErr := readZipFile(payloadFile)
			if readErr == nil {
				var payload schemapack.RunPayload
				if err := decodeStrictJSON(payloadBytes, &payload); err == nil {
					result.RunPayload = &payload
				}
			}
		}
		if sourceFile, ok := bundle.Files["source/runpack.zip"]; ok {
			sourceBytes, readErr := readZipFile(sourceFile)
			if readErr == nil {
				runData, runErr := readRunpackFromBytes(sourceBytes)
				if runErr == nil {
					result.RunLineage = buildRunLineage(runData)
				}
			}
		}
	case string(BuildTypeJob):
		if payloadFile, ok := bundle.Files["job_payload.json"]; ok {
			payloadBytes, readErr := readZipFile(payloadFile)
			if readErr == nil {
				var payload schemapack.JobPayload
				if err := decodeStrictJSON(payloadBytes, &payload); err == nil {
					result.JobPayload = &payload
				}
			}
		}
		stateFile, stateExists := bundle.Files["job_state.json"]
		eventsFile, eventsExist := bundle.Files["job_events.jsonl"]
		if stateExists && eventsExist {
			stateBytes, readStateErr := readZipFile(stateFile)
			eventsBytes, readEventsErr := readZipFile(eventsFile)
			if readStateErr == nil && readEventsErr == nil {
				var state jobruntime.JobState
				if decodeStrictJSON(stateBytes, &state) == nil {
					if events, parseErr := parseJobEvents(eventsBytes); parseErr == nil {
						result.JobLineage = buildJobLineage(state, events)
					}
				}
			}
		}
	}
	return result, nil
}

type artifactInfo struct {
	PackID              string
	PackType            string
	ManifestDigest      string
	Files               map[string]string
	ContextSetDigest    string
	ContextEvidenceMode string
	ContextRefCount     int
	ContextRefs         *schemarunpack.Refs
}

func collectArtifactInfo(path string) (artifactInfo, error) {
	bundle, err := openZip(path)
	if err != nil {
		return artifactInfo{}, err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if _, ok := bundle.Files[manifestFileName]; !ok {
		if _, runManifest := bundle.Files["manifest.json"]; runManifest {
			legacy, readErr := runpack.ReadRunpack(path)
			if readErr != nil {
				return artifactInfo{}, readErr
			}
			files := make(map[string]string, len(legacy.Manifest.Files))
			for _, entry := range legacy.Manifest.Files {
				files[entry.Path] = entry.SHA256
			}
			refsCopy := legacy.Refs
			return artifactInfo{
				PackID:              legacy.Manifest.ManifestDigest,
				PackType:            string(BuildTypeRun),
				ManifestDigest:      legacy.Manifest.ManifestDigest,
				Files:               files,
				ContextSetDigest:    legacy.Refs.ContextSetDigest,
				ContextEvidenceMode: legacy.Refs.ContextEvidenceMode,
				ContextRefCount:     len(legacy.Refs.Receipts),
				ContextRefs:         &refsCopy,
			}, nil
		}
		return artifactInfo{}, fmt.Errorf("missing %s", manifestFileName)
	}

	manifestBytes, err := readZipFile(bundle.Files[manifestFileName])
	if err != nil {
		return artifactInfo{}, err
	}
	manifest, err := parsePackManifest(manifestBytes)
	if err != nil {
		return artifactInfo{}, err
	}
	files := make(map[string]string, len(manifest.Contents))
	for _, entry := range manifest.Contents {
		files[entry.Path] = entry.SHA256
	}
	manifestDigest, err := jcs.DigestJCS(manifestBytes)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("digest manifest: %w", err)
	}
	info := artifactInfo{PackID: manifest.PackID, PackType: manifest.PackType, ManifestDigest: manifestDigest, Files: files}
	if manifest.PackType == string(BuildTypeRun) {
		if payloadFile, ok := bundle.Files["run_payload.json"]; ok {
			payloadBytes, readErr := readZipFile(payloadFile)
			if readErr == nil {
				var payload schemapack.RunPayload
				if decodeStrictJSON(payloadBytes, &payload) == nil {
					info.ContextSetDigest = strings.TrimSpace(payload.ContextSetDigest)
					info.ContextEvidenceMode = strings.TrimSpace(payload.ContextEvidenceMode)
					info.ContextRefCount = payload.ContextRefCount
				}
			}
		}
		if sourceRunpack, ok := bundle.Files["source/runpack.zip"]; ok {
			sourceBytes, readErr := readZipFile(sourceRunpack)
			if readErr == nil {
				runData, runErr := readRunpackFromBytes(sourceBytes)
				if runErr == nil {
					refsCopy := runData.Refs
					info.ContextRefs = &refsCopy
					info.ContextSetDigest = strings.TrimSpace(runData.Refs.ContextSetDigest)
					info.ContextEvidenceMode = strings.TrimSpace(runData.Refs.ContextEvidenceMode)
					info.ContextRefCount = len(runData.Refs.Receipts)
				}
			}
		}
	}
	return info, nil
}

func parsePackManifest(payload []byte) (schemapack.Manifest, error) {
	var manifest schemapack.Manifest
	if err := decodeStrictJSON(payload, &manifest); err != nil {
		return schemapack.Manifest{}, fmt.Errorf("parse pack manifest: %w", err)
	}
	if manifest.SchemaID != manifestSchemaID {
		return schemapack.Manifest{}, fmt.Errorf("unsupported manifest schema_id: %s", manifest.SchemaID)
	}
	if manifest.SchemaVersion != manifestSchemaVersion {
		return schemapack.Manifest{}, fmt.Errorf("unsupported manifest schema_version: %s", manifest.SchemaVersion)
	}
	if manifest.PackType != string(BuildTypeRun) && manifest.PackType != string(BuildTypeJob) {
		return schemapack.Manifest{}, fmt.Errorf("invalid pack_type: %s", manifest.PackType)
	}
	if strings.TrimSpace(manifest.SourceRef) == "" {
		return schemapack.Manifest{}, fmt.Errorf("manifest missing source_ref")
	}
	if manifest.CreatedAt.IsZero() {
		return schemapack.Manifest{}, fmt.Errorf("manifest missing created_at")
	}
	if strings.TrimSpace(manifest.ProducerVersion) == "" {
		return schemapack.Manifest{}, fmt.Errorf("manifest missing producer_version")
	}
	if !isSHA256Hex(manifest.PackID) {
		return schemapack.Manifest{}, fmt.Errorf("manifest pack_id must be sha256 hex")
	}
	if manifest.Contents == nil {
		return schemapack.Manifest{}, fmt.Errorf("manifest missing contents")
	}
	for _, entry := range manifest.Contents {
		if strings.TrimSpace(entry.Path) == "" {
			return schemapack.Manifest{}, fmt.Errorf("manifest entry path is required")
		}
		if !isSHA256Hex(entry.SHA256) {
			return schemapack.Manifest{}, fmt.Errorf("manifest entry sha256 must be sha256 hex")
		}
		if strings.TrimSpace(entry.Type) == "" {
			return schemapack.Manifest{}, fmt.Errorf("manifest entry type is required")
		}
	}
	for _, sig := range manifest.Signatures {
		if strings.TrimSpace(sig.Alg) == "" || strings.TrimSpace(sig.KeyID) == "" || strings.TrimSpace(sig.Sig) == "" {
			return schemapack.Manifest{}, fmt.Errorf("manifest signature fields are required")
		}
		if strings.TrimSpace(sig.SignedDigest) != "" && !isSHA256Hex(sig.SignedDigest) {
			return schemapack.Manifest{}, fmt.Errorf("manifest signature signed_digest must be sha256 hex")
		}
	}
	return manifest, nil
}

type openedZip struct {
	Reader *zip.ReadCloser
	Files  map[string]*zip.File
}

func (bundle *openedZip) Close() error {
	if bundle == nil || bundle.Reader == nil {
		return nil
	}
	return bundle.Reader.Close()
}

func openZip(path string) (*openedZip, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}
	return &openedZip{Reader: reader, Files: files}, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	payload, err := io.ReadAll(io.LimitReader(reader, maxZipEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxZipEntryBytes {
		return nil, fmt.Errorf("zip entry too large")
	}
	return payload, nil
}

func hashZipFile(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()
	hasher := sha256.New()
	n, err := io.Copy(hasher, io.LimitReader(reader, maxZipEntryBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxZipEntryBytes {
		return "", fmt.Errorf("zip entry too large")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jcs.CanonicalizeJSON(raw)
}

func canonicalJSONL[T any](values []T) ([]byte, error) {
	if len(values) == 0 {
		return []byte{}, nil
	}
	var buffer bytes.Buffer
	for _, value := range values {
		line, err := canonicalJSON(value)
		if err != nil {
			return nil, err
		}
		buffer.Write(line)
		buffer.WriteByte('\n')
	}
	return buffer.Bytes(), nil
}

func detectEntryType(path string) string {
	lower := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.HasSuffix(lower, ".json"):
		return "json"
	case strings.HasSuffix(lower, ".jsonl"):
		return "jsonl"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return "blob"
	}
}

func computePackID(manifest schemapack.Manifest) (string, error) {
	manifest.PackID = ""
	manifest.Signatures = nil
	raw, err := canonicalJSON(manifest)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeTime(value time.Time) time.Time {
	if value.IsZero() {
		return deterministicTimestamp
	}
	return value.UTC()
}

func verificationError(err error) error {
	return coreerrors.Wrap(err, coreerrors.CategoryVerification, "pack_verify_failed", "re-run verify after checking artifact integrity", false)
}

func verifyPayloadContracts(bundle *openedZip, manifest schemapack.Manifest) error {
	switch manifest.PackType {
	case string(BuildTypeRun):
		payloadFile, ok := bundle.Files["run_payload.json"]
		if !ok {
			return fmt.Errorf("missing run_payload.json")
		}
		if _, hasSource := bundle.Files["source/runpack.zip"]; !hasSource {
			return fmt.Errorf("missing source/runpack.zip")
		}
		payloadBytes, err := readZipFile(payloadFile)
		if err != nil {
			return fmt.Errorf("read run_payload.json: %w", err)
		}
		var payload schemapack.RunPayload
		if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
			return fmt.Errorf("parse run_payload.json: %w", err)
		}
		if err := validateRunPayload(payload); err != nil {
			return err
		}
		if payload.RunID != strings.TrimSpace(manifest.SourceRef) {
			return fmt.Errorf("run payload run_id does not match manifest source_ref")
		}
	case string(BuildTypeJob):
		payloadFile, ok := bundle.Files["job_payload.json"]
		if !ok {
			return fmt.Errorf("missing job_payload.json")
		}
		stateFile, ok := bundle.Files["job_state.json"]
		if !ok {
			return fmt.Errorf("missing job_state.json")
		}
		eventsFile, ok := bundle.Files["job_events.jsonl"]
		if !ok {
			return fmt.Errorf("missing job_events.jsonl")
		}
		payloadBytes, err := readZipFile(payloadFile)
		if err != nil {
			return fmt.Errorf("read job_payload.json: %w", err)
		}
		var payload schemapack.JobPayload
		if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
			return fmt.Errorf("parse job_payload.json: %w", err)
		}
		if err := validateJobPayload(payload); err != nil {
			return err
		}

		stateBytes, err := readZipFile(stateFile)
		if err != nil {
			return fmt.Errorf("read job_state.json: %w", err)
		}
		var state jobruntime.JobState
		if err := decodeStrictJSON(stateBytes, &state); err != nil {
			return fmt.Errorf("parse job_state.json: %w", err)
		}
		if err := validateJobState(state); err != nil {
			return err
		}

		eventsBytes, err := readZipFile(eventsFile)
		if err != nil {
			return fmt.Errorf("read job_events.jsonl: %w", err)
		}
		events, err := parseJobEvents(eventsBytes)
		if err != nil {
			return err
		}

		if payload.JobID != strings.TrimSpace(manifest.SourceRef) {
			return fmt.Errorf("job payload job_id does not match manifest source_ref")
		}
		if state.JobID != payload.JobID {
			return fmt.Errorf("job_state job_id does not match job payload")
		}
		if payload.CheckpointCount != len(state.Checkpoints) {
			return fmt.Errorf("job payload checkpoint_count does not match job_state")
		}
		if payload.ApprovalCount != len(state.Approvals) {
			return fmt.Errorf("job payload approval_count does not match job_state")
		}
		for _, event := range events {
			if strings.TrimSpace(event.JobID) != payload.JobID {
				return fmt.Errorf("job event job_id does not match job payload")
			}
		}
	default:
		return fmt.Errorf("unsupported pack type: %s", manifest.PackType)
	}
	return nil
}

func validateRunPayload(payload schemapack.RunPayload) error {
	if payload.SchemaID != "gait.pack.run" {
		return fmt.Errorf("run payload schema_id must be gait.pack.run")
	}
	if payload.SchemaVersion != "1.0.0" {
		return fmt.Errorf("run payload schema_version must be 1.0.0")
	}
	if payload.CreatedAt.IsZero() {
		return fmt.Errorf("run payload created_at is required")
	}
	if strings.TrimSpace(payload.RunID) == "" {
		return fmt.Errorf("run payload run_id is required")
	}
	if payload.CaptureMode != "reference" && payload.CaptureMode != "raw" {
		return fmt.Errorf("run payload capture_mode must be reference or raw")
	}
	if !isSHA256Hex(payload.ManifestDigest) {
		return fmt.Errorf("run payload manifest_digest must be sha256 hex")
	}
	if payload.IntentsCount < 0 || payload.ResultsCount < 0 || payload.RefsCount < 0 {
		return fmt.Errorf("run payload counts must be >= 0")
	}
	if strings.TrimSpace(payload.ContextSetDigest) != "" && !isSHA256Hex(payload.ContextSetDigest) {
		return fmt.Errorf("run payload context_set_digest must be sha256 hex")
	}
	if payload.ContextRefCount < 0 {
		return fmt.Errorf("run payload context_ref_count must be >= 0")
	}
	if strings.TrimSpace(payload.ContextEvidenceMode) != "" &&
		strings.TrimSpace(payload.ContextEvidenceMode) != contextproof.EvidenceModeBestEffort &&
		strings.TrimSpace(payload.ContextEvidenceMode) != contextproof.EvidenceModeRequired {
		return fmt.Errorf("run payload context_evidence_mode must be best_effort or required")
	}
	if strings.TrimSpace(payload.ContextPrivacyMode) != "" {
		switch strings.TrimSpace(payload.ContextPrivacyMode) {
		case "mixed":
		default:
			// individual modes are free-form but must be non-empty strings.
		}
	}
	return nil
}

func validateJobPayload(payload schemapack.JobPayload) error {
	if payload.SchemaID != "gait.pack.job" {
		return fmt.Errorf("job payload schema_id must be gait.pack.job")
	}
	if payload.SchemaVersion != "1.0.0" {
		return fmt.Errorf("job payload schema_version must be 1.0.0")
	}
	if payload.CreatedAt.IsZero() {
		return fmt.Errorf("job payload created_at is required")
	}
	if strings.TrimSpace(payload.JobID) == "" {
		return fmt.Errorf("job payload job_id is required")
	}
	if !validJobStatus(payload.Status) {
		return fmt.Errorf("job payload status is invalid")
	}
	if strings.TrimSpace(payload.StopReason) == "" {
		return fmt.Errorf("job payload stop_reason is required")
	}
	if strings.TrimSpace(payload.StatusReasonCode) == "" {
		return fmt.Errorf("job payload status_reason_code is required")
	}
	if strings.TrimSpace(payload.EnvironmentFingerprint) == "" {
		return fmt.Errorf("job payload environment_fingerprint is required")
	}
	if payload.CheckpointCount < 0 || payload.ApprovalCount < 0 {
		return fmt.Errorf("job payload counts must be >= 0")
	}
	return nil
}

func validateJobState(state jobruntime.JobState) error {
	if strings.TrimSpace(state.JobID) == "" {
		return fmt.Errorf("job_state job_id is required")
	}
	if strings.TrimSpace(state.Status) == "" || !validJobStatus(state.Status) {
		return fmt.Errorf("job_state status is invalid")
	}
	if strings.TrimSpace(state.StopReason) == "" {
		return fmt.Errorf("job_state stop_reason is required")
	}
	if strings.TrimSpace(state.StatusReasonCode) == "" {
		return fmt.Errorf("job_state status_reason_code is required")
	}
	if strings.TrimSpace(state.EnvironmentFingerprint) == "" {
		return fmt.Errorf("job_state environment_fingerprint is required")
	}
	return nil
}

func validJobStatus(status string) bool {
	switch status {
	case jobruntime.StatusRunning,
		jobruntime.StatusPaused,
		jobruntime.StatusDecisionNeeded,
		jobruntime.StatusBlocked,
		jobruntime.StatusCompleted,
		jobruntime.StatusCancelled:
		return true
	default:
		return false
	}
}

func decodeStrictJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple json values")
		}
		return err
	}
	return nil
}

func parseJobEvents(payload []byte) ([]jobruntime.Event, error) {
	if len(payload) == 0 {
		return []jobruntime.Event{}, nil
	}
	events := []jobruntime.Event{}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var event jobruntime.Event
		if err := decodeStrictJSON(raw, &event); err != nil {
			return nil, fmt.Errorf("parse job_events.jsonl line %d: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan job_events.jsonl: %w", err)
	}
	return events, nil
}

func buildRunLineage(data runpack.Runpack) *RunLineage {
	linksByID := make(map[string]RunIntentResultLink, len(data.Intents))
	for _, intent := range data.Intents {
		linksByID[intent.IntentID] = RunIntentResultLink{
			IntentID: intent.IntentID,
			ToolName: intent.ToolName,
		}
	}
	for _, result := range data.Results {
		link := linksByID[result.IntentID]
		link.IntentID = result.IntentID
		link.Status = result.Status
		linksByID[result.IntentID] = link
	}
	intentIDs := make([]string, 0, len(linksByID))
	for intentID := range linksByID {
		intentIDs = append(intentIDs, intentID)
	}
	sort.Strings(intentIDs)
	links := make([]RunIntentResultLink, 0, len(intentIDs))
	for _, intentID := range intentIDs {
		links = append(links, linksByID[intentID])
	}
	return &RunLineage{
		TimelineEvents: len(data.Run.Timeline),
		ReceiptCount:   len(data.Refs.Receipts),
		IntentResults:  links,
	}
}

func detectContextPrivacyMode(receipts []schemarunpack.RefReceipt) string {
	if len(receipts) == 0 {
		return ""
	}
	first := strings.TrimSpace(receipts[0].RedactionMode)
	if first == "" {
		first = "unknown"
	}
	for i := 1; i < len(receipts); i++ {
		mode := strings.TrimSpace(receipts[i].RedactionMode)
		if mode == "" {
			mode = "unknown"
		}
		if mode != first {
			return "mixed"
		}
	}
	return first
}

func buildJobLineage(state jobruntime.JobState, events []jobruntime.Event) *JobLineage {
	checkpointRefs := make([]JobCheckpointRef, 0, len(state.Checkpoints))
	for _, checkpoint := range state.Checkpoints {
		checkpointRefs = append(checkpointRefs, JobCheckpointRef{
			CheckpointID: checkpoint.CheckpointID,
			Type:         checkpoint.Type,
			ReasonCode:   checkpoint.ReasonCode,
		})
	}
	sort.Slice(checkpointRefs, func(i, j int) bool {
		return checkpointRefs[i].CheckpointID < checkpointRefs[j].CheckpointID
	})
	lastEventType := ""
	if len(events) > 0 {
		lastEventType = strings.TrimSpace(events[len(events)-1].Type)
	}
	return &JobLineage{
		EventCount:     len(events),
		LastEventType:  lastEventType,
		CheckpointRefs: checkpointRefs,
	}
}

func readRunpackFromBytes(payload []byte) (runpack.Runpack, error) {
	tempFile, err := os.CreateTemp("", "gait-pack-runpack-*.zip")
	if err != nil {
		return runpack.Runpack{}, fmt.Errorf("create temp runpack: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()
	if _, err := tempFile.Write(payload); err != nil {
		return runpack.Runpack{}, fmt.Errorf("write temp runpack: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return runpack.Runpack{}, fmt.Errorf("close temp runpack: %w", err)
	}
	return runpack.ReadRunpack(tempPath)
}

func isSHA256Hex(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 64 {
		return false
	}
	_, err := hex.DecodeString(trimmed)
	return err == nil
}

// ExtractRunpack returns legacy runpack bytes or the embedded source runpack bytes from a PackSpec run artifact.
func ExtractRunpack(path string) ([]byte, error) {
	bundle, err := openZip(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if _, ok := bundle.Files["manifest.json"]; ok {
		// #nosec G304 -- caller provides explicit local path.
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read legacy runpack bytes: %w", readErr)
		}
		return raw, nil
	}

	manifestFile, ok := bundle.Files[manifestFileName]
	if !ok {
		return nil, fmt.Errorf("missing %s", manifestFileName)
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestFileName, err)
	}
	manifest, err := parsePackManifest(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestFileName, err)
	}
	if manifest.PackType != string(BuildTypeRun) {
		return nil, fmt.Errorf("pack type %s does not contain a runpack source", manifest.PackType)
	}
	sourceFile, ok := bundle.Files["source/runpack.zip"]
	if !ok {
		return nil, fmt.Errorf("missing source/runpack.zip")
	}
	sourceBytes, err := readZipFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("read source/runpack.zip: %w", err)
	}
	return sourceBytes, nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func convertRunpackMismatches(input []runpack.HashMismatch) []HashMismatch {
	output := make([]HashMismatch, 0, len(input))
	for _, mismatch := range input {
		output = append(output, HashMismatch{Path: mismatch.Path, Expected: mismatch.Expected, Actual: mismatch.Actual})
	}
	return output
}

func convertGuardMismatches(input []guard.HashMismatch) []HashMismatch {
	output := make([]HashMismatch, 0, len(input))
	for _, mismatch := range input {
		output = append(output, HashMismatch{Path: mismatch.Path, Expected: mismatch.Expected, Actual: mismatch.Actual})
	}
	return output
}

func BuildJobPackFromPath(root string, jobID string, outputPath string, producerVersion string, signKey ed25519.PrivateKey) (BuildResult, error) {
	state, events, err := jobruntime.Inspect(root, jobID)
	if err != nil {
		return BuildResult{}, err
	}
	return BuildJobPack(BuildJobOptions{
		State:             state,
		Events:            events,
		OutputPath:        outputPath,
		ProducerVersion:   producerVersion,
		SigningPrivateKey: signKey,
	})
}

func LoadRunpackManifest(path string) (schemarunpack.Manifest, error) {
	data, err := runpack.ReadRunpack(path)
	if err != nil {
		return schemarunpack.Manifest{}, err
	}
	return data.Manifest, nil
}
