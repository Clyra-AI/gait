package gate

import (
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
)

type TraceRecord struct {
	SchemaID                   string                             `json:"schema_id"`
	SchemaVersion              string                             `json:"schema_version"`
	CreatedAt                  time.Time                          `json:"created_at"`
	ObservedAt                 time.Time                          `json:"observed_at,omitempty"`
	ProducerVersion            string                             `json:"producer_version"`
	TraceID                    string                             `json:"trace_id"`
	EventID                    string                             `json:"event_id,omitempty"`
	CorrelationID              string                             `json:"correlation_id,omitempty"`
	ToolName                   string                             `json:"tool_name"`
	ArgsDigest                 string                             `json:"args_digest"`
	IntentDigest               string                             `json:"intent_digest"`
	PolicyDigest               string                             `json:"policy_digest"`
	AgentID                    string                             `json:"agent_id,omitempty"`
	AgentIdentity              *AgentIdentity                     `json:"agent_identity,omitempty"`
	RunID                      string                             `json:"run_id,omitempty"`
	WorkflowID                 string                             `json:"workflow_id,omitempty"`
	Repo                       string                             `json:"repo,omitempty"`
	Environment                string                             `json:"environment,omitempty"`
	CredentialRef              string                             `json:"credential_ref,omitempty"`
	CredentialSource           string                             `json:"credential_source,omitempty"`
	CredentialAccessType       string                             `json:"credential_access_type,omitempty"`
	CredentialIssuer           string                             `json:"credential_issuer,omitempty"`
	CredentialSubject          string                             `json:"credential_subject,omitempty"`
	CredentialOwner            string                             `json:"credential_owner,omitempty"`
	CredentialTargetBinding    string                             `json:"credential_target_binding,omitempty"`
	CredentialRunBinding       string                             `json:"credential_run_binding,omitempty"`
	CredentialJobBinding       string                             `json:"credential_job_binding,omitempty"`
	CredentialTTLSeconds       int64                              `json:"credential_ttl_seconds,omitempty"`
	BrokerCredentialRef        string                             `json:"broker_credential_ref,omitempty"`
	BrokerCredentialSource     string                             `json:"broker_credential_source,omitempty"`
	BrokerCredentialAccessType string                             `json:"broker_credential_access_type,omitempty"`
	BrokerCredentialIssuer     string                             `json:"broker_credential_issuer,omitempty"`
	BrokerRequestDigest        string                             `json:"broker_request_digest,omitempty"`
	BrokerTargetBinding        string                             `json:"broker_target_binding,omitempty"`
	BrokerRunBinding           string                             `json:"broker_run_binding,omitempty"`
	BrokerJobBinding           string                             `json:"broker_job_binding,omitempty"`
	ApprovalRef                string                             `json:"approval_ref,omitempty"`
	WrkrInventoryRef           string                             `json:"wrkr_inventory_ref,omitempty"`
	AgentActionBOMRef          string                             `json:"agent_action_bom_ref,omitempty"`
	PolicyID                   string                             `json:"policy_id,omitempty"`
	PolicyVersion              string                             `json:"policy_version,omitempty"`
	MatchedRuleIDs             []string                           `json:"matched_rule_ids,omitempty"`
	Verdict                    string                             `json:"verdict"`
	ContextSetDigest           string                             `json:"context_set_digest,omitempty"`
	ContextEvidenceMode        string                             `json:"context_evidence_mode,omitempty"`
	ContextRefCount            int                                `json:"context_ref_count,omitempty"`
	ContextSource              string                             `json:"context_source,omitempty"`
	Script                     bool                               `json:"script,omitempty"`
	StepCount                  int                                `json:"step_count,omitempty"`
	ScriptHash                 string                             `json:"script_hash,omitempty"`
	CompositeRiskClass         string                             `json:"composite_risk_class,omitempty"`
	StepVerdicts               []TraceStepVerdict                 `json:"step_verdicts,omitempty"`
	PreApproved                bool                               `json:"pre_approved,omitempty"`
	PatternID                  string                             `json:"pattern_id,omitempty"`
	RegistryReason             string                             `json:"registry_reason,omitempty"`
	Violations                 []string                           `json:"violations,omitempty"`
	LatencyMS                  float64                            `json:"latency_ms,omitempty"`
	ApprovalTokenRef           string                             `json:"approval_token_ref,omitempty"`
	DelegationRef              *DelegationRef                     `json:"delegation_ref,omitempty"`
	FreezeWindow               *FreezeWindowDecision              `json:"freeze_window,omitempty"`
	Sandbox                    *SandboxDecision                   `json:"sandbox,omitempty"`
	KillSwitch                 *KillSwitchDecision                `json:"kill_switch,omitempty"`
	MCPTrust                   *MCPTrustDecision                  `json:"mcp_trust,omitempty"`
	Relationship               *schemacommon.RelationshipEnvelope `json:"relationship,omitempty"`
	SkillProvenance            *SkillProvenance                   `json:"skill_provenance,omitempty"`
	Signature                  *Signature                         `json:"signature,omitempty"`
}

