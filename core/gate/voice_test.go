package gate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemavoice "github.com/Clyra-AI/gait/core/schema/v1/voice"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestCommitmentIntentToIntent(t *testing.T) {
	intent, err := CommitmentIntentToIntent(schemavoice.CommitmentIntent{
		SchemaID:        commitmentIntentSchemaID,
		SchemaVersion:   commitmentIntentSchemaV1,
		CreatedAt:       time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		CallID:          "call_demo",
		TurnIndex:       2,
		CallSeq:         5,
		CommitmentClass: "quote",
		Context: schemavoice.CommitmentContext{
			Identity:  "agent.voice",
			Workspace: "/srv/voice",
			RiskClass: "high",
			SessionID: "sess_demo",
			RequestID: "req_demo",
		},
		QuoteMinCents: 1000,
		QuoteMaxCents: 2200,
	})
	if err != nil {
		t.Fatalf("commitment intent to intent: %v", err)
	}
	if intent.ToolName != "voice.commitment.quote" {
		t.Fatalf("unexpected tool name: %s", intent.ToolName)
	}
	if intent.Context.Identity != "agent.voice" || intent.Context.Workspace != "/srv/voice" {
		t.Fatalf("unexpected context mapping: %#v", intent.Context)
	}
	if intent.IntentDigest == "" || intent.ArgsDigest == "" {
		t.Fatalf("expected digests to be populated")
	}
	if _, ok := intent.Args["quote_min_cents"]; !ok {
		t.Fatalf("expected quote_min_cents in normalized args")
	}
}

func TestNormalizeCommitmentIntentRejectsInvalidClass(t *testing.T) {
	_, err := NormalizeCommitmentIntent(schemavoice.CommitmentIntent{
		SchemaID:        commitmentIntentSchemaID,
		SchemaVersion:   commitmentIntentSchemaV1,
		CreatedAt:       time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		CallID:          "call_demo",
		TurnIndex:       0,
		CallSeq:         1,
		CommitmentClass: "unknown",
		Context: schemavoice.CommitmentContext{
			Identity:  "agent.voice",
			Workspace: "/srv/voice",
			RiskClass: "high",
		},
	})
	if err == nil {
		t.Fatalf("expected invalid commitment_class error")
	}
}

func TestMintAndValidateSayToken(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "say_token.json")
	now := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	result, err := MintSayToken(MintSayTokenOptions{
		ProducerVersion:    "test",
		CommitmentClass:    "refund",
		IntentDigest:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CallID:             "call_demo",
		TurnIndex:          3,
		CallSeq:            8,
		RefundCeilingCents: 5000,
		TTL:                30 * time.Minute,
		Now:                now,
		SigningPrivateKey:  keyPair.Private,
		TokenPath:          tokenPath,
	})
	if err != nil {
		t.Fatalf("mint say token: %v", err)
	}
	if result.TokenPath != tokenPath {
		t.Fatalf("unexpected token path: %s", result.TokenPath)
	}
	parsed, err := ReadSayToken(tokenPath)
	if err != nil {
		t.Fatalf("read say token: %v", err)
	}
	if err := ValidateSayToken(parsed, keyPair.Public, SayTokenValidationOptions{
		Now:                     now.Add(5 * time.Minute),
		ExpectedIntentDigest:    result.Token.IntentDigest,
		ExpectedPolicyDigest:    result.Token.PolicyDigest,
		ExpectedCallID:          "call_demo",
		ExpectedTurnIndex:       3,
		ExpectedCallSeq:         8,
		ExpectedCommitmentClass: "refund",
	}); err != nil {
		t.Fatalf("validate say token: %v", err)
	}
}

func TestValidateSayTokenMismatchAndExpired(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	result, err := MintSayToken(MintSayTokenOptions{
		ProducerVersion:   "test",
		CommitmentClass:   "quote",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CallID:            "call_demo",
		TurnIndex:         2,
		CallSeq:           4,
		QuoteMinCents:     1000,
		QuoteMaxCents:     1200,
		TTL:               1 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(t.TempDir(), "say_token.json"),
	})
	if err != nil {
		t.Fatalf("mint say token: %v", err)
	}
	err = ValidateSayToken(result.Token, keyPair.Public, SayTokenValidationOptions{
		ExpectedIntentDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	})
	if err == nil {
		t.Fatalf("expected intent mismatch error")
	}
	var tokenErr *SayTokenError
	if !errors.As(err, &tokenErr) || tokenErr.Code != SayTokenCodeIntentMismatch {
		t.Fatalf("unexpected token error: %v", err)
	}
	err = ValidateSayToken(result.Token, keyPair.Public, SayTokenValidationOptions{
		Now:               now.Add(2 * time.Minute),
		ExpectedTurnIndex: -1,
	})
	if err == nil {
		t.Fatalf("expected expired token error")
	}
	if !errors.As(err, &tokenErr) || tokenErr.Code != SayTokenCodeExpired {
		t.Fatalf("unexpected expired token error: %v", err)
	}
}

