package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	DelegationCodeChainMismatch   = "delegation_token_chain_mismatch"
)

type MintDelegationTokenOptions struct {
	ProducerVersion                                                                string
	DelegatorIdentity                                                              string
	DelegateIdentity                                                               string
	Scope                                                                          []string
	ScopeClass                                                                     string
	IntentDigest                                                                   string
	PolicyDigest                                                                   string
	TTL                                                                            time.Duration
	Now                                                                            time.Time
	SigningPrivateKey                                                              ed25519.PrivateKey
	TokenPath                                                                      string
	ActionClasses, TargetScope, EnvironmentScope, DataClasses, NetworkDestinations []string
	MaxOperations, MaxTargets, MaxDescendantDepth                                  int
	ContractDigest                                                                 string
	ParentTokenID, ParentTokenDigest, OriginAuthorityDigest                        string
	Depth                                                                          int
}

type MintDelegationTokenResult struct {
	Token     schemagate.DelegationToken
	TokenPath string
}

type DelegationValidationOptions struct {
	Now                                                                                                                    time.Time
	ExpectedDelegator                                                                                                      string
	ExpectedDelegate                                                                                                       string
	RequiredScope                                                                                                          []string
	ExpectedIntentDigest                                                                                                   string
	ExpectedPolicyDigest                                                                                                   string
	ExpectedContractDigest                                                                                                 string
	RequiredActionClasses, RequiredTargetScope, RequiredEnvironmentScope, RequiredDataClasses, RequiredNetworkDestinations []string
	OperationCount, TargetCount, DescendantDepth                                                                           int
	RequireExactBindings                                                                                                   bool
}

type DelegationTokenError struct {
	Code string
	Err  error
}

type DelegationChainValidationOptions struct {
	Now                  time.Time
	RequiredScope        []string
	ExpectedIntentDigest string
	ExpectedPolicyDigest string
	RequireExactBindings bool
}

type DelegationChainValidationResult struct {
	Complete            bool
	RequiredDelegations int
	ValidDelegations    int
	ValidTokenIDs       []string
	Entries             []schemagate.DelegationAuditEntry
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
		ActionClasses:     normalizeStringListLower(opts.ActionClasses), TargetScope: normalizeStringListLower(opts.TargetScope), EnvironmentScope: normalizeStringListLower(opts.EnvironmentScope), DataClasses: normalizeStringListLower(opts.DataClasses), NetworkDestinations: normalizeStringListLower(opts.NetworkDestinations), MaxOperations: opts.MaxOperations, MaxTargets: opts.MaxTargets, MaxDescendantDepth: opts.MaxDescendantDepth, ContractDigest: opts.ContractDigest,
		ParentTokenID: opts.ParentTokenID, ParentTokenDigest: opts.ParentTokenDigest, OriginAuthorityDigest: opts.OriginAuthorityDigest, Depth: opts.Depth,
	}
	normalizedToken, normalizeErr := normalizeDelegationToken(token)
	if normalizeErr != nil {
		return MintDelegationTokenResult{}, normalizeErr
	}
	token = normalizedToken

	signable := token
	signable.Signature = nil
	if token.ContractDigest != "" {
		signable.TokenID = ""
		raw, _ := json.Marshal(signable)
		digest, digestErr := jcs.DigestJCS(raw)
		if digestErr != nil {
			return MintDelegationTokenResult{}, fmt.Errorf("digest delegation token: %w", digestErr)
		}
		token.TokenID = strings.TrimPrefix(digest, "sha256:")[:24]
		signable = token
		signable.Signature = nil
	}
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
	if err := safeTokenPath(path); err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write delegation token: %w", err)
	}
	return nil
}

