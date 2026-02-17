package pack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	schemapack "github.com/Clyra-AI/gait/core/schema/v1/pack"
	schemavoice "github.com/Clyra-AI/gait/core/schema/v1/voice"
)

func TestBuildCallPackVerifyInspectAndExtractRunpack(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_case")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_case")

	result, err := BuildCallPack(BuildCallOptions{
		CallRecordPath:  callRecordPath,
		OutputPath:      filepath.Join(workDir, "callpack.zip"),
		ProducerVersion: "test-vvoice",
	})
	if err != nil {
		t.Fatalf("build call pack: %v", err)
	}
	if result.Manifest.PackType != string(BuildTypeCall) {
		t.Fatalf("unexpected call pack type: %s", result.Manifest.PackType)
	}
	verifyResult, err := Verify(result.Path, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify call pack: %v", err)
	}
	if verifyResult.PackType != string(BuildTypeCall) {
		t.Fatalf("unexpected verify pack type: %s", verifyResult.PackType)
	}
	if len(verifyResult.HashMismatches) > 0 || len(verifyResult.MissingFiles) > 0 {
		t.Fatalf("expected clean verify result, got %#v", verifyResult)
	}
	inspectResult, err := Inspect(result.Path)
	if err != nil {
		t.Fatalf("inspect call pack: %v", err)
	}
	if inspectResult.CallPayload == nil {
		t.Fatalf("expected call payload in inspect output")
	}
	if inspectResult.CallPayload.CallID != "call_voice_case" {
		t.Fatalf("unexpected call payload call_id: %s", inspectResult.CallPayload.CallID)
	}
	diffResult, err := Diff(result.Path, result.Path)
	if err != nil {
		t.Fatalf("diff identical call packs: %v", err)
	}
	if diffResult.Result.Summary.Changed {
		t.Fatalf("identical callpack diff should not report changed")
	}
	extractedRunpack, err := ExtractRunpack(result.Path)
	if err != nil {
		t.Fatalf("extract runpack from callpack: %v", err)
	}
	originalRunpack, err := os.ReadFile(runpackPath)
	if err != nil {
		t.Fatalf("read source runpack: %v", err)
	}
	if !bytes.Equal(extractedRunpack, originalRunpack) {
		t.Fatalf("expected extracted runpack bytes to match source")
	}
}

func TestVerifyRejectsCallPackSpeakWithoutToken(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_missing_token")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_missing_token")

	result, err := BuildCallPack(BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     filepath.Join(workDir, "callpack.zip"),
	})
	if err != nil {
		t.Fatalf("build call pack: %v", err)
	}

	mutatedPath := filepath.Join(workDir, "callpack_missing_token.zip")
	invalidSpeak := []byte(`{"call_id":"call_voice_missing_token","call_seq":5,"turn_index":2,"commitment_class":"quote","say_token_id":"","spoken_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","emitted_at":"2026-02-15T00:00:05Z"}` + "\n")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, mutatedPath, map[string][]byte{
		"speak_receipts.jsonl": invalidSpeak,
	}); err != nil {
		t.Fatalf("rewrite callpack with invalid speak receipt: %v", err)
	}
	if _, err := Verify(mutatedPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify to fail when speak receipt has missing say_token_id")
	} else if coreerrors.CategoryOf(err) != coreerrors.CategoryVerification {
		t.Fatalf("expected verification-category error, got %q (%v)", coreerrors.CategoryOf(err), err)
	}
}

