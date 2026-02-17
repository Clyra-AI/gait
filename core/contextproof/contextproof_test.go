package contextproof

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestNormalizeEnvelopeComputesDigest(t *testing.T) {
	envelope := schemacontext.Envelope{
		ContextSetID: "ctx_set_test",
		EvidenceMode: EvidenceModeRequired,
		Records: []schemacontext.ReferenceRecord{
			{
				RefID:         "ctx_a",
				SourceType:    "doc_store",
				SourceLocator: "docs://a",
				QueryDigest:   "1111111111111111111111111111111111111111111111111111111111111111",
				ContentDigest: "2222222222222222222222222222222222222222222222222222222222222222",
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
				Immutability:  "immutable",
			},
		},
	}
	normalized, err := NormalizeEnvelope(envelope)
	if err != nil {
		t.Fatalf("normalize envelope: %v", err)
	}
	if normalized.ContextSetDigest == "" {
		t.Fatalf("expected computed context_set_digest")
	}
}

func TestClassifyRefsDrift(t *testing.T) {
	left := schemarunpack.Refs{
		SchemaID:            "gait.runpack.refs",
		SchemaVersion:       "1.0.0",
		RunID:               "run_demo",
		ContextEvidenceMode: EvidenceModeRequired,
		ContextRefCount:     1,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ctx_1",
				SourceType:    "doc_store",
				SourceLocator: "docs://a",
				QueryDigest:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ContentDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
			},
		},
	}
	right := left
	right.Receipts = append([]schemarunpack.RefReceipt(nil), left.Receipts...)
	right.Receipts[0].RetrievedAt = left.Receipts[0].RetrievedAt.Add(5 * time.Minute)

	classification, changed, runtimeOnly, err := ClassifyRefsDrift(left, right)
	if err != nil {
		t.Fatalf("classify refs drift: %v", err)
	}
	if classification != driftRuntimeOnly {
		t.Fatalf("expected runtime_only classification, got %s", classification)
	}
	if !changed || !runtimeOnly {
		t.Fatalf("expected changed/runtimeOnly true, got changed=%t runtimeOnly=%t", changed, runtimeOnly)
	}
}

func TestBuildAndDigestEnvelope(t *testing.T) {
	built, err := BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx_2",
			SourceType:    "doc_store",
			SourceLocator: "docs://b",
			QueryDigest:   strings.Repeat("1", 64),
			ContentDigest: strings.Repeat("2", 64),
			RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode: "reference",
			Immutability:  "immutable",
		},
	}, BuildEnvelopeOptions{
		ContextSetID:    "ctx_set_build",
		EvidenceMode:    EvidenceModeRequired,
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	digest, err := DigestEnvelope(built)
	if err != nil {
		t.Fatalf("digest envelope: %v", err)
	}
	if digest == "" {
		t.Fatalf("expected digest")
	}
	if digest != built.ContextSetDigest {
		t.Fatalf("digest mismatch: got=%s want=%s", digest, built.ContextSetDigest)
	}
}

func TestTransformEnvelopePrivacy(t *testing.T) {
	envelope, err := BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx_3",
			SourceType:    "doc_store",
			SourceLocator: "docs://c",
			QueryDigest:   strings.Repeat("1", 64),
			ContentDigest: strings.Repeat("2", 64),
			RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode: "reference",
			Immutability:  "immutable",
		},
	}, BuildEnvelopeOptions{
		ContextSetID: "ctx_set_privacy",
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	metadata, err := TransformEnvelopePrivacy(envelope, PrivacyModeMetadata)
	if err != nil {
		t.Fatalf("metadata transform: %v", err)
	}
	if metadata.ContextSetDigest == "" {
		t.Fatalf("metadata mode should still produce digest")
	}
	if metadata.Records[0].SourceLocator != "redacted:metadata" {
		t.Fatalf("metadata source locator mismatch: %s", metadata.Records[0].SourceLocator)
	}
}

func TestLoadEnvelopeRejectsOversizedPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "context_envelope.json")
	oversized := strings.Repeat("a", int(MaxEnvelopeBytes)+1)
	if err := os.WriteFile(path, []byte(oversized), 0o600); err != nil {
		t.Fatalf("write oversized payload: %v", err)
	}
	if _, err := LoadEnvelope(path); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestParseEnvelopeInvalidPayload(t *testing.T) {
	if _, err := ParseEnvelope([]byte(`{"schema_id":"gait.context.envelope","schema_version":"1.0.0"`)); err == nil {
		t.Fatalf("expected parse failure for malformed payload")
	}
}

func TestNormalizeEvidenceModeValidation(t *testing.T) {
	if _, err := NormalizeEvidenceMode("unsupported"); err == nil {
		t.Fatalf("expected unsupported evidence mode to fail")
	}
	mode, err := NormalizeEvidenceMode("REQUIRED")
	if err != nil {
		t.Fatalf("normalize required mode: %v", err)
	}
	if mode != EvidenceModeRequired {
		t.Fatalf("expected normalized mode required, got %s", mode)
	}
}

func TestNormalizePrivacyModeValidation(t *testing.T) {
	mode, err := NormalizePrivacyMode("")
	if err != nil {
		t.Fatalf("default privacy mode: %v", err)
	}
	if mode != PrivacyModeMetadata {
		t.Fatalf("expected default metadata mode, got %s", mode)
	}
	if _, err := NormalizePrivacyMode("invalid"); err == nil {
		t.Fatalf("expected invalid privacy mode to fail")
	}
}

func TestNormalizeRefsValidation(t *testing.T) {
	_, err := NormalizeRefs(schemarunpack.Refs{
		ContextSetDigest: "not-a-digest",
	})
	if err == nil {
		t.Fatalf("expected invalid context_set_digest validation failure")
	}
}

func TestEnvelopeFromRefsRoundTrip(t *testing.T) {
	refs := schemarunpack.Refs{
		SchemaID:            "gait.runpack.refs",
		SchemaVersion:       "1.0.0",
		CreatedAt:           time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		ProducerVersion:     "0.0.0-test",
		RunID:               "run_ctx_roundtrip",
		ContextSetDigest:    strings.Repeat("a", 64),
		ContextEvidenceMode: EvidenceModeRequired,
		ContextRefCount:     1,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ctx_1",
				SourceType:    "doc_store",
				SourceLocator: "docs://a",
				QueryDigest:   strings.Repeat("b", 64),
				ContentDigest: strings.Repeat("c", 64),
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
				Immutability:  "immutable",
			},
		},
	}

	envelope, ok, err := EnvelopeFromRefs(refs)
	if err == nil || ok {
		t.Fatalf("expected digest mismatch error when refs context_set_digest does not match receipts")
	}
	if !strings.Contains(err.Error(), "context_set_digest mismatch") {
		t.Fatalf("unexpected envelope error: %v", err)
	}

	normalizedRefs := refs
	normalizedRefs.ContextSetDigest = ""
	envelope, ok, err = EnvelopeFromRefs(normalizedRefs)
	if err != nil {
		t.Fatalf("build envelope from refs: %v", err)
	}
	if !ok || envelope.ContextSetDigest == "" {
		t.Fatalf("expected context envelope to be generated from refs")
	}
}

