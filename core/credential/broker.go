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
	ToolName      string
	Identity      string
	Workspace     string
	SessionID     string
	RequestID     string
	RunID         string
	JobID         string
	Reference     string
	Scope         []string
	TargetBinding string
}

type Response struct {
	IssuedBy      string    `json:"issued_by"`
	Source        string    `json:"source,omitempty"`
	AccessType    string    `json:"access_type,omitempty"`
	Issuer        string    `json:"issuer,omitempty"`
	Subject       string    `json:"subject,omitempty"`
	Owner         string    `json:"owner,omitempty"`
	Scope         []string  `json:"scope,omitempty"`
	CredentialRef string    `json:"credential_ref"`
	TargetBinding string    `json:"target_binding,omitempty"`
	RunBinding    string    `json:"run_binding,omitempty"`
	JobBinding    string    `json:"job_binding,omitempty"`
	RequestDigest string    `json:"request_digest,omitempty"`
	IssuedAt      time.Time `json:"issued_at,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	TTLSeconds    int64     `json:"ttl_seconds,omitempty"`
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
	if response.IssuedBy == "" {
		response.IssuedBy = broker.Name()
	}
	if response.Source == "" {
		response.Source = broker.Name()
	}
	if len(response.Scope) == 0 && len(normalized.Scope) > 0 {
		response.Scope = append([]string(nil), normalized.Scope...)
	}
	if response.TargetBinding == "" {
		response.TargetBinding = normalized.TargetBinding
	}
	if response.RunBinding == "" {
		response.RunBinding = normalized.RunID
	}
	if response.JobBinding == "" {
		response.JobBinding = normalized.JobID
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
	return Request{
		ToolName:      toolName,
		Identity:      identity,
		Workspace:     strings.TrimSpace(request.Workspace),
		SessionID:     strings.TrimSpace(request.SessionID),
		RequestID:     strings.TrimSpace(request.RequestID),
		RunID:         strings.TrimSpace(request.RunID),
		JobID:         strings.TrimSpace(request.JobID),
		Reference:     strings.TrimSpace(request.Reference),
		Scope:         normalizeScope(request.Scope),
		TargetBinding: strings.TrimSpace(request.TargetBinding),
	}, nil
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
