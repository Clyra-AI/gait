package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/fsx"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

const (
	brokerCredentialSchemaID = "gait.gate.broker_credential_record"
	brokerCredentialSchemaV1 = "1.0.0"
)

type BuildBrokerCredentialRecordOptions struct {
	CreatedAt       time.Time
	ProducerVersion string
	TraceID         string
	ToolName        string
	Identity        string
	Broker          string
	Reference       string
	Scope           []string
	CredentialRef   string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	TTLSeconds      int64
}

func BuildBrokerCredentialRecord(opts BuildBrokerCredentialRecordOptions) schemagate.BrokerCredentialRecord {
	createdAt := opts.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	return schemagate.BrokerCredentialRecord{
		SchemaID:        brokerCredentialSchemaID,
		SchemaVersion:   brokerCredentialSchemaV1,
		CreatedAt:       createdAt,
		ProducerVersion: producerVersion,
		TraceID:         strings.TrimSpace(opts.TraceID),
		ToolName:        strings.TrimSpace(opts.ToolName),
		Identity:        strings.TrimSpace(opts.Identity),
		Broker:          strings.TrimSpace(opts.Broker),
		Reference:       strings.TrimSpace(opts.Reference),
		Scope:           uniqueSorted(opts.Scope),
		CredentialRef:   strings.TrimSpace(opts.CredentialRef),
		IssuedAt:        opts.IssuedAt.UTC(),
		ExpiresAt:       opts.ExpiresAt.UTC(),
		TTLSeconds:      opts.TTLSeconds,
	}
}

func WriteBrokerCredentialRecord(path string, record schemagate.BrokerCredentialRecord) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create credential evidence directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credential evidence: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write credential evidence: %w", err)
	}
	return nil
}
