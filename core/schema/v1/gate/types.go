package gate

import "time"

type TraceRecord struct {
	SchemaID         string           `json:"schema_id"`
	SchemaVersion    string           `json:"schema_version"`
	CreatedAt        time.Time        `json:"created_at"`
	ProducerVersion  string           `json:"producer_version"`
	TraceID          string           `json:"trace_id"`
	CorrelationID    string           `json:"correlation_id,omitempty"`
	ToolName         string           `json:"tool_name"`
	ArgsDigest       string           `json:"args_digest"`
	IntentDigest     string           `json:"intent_digest"`
	PolicyDigest     string           `json:"policy_digest"`
	Verdict          string           `json:"verdict"`
	Violations       []string         `json:"violations,omitempty"`
	LatencyMS        float64          `json:"latency_ms,omitempty"`
	ApprovalTokenRef string           `json:"approval_token_ref,omitempty"`
	SkillProvenance  *SkillProvenance `json:"skill_provenance,omitempty"`
	Signature        *Signature       `json:"signature,omitempty"`
}

type Signature struct {
	Alg          string `json:"alg"`
	KeyID        string `json:"key_id"`
	Sig          string `json:"sig"`
	SignedDigest string `json:"signed_digest,omitempty"`
}

type IntentRequest struct {
	SchemaID        string                `json:"schema_id"`
	SchemaVersion   string                `json:"schema_version"`
	CreatedAt       time.Time             `json:"created_at"`
	ProducerVersion string                `json:"producer_version"`
	ToolName        string                `json:"tool_name"`
	Args            map[string]any        `json:"args"`
	ArgsDigest      string                `json:"args_digest,omitempty"`
	IntentDigest    string                `json:"intent_digest,omitempty"`
	Targets         []IntentTarget        `json:"targets"`
	ArgProvenance   []IntentArgProvenance `json:"arg_provenance,omitempty"`
	SkillProvenance *SkillProvenance      `json:"skill_provenance,omitempty"`
	Context         IntentContext         `json:"context"`
}

type IntentTarget struct {
	Kind           string `json:"kind"`
	Value          string `json:"value"`
	Operation      string `json:"operation,omitempty"`
	Sensitivity    string `json:"sensitivity,omitempty"`
	EndpointClass  string `json:"endpoint_class,omitempty"`
	EndpointDomain string `json:"endpoint_domain,omitempty"`
	Destructive    bool   `json:"destructive,omitempty"`
}

type IntentArgProvenance struct {
	ArgPath         string `json:"arg_path"`
	Source          string `json:"source"`
	SourceRef       string `json:"source_ref,omitempty"`
	IntegrityDigest string `json:"integrity_digest,omitempty"`
}

type IntentContext struct {
	Identity  string `json:"identity"`
	Workspace string `json:"workspace"`
	RiskClass string `json:"risk_class"`
	SessionID string `json:"session_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type SkillProvenance struct {
	SkillName      string `json:"skill_name"`
	SkillVersion   string `json:"skill_version,omitempty"`
	Source         string `json:"source"`
	Publisher      string `json:"publisher"`
	Digest         string `json:"digest,omitempty"`
	SignatureKeyID string `json:"signature_key_id,omitempty"`
}

type GateResult struct {
	SchemaID        string    `json:"schema_id"`
	SchemaVersion   string    `json:"schema_version"`
	CreatedAt       time.Time `json:"created_at"`
	ProducerVersion string    `json:"producer_version"`
	Verdict         string    `json:"verdict"`
	ReasonCodes     []string  `json:"reason_codes"`
	Violations      []string  `json:"violations"`
}

type ApprovalToken struct {
	SchemaID         string     `json:"schema_id"`
	SchemaVersion    string     `json:"schema_version"`
	CreatedAt        time.Time  `json:"created_at"`
	ProducerVersion  string     `json:"producer_version"`
	TokenID          string     `json:"token_id"`
	ApproverIdentity string     `json:"approver_identity"`
	ReasonCode       string     `json:"reason_code"`
	IntentDigest     string     `json:"intent_digest"`
	PolicyDigest     string     `json:"policy_digest"`
	Scope            []string   `json:"scope"`
	ExpiresAt        time.Time  `json:"expires_at"`
	Signature        *Signature `json:"signature,omitempty"`
}

type ApprovalAuditEntry struct {
	TokenID          string    `json:"token_id,omitempty"`
	ApproverIdentity string    `json:"approver_identity,omitempty"`
	ReasonCode       string    `json:"reason_code,omitempty"`
	Scope            []string  `json:"scope,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	Valid            bool      `json:"valid"`
	ErrorCode        string    `json:"error_code,omitempty"`
}

type ApprovalAuditRecord struct {
	SchemaID          string               `json:"schema_id"`
	SchemaVersion     string               `json:"schema_version"`
	CreatedAt         time.Time            `json:"created_at"`
	ProducerVersion   string               `json:"producer_version"`
	TraceID           string               `json:"trace_id"`
	ToolName          string               `json:"tool_name"`
	IntentDigest      string               `json:"intent_digest"`
	PolicyDigest      string               `json:"policy_digest"`
	RequiredApprovals int                  `json:"required_approvals"`
	ValidApprovals    int                  `json:"valid_approvals"`
	Approved          bool                 `json:"approved"`
	Approvers         []string             `json:"approvers,omitempty"`
	Entries           []ApprovalAuditEntry `json:"entries"`
}

type BrokerCredentialRecord struct {
	SchemaID        string    `json:"schema_id"`
	SchemaVersion   string    `json:"schema_version"`
	CreatedAt       time.Time `json:"created_at"`
	ProducerVersion string    `json:"producer_version"`
	TraceID         string    `json:"trace_id"`
	ToolName        string    `json:"tool_name"`
	Identity        string    `json:"identity"`
	Broker          string    `json:"broker"`
	Reference       string    `json:"reference,omitempty"`
	Scope           []string  `json:"scope,omitempty"`
	CredentialRef   string    `json:"credential_ref"`
}
