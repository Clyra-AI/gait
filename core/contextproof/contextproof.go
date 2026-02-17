package contextproof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

const (
	EnvelopeSchemaID      = "gait.context.envelope"
	EnvelopeSchemaVersion = "1.0.0"
	BudgetSchemaID        = "gait.context.budget_report"
	BudgetSchemaVersion   = "1.0.0"
	MaxEnvelopeBytes      = int64(1024 * 1024)

	EvidenceModeBestEffort = "best_effort"
	EvidenceModeRequired   = "required"

	PrivacyModeMetadata = "metadata"
	PrivacyModeHashes   = "hashes"
	PrivacyModeRaw      = "raw"

	driftNone        = "none"
	driftRuntimeOnly = "runtime_only"
	driftSemantic    = "semantic"
)

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var zeroDigest = strings.Repeat("0", 64)

func NormalizeEvidenceMode(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return "", nil
	}
	if normalized != EvidenceModeBestEffort && normalized != EvidenceModeRequired {
		return "", fmt.Errorf("context evidence mode must be best_effort or required")
	}
	return normalized, nil
}

func ParseEnvelope(payload []byte) (schemacontext.Envelope, error) {
	var envelope schemacontext.Envelope
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return schemacontext.Envelope{}, fmt.Errorf("parse context envelope: %w", err)
	}
	return NormalizeEnvelope(envelope)
}

func LoadEnvelope(path string) (schemacontext.Envelope, error) {
	info, err := os.Stat(path)
	if err != nil {
		return schemacontext.Envelope{}, fmt.Errorf("stat context envelope: %w", err)
	}
	if info.Size() > MaxEnvelopeBytes {
		return schemacontext.Envelope{}, fmt.Errorf("context envelope exceeds size limit (%d bytes)", MaxEnvelopeBytes)
	}
	// #nosec G304 -- path is explicit local user input.
	payload, err := os.ReadFile(path)
	if err != nil {
		return schemacontext.Envelope{}, fmt.Errorf("read context envelope: %w", err)
	}
	if int64(len(payload)) > MaxEnvelopeBytes {
		return schemacontext.Envelope{}, fmt.Errorf("context envelope exceeds size limit (%d bytes)", MaxEnvelopeBytes)
	}
	return ParseEnvelope(payload)
}

type BuildEnvelopeOptions struct {
	ContextSetID    string
	EvidenceMode    string
	ProducerVersion string
	CreatedAt       time.Time
}

func BuildEnvelope(records []schemacontext.ReferenceRecord, options BuildEnvelopeOptions) (schemacontext.Envelope, error) {
	envelope := schemacontext.Envelope{
		SchemaID:         EnvelopeSchemaID,
		SchemaVersion:    EnvelopeSchemaVersion,
		CreatedAt:        options.CreatedAt,
		ProducerVersion:  strings.TrimSpace(options.ProducerVersion),
		ContextSetID:     strings.TrimSpace(options.ContextSetID),
		ContextSetDigest: "",
		EvidenceMode:     strings.TrimSpace(options.EvidenceMode),
		Records:          append([]schemacontext.ReferenceRecord(nil), records...),
	}
	return NormalizeEnvelope(envelope)
}

func NormalizeEnvelope(input schemacontext.Envelope) (schemacontext.Envelope, error) {
	envelope := input
	if strings.TrimSpace(envelope.SchemaID) == "" {
		envelope.SchemaID = EnvelopeSchemaID
	}
	if envelope.SchemaID != EnvelopeSchemaID {
		return schemacontext.Envelope{}, fmt.Errorf("unsupported context envelope schema_id: %s", envelope.SchemaID)
	}
	if strings.TrimSpace(envelope.SchemaVersion) == "" {
		envelope.SchemaVersion = EnvelopeSchemaVersion
	}
	if envelope.SchemaVersion != EnvelopeSchemaVersion {
		return schemacontext.Envelope{}, fmt.Errorf("unsupported context envelope schema_version: %s", envelope.SchemaVersion)
	}
	if envelope.CreatedAt.IsZero() {
		envelope.CreatedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	} else {
		envelope.CreatedAt = envelope.CreatedAt.UTC()
	}
	if strings.TrimSpace(envelope.ProducerVersion) == "" {
		envelope.ProducerVersion = "0.0.0-dev"
	}
	envelope.ContextSetID = strings.TrimSpace(envelope.ContextSetID)
	if envelope.ContextSetID == "" {
		return schemacontext.Envelope{}, fmt.Errorf("context_set_id is required")
	}
	evidenceMode, err := NormalizeEvidenceMode(envelope.EvidenceMode)
	if err != nil {
		return schemacontext.Envelope{}, err
	}
	if evidenceMode == "" {
		evidenceMode = EvidenceModeBestEffort
	}
	envelope.EvidenceMode = evidenceMode

	records := make([]schemacontext.ReferenceRecord, 0, len(envelope.Records))
	for _, record := range envelope.Records {
		normalized, normErr := normalizeRecord(record)
		if normErr != nil {
			return schemacontext.Envelope{}, normErr
		}
		records = append(records, normalized)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].RefID != records[j].RefID {
			return records[i].RefID < records[j].RefID
		}
		if records[i].SourceType != records[j].SourceType {
			return records[i].SourceType < records[j].SourceType
		}
		if records[i].SourceLocator != records[j].SourceLocator {
			return records[i].SourceLocator < records[j].SourceLocator
		}
		return records[i].ContentDigest < records[j].ContentDigest
	})
	envelope.Records = records

	computedDigest, err := ContextSetDigest(records)
	if err != nil {
		return schemacontext.Envelope{}, err
	}
	providedDigest := strings.ToLower(strings.TrimSpace(envelope.ContextSetDigest))
	if providedDigest == "" {
		envelope.ContextSetDigest = computedDigest
		return envelope, nil
	}
	if !digestPattern.MatchString(providedDigest) {
		return schemacontext.Envelope{}, fmt.Errorf("context_set_digest must be sha256 hex")
	}
	if computedDigest != providedDigest {
		return schemacontext.Envelope{}, fmt.Errorf("context_set_digest mismatch")
	}
	envelope.ContextSetDigest = providedDigest
	return envelope, nil
}

