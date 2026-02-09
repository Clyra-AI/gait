package registry

import "time"

type RegistryPack struct {
	SchemaID        string         `json:"schema_id"`
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       time.Time      `json:"created_at"`
	ProducerVersion string         `json:"producer_version"`
	PackName        string         `json:"pack_name"`
	PackVersion     string         `json:"pack_version"`
	PackType        string         `json:"pack_type,omitempty"`
	Publisher       string         `json:"publisher,omitempty"`
	Source          string         `json:"source,omitempty"`
	RiskClass       string         `json:"risk_class,omitempty"`
	UseCase         string         `json:"use_case,omitempty"`
	Compatibility   []string       `json:"compatibility,omitempty"`
	Provenance      map[string]any `json:"provenance,omitempty"`
	Artifacts       []PackArtifact `json:"artifacts"`
	Signatures      []SignatureRef `json:"signatures,omitempty"`
}

type PackArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Media  string `json:"media_type,omitempty"`
}

type SignatureRef struct {
	Alg          string `json:"alg"`
	KeyID        string `json:"key_id"`
	Sig          string `json:"sig"`
	SignedDigest string `json:"signed_digest,omitempty"`
}

type VerificationReport struct {
	SchemaID          string    `json:"schema_id"`
	SchemaVersion     string    `json:"schema_version"`
	CreatedAt         time.Time `json:"created_at"`
	ProducerVersion   string    `json:"producer_version"`
	PackName          string    `json:"pack_name"`
	PackVersion       string    `json:"pack_version"`
	Digest            string    `json:"digest"`
	MetadataPath      string    `json:"metadata_path"`
	PinPath           string    `json:"pin_path,omitempty"`
	PinDigest         string    `json:"pin_digest,omitempty"`
	PinPresent        bool      `json:"pin_present"`
	PinVerified       bool      `json:"pin_verified"`
	SignatureVerified bool      `json:"signature_verified"`
	Publisher         string    `json:"publisher,omitempty"`
	Source            string    `json:"source,omitempty"`
	PublisherAllowed  bool      `json:"publisher_allowed"`
	Status            string    `json:"status"`
	FailureReasons    []string  `json:"failure_reasons,omitempty"`
}
