package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errUnsupportedProviderReceipt = errors.New("unsupported provider receipt")

type providerReceipt struct {
	Provider                 string   `json:"provider,omitempty"`
	Source                   string   `json:"source,omitempty"`
	AccessType               string   `json:"access_type,omitempty"`
	Issuer                   string   `json:"issuer,omitempty"`
	Subject                  string   `json:"subject,omitempty"`
	Owner                    string   `json:"owner,omitempty"`
	Scope                    []string `json:"scope,omitempty"`
	CredentialRef            string   `json:"credential_ref,omitempty"`
	TargetBinding            string   `json:"target_binding,omitempty"`
	RunBinding               string   `json:"run_binding,omitempty"`
	JobBinding               string   `json:"job_binding,omitempty"`
	RequestDigest            string   `json:"request_digest,omitempty"`
	IssuedAt                 string   `json:"issued_at,omitempty"`
	ExpiresAt                string   `json:"expires_at,omitempty"`
	TTLSeconds               int64    `json:"ttl_seconds,omitempty"`
	AssumedRoleARN           string   `json:"assumed_role_arn,omitempty"`
	SessionARN               string   `json:"session_arn,omitempty"`
	Repository               string   `json:"repository,omitempty"`
	WorkflowRef              string   `json:"workflow_ref,omitempty"`
	LeaseID                  string   `json:"lease_id,omitempty"`
	Principal                string   `json:"principal,omitempty"`
	WorkloadIdentityProvider string   `json:"workload_identity_provider,omitempty"`
	ServicePrincipalID       string   `json:"service_principal_id,omitempty"`
	FederatedCredentialID    string   `json:"federated_credential_id,omitempty"`
	BrokerSessionID          string   `json:"broker_session_id,omitempty"`
}

func NormalizeProviderReceipt(provider string, payload []byte, request Request) (Response, error) {
	var receipt providerReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return Response{}, fmt.Errorf("parse provider receipt: %w", err)
	}
	return normalizeProviderReceipt(provider, receipt, request)
}

func normalizeProviderReceipt(provider string, receipt providerReceipt, request Request) (Response, error) {
	resolvedProvider := normalizeProviderName(receipt.Provider)
	if resolvedProvider == "" {
		resolvedProvider = normalizeProviderName(provider)
	}
	if resolvedProvider == "" {
		return Response{}, errUnsupportedProviderReceipt
	}

	if err := rejectProviderSecretLikeFields(receipt); err != nil {
		return Response{}, err
	}

	normalized := Response{
		IssuedBy:      resolvedProvider,
		Source:        resolvedProvider,
		AccessType:    "jit",
		Issuer:        defaultProviderIssuer(resolvedProvider),
		Subject:       strings.TrimSpace(request.Identity),
		Owner:         strings.TrimSpace(request.Identity),
		Scope:         normalizeScope(receipt.Scope),
		CredentialRef: strings.TrimSpace(receipt.CredentialRef),
		TargetBinding: strings.TrimSpace(receipt.TargetBinding),
		RunBinding:    strings.TrimSpace(receipt.RunBinding),
		JobBinding:    strings.TrimSpace(receipt.JobBinding),
		RequestDigest: strings.TrimSpace(receipt.RequestDigest),
		TTLSeconds:    receipt.TTLSeconds,
	}
	if source := strings.ToLower(strings.TrimSpace(receipt.Source)); source != "" {
		normalized.Source = source
	}
	if accessType := strings.ToLower(strings.TrimSpace(receipt.AccessType)); accessType != "" {
		normalized.AccessType = accessType
	}
	if issuer := strings.TrimSpace(receipt.Issuer); issuer != "" {
		normalized.Issuer = issuer
	}
	if subject := strings.TrimSpace(receipt.Subject); subject != "" {
		normalized.Subject = subject
	}
	if owner := strings.TrimSpace(receipt.Owner); owner != "" {
		normalized.Owner = owner
	}
	if issuedAt, err := parseOptionalRFC3339(receipt.IssuedAt); err != nil {
		return Response{}, err
	} else {
		normalized.IssuedAt = issuedAt
	}
	if expiresAt, err := parseOptionalRFC3339(receipt.ExpiresAt); err != nil {
		return Response{}, err
	} else {
		normalized.ExpiresAt = expiresAt
	}
	if len(normalized.Scope) == 0 && len(request.Scope) > 0 {
		normalized.Scope = normalizeScope(request.Scope)
	}
	if normalized.TargetBinding == "" {
		normalized.TargetBinding = strings.TrimSpace(request.TargetBinding)
	}
	if normalized.RunBinding == "" {
		normalized.RunBinding = strings.TrimSpace(request.RunID)
	}
	if normalized.JobBinding == "" {
		normalized.JobBinding = strings.TrimSpace(request.JobID)
	}
	if normalized.RequestDigest == "" {
		requestDigest, err := RequestDigest(request)
		if err != nil {
			return Response{}, err
		}
		normalized.RequestDigest = requestDigest
	}
	if normalized.CredentialRef == "" {
		credentialRef, err := deriveProviderCredentialRef(resolvedProvider, receipt)
		if err != nil {
			return Response{}, err
		}
		normalized.CredentialRef = credentialRef
	}
	if normalized.TTLSeconds == 0 && !normalized.IssuedAt.IsZero() && !normalized.ExpiresAt.IsZero() && normalized.ExpiresAt.After(normalized.IssuedAt) {
		normalized.TTLSeconds = int64(normalized.ExpiresAt.Sub(normalized.IssuedAt).Seconds())
	}
	return normalized, nil
}

