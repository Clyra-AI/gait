package gate

import (
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
)

type TraceRecord struct {
	SchemaID            string                             `json:"schema_id"`
	SchemaVersion       string                             `json:"schema_version"`
	CreatedAt           time.Time                          `json:"created_at"`
	ObservedAt          time.Time                          `json:"observed_at,omitempty"`
	ProducerVersion     string                             `json:"producer_version"`
	TraceID             string                             `json:"trace_id"`
	EventID             string                             `json:"event_id,omitempty"`
	CorrelationID       string                             `json:"correlation_id,omitempty"`
	ToolName            string                             `json:"tool_name"`
	ArgsDigest          string                             `json:"args_digest"`
	IntentDigest        string                             `json:"intent_digest"`
	PolicyDigest        string                             `json:"policy_digest"`
	PolicyID            string                             `json:"policy_id,omitempty"`
	PolicyVersion       string                             `json:"policy_version,omitempty"`
	MatchedRuleIDs      []string                           `json:"matched_rule_ids,omitempty"`
	Verdict             string                             `json:"verdict"`
	ContextSetDigest    string                             `json:"context_set_digest,omitempty"`
	ContextEvidenceMode string                             `json:"context_evidence_mode,omitempty"`
	ContextRefCount     int                                `json:"context_ref_count,omitempty"`
	ContextSource       string                             `json:"context_source,omitempty"`
	Script              bool                               `json:"script,omitempty"`
	StepCount           int                                `json:"step_count,omitempty"`
	ScriptHash          string                             `json:"script_hash,omitempty"`
	CompositeRiskClass  string                             `json:"composite_risk_class,omitempty"`
	StepVerdicts        []TraceStepVerdict                 `json:"step_verdicts,omitempty"`
	PreApproved         bool                               `json:"pre_approved,omitempty"`
	PatternID           string                             `json:"pattern_id,omitempty"`
	RegistryReason      string                             `json:"registry_reason,omitempty"`
	Violations          []string                           `json:"violations,omitempty"`
	LatencyMS           float64                            `json:"latency_ms,omitempty"`
	ApprovalTokenRef    string                             `json:"approval_token_ref,omitempty"`
	DelegationRef       *DelegationRef                     `json:"delegation_ref,omitempty"`
	Relationship        *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	SkillProvenance     *SkillProvenance                   `json:"skill_provenance,omitempty"`
	Signature           *Signature                         `json:"signature,omitempty"`
}

