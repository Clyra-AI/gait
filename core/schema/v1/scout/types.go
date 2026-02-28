package scout

import (
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
)

type InventorySnapshot struct {
	SchemaID        string          `json:"schema_id"`
	SchemaVersion   string          `json:"schema_version"`
	CreatedAt       time.Time       `json:"created_at"`
	ProducerVersion string          `json:"producer_version"`
	SnapshotID      string          `json:"snapshot_id"`
	Workspace       string          `json:"workspace,omitempty"`
	Items           []InventoryItem `json:"items"`
}

type InventoryItem struct {
	ID           string                             `json:"id"`
	Kind         string                             `json:"kind"`
	Name         string                             `json:"name"`
	Locator      string                             `json:"locator"`
	RiskLevel    string                             `json:"risk_level,omitempty"`
	Tags         []string                           `json:"tags,omitempty"`
	LastSeenRun  string                             `json:"last_seen_run,omitempty"`
	Relationship *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
}

type AdoptionEvent struct {
	SchemaID        string             `json:"schema_id"`
	SchemaVersion   string             `json:"schema_version"`
	CreatedAt       time.Time          `json:"created_at"`
	ProducerVersion string             `json:"producer_version"`
	Command         string             `json:"command"`
	WorkflowID      string             `json:"workflow_id,omitempty"`
	Success         bool               `json:"success"`
	ExitCode        int                `json:"exit_code"`
	ElapsedMS       int64              `json:"elapsed_ms"`
	Milestones      []string           `json:"milestones,omitempty"`
	Environment     AdoptionEnvContext `json:"environment"`
}

type AdoptionEnvContext struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type OperationalEvent struct {
	SchemaID        string             `json:"schema_id"`
	SchemaVersion   string             `json:"schema_version"`
	CreatedAt       time.Time          `json:"created_at"`
	ProducerVersion string             `json:"producer_version"`
	CorrelationID   string             `json:"correlation_id"`
	Command         string             `json:"command"`
	Phase           string             `json:"phase"`
	ExitCode        int                `json:"exit_code"`
	ErrorCategory   string             `json:"error_category"`
	Retryable       bool               `json:"retryable"`
	ElapsedMS       int64              `json:"elapsed_ms"`
	Environment     AdoptionEnvContext `json:"environment"`
}

type RunFingerprint struct {
	SchemaID            string    `json:"schema_id"`
	SchemaVersion       string    `json:"schema_version"`
	CreatedAt           time.Time `json:"created_at"`
	ProducerVersion     string    `json:"producer_version"`
	RunID               string    `json:"run_id"`
	Fingerprint         string    `json:"fingerprint"`
	ActionSequence      []string  `json:"action_sequence"`
	ToolClasses         []string  `json:"tool_classes"`
	TargetSystems       []string  `json:"target_systems"`
	ReasonCodeVector    []string  `json:"reason_code_vector"`
	RefReceiptDigests   []string  `json:"ref_receipt_digests"`
	SourceRunpack       string    `json:"source_runpack,omitempty"`
	TraceCount          int       `json:"trace_count,omitempty"`
	RegressEvidenceSeen bool      `json:"regress_evidence_seen,omitempty"`
}

type SignalFixSuggestion struct {
	Kind        string `json:"kind"`
	Summary     string `json:"summary"`
	LikelyScope string `json:"likely_scope"`
}

type SignalIssue struct {
	Rank             int                   `json:"rank"`
	FamilyID         string                `json:"family_id"`
	Fingerprint      string                `json:"fingerprint"`
	Count            int                   `json:"count"`
	CanonicalRunID   string                `json:"canonical_run_id"`
	TopFailureReason string                `json:"top_failure_reason,omitempty"`
	Drivers          []string              `json:"drivers,omitempty"`
	SeverityScore    int                   `json:"severity_score"`
	SeverityLevel    string                `json:"severity_level"`
	Suggestions      []SignalFixSuggestion `json:"suggestions,omitempty"`
}

type SignalFamily struct {
	FamilyID         string                `json:"family_id"`
	Fingerprint      string                `json:"fingerprint"`
	Count            int                   `json:"count"`
	RunIDs           []string              `json:"run_ids"`
	CanonicalRunID   string                `json:"canonical_run_id"`
	TopFailureReason string                `json:"top_failure_reason,omitempty"`
	Drivers          []string              `json:"drivers,omitempty"`
	SeverityScore    int                   `json:"severity_score"`
	SeverityLevel    string                `json:"severity_level"`
	ArtifactPointers []string              `json:"artifact_pointers,omitempty"`
	Suggestions      []SignalFixSuggestion `json:"suggestions,omitempty"`
}

type SignalReport struct {
	SchemaID        string           `json:"schema_id"`
	SchemaVersion   string           `json:"schema_version"`
	CreatedAt       time.Time        `json:"created_at"`
	ProducerVersion string           `json:"producer_version"`
	RunCount        int              `json:"run_count"`
	FamilyCount     int              `json:"family_count"`
	Fingerprints    []RunFingerprint `json:"fingerprints"`
	Families        []SignalFamily   `json:"families"`
	TopIssues       []SignalIssue    `json:"top_issues"`
}