type MCPTrustDecision struct {
	ServerID         string    `json:"server_id,omitempty"`
	ServerName       string    `json:"server_name,omitempty"`
	Publisher        string    `json:"publisher,omitempty"`
	Source           string    `json:"source,omitempty"`
	Status           string    `json:"status,omitempty"`
	DecisionSource   string    `json:"decision_source,omitempty"`
	Score            float64   `json:"score,omitempty"`
	Threshold        float64   `json:"threshold,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
	MaxAgeSeconds    int64     `json:"max_age_seconds,omitempty"`
	Required         bool      `json:"required,omitempty"`
	Enforced         bool      `json:"enforced,omitempty"`
	RegistryVerified bool      `json:"registry_verified,omitempty"`
	PublisherAllowed bool      `json:"publisher_allowed,omitempty"`
	ReasonCodes      []string  `json:"reason_codes,omitempty"`
}

type FreezeWindowDecision struct {
	Status      string    `json:"status,omitempty"`
	Effect      string    `json:"effect,omitempty"`
	Timezone    string    `json:"timezone,omitempty"`
	EvaluatedAt time.Time `json:"evaluated_at,omitempty"`
	WindowName  string    `json:"window_name,omitempty"`
	WindowStart time.Time `json:"window_start,omitempty"`
	WindowEnd   time.Time `json:"window_end,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	ReasonCode  string    `json:"reason_code,omitempty"`
}

type SandboxMetadata struct {
	NetworkMode         string   `json:"network_mode,omitempty"`
	WritablePaths       []string `json:"writable_paths,omitempty"`
	ReadOnlyRoots       []string `json:"read_only_roots,omitempty"`
	EnvExposureMode     string   `json:"env_exposure_mode,omitempty"`
	TimeoutSeconds      int64    `json:"timeout_seconds,omitempty"`
	FilesystemIsolation string   `json:"filesystem_isolation,omitempty"`
	UserMode            string   `json:"user_mode,omitempty"`
	EvidenceRef         string   `json:"evidence_ref,omitempty"`
	EvidenceDigest      string   `json:"evidence_digest,omitempty"`
}

type SandboxDecision struct {
	Status              string   `json:"status,omitempty"`
	NetworkMode         string   `json:"network_mode,omitempty"`
	EnvExposureMode     string   `json:"env_exposure_mode,omitempty"`
	TimeoutSeconds      int64    `json:"timeout_seconds,omitempty"`
	FilesystemIsolation string   `json:"filesystem_isolation,omitempty"`
	UserMode            string   `json:"user_mode,omitempty"`
	EvidenceRef         string   `json:"evidence_ref,omitempty"`
	EvidenceDigest      string   `json:"evidence_digest,omitempty"`
	ReasonCodes         []string `json:"reason_codes,omitempty"`
}

type KillSwitchState struct {
	SchemaID        string            `json:"schema_id"`
	SchemaVersion   string            `json:"schema_version"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	ProducerVersion string            `json:"producer_version"`
	Entries         []KillSwitchEntry `json:"entries"`
}

type KillSwitchEntry struct {
	EntryID           string    `json:"entry_id"`
	Enabled           bool      `json:"enabled"`
	AgentID           string    `json:"agent_id,omitempty"`
	Identity          string    `json:"identity,omitempty"`
	ToolName          string    `json:"tool_name,omitempty"`
	TargetKind        string    `json:"target_kind,omitempty"`
	TargetValue       string    `json:"target_value,omitempty"`
	Environment       string    `json:"environment,omitempty"`
	PathPrefixes      []string  `json:"path_prefixes,omitempty"`
	WorkspacePrefixes []string  `json:"workspace_prefixes,omitempty"`
	Reason            string    `json:"reason,omitempty"`
	Actor             string    `json:"actor,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at,omitempty"`
}

type KillSwitchDecision struct {
	Status          string    `json:"status,omitempty"`
	ReasonCode      string    `json:"reason_code,omitempty"`
	ReasonCodes     []string  `json:"reason_codes,omitempty"`
	MatchedEntryIDs []string  `json:"matched_entry_ids,omitempty"`
	EvaluatedAt     time.Time `json:"evaluated_at,omitempty"`
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

type AgentIdentity struct {
	LifecycleStates []string  `json:"lifecycle_states,omitempty"`
	Owner           string    `json:"owner,omitempty"`
	ManifestDigest  string    `json:"manifest_digest,omitempty"`
	Publisher       string    `json:"publisher,omitempty"`
	Source          string    `json:"source,omitempty"`
	IssuedAt        time.Time `json:"issued_at,omitempty"`
	ApprovedAt      time.Time `json:"approved_at,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Revoked         bool      `json:"revoked,omitempty"`
}