func normalizeProviderName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "aws", "aws_sts", "sts":
		return "aws_sts"
	case "github", "github_oidc", "oidc":
		return "github_oidc"
	case "vault", "vault_dynamic":
		return "vault_dynamic"
	case "gcp", "gcp_sts":
		return "gcp_sts"
	case "azure", "azure_federated":
		return "azure_federated"
	case "okta", "cyberark", "okta_cyberark":
		return "okta_cyberark"
	case "command":
		return "command"
	case "file":
		return "file"
	default:
		return ""
	}
}

func defaultProviderIssuer(provider string) string {
	switch provider {
	case "aws_sts":
		return "sts.amazonaws.com"
	case "github_oidc":
		return "token.actions.githubusercontent.com"
	case "vault_dynamic":
		return "vault"
	case "gcp_sts":
		return "sts.googleapis.com"
	case "azure_federated":
		return "login.microsoftonline.com"
	case "okta_cyberark":
		return "okta-cyberark"
	case "command":
		return "command"
	case "file":
		return "file"
	default:
		return provider
	}
}

func deriveProviderCredentialRef(provider string, receipt providerReceipt) (string, error) {
	switch provider {
	case "aws_sts":
		if value := strings.TrimSpace(receipt.AssumedRoleARN); value != "" {
			return "aws_sts:" + value, nil
		}
		if value := strings.TrimSpace(receipt.SessionARN); value != "" {
			return "aws_sts:" + value, nil
		}
		return "", fmt.Errorf("aws_sts provider receipt requires assumed_role_arn or session_arn when credential_ref is omitted")
	case "github_oidc":
		repository := strings.TrimSpace(receipt.Repository)
		if repository == "" {
			return "", fmt.Errorf("github_oidc provider receipt requires repository when credential_ref is omitted")
		}
		workflowRef := strings.TrimSpace(receipt.WorkflowRef)
		if workflowRef == "" {
			return "github_oidc:" + repository, nil
		}
		return "github_oidc:" + repository + ":" + workflowRef, nil
	case "vault_dynamic":
		if value := strings.TrimSpace(receipt.LeaseID); value != "" {
			return "vault_dynamic:" + value, nil
		}
		return "", fmt.Errorf("vault_dynamic provider receipt requires lease_id when credential_ref is omitted")
	case "gcp_sts":
		if value := strings.TrimSpace(receipt.Principal); value != "" {
			return "gcp_sts:" + value, nil
		}
		if value := strings.TrimSpace(receipt.WorkloadIdentityProvider); value != "" {
			return "gcp_sts:" + value, nil
		}
		return "", fmt.Errorf("gcp_sts provider receipt requires principal or workload_identity_provider when credential_ref is omitted")
	case "azure_federated":
		servicePrincipalID := strings.TrimSpace(receipt.ServicePrincipalID)
		if servicePrincipalID == "" {
			return "", fmt.Errorf("azure_federated provider receipt requires service_principal_id when credential_ref is omitted")
		}
		federatedCredentialID := strings.TrimSpace(receipt.FederatedCredentialID)
		if federatedCredentialID == "" {
			return "azure_federated:" + servicePrincipalID, nil
		}
		return "azure_federated:" + servicePrincipalID + ":" + federatedCredentialID, nil
	case "okta_cyberark":
		if value := strings.TrimSpace(receipt.BrokerSessionID); value != "" {
			return "okta_cyberark:" + value, nil
		}
		return "", fmt.Errorf("okta_cyberark provider receipt requires broker_session_id when credential_ref is omitted")
	case "command", "file":
		return "", fmt.Errorf("%s provider receipt requires credential_ref", provider)
	default:
		return "", fmt.Errorf("unsupported provider receipt: %s", provider)
	}
}

