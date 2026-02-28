package runpack

import (
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
)

type Manifest struct {
	SchemaID        string         `json:"schema_id"`
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       time.Time      `json:"created_at"`
	ProducerVersion string         `json:"producer_version"`
	RunID           string         `json:"run_id"`
	CaptureMode     string         `json:"capture_mode"`
	Files           []ManifestFile `json:"files"`
	ManifestDigest  string         `json:"manifest_digest"`
	Signatures      []Signature    `json:"signatures,omitempty"`
}

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Signature struct {
	Alg          string `json:"alg"`
	KeyID        string `json:"key_id"`
	Sig          string `json:"sig"`
	SignedDigest string `json:"signed_digest,omitempty"`
}

type Run struct {
	SchemaID        string        `json:"schema_id"`
	SchemaVersion   string        `json:"schema_version"`
	CreatedAt       time.Time     `json:"created_at"`
	ProducerVersion string        `json:"producer_version"`
	RunID           string        `json:"run_id"`
	Env             RunEnv        `json:"env"`
	Timeline        []TimelineEvt `json:"timeline"`
}

type RunEnv struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Runtime string `json:"runtime"`
}

type TimelineEvt struct {
	Event        string                             `json:"event"`
	TS           time.Time                          `json:"ts"`
	Ref          string                             `json:"ref,omitempty"`
	Relationship *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
}

type IntentRecord struct {
	SchemaID        string         `json:"schema_id"`
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       time.Time      `json:"created_at"`
	ProducerVersion string         `json:"producer_version"`
	RunID           string         `json:"run_id"`
	IntentID        string         `json:"intent_id"`
	ToolName        string         `json:"tool_name"`
	ArgsDigest      string         `json:"args_digest"`
	Args            map[string]any `json:"args,omitempty"`
}

type ResultRecord struct {
	SchemaID        string         `json:"schema_id"`
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       time.Time      `json:"created_at"`
	ProducerVersion string         `json:"producer_version"`
	RunID           string         `json:"run_id"`
	IntentID        string         `json:"intent_id"`
	Status          string         `json:"status"`
	ResultDigest    string         `json:"result_digest"`
	Result          map[string]any `json:"result,omitempty"`
}

type Refs struct {
	SchemaID            string       `json:"schema_id"`
	SchemaVersion       string       `json:"schema_version"`
	CreatedAt           time.Time    `json:"created_at"`
	ProducerVersion     string       `json:"producer_version"`
	RunID               string       `json:"run_id"`
	ContextSetDigest    string       `json:"context_set_digest,omitempty"`
	ContextEvidenceMode string       `json:"context_evidence_mode,omitempty"`
	ContextRefCount     int          `json:"context_ref_count,omitempty"`
	Receipts            []RefReceipt `json:"receipts"`
}

type RefReceipt struct {
	RefID               string         `json:"ref_id"`
	SourceType          string         `json:"source_type"`
	SourceLocator       string         `json:"source_locator"`
	QueryDigest         string         `json:"query_digest"`
	ContentDigest       string         `json:"content_digest"`
	RetrievedAt         time.Time      `json:"retrieved_at"`
	RedactionMode       string         `json:"redaction_mode"`
	Immutability        string         `json:"immutability,omitempty"`
	FreshnessSLASeconds int64          `json:"freshness_sla_seconds,omitempty"`
	SensitivityLabel    string         `json:"sensitivity_label,omitempty"`
	RetrievalParams     map[string]any `json:"retrieval_params,omitempty"`
}

type SessionJournal struct {
	SchemaID        string              `json:"schema_id"`
	SchemaVersion   string              `json:"schema_version"`
	CreatedAt       time.Time           `json:"created_at"`
	ProducerVersion string              `json:"producer_version"`
	SessionID       string              `json:"session_id"`
	RunID           string              `json:"run_id"`
	StartedAt       time.Time           `json:"started_at"`
	Events          []SessionEvent      `json:"events"`
	Checkpoints     []SessionCheckpoint `json:"checkpoints,omitempty"`
}

type SessionEvent struct {
	SchemaID               string                             `json:"schema_id"`
	SchemaVersion          string                             `json:"schema_version"`
	CreatedAt              time.Time                          `json:"created_at"`
	ProducerVersion        string                             `json:"producer_version"`
	SessionID              string                             `json:"session_id"`
	RunID                  string                             `json:"run_id"`
	Sequence               int64                              `json:"sequence"`
	IntentID               string                             `json:"intent_id,omitempty"`
	ToolName               string                             `json:"tool_name,omitempty"`
	IntentDigest           string                             `json:"intent_digest,omitempty"`
	PolicyDigest           string                             `json:"policy_digest,omitempty"`
	PolicyID               string                             `json:"policy_id,omitempty"`
	PolicyVersion          string                             `json:"policy_version,omitempty"`
	MatchedRuleIDs         []string                           `json:"matched_rule_ids,omitempty"`
	TraceID                string                             `json:"trace_id,omitempty"`
	TracePath              string                             `json:"trace_path,omitempty"`
	Verdict                string                             `json:"verdict,omitempty"`
	ReasonCodes            []string                           `json:"reason_codes,omitempty"`
	Violations             []string                           `json:"violations,omitempty"`
	Relationship           *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	SafetyInvariantVersion string                             `json:"safety_invariant_version,omitempty"`
	SafetyInvariantHash    string                             `json:"safety_invariant_hash,omitempty"`
}

type SessionCheckpoint struct {
	SchemaID               string                             `json:"schema_id"`
	SchemaVersion          string                             `json:"schema_version"`
	CreatedAt              time.Time                          `json:"created_at"`
	ProducerVersion        string                             `json:"producer_version"`
	SessionID              string                             `json:"session_id"`
	RunID                  string                             `json:"run_id"`
	CheckpointIndex        int                                `json:"checkpoint_index"`
	SequenceStart          int64                              `json:"sequence_start"`
	SequenceEnd            int64                              `json:"sequence_end"`
	RunpackPath            string                             `json:"runpack_path"`
	ManifestDigest         string                             `json:"manifest_digest"`
	PrevCheckpointDigest   string                             `json:"prev_checkpoint_digest,omitempty"`
	CheckpointDigest       string                             `json:"checkpoint_digest"`
	Relationship           *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	SafetyInvariantVersion string                             `json:"safety_invariant_version,omitempty"`
	SafetyInvariantHash    string                             `json:"safety_invariant_hash,omitempty"`
}

type SessionChain struct {
	SchemaID        string              `json:"schema_id"`
	SchemaVersion   string              `json:"schema_version"`
	CreatedAt       time.Time           `json:"created_at"`
	ProducerVersion string              `json:"producer_version"`
	SessionID       string              `json:"session_id"`
	RunID           string              `json:"run_id"`
	Checkpoints     []SessionCheckpoint `json:"checkpoints"`
}