func ContextSetDigest(records []schemacontext.ReferenceRecord) (string, error) {
	raw, err := json.Marshal(records)
	if err != nil {
		return "", fmt.Errorf("marshal context records: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest context records: %w", err)
	}
	return digest, nil
}

func DigestEnvelope(envelope schemacontext.Envelope) (string, error) {
	normalized, err := NormalizeEnvelope(envelope)
	if err != nil {
		return "", err
	}
	return normalized.ContextSetDigest, nil
}

func VerifyEnvelope(envelope schemacontext.Envelope) error {
	_, err := NormalizeEnvelope(envelope)
	return err
}

func ApplyEnvelopeToRefs(refs *schemarunpack.Refs, envelope schemacontext.Envelope) {
	if refs == nil {
		return
	}
	refs.ContextSetDigest = envelope.ContextSetDigest
	refs.ContextEvidenceMode = envelope.EvidenceMode
	refs.ContextRefCount = len(envelope.Records)
	receipts := make([]schemarunpack.RefReceipt, 0, len(envelope.Records))
	for _, record := range envelope.Records {
		receipts = append(receipts, schemarunpack.RefReceipt{
			RefID:               record.RefID,
			SourceType:          record.SourceType,
			SourceLocator:       record.SourceLocator,
			QueryDigest:         record.QueryDigest,
			ContentDigest:       record.ContentDigest,
			RetrievedAt:         record.RetrievedAt.UTC(),
			RedactionMode:       record.RedactionMode,
			Immutability:        record.Immutability,
			FreshnessSLASeconds: record.FreshnessSLASeconds,
			SensitivityLabel:    record.SensitivityLabel,
			RetrievalParams:     record.RetrievalParams,
		})
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].RefID < receipts[j].RefID })
	refs.Receipts = receipts
}

func EnvelopeFromRefs(refs schemarunpack.Refs) (schemacontext.Envelope, bool, error) {
	normalized, err := NormalizeRefs(refs)
	if err != nil {
		return schemacontext.Envelope{}, false, err
	}
	if strings.TrimSpace(normalized.ContextSetDigest) == "" && len(normalized.Receipts) == 0 {
		return schemacontext.Envelope{}, false, nil
	}
	records := make([]schemacontext.ReferenceRecord, 0, len(normalized.Receipts))
	for _, receipt := range normalized.Receipts {
		records = append(records, schemacontext.ReferenceRecord{
			RefID:               receipt.RefID,
			SourceType:          receipt.SourceType,
			SourceLocator:       receipt.SourceLocator,
			QueryDigest:         receipt.QueryDigest,
			ContentDigest:       receipt.ContentDigest,
			RetrievedAt:         receipt.RetrievedAt,
			RedactionMode:       receipt.RedactionMode,
			Immutability:        receipt.Immutability,
			FreshnessSLASeconds: receipt.FreshnessSLASeconds,
			SensitivityLabel:    receipt.SensitivityLabel,
			RetrievalParams:     receipt.RetrievalParams,
		})
	}
	envelope := schemacontext.Envelope{
		SchemaID:         EnvelopeSchemaID,
		SchemaVersion:    EnvelopeSchemaVersion,
		CreatedAt:        normalized.CreatedAt,
		ProducerVersion:  normalized.ProducerVersion,
		ContextSetID:     normalized.RunID,
		ContextSetDigest: normalized.ContextSetDigest,
		EvidenceMode:     normalized.ContextEvidenceMode,
		Records:          records,
	}
	normalizedEnvelope, err := NormalizeEnvelope(envelope)
	if err != nil {
		return schemacontext.Envelope{}, false, err
	}
	return normalizedEnvelope, true, nil
}