func TestBuildCallPackRejectsSpeakWithoutPriorAllowDecision(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_allow_order")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_allow_order")

	recordBytes, err := os.ReadFile(callRecordPath)
	if err != nil {
		t.Fatalf("read call record: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatalf("decode call record: %v", err)
	}
	decisions, ok := record["gate_decisions"].([]any)
	if !ok || len(decisions) == 0 {
		t.Fatalf("expected gate_decisions in call record")
	}
	firstDecision, ok := decisions[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected gate_decisions shape")
	}
	firstDecision["call_seq"] = 9
	mutated, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("encode mutated call record: %v", err)
	}
	mutated = append(mutated, '\n')
	if err := os.WriteFile(callRecordPath, mutated, 0o600); err != nil {
		t.Fatalf("write mutated call record: %v", err)
	}

	_, err = BuildCallPack(BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     filepath.Join(workDir, "callpack.zip"),
	})
	if err == nil {
		t.Fatalf("expected callpack build to fail without prior allow decision")
	}
	if !strings.Contains(err.Error(), "missing prior gate decision") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCallPackRejectsSpeakAfterLatestNonAllowDecision(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_latest_verdict")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_latest_verdict")

	recordBytes, err := os.ReadFile(callRecordPath)
	if err != nil {
		t.Fatalf("read call record: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatalf("decode call record: %v", err)
	}
	decisions, ok := record["gate_decisions"].([]any)
	if !ok {
		t.Fatalf("expected gate_decisions array")
	}
	decisions = append(decisions, map[string]any{
		"call_id":          "call_voice_latest_verdict",
		"call_seq":         4,
		"turn_index":       2,
		"commitment_class": "quote",
		"verdict":          "block",
		"reason_codes":     []string{"blocked_after_allow"},
		"intent_digest":    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"policy_digest":    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"approval_ref":     "",
	})
	record["gate_decisions"] = decisions
	mutated, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("encode mutated call record: %v", err)
	}
	mutated = append(mutated, '\n')
	if err := os.WriteFile(callRecordPath, mutated, 0o600); err != nil {
		t.Fatalf("write mutated call record: %v", err)
	}

	_, err = BuildCallPack(BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     filepath.Join(workDir, "callpack.zip"),
	})
	if err == nil {
		t.Fatalf("expected callpack build to fail when latest gate verdict before speech is non-allow")
	}
	if !strings.Contains(err.Error(), "latest prior gate verdict is block") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCallPackRejectsGatedEmitWithoutSpeakReceipt(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_missing_receipt")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_missing_receipt")

	recordBytes, err := os.ReadFile(callRecordPath)
	if err != nil {
		t.Fatalf("read call record: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatalf("decode call record: %v", err)
	}
	record["speak_receipts"] = []any{}
	mutated, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("encode mutated call record: %v", err)
	}
	mutated = append(mutated, '\n')
	if err := os.WriteFile(callRecordPath, mutated, 0o600); err != nil {
		t.Fatalf("write mutated call record: %v", err)
	}

	_, err = BuildCallPack(BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     filepath.Join(workDir, "callpack.zip"),
	})
	if err == nil {
		t.Fatalf("expected callpack build to fail without speak receipt for gated tts.emitted")
	}
	if !strings.Contains(err.Error(), "missing speak receipt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsCallPackManifestPrivacyMismatch(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_privacy_mismatch")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_privacy_mismatch")

	result, err := BuildCallPack(BuildCallOptions{
		CallRecordPath: callRecordPath,
		OutputPath:     filepath.Join(workDir, "callpack.zip"),
	})
	if err != nil {
		t.Fatalf("build call pack: %v", err)
	}

	mutatedPath := filepath.Join(workDir, "callpack_manifest_privacy_mismatch.zip")
	mutatedManifest := []byte(`{"schema_id":"gait.voice.callpack_manifest","schema_version":"1.0.0","created_at":"2026-02-15T00:00:00Z","producer_version":"test","call_id":"call_voice_privacy_mismatch","privacy_mode":"dispute_encrypted","event_count":8,"commitment_count":1,"decision_count":1,"speak_receipt_count":1,"reference_digest_count":1}` + "\n")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, mutatedPath, map[string][]byte{
		"callpack_manifest.json": mutatedManifest,
	}); err != nil {
		t.Fatalf("rewrite callpack with privacy mismatch: %v", err)
	}

	if _, err := Verify(mutatedPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected verify to fail on callpack manifest privacy mismatch")
	} else if coreerrors.CategoryOf(err) != coreerrors.CategoryVerification {
		t.Fatalf("expected verification-category error, got %q (%v)", coreerrors.CategoryOf(err), err)
	} else if !strings.Contains(err.Error(), "privacy_mode does not match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCallPayloadAndManifestContracts(t *testing.T) {
	createdAt := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	basePayload := schemapack.CallPayload{
		SchemaID:             callPayloadSchemaID,
		SchemaVersion:        callPayloadSchemaVersion,
		CreatedAt:            createdAt,
		CallID:               "call_demo",
		PrivacyMode:          privacyModeHashOnly,
		EventCount:           1,
		CommitmentCount:      1,
		DecisionCount:        1,
		SpeakReceiptCount:    1,
		ReferenceDigestCount: 1,
	}
	baseManifest := schemavoice.CallpackManifest{
		SchemaID:             callpackManifestSchemaID,
		SchemaVersion:        callpackManifestSchemaV1,
		CreatedAt:            createdAt,
		ProducerVersion:      "test",
		CallID:               "call_demo",
		PrivacyMode:          privacyModeHashOnly,
		EventCount:           1,
		CommitmentCount:      1,
		DecisionCount:        1,
		SpeakReceiptCount:    1,
		ReferenceDigestCount: 1,
	}
	if err := validateCallPayload(basePayload); err != nil {
		t.Fatalf("expected valid call payload, got %v", err)
	}
	if err := validateCallpackManifest(baseManifest); err != nil {
		t.Fatalf("expected valid callpack manifest, got %v", err)
	}

	payloadCases := []struct {
		name    string
		mutate  func(*schemapack.CallPayload)
		wantErr string
	}{
		{name: "schema_id", mutate: func(payload *schemapack.CallPayload) { payload.SchemaID = "bad" }, wantErr: "schema_id"},
		{name: "schema_version", mutate: func(payload *schemapack.CallPayload) { payload.SchemaVersion = "2.0.0" }, wantErr: "schema_version"},
		{name: "created_at", mutate: func(payload *schemapack.CallPayload) { payload.CreatedAt = time.Time{} }, wantErr: "created_at"},
		{name: "call_id", mutate: func(payload *schemapack.CallPayload) { payload.CallID = "" }, wantErr: "call_id"},
		{name: "privacy_mode", mutate: func(payload *schemapack.CallPayload) { payload.PrivacyMode = "cleartext" }, wantErr: "privacy_mode"},
		{name: "counts", mutate: func(payload *schemapack.CallPayload) { payload.EventCount = -1 }, wantErr: "counts must be >= 0"},
	}
	for _, testCase := range payloadCases {
		t.Run("payload_"+testCase.name, func(t *testing.T) {
			candidate := basePayload
			testCase.mutate(&candidate)
			if err := validateCallPayload(candidate); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected payload error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}

	manifestCases := []struct {
		name    string
		mutate  func(*schemavoice.CallpackManifest)
		wantErr string
	}{
		{name: "schema_id", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.SchemaID = "bad" }, wantErr: "schema_id"},
		{name: "schema_version", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.SchemaVersion = "2.0.0" }, wantErr: "schema_version"},
		{name: "created_at", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.CreatedAt = time.Time{} }, wantErr: "created_at"},
		{name: "producer_version", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.ProducerVersion = " " }, wantErr: "producer_version"},
		{name: "call_id", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.CallID = " " }, wantErr: "call_id"},
		{name: "privacy_mode", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.PrivacyMode = "cleartext" }, wantErr: "privacy_mode"},
		{name: "counts", mutate: func(manifest *schemavoice.CallpackManifest) { manifest.DecisionCount = -1 }, wantErr: "counts must be >= 0"},
	}
	for _, testCase := range manifestCases {
		t.Run("manifest_"+testCase.name, func(t *testing.T) {
			candidate := baseManifest
			testCase.mutate(&candidate)
			if err := validateCallpackManifest(candidate); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected manifest error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestNormalizeReferenceDigestsAndVoiceRows(t *testing.T) {
	referenceInput := []schemavoice.ReferenceDigest{
		{RefID: "ref_b", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{RefID: "ref_a", SHA256: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{RefID: "ref_a", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{RefID: "", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{RefID: "ref_bad", SHA256: "not_a_digest"},
	}
	references := normalizeReferenceDigests(referenceInput)
	if len(references) != 2 {
		t.Fatalf("expected 2 normalized reference digests, got %#v", references)
	}
	if references[0].RefID != "ref_a" || references[1].RefID != "ref_b" {
		t.Fatalf("expected sorted references, got %#v", references)
	}

	createdAt := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	validEvent := schemavoice.CallEvent{
		SchemaID:        "gait.voice.call_event",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		CallID:          "call_demo",
		CallSeq:         1,
		TurnIndex:       0,
		EventType:       "tts.emitted",
		CommitmentClass: "quote",
		IntentDigest:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		PolicyDigest:    "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		PayloadDigest:   "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
		SayTokenID:      "say_demo",
	}
	normalizedEvent, err := normalizeCallEvent(validEvent, "call_demo")
	if err != nil {
		t.Fatalf("normalize call event: %v", err)
	}
	if normalizedEvent.IntentDigest != strings.ToLower(validEvent.IntentDigest) {
		t.Fatalf("expected lowercase intent digest, got %q", normalizedEvent.IntentDigest)
	}

	invalidEvent := validEvent
	invalidEvent.PayloadDigest = "bad"
	if _, err := normalizeCallEvent(invalidEvent, "call_demo"); err == nil || !strings.Contains(err.Error(), "payload_digest") {
		t.Fatalf("expected invalid payload digest error, got %v", err)
	}

	validDecision := schemavoice.GateDecision{
		CallID:          "call_demo",
		CallSeq:         2,
		TurnIndex:       0,
		CommitmentClass: "quote",
		Verdict:         "allow",
		ReasonCodes:     []string{"allow_rule", " allow_rule ", "second"},
		IntentDigest:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		PolicyDigest:    "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
	}
	normalizedDecision, err := normalizeGateDecision(validDecision, "call_demo")
	if err != nil {
		t.Fatalf("normalize gate decision: %v", err)
	}
	if len(normalizedDecision.ReasonCodes) != 2 {
		t.Fatalf("expected deduped reason codes, got %#v", normalizedDecision.ReasonCodes)
	}

	invalidDecision := validDecision
	invalidDecision.Verdict = "unknown"
	if _, err := normalizeGateDecision(invalidDecision, "call_demo"); err == nil || !strings.Contains(err.Error(), "unsupported gate decision verdict") {
		t.Fatalf("expected verdict validation error, got %v", err)
	}

	validReceipt := schemavoice.SpeakReceipt{
		CallID:          "call_demo",
		CallSeq:         3,
		TurnIndex:       0,
		CommitmentClass: "quote",
		SayTokenID:      "say_demo",
		SpokenDigest:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		EmittedAt:       createdAt,
	}
	normalizedReceipt, err := normalizeSpeakReceipt(validReceipt, "call_demo")
	if err != nil {
		t.Fatalf("normalize speak receipt: %v", err)
	}
	if normalizedReceipt.SpokenDigest != strings.ToLower(validReceipt.SpokenDigest) {
		t.Fatalf("expected lowercase spoken digest, got %q", normalizedReceipt.SpokenDigest)
	}

	invalidReceipt := validReceipt
	invalidReceipt.SayTokenID = " "
	if _, err := normalizeSpeakReceipt(invalidReceipt, "call_demo"); err == nil || !strings.Contains(err.Error(), "say_token_id is required") {
		t.Fatalf("expected missing say_token_id error, got %v", err)
	}
}

func TestNormalizeCallRecordValidationPaths(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixture(t, workDir, "run_voice_record_validation")
	callRecordPath := writeCallRecordFixture(t, workDir, runpackPath, "call_voice_record_validation")

	recordBytes, err := os.ReadFile(callRecordPath)
	if err != nil {
		t.Fatalf("read call record fixture: %v", err)
	}
	var base voiceCallRecord
	if err := json.Unmarshal(recordBytes, &base); err != nil {
		t.Fatalf("decode call record fixture: %v", err)
	}
	if _, err := normalizeCallRecord(base); err != nil {
		t.Fatalf("expected baseline call record to normalize, got %v", err)
	}

	clone := func(input voiceCallRecord) voiceCallRecord {
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("marshal clone source: %v", err)
		}
		var copied voiceCallRecord
		if err := json.Unmarshal(raw, &copied); err != nil {
			t.Fatalf("unmarshal clone: %v", err)
		}
		return copied
	}

	cases := []struct {
		name    string
		mutate  func(*voiceCallRecord)
		wantErr string
	}{
		{name: "schema_id", mutate: func(record *voiceCallRecord) { record.SchemaID = "bad" }, wantErr: "unsupported call record schema_id"},
		{name: "schema_version", mutate: func(record *voiceCallRecord) { record.SchemaVersion = "2.0.0" }, wantErr: "unsupported call record schema_version"},
		{name: "created_at", mutate: func(record *voiceCallRecord) { record.CreatedAt = time.Time{} }, wantErr: "created_at is required"},
		{name: "call_id", mutate: func(record *voiceCallRecord) { record.CallID = "" }, wantErr: "call_id is required"},
		{name: "runpack_path", mutate: func(record *voiceCallRecord) { record.RunpackPath = "" }, wantErr: "runpack_path is required"},
		{name: "privacy_mode", mutate: func(record *voiceCallRecord) { record.PrivacyMode = "cleartext" }, wantErr: "privacy_mode must be hash_only or dispute_encrypted"},
		{
			name: "missing_required_event_type",
			mutate: func(record *voiceCallRecord) {
				record.Events = record.Events[1:]
			},
			wantErr: "missing required type",
		},
		{
			name: "commitment_call_id_mismatch",
			mutate: func(record *voiceCallRecord) {
				record.Commitments[0].CallID = "other_call"
			},
			wantErr: "commitment call_id does not match call record",
		},
		{
			name: "decision_invalid_verdict",
			mutate: func(record *voiceCallRecord) {
				record.GateDecisions[0].Verdict = "unknown"
			},
			wantErr: "unsupported gate decision verdict",
		},
		{
			name: "gated_event_missing_receipt",
			mutate: func(record *voiceCallRecord) {
				record.SpeakReceipts = nil
			},
			wantErr: "missing speak receipt",
		},
		{
			name: "gated_event_token_mismatch",
			mutate: func(record *voiceCallRecord) {
				record.SpeakReceipts[0].SayTokenID = "different"
			},
			wantErr: "say_token_id mismatch",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := clone(base)
			testCase.mutate(&candidate)
			if _, err := normalizeCallRecord(candidate); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected normalizeCallRecord error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func writeCallRecordFixture(t *testing.T, dir string, runpackPath string, callID string) string {
	t.Helper()
	createdAt := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	record := map[string]any{
		"schema_id":        callRecordSchemaID,
		"schema_version":   callRecordSchemaVersion,
		"created_at":       createdAt.Format(time.RFC3339Nano),
		"producer_version": "test",
		"call_id":          callID,
		"runpack_path":     runpackPath,
		"privacy_mode":     "hash_only",
		"events": []map[string]any{
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Format(time.RFC3339Nano), "call_id": callID, "call_seq": 1, "turn_index": 1, "event_type": "asr.final", "payload_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(1 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 2, "turn_index": 2, "event_type": "commitment.declared", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(2 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 3, "turn_index": 2, "event_type": "gate.decision", "commitment_class": "quote", "intent_digest": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "policy_digest": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(3 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 4, "turn_index": 2, "event_type": "tts.request", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(4 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 5, "turn_index": 2, "event_type": "tts.emitted", "commitment_class": "quote", "say_token_id": "say_demo"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(5 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 6, "turn_index": 2, "event_type": "tool.intent"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(6 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 7, "turn_index": 2, "event_type": "tool.result"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(7 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 8, "turn_index": 2, "event_type": "approval.granted", "commitment_class": "quote"},
		},
		"commitments": []schemavoice.CommitmentIntent{
			{
				SchemaID:        "gait.voice.commitment_intent",
				SchemaVersion:   "1.0.0",
				CreatedAt:       createdAt.Add(1 * time.Second),
				ProducerVersion: "test",
				CallID:          callID,
				TurnIndex:       2,
				CallSeq:         2,
				CommitmentClass: "quote",
				Context: schemavoice.CommitmentContext{
					Identity:  "agent.voice",
					Workspace: "/srv/voice",
					RiskClass: "high",
				},
				QuoteMinCents: 1000,
				QuoteMaxCents: 1200,
			},
		},
		"gate_decisions": []schemavoice.GateDecision{
			{
				CallID:          callID,
				CallSeq:         3,
				TurnIndex:       2,
				CommitmentClass: "quote",
				Verdict:         "allow",
				IntentDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PolicyDigest:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				ReasonCodes:     []string{"allow_rule"},
			},
		},
		"speak_receipts": []schemavoice.SpeakReceipt{
			{
				CallID:          callID,
				CallSeq:         5,
				TurnIndex:       2,
				CommitmentClass: "quote",
				SayTokenID:      "say_demo",
				SpokenDigest:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				EmittedAt:       createdAt.Add(4 * time.Second),
			},
		},
		"reference_digests": []schemavoice.ReferenceDigest{
			{RefID: "ref_1", SHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
		},
	}
	recordPath := filepath.Join(dir, "call_record.json")
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal call record: %v", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(recordPath, content, 0o600); err != nil {
		t.Fatalf("write call record: %v", err)
	}
	return recordPath
}
