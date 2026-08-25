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

	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
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
	ApprovalCodeTargetsExceeded     = "approval_token_max_targets_exceeded"
	ApprovalCodeOpsExceeded         = "approval_token_max_ops_exceeded"
)

type MintApprovalTokenOptions struct {
	ProducerVersion                                                            string
	ApproverIdentity                                                           string
	ReasonCode                                                                 string
	IntentDigest                                                               string
	PolicyDigest                                                               string
	DelegationBindingDigest                                                    string
	Scope                                                                      []string
	MaxTargets                                                                 int
	MaxOps                                                                     int
	TTL                                                                        time.Duration
	Now                                                                        time.Time
	SigningPrivateKey                                                          ed25519.PrivateKey
	TokenPath                                                                  string
	ContractFamilyID                                                           string
	ContractID                                                                 string
	ContractRevision                                                           int
	ProposalDigest                                                             string
	ActivationDigest                                                           string
	TargetScope, EnvironmentScope, OutcomeScope, EffectScope, ContainmentScope []string
}

type MintApprovalTokenResult struct {
	Token     schemagate.ApprovalToken
	TokenPath string
}

type ApprovalValidationOptions struct {
	Now                                                                                                                time.Time
	ExpectedIntentDigest                                                                                               string
	ExpectedPolicyDigest                                                                                               string
	ExpectedDelegationBindingDigest                                                                                    string
	RequiredScope                                                                                                      []string
	TargetCount                                                                                                        int
	OperationCount                                                                                                     int
	ExpectedContractFamilyID                                                                                           string
	ExpectedContractID                                                                                                 string
	ExpectedContractRevision                                                                                           int
	ExpectedProposalDigest                                                                                             string
	ExpectedActivationDigest                                                                                           string
	ExpectedTargetScope, ExpectedEnvironmentScope, ExpectedOutcomeScope, ExpectedEffectScope, ExpectedContainmentScope []string
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
	if opts.MaxTargets < 0 {
		return MintApprovalTokenResult{}, fmt.Errorf("max_targets must be >= 0")
	}
	if opts.MaxOps < 0 {
		return MintApprovalTokenResult{}, fmt.Errorf("max_ops must be >= 0")
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
		TokenID:                 computeApprovalTokenID(intentDigest, policyDigest, approver, reasonCode, scope, opts.MaxTargets, opts.MaxOps, createdAt.Add(opts.TTL)),
		ApproverIdentity:        approver,
		ReasonCode:              reasonCode,
		IntentDigest:            intentDigest,
		PolicyDigest:            policyDigest,
		DelegationBindingDigest: delegationBindingDigest,
		Scope:                   scope,
		MaxTargets:              opts.MaxTargets,
		MaxOps:                  opts.MaxOps,
		ExpiresAt:               createdAt.Add(opts.TTL),
		ContractFamilyID:        opts.ContractFamilyID, ContractID: opts.ContractID, ContractRevision: opts.ContractRevision, ProposalDigest: opts.ProposalDigest, ActivationDigest: opts.ActivationDigest,
		TargetScope: normalizeStringListLower(opts.TargetScope), EnvironmentScope: normalizeStringListLower(opts.EnvironmentScope), OutcomeScope: normalizeStringListLower(opts.OutcomeScope), EffectScope: normalizeStringListLower(opts.EffectScope), ContainmentScope: normalizeStringListLower(opts.ContainmentScope),
	}
	var normalizeErr error
	token, normalizeErr = normalizeApprovalToken(token)
	if normalizeErr != nil {
		return MintApprovalTokenResult{}, normalizeErr
	}

	signable := token
	signable.Signature = nil
	if isContractBoundApproval(token) {
		signable.TokenID = ""
		raw, _ := json.Marshal(signable)
		digest, digestErr := jcs.DigestJCS(raw)
		if digestErr != nil {
			return MintApprovalTokenResult{}, fmt.Errorf("digest approval token: %w", digestErr)
		}
		token.TokenID = strings.TrimPrefix(digest, "sha256:")[:24]
		signable = token
		signable.Signature = nil
	}
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
	if err := safeTokenPath(path); err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write approval token: %w", err)
	}
	return nil
}

