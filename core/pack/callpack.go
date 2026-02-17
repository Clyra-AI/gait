package pack

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/runpack"
	schemapack "github.com/Clyra-AI/gait/core/schema/v1/pack"
	schemavoice "github.com/Clyra-AI/gait/core/schema/v1/voice"
	"github.com/Clyra-AI/gait/core/zipx"
)

const (
	callRecordSchemaID          = "gait.voice.call_record"
	callRecordSchemaVersion     = "1.0.0"
	callPayloadSchemaID         = "gait.pack.call"
	callPayloadSchemaVersion    = "1.0.0"
	callpackManifestSchemaID    = "gait.voice.callpack_manifest"
	callpackManifestSchemaV1    = "1.0.0"
	privacyModeHashOnly         = "hash_only"
	privacyModeDisputeEncrypted = "dispute_encrypted"
)

var callEventTypesRequired = []string{
	"asr.final",
	"commitment.declared",
	"gate.decision",
	"tts.request",
	"tts.emitted",
	"tool.intent",
	"tool.result",
}

type BuildCallOptions struct {
	CallRecordPath    string
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
}

type voiceCallRecord struct {
	SchemaID               string                         `json:"schema_id"`
	SchemaVersion          string                         `json:"schema_version"`
	CreatedAt              time.Time                      `json:"created_at"`
	ProducerVersion        string                         `json:"producer_version"`
	CallID                 string                         `json:"call_id"`
	RunpackPath            string                         `json:"runpack_path"`
	PrivacyMode            string                         `json:"privacy_mode"`
	EnvironmentFingerprint string                         `json:"environment_fingerprint,omitempty"`
	Events                 []schemavoice.CallEvent        `json:"events"`
	Commitments            []schemavoice.CommitmentIntent `json:"commitments"`
	GateDecisions          []schemavoice.GateDecision     `json:"gate_decisions"`
	SpeakReceipts          []schemavoice.SpeakReceipt     `json:"speak_receipts"`
	ReferenceDigests       []schemavoice.ReferenceDigest  `json:"reference_digests,omitempty"`
}