func TestSayTokenErrorHelpers(t *testing.T) {
	var nilErr *SayTokenError
	if nilErr.Error() != "" {
		t.Fatalf("nil SayTokenError should render empty string")
	}
	if nilErr.Unwrap() != nil {
		t.Fatalf("nil SayTokenError should unwrap to nil")
	}

	codeOnly := &SayTokenError{Code: SayTokenCodeSchemaInvalid}
	if codeOnly.Error() != SayTokenCodeSchemaInvalid {
		t.Fatalf("unexpected code-only error string: %q", codeOnly.Error())
	}

	baseErr := errors.New("boom")
	withCause := &SayTokenError{Code: SayTokenCodeSignatureFailed, Err: baseErr}
	if !strings.Contains(withCause.Error(), "boom") {
		t.Fatalf("expected wrapped message in error: %q", withCause.Error())
	}
	if !errors.Is(withCause, baseErr) {
		t.Fatalf("expected wrapped error to be discoverable via errors.Is")
	}
}

func TestNormalizeCommitmentIntentValidationAndDefaults(t *testing.T) {
	base := schemavoice.CommitmentIntent{
		CreatedAt:       time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
		CallID:          "call_demo",
		TurnIndex:       2,
		CallSeq:         5,
		CommitmentClass: "quote",
		Context: schemavoice.CommitmentContext{
			Identity:  "agent.voice",
			Workspace: "/srv/voice",
			RiskClass: "high",
		},
		ApprovalArtifactRefs: []string{"ref_a", "  ", "ref_a", "ref_b"},
		EvidenceReferenceDigests: []string{
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"invalid",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		},
	}

	normalized, err := NormalizeCommitmentIntent(base)
	if err != nil {
		t.Fatalf("normalize commitment intent: %v", err)
	}
	if normalized.SchemaID != commitmentIntentSchemaID || normalized.SchemaVersion != commitmentIntentSchemaV1 {
		t.Fatalf("expected schema defaults, got %q %q", normalized.SchemaID, normalized.SchemaVersion)
	}
	if normalized.ProducerVersion == "" {
		t.Fatalf("expected producer version default")
	}
	if len(normalized.ApprovalArtifactRefs) != 2 {
		t.Fatalf("expected deduped approval refs, got %#v", normalized.ApprovalArtifactRefs)
	}
	if len(normalized.EvidenceReferenceDigests) != 2 {
		t.Fatalf("expected filtered evidence digests, got %#v", normalized.EvidenceReferenceDigests)
	}
	if normalized.EvidenceReferenceDigests[0] > normalized.EvidenceReferenceDigests[1] {
		t.Fatalf("expected sorted evidence digests, got %#v", normalized.EvidenceReferenceDigests)
	}

	cases := []struct {
		name    string
		mutate  func(*schemavoice.CommitmentIntent)
		wantErr string
	}{
		{name: "bad_schema_id", mutate: func(intent *schemavoice.CommitmentIntent) { intent.SchemaID = "bad" }, wantErr: "unsupported schema_id"},
		{name: "bad_schema_version", mutate: func(intent *schemavoice.CommitmentIntent) { intent.SchemaVersion = "2.0.0" }, wantErr: "unsupported schema_version"},
		{name: "missing_created_at", mutate: func(intent *schemavoice.CommitmentIntent) { intent.CreatedAt = time.Time{} }, wantErr: "created_at is required"},
		{name: "missing_call_id", mutate: func(intent *schemavoice.CommitmentIntent) { intent.CallID = " " }, wantErr: "call_id is required"},
		{name: "negative_turn_index", mutate: func(intent *schemavoice.CommitmentIntent) { intent.TurnIndex = -1 }, wantErr: "turn_index must be >= 0"},
		{name: "invalid_call_seq", mutate: func(intent *schemavoice.CommitmentIntent) { intent.CallSeq = 0 }, wantErr: "call_seq must be >= 1"},
		{name: "bad_class", mutate: func(intent *schemavoice.CommitmentIntent) { intent.CommitmentClass = "not_real" }, wantErr: "unsupported commitment_class"},
		{name: "bad_utterance_digest", mutate: func(intent *schemavoice.CommitmentIntent) { intent.UtteranceDigest = "not_hex" }, wantErr: "utterance_digest must be sha256 hex"},
		{name: "missing_context_identity", mutate: func(intent *schemavoice.CommitmentIntent) { intent.Context.Identity = "" }, wantErr: "context identity, workspace, and risk_class are required"},
		{name: "negative_bounds", mutate: func(intent *schemavoice.CommitmentIntent) { intent.QuoteMinCents = -1 }, wantErr: "bound values must be >= 0"},
		{name: "quote_max_lt_quote_min", mutate: func(intent *schemavoice.CommitmentIntent) { intent.QuoteMinCents = 500; intent.QuoteMaxCents = 100 }, wantErr: "quote_max_cents must be >= quote_min_cents"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			invalid := base
			testCase.mutate(&invalid)
			if _, err := NormalizeCommitmentIntent(invalid); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestMintSayTokenInputValidation(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	base := MintSayTokenOptions{
		ProducerVersion:   "test",
		CommitmentClass:   "quote",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CallID:            "call_demo",
		TurnIndex:         1,
		CallSeq:           2,
		QuoteMinCents:     100,
		QuoteMaxCents:     200,
		TTL:               2 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
	}

	cases := []struct {
		name    string
		mutate  func(*MintSayTokenOptions)
		wantErr string
	}{
		{name: "missing_private_key", mutate: func(opts *MintSayTokenOptions) { opts.SigningPrivateKey = nil }, wantErr: "signing private key is required"},
		{name: "invalid_ttl", mutate: func(opts *MintSayTokenOptions) { opts.TTL = 0 }, wantErr: "ttl must be greater than 0"},
		{name: "invalid_class", mutate: func(opts *MintSayTokenOptions) { opts.CommitmentClass = "fake" }, wantErr: "unsupported commitment_class"},
		{name: "invalid_intent_digest", mutate: func(opts *MintSayTokenOptions) { opts.IntentDigest = "bad" }, wantErr: "intent_digest must be sha256 hex"},
		{name: "invalid_policy_digest", mutate: func(opts *MintSayTokenOptions) { opts.PolicyDigest = "bad" }, wantErr: "policy_digest must be sha256 hex"},
		{name: "missing_call_id", mutate: func(opts *MintSayTokenOptions) { opts.CallID = "" }, wantErr: "call_id is required"},
		{name: "negative_turn_index", mutate: func(opts *MintSayTokenOptions) { opts.TurnIndex = -1 }, wantErr: "turn_index must be >= 0"},
		{name: "invalid_call_seq", mutate: func(opts *MintSayTokenOptions) { opts.CallSeq = 0 }, wantErr: "call_seq must be >= 1"},
		{name: "negative_bound", mutate: func(opts *MintSayTokenOptions) { opts.RefundCeilingCents = -1 }, wantErr: "bound values must be >= 0"},
		{name: "quote_max_lt_min", mutate: func(opts *MintSayTokenOptions) { opts.QuoteMinCents = 500; opts.QuoteMaxCents = 100 }, wantErr: "quote_max_cents must be >= quote_min_cents"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			opts := base
			testCase.mutate(&opts)
			if _, err := MintSayToken(opts); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestValidateSayTokenAdditionalErrors(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	minted, err := MintSayToken(MintSayTokenOptions{
		ProducerVersion:   "test",
		CommitmentClass:   "quote",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CallID:            "call_demo",
		TurnIndex:         2,
		CallSeq:           5,
		TTL:               5 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(t.TempDir(), "say_token.json"),
	})
	if err != nil {
		t.Fatalf("mint say token: %v", err)
	}

	cases := []struct {
		name       string
		token      schemavoice.SayToken
		publicKey  []byte
		opts       SayTokenValidationOptions
		wantCode   string
		wantErrSub string
	}{
		{
			name:      "missing_public_key",
			token:     minted.Token,
			publicKey: nil,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1},
			wantCode:  SayTokenCodeSignatureFailed,
		},
		{
			name: "missing_signature",
			token: func() schemavoice.SayToken {
				noSig := minted.Token
				noSig.Signature = nil
				return noSig
			}(),
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1},
			wantCode:  SayTokenCodeSignatureMiss,
		},
		{
			name:      "policy_mismatch",
			token:     minted.Token,
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1, ExpectedPolicyDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
			wantCode:  SayTokenCodePolicyMismatch,
		},
		{
			name:      "call_id_mismatch",
			token:     minted.Token,
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1, ExpectedCallID: "other_call"},
			wantCode:  SayTokenCodeCallMismatch,
		},
		{
			name:      "turn_index_mismatch",
			token:     minted.Token,
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: 99},
			wantCode:  SayTokenCodeCallMismatch,
		},
		{
			name:      "call_seq_mismatch",
			token:     minted.Token,
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1, ExpectedCallSeq: 99},
			wantCode:  SayTokenCodeCallMismatch,
		},
		{
			name:      "class_mismatch",
			token:     minted.Token,
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1, ExpectedCommitmentClass: "refund"},
			wantCode:  SayTokenCodeClassMismatch,
		},
		{
			name: "schema_invalid",
			token: func() schemavoice.SayToken {
				invalid := minted.Token
				invalid.SchemaID = "bad"
				return invalid
			}(),
			publicKey: keyPair.Public,
			opts:      SayTokenValidationOptions{ExpectedTurnIndex: -1},
			wantCode:  SayTokenCodeSchemaInvalid,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateSayToken(testCase.token, testCase.publicKey, testCase.opts)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			var tokenErr *SayTokenError
			if !errors.As(err, &tokenErr) {
				t.Fatalf("expected SayTokenError, got %T (%v)", err, err)
			}
			if tokenErr.Code != testCase.wantCode {
				t.Fatalf("expected code %q, got %q (%v)", testCase.wantCode, tokenErr.Code, err)
			}
		})
	}
}

