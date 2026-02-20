package mcp

import (
	"context"
	"time"
)

type ToolCall struct {
	Name          string          `json:"name"`
	Args          map[string]any  `json:"args,omitempty"`
	Script        *ScriptCall     `json:"script,omitempty"`
	Target        string          `json:"target,omitempty"`
	Targets       []Target        `json:"targets,omitempty"`
	ArgProvenance []ArgProvenance `json:"arg_provenance,omitempty"`
	Context       CallContext     `json:"context,omitempty"`
	Delegation    *Delegation     `json:"delegation,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}

type ScriptCall struct {
	Steps []ScriptStep `json:"steps"`
}

type ScriptStep struct {
	Name          string          `json:"name"`
	Args          map[string]any  `json:"args,omitempty"`
	Targets       []Target        `json:"targets,omitempty"`
	ArgProvenance []ArgProvenance `json:"arg_provenance,omitempty"`
}

type Target struct {
	Kind        string `json:"kind"`
	Value       string `json:"value"`
	Operation   string `json:"operation,omitempty"`
	Sensitivity string `json:"sensitivity,omitempty"`
}

type ArgProvenance struct {
	ArgPath         string `json:"arg_path"`
	Source          string `json:"source"`
	SourceRef       string `json:"source_ref,omitempty"`
	IntegrityDigest string `json:"integrity_digest,omitempty"`
}

type CallContext struct {
	Identity               string         `json:"identity,omitempty"`
	Workspace              string         `json:"workspace,omitempty"`
	RiskClass              string         `json:"risk_class,omitempty"`
	SessionID              string         `json:"session_id,omitempty"`
	RequestID              string         `json:"request_id,omitempty"`
	RunID                  string         `json:"run_id,omitempty"`
	AuthMode               string         `json:"auth_mode,omitempty"`
	OAuthEvidence          *OAuthEvidence `json:"oauth_evidence,omitempty"`
	AuthContext            map[string]any `json:"auth_context,omitempty"`
	CredentialScopes       []string       `json:"credential_scopes,omitempty"`
	EnvironmentFingerprint string         `json:"environment_fingerprint,omitempty"`
}

type OAuthEvidence struct {
	Issuer      string   `json:"issuer,omitempty"`
	Audience    []string `json:"audience,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	ClientID    string   `json:"client_id,omitempty"`
	TokenType   string   `json:"token_type,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	DCRClientID string   `json:"dcr_client_id,omitempty"`
	RedirectURI string   `json:"redirect_uri,omitempty"`
	TokenBind   string   `json:"token_binding,omitempty"`
	AuthTime    string   `json:"auth_time,omitempty"`
	EvidenceRef string   `json:"evidence_ref,omitempty"`
}

type Delegation struct {
	RequesterIdentity string           `json:"requester_identity"`
	ScopeClass        string           `json:"scope_class,omitempty"`
	TokenRefs         []string         `json:"token_refs,omitempty"`
	Chain             []DelegationLink `json:"chain,omitempty"`
}

type DelegationLink struct {
	DelegatorIdentity string `json:"delegator_identity"`
	DelegateIdentity  string `json:"delegate_identity"`
	ScopeClass        string `json:"scope_class,omitempty"`
	TokenRef          string `json:"token_ref,omitempty"`
}

type ToolResult struct {
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
}

type Adapter interface {
	CallTool(context.Context, ToolCall) (ToolResult, error)
}