func BuildCallPack(options BuildCallOptions) (BuildResult, error) {
	recordPath := strings.TrimSpace(options.CallRecordPath)
	if recordPath == "" {
		return BuildResult{}, fmt.Errorf("call record path is required")
	}
	// #nosec G304 -- call record path is explicit local input.
	content, err := os.ReadFile(recordPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read call record: %w", err)
	}
	var record voiceCallRecord
	if err := decodeStrictJSON(content, &record); err != nil {
		return BuildResult{}, fmt.Errorf("parse call record: %w", err)
	}
	record, err = normalizeCallRecord(record)
	if err != nil {
		return BuildResult{}, err
	}
	runpackPath := record.RunpackPath
	if !filepath.IsAbs(runpackPath) {
		runpackPath = filepath.Join(filepath.Dir(recordPath), runpackPath)
	}
	verifyResult, err := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{RequireSignature: false})
	if err != nil {
		return BuildResult{}, fmt.Errorf("verify source runpack: %w", err)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 {
		return BuildResult{}, fmt.Errorf("source runpack failed integrity checks")
	}
	// #nosec G304 -- normalized runpack path derived from explicit local input.
	runpackBytes, err := os.ReadFile(runpackPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read source runpack: %w", err)
	}

	payload := schemapack.CallPayload{
		SchemaID:               callPayloadSchemaID,
		SchemaVersion:          callPayloadSchemaVersion,
		CreatedAt:              record.CreatedAt.UTC(),
		CallID:                 record.CallID,
		RunID:                  strings.TrimSpace(verifyResult.RunID),
		PrivacyMode:            record.PrivacyMode,
		EventCount:             len(record.Events),
		CommitmentCount:        len(record.Commitments),
		DecisionCount:          len(record.GateDecisions),
		SpeakReceiptCount:      len(record.SpeakReceipts),
		ReferenceDigestCount:   len(record.ReferenceDigests),
		EnvironmentFingerprint: record.EnvironmentFingerprint,
	}
	payloadBytes, err := canonicalJSON(payload)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode call payload: %w", err)
	}
	callpackManifest := schemavoice.CallpackManifest{
		SchemaID:               callpackManifestSchemaID,
		SchemaVersion:          callpackManifestSchemaV1,
		CreatedAt:              record.CreatedAt.UTC(),
		ProducerVersion:        record.ProducerVersion,
		CallID:                 record.CallID,
		PrivacyMode:            record.PrivacyMode,
		EventCount:             len(record.Events),
		CommitmentCount:        len(record.Commitments),
		DecisionCount:          len(record.GateDecisions),
		SpeakReceiptCount:      len(record.SpeakReceipts),
		ReferenceDigestCount:   len(record.ReferenceDigests),
		EnvironmentFingerprint: record.EnvironmentFingerprint,
	}
	callpackManifestBytes, err := canonicalJSON(callpackManifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode callpack manifest: %w", err)
	}
	eventBytes, err := canonicalJSONL(record.Events)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode call events: %w", err)
	}
	commitmentBytes, err := canonicalJSONL(record.Commitments)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode commitments: %w", err)
	}
	decisionBytes, err := canonicalJSONL(record.GateDecisions)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode gate decisions: %w", err)
	}
	speakBytes, err := canonicalJSONL(record.SpeakReceipts)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode speak receipts: %w", err)
	}
	referenceBytes, err := canonicalJSON(record.ReferenceDigests)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode reference digests: %w", err)
	}
	files := []zipx.File{
		{Path: "call_payload.json", Data: payloadBytes, Mode: 0o644},
		{Path: "callpack_manifest.json", Data: callpackManifestBytes, Mode: 0o644},
		{Path: "call_events.jsonl", Data: eventBytes, Mode: 0o644},
		{Path: "commitments.jsonl", Data: commitmentBytes, Mode: 0o644},
		{Path: "gate_decisions.jsonl", Data: decisionBytes, Mode: 0o644},
		{Path: "speak_receipts.jsonl", Data: speakBytes, Mode: 0o644},
		{Path: "reference_digests.json", Data: referenceBytes, Mode: 0o644},
		{Path: "source/runpack.zip", Data: runpackBytes, Mode: 0o644},
	}
	return buildPackWithFiles(buildPackOptions{
		PackType:          string(BuildTypeCall),
		SourceRef:         record.CallID,
		OutputPath:        options.OutputPath,
		ProducerVersion:   options.ProducerVersion,
		SigningPrivateKey: options.SigningPrivateKey,
		Files:             files,
		OutputDirFallback: filepath.Dir(recordPath),
	})
}