type TraceStepVerdict struct {
	Index       int      `json:"index"`
	ToolName    string   `json:"tool_name"`
	Verdict     string   `json:"verdict"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Violations  []string `json:"violations,omitempty"`
	MatchedRule string   `json:"matched_rule,omitempty"`
}

type Signature struct {
	Alg          string `json:"alg"`
	KeyID        string `json:"key_id"`
	Sig          string `json:"sig"`
	SignedDigest string `json:"signed_digest,omitempty"`
}

type IntentRequest struct {
	SchemaID        string                             `json:"schema_id"`
	SchemaVersion   string                             `json:"schema_version"`
	CreatedAt       time.Time                          `json:"created_at"`
	ProducerVersion string                             `json:"producer_version"`
	ToolName        string                             `json:"tool_name"`
	Args            map[string]any                     `json:"args"`
	ArgsDigest      string                             `json:"args_digest,omitempty"`
	IntentDigest    string                             `json:"intent_digest,omitempty"`
	ScriptHash      string                             `json:"script_hash,omitempty"`
	Script          *IntentScript                      `json:"script,omitempty"`
	Targets         []IntentTarget                     `json:"targets"`
	ArgProvenance   []IntentArgProvenance              `json:"arg_provenance,omitempty"`
	SkillProvenance *SkillProvenance                   `json:"skill_provenance,omitempty"`
	Delegation      *IntentDelegation                  `json:"delegation,omitempty"`
	Relationship    *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	Context         IntentContext                      `json:"context"`
}

type IntentScript struct {
	Steps []IntentScriptStep `json:"steps"`
}

type IntentScriptStep struct {
	ToolName      string                `json:"tool_name"`
	Args          map[string]any        `json:"args"`
	Targets       []IntentTarget        `json:"targets,omitempty"`
	ArgProvenance []IntentArgProvenance `json:"arg_provenance,omitempty"`
}

type IntentTarget struct {
	Kind            string `json:"kind"`
	Value           string `json:"value"`
	Operation       string `json:"operation,omitempty"`
	Sensitivity     string `json:"sensitivity,omitempty"`
	EndpointClass   string `json:"endpoint_class,omitempty"`
	EndpointDomain  string `json:"endpoint_domain,omitempty"`
	Destructive     bool   `json:"destructive,omitempty"`
	DiscoveryMethod string `json:"discovery_method,omitempty"`
	ReadOnlyHint    bool   `json:"read_only_hint,omitempty"`
	DestructiveHint bool   `json:"destructive_hint,omitempty"`
	IdempotentHint  bool   `json:"idempotent_hint,omitempty"`
	OpenWorldHint   bool   `json:"open_world_hint,omitempty"`
}

type IntentArgProvenance struct {
	ArgPath         string `json:"arg_path"`
	Source          string `json:"source"`
	SourceRef       string `json:"source_ref,omitempty"`
	IntegrityDigest string `json:"integrity_digest,omitempty"`
}

type IntentContext struct {
	Identity               string         `json:"identity"`
	Workspace              string         `json:"workspace"`
	RiskClass              string         `json:"risk_class"`
	Phase                  string         `json:"phase,omitempty"`
	JobID                  string         `json:"job_id,omitempty"`
	SessionID              string         `json:"session_id,omitempty"`
	RequestID              string         `json:"request_id,omitempty"`
	AuthContext            map[string]any `json:"auth_context,omitempty"`
	CredentialScopes       []string       `json:"credential_scopes,omitempty"`
	EnvironmentFingerprint string         `json:"environment_fingerprint,omitempty"`
	ContextSetDigest       string         `json:"context_set_digest,omitempty"`
	ContextEvidenceMode    string         `json:"context_evidence_mode,omitempty"`
	ContextRefs            []string       `json:"context_refs,omitempty"`
}

type IntentDelegation struct {
	RequesterIdentity string           `json:"requester_identity"`
	ScopeClass        string           `json:"scope_class,omitempty"`
	TokenRefs         []string         `json:"token_refs,omitempty"`
	Chain             []DelegationLink `json:"chain,omitempty"`
	IssuedAt          time.Time        `json:"issued_at,omitempty"`
	ExpiresAt         time.Time        `json:"expires_at,omitempty"`
}

type DelegationLink struct {
	DelegatorIdentity string    `json:"delegator_identity"`
	DelegateIdentity  string    `json:"delegate_identity"`
	ScopeClass        string    `json:"scope_class,omitempty"`
	TokenRef          string    `json:"token_ref,omitempty"`
	IssuedAt          time.Time `json:"issued_at,omitempty"`
	ExpiresAt         time.Time `json:"expires_at,omitempty"`
}

type DelegationRef struct {
	DelegationTokenRef string   `json:"delegation_token_ref,omitempty"`
	RequesterIdentity  string   `json:"requester_identity,omitempty"`
	DelegationDepth    int      `json:"delegation_depth,omitempty"`
	ScopeClass         string   `json:"scope_class,omitempty"`
	ChainDigest        string   `json:"chain_digest,omitempty"`
	ReasonCodes        []string `json:"reason_codes,omitempty"`
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
	SchemaID                string     `json:"schema_id"`
	SchemaVersion           string     `json:"schema_version"`
	CreatedAt               time.Time  `json:"created_at"`
	ProducerVersion         string     `json:"producer_version"`
	TokenID                 string     `json:"token_id"`
	ApproverIdentity        string     `json:"approver_identity"`
	ReasonCode              string     `json:"reason_code"`
	IntentDigest            string     `json:"intent_digest"`
	PolicyDigest            string     `json:"policy_digest"`
	DelegationBindingDigest string     `json:"delegation_binding_digest,omitempty"`
	Scope                   []string   `json:"scope"`
	MaxTargets              int        `json:"max_targets,omitempty"`
	MaxOps                  int        `json:"max_ops,omitempty"`
	ExpiresAt               time.Time  `json:"expires_at"`
	Signature               *Signature `json:"signature,omitempty"`
}

type DelegationToken struct {
	SchemaID          string     `json:"schema_id"`
	SchemaVersion     string     `json:"schema_version"`
	CreatedAt         time.Time  `json:"created_at"`
	ProducerVersion   string     `json:"producer_version"`
	TokenID           string     `json:"token_id"`
	DelegatorIdentity string     `json:"delegator_identity"`
	DelegateIdentity  string     `json:"delegate_identity"`
	Scope             []string   `json:"scope"`
	ScopeClass        string     `json:"scope_class,omitempty"`
	IntentDigest      string     `json:"intent_digest,omitempty"`
	PolicyDigest      string     `json:"policy_digest,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
	Signature         *Signature `json:"signature,omitempty"`
}