func ReadApprovalToken(path string) (schemagate.ApprovalToken, error) {
	// #nosec G304 -- approval token path is explicit local user input.
	content, err := readTokenFile(path)
	if err != nil {
		return schemagate.ApprovalToken{}, fmt.Errorf("read approval token: %w", err)
	}
	var token schemagate.ApprovalToken
	if err := strictTokenDecode(content, &token); err != nil {
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
	expectsBound := opts.ExpectedContractFamilyID != "" || opts.ExpectedContractID != "" || opts.ExpectedContractRevision > 0 || opts.ExpectedProposalDigest != "" || opts.ExpectedActivationDigest != "" || len(opts.ExpectedTargetScope) > 0 || len(opts.ExpectedEnvironmentScope) > 0 || len(opts.ExpectedOutcomeScope) > 0 || len(opts.ExpectedEffectScope) > 0 || len(opts.ExpectedContainmentScope) > 0
	if expectsBound && (!isContractBoundApproval(normalized) || normalized.MaxOps <= 0 || normalized.MaxTargets <= 0) {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("contract-bound token required")}
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
	if opts.ExpectedContractFamilyID != "" && normalized.ContractFamilyID != opts.ExpectedContractFamilyID {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("contract family mismatch")}
	}
	if opts.ExpectedContractID != "" && normalized.ContractID != opts.ExpectedContractID {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("contract id mismatch")}
	}
	if opts.ExpectedContractRevision > 0 && normalized.ContractRevision != opts.ExpectedContractRevision {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("contract revision mismatch")}
	}
	if opts.ExpectedProposalDigest != "" && normalized.ProposalDigest != opts.ExpectedProposalDigest {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("proposal digest mismatch")}
	}
	if opts.ExpectedActivationDigest != "" && normalized.ActivationDigest != opts.ExpectedActivationDigest {
		return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("activation digest mismatch")}
	}
	for _, pair := range []struct{ want, got []string }{{opts.ExpectedTargetScope, normalized.TargetScope}, {opts.ExpectedEnvironmentScope, normalized.EnvironmentScope}, {opts.ExpectedOutcomeScope, normalized.OutcomeScope}, {opts.ExpectedEffectScope, normalized.EffectScope}, {opts.ExpectedContainmentScope, normalized.ContainmentScope}} {
		if len(pair.want) > 0 && !matchesApprovalScope(pair.want, pair.got) {
			return &ApprovalTokenError{Code: ApprovalCodeScopeMismatch, Err: fmt.Errorf("contract axis scope mismatch")}
		}
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
	if normalized.MaxTargets > 0 && opts.TargetCount > normalized.MaxTargets {
		return &ApprovalTokenError{Code: ApprovalCodeTargetsExceeded, Err: fmt.Errorf("target count exceeds token max_targets")}
	}
	if normalized.MaxOps > 0 && opts.OperationCount > normalized.MaxOps {
		return &ApprovalTokenError{Code: ApprovalCodeOpsExceeded, Err: fmt.Errorf("operation count exceeds token max_ops")}
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
	phase := strings.ToLower(strings.TrimSpace(normalizedIntent.Context.Phase))
	if phase == "" {
		phase = "apply"
	}
	scope := []string{fmt.Sprintf("tool:%s", normalizedIntent.ToolName)}
	if phase == "apply" && IntentContainsDestructiveTarget(normalizedIntent.Targets) {
		scope = append(scope, "phase:apply", "destructive:apply")
	}
	scope = normalizeStringListLower(scope)
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
	if normalized.MaxTargets < 0 {
		return schemagate.ApprovalToken{}, fmt.Errorf("max_targets must be >= 0")
	}
	if normalized.MaxOps < 0 {
		return schemagate.ApprovalToken{}, fmt.Errorf("max_ops must be >= 0")
	}
	normalized.ContractFamilyID = strings.TrimSpace(normalized.ContractFamilyID)
	normalized.ContractID = strings.TrimSpace(normalized.ContractID)
	normalized.ProposalDigest = strings.ToLower(strings.TrimSpace(normalized.ProposalDigest))
	normalized.ActivationDigest = strings.ToLower(strings.TrimSpace(normalized.ActivationDigest))
	normalized.TargetScope = normalizeStringListLower(normalized.TargetScope)
	normalized.EnvironmentScope = normalizeStringListLower(normalized.EnvironmentScope)
	normalized.OutcomeScope = normalizeStringListLower(normalized.OutcomeScope)
	normalized.EffectScope = normalizeStringListLower(normalized.EffectScope)
	normalized.ContainmentScope = normalizeStringListLower(normalized.ContainmentScope)
	bound := normalized.ContractFamilyID != "" || normalized.ContractID != "" || normalized.ContractRevision > 0 || normalized.ProposalDigest != "" || normalized.ActivationDigest != "" || len(normalized.TargetScope) > 0 || len(normalized.EnvironmentScope) > 0 || len(normalized.OutcomeScope) > 0 || len(normalized.EffectScope) > 0 || len(normalized.ContainmentScope) > 0
	if bound {
		if normalized.ContractFamilyID == "" || normalized.ContractID == "" || normalized.ContractRevision < 1 || !isDigestHex(normalized.ProposalDigest) || !isDigestHex(normalized.ActivationDigest) || normalized.MaxOps <= 0 || normalized.MaxTargets <= 0 || len(normalized.TargetScope) == 0 || len(normalized.EnvironmentScope) == 0 || len(normalized.OutcomeScope) == 0 || len(normalized.EffectScope) == 0 || len(normalized.ContainmentScope) == 0 {
			return schemagate.ApprovalToken{}, fmt.Errorf("incomplete contract binding")
		}
		for _, v := range [][]string{normalized.TargetScope, normalized.EnvironmentScope, normalized.OutcomeScope, normalized.EffectScope, normalized.ContainmentScope} {
			if hasDuplicate(v) {
				return schemagate.ApprovalToken{}, fmt.Errorf("duplicate contract scope")
			}
			for _, x := range v {
				if x == "*" || x == "" {
					return schemagate.ApprovalToken{}, fmt.Errorf("invalid contract scope")
				}
			}
		}
	}
	if normalized.CreatedAt.IsZero() {
		return schemagate.ApprovalToken{}, fmt.Errorf("created_at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return schemagate.ApprovalToken{}, fmt.Errorf("expires_at is required")
	}
	return normalized, nil
}
func hasDuplicate(v []string) bool {
	m := map[string]bool{}
	for _, x := range v {
		if m[x] {
			return true
		}
		m[x] = true
	}
	return false
}
func isContractBoundApproval(t schemagate.ApprovalToken) bool {
	return t.ContractFamilyID != "" || t.ContractID != "" || t.ContractRevision > 0 || t.ProposalDigest != "" || t.ActivationDigest != "" || len(t.TargetScope) > 0 || len(t.EnvironmentScope) > 0 || len(t.OutcomeScope) > 0 || len(t.EffectScope) > 0 || len(t.ContainmentScope) > 0
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

func computeApprovalTokenID(intentDigest, policyDigest, approver, reasonCode string, scope []string, maxTargets int, maxOps int, expiresAt time.Time) string {
	raw := intentDigest + ":" + policyDigest + ":" + approver + ":" + reasonCode + ":" + strings.Join(scope, ",") +
		fmt.Sprintf(":%d:%d:", maxTargets, maxOps) + expiresAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}