func normalizeCallRecord(input voiceCallRecord) (voiceCallRecord, error) {
	output := input
	if strings.TrimSpace(output.SchemaID) == "" {
		output.SchemaID = callRecordSchemaID
	}
	if output.SchemaID != callRecordSchemaID {
		return voiceCallRecord{}, fmt.Errorf("unsupported call record schema_id: %s", output.SchemaID)
	}
	if strings.TrimSpace(output.SchemaVersion) == "" {
		output.SchemaVersion = callRecordSchemaVersion
	}
	if output.SchemaVersion != callRecordSchemaVersion {
		return voiceCallRecord{}, fmt.Errorf("unsupported call record schema_version: %s", output.SchemaVersion)
	}
	if output.CreatedAt.IsZero() {
		return voiceCallRecord{}, fmt.Errorf("call record created_at is required")
	}
	output.CreatedAt = output.CreatedAt.UTC()
	output.ProducerVersion = strings.TrimSpace(output.ProducerVersion)
	if output.ProducerVersion == "" {
		output.ProducerVersion = "0.0.0-dev"
	}
	output.CallID = strings.TrimSpace(output.CallID)
	if output.CallID == "" {
		return voiceCallRecord{}, fmt.Errorf("call record call_id is required")
	}
	output.RunpackPath = strings.TrimSpace(output.RunpackPath)
	if output.RunpackPath == "" {
		return voiceCallRecord{}, fmt.Errorf("call record runpack_path is required")
	}
	output.PrivacyMode = strings.ToLower(strings.TrimSpace(output.PrivacyMode))
	if output.PrivacyMode == "" {
		output.PrivacyMode = privacyModeHashOnly
	}
	if output.PrivacyMode != privacyModeHashOnly && output.PrivacyMode != privacyModeDisputeEncrypted {
		return voiceCallRecord{}, fmt.Errorf("call record privacy_mode must be hash_only or dispute_encrypted")
	}
	output.EnvironmentFingerprint = strings.TrimSpace(output.EnvironmentFingerprint)

	events := make([]schemavoice.CallEvent, 0, len(output.Events))
	for _, event := range output.Events {
		normalized, err := normalizeCallEvent(event, output.CallID)
		if err != nil {
			return voiceCallRecord{}, err
		}
		events = append(events, normalized)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CallSeq != events[j].CallSeq {
			return events[i].CallSeq < events[j].CallSeq
		}
		return events[i].EventType < events[j].EventType
	})
	if err := ensureCallEventCoverage(events); err != nil {
		return voiceCallRecord{}, err
	}
	output.Events = events

	commitments := make([]schemavoice.CommitmentIntent, 0, len(output.Commitments))
	for _, commitment := range output.Commitments {
		normalized, err := gate.NormalizeCommitmentIntent(commitment)
		if err != nil {
			return voiceCallRecord{}, fmt.Errorf("normalize commitment: %w", err)
		}
		if normalized.CallID != output.CallID {
			return voiceCallRecord{}, fmt.Errorf("commitment call_id does not match call record")
		}
		commitments = append(commitments, normalized)
	}
	sort.Slice(commitments, func(i, j int) bool {
		if commitments[i].CallSeq != commitments[j].CallSeq {
			return commitments[i].CallSeq < commitments[j].CallSeq
		}
		return commitments[i].CommitmentClass < commitments[j].CommitmentClass
	})
	output.Commitments = commitments

	decisions := make([]schemavoice.GateDecision, 0, len(output.GateDecisions))
	for _, decision := range output.GateDecisions {
		normalized, err := normalizeGateDecision(decision, output.CallID)
		if err != nil {
			return voiceCallRecord{}, err
		}
		decisions = append(decisions, normalized)
	}
	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].CallSeq != decisions[j].CallSeq {
			return decisions[i].CallSeq < decisions[j].CallSeq
		}
		return decisions[i].CommitmentClass < decisions[j].CommitmentClass
	})
	output.GateDecisions = decisions

	receipts := make([]schemavoice.SpeakReceipt, 0, len(output.SpeakReceipts))
	for _, receipt := range output.SpeakReceipts {
		normalized, err := normalizeSpeakReceipt(receipt, output.CallID)
		if err != nil {
			return voiceCallRecord{}, err
		}
		receipts = append(receipts, normalized)
	}
	sort.Slice(receipts, func(i, j int) bool {
		if receipts[i].CallSeq != receipts[j].CallSeq {
			return receipts[i].CallSeq < receipts[j].CallSeq
		}
		return receipts[i].CommitmentClass < receipts[j].CommitmentClass
	})
	output.SpeakReceipts = receipts
	if err := ensureGatedTTSEventsHaveSpeakReceipt(output.Events, output.SpeakReceipts); err != nil {
		return voiceCallRecord{}, err
	}
	if err := ensureSpeakReceiptsHaveAllowDecision(output.GateDecisions, output.SpeakReceipts); err != nil {
		return voiceCallRecord{}, err
	}

	output.ReferenceDigests = normalizeReferenceDigests(output.ReferenceDigests)
	return output, nil
}