func TestReadSayTokenParseFailure(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "bad_token.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed token: %v", err)
	}
	if _, err := ReadSayToken(path); err == nil || !strings.Contains(err.Error(), "parse say token") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestNormalizeSayTokenValidationPaths(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	minted, err := MintSayToken(MintSayTokenOptions{
		ProducerVersion:   "test",
		CommitmentClass:   "quote",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CallID:            "call_demo",
		TurnIndex:         1,
		CallSeq:           2,
		TTL:               5 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(t.TempDir(), "say_token.json"),
	})
	if err != nil {
		t.Fatalf("mint say token: %v", err)
	}

	token := minted.Token
	token.SchemaID = ""
	token.SchemaVersion = ""
	normalized, err := normalizeSayToken(token)
	if err != nil {
		t.Fatalf("normalize valid token with defaults: %v", err)
	}
	if normalized.SchemaID != sayCapabilitySchemaID || normalized.SchemaVersion != sayCapabilitySchemaV1 {
		t.Fatalf("expected schema defaults, got %q %q", normalized.SchemaID, normalized.SchemaVersion)
	}

	cases := []struct {
		name    string
		mutate  func(*schemavoice.SayToken)
		wantErr string
	}{
		{name: "schema_id", mutate: func(candidate *schemavoice.SayToken) { candidate.SchemaID = "bad" }, wantErr: "unsupported schema_id"},
		{name: "schema_version", mutate: func(candidate *schemavoice.SayToken) { candidate.SchemaVersion = "2.0.0" }, wantErr: "unsupported schema_version"},
		{name: "token_id", mutate: func(candidate *schemavoice.SayToken) { candidate.TokenID = "" }, wantErr: "token_id is required"},
		{name: "class", mutate: func(candidate *schemavoice.SayToken) { candidate.CommitmentClass = "fake" }, wantErr: "unsupported commitment_class"},
		{name: "intent_digest", mutate: func(candidate *schemavoice.SayToken) { candidate.IntentDigest = "bad" }, wantErr: "intent_digest must be sha256 hex"},
		{name: "policy_digest", mutate: func(candidate *schemavoice.SayToken) { candidate.PolicyDigest = "bad" }, wantErr: "policy_digest must be sha256 hex"},
		{name: "call_id", mutate: func(candidate *schemavoice.SayToken) { candidate.CallID = " " }, wantErr: "call_id is required"},
		{name: "turn_index", mutate: func(candidate *schemavoice.SayToken) { candidate.TurnIndex = -1 }, wantErr: "turn_index must be >= 0"},
		{name: "call_seq", mutate: func(candidate *schemavoice.SayToken) { candidate.CallSeq = 0 }, wantErr: "call_seq must be >= 1"},
		{name: "negative_bound", mutate: func(candidate *schemavoice.SayToken) { candidate.RefundCeilingCents = -1 }, wantErr: "bound values must be >= 0"},
		{name: "quote_bounds", mutate: func(candidate *schemavoice.SayToken) { candidate.QuoteMinCents = 5; candidate.QuoteMaxCents = 1 }, wantErr: "quote_max_cents must be >= quote_min_cents"},
		{name: "created_at", mutate: func(candidate *schemavoice.SayToken) { candidate.CreatedAt = time.Time{} }, wantErr: "created_at is required"},
		{name: "expires_at", mutate: func(candidate *schemavoice.SayToken) { candidate.ExpiresAt = time.Time{} }, wantErr: "expires_at is required"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := minted.Token
			testCase.mutate(&candidate)
			if _, err := normalizeSayToken(candidate); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected normalizeSayToken error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}
