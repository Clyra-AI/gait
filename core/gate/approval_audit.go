package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	approvalAuditSchemaID = "gait.gate.approval_audit_record"
	approvalAuditSchemaV1 = "1.0.0"
)

type BuildApprovalAuditOptions struct {
	CreatedAt         time.Time
	ProducerVersion   string
	TraceID           string
	ToolName          string
	IntentDigest      string
	PolicyDigest      string
	RequiredApprovals int
	Entries           []schemagate.ApprovalAuditEntry
}

func BuildApprovalAuditRecord(opts BuildApprovalAuditOptions) schemagate.ApprovalAuditRecord {
	createdAt := opts.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	requiredApprovals := opts.RequiredApprovals
	if requiredApprovals < 0 {
		requiredApprovals = 0
	}

	entries := append([]schemagate.ApprovalAuditEntry(nil), opts.Entries...)
	for index := range entries {
		entries[index].TokenID = strings.TrimSpace(entries[index].TokenID)
		entries[index].ApproverIdentity = strings.TrimSpace(entries[index].ApproverIdentity)
		entries[index].ReasonCode = strings.TrimSpace(entries[index].ReasonCode)
		entries[index].Scope = uniqueSorted(entries[index].Scope)
		entries[index].ErrorCode = strings.TrimSpace(entries[index].ErrorCode)
		entries[index].ExpiresAt = entries[index].ExpiresAt.UTC()
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TokenID != entries[j].TokenID {
			return entries[i].TokenID < entries[j].TokenID
		}
		if entries[i].ApproverIdentity != entries[j].ApproverIdentity {
			return entries[i].ApproverIdentity < entries[j].ApproverIdentity
		}
		if entries[i].ReasonCode != entries[j].ReasonCode {
			return entries[i].ReasonCode < entries[j].ReasonCode
		}
		if entries[i].ExpiresAt != entries[j].ExpiresAt {
			return entries[i].ExpiresAt.Before(entries[j].ExpiresAt)
		}
		return strings.Join(entries[i].Scope, ",") < strings.Join(entries[j].Scope, ",")
	})

	validApprovals := 0
	approvers := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Valid {
			validApprovals++
			if entry.ApproverIdentity != "" {
				approvers = append(approvers, entry.ApproverIdentity)
			}
		}
	}

	return schemagate.ApprovalAuditRecord{
		SchemaID:          approvalAuditSchemaID,
		SchemaVersion:     approvalAuditSchemaV1,
		CreatedAt:         createdAt,
		ProducerVersion:   producerVersion,
		TraceID:           strings.TrimSpace(opts.TraceID),
		ToolName:          strings.TrimSpace(opts.ToolName),
		IntentDigest:      strings.ToLower(strings.TrimSpace(opts.IntentDigest)),
		PolicyDigest:      strings.ToLower(strings.TrimSpace(opts.PolicyDigest)),
		RequiredApprovals: requiredApprovals,
		ValidApprovals:    validApprovals,
		Approved:          validApprovals >= requiredApprovals,
		Approvers:         uniqueSorted(approvers),
		Relationship:      buildApprovalAuditRelationship(opts.TraceID, opts.ToolName, opts.PolicyDigest, approvers),
		Entries:           entries,
	}
}

func WriteApprovalAuditRecord(path string, record schemagate.ApprovalAuditRecord) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create approval audit directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval audit record: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write approval audit record: %w", err)
	}
	return nil
}
