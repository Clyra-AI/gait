package credential

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrCredentialUnavailable = errors.New("credential unavailable")

type Request struct {
	ToolName  string
	Identity  string
	Workspace string
	SessionID string
	RequestID string
	Reference string
	Scope     []string
}

type Response struct {
	IssuedBy      string    `json:"issued_by"`
	CredentialRef string    `json:"credential_ref"`
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
	response, err := broker.Issue(normalized)
	if err != nil {
		return Response{}, err
	}
	response.IssuedBy = strings.TrimSpace(response.IssuedBy)
	response.CredentialRef = strings.TrimSpace(response.CredentialRef)
	if response.IssuedBy == "" {
		response.IssuedBy = broker.Name()
	}
	if response.CredentialRef == "" {
		return Response{}, fmt.Errorf("broker returned empty credential reference")
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
		ToolName:  toolName,
		Identity:  identity,
		Workspace: strings.TrimSpace(request.Workspace),
		SessionID: strings.TrimSpace(request.SessionID),
		RequestID: strings.TrimSpace(request.RequestID),
		Reference: strings.TrimSpace(request.Reference),
		Scope:     normalizeScope(request.Scope),
	}, nil
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
