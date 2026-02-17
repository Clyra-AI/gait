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

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	"github.com/Clyra-AI/gait/core/sign"
)

const (
	// #nosec G101 -- schema identifiers are fixed protocol constants, not credentials.
	approvalTokenSchemaID = "gait.gate.approval_token"
	approvalTokenSchemaV1 = "1.0.0"

	ApprovalReasonMissingToken      = "approval_token_missing"
	ApprovalReasonGranted           = "approval_granted"
	ApprovalReasonChainInsufficient = "approval_chain_insufficient"
	ApprovalReasonDistinctApprovers = "approval_distinct_approvers_required"
	ApprovalCodeSchemaInvalid       = "approval_token_invalid"
	ApprovalCodeSignatureMiss       = "approval_token_signature_missing"
	ApprovalCodeSignatureFailed     = "approval_token_signature_invalid"
	ApprovalCodeExpired             = "approval_token_expired"
	ApprovalCodeIntentMismatch      = "approval_token_intent_mismatch"
	ApprovalCodePolicyMismatch      = "approval_token_policy_mismatch"
	ApprovalCodeDelegationMismatch  = "approval_token_delegation_binding_mismatch"
	ApprovalCodeScopeMismatch       = "approval_token_scope_mismatch"
)

type MintApprovalTokenOptions struct {
	ProducerVersion         string
	ApproverIdentity        string
	ReasonCode              string
	IntentDigest            string
	PolicyDigest            string
	DelegationBindingDigest string
	Scope                   []string
	TTL                     time.Duration
	Now                     time.Time
	SigningPrivateKey       ed25519.PrivateKey
	TokenPath               string
}

type MintApprovalTokenResult struct {
	Token     schemagate.ApprovalToken
	TokenPath string
}

type ApprovalValidationOptions struct {
	Now                             time.Time
	ExpectedIntentDigest            string
	ExpectedPolicyDigest            string
	ExpectedDelegationBindingDigest string
	RequiredScope                   []string
}

type ApprovalTokenError struct {
	Code string
	Err  error
}

func (e *ApprovalTokenError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *ApprovalTokenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func MintApprovalToken(opts MintApprovalTokenOptions) (MintApprovalTokenResult, error) {
	if len(opts.SigningPrivateKey) == 0 {
		return MintApprovalTokenResult{}, fmt.Errorf("signing private key is required")
	}
	if opts.TTL <= 0 {
		return MintApprovalTokenResult{}, fmt.Errorf("ttl must be greater than 0")
	}
	intentDigest := strings.ToLower(strings.TrimSpace(opts.IntentDigest))
	if !isDigestHex(intentDigest) {
		return MintApprovalTokenResult{}, fmt.Errorf("intent_digest must be sha256 hex")
	}
	policyDigest := strings.ToLower(strings.TrimSpace(opts.PolicyDigest))
	if !isDigestHex(policyDigest) {
		return MintApprovalTokenResult{}, fmt.Errorf("policy_digest must be sha256 hex")
	}
	delegationBindingDigest := strings.ToLower(strings.TrimSpace(opts.DelegationBindingDigest))
	if delegationBindingDigest != "" && !isDigestHex(delegationBindingDigest) {
		return MintApprovalTokenResult{}, fmt.Errorf("delegation_binding_digest must be sha256 hex when set")
	}
	approver := strings.TrimSpace(opts.ApproverIdentity)
	if approver == "" {
		return MintApprovalTokenResult{}, fmt.Errorf("approver identity is required")
	}
	reasonCode := strings.TrimSpace(opts.ReasonCode)
	if reasonCode == "" {
		return MintApprovalTokenResult{}, fmt.Errorf("reason code is required")
	}
	scope := normalizeStringListLower(opts.Scope)
	if len(scope) == 0 {
		return MintApprovalTokenResult{}, fmt.Errorf("scope must include at least one value")
	}

	createdAt := opts.Now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	token := schemagate.ApprovalToken{
		SchemaID:                approvalTokenSchemaID,
		SchemaVersion:           approvalTokenSchemaV1,
		CreatedAt:               createdAt,
		ProducerVersion:         producerVersion,
		TokenID:                 computeApprovalTokenID(intentDigest, policyDigest, approver, reasonCode, scope, createdAt.Add(opts.TTL)),
		ApproverIdentity:        approver,
		ReasonCode:              reasonCode,
		IntentDigest:            intentDigest,
		PolicyDigest:            policyDigest,
		DelegationBindingDigest: delegationBindingDigest,
		Scope:                   scope,
		ExpiresAt:               createdAt.Add(opts.TTL),
	}

	signable := token
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return MintApprovalTokenResult{}, fmt.Errorf("marshal signable approval token: %w", err)
	}
	signature, err := sign.SignJSON(opts.SigningPrivateKey, signableRaw)
	if err != nil {
		return MintApprovalTokenResult{}, fmt.Errorf("sign approval token: %w", err)
	}
	token.Signature = &schemagate.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}

	tokenPath := strings.TrimSpace(opts.TokenPath)
	if tokenPath == "" {
		tokenPath = fmt.Sprintf("approval_%s.json", token.TokenID)
	}
	if err := WriteApprovalToken(tokenPath, token); err != nil {
		return MintApprovalTokenResult{}, err
	}
	return MintApprovalTokenResult{
		Token:     token,
		TokenPath: tokenPath,
	}, nil
}

