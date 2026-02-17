package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	"github.com/Clyra-AI/gait/core/sign"
)

const (
	// #nosec G101 -- schema identifiers are fixed protocol constants, not credentials.
	delegationTokenSchemaID = "gait.gate.delegation_token"
	delegationTokenSchemaV1 = "1.0.0"

	DelegationCodeSchemaInvalid   = "delegation_token_invalid"
	DelegationCodeSignatureMiss   = "delegation_token_signature_missing"
	DelegationCodeSignatureFailed = "delegation_token_signature_invalid"
	DelegationCodeExpired         = "delegation_token_expired"
	DelegationCodeDelegatorMis    = "delegation_token_delegator_mismatch"
	DelegationCodeDelegateMis     = "delegation_token_delegate_mismatch"
	DelegationCodeScopeMismatch   = "delegation_token_scope_mismatch"
	DelegationCodeIntentMismatch  = "delegation_token_intent_mismatch"
	DelegationCodePolicyMismatch  = "delegation_token_policy_mismatch"
)

type MintDelegationTokenOptions struct {
	ProducerVersion   string
	DelegatorIdentity string
	DelegateIdentity  string
	Scope             []string
	ScopeClass        string
	IntentDigest      string
	PolicyDigest      string
	TTL               time.Duration
	Now               time.Time
	SigningPrivateKey ed25519.PrivateKey
	TokenPath         string
}

type MintDelegationTokenResult struct {
	Token     schemagate.DelegationToken
	TokenPath string
}

type DelegationValidationOptions struct {
	Now                  time.Time
	ExpectedDelegator    string
	ExpectedDelegate     string
	RequiredScope        []string
	ExpectedIntentDigest string
	ExpectedPolicyDigest string
}

type DelegationTokenError struct {
	Code string
	Err  error
}

func (e *DelegationTokenError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *DelegationTokenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func MintDelegationToken(opts MintDelegationTokenOptions) (MintDelegationTokenResult, error) {
	if len(opts.SigningPrivateKey) == 0 {
		return MintDelegationTokenResult{}, fmt.Errorf("signing private key is required")
	}
	if opts.TTL <= 0 {
		return MintDelegationTokenResult{}, fmt.Errorf("ttl must be greater than 0")
	}
	delegator := strings.TrimSpace(opts.DelegatorIdentity)
	delegate := strings.TrimSpace(opts.DelegateIdentity)
	if delegator == "" || delegate == "" {
		return MintDelegationTokenResult{}, fmt.Errorf("delegator and delegate identities are required")
	}
	scope := normalizeStringListLower(opts.Scope)
	if len(scope) == 0 {
		return MintDelegationTokenResult{}, fmt.Errorf("scope must include at least one value")
	}
	intentDigest := strings.ToLower(strings.TrimSpace(opts.IntentDigest))
	if intentDigest != "" && !isDigestHex(intentDigest) {
		return MintDelegationTokenResult{}, fmt.Errorf("intent_digest must be sha256 hex when set")
	}
	policyDigest := strings.ToLower(strings.TrimSpace(opts.PolicyDigest))
	if policyDigest != "" && !isDigestHex(policyDigest) {
		return MintDelegationTokenResult{}, fmt.Errorf("policy_digest must be sha256 hex when set")
	}

	createdAt := opts.Now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	scopeClass := strings.ToLower(strings.TrimSpace(opts.ScopeClass))
	expiresAt := createdAt.Add(opts.TTL)

	token := schemagate.DelegationToken{
		SchemaID:          delegationTokenSchemaID,
		SchemaVersion:     delegationTokenSchemaV1,
		CreatedAt:         createdAt,
		ProducerVersion:   producerVersion,
		TokenID:           computeDelegationTokenID(delegator, delegate, scope, scopeClass, intentDigest, policyDigest, expiresAt),
		DelegatorIdentity: delegator,
		DelegateIdentity:  delegate,
		Scope:             scope,
		ScopeClass:        scopeClass,
		IntentDigest:      intentDigest,
		PolicyDigest:      policyDigest,
		ExpiresAt:         expiresAt,
	}

	signable := token
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return MintDelegationTokenResult{}, fmt.Errorf("marshal signable delegation token: %w", err)
	}
	signature, err := sign.SignJSON(opts.SigningPrivateKey, signableRaw)
	if err != nil {
		return MintDelegationTokenResult{}, fmt.Errorf("sign delegation token: %w", err)
	}
	token.Signature = &schemagate.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}

	tokenPath := strings.TrimSpace(opts.TokenPath)
	if tokenPath == "" {
		tokenPath = fmt.Sprintf("delegation_%s.json", token.TokenID)
	}
	if err := WriteDelegationToken(tokenPath, token); err != nil {
		return MintDelegationTokenResult{}, err
	}
	return MintDelegationTokenResult{
		Token:     token,
		TokenPath: tokenPath,
	}, nil
}

