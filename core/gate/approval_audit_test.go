package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestBuildApprovalAuditRecordDeterministic(t *testing.T) {
	record := BuildApprovalAuditRecord(BuildApprovalAuditOptions{
		CreatedAt:         time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion:   "0.0.0-dev",
		TraceID:           "trace_1",
		ToolName:          "tool.write",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RequiredApprovals: 2,
		Entries: []schemagate.ApprovalAuditEntry{
			{
				TokenID:          "token_b",
				ApproverIdentity: "bob",
				ReasonCode:       "ticket-2",
				Scope:            []string{"tool:tool.write"},
				ExpiresAt:        time.Date(2026, time.February, 5, 1, 0, 0, 0, time.UTC),
				Valid:            true,
			},
			{
				TokenID:          "token_a",
				ApproverIdentity: "alice",
				ReasonCode:       "ticket-1",
				Scope:            []string{"tool:tool.write"},
				ExpiresAt:        time.Date(2026, time.February, 5, 1, 0, 0, 0, time.UTC),
				Valid:            true,
			},
		},
	})

	if !record.Approved || record.ValidApprovals != 2 {
		t.Fatalf("unexpected approval summary: %#v", record)
	}
	if !reflect.DeepEqual(record.Approvers, []string{"alice", "bob"}) {
		t.Fatalf("unexpected approvers: %#v", record.Approvers)
	}
	if len(record.Entries) != 2 || record.Entries[0].TokenID != "token_a" {
		t.Fatalf("expected sorted entries, got %#v", record.Entries)
	}
}

func TestWriteApprovalAuditRecord(t *testing.T) {
	record := BuildApprovalAuditRecord(BuildApprovalAuditOptions{
		CreatedAt:         time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion:   "0.0.0-dev",
		TraceID:           "trace_1",
		ToolName:          "tool.write",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RequiredApprovals: 1,
		Entries: []schemagate.ApprovalAuditEntry{
			{
				TokenID:          "token_a",
				ApproverIdentity: "alice",
				ReasonCode:       "ticket-1",
				Scope:            []string{"tool:tool.write"},
				ExpiresAt:        time.Date(2026, time.February, 5, 1, 0, 0, 0, time.UTC),
				Valid:            true,
			},
		},
	})
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := WriteApprovalAuditRecord(path, record); err != nil {
		t.Fatalf("write audit record: %v", err)
	}

	content, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatalf("read audit record: %v", err)
	}
	var parsed schemagate.ApprovalAuditRecord
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse audit record: %v", err)
	}
	if parsed.TraceID != "trace_1" || parsed.SchemaID != approvalAuditSchemaID {
		t.Fatalf("unexpected parsed audit record: %#v", parsed)
	}
}