func TestTransformEnvelopePrivacyModes(t *testing.T) {
	envelope, err := BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx_hash",
			SourceType:    "doc_store",
			SourceLocator: "docs://hash",
			QueryDigest:   strings.Repeat("1", 64),
			ContentDigest: strings.Repeat("2", 64),
			RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode: "reference",
			Immutability:  "immutable",
			RetrievalParams: map[string]any{
				"limit": 20,
			},
		},
	}, BuildEnvelopeOptions{ContextSetID: "ctx_transform"})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}

	hashesEnvelope, err := TransformEnvelopePrivacy(envelope, PrivacyModeHashes)
	if err != nil {
		t.Fatalf("hashes transform: %v", err)
	}
	if hashesEnvelope.Records[0].SourceLocator != "redacted:hashes" {
		t.Fatalf("expected hashes mode redacted source locator")
	}
	if hashesEnvelope.Records[0].RetrievalParams != nil {
		t.Fatalf("expected retrieval params to be removed")
	}

	rawEnvelope, err := TransformEnvelopePrivacy(envelope, PrivacyModeRaw)
	if err != nil {
		t.Fatalf("raw transform: %v", err)
	}
	rawBytes, err := json.Marshal(rawEnvelope)
	if err != nil {
		t.Fatalf("marshal raw envelope: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Fatalf("expected non-empty raw envelope")
	}
}

func TestApplyEnvelopeToRefsAndVerify(t *testing.T) {
	envelope, err := BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:               "ctx_apply",
			SourceType:          "doc_store",
			SourceLocator:       "docs://apply",
			QueryDigest:         strings.Repeat("1", 64),
			ContentDigest:       strings.Repeat("2", 64),
			RetrievedAt:         time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode:       "reference",
			Immutability:        "immutable",
			FreshnessSLASeconds: 30,
		},
	}, BuildEnvelopeOptions{
		ContextSetID: "ctx_apply_set",
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	if err := VerifyEnvelope(envelope); err != nil {
		t.Fatalf("verify envelope: %v", err)
	}

	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		RunID:           "run_apply",
		Receipts:        []schemarunpack.RefReceipt{},
		ContextRefCount: 0,
	}
	ApplyEnvelopeToRefs(&refs, envelope)
	if refs.ContextSetDigest != envelope.ContextSetDigest {
		t.Fatalf("expected refs context digest to match envelope")
	}
	if refs.ContextEvidenceMode != envelope.EvidenceMode {
		t.Fatalf("expected evidence mode to propagate")
	}
	if refs.ContextRefCount != 1 || len(refs.Receipts) != 1 {
		t.Fatalf("expected one propagated context receipt")
	}
}

func TestLoadEnvelopePathCases(t *testing.T) {
	if _, err := LoadEnvelope(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("expected missing envelope to fail")
	}

	envelope, err := BuildEnvelope([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx_file",
			SourceType:    "doc_store",
			SourceLocator: "docs://file",
			QueryDigest:   strings.Repeat("a", 64),
			ContentDigest: strings.Repeat("b", 64),
			RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode: "reference",
			Immutability:  "immutable",
		},
	}, BuildEnvelopeOptions{
		ContextSetID: "ctx_file_set",
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	path := filepath.Join(t.TempDir(), "envelope.json")
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write envelope file: %v", err)
	}
	loaded, err := LoadEnvelope(path)
	if err != nil {
		t.Fatalf("load envelope: %v", err)
	}
	if loaded.ContextSetDigest != envelope.ContextSetDigest {
		t.Fatalf("loaded digest mismatch")
	}
}

func TestClassifyRefsDriftNoneAndSemantic(t *testing.T) {
	base := schemarunpack.Refs{
		SchemaID:            "gait.runpack.refs",
		SchemaVersion:       "1.0.0",
		RunID:               "run_drift",
		ContextEvidenceMode: EvidenceModeRequired,
		ContextSetDigest:    strings.Repeat("a", 64),
		ContextRefCount:     1,
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ctx_1",
				SourceType:    "doc_store",
				SourceLocator: "docs://a",
				QueryDigest:   strings.Repeat("b", 64),
				ContentDigest: strings.Repeat("c", 64),
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
			},
		},
	}
	classification, changed, runtimeOnly, err := ClassifyRefsDrift(base, base)
	if err != nil {
		t.Fatalf("classify none: %v", err)
	}
	if classification != "none" || changed || runtimeOnly {
		t.Fatalf("expected none classification")
	}

	semantic := base
	semantic.Receipts = append([]schemarunpack.RefReceipt(nil), base.Receipts...)
	semantic.Receipts[0].ContentDigest = strings.Repeat("d", 64)
	classification, changed, runtimeOnly, err = ClassifyRefsDrift(base, semantic)
	if err != nil {
		t.Fatalf("classify semantic: %v", err)
	}
	if classification != "semantic" || !changed || runtimeOnly {
		t.Fatalf("expected semantic classification")
	}
}

