package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemavoice "github.com/Clyra-AI/gait/core/schema/v1/voice"
	"github.com/Clyra-AI/gait/core/sign"
)

const (
	commitmentIntentSchemaID = "gait.voice.commitment_intent"
	commitmentIntentSchemaV1 = "1.0.0"
	sayCapabilitySchemaID    = "gait.voice.say_token"
	sayCapabilitySchemaV1    = "1.0.0"

	SayTokenCodeSchemaInvalid   = "say_token_invalid"
	SayTokenCodeSignatureMiss   = "say_token_signature_missing"
	SayTokenCodeSignatureFailed = "say_token_signature_invalid"
	SayTokenCodeExpired         = "say_token_expired"
	SayTokenCodeIntentMismatch  = "say_token_intent_mismatch"
	SayTokenCodePolicyMismatch  = "say_token_policy_mismatch"
	SayTokenCodeCallMismatch    = "say_token_call_binding_mismatch"
	SayTokenCodeClassMismatch   = "say_token_class_mismatch"
)

var commitmentClasses = map[string]struct{}{
	"refund":         {},
	"quote":          {},
	"eligibility":    {},
	"schedule":       {},
	"cancel":         {},
	"account_change": {},
}

type MintSayTokenOptions struct {
	ProducerVersion    string
	CommitmentClass    string
	IntentDigest       string
	PolicyDigest       string
	CallID             string
	TurnIndex          int
	CallSeq            int
	Currency           string
	QuoteMinCents      int64
	QuoteMaxCents      int64
	RefundCeilingCents int64
	TTL                time.Duration
	Now                time.Time
	SigningPrivateKey  ed25519.PrivateKey
	TokenPath          string
}

type MintSayTokenResult struct {
	Token     schemavoice.SayToken
	TokenPath string
}

type SayTokenValidationOptions struct {
	Now                     time.Time
	ExpectedIntentDigest    string
	ExpectedPolicyDigest    string
	ExpectedCallID          string
	ExpectedTurnIndex       int
	ExpectedCallSeq         int
	ExpectedCommitmentClass string
}

type SayTokenError struct {
	Code string
	Err  error
}