type DelegationAuditEntry struct {
	TokenID           string    `json:"token_id,omitempty"`
	DelegatorIdentity string    `json:"delegator_identity,omitempty"`
	DelegateIdentity  string    `json:"delegate_identity,omitempty"`
	Scope             []string  `json:"scope,omitempty"`
	ExpiresAt         time.Time `json:"expires_at,omitempty"`
	Valid             bool      `json:"valid"`
	ErrorCode         string    `json:"error_code,omitempty"`
}

type DelegationAuditRecord struct {
	SchemaID           string                             `json:"schema_id"`
	SchemaVersion      string                             `json:"schema_version"`
	CreatedAt          time.Time                          `json:"created_at"`
	ProducerVersion    string                             `json:"producer_version"`
	TraceID            string                             `json:"trace_id"`
	ToolName           string                             `json:"tool_name"`
	IntentDigest       string                             `json:"intent_digest"`
	PolicyDigest       string                             `json:"policy_digest"`
	DelegationRequired bool                               `json:"delegation_required"`
	ValidDelegations   int                                `json:"valid_delegations"`
	Delegated          bool                               `json:"delegated"`
	DelegationRef      string                             `json:"delegation_ref,omitempty"`
	Relationship       *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	Entries            []DelegationAuditEntry             `json:"entries"`
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
	SchemaID          string                             `json:"schema_id"`
	SchemaVersion     string                             `json:"schema_version"`
	CreatedAt         time.Time                          `json:"created_at"`
	ProducerVersion   string                             `json:"producer_version"`
	TraceID           string                             `json:"trace_id"`
	ToolName          string                             `json:"tool_name"`
	IntentDigest      string                             `json:"intent_digest"`
	PolicyDigest      string                             `json:"policy_digest"`
	RequiredApprovals int                                `json:"required_approvals"`
	ValidApprovals    int                                `json:"valid_approvals"`
	Approved          bool                               `json:"approved"`
	Approvers         []string                           `json:"approvers,omitempty"`
	Relationship      *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	Entries           []ApprovalAuditEntry               `json:"entries"`
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
	IssuedAt        time.Time `json:"issued_at,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	TTLSeconds      int64     `json:"ttl_seconds,omitempty"`
}

type ApprovedScriptEntry struct {
	SchemaID         string     `json:"schema_id"`
	SchemaVersion    string     `json:"schema_version"`
	CreatedAt        time.Time  `json:"created_at"`
	ProducerVersion  string     `json:"producer_version"`
	PatternID        string     `json:"pattern_id"`
	PolicyDigest     string     `json:"policy_digest"`
	ScriptHash       string     `json:"script_hash"`
	ToolSequence     []string   `json:"tool_sequence"`
	Scope            []string   `json:"scope,omitempty"`
	ApproverIdentity string     `json:"approver_identity"`
	ExpiresAt        time.Time  `json:"expires_at"`
	Signature        *Signature `json:"signature,omitempty"`
}