func NormalizeRefs(refs schemarunpack.Refs) (schemarunpack.Refs, error) {
	output := refs
	mode, err := NormalizeEvidenceMode(output.ContextEvidenceMode)
	if err != nil {
		return schemarunpack.Refs{}, err
	}
	output.ContextEvidenceMode = mode
	output.ContextSetDigest = strings.ToLower(strings.TrimSpace(output.ContextSetDigest))
	if output.ContextSetDigest != "" && !digestPattern.MatchString(output.ContextSetDigest) {
		return schemarunpack.Refs{}, fmt.Errorf("refs context_set_digest must be sha256 hex")
	}
	if output.ContextRefCount < 0 {
		return schemarunpack.Refs{}, fmt.Errorf("refs context_ref_count must be >= 0")
	}
	if output.Receipts == nil {
		output.Receipts = []schemarunpack.RefReceipt{}
	}
	for i := range output.Receipts {
		output.Receipts[i].QueryDigest = strings.ToLower(strings.TrimSpace(output.Receipts[i].QueryDigest))
		output.Receipts[i].ContentDigest = strings.ToLower(strings.TrimSpace(output.Receipts[i].ContentDigest))
		output.Receipts[i].Immutability = strings.ToLower(strings.TrimSpace(output.Receipts[i].Immutability))
		if output.Receipts[i].Immutability == "" {
			output.Receipts[i].Immutability = "unknown"
		}
		if output.Receipts[i].RetrievedAt.IsZero() {
			output.Receipts[i].RetrievedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
		} else {
			output.Receipts[i].RetrievedAt = output.Receipts[i].RetrievedAt.UTC()
		}
	}
	sort.Slice(output.Receipts, func(i, j int) bool { return output.Receipts[i].RefID < output.Receipts[j].RefID })
	if output.ContextRefCount == 0 && len(output.Receipts) > 0 {
		output.ContextRefCount = len(output.Receipts)
	}
	return output, nil
}

func NormalizePrivacyMode(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return PrivacyModeMetadata, nil
	}
	switch normalized {
	case PrivacyModeMetadata, PrivacyModeHashes, PrivacyModeRaw:
		return normalized, nil
	default:
		return "", fmt.Errorf("privacy mode must be one of: metadata, hashes, raw")
	}
}

func TransformEnvelopePrivacy(envelope schemacontext.Envelope, mode string) (schemacontext.Envelope, error) {
	normalizedMode, err := NormalizePrivacyMode(mode)
	if err != nil {
		return schemacontext.Envelope{}, err
	}
	output := envelope
	output.Records = append([]schemacontext.ReferenceRecord(nil), envelope.Records...)
	for i := range output.Records {
		switch normalizedMode {
		case PrivacyModeMetadata:
			output.Records[i].RetrievalParams = nil
			output.Records[i].SensitivityLabel = ""
			output.Records[i].QueryDigest = zeroDigest
			output.Records[i].ContentDigest = zeroDigest
			output.Records[i].SourceLocator = "redacted:metadata"
		case PrivacyModeHashes:
			output.Records[i].RetrievalParams = nil
			output.Records[i].SensitivityLabel = ""
			output.Records[i].SourceLocator = "redacted:hashes"
		case PrivacyModeRaw:
		}
	}
	if normalizedMode != PrivacyModeRaw {
		output.ContextSetDigest = ""
	}
	return NormalizeEnvelope(output)
}

func normalizeRecord(record schemacontext.ReferenceRecord) (schemacontext.ReferenceRecord, error) {
	output := record
	output.RefID = strings.TrimSpace(output.RefID)
	output.SourceType = strings.TrimSpace(output.SourceType)
	output.SourceLocator = strings.TrimSpace(output.SourceLocator)
	output.QueryDigest = strings.ToLower(strings.TrimSpace(output.QueryDigest))
	output.ContentDigest = strings.ToLower(strings.TrimSpace(output.ContentDigest))
	output.RedactionMode = strings.TrimSpace(output.RedactionMode)
	output.Immutability = strings.ToLower(strings.TrimSpace(output.Immutability))
	if output.RefID == "" || output.SourceType == "" || output.SourceLocator == "" || output.RedactionMode == "" {
		return schemacontext.ReferenceRecord{}, fmt.Errorf("context record fields ref_id/source_type/source_locator/redaction_mode are required")
	}
	if !digestPattern.MatchString(output.QueryDigest) || !digestPattern.MatchString(output.ContentDigest) {
		return schemacontext.ReferenceRecord{}, fmt.Errorf("context record digests must be sha256 hex")
	}
	switch output.Immutability {
	case "", "unknown":
		output.Immutability = "unknown"
	case "mutable", "immutable":
	default:
		return schemacontext.ReferenceRecord{}, fmt.Errorf("context record immutability must be unknown|mutable|immutable")
	}
	if output.RetrievedAt.IsZero() {
		output.RetrievedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	} else {
		output.RetrievedAt = output.RetrievedAt.UTC()
	}
	if output.FreshnessSLASeconds < 0 {
		return schemacontext.ReferenceRecord{}, fmt.Errorf("context record freshness_sla_seconds must be >= 0")
	}
	return output, nil
}