func (e *SayTokenError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *SayTokenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsCommitmentClass(value string) bool {
	_, ok := commitmentClasses[normalizeCommitmentClass(value)]
	return ok
}

func NormalizeCommitmentIntent(input schemavoice.CommitmentIntent) (schemavoice.CommitmentIntent, error) {
	output := input
	if strings.TrimSpace(output.SchemaID) == "" {
		output.SchemaID = commitmentIntentSchemaID
	}
	if output.SchemaID != commitmentIntentSchemaID {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("unsupported schema_id: %s", output.SchemaID)
	}
	if strings.TrimSpace(output.SchemaVersion) == "" {
		output.SchemaVersion = commitmentIntentSchemaV1
	}
	if output.SchemaVersion != commitmentIntentSchemaV1 {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("unsupported schema_version: %s", output.SchemaVersion)
	}
	if output.CreatedAt.IsZero() {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("created_at is required")
	}
	output.CreatedAt = output.CreatedAt.UTC()
	output.ProducerVersion = strings.TrimSpace(output.ProducerVersion)
	if output.ProducerVersion == "" {
		output.ProducerVersion = "0.0.0-dev"
	}
	output.CallID = strings.TrimSpace(output.CallID)
	if output.CallID == "" {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("call_id is required")
	}
	if output.TurnIndex < 0 {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("turn_index must be >= 0")
	}
	if output.CallSeq <= 0 {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("call_seq must be >= 1")
	}
	output.CommitmentClass = normalizeCommitmentClass(output.CommitmentClass)
	if !IsCommitmentClass(output.CommitmentClass) {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("unsupported commitment_class: %s", output.CommitmentClass)
	}
	output.UtteranceDigest = strings.ToLower(strings.TrimSpace(output.UtteranceDigest))
	if output.UtteranceDigest != "" && !isDigestHex(output.UtteranceDigest) {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("utterance_digest must be sha256 hex when set")
	}
	output.Context.Identity = strings.TrimSpace(output.Context.Identity)
	output.Context.Workspace = strings.TrimSpace(output.Context.Workspace)
	output.Context.RiskClass = strings.ToLower(strings.TrimSpace(output.Context.RiskClass))
	output.Context.SessionID = strings.TrimSpace(output.Context.SessionID)
	output.Context.RequestID = strings.TrimSpace(output.Context.RequestID)
	output.Context.EnvironmentFingerprint = strings.TrimSpace(output.Context.EnvironmentFingerprint)
	if output.Context.Identity == "" || output.Context.Workspace == "" || output.Context.RiskClass == "" {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("context identity, workspace, and risk_class are required")
	}
	output.Currency = strings.ToUpper(strings.TrimSpace(output.Currency))
	if output.QuoteMinCents < 0 || output.QuoteMaxCents < 0 || output.RefundCeilingCents < 0 {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("bound values must be >= 0")
	}
	if output.QuoteMaxCents > 0 && output.QuoteMinCents > output.QuoteMaxCents {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("quote_max_cents must be >= quote_min_cents")
	}
	output.ApprovalArtifactRefs = normalizeStringList(output.ApprovalArtifactRefs)
	output.EvidenceReferenceDigests = normalizeDigests(output.EvidenceReferenceDigests)
	return output, nil
}

func CommitmentIntentToIntent(input schemavoice.CommitmentIntent) (schemagate.IntentRequest, error) {
	normalized, err := NormalizeCommitmentIntent(input)
	if err != nil {
		return schemagate.IntentRequest{}, err
	}
	args := map[string]any{
		"call_id":          normalized.CallID,
		"turn_index":       normalized.TurnIndex,
		"call_seq":         normalized.CallSeq,
		"commitment_class": normalized.CommitmentClass,
	}
	if normalized.UtteranceDigest != "" {
		args["utterance_digest"] = normalized.UtteranceDigest
	}
	if normalized.Currency != "" {
		args["currency"] = normalized.Currency
	}
	if normalized.QuoteMinCents > 0 {
		args["quote_min_cents"] = normalized.QuoteMinCents
	}
	if normalized.QuoteMaxCents > 0 {
		args["quote_max_cents"] = normalized.QuoteMaxCents
	}
	if normalized.RefundCeilingCents > 0 {
		args["refund_ceiling_cents"] = normalized.RefundCeilingCents
	}
	if normalized.ApprovalRequired {
		args["approval_required"] = true
	}
	if len(normalized.ApprovalArtifactRefs) > 0 {
		args["approval_artifact_refs"] = append([]string(nil), normalized.ApprovalArtifactRefs...)
	}
	if len(normalized.EvidenceReferenceDigests) > 0 {
		args["evidence_reference_digests"] = append([]string(nil), normalized.EvidenceReferenceDigests...)
	}

	intent := schemagate.IntentRequest{
		SchemaID:        intentRequestSchemaID,
		SchemaVersion:   intentRequestSchemaV1,
		CreatedAt:       normalized.CreatedAt,
		ProducerVersion: normalized.ProducerVersion,
		ToolName:        "voice.commitment." + normalized.CommitmentClass,
		Args:            args,
		Targets: []schemagate.IntentTarget{{
			Kind:          "other",
			Value:         normalized.CommitmentClass,
			Operation:     "speak",
			EndpointClass: "ui.type",
		}},
		Context: schemagate.IntentContext{
			Identity:               normalized.Context.Identity,
			Workspace:              normalized.Context.Workspace,
			RiskClass:              normalized.Context.RiskClass,
			SessionID:              normalized.Context.SessionID,
			RequestID:              normalized.Context.RequestID,
			EnvironmentFingerprint: normalized.Context.EnvironmentFingerprint,
		},
	}
	return NormalizeIntent(intent)
}

func MintSayToken(opts MintSayTokenOptions) (MintSayTokenResult, error) {
	if len(opts.SigningPrivateKey) == 0 {
		return MintSayTokenResult{}, fmt.Errorf("signing private key is required")
	}
	if opts.TTL <= 0 {
		return MintSayTokenResult{}, fmt.Errorf("ttl must be greater than 0")
	}
	commitmentClass := normalizeCommitmentClass(opts.CommitmentClass)
	if !IsCommitmentClass(commitmentClass) {
		return MintSayTokenResult{}, fmt.Errorf("unsupported commitment_class: %s", opts.CommitmentClass)
	}
	intentDigest := strings.ToLower(strings.TrimSpace(opts.IntentDigest))
	if !isDigestHex(intentDigest) {
		return MintSayTokenResult{}, fmt.Errorf("intent_digest must be sha256 hex")
	}
	policyDigest := strings.ToLower(strings.TrimSpace(opts.PolicyDigest))
	if !isDigestHex(policyDigest) {
		return MintSayTokenResult{}, fmt.Errorf("policy_digest must be sha256 hex")
	}
	callID := strings.TrimSpace(opts.CallID)
	if callID == "" {
		return MintSayTokenResult{}, fmt.Errorf("call_id is required")
	}
	if opts.TurnIndex < 0 {
		return MintSayTokenResult{}, fmt.Errorf("turn_index must be >= 0")
	}
	if opts.CallSeq <= 0 {
		return MintSayTokenResult{}, fmt.Errorf("call_seq must be >= 1")
	}
	if opts.QuoteMinCents < 0 || opts.QuoteMaxCents < 0 || opts.RefundCeilingCents < 0 {
		return MintSayTokenResult{}, fmt.Errorf("bound values must be >= 0")
	}
	if opts.QuoteMaxCents > 0 && opts.QuoteMinCents > opts.QuoteMaxCents {
		return MintSayTokenResult{}, fmt.Errorf("quote_max_cents must be >= quote_min_cents")
	}

	createdAt := opts.Now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	token := schemavoice.SayToken{
		SchemaID:           sayCapabilitySchemaID,
		SchemaVersion:      sayCapabilitySchemaV1,
		CreatedAt:          createdAt,
		ProducerVersion:    producerVersion,
		TokenID:            computeSayTokenID(intentDigest, policyDigest, callID, opts.TurnIndex, opts.CallSeq, commitmentClass, createdAt.Add(opts.TTL)),
		CommitmentClass:    commitmentClass,
		IntentDigest:       intentDigest,
		PolicyDigest:       policyDigest,
		CallID:             callID,
		TurnIndex:          opts.TurnIndex,
		CallSeq:            opts.CallSeq,
		Currency:           strings.ToUpper(strings.TrimSpace(opts.Currency)),
		QuoteMinCents:      opts.QuoteMinCents,
		QuoteMaxCents:      opts.QuoteMaxCents,
		RefundCeilingCents: opts.RefundCeilingCents,
		ExpiresAt:          createdAt.Add(opts.TTL),
	}
	signable := token
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return MintSayTokenResult{}, fmt.Errorf("marshal signable say token: %w", err)
	}
	signature, err := sign.SignJSON(opts.SigningPrivateKey, signableRaw)
	if err != nil {
		return MintSayTokenResult{}, fmt.Errorf("sign say token: %w", err)
	}
	token.Signature = &schemavoice.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}
	tokenPath := strings.TrimSpace(opts.TokenPath)
	if tokenPath == "" {
		tokenPath = fmt.Sprintf("say_token_%s.json", token.TokenID)
	}
	if err := WriteSayToken(tokenPath, token); err != nil {
		return MintSayTokenResult{}, err
	}
	return MintSayTokenResult{Token: token, TokenPath: tokenPath}, nil
}

