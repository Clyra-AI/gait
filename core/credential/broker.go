package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	jcs "github.com/Clyra-AI/proof/canon"
)

var ErrCredentialUnavailable = errors.New("credential unavailable")

type Request struct {
	ToolName                                                                              string
	Identity                                                                              string
	Workspace                                                                             string
	SessionID                                                                             string
	RequestID                                                                             string
	RunID                                                                                 string
	JobID                                                                                 string
	Reference                                                                             string
	Scope                                                                                 []string
	TargetBinding                                                                         string
	ContractFamilyID, ContractID                                                          string
	ContractRevision                                                                      int
	ProposalDigest, ActivationDigest, PolicyDigest, ApprovalTokenDigest, DelegationDigest string
	ExpectedOutcome                                                                       string
	EffectScope, ContainmentScope                                                         []string
}

type Response struct {
	IssuedBy            string    `json:"issued_by"`
	Source              string    `json:"source,omitempty"`
	AccessType          string    `json:"access_type,omitempty"`
	Issuer              string    `json:"issuer,omitempty"`
	Subject             string    `json:"subject,omitempty"`
	Owner               string    `json:"owner,omitempty"`
	Scope               []string  `json:"scope,omitempty"`
	CredentialRef       string    `json:"credential_ref"`
	TargetBinding       string    `json:"target_binding,omitempty"`
	RunBinding          string    `json:"run_binding,omitempty"`
	JobBinding          string    `json:"job_binding,omitempty"`
	RequestDigest       string    `json:"request_digest,omitempty"`
	IssuedAt            time.Time `json:"issued_at,omitempty"`
	ExpiresAt           time.Time `json:"expires_at,omitempty"`
	TTLSeconds          int64     `json:"ttl_seconds,omitempty"`
	ContractFamilyID    string    `json:"contract_family_id,omitempty"`
	ContractID          string    `json:"contract_id,omitempty"`
	ContractRevision    int       `json:"contract_revision,omitempty"`
	ProposalDigest      string    `json:"proposal_digest,omitempty"`
	ActivationDigest    string    `json:"activation_digest,omitempty"`
	PolicyDigest        string    `json:"policy_digest,omitempty"`
	ApprovalTokenDigest string    `json:"approval_token_digest,omitempty"`
	DelegationDigest    string    `json:"delegation_digest,omitempty"`
	ExpectedOutcome     string    `json:"expected_outcome,omitempty"`
	EffectScope         []string  `json:"effect_scope,omitempty"`
	ContainmentScope    []string  `json:"containment_scope,omitempty"`
}

type Broker interface {
	Name() string
	Issue(Request) (Response, error)
}

func Issue(broker Broker, request Request) (Response, error) {
	if broker == nil {
		return Response{}, fmt.Errorf("broker is required")
	}
	normalized, err := normalizeRequest(request)
	if err != nil {
		return Response{}, err
	}
	requestDigest, err := RequestDigest(normalized)
	if err != nil {
		return Response{}, err
	}
	response, err := broker.Issue(normalized)
	if err != nil {
		return Response{}, err
	}
	response.IssuedBy = strings.TrimSpace(response.IssuedBy)
	response.Source = strings.ToLower(strings.TrimSpace(response.Source))
	response.AccessType = strings.ToLower(strings.TrimSpace(response.AccessType))
	response.Issuer = strings.TrimSpace(response.Issuer)
	response.Subject = strings.TrimSpace(response.Subject)
	response.Owner = strings.TrimSpace(response.Owner)
	response.Scope = normalizeScope(response.Scope)
	response.CredentialRef = strings.TrimSpace(response.CredentialRef)
	response.TargetBinding = strings.TrimSpace(response.TargetBinding)
	response.RunBinding = strings.TrimSpace(response.RunBinding)
	response.JobBinding = strings.TrimSpace(response.JobBinding)
	response.RequestDigest = strings.ToLower(strings.TrimSpace(response.RequestDigest))
	if normalized.ContractFamilyID != "" && (response.ContractFamilyID != normalized.ContractFamilyID || response.ContractID != normalized.ContractID || response.ContractRevision != normalized.ContractRevision || response.ProposalDigest != normalized.ProposalDigest || response.ActivationDigest != normalized.ActivationDigest || response.PolicyDigest != normalized.PolicyDigest || response.ExpectedOutcome != normalized.ExpectedOutcome || !sameStringSet(response.EffectScope, normalized.EffectScope) || !sameStringSet(response.ContainmentScope, normalized.ContainmentScope)) {
		return Response{}, fmt.Errorf("broker contract binding mismatch")
	}
	if response.IssuedBy == "" {
		response.IssuedBy = broker.Name()
	}
	if response.Source == "" {
		response.Source = broker.Name()
	}
	if response.CredentialRef == "" {
		return Response{}, fmt.Errorf("broker returned empty credential reference")
	}
	if response.RequestDigest == "" {
		response.RequestDigest = requestDigest
	}
	response.IssuedAt = response.IssuedAt.UTC()
	response.ExpiresAt = response.ExpiresAt.UTC()
	if response.IssuedAt.IsZero() && !response.ExpiresAt.IsZero() {
		response.IssuedAt = time.Now().UTC()
	}
	if !response.IssuedAt.IsZero() && !response.ExpiresAt.IsZero() && response.ExpiresAt.After(response.IssuedAt) && response.TTLSeconds == 0 {
		response.TTLSeconds = int64(response.ExpiresAt.Sub(response.IssuedAt).Seconds())
	}
	if response.TTLSeconds > 0 && !response.IssuedAt.IsZero() && response.ExpiresAt.IsZero() {
		response.ExpiresAt = response.IssuedAt.Add(time.Duration(response.TTLSeconds) * time.Second)
	}
	return response, nil
}