type IntentContext struct {
	Identity                string           `json:"identity"`
	Workspace               string           `json:"workspace"`
	RiskClass               string           `json:"risk_class"`
	Phase                   string           `json:"phase,omitempty"`
	AgentID                 string           `json:"agent_id,omitempty"`
	AgentIdentity           *AgentIdentity   `json:"agent_identity,omitempty"`
	RunID                   string           `json:"run_id,omitempty"`
	WorkflowID              string           `json:"workflow_id,omitempty"`
	Repo                    string           `json:"repo,omitempty"`
	Environment             string           `json:"environment,omitempty"`
	JobID                   string           `json:"job_id,omitempty"`
	SessionID               string           `json:"session_id,omitempty"`
	RequestID               string           `json:"request_id,omitempty"`
	CredentialRef           string           `json:"credential_ref,omitempty"`
	CredentialSource        string           `json:"credential_source,omitempty"`
	CredentialAccessType    string           `json:"credential_access_type,omitempty"`
	CredentialIssuer        string           `json:"credential_issuer,omitempty"`
	CredentialSubject       string           `json:"credential_subject,omitempty"`
	CredentialOwner         string           `json:"credential_owner,omitempty"`
	CredentialTargetBinding string           `json:"credential_target_binding,omitempty"`
	CredentialRunBinding    string           `json:"credential_run_binding,omitempty"`
	CredentialJobBinding    string           `json:"credential_job_binding,omitempty"`
	CredentialTTLSeconds    int64            `json:"credential_ttl_seconds,omitempty"`
	ApprovalRef             string           `json:"approval_ref,omitempty"`
	WrkrInventoryRef        string           `json:"wrkr_inventory_ref,omitempty"`
	AgentActionBOMRef       string           `json:"agent_action_bom_ref,omitempty"`
	AuthContext             map[string]any   `json:"auth_context,omitempty"`
	Sandbox                 *SandboxMetadata `json:"sandbox,omitempty"`
	CredentialScopes        []string         `json:"credential_scopes,omitempty"`
	EnvironmentFingerprint  string           `json:"environment_fingerprint,omitempty"`
	ContextSetDigest        string           `json:"context_set_digest,omitempty"`
	ContextEvidenceMode     string           `json:"context_evidence_mode,omitempty"`
	ContextRefs             []string         `json:"context_refs,omitempty"`
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

type PolicyExplain struct {
	OK                       bool                    `json:"ok"`
	SchemaID                 string                  `json:"schema_id"`
	SchemaVersion            string                  `json:"schema_version"`
	CreatedAt                time.Time               `json:"created_at"`
	ProducerVersion          string                  `json:"producer_version"`
	Verdict                  string                  `json:"verdict"`
	MatchedRule              string                  `json:"matched_rule,omitempty"`
	MatchedRules             []PolicyExplainRule     `json:"matched_rules,omitempty"`
	ReasonCodes              []string                `json:"reason_codes,omitempty"`
	Violations               []string                `json:"violations,omitempty"`
	MissingFields            []string                `json:"missing_fields,omitempty"`
	FailClosedReasonCodes    []string                `json:"fail_closed_reason_codes,omitempty"`
	ApprovalRequired         bool                    `json:"approval_required,omitempty"`
	RequiredApprovals        int                     `json:"required_approvals,omitempty"`
	ValidApprovals           int                     `json:"valid_approvals,omitempty"`
	RequireBrokerCredential  bool                    `json:"require_broker_credential,omitempty"`
	BrokerReference          string                  `json:"broker_reference,omitempty"`
	BrokerScopes             []string                `json:"broker_scopes,omitempty"`
	RequireDelegation        bool                    `json:"require_delegation,omitempty"`
	RequiredDelegationScopes []string                `json:"required_delegation_scopes,omitempty"`
	CredentialPosture        *PolicyCredentialState  `json:"credential_posture,omitempty"`
	ContextEvidenceStatus    string                  `json:"context_evidence_status,omitempty"`
	FreezeWindow             *FreezeWindowDecision   `json:"freeze_window,omitempty"`
	KillSwitch               *KillSwitchDecision     `json:"kill_switch,omitempty"`
	Sandbox                  *SandboxDecision        `json:"sandbox,omitempty"`
	ProofRefs                *PolicyExplainProofRefs `json:"proof_refs,omitempty"`
}

type PolicyExplainRule struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	Effect   string `json:"effect"`
}