func WriteSayToken(path string, token schemavoice.SayToken) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create say token directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal say token: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write say token: %w", err)
	}
	return nil
}

func ReadSayToken(path string) (schemavoice.SayToken, error) {
	// #nosec G304 -- say token path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemavoice.SayToken{}, fmt.Errorf("read say token: %w", err)
	}
	var token schemavoice.SayToken
	if err := json.Unmarshal(content, &token); err != nil {
		return schemavoice.SayToken{}, fmt.Errorf("parse say token: %w", err)
	}
	return token, nil
}

func ValidateSayToken(token schemavoice.SayToken, publicKey ed25519.PublicKey, opts SayTokenValidationOptions) error {
	normalized, err := normalizeSayToken(token)
	if err != nil {
		return &SayTokenError{Code: SayTokenCodeSchemaInvalid, Err: err}
	}
	if len(publicKey) == 0 {
		return &SayTokenError{Code: SayTokenCodeSignatureFailed, Err: fmt.Errorf("verification public key is required")}
	}
	if normalized.Signature == nil {
		return &SayTokenError{Code: SayTokenCodeSignatureMiss, Err: fmt.Errorf("signature missing")}
	}
	signable := normalized
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return &SayTokenError{Code: SayTokenCodeSchemaInvalid, Err: fmt.Errorf("marshal signable token: %w", err)}
	}
	ok, err := sign.VerifyJSON(publicKey, sign.Signature{
		Alg:          normalized.Signature.Alg,
		KeyID:        normalized.Signature.KeyID,
		Sig:          normalized.Signature.Sig,
		SignedDigest: normalized.Signature.SignedDigest,
	}, signableRaw)
	if err != nil {
		return &SayTokenError{Code: SayTokenCodeSignatureFailed, Err: err}
	}
	if !ok {
		return &SayTokenError{Code: SayTokenCodeSignatureFailed, Err: fmt.Errorf("signature verification failed")}
	}
	expectedIntent := strings.ToLower(strings.TrimSpace(opts.ExpectedIntentDigest))
	if expectedIntent != "" && normalized.IntentDigest != expectedIntent {
		return &SayTokenError{Code: SayTokenCodeIntentMismatch, Err: fmt.Errorf("intent digest mismatch")}
	}
	expectedPolicy := strings.ToLower(strings.TrimSpace(opts.ExpectedPolicyDigest))
	if expectedPolicy != "" && normalized.PolicyDigest != expectedPolicy {
		return &SayTokenError{Code: SayTokenCodePolicyMismatch, Err: fmt.Errorf("policy digest mismatch")}
	}
	expectedCallID := strings.TrimSpace(opts.ExpectedCallID)
	if expectedCallID != "" && normalized.CallID != expectedCallID {
		return &SayTokenError{Code: SayTokenCodeCallMismatch, Err: fmt.Errorf("call_id mismatch")}
	}
	if opts.ExpectedTurnIndex >= 0 && normalized.TurnIndex != opts.ExpectedTurnIndex {
		return &SayTokenError{Code: SayTokenCodeCallMismatch, Err: fmt.Errorf("turn_index mismatch")}
	}
	if opts.ExpectedCallSeq > 0 && normalized.CallSeq != opts.ExpectedCallSeq {
		return &SayTokenError{Code: SayTokenCodeCallMismatch, Err: fmt.Errorf("call_seq mismatch")}
	}
	expectedClass := normalizeCommitmentClass(opts.ExpectedCommitmentClass)
	if expectedClass != "" && normalized.CommitmentClass != expectedClass {
		return &SayTokenError{Code: SayTokenCodeClassMismatch, Err: fmt.Errorf("commitment_class mismatch")}
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !now.Before(normalized.ExpiresAt.UTC()) {
		return &SayTokenError{Code: SayTokenCodeExpired, Err: fmt.Errorf("token expired")}
	}
	return nil
}