func ReadDelegationToken(path string) (schemagate.DelegationToken, error) {
	// #nosec G304 -- delegation token path is explicit local user input.
	content, err := readTokenFile(path)
	if err != nil {
		return schemagate.DelegationToken{}, fmt.Errorf("read delegation token: %w", err)
	}
	var token schemagate.DelegationToken
	if err := strictTokenDecode(content, &token); err != nil {
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
	if normalized.Revoked {
		return &DelegationTokenError{Code: DelegationCodeChainMismatch, Err: fmt.Errorf("delegation revoked")}
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

	expectedIntent := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(opts.ExpectedIntentDigest)), "sha256:")
	if (opts.RequireExactBindings || (expectedIntent != "" && (isContractBoundDelegation(normalized) || normalized.IntentDigest != ""))) && normalized.IntentDigest != expectedIntent {
		return &DelegationTokenError{Code: DelegationCodeIntentMismatch, Err: fmt.Errorf("intent digest mismatch")}
	}
	expectedPolicy := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(opts.ExpectedPolicyDigest)), "sha256:")
	if (opts.RequireExactBindings || (expectedPolicy != "" && (isContractBoundDelegation(normalized) || normalized.PolicyDigest != ""))) && normalized.PolicyDigest != expectedPolicy {
		return &DelegationTokenError{Code: DelegationCodePolicyMismatch, Err: fmt.Errorf("policy digest mismatch")}
	}
	if opts.ExpectedContractDigest != "" && normalized.ContractDigest != strings.ToLower(strings.TrimSpace(opts.ExpectedContractDigest)) {
		return &DelegationTokenError{Code: DelegationCodePolicyMismatch, Err: fmt.Errorf("contract digest mismatch")}
	}
	if opts.OperationCount > 0 && (normalized.MaxOperations <= 0 || opts.OperationCount > normalized.MaxOperations) {
		return &DelegationTokenError{Code: DelegationCodeScopeMismatch, Err: fmt.Errorf("operation limit exceeded")}
	}
	if opts.TargetCount > 0 && (normalized.MaxTargets <= 0 || opts.TargetCount > normalized.MaxTargets) {
		return &DelegationTokenError{Code: DelegationCodeScopeMismatch, Err: fmt.Errorf("target limit exceeded")}
	}
	if opts.DescendantDepth > 0 && (normalized.MaxDescendantDepth <= 0 || opts.DescendantDepth > normalized.MaxDescendantDepth) {
		return &DelegationTokenError{Code: DelegationCodeChainMismatch, Err: fmt.Errorf("descendant depth exceeded")}
	}
	if (len(opts.RequiredActionClasses) > 0 && len(normalized.ActionClasses) == 0) || (len(opts.RequiredTargetScope) > 0 && len(normalized.TargetScope) == 0) || (len(opts.RequiredEnvironmentScope) > 0 && len(normalized.EnvironmentScope) == 0) || (len(opts.RequiredDataClasses) > 0 && len(normalized.DataClasses) == 0) || (len(opts.RequiredNetworkDestinations) > 0 && len(normalized.NetworkDestinations) == 0) || !matchesDelegationScope(opts.RequiredActionClasses, normalized.ActionClasses, "") || !matchesDelegationScope(opts.RequiredTargetScope, normalized.TargetScope, "") || !matchesDelegationScope(opts.RequiredEnvironmentScope, normalized.EnvironmentScope, "") || !matchesDelegationScope(opts.RequiredDataClasses, normalized.DataClasses, "") || !matchesDelegationScope(opts.RequiredNetworkDestinations, normalized.NetworkDestinations, "") {
		return &DelegationTokenError{Code: DelegationCodeScopeMismatch, Err: fmt.Errorf("capability axis expanded")}
	}

	requiredScope := normalizeStringListLower(opts.RequiredScope)
	if len(requiredScope) > 0 && !matchesDelegationScope(requiredScope, normalized.Scope, normalized.ScopeClass) {
		return &DelegationTokenError{Code: DelegationCodeScopeMismatch, Err: fmt.Errorf("scope mismatch")}
	}
	return nil
}

