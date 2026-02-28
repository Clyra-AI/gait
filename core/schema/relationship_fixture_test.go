package schema_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	validate "github.com/Clyra-AI/proof/schema"
)

func TestRelationshipFixturesValidateAgainstSchemas(t *testing.T) {
	repoRoot := resolveRepoRoot(t)
	testCases := []struct {
		name      string
		schema    string
		fixture   string
		shouldErr bool
	}{
		{
			name:    "intent_without_relationship",
			schema:  "schemas/v1/gate/intent_request.schema.json",
			fixture: "core/schema/testdata/gate_intent_request_valid.json",
		},
		{
			name:    "intent_with_relationship",
			schema:  "schemas/v1/gate/intent_request.schema.json",
			fixture: "core/schema/testdata/gate_intent_request_relationship_valid.json",
		},
		{
			name:    "trace_without_relationship",
			schema:  "schemas/v1/gate/trace_record.schema.json",
			fixture: "core/schema/testdata/gate_trace_record_valid.json",
		},
		{
			name:    "trace_with_relationship",
			schema:  "schemas/v1/gate/trace_record.schema.json",
			fixture: "core/schema/testdata/gate_trace_record_relationship_valid.json",
		},
		{
			name:      "trace_invalid_relationship",
			schema:    "schemas/v1/gate/trace_record.schema.json",
			fixture:   "core/schema/testdata/gate_trace_record_relationship_invalid.json",
			shouldErr: true,
		},
		{
			name:    "approval_audit_without_relationship",
			schema:  "schemas/v1/gate/approval_audit_record.schema.json",
			fixture: "core/schema/testdata/gate_approval_audit_record_valid.json",
		},
		{
			name:    "approval_audit_with_relationship",
			schema:  "schemas/v1/gate/approval_audit_record.schema.json",
			fixture: "core/schema/testdata/gate_approval_audit_record_relationship_valid.json",
		},
		{
			name:    "delegation_audit_without_relationship",
			schema:  "schemas/v1/gate/delegation_audit_record.schema.json",
			fixture: "core/schema/testdata/gate_delegation_audit_record_valid.json",
		},
		{
			name:    "delegation_audit_with_relationship",
			schema:  "schemas/v1/gate/delegation_audit_record.schema.json",
			fixture: "core/schema/testdata/gate_delegation_audit_record_relationship_valid.json",
		},
		{
			name:    "inventory_without_relationship",
			schema:  "schemas/v1/scout/inventory_snapshot.schema.json",
			fixture: "core/schema/testdata/scout_inventory_snapshot_valid.json",
		},
		{
			name:    "inventory_with_relationship",
			schema:  "schemas/v1/scout/inventory_snapshot.schema.json",
			fixture: "core/schema/testdata/scout_inventory_snapshot_relationship_valid.json",
		},
		{
			name:    "run_without_relationship",
			schema:  "schemas/v1/runpack/run.schema.json",
			fixture: "core/schema/testdata/run_valid.json",
		},
		{
			name:    "run_with_relationship",
			schema:  "schemas/v1/runpack/run.schema.json",
			fixture: "core/schema/testdata/run_relationship_valid.json",
		},
		{
			name:    "session_journal_without_relationship",
			schema:  "schemas/v1/runpack/session_journal.schema.json",
			fixture: "core/schema/testdata/session_journal_valid.json",
		},
		{
			name:    "session_journal_with_relationship",
			schema:  "schemas/v1/runpack/session_journal.schema.json",
			fixture: "core/schema/testdata/session_journal_relationship_valid.json",
		},
		{
			name:    "session_checkpoint_without_relationship",
			schema:  "schemas/v1/runpack/session_checkpoint.schema.json",
			fixture: "core/schema/testdata/session_checkpoint_valid.json",
		},
		{
			name:    "session_checkpoint_with_relationship",
			schema:  "schemas/v1/runpack/session_checkpoint.schema.json",
			fixture: "core/schema/testdata/session_checkpoint_relationship_valid.json",
		},
		{
			name:    "session_chain_without_relationship",
			schema:  "schemas/v1/runpack/session_chain.schema.json",
			fixture: "core/schema/testdata/session_chain_valid.json",
		},
		{
			name:    "session_chain_with_relationship",
			schema:  "schemas/v1/runpack/session_chain.schema.json",
			fixture: "core/schema/testdata/session_chain_relationship_valid.json",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			schemaPath := filepath.Join(repoRoot, testCase.schema)
			fixturePath := filepath.Join(repoRoot, testCase.fixture)
			// #nosec G304 -- paths are repository-local constants in test table.
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			err = validate.ValidateJSON(schemaPath, data)
			if testCase.shouldErr {
				if err == nil {
					t.Fatalf("expected schema validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validate fixture: %v", err)
			}
		})
	}
}