func normalizeCallEvent(input schemavoice.CallEvent, expectedCallID string) (schemavoice.CallEvent, error) {
	output := input
	if strings.TrimSpace(output.SchemaID) == "" {
		output.SchemaID = "gait.voice.call_event"
	}
	if output.SchemaID != "gait.voice.call_event" {
		return schemavoice.CallEvent{}, fmt.Errorf("unsupported call event schema_id: %s", output.SchemaID)
	}
	if strings.TrimSpace(output.SchemaVersion) == "" {
		output.SchemaVersion = "1.0.0"
	}
	if output.SchemaVersion != "1.0.0" {
		return schemavoice.CallEvent{}, fmt.Errorf("unsupported call event schema_version: %s", output.SchemaVersion)
	}
	if output.CreatedAt.IsZero() {
		return schemavoice.CallEvent{}, fmt.Errorf("call event created_at is required")
	}
	output.CreatedAt = output.CreatedAt.UTC()
	output.CallID = strings.TrimSpace(output.CallID)
	if output.CallID == "" || output.CallID != expectedCallID {
		return schemavoice.CallEvent{}, fmt.Errorf("call event call_id must match call record")
	}
	if output.CallSeq <= 0 {
		return schemavoice.CallEvent{}, fmt.Errorf("call event call_seq must be >= 1")
	}
	if output.TurnIndex < 0 {
		return schemavoice.CallEvent{}, fmt.Errorf("call event turn_index must be >= 0")
	}
	output.EventType = strings.TrimSpace(output.EventType)
	if output.EventType == "" {
		return schemavoice.CallEvent{}, fmt.Errorf("call event event_type is required")
	}
	output.CommitmentClass = strings.ToLower(strings.TrimSpace(output.CommitmentClass))
	if output.CommitmentClass != "" && !gate.IsCommitmentClass(output.CommitmentClass) {
		return schemavoice.CallEvent{}, fmt.Errorf("unsupported call event commitment_class: %s", output.CommitmentClass)
	}
	output.IntentDigest = strings.ToLower(strings.TrimSpace(output.IntentDigest))
	if output.IntentDigest != "" && !isSHA256Hex(output.IntentDigest) {
		return schemavoice.CallEvent{}, fmt.Errorf("call event intent_digest must be sha256 hex")
	}
	output.PolicyDigest = strings.ToLower(strings.TrimSpace(output.PolicyDigest))
	if output.PolicyDigest != "" && !isSHA256Hex(output.PolicyDigest) {
		return schemavoice.CallEvent{}, fmt.Errorf("call event policy_digest must be sha256 hex")
	}
	output.SayTokenID = strings.TrimSpace(output.SayTokenID)
	output.PayloadDigest = strings.ToLower(strings.TrimSpace(output.PayloadDigest))
	if output.PayloadDigest != "" && !isSHA256Hex(output.PayloadDigest) {
		return schemavoice.CallEvent{}, fmt.Errorf("call event payload_digest must be sha256 hex")
	}
	return output, nil
}

func ensureCallEventCoverage(events []schemavoice.CallEvent) error {
	if len(events) == 0 {
		return fmt.Errorf("call record events are required")
	}
	lastSeq := 0
	seenTypes := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event.CallSeq < lastSeq {
			return fmt.Errorf("call events must be sorted by call_seq")
		}
		lastSeq = event.CallSeq
		seenTypes[event.EventType] = struct{}{}
	}
	for _, requiredType := range callEventTypesRequired {
		if _, ok := seenTypes[requiredType]; !ok {
			return fmt.Errorf("call events missing required type: %s", requiredType)
		}
	}
	return nil
}

func normalizeGateDecision(input schemavoice.GateDecision, expectedCallID string) (schemavoice.GateDecision, error) {
	output := input
	output.CallID = strings.TrimSpace(output.CallID)
	if output.CallID == "" || output.CallID != expectedCallID {
		return schemavoice.GateDecision{}, fmt.Errorf("gate decision call_id must match call record")
	}
	if output.CallSeq <= 0 {
		return schemavoice.GateDecision{}, fmt.Errorf("gate decision call_seq must be >= 1")
	}
	if output.TurnIndex < 0 {
		return schemavoice.GateDecision{}, fmt.Errorf("gate decision turn_index must be >= 0")
	}
	output.CommitmentClass = strings.ToLower(strings.TrimSpace(output.CommitmentClass))
	if !gate.IsCommitmentClass(output.CommitmentClass) {
		return schemavoice.GateDecision{}, fmt.Errorf("unsupported gate decision commitment_class: %s", output.CommitmentClass)
	}
	output.Verdict = strings.ToLower(strings.TrimSpace(output.Verdict))
	switch output.Verdict {
	case "allow", "block", "dry_run", "require_approval":
	default:
		return schemavoice.GateDecision{}, fmt.Errorf("unsupported gate decision verdict: %s", output.Verdict)
	}
	output.ReasonCodes = normalizeReasonCodes(output.ReasonCodes)
	output.IntentDigest = strings.ToLower(strings.TrimSpace(output.IntentDigest))
	if output.IntentDigest != "" && !isSHA256Hex(output.IntentDigest) {
		return schemavoice.GateDecision{}, fmt.Errorf("gate decision intent_digest must be sha256 hex")
	}
	output.PolicyDigest = strings.ToLower(strings.TrimSpace(output.PolicyDigest))
	if output.PolicyDigest != "" && !isSHA256Hex(output.PolicyDigest) {
		return schemavoice.GateDecision{}, fmt.Errorf("gate decision policy_digest must be sha256 hex")
	}
	output.ApprovalRef = strings.TrimSpace(output.ApprovalRef)
	return output, nil
}

