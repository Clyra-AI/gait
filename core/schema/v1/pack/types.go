package pack

import "time"

type Manifest struct {
	SchemaID        string      `json:"schema_id"`
	SchemaVersion   string      `json:"schema_version"`
	CreatedAt       time.Time   `json:"created_at"`
	ProducerVersion string      `json:"producer_version"`
	PackID          string      `json:"pack_id"`
	PackType        string      `json:"pack_type"`
	SourceRef       string      `json:"source_ref"`
	Contents        []PackEntry `json:"contents"`
	Signatures      []Signature `json:"signatures,omitempty"`
}

type PackEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Type   string `json:"type"`
}

type Signature struct {
	Alg          string `json:"alg"`
	KeyID        string `json:"key_id"`
	Sig          string `json:"sig"`
	SignedDigest string `json:"signed_digest,omitempty"`
}

type RunPayload struct {
	SchemaID       string    `json:"schema_id"`
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	RunID          string    `json:"run_id"`
	CaptureMode    string    `json:"capture_mode"`
	ManifestDigest string    `json:"manifest_digest"`
	IntentsCount   int       `json:"intents_count"`
	ResultsCount   int       `json:"results_count"`
	RefsCount      int       `json:"refs_count"`
}

type JobPayload struct {
	SchemaID               string    `json:"schema_id"`
	SchemaVersion          string    `json:"schema_version"`
	CreatedAt              time.Time `json:"created_at"`
	JobID                  string    `json:"job_id"`
	Status                 string    `json:"status"`
	StopReason             string    `json:"stop_reason"`
	StatusReasonCode       string    `json:"status_reason_code"`
	EnvironmentFingerprint string    `json:"environment_fingerprint"`
	CheckpointCount        int       `json:"checkpoint_count"`
	ApprovalCount          int       `json:"approval_count"`
}

type DiffSummary struct {
	Changed       bool     `json:"changed"`
	AddedFiles    []string `json:"added_files,omitempty"`
	RemovedFiles  []string `json:"removed_files,omitempty"`
	ChangedFiles  []string `json:"changed_files,omitempty"`
	ManifestDelta bool     `json:"manifest_delta"`
}

type DiffResult struct {
	SchemaID      string      `json:"schema_id"`
	SchemaVersion string      `json:"schema_version"`
	CreatedAt     time.Time   `json:"created_at"`
	LeftPackID    string      `json:"left_pack_id"`
	RightPackID   string      `json:"right_pack_id"`
	LeftPackType  string      `json:"left_pack_type"`
	RightPackType string      `json:"right_pack_type"`
	Summary       DiffSummary `json:"summary"`
}
