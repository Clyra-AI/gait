package gate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
)

func TestMintAndValidateDelegationToken(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "delegation.json")

	minted, err := MintDelegationToken(MintDelegationTokenOptions{
		ProducerVersion:   "test",
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		ScopeClass:        "write",
		IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		TTL:               time.Hour,
		Now:               time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
		SigningPrivateKey: keyPair.Private,
		TokenPath:         tokenPath,
	})
	if err != nil {
		t.Fatalf("mint delegation token: %v", err)
	}
	if minted.Token.TokenID == "" || minted.TokenPath == "" {
		t.Fatalf("unexpected mint result: %#v", minted)
	}

	token, err := ReadDelegationToken(tokenPath)
	if err != nil {
		t.Fatalf("read delegation token: %v", err)
	}
	if err := ValidateDelegationToken(token, keyPair.Public, DelegationValidationOptions{
		Now:                  time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		ExpectedDelegator:    "agent.lead",
		ExpectedDelegate:     "agent.specialist",
		RequiredScope:        []string{"tool:tool.write"},
		ExpectedIntentDigest: token.IntentDigest,
		ExpectedPolicyDigest: token.PolicyDigest,
	}); err != nil {
		t.Fatalf("validate delegation token: %v", err)
	}
}

func TestValidateDelegationTokenErrorCodes(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "delegation_error_codes.json")
	minted, err := MintDelegationToken(MintDelegationTokenOptions{
		ProducerVersion:   "test",
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		TTL:               time.Hour,
		Now:               time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
		SigningPrivateKey: keyPair.Private,
		TokenPath:         tokenPath,
	})
	if err != nil {
		t.Fatalf("mint delegation token: %v", err)
	}

	err = ValidateDelegationToken(minted.Token, keyPair.Public, DelegationValidationOptions{
		Now:              time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		ExpectedDelegate: "agent.other",
	})
	if err == nil {
		t.Fatalf("expected delegate mismatch error")
	}
	var tokenErr *DelegationTokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("expected DelegationTokenError, got %T", err)
	}
	if tokenErr.Code != DelegationCodeDelegateMis {
		t.Fatalf("unexpected delegation token error code: %s", tokenErr.Code)
	}

	err = ValidateDelegationToken(minted.Token, keyPair.Public, DelegationValidationOptions{
		Now: time.Date(2026, time.February, 10, 2, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("expected expired token error")
	}
	if !errors.As(err, &tokenErr) {
		t.Fatalf("expected DelegationTokenError, got %T", err)
	}
	if tokenErr.Code != DelegationCodeExpired {
		t.Fatalf("unexpected expiration error code: %s", tokenErr.Code)
	}
}

func TestDelegationTokenErrorHelpers(t *testing.T) {
	var nilErr *DelegationTokenError
	if nilErr.Error() != "" {
		t.Fatalf("nil error should format as empty string")
	}
	if nilErr.Unwrap() != nil {
		t.Fatalf("nil error should unwrap to nil")
	}

	wrapped := &DelegationTokenError{Code: "delegation_code", Err: errors.New("boom")}
	if got := wrapped.Error(); !strings.Contains(got, "delegation_code") || !strings.Contains(got, "boom") {
		t.Fatalf("unexpected wrapped error string: %q", got)
	}
	if wrapped.Unwrap() == nil {
		t.Fatalf("expected unwrap to return wrapped error")
	}
}

func TestValidateDelegationTokenAdditionalFailureModes(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "delegation_additional_failures.json")
	minted, err := MintDelegationToken(MintDelegationTokenOptions{
		ProducerVersion:   "test",
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		ScopeClass:        "write",
		IntentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		TTL:               time.Hour,
		Now:               time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
		SigningPrivateKey: keyPair.Private,
		TokenPath:         tokenPath,
	})
	if err != nil {
		t.Fatalf("mint delegation token: %v", err)
	}

	invalid := minted.Token
	invalid.SchemaID = "invalid.schema"
	err = ValidateDelegationToken(invalid, keyPair.Public, DelegationValidationOptions{
		Now: time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
	})
	assertDelegationErrorCode(t, err, DelegationCodeSchemaInvalid)

	noSignature := minted.Token
	noSignature.Signature = nil
	err = ValidateDelegationToken(noSignature, keyPair.Public, DelegationValidationOptions{
		Now: time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
	})
	assertDelegationErrorCode(t, err, DelegationCodeSignatureMiss)

	err = ValidateDelegationToken(minted.Token, nil, DelegationValidationOptions{
		Now: time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
	})
	assertDelegationErrorCode(t, err, DelegationCodeSignatureFailed)

	err = ValidateDelegationToken(minted.Token, keyPair.Public, DelegationValidationOptions{
		Now:           time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		RequiredScope: []string{"tool:tool.delete"},
	})
	assertDelegationErrorCode(t, err, DelegationCodeScopeMismatch)

	err = ValidateDelegationToken(minted.Token, keyPair.Public, DelegationValidationOptions{
		Now:                  time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		ExpectedIntentDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	})
	assertDelegationErrorCode(t, err, DelegationCodeIntentMismatch)

	err = ValidateDelegationToken(minted.Token, keyPair.Public, DelegationValidationOptions{
		Now:                  time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		ExpectedPolicyDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	assertDelegationErrorCode(t, err, DelegationCodePolicyMismatch)
}

func TestDelegationBindingDigest(t *testing.T) {
	if digest, err := DelegationBindingDigest(schemagate.IntentRequest{
		ToolName: "tool.read",
		Context: schemagate.IntentContext{
			Identity:  "agent.reader",
			Workspace: "/repo/gait",
			RiskClass: "low",
		},
	}); err != nil || digest != "" {
		t.Fatalf("expected empty delegation binding for intent without delegation, got digest=%q err=%v", digest, err)
	}

	intent := schemagate.IntentRequest{
		ToolName: "tool.write",
		Context: schemagate.IntentContext{
			Identity:  "agent.specialist",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
		Delegation: &schemagate.IntentDelegation{
			RequesterIdentity: "agent.specialist",
			ScopeClass:        "write",
			Chain: []schemagate.DelegationLink{
				{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.specialist", ScopeClass: "write"},
			},
			TokenRefs: []string{"token_b", "token_a"},
		},
	}
	digestA, err := DelegationBindingDigest(intent)
	if err != nil {
		t.Fatalf("delegation binding digest: %v", err)
	}
	if digestA == "" {
		t.Fatalf("expected non-empty delegation binding digest")
	}
	digestB, err := DelegationBindingDigest(intent)
	if err != nil {
		t.Fatalf("delegation binding digest second pass: %v", err)
	}
	if digestA != digestB {
		t.Fatalf("expected deterministic delegation binding digest, first=%s second=%s", digestA, digestB)
	}

	if _, err := DelegationBindingDigest(schemagate.IntentRequest{}); err == nil {
		t.Fatalf("expected normalize intent error for empty intent")
	}
}

func TestDelegationMintAndReadInputValidation(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()

	_, err = MintDelegationToken(MintDelegationTokenOptions{})
	if err == nil || !strings.Contains(err.Error(), "signing private key is required") {
		t.Fatalf("expected missing key mint validation error, got %v", err)
	}
	_, err = MintDelegationToken(MintDelegationTokenOptions{
		SigningPrivateKey: keyPair.Private,
		TTL:               time.Hour,
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		IntentDigest:      "not_hex",
		TokenPath:         filepath.Join(workDir, "invalid_intent_digest.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "intent_digest must be sha256 hex") {
		t.Fatalf("expected intent digest validation error, got %v", err)
	}
	_, err = MintDelegationToken(MintDelegationTokenOptions{
		SigningPrivateKey: keyPair.Private,
		TTL:               time.Hour,
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		PolicyDigest:      "not_hex",
		TokenPath:         filepath.Join(workDir, "invalid_policy_digest.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "policy_digest must be sha256 hex") {
		t.Fatalf("expected policy digest validation error, got %v", err)
	}

	if _, err := ReadDelegationToken(filepath.Join(workDir, "missing.json")); err == nil {
		t.Fatalf("expected missing delegation token file error")
	}
	malformedPath := filepath.Join(workDir, "malformed.json")
	if writeErr := os.WriteFile(malformedPath, []byte("{"), 0o600); writeErr != nil {
		t.Fatalf("write malformed delegation token: %v", writeErr)
	}
	if _, err := ReadDelegationToken(malformedPath); err == nil || !strings.Contains(err.Error(), "parse delegation token") {
		t.Fatalf("expected delegation token parse error, got %v", err)
	}
}

func TestDelegationAuditRecordBuildAndWrite(t *testing.T) {
	record := BuildDelegationAuditRecord(BuildDelegationAuditOptions{
		CreatedAt:          time.Date(2026, time.February, 11, 0, 0, 0, 0, time.UTC),
		ProducerVersion:    "test",
		TraceID:            "trace_demo",
		ToolName:           "tool.write",
		IntentDigest:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyDigest:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		DelegationRequired: true,
		DelegationRef:      "delegation_ref",
		Entries: []schemagate.DelegationAuditEntry{
			{DelegatorIdentity: "agent.z", DelegateIdentity: "agent.y", TokenID: "token_2", Valid: false},
			{DelegatorIdentity: "agent.a", DelegateIdentity: "agent.b", TokenID: "token_1", Valid: true},
		},
	})
	if record.ValidDelegations != 1 || !record.Delegated {
		t.Fatalf("expected valid delegation count and delegated flag to be set: %#v", record)
	}
	if len(record.Entries) != 2 || record.Entries[0].DelegatorIdentity != "agent.a" {
		t.Fatalf("expected sorted delegation audit entries, got %#v", record.Entries)
	}
	if record.Relationship == nil {
		t.Fatalf("expected relationship envelope in delegation audit record")
	}
	if record.Relationship.ParentRef == nil || record.Relationship.ParentRef.Kind != "trace" || record.Relationship.ParentRef.ID != "trace_demo" {
		t.Fatalf("unexpected relationship parent_ref: %#v", record.Relationship.ParentRef)
	}
	if record.Relationship.PolicyRef == nil || record.Relationship.PolicyRef.PolicyDigest != record.PolicyDigest {
		t.Fatalf("expected policy_ref digest in relationship: %#v", record.Relationship.PolicyRef)
	}
	if len(record.Relationship.Edges) == 0 {
		t.Fatalf("expected relationship edges in delegation audit record")
	}

	workDir := t.TempDir()
	path := filepath.Join(workDir, "audit", "delegation_audit.json")
	if err := WriteDelegationAuditRecord(path, record); err != nil {
		t.Fatalf("write delegation audit record: %v", err)
	}

	parentFile := filepath.Join(workDir, "audit_parent_file")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := WriteDelegationAuditRecord(filepath.Join(parentFile, "record.json"), record); err == nil {
		t.Fatalf("expected write delegation audit record failure when parent is not directory")
	}
}

func assertDelegationErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected delegation token error code %s", code)
	}
	var tokenErr *DelegationTokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("expected DelegationTokenError, got %T", err)
	}
	if tokenErr.Code != code {
		t.Fatalf("unexpected delegation token error code, got %s want %s", tokenErr.Code, code)
	}
}

func TestValidateDelegationTokenSchemaFailuresAndSignatureTamper(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "delegation_schema_failures.json")
	minted, err := MintDelegationToken(MintDelegationTokenOptions{
		ProducerVersion:   "test",
		DelegatorIdentity: "agent.lead",
		DelegateIdentity:  "agent.specialist",
		Scope:             []string{"tool:tool.write"},
		ScopeClass:        "write",
		TTL:               time.Hour,
		Now:               time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
		SigningPrivateKey: keyPair.Private,
		TokenPath:         tokenPath,
	})
	if err != nil {
		t.Fatalf("mint delegation token: %v", err)
	}

	malformedCases := []schemagate.DelegationToken{
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.TokenID = ""
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.DelegatorIdentity = ""
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.Scope = nil
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.IntentDigest = "not_hex"
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.PolicyDigest = "not_hex"
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.CreatedAt = time.Time{}
			return tk
		}(),
		func() schemagate.DelegationToken {
			tk := minted.Token
			tk.ExpiresAt = time.Time{}
			return tk
		}(),
	}
	for _, token := range malformedCases {
		err := ValidateDelegationToken(token, keyPair.Public, DelegationValidationOptions{
			Now: time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
		})
		assertDelegationErrorCode(t, err, DelegationCodeSchemaInvalid)
	}

	tampered := minted.Token
	if tampered.Signature == nil {
		t.Fatalf("minted token signature missing")
	}
	tampered.Signature.Sig = "AAAA"
	err = ValidateDelegationToken(tampered, keyPair.Public, DelegationValidationOptions{
		Now: time.Date(2026, time.February, 10, 0, 30, 0, 0, time.UTC),
	})
	assertDelegationErrorCode(t, err, DelegationCodeSignatureFailed)

	parentFile := filepath.Join(workDir, "token_parent_file")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := WriteDelegationToken(filepath.Join(parentFile, "delegation.json"), minted.Token); err == nil {
		t.Fatalf("expected write delegation token failure when parent is not directory")
	}
}
