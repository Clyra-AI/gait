package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

func TestBuildAndWriteBrokerCredentialRecord(t *testing.T) {
	record := BuildBrokerCredentialRecord(BuildBrokerCredentialRecordOptions{
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		TraceID:         "trace_1",
		ToolName:        "tool.write",
		Identity:        "alice",
		Broker:          "stub",
		Reference:       "egress",
		Scope:           []string{"export"},
		CredentialRef:   "stub:abc",
		IssuedAt:        time.Date(2026, time.February, 5, 0, 1, 0, 0, time.UTC),
		ExpiresAt:       time.Date(2026, time.February, 5, 0, 6, 0, 0, time.UTC),
		TTLSeconds:      300,
	})
	if record.SchemaID != brokerCredentialSchemaID || record.CredentialRef != "stub:abc" {
		t.Fatalf("unexpected broker credential record: %#v", record)
	}
	if record.TTLSeconds != 300 || record.IssuedAt.IsZero() || record.ExpiresAt.IsZero() {
		t.Fatalf("expected ttl metadata in broker credential record: %#v", record)
	}

	path := filepath.Join(t.TempDir(), "credential_evidence.json")
	if err := WriteBrokerCredentialRecord(path, record); err != nil {
		t.Fatalf("write broker credential record: %v", err)
	}

	content, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatalf("read broker credential record: %v", err)
	}
	var parsed schemagate.BrokerCredentialRecord
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse broker credential record: %v", err)
	}
	if parsed.Broker != "stub" || parsed.TraceID != "trace_1" {
		t.Fatalf("unexpected parsed broker record: %#v", parsed)
	}
}