func WriteApprovalToken(path string, token schemagate.ApprovalToken) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create approval token directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval token: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write approval token: %w", err)
	}
	return nil
}

func ReadApprovalToken(path string) (schemagate.ApprovalToken, error) {
	// #nosec G304 -- approval token path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemagate.ApprovalToken{}, fmt.Errorf("read approval token: %w", err)
	}
	var token schemagate.ApprovalToken
	if err := json.Unmarshal(content, &token); err != nil {
		return schemagate.ApprovalToken{}, fmt.Errorf("parse approval token: %w", err)
	}
	return token, nil
}

func ValidateApprovalToken(token schemagate.ApprovalToken, publicKey ed25519.PublicKey, opts ApprovalValidationOptions) error {
	normalized, err := normalizeApprovalToken(token)
	if err != nil {
		return &ApprovalTokenError{Code: ApprovalCodeSchemaInvalid, Err: err}
	}
	if len(publicKey) == 0 {
		return &ApprovalTokenError{Code: ApprovalCodeSignatureFailed, Err: fmt.Errorf("verification public key is required")}
	}
	if normalized.Signature == nil {
		return &ApprovalTokenError{Code: ApprovalCodeSignatureMiss, Err: fmt.Errorf("signature missing")}
	}

	signable := normalized
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return &ApprovalTokenError{Code: ApprovalCodeSchemaInvalid, Err: fmt.Errorf("marshal signable token: %w", err)}
	}
	ok, err := sign.VerifyJSON(publicKey, sign.Signature{
		Alg:          normalized.Signature.Alg,
		KeyID:        normalized.Signature.KeyID,
		Sig:          normalized.Signature.Sig,
		SignedDigest: normalized.Signature.SignedDigest,
	}, signableRaw)
	if err != nil {
		return &ApprovalTokenError{Code: ApprovalCodeSignatureFailed, Err: err}
	}
	if !ok {
		return &ApprovalTokenError{Code: ApprovalCodeSignatureFailed, Err: fmt.Errorf("signature verification failed")}
	}

	expectedIntent := strings.ToLower(strings.TrimSpace(opts.ExpectedIntentDigest))
	if expectedIntent != "" && normalized.IntentDigest != expectedIntent {
		return &ApprovalTokenError{Code: ApprovalCodeIntentMismatch, Err: fmt.Errorf("intent digest mismatch")}
	}
	expectedPolicy := strings.ToLower(strings.TrimSpace(opts.ExpectedPolicyDigest))
	if expectedPolicy != "" && normalized.PolicyDigest != expectedPolicy {
		return &ApprovalTokenError{Code: ApprovalCodePolicyMismatch, Err: fmt.Errorf("policy digest mismatch")}
	}
	expectedDelegationBindingDigest := strings.ToLower(strings.TrimSpace(opts.ExpectedDelegationBindingDigest))
	if expectedDelegationBindingDigest != "" && normalized.DelegationBindingDigest != expectedDelegationBindingDigest {
		return &ApprovalTokenError{Code: ApprovalCodeDelegationMismatch, Err: fmt.Errorf("delegation binding digest mismatch")}
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !now.Before(normalized.ExpiresAt.UTC()) {
		return &ApprovalTokenError{Code: ApprovalCodeExpired, Err: fmt.Errorf("token expired")}
	}
	requiredScope := normalizeStringListLower(opts.RequiredScope)
	if len(requiredScope) > 0 && !matchesApprovalScope(requiredScope, normalized.Scope) {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("scope mismatch")}
	}
	return nil
}

