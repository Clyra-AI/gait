package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	delegationAuditSchemaID = "gait.gate.delegation_audit_record"
	delegationAuditSchemaV1 = "1.0.0"
)

type BuildDelegationAuditOptions struct {
	CreatedAt          time.Time
	ProducerVersion    string
	TraceID            string
	ToolName           string
	IntentDigest       string
	PolicyDigest       string
	DelegationRequired bool
	DelegationRef      string
	Entries            []schemagate.DelegationAuditEntry
}

func BuildDelegationAuditRecord(opts BuildDelegationAuditOptions) schemagate.DelegationAuditRecord {
	createdAt := opts.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	entries := append([]schemagate.DelegationAuditEntry{}, opts.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].DelegatorIdentity != entries[j].DelegatorIdentity {
			return entries[i].DelegatorIdentity < entries[j].DelegatorIdentity
		}
		if entries[i].DelegateIdentity != entries[j].DelegateIdentity {
			return entries[i].DelegateIdentity < entries[j].DelegateIdentity
		}
		return entries[i].TokenID < entries[j].TokenID
	})
	validDelegations := 0
	for _, entry := range entries {
		if entry.Valid {
			validDelegations++
		}
	}
	return schemagate.DelegationAuditRecord{
		SchemaID:           delegationAuditSchemaID,
		SchemaVersion:      delegationAuditSchemaV1,
		CreatedAt:          createdAt,
		ProducerVersion:    producerVersion,
		TraceID:            strings.TrimSpace(opts.TraceID),
		ToolName:           strings.TrimSpace(opts.ToolName),
		IntentDigest:       strings.ToLower(strings.TrimSpace(opts.IntentDigest)),
		PolicyDigest:       strings.ToLower(strings.TrimSpace(opts.PolicyDigest)),
		DelegationRequired: opts.DelegationRequired,
		ValidDelegations:   validDelegations,
		Delegated:          validDelegations > 0,
		DelegationRef:      strings.TrimSpace(opts.DelegationRef),
		Entries:            entries,
	}
}

func WriteDelegationAuditRecord(path string, record schemagate.DelegationAuditRecord) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create delegation audit directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delegation audit record: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write delegation audit record: %w", err)
	}
	return nil
}