func WriteDelegationToken(path string, token schemagate.DelegationToken) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create delegation token directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delegation token: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write delegation token: %w", err)
	}
	return nil
}

func ReadDelegationToken(path string) (schemagate.DelegationToken, error) {
	// #nosec G304 -- delegation token path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemagate.DelegationToken{}, fmt.Errorf("read delegation token: %w", err)
	}
	var token schemagate.DelegationToken
	if err := json.Unmarshal(content, &token); err != nil {
		return schemagate.DelegationToken{}, fmt.Errorf("parse delegation token: %w", err)
	}
	return token, nil
}

func ValidateDelegationToken(token schemagate.DelegationToken, publicKey ed25519.PublicKey, opts DelegationValidationOptions) error {
	normalized, err := normalizeDelegationToken(token)
	if err != nil {
		return &DelegationTokenError{Code: DelegationCodeSchemaInvalid, Err: err}
	}
	if len(publicKey) == 0 {
		return &DelegationTokenError{Code: DelegationCodeSignatureFailed, Err: fmt.Errorf("verification public key is required")}
	}
	if normalized.Signature == nil {
		return &DelegationTokenError{Code: DelegationCodeSignatureMiss, Err: fmt.Errorf("signature missing")}
	}

	signable := normalized
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return &DelegationTokenError{Code: DelegationCodeSchemaInvalid, Err: fmt.Errorf("marshal signable token: %w", err)}
	}
	ok, err := sign.VerifyJSON(publicKey, sign.Signature{
		Alg:          normalized.Signature.Alg,
		KeyID:        normalized.Signature.KeyID,
		Sig:          normalized.Signature.Sig,
		SignedDigest: normalized.Signature.SignedDigest,
	}, signableRaw)
	if err != nil {
		return &DelegationTokenError{Code: DelegationCodeSignatureFailed, Err: err}
	}
	if !ok {
		return &DelegationTokenError{Code: DelegationCodeSignatureFailed, Err: fmt.Errorf("signature verification failed")}
	}

	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !now.Before(normalized.ExpiresAt.UTC()) {
		return &DelegationTokenError{Code: DelegationCodeExpired, Err: fmt.Errorf("token expired")}
	}

	expectedDelegator := strings.TrimSpace(opts.ExpectedDelegator)
	if expectedDelegator != "" && normalized.DelegatorIdentity != expectedDelegator {
		return &DelegationTokenError{Code: DelegationCodeDelegatorMis, Err: fmt.Errorf("delegator mismatch")}
	}
	expectedDelegate := strings.TrimSpace(opts.ExpectedDelegate)
	if expectedDelegate != "" && normalized.DelegateIdentity != expectedDelegate {
		return &DelegationTokenError{Code: DelegationCodeDelegateMis, Err: fmt.Errorf("delegate mismatch")}
	}

	expectedIntent := strings.ToLower(strings.TrimSpace(opts.ExpectedIntentDigest))
	if expectedIntent != "" && normalized.IntentDigest != "" && normalized.IntentDigest != expectedIntent {
		return &DelegationTokenError{Code: DelegationCodeIntentMismatch, Err: fmt.Errorf("intent digest mismatch")}
	}
	expectedPolicy := strings.ToLower(strings.TrimSpace(opts.ExpectedPolicyDigest))
	if expectedPolicy != "" && normalized.PolicyDigest != "" && normalized.PolicyDigest != expectedPolicy {
		return &DelegationTokenError{Code: DelegationCodePolicyMismatch, Err: fmt.Errorf("policy digest mismatch")}
	}

	requiredScope := normalizeStringListLower(opts.RequiredScope)
	if len(requiredScope) > 0 && !matchesApprovalScope(requiredScope, normalized.Scope) {
		return &DelegationTokenError{Code: DelegationCodeScopeMismatch, Err: fmt.Errorf("scope mismatch")}
	}
	return nil
}

