package guard

import "time"

type PackManifest struct {
	SchemaID        string      `json:"schema_id"`
	SchemaVersion   string      `json:"schema_version"`
	CreatedAt       time.Time   `json:"created_at"`
	ProducerVersion string      `json:"producer_version"`
	PackID          string      `json:"pack_id"`
	RunID           string      `json:"run_id"`
	CaseID          string      `json:"case_id,omitempty"`
	TemplateID      string      `json:"template_id,omitempty"`
	GeneratedAt     time.Time   `json:"generated_at"`
	ControlIndex    []Control   `json:"control_index,omitempty"`
	EvidencePtrs    []Evidence  `json:"evidence_pointers,omitempty"`
	IncidentWindow  *Window     `json:"incident_window,omitempty"`
	Rendered        []Rendered  `json:"rendered_artifacts,omitempty"`
	Contents        []PackEntry `json:"contents"`
}

type PackEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Type   string `json:"type"`
}

type Control struct {
	ControlID     string   `json:"control_id"`
	Title         string   `json:"title"`
	EvidencePaths []string `json:"evidence_paths"`
}

type Evidence struct {
	PointerID string `json:"pointer_id"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	SHA256    string `json:"sha256"`
}

type Window struct {
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	WindowSeconds   int64     `json:"window_seconds"`
	SelectionAnchor string    `json:"selection_anchor,omitempty"`
}

type Rendered struct {
	Format string `json:"format"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type RetentionReport struct {
	SchemaID        string               `json:"schema_id"`
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	ProducerVersion string               `json:"producer_version"`
	RootPath        string               `json:"root_path"`
	DryRun          bool                 `json:"dry_run"`
	TraceTTLSeconds int64                `json:"trace_ttl_seconds"`
	PackTTLSeconds  int64                `json:"pack_ttl_seconds"`
	ScannedFiles    int                  `json:"scanned_files"`
	DeletedFiles    []RetentionFileEvent `json:"deleted_files"`
	KeptFiles       []RetentionFileEvent `json:"kept_files"`
}

type RetentionFileEvent struct {
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	ModifiedAt time.Time `json:"modified_at"`
	AgeSeconds int64     `json:"age_seconds"`
	Action     string    `json:"action"`
}

type EncryptedArtifact struct {
	SchemaID        string               `json:"schema_id"`
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	ProducerVersion string               `json:"producer_version"`
	Algorithm       string               `json:"algorithm"`
	KeySource       EncryptedArtifactKey `json:"key_source"`
	Nonce           string               `json:"nonce"`
	Ciphertext      string               `json:"ciphertext"`
	PlainSHA256     string               `json:"plain_sha256"`
	PlainSize       int                  `json:"plain_size"`
}

type EncryptedArtifactKey struct {
	Mode    string `json:"mode"`
	Ref     string `json:"ref,omitempty"`
	Command string `json:"command,omitempty"`
}