func TestTraceRelationshipFixturePreservesDeterministicFields(t *testing.T) {
	repoRoot := resolveRepoRoot(t)
	baseFixture := filepath.Join(repoRoot, "core/schema/testdata/gate_trace_record_valid.json")
	relationshipFixture := filepath.Join(repoRoot, "core/schema/testdata/gate_trace_record_relationship_valid.json")
	base := map[string]any{}
	withRelationship := map[string]any{}

	// #nosec G304 -- paths are fixed repository-local fixtures.
	baseRaw, err := os.ReadFile(baseFixture)
	if err != nil {
		t.Fatalf("read base fixture: %v", err)
	}
	// #nosec G304 -- paths are fixed repository-local fixtures.
	relationshipRaw, err := os.ReadFile(relationshipFixture)
	if err != nil {
		t.Fatalf("read relationship fixture: %v", err)
	}
	if err := json.Unmarshal(baseRaw, &base); err != nil {
		t.Fatalf("parse base fixture: %v", err)
	}
	if err := json.Unmarshal(relationshipRaw, &withRelationship); err != nil {
		t.Fatalf("parse relationship fixture: %v", err)
	}

	keys := []string{"trace_id", "intent_digest", "policy_digest", "verdict"}
	for _, key := range keys {
		if base[key] != withRelationship[key] {
			t.Fatalf("expected %s to remain stable, base=%v relationship=%v", key, base[key], withRelationship[key])
		}
	}
}

func TestBaseFixturesHaveStableByteDigests(t *testing.T) {
	repoRoot := resolveRepoRoot(t)
	testCases := []struct {
		fixture        string
		expectedSHA256 string
	}{
		{
			fixture:        "core/schema/testdata/gate_trace_record_valid.json",
			expectedSHA256: "dbf1ff6fe53e2df77c3a6846a3e76e63c97c8fee1f5706ea3fb908e34c423a3b",
		},
		{
			fixture:        "core/schema/testdata/gate_intent_request_valid.json",
			expectedSHA256: "7fc28ca4e27eb77b5a3f5e0d21ea8e97ec626388355f7139612ad2723585f7b9",
		},
		{
			fixture:        "core/schema/testdata/run_valid.json",
			expectedSHA256: "de76b8f9595d3a7df632868d0e2743202d1d102dab9eb43da5855d4196150b6d",
		},
		{
			fixture:        "core/schema/testdata/session_journal_valid.json",
			expectedSHA256: "002b158b8b8cc822555675998ae8eb97890a95b4f372dcd466c81a71c3401b43",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.fixture, func(t *testing.T) {
			fixturePath := filepath.Join(repoRoot, testCase.fixture)
			// #nosec G304 -- fixture path is repository-local in test table.
			content, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			digest := sha256.Sum256(normalizeFixtureLineEndings(content))
			if got := hex.EncodeToString(digest[:]); got != testCase.expectedSHA256 {
				t.Fatalf("fixture digest changed: got %s want %s", got, testCase.expectedSHA256)
			}
		})
	}
}

func TestReadersTolerateUnknownAdditiveFields(t *testing.T) {
	repoRoot := resolveRepoRoot(t)
	testCases := []struct {
		name    string
		fixture string
		decode  func([]byte) error
	}{
		{
			name:    "trace_reader",
			fixture: "core/schema/testdata/gate_trace_record_relationship_valid.json",
			decode: func(data []byte) error {
				var record schemagate.TraceRecord
				return json.Unmarshal(data, &record)
			},
		},
		{
			name:    "intent_reader",
			fixture: "core/schema/testdata/gate_intent_request_relationship_valid.json",
			decode: func(data []byte) error {
				var record schemagate.IntentRequest
				return json.Unmarshal(data, &record)
			},
		},
		{
			name:    "run_reader",
			fixture: "core/schema/testdata/run_relationship_valid.json",
			decode: func(data []byte) error {
				var record schemarunpack.Run
				return json.Unmarshal(data, &record)
			},
		},
		{
			name:    "session_journal_reader",
			fixture: "core/schema/testdata/session_journal_relationship_valid.json",
			decode: func(data []byte) error {
				var record schemarunpack.SessionJournal
				return json.Unmarshal(data, &record)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixturePath := filepath.Join(repoRoot, testCase.fixture)
			// #nosec G304 -- fixture path is repository-local in test table.
			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			payload["unknown_top_level"] = "additive"
			if relationship, ok := payload["relationship"].(map[string]any); ok {
				relationship["unknown_relationship_field"] = true
				payload["relationship"] = relationship
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			if err := testCase.decode(encoded); err != nil {
				t.Fatalf("decode with additive fields: %v", err)
			}
		})
	}
}

func resolveRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func normalizeFixtureLineEndings(content []byte) []byte {
	// Keep digest checks stable across Git checkout EOL conversion on Windows.
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}