func normalizeDelegationToken(token schemagate.DelegationToken) (schemagate.DelegationToken, error) {
	normalized := token
	if normalized.SchemaID == "" {
		normalized.SchemaID = delegationTokenSchemaID
	}
	if normalized.SchemaID != delegationTokenSchemaID {
		return schemagate.DelegationToken{}, fmt.Errorf("unsupported schema_id: %s", normalized.SchemaID)
	}
	if normalized.SchemaVersion == "" {
		normalized.SchemaVersion = delegationTokenSchemaV1
	}
	if normalized.SchemaVersion != delegationTokenSchemaV1 {
		return schemagate.DelegationToken{}, fmt.Errorf("unsupported schema_version: %s", normalized.SchemaVersion)
	}
	normalized.TokenID = strings.TrimSpace(normalized.TokenID)
	if normalized.TokenID == "" {
		return schemagate.DelegationToken{}, fmt.Errorf("token_id is required")
	}
	normalized.DelegatorIdentity = strings.TrimSpace(normalized.DelegatorIdentity)
	normalized.DelegateIdentity = strings.TrimSpace(normalized.DelegateIdentity)
	if normalized.DelegatorIdentity == "" || normalized.DelegateIdentity == "" {
		return schemagate.DelegationToken{}, fmt.Errorf("delegator_identity and delegate_identity are required")
	}
	normalized.Scope = normalizeStringListLower(normalized.Scope)
	if len(normalized.Scope) == 0 {
		return schemagate.DelegationToken{}, fmt.Errorf("scope is required")
	}
	normalized.ScopeClass = strings.ToLower(strings.TrimSpace(normalized.ScopeClass))
	normalized.IntentDigest = strings.ToLower(strings.TrimSpace(normalized.IntentDigest))
	if normalized.IntentDigest != "" && !isDigestHex(normalized.IntentDigest) {
		return schemagate.DelegationToken{}, fmt.Errorf("intent_digest must be sha256 hex when set")
	}
	normalized.PolicyDigest = strings.ToLower(strings.TrimSpace(normalized.PolicyDigest))
	if normalized.PolicyDigest != "" && !isDigestHex(normalized.PolicyDigest) {
		return schemagate.DelegationToken{}, fmt.Errorf("policy_digest must be sha256 hex when set")
	}
	if normalized.CreatedAt.IsZero() {
		return schemagate.DelegationToken{}, fmt.Errorf("created_at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return schemagate.DelegationToken{}, fmt.Errorf("expires_at is required")
	}
	return normalized, nil
}

func computeDelegationTokenID(delegator, delegate string, scope []string, scopeClass, intentDigest, policyDigest string, expiresAt time.Time) string {
	raw := strings.Join([]string{
		delegator,
		delegate,
		strings.Join(scope, ","),
		scopeClass,
		intentDigest,
		policyDigest,
		expiresAt.UTC().Format(time.RFC3339Nano),
	}, ":")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}

func DelegationDigest(delegation schemagate.IntentDelegation) (string, error) {
	raw, err := json.Marshal(delegation)
	if err != nil {
		return "", fmt.Errorf("marshal delegation: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest delegation: %w", err)
	}
	return digest, nil
}

func DelegationBindingDigest(intent schemagate.IntentRequest) (string, error) {
	normalized, err := NormalizeIntent(intent)
	if err != nil {
		return "", fmt.Errorf("normalize intent: %w", err)
	}
	if normalized.Delegation == nil {
		return "", nil
	}
	return DelegationDigest(*normalized.Delegation)
}
