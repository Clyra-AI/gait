package regress

import "time"

type RegressResult struct {
	SchemaID        string         `json:"schema_id"`
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       time.Time      `json:"created_at"`
	ProducerVersion string         `json:"producer_version"`
	FixtureSet      string         `json:"fixture_set"`
	Status          string         `json:"status"`
	Graders         []GraderResult `json:"graders"`
}

type GraderResult struct {
	Name               string         `json:"name"`
	Status             string         `json:"status"`
	ReasonCodes        []string       `json:"reason_codes"`
	ContextConformance string         `json:"context_conformance,omitempty"`
	Details            map[string]any `json:"details,omitempty"`
}