func ApprovalContext(policy Policy, intent schemagate.IntentRequest) (string, string, []string, error) {
	policyDigest, err := PolicyDigest(policy)
	if err != nil {
		return "", "", nil, err
	}
	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		return "", "", nil, fmt.Errorf("normalize intent: %w", err)
	}
	scope := []string{fmt.Sprintf("tool:%s", normalizedIntent.ToolName)}
	return policyDigest, normalizedIntent.IntentDigest, scope, nil
}

func normalizeApprovalToken(token schemagate.ApprovalToken) (schemagate.ApprovalToken, error) {
	normalized := token
	if normalized.SchemaID == "" {
		normalized.SchemaID = approvalTokenSchemaID
	}
	if normalized.SchemaID != approvalTokenSchemaID {
		return schemagate.ApprovalToken{}, fmt.Errorf("unsupported schema_id: %s", normalized.SchemaID)
	}
	if normalized.SchemaVersion == "" {
		normalized.SchemaVersion = approvalTokenSchemaV1
	}
	if normalized.SchemaVersion != approvalTokenSchemaV1 {
		return schemagate.ApprovalToken{}, fmt.Errorf("unsupported schema_version: %s", normalized.SchemaVersion)
	}
	normalized.TokenID = strings.TrimSpace(normalized.TokenID)
	if normalized.TokenID == "" {
		return schemagate.ApprovalToken{}, fmt.Errorf("token_id is required")
	}
	normalized.ApproverIdentity = strings.TrimSpace(normalized.ApproverIdentity)
	if normalized.ApproverIdentity == "" {
		return schemagate.ApprovalToken{}, fmt.Errorf("approver_identity is required")
	}
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	if normalized.ReasonCode == "" {
		return schemagate.ApprovalToken{}, fmt.Errorf("reason_code is required")
	}
	normalized.IntentDigest = strings.ToLower(strings.TrimSpace(normalized.IntentDigest))
	if !isDigestHex(normalized.IntentDigest) {
		return schemagate.ApprovalToken{}, fmt.Errorf("intent_digest must be sha256 hex")
	}
	normalized.PolicyDigest = strings.ToLower(strings.TrimSpace(normalized.PolicyDigest))
	if !isDigestHex(normalized.PolicyDigest) {
		return schemagate.ApprovalToken{}, fmt.Errorf("policy_digest must be sha256 hex")
	}
	normalized.DelegationBindingDigest = strings.ToLower(strings.TrimSpace(normalized.DelegationBindingDigest))
	if normalized.DelegationBindingDigest != "" && !isDigestHex(normalized.DelegationBindingDigest) {
		return schemagate.ApprovalToken{}, fmt.Errorf("delegation_binding_digest must be sha256 hex when set")
	}
	normalized.Scope = normalizeStringListLower(normalized.Scope)
	if len(normalized.Scope) == 0 {
		return schemagate.ApprovalToken{}, fmt.Errorf("scope is required")
	}
	if normalized.CreatedAt.IsZero() {
		return schemagate.ApprovalToken{}, fmt.Errorf("created_at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return schemagate.ApprovalToken{}, fmt.Errorf("expires_at is required")
	}
	return normalized, nil
}

func matchesApprovalScope(requiredScope []string, tokenScope []string) bool {
	if len(requiredScope) == 0 {
		return true
	}
	tokenSet := make(map[string]struct{}, len(tokenScope))
	for _, scope := range tokenScope {
		tokenSet[scope] = struct{}{}
	}
	if _, ok := tokenSet["*"]; ok {
		return true
	}
	for _, scope := range requiredScope {
		if _, ok := tokenSet[scope]; !ok {
			return false
		}
	}
	return true
}

func isDigestHex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func computeApprovalTokenID(intentDigest, policyDigest, approver, reasonCode string, scope []string, expiresAt time.Time) string {
	raw := intentDigest + ":" + policyDigest + ":" + approver + ":" + reasonCode + ":" + strings.Join(scope, ",") + ":" + expiresAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}