func ValidateDelegationChain(delegation *schemagate.IntentDelegation, tokens []schemagate.DelegationToken, publicKey ed25519.PublicKey, opts DelegationChainValidationOptions) (DelegationChainValidationResult, error) {
	normalizedDelegation, err := normalizeDelegation(delegation)
	if err != nil {
		return DelegationChainValidationResult{}, err
	}
	if normalizedDelegation == nil {
		return DelegationChainValidationResult{}, nil
	}

	requiredLinks := append([]schemagate.DelegationLink(nil), normalizedDelegation.Chain...)
	if len(requiredLinks) == 0 {
		requiredLinks = []schemagate.DelegationLink{{
			DelegateIdentity: normalizedDelegation.RequesterIdentity,
			ScopeClass:       normalizedDelegation.ScopeClass,
		}}
	}

	used := make([]bool, len(tokens))
	matchedLinks := make([]bool, len(requiredLinks))
	entries := make([]schemagate.DelegationAuditEntry, 0, len(tokens)+len(requiredLinks))
	validTokenIDs := make([]string, 0, len(requiredLinks))
	validDelegations := 0
	matchedTokens := make([]schemagate.DelegationToken, 0, len(requiredLinks))
	requiredScope := normalizeStringListLower(opts.RequiredScope)

	for linkIndex, link := range requiredLinks {
		matched := false
		for index, token := range tokens {
			if used[index] {
				continue
			}
			validateErr := ValidateDelegationToken(token, publicKey, DelegationValidationOptions{
				Now:                  opts.Now,
				ExpectedDelegator:    strings.TrimSpace(link.DelegatorIdentity),
				ExpectedDelegate:     strings.TrimSpace(link.DelegateIdentity),
				RequiredScope:        requiredScope,
				ExpectedIntentDigest: opts.ExpectedIntentDigest,
				ExpectedPolicyDigest: opts.ExpectedPolicyDigest,
				RequireExactBindings: opts.RequireExactBindings,
			})
			if validateErr != nil {
				continue
			}
			used[index] = true
			matchedLinks[linkIndex] = true
			matched = true
			validDelegations++
			matchedTokens = append(matchedTokens, token)
			if token.TokenID != "" {
				validTokenIDs = append(validTokenIDs, token.TokenID)
			}
			entries = append(entries, schemagate.DelegationAuditEntry{
				TokenID:           token.TokenID,
				DelegatorIdentity: token.DelegatorIdentity,
				DelegateIdentity:  token.DelegateIdentity,
				Scope:             mergeUniqueSorted(nil, token.Scope),
				ExpiresAt:         token.ExpiresAt.UTC(),
				Valid:             true,
			})
			break
		}
		if matched {
			continue
		}
		entries = append(entries, schemagate.DelegationAuditEntry{
			DelegatorIdentity: strings.TrimSpace(link.DelegatorIdentity),
			DelegateIdentity:  strings.TrimSpace(link.DelegateIdentity),
			Valid:             false,
			ErrorCode:         "delegation_token_missing",
		})
	}
	for i := 1; i < len(matchedTokens); i++ {
		if isContractBoundDelegation(matchedTokens[i-1]) || isContractBoundDelegation(matchedTokens[i]) {
			if !isContractBoundDelegation(matchedTokens[i-1]) || !isContractBoundDelegation(matchedTokens[i]) {
				return DelegationChainValidationResult{Complete: false, RequiredDelegations: len(requiredLinks), ValidDelegations: validDelegations, Entries: entries}, &DelegationTokenError{Code: DelegationCodeChainMismatch, Err: fmt.Errorf("mixed bound and unbound delegation chain")}
			}
			if err := ValidateDelegationNonExpansion(matchedTokens[i-1], matchedTokens[i]); err != nil {
				return DelegationChainValidationResult{Complete: false, RequiredDelegations: len(requiredLinks), ValidDelegations: validDelegations, Entries: entries}, &DelegationTokenError{Code: DelegationCodeChainMismatch, Err: err}
			}
		}
	}

	for index, token := range tokens {
		if used[index] {
			continue
		}
		errorCode := DelegationCodeChainMismatch
		for linkIndex, link := range requiredLinks {
			if matchedLinks[linkIndex] {
				continue
			}
			validateErr := ValidateDelegationToken(token, publicKey, DelegationValidationOptions{
				Now:                  opts.Now,
				ExpectedDelegator:    strings.TrimSpace(link.DelegatorIdentity),
				ExpectedDelegate:     strings.TrimSpace(link.DelegateIdentity),
				RequiredScope:        requiredScope,
				ExpectedIntentDigest: opts.ExpectedIntentDigest,
				ExpectedPolicyDigest: opts.ExpectedPolicyDigest,
			})
			if validateErr == nil {
				errorCode = ""
				break
			}
			var tokenErr *DelegationTokenError
			if errors.As(validateErr, &tokenErr) && tokenErr.Code != "" {
				errorCode = tokenErr.Code
				break
			}
		}
		if errorCode == "" {
			errorCode = DelegationCodeChainMismatch
		}
		entries = append(entries, schemagate.DelegationAuditEntry{
			TokenID:           token.TokenID,
			DelegatorIdentity: token.DelegatorIdentity,
			DelegateIdentity:  token.DelegateIdentity,
			Scope:             mergeUniqueSorted(nil, token.Scope),
			ExpiresAt:         token.ExpiresAt.UTC(),
			Valid:             false,
			ErrorCode:         errorCode,
		})
	}

	return DelegationChainValidationResult{
		Complete:            validDelegations == len(requiredLinks),
		RequiredDelegations: len(requiredLinks),
		ValidDelegations:    validDelegations,
		ValidTokenIDs:       mergeUniqueSorted(nil, validTokenIDs),
		Entries:             entries,
	}, nil
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
	normalized.ContractDigest = strings.ToLower(strings.TrimSpace(normalized.ContractDigest))
	normalized.ParentTokenID = strings.TrimSpace(normalized.ParentTokenID)
	normalized.ParentTokenDigest = strings.ToLower(strings.TrimSpace(normalized.ParentTokenDigest))
	normalized.OriginAuthorityDigest = strings.ToLower(strings.TrimSpace(normalized.OriginAuthorityDigest))
	for _, v := range [][]string{normalized.ActionClasses, normalized.TargetScope, normalized.EnvironmentScope, normalized.DataClasses, normalized.NetworkDestinations} {
		for i := range v {
			v[i] = strings.ToLower(strings.TrimSpace(v[i]))
		}
	}
	for _, v := range [][]string{normalized.ActionClasses, normalized.TargetScope, normalized.EnvironmentScope, normalized.DataClasses, normalized.NetworkDestinations} {
		if len(v) > 0 {
			if hasDuplicate(v) {
				return schemagate.DelegationToken{}, fmt.Errorf("delegation axis duplicate")
			}
			for _, x := range v {
				if x == "" || x == "*" {
					return schemagate.DelegationToken{}, fmt.Errorf("delegation axis invalid")
				}
			}
		}
	}
	bound := normalized.ContractDigest != "" || len(normalized.ActionClasses) > 0 || len(normalized.TargetScope) > 0 || len(normalized.EnvironmentScope) > 0 || len(normalized.DataClasses) > 0 || len(normalized.NetworkDestinations) > 0 || normalized.MaxOperations > 0 || normalized.MaxTargets > 0 || normalized.MaxDescendantDepth > 0
	if bound && (!isDigestHex(normalized.ContractDigest) || len(normalized.ActionClasses) == 0 || len(normalized.TargetScope) == 0 || len(normalized.EnvironmentScope) == 0 || len(normalized.DataClasses) == 0 || len(normalized.NetworkDestinations) == 0 || normalized.MaxOperations < 1 || normalized.MaxTargets < 1 || normalized.MaxDescendantDepth < 1 || (normalized.Depth == 0 && (normalized.ParentTokenID != "" || normalized.ParentTokenDigest != "")) || (normalized.Depth > 0 && (normalized.ParentTokenID == "" || !validParentTokenDigest(normalized.ParentTokenDigest)))) {
		return schemagate.DelegationToken{}, fmt.Errorf("incomplete delegation binding")
	}
	if normalized.CreatedAt.IsZero() {
		return schemagate.DelegationToken{}, fmt.Errorf("created_at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return schemagate.DelegationToken{}, fmt.Errorf("expires_at is required")
	}
	return normalized, nil
}

func validParentTokenDigest(value string) bool {
	return isDigestHex(strings.TrimPrefix(strings.TrimSpace(value), "sha256:"))
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

func DelegationTokenDigest(t schemagate.DelegationToken) (string, error) {
	t.Signature = nil
	raw, e := json.Marshal(t)
	if e != nil {
		return "", e
	}
	d, e := jcs.DigestJCS(raw)
	if e != nil {
		return "", e
	}
	return "sha256:" + strings.TrimPrefix(d, "sha256:"), nil
}
func isContractBoundDelegation(t schemagate.DelegationToken) bool {
	return t.ContractDigest != "" || len(t.ActionClasses) > 0 || t.ParentTokenID != "" || t.Depth > 0
}
func ValidateDelegationNonExpansion(parent, child schemagate.DelegationToken) error {
	if parent.Revoked || child.Revoked {
		return fmt.Errorf("delegation_token_revoked")
	}
	if child.DelegatorIdentity != parent.DelegateIdentity || child.ParentTokenID != parent.TokenID {
		return fmt.Errorf("delegation_parent_mismatch")
	}
	pd, _ := DelegationTokenDigest(parent)
	if child.ParentTokenDigest != pd {
		return fmt.Errorf("delegation_parent_digest_mismatch")
	}
	if child.OriginAuthorityDigest != parent.OriginAuthorityDigest || child.ContractDigest != parent.ContractDigest || child.IntentDigest != parent.IntentDigest || child.PolicyDigest != parent.PolicyDigest {
		return fmt.Errorf("delegation_inherited_binding_mismatch")
	}
	if child.Depth != parent.Depth+1 || child.ExpiresAt.After(parent.ExpiresAt) || child.MaxOperations > parent.MaxOperations || child.MaxTargets > parent.MaxTargets || child.MaxDescendantDepth > parent.MaxDescendantDepth {
		return fmt.Errorf("delegation_authority_expanded")
	}
	for _, pair := range [][2][]string{{parent.Scope, child.Scope}, {parent.ActionClasses, child.ActionClasses}, {parent.TargetScope, child.TargetScope}, {parent.EnvironmentScope, child.EnvironmentScope}, {parent.DataClasses, child.DataClasses}, {parent.NetworkDestinations, child.NetworkDestinations}} {
		for _, v := range pair[1] {
			found := false
			for _, p := range pair[0] {
				if p == v {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("delegation_scope_expanded")
			}
		}
	}
	return nil
}

func matchesDelegationScope(requiredScope []string, tokenScope []string, scopeClass string) bool {
	if len(requiredScope) == 0 {
		return true
	}
	tokenSet := make(map[string]struct{}, len(tokenScope)+1)
	for _, scope := range tokenScope {
		tokenSet[scope] = struct{}{}
	}
	normalizedScopeClass := strings.ToLower(strings.TrimSpace(scopeClass))
	if normalizedScopeClass != "" {
		tokenSet[normalizedScopeClass] = struct{}{}
	}
	if _, ok := tokenSet["*"]; ok {
		return true
	}
	for _, scope := range requiredScope {
		if _, ok := tokenSet[scope]; ok {
			return true
		}
	}
	return false
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