func normalizeSpeakReceipt(input schemavoice.SpeakReceipt, expectedCallID string) (schemavoice.SpeakReceipt, error) {
	output := input
	output.CallID = strings.TrimSpace(output.CallID)
	if output.CallID == "" || output.CallID != expectedCallID {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt call_id must match call record")
	}
	if output.CallSeq <= 0 {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt call_seq must be >= 1")
	}
	if output.TurnIndex < 0 {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt turn_index must be >= 0")
	}
	output.CommitmentClass = strings.ToLower(strings.TrimSpace(output.CommitmentClass))
	if !gate.IsCommitmentClass(output.CommitmentClass) {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("unsupported speak receipt commitment_class: %s", output.CommitmentClass)
	}
	output.SayTokenID = strings.TrimSpace(output.SayTokenID)
	if output.SayTokenID == "" {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt say_token_id is required")
	}
	output.SpokenDigest = strings.ToLower(strings.TrimSpace(output.SpokenDigest))
	if !isSHA256Hex(output.SpokenDigest) {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt spoken_digest must be sha256 hex")
	}
	if output.EmittedAt.IsZero() {
		return schemavoice.SpeakReceipt{}, fmt.Errorf("speak receipt emitted_at is required")
	}
	output.EmittedAt = output.EmittedAt.UTC()
	return output, nil
}

func ensureSpeakReceiptsHaveAllowDecision(decisions []schemavoice.GateDecision, receipts []schemavoice.SpeakReceipt) error {
	for _, receipt := range receipts {
		hasPriorDecision := false
		latestDecision := schemavoice.GateDecision{}
		for _, decision := range decisions {
			if decision.CommitmentClass != receipt.CommitmentClass {
				continue
			}
			if decision.TurnIndex != receipt.TurnIndex {
				continue
			}
			if decision.CallSeq > receipt.CallSeq {
				continue
			}
			if !hasPriorDecision || decision.CallSeq > latestDecision.CallSeq {
				latestDecision = decision
				hasPriorDecision = true
			}
		}
		if !hasPriorDecision {
			return fmt.Errorf(
				"speak receipt missing prior gate decision for turn_index=%d call_seq=%d class=%s",
				receipt.TurnIndex,
				receipt.CallSeq,
				receipt.CommitmentClass,
			)
		}
		if latestDecision.Verdict != "allow" {
			return fmt.Errorf(
				"speak receipt latest prior gate verdict is %s (expected allow) for turn_index=%d call_seq=%d class=%s",
				latestDecision.Verdict,
				receipt.TurnIndex,
				receipt.CallSeq,
				receipt.CommitmentClass,
			)
		}
	}
	return nil
}

func ensureGatedTTSEventsHaveSpeakReceipt(events []schemavoice.CallEvent, receipts []schemavoice.SpeakReceipt) error {
	receiptByKey := make(map[string]schemavoice.SpeakReceipt, len(receipts))
	for _, receipt := range receipts {
		key := fmt.Sprintf("%d:%d:%s", receipt.CallSeq, receipt.TurnIndex, receipt.CommitmentClass)
		receiptByKey[key] = receipt
	}
	for _, event := range events {
		if event.EventType != "tts.emitted" {
			continue
		}
		if strings.TrimSpace(event.CommitmentClass) == "" {
			continue
		}
		if strings.TrimSpace(event.SayTokenID) == "" {
			return fmt.Errorf("gated tts.emitted event missing say_token_id for call_seq=%d turn_index=%d", event.CallSeq, event.TurnIndex)
		}
		key := fmt.Sprintf("%d:%d:%s", event.CallSeq, event.TurnIndex, event.CommitmentClass)
		receipt, ok := receiptByKey[key]
		if !ok {
			return fmt.Errorf(
				"gated tts.emitted event missing speak receipt for call_seq=%d turn_index=%d class=%s",
				event.CallSeq,
				event.TurnIndex,
				event.CommitmentClass,
			)
		}
		if receipt.SayTokenID != event.SayTokenID {
			return fmt.Errorf(
				"gated tts.emitted event say_token_id mismatch for call_seq=%d turn_index=%d class=%s",
				event.CallSeq,
				event.TurnIndex,
				event.CommitmentClass,
			)
		}
	}
	return nil
}