func parseOptionalRFC3339(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("provider receipt timestamp must be RFC3339: %w", err)
	}
	return parsed.UTC(), nil
}

func rejectProviderSecretLikeFields(receipt providerReceipt) error {
	candidates := map[string]string{
		"assumed_role_arn":           strings.TrimSpace(receipt.AssumedRoleARN),
		"session_arn":                strings.TrimSpace(receipt.SessionARN),
		"repository":                 strings.TrimSpace(receipt.Repository),
		"workflow_ref":               strings.TrimSpace(receipt.WorkflowRef),
		"lease_id":                   strings.TrimSpace(receipt.LeaseID),
		"principal":                  strings.TrimSpace(receipt.Principal),
		"workload_identity_provider": strings.TrimSpace(receipt.WorkloadIdentityProvider),
		"service_principal_id":       strings.TrimSpace(receipt.ServicePrincipalID),
		"federated_credential_id":    strings.TrimSpace(receipt.FederatedCredentialID),
		"broker_session_id":          strings.TrimSpace(receipt.BrokerSessionID),
		"credential_ref":             strings.TrimSpace(receipt.CredentialRef),
		"issuer":                     strings.TrimSpace(receipt.Issuer),
		"source":                     strings.TrimSpace(receipt.Source),
		"subject":                    strings.TrimSpace(receipt.Subject),
		"owner":                      strings.TrimSpace(receipt.Owner),
	}
	for field, value := range candidates {
		if value == "" {
			continue
		}
		if strings.Contains(strings.ToLower(field), "credential") && looksLikeSecretReceiptValue(value) {
			return fmt.Errorf("%s must not contain raw credential material", field)
		}
		if (strings.Contains(strings.ToLower(field), "session") || strings.Contains(strings.ToLower(field), "lease")) && looksLikeSecretReceiptValue(value) {
			return fmt.Errorf("%s must not contain raw credential material", field)
		}
	}
	return nil
}

func looksLikeSecretReceiptValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"ref:", "stub:", "cmd:", "env:", "aws_sts:", "github_oidc:", "vault_dynamic:", "gcp_sts:", "azure_federated:", "okta_cyberark:", "file:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	if strings.HasPrefix(trimmed, "ghp_") ||
		strings.HasPrefix(trimmed, "gho_") ||
		strings.HasPrefix(trimmed, "github_pat_") ||
		strings.HasPrefix(trimmed, "sk-") ||
		strings.HasPrefix(trimmed, "AKIA") ||
		strings.HasPrefix(trimmed, "ASIA") {
		return true
	}
	return false
}