type PolicyCredentialState struct {
	Present       bool   `json:"present"`
	Source        string `json:"source,omitempty"`
	AccessType    string `json:"access_type,omitempty"`
	Issuer        string `json:"issuer,omitempty"`
	TTLSeconds    int64  `json:"ttl_seconds,omitempty"`
	CredentialRef string `json:"credential_ref,omitempty"`
}

type PolicyExplainProofRefs struct {
	TraceID                string `json:"trace_id,omitempty"`
	TracePath              string `json:"trace_path,omitempty"`
	ApprovalAuditPath      string `json:"approval_audit_path,omitempty"`
	DelegationAuditPath    string `json:"delegation_audit_path,omitempty"`
	CredentialEvidencePath string `json:"credential_evidence_path,omitempty"`
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
	SchemaID             string    `json:"schema_id"`
	SchemaVersion        string    `json:"schema_version"`
	CreatedAt            time.Time `json:"created_at"`
	ProducerVersion      string    `json:"producer_version"`
	TraceID              string    `json:"trace_id"`
	ToolName             string    `json:"tool_name"`
	Identity             string    `json:"identity"`
	Broker               string    `json:"broker"`
	Reference            string    `json:"reference,omitempty"`
	CredentialSource     string    `json:"credential_source,omitempty"`
	CredentialAccessType string    `json:"credential_access_type,omitempty"`
	CredentialIssuer     string    `json:"credential_issuer,omitempty"`
	CredentialSubject    string    `json:"credential_subject,omitempty"`
	CredentialOwner      string    `json:"credential_owner,omitempty"`
	Scope                []string  `json:"scope,omitempty"`
	CredentialRef        string    `json:"credential_ref"`
	TargetBinding        string    `json:"target_binding,omitempty"`
	RunBinding           string    `json:"run_binding,omitempty"`
	JobBinding           string    `json:"job_binding,omitempty"`
	RequestDigest        string    `json:"request_digest,omitempty"`
	IssuedAt             time.Time `json:"issued_at,omitempty"`
	ExpiresAt            time.Time `json:"expires_at,omitempty"`
	TTLSeconds           int64     `json:"ttl_seconds,omitempty"`
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

type AuthorizationBundle struct {
	SchemaID                 string                `json:"schema_id"`
	SchemaVersion            string                `json:"schema_version"`
	CreatedAt                time.Time             `json:"created_at"`
	ProducerVersion          string                `json:"producer_version"`
	TraceID                  string                `json:"trace_id"`
	PolicyDigest             string                `json:"policy_digest"`
	IntentDigest             string                `json:"intent_digest"`
	TracePath                string                `json:"trace_path,omitempty"`
	TraceDigest              string                `json:"trace_digest,omitempty"`
	ApprovalAuditPath        string                `json:"approval_audit_path,omitempty"`
	ApprovalAuditDigest      string                `json:"approval_audit_digest,omitempty"`
	CredentialEvidencePath   string                `json:"credential_evidence_path,omitempty"`
	CredentialEvidenceDigest string                `json:"credential_evidence_digest,omitempty"`
	DelegationAuditPath      string                `json:"delegation_audit_path,omitempty"`
	DelegationAuditDigest    string                `json:"delegation_audit_digest,omitempty"`
	ContextEvidencePath      string                `json:"context_evidence_path,omitempty"`
	ContextEvidenceDigest    string                `json:"context_evidence_digest,omitempty"`
	OutcomePath              string                `json:"outcome_path,omitempty"`
	OutcomeDigest            string                `json:"outcome_digest,omitempty"`
	OutcomeStatus            string                `json:"outcome_status,omitempty"`
	FreezeWindow             *FreezeWindowDecision `json:"freeze_window,omitempty"`
	KillSwitch               *KillSwitchDecision   `json:"kill_switch,omitempty"`
	Sandbox                  *SandboxDecision      `json:"sandbox,omitempty"`
}