func normalizeReferenceDigests(values []schemavoice.ReferenceDigest) []schemavoice.ReferenceDigest {
	if len(values) == 0 {
		return nil
	}
	out := make([]schemavoice.ReferenceDigest, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		refID := strings.TrimSpace(value.RefID)
		digest := strings.ToLower(strings.TrimSpace(value.SHA256))
		if refID == "" || !isSHA256Hex(digest) {
			continue
		}
		key := refID + "\x00" + digest
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, schemavoice.ReferenceDigest{RefID: refID, SHA256: digest})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RefID != out[j].RefID {
			return out[i].RefID < out[j].RefID
		}
		return out[i].SHA256 < out[j].SHA256
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeReasonCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func verifyCallPayloadContracts(bundle *openedZip, manifest schemapack.Manifest) error {
	payloadFile, ok := bundle.Files["call_payload.json"]
	if !ok {
		return fmt.Errorf("missing call_payload.json")
	}
	callManifestFile, ok := bundle.Files["callpack_manifest.json"]
	if !ok {
		return fmt.Errorf("missing callpack_manifest.json")
	}
	eventsFile, ok := bundle.Files["call_events.jsonl"]
	if !ok {
		return fmt.Errorf("missing call_events.jsonl")
	}
	commitmentsFile, ok := bundle.Files["commitments.jsonl"]
	if !ok {
		return fmt.Errorf("missing commitments.jsonl")
	}
	decisionsFile, ok := bundle.Files["gate_decisions.jsonl"]
	if !ok {
		return fmt.Errorf("missing gate_decisions.jsonl")
	}
	speakFile, ok := bundle.Files["speak_receipts.jsonl"]
	if !ok {
		return fmt.Errorf("missing speak_receipts.jsonl")
	}
	referencesFile, ok := bundle.Files["reference_digests.json"]
	if !ok {
		return fmt.Errorf("missing reference_digests.json")
	}
	if _, ok := bundle.Files["source/runpack.zip"]; !ok {
		return fmt.Errorf("missing source/runpack.zip")
	}

	payloadBytes, err := readZipFile(payloadFile)
	if err != nil {
		return fmt.Errorf("read call_payload.json: %w", err)
	}
	var payload schemapack.CallPayload
	if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
		return fmt.Errorf("parse call_payload.json: %w", err)
	}
	if err := validateCallPayload(payload); err != nil {
		return err
	}
	if payload.CallID != strings.TrimSpace(manifest.SourceRef) {
		return fmt.Errorf("call payload call_id does not match manifest source_ref")
	}

	callManifestBytes, err := readZipFile(callManifestFile)
	if err != nil {
		return fmt.Errorf("read callpack_manifest.json: %w", err)
	}
	var callManifest schemavoice.CallpackManifest
	if err := decodeStrictJSON(callManifestBytes, &callManifest); err != nil {
		return fmt.Errorf("parse callpack_manifest.json: %w", err)
	}
	if err := validateCallpackManifest(callManifest); err != nil {
		return err
	}
	if callManifest.CallID != payload.CallID {
		return fmt.Errorf("callpack manifest call_id does not match call payload")
	}

	eventsBytes, err := readZipFile(eventsFile)
	if err != nil {
		return fmt.Errorf("read call_events.jsonl: %w", err)
	}
	events, err := parseCallEvents(eventsBytes)
	if err != nil {
		return err
	}
	normalizedEvents := make([]schemavoice.CallEvent, 0, len(events))
	for _, event := range events {
		normalized, err := normalizeCallEvent(event, payload.CallID)
		if err != nil {
			return fmt.Errorf("normalize call event: %w", err)
		}
		normalizedEvents = append(normalizedEvents, normalized)
	}
	events = normalizedEvents
	if err := ensureCallEventCoverage(events); err != nil {
		return err
	}

	commitmentBytes, err := readZipFile(commitmentsFile)
	if err != nil {
		return fmt.Errorf("read commitments.jsonl: %w", err)
	}
	commitments, err := parseCommitments(commitmentBytes)
	if err != nil {
		return err
	}

	decisionBytes, err := readZipFile(decisionsFile)
	if err != nil {
		return fmt.Errorf("read gate_decisions.jsonl: %w", err)
	}
	decisions, err := parseGateDecisions(decisionBytes)
	if err != nil {
		return err
	}
	normalizedDecisions := make([]schemavoice.GateDecision, 0, len(decisions))
	for _, decision := range decisions {
		normalized, err := normalizeGateDecision(decision, payload.CallID)
		if err != nil {
			return fmt.Errorf("normalize gate decision: %w", err)
		}
		normalizedDecisions = append(normalizedDecisions, normalized)
	}
	decisions = normalizedDecisions

	speakBytes, err := readZipFile(speakFile)
	if err != nil {
		return fmt.Errorf("read speak_receipts.jsonl: %w", err)
	}
	speakReceipts, err := parseSpeakReceipts(speakBytes)
	if err != nil {
		return err
	}
	normalizedSpeakReceipts := make([]schemavoice.SpeakReceipt, 0, len(speakReceipts))
	for _, receipt := range speakReceipts {
		normalized, err := normalizeSpeakReceipt(receipt, payload.CallID)
		if err != nil {
			return fmt.Errorf("normalize speak receipt: %w", err)
		}
		normalizedSpeakReceipts = append(normalizedSpeakReceipts, normalized)
	}
	speakReceipts = normalizedSpeakReceipts
	if err := ensureGatedTTSEventsHaveSpeakReceipt(events, speakReceipts); err != nil {
		return err
	}

	referenceBytes, err := readZipFile(referencesFile)
	if err != nil {
		return fmt.Errorf("read reference_digests.json: %w", err)
	}
	var references []schemavoice.ReferenceDigest
	if err := decodeStrictJSON(referenceBytes, &references); err != nil {
		return fmt.Errorf("parse reference_digests.json: %w", err)
	}
	references = normalizeReferenceDigests(references)

	for _, commitment := range commitments {
		if commitment.CallID != payload.CallID {
			return fmt.Errorf("commitment call_id does not match payload")
		}
	}
	if err := ensureSpeakReceiptsHaveAllowDecision(decisions, speakReceipts); err != nil {
		return err
	}
	if payload.EventCount != len(events) {
		return fmt.Errorf("call payload event_count does not match call_events")
	}
	if payload.CommitmentCount != len(commitments) {
		return fmt.Errorf("call payload commitment_count does not match commitments")
	}
	if payload.DecisionCount != len(decisions) {
		return fmt.Errorf("call payload decision_count does not match gate_decisions")
	}
	if payload.SpeakReceiptCount != len(speakReceipts) {
		return fmt.Errorf("call payload speak_receipt_count does not match speak_receipts")
	}
	if payload.ReferenceDigestCount != len(references) {
		return fmt.Errorf("call payload reference_digest_count does not match reference_digests")
	}
	if callManifest.EventCount != payload.EventCount ||
		callManifest.CommitmentCount != payload.CommitmentCount ||
		callManifest.DecisionCount != payload.DecisionCount ||
		callManifest.SpeakReceiptCount != payload.SpeakReceiptCount ||
		callManifest.ReferenceDigestCount != payload.ReferenceDigestCount {
		return fmt.Errorf("callpack manifest counts do not match call payload")
	}
	if callManifest.PrivacyMode != payload.PrivacyMode {
		return fmt.Errorf("callpack manifest privacy_mode does not match call payload")
	}
	if strings.TrimSpace(callManifest.EnvironmentFingerprint) != strings.TrimSpace(payload.EnvironmentFingerprint) {
		return fmt.Errorf("callpack manifest environment_fingerprint does not match call payload")
	}
	if !callManifest.CreatedAt.Equal(payload.CreatedAt) {
		return fmt.Errorf("callpack manifest created_at does not match call payload")
	}
	return nil
}