func TestNormalizeEnvelopeValidationErrors(t *testing.T) {
	base := schemacontext.Envelope{
		SchemaID:         EnvelopeSchemaID,
		SchemaVersion:    EnvelopeSchemaVersion,
		CreatedAt:        time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
		ProducerVersion:  "0.0.0-test",
		ContextSetID:     "ctx_err",
		EvidenceMode:     EvidenceModeRequired,
		ContextSetDigest: "",
		Records: []schemacontext.ReferenceRecord{
			{
				RefID:         "ctx_1",
				SourceType:    "doc_store",
				SourceLocator: "docs://err",
				QueryDigest:   strings.Repeat("1", 64),
				ContentDigest: strings.Repeat("2", 64),
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
				Immutability:  "immutable",
			},
		},
	}

	tests := []struct {
		name     string
		mutate   func(schemacontext.Envelope) schemacontext.Envelope
		contains string
	}{
		{
			name: "schema_id",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.SchemaID = "other"
				return in
			},
			contains: "unsupported context envelope schema_id",
		},
		{
			name: "schema_version",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.SchemaVersion = "9.9.9"
				return in
			},
			contains: "unsupported context envelope schema_version",
		},
		{
			name: "missing_set_id",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.ContextSetID = " "
				return in
			},
			contains: "context_set_id is required",
		},
		{
			name: "invalid_evidence_mode",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.EvidenceMode = "bad"
				return in
			},
			contains: "context evidence mode",
		},
		{
			name: "invalid_digest_pattern",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.ContextSetDigest = "bad"
				return in
			},
			contains: "context_set_digest must be sha256 hex",
		},
		{
			name: "digest_mismatch",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.ContextSetDigest = strings.Repeat("f", 64)
				return in
			},
			contains: "context_set_digest mismatch",
		},
		{
			name: "record_immutability",
			mutate: func(in schemacontext.Envelope) schemacontext.Envelope {
				in.Records[0].Immutability = "invalid"
				return in
			},
			contains: "immutability",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NormalizeEnvelope(testCase.mutate(base))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(testCase.contains)) {
				t.Fatalf("expected %q error, got %v", testCase.contains, err)
			}
		})
	}
}

func TestContextSetDigestMarshalError(t *testing.T) {
	_, err := ContextSetDigest([]schemacontext.ReferenceRecord{
		{
			RefID:         "ctx_bad",
			SourceType:    "doc_store",
			SourceLocator: "docs://bad",
			QueryDigest:   strings.Repeat("1", 64),
			ContentDigest: strings.Repeat("2", 64),
			RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			RedactionMode: "reference",
			Immutability:  "immutable",
			RetrievalParams: map[string]any{
				"bad": func() {},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected marshal error for unsupported retrieval params")
	}
}

func TestClassifyRefsDriftMarshalError(t *testing.T) {
	left := schemarunpack.Refs{
		SchemaID:      "gait.runpack.refs",
		SchemaVersion: "1.0.0",
		RunID:         "run_bad",
		Receipts: []schemarunpack.RefReceipt{
			{
				RefID:         "ctx_bad",
				SourceType:    "doc_store",
				SourceLocator: "docs://bad",
				QueryDigest:   strings.Repeat("1", 64),
				ContentDigest: strings.Repeat("2", 64),
				RetrievedAt:   time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
				RedactionMode: "reference",
				RetrievalParams: map[string]any{
					"bad": func() {},
				},
			},
		},
	}
	if _, _, _, err := ClassifyRefsDrift(left, left); err == nil {
		t.Fatalf("expected classify drift marshal error")
	}
}