func normalizeSayToken(token schemavoice.SayToken) (schemavoice.SayToken, error) {
	normalized := token
	if strings.TrimSpace(normalized.SchemaID) == "" {
		normalized.SchemaID = sayCapabilitySchemaID
	}
	if normalized.SchemaID != sayCapabilitySchemaID {
		return schemavoice.SayToken{}, fmt.Errorf("unsupported schema_id: %s", normalized.SchemaID)
	}
	if strings.TrimSpace(normalized.SchemaVersion) == "" {
		normalized.SchemaVersion = sayCapabilitySchemaV1
	}
	if normalized.SchemaVersion != sayCapabilitySchemaV1 {
		return schemavoice.SayToken{}, fmt.Errorf("unsupported schema_version: %s", normalized.SchemaVersion)
	}
	normalized.TokenID = strings.TrimSpace(normalized.TokenID)
	if normalized.TokenID == "" {
		return schemavoice.SayToken{}, fmt.Errorf("token_id is required")
	}
	normalized.CommitmentClass = normalizeCommitmentClass(normalized.CommitmentClass)
	if !IsCommitmentClass(normalized.CommitmentClass) {
		return schemavoice.SayToken{}, fmt.Errorf("unsupported commitment_class: %s", normalized.CommitmentClass)
	}
	normalized.IntentDigest = strings.ToLower(strings.TrimSpace(normalized.IntentDigest))
	if !isDigestHex(normalized.IntentDigest) {
		return schemavoice.SayToken{}, fmt.Errorf("intent_digest must be sha256 hex")
	}
	normalized.PolicyDigest = strings.ToLower(strings.TrimSpace(normalized.PolicyDigest))
	if !isDigestHex(normalized.PolicyDigest) {
		return schemavoice.SayToken{}, fmt.Errorf("policy_digest must be sha256 hex")
	}
	normalized.CallID = strings.TrimSpace(normalized.CallID)
	if normalized.CallID == "" {
		return schemavoice.SayToken{}, fmt.Errorf("call_id is required")
	}
	if normalized.TurnIndex < 0 {
		return schemavoice.SayToken{}, fmt.Errorf("turn_index must be >= 0")
	}
	if normalized.CallSeq <= 0 {
		return schemavoice.SayToken{}, fmt.Errorf("call_seq must be >= 1")
	}
	normalized.Currency = strings.ToUpper(strings.TrimSpace(normalized.Currency))
	if normalized.QuoteMinCents < 0 || normalized.QuoteMaxCents < 0 || normalized.RefundCeilingCents < 0 {
		return schemavoice.SayToken{}, fmt.Errorf("bound values must be >= 0")
	}
	if normalized.QuoteMaxCents > 0 && normalized.QuoteMinCents > normalized.QuoteMaxCents {
		return schemavoice.SayToken{}, fmt.Errorf("quote_max_cents must be >= quote_min_cents")
	}
	if normalized.CreatedAt.IsZero() {
		return schemavoice.SayToken{}, fmt.Errorf("created_at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return schemavoice.SayToken{}, fmt.Errorf("expires_at is required")
	}
	return normalized, nil
}

func computeSayTokenID(intentDigest, policyDigest, callID string, turnIndex int, callSeq int, commitmentClass string, expiresAt time.Time) string {
	raw := intentDigest + ":" + policyDigest + ":" + callID + ":" + fmt.Sprintf("%d:%d", turnIndex, callSeq) + ":" + commitmentClass + ":" + expiresAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}

func normalizeCommitmentClass(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeDigests(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		digest := strings.ToLower(strings.TrimSpace(value))
		if digest == "" {
			continue
		}
		if !isDigestHex(digest) {
			continue
		}
		if _, ok := seen[digest]; ok {
			continue
		}
		seen[digest] = struct{}{}
		out = append(out, digest)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}