func validateCallPayload(payload schemapack.CallPayload) error {
	if payload.SchemaID != callPayloadSchemaID {
		return fmt.Errorf("call payload schema_id must be gait.pack.call")
	}
	if payload.SchemaVersion != callPayloadSchemaVersion {
		return fmt.Errorf("call payload schema_version must be 1.0.0")
	}
	if payload.CreatedAt.IsZero() {
		return fmt.Errorf("call payload created_at is required")
	}
	if strings.TrimSpace(payload.CallID) == "" {
		return fmt.Errorf("call payload call_id is required")
	}
	if payload.PrivacyMode != privacyModeHashOnly && payload.PrivacyMode != privacyModeDisputeEncrypted {
		return fmt.Errorf("call payload privacy_mode must be hash_only or dispute_encrypted")
	}
	if payload.EventCount < 0 || payload.CommitmentCount < 0 || payload.DecisionCount < 0 || payload.SpeakReceiptCount < 0 || payload.ReferenceDigestCount < 0 {
		return fmt.Errorf("call payload counts must be >= 0")
	}
	return nil
}

func validateCallpackManifest(manifest schemavoice.CallpackManifest) error {
	if manifest.SchemaID != callpackManifestSchemaID {
		return fmt.Errorf("callpack manifest schema_id must be gait.voice.callpack_manifest")
	}
	if manifest.SchemaVersion != callpackManifestSchemaV1 {
		return fmt.Errorf("callpack manifest schema_version must be 1.0.0")
	}
	if manifest.CreatedAt.IsZero() {
		return fmt.Errorf("callpack manifest created_at is required")
	}
	if strings.TrimSpace(manifest.ProducerVersion) == "" {
		return fmt.Errorf("callpack manifest producer_version is required")
	}
	if strings.TrimSpace(manifest.CallID) == "" {
		return fmt.Errorf("callpack manifest call_id is required")
	}
	if manifest.PrivacyMode != privacyModeHashOnly && manifest.PrivacyMode != privacyModeDisputeEncrypted {
		return fmt.Errorf("callpack manifest privacy_mode must be hash_only or dispute_encrypted")
	}
	if manifest.EventCount < 0 || manifest.CommitmentCount < 0 || manifest.DecisionCount < 0 || manifest.SpeakReceiptCount < 0 || manifest.ReferenceDigestCount < 0 {
		return fmt.Errorf("callpack manifest counts must be >= 0")
	}
	return nil
}

