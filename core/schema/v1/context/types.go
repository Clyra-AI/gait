package context

import "time"

type ReferenceRecord struct {
	RefID               string         `json:"ref_id"`
	SourceType          string         `json:"source_type"`
	SourceLocator       string         `json:"source_locator"`
	QueryDigest         string         `json:"query_digest"`
	ContentDigest       string         `json:"content_digest"`
	RetrievedAt         time.Time      `json:"retrieved_at"`
	RedactionMode       string         `json:"redaction_mode"`
	Immutability        string         `json:"immutability"`
	SensitivityLabel    string         `json:"sensitivity_label,omitempty"`
	FreshnessSLASeconds int64          `json:"freshness_sla_seconds,omitempty"`
	RetrievalParams     map[string]any `json:"retrieval_params,omitempty"`
}

type Envelope struct {
	SchemaID         string            `json:"schema_id"`
	SchemaVersion    string            `json:"schema_version"`
	CreatedAt        time.Time         `json:"created_at"`
	ProducerVersion  string            `json:"producer_version"`
	ContextSetID     string            `json:"context_set_id"`
	ContextSetDigest string            `json:"context_set_digest"`
	EvidenceMode     string            `json:"evidence_mode"`
	Records          []ReferenceRecord `json:"records"`
}

type BudgetReport struct {
	SchemaID         string         `json:"schema_id"`
	SchemaVersion    string         `json:"schema_version"`
	CreatedAt        time.Time      `json:"created_at"`
	ProducerVersion  string         `json:"producer_version"`
	ContextSetDigest string         `json:"context_set_digest"`
	ItemsConsidered  int            `json:"items_considered"`
	ItemsIncluded    int            `json:"items_included"`
	ItemsDropped     int            `json:"items_dropped"`
	DropReasons      map[string]int `json:"drop_reasons,omitempty"`
}