func normalizeRequest(request Request) (Request, error) {
	toolName := strings.ToLower(strings.TrimSpace(request.ToolName))
	if toolName == "" {
		return Request{}, fmt.Errorf("tool_name is required")
	}
	identity := strings.TrimSpace(request.Identity)
	if identity == "" {
		return Request{}, fmt.Errorf("identity is required")
	}
	n := Request{
		ToolName:         toolName,
		Identity:         identity,
		Workspace:        strings.TrimSpace(request.Workspace),
		SessionID:        strings.TrimSpace(request.SessionID),
		RequestID:        strings.TrimSpace(request.RequestID),
		RunID:            strings.TrimSpace(request.RunID),
		JobID:            strings.TrimSpace(request.JobID),
		Reference:        strings.TrimSpace(request.Reference),
		Scope:            normalizeScope(request.Scope),
		TargetBinding:    strings.TrimSpace(request.TargetBinding),
		ContractFamilyID: strings.TrimSpace(request.ContractFamilyID), ContractID: strings.TrimSpace(request.ContractID), ContractRevision: request.ContractRevision, ProposalDigest: strings.ToLower(strings.TrimSpace(request.ProposalDigest)), ActivationDigest: strings.ToLower(strings.TrimSpace(request.ActivationDigest)), PolicyDigest: strings.ToLower(strings.TrimSpace(request.PolicyDigest)), ApprovalTokenDigest: strings.ToLower(strings.TrimSpace(request.ApprovalTokenDigest)), DelegationDigest: strings.ToLower(strings.TrimSpace(request.DelegationDigest)), ExpectedOutcome: strings.TrimSpace(request.ExpectedOutcome), EffectScope: normalizeScope(request.EffectScope), ContainmentScope: normalizeScope(request.ContainmentScope),
	}
	bound := n.ContractFamilyID != "" || n.ContractID != "" || n.ContractRevision > 0 || n.ProposalDigest != "" || n.ActivationDigest != "" || n.PolicyDigest != "" || n.ApprovalTokenDigest != "" || n.DelegationDigest != "" || n.ExpectedOutcome != "" || len(n.EffectScope) > 0 || len(n.ContainmentScope) > 0
	if bound {
		if n.ContractFamilyID == "" || n.ContractID == "" || n.ContractRevision < 1 || !validRequestDigest(n.ProposalDigest) || !validRequestDigest(n.ActivationDigest) || !validRequestDigest(n.PolicyDigest) || n.ExpectedOutcome == "" || len(n.EffectScope) == 0 || len(n.ContainmentScope) == 0 {
			return Request{}, fmt.Errorf("contract binding incomplete")
		}
		for _, v := range [][]string{n.EffectScope, n.ContainmentScope} {
			if len(v) == 0 || hasDuplicateStrings(v) {
				return Request{}, fmt.Errorf("contract scope invalid")
			}
		}
	}
	return n, nil
}
func validRequestDigest(v string) bool { return len(v) == 71 && strings.HasPrefix(v, "sha256:") }
func hasDuplicateStrings(v []string) bool {
	m := map[string]bool{}
	for _, x := range v {
		if m[x] {
			return true
		}
		m[x] = true
	}
	return false
}
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	return strings.Join(normalizeScope(a), "\x00") == strings.Join(normalizeScope(b), "\x00")
}

func RequestDigest(request Request) (string, error) {
	normalized, err := normalizeRequest(request)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal broker request: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest broker request: %w", err)
	}
	return digest, nil
}

func normalizeScope(scope []string) []string {
	if len(scope) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(scope))
	values := make([]string, 0, len(scope))
	for _, value := range scope {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
	}
	return values
}