func parseCallEvents(payload []byte) ([]schemavoice.CallEvent, error) {
	return parseJSONL(payload, "call_events.jsonl", func(raw []byte) (schemavoice.CallEvent, error) {
		var event schemavoice.CallEvent
		if err := decodeStrictJSON(raw, &event); err != nil {
			return schemavoice.CallEvent{}, err
		}
		return event, nil
	})
}

func parseCommitments(payload []byte) ([]schemavoice.CommitmentIntent, error) {
	return parseJSONL(payload, "commitments.jsonl", func(raw []byte) (schemavoice.CommitmentIntent, error) {
		var commitment schemavoice.CommitmentIntent
		if err := decodeStrictJSON(raw, &commitment); err != nil {
			return schemavoice.CommitmentIntent{}, err
		}
		normalized, err := gate.NormalizeCommitmentIntent(commitment)
		if err != nil {
			return schemavoice.CommitmentIntent{}, err
		}
		return normalized, nil
	})
}

func parseGateDecisions(payload []byte) ([]schemavoice.GateDecision, error) {
	return parseJSONL(payload, "gate_decisions.jsonl", func(raw []byte) (schemavoice.GateDecision, error) {
		var decision schemavoice.GateDecision
		if err := decodeStrictJSON(raw, &decision); err != nil {
			return schemavoice.GateDecision{}, err
		}
		return decision, nil
	})
}

func parseSpeakReceipts(payload []byte) ([]schemavoice.SpeakReceipt, error) {
	return parseJSONL(payload, "speak_receipts.jsonl", func(raw []byte) (schemavoice.SpeakReceipt, error) {
		var receipt schemavoice.SpeakReceipt
		if err := decodeStrictJSON(raw, &receipt); err != nil {
			return schemavoice.SpeakReceipt{}, err
		}
		return receipt, nil
	})
}

func parseJSONL[T any](payload []byte, fileName string, decode func(raw []byte) (T, error)) ([]T, error) {
	if len(payload) == 0 {
		return []T{}, nil
	}
	items := []T{}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		item, err := decode(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", fileName, line, err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", fileName, err)
	}
	return items, nil
}
