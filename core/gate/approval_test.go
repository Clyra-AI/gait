package gate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
)

func TestMintApprovalTokenAndValidate(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workDir := t.TempDir()
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)
	tokenPath := filepath.Join(workDir, "approval.json")
	result, err := MintApprovalToken(MintApprovalTokenOptions{
		ProducerVersion:   "test",
		ApproverIdentity:  "alice",
		ReasonCode:        "incident_hotfix",
		IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		Scope:             []string{"tool:tool.write"},
		TTL:               30 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         tokenPath,
	})
	if err != nil {
		t.Fatalf("mint approval token: %v", err)
	}
	if result.TokenPath != tokenPath {
		t.Fatalf("unexpected token path: %s", result.TokenPath)
	}
	if result.Token.TokenID == "" || result.Token.Signature == nil {
		t.Fatalf("expected token id and signature: %#v", result.Token)
	}

	loaded, err := ReadApprovalToken(tokenPath)
	if err != nil {
		t.Fatalf("read approval token: %v", err)
	}
	if loaded.TokenID != result.Token.TokenID {
		t.Fatalf("unexpected loaded token id: %s", loaded.TokenID)
	}

	err = ValidateApprovalToken(loaded, keyPair.Public, ApprovalValidationOptions{
		Now:                  now.Add(10 * time.Minute),
		ExpectedIntentDigest: result.Token.IntentDigest,
		ExpectedPolicyDigest: result.Token.PolicyDigest,
		RequiredScope:        []string{"tool:tool.write"},
	})
	if err != nil {
		t.Fatalf("validate approval token: %v", err)
	}
}

func TestValidateApprovalTokenFailureCodes(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)
	workDir := t.TempDir()
	baseResult, err := MintApprovalToken(MintApprovalTokenOptions{
		ProducerVersion:   "test",
		ApproverIdentity:  "alice",
		ReasonCode:        "incident_hotfix",
		IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		Scope:             []string{"tool:tool.write"},
		TTL:               30 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(workDir, "approval.json"),
	})
	if err != nil {
		t.Fatalf("mint approval token: %v", err)
	}

	tests := []struct {
		name         string
		token        func() schemagate.ApprovalToken
		opts         ApprovalValidationOptions
		expectedCode string
	}{
		{
			name: "signature_missing",
			token: func() schemagate.ApprovalToken {
				token := baseResult.Token
				token.Signature = nil
				return token
			},
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodeSignatureMiss,
		},
		{
			name: "signature_invalid",
			token: func() schemagate.ApprovalToken {
				token := baseResult.Token
				token.PolicyDigest = "3333333333333333333333333333333333333333333333333333333333333333"
				return token
			},
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: "3333333333333333333333333333333333333333333333333333333333333333",
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodeSignatureFailed,
		},
		{
			name:  "intent_mismatch",
			token: func() schemagate.ApprovalToken { return baseResult.Token },
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodeIntentMismatch,
		},
		{
			name:  "policy_mismatch",
			token: func() schemagate.ApprovalToken { return baseResult.Token },
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodePolicyMismatch,
		},
		{
			name:  "scope_mismatch",
			token: func() schemagate.ApprovalToken { return baseResult.Token },
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
				RequiredScope:        []string{"tool:tool.delete"},
			},
			expectedCode: ApprovalCodeScopeMismatch,
		},
		{
			name:  "expired",
			token: func() schemagate.ApprovalToken { return baseResult.Token },
			opts: ApprovalValidationOptions{
				Now:                  baseResult.Token.ExpiresAt,
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodeExpired,
		},
		{
			name: "schema_invalid",
			token: func() schemagate.ApprovalToken {
				token := baseResult.Token
				token.TokenID = ""
				return token
			},
			opts: ApprovalValidationOptions{
				Now:                  now.Add(1 * time.Minute),
				ExpectedIntentDigest: baseResult.Token.IntentDigest,
				ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
				RequiredScope:        []string{"tool:tool.write"},
			},
			expectedCode: ApprovalCodeSchemaInvalid,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateApprovalToken(testCase.token(), keyPair.Public, testCase.opts)
			if err == nil {
				t.Fatalf("expected validation failure")
			}
			var tokenErr *ApprovalTokenError
			if !errors.As(err, &tokenErr) {
				t.Fatalf("expected approval token error, got: %v", err)
			}
			if tokenErr.Code != testCase.expectedCode {
				t.Fatalf("unexpected error code: got=%s want=%s err=%v", tokenErr.Code, testCase.expectedCode, err)
			}
		})
	}
}

func TestValidateApprovalTokenMaxTargetsAndOps(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)
	workDir := t.TempDir()
	baseResult, err := MintApprovalToken(MintApprovalTokenOptions{
		ProducerVersion:   "test",
		ApproverIdentity:  "alice",
		ReasonCode:        "incident_hotfix",
		IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		Scope:             []string{"tool:tool.write"},
		MaxTargets:        2,
		MaxOps:            3,
		TTL:               30 * time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(workDir, "approval.json"),
	})
	if err != nil {
		t.Fatalf("mint approval token: %v", err)
	}

	if err := ValidateApprovalToken(baseResult.Token, keyPair.Public, ApprovalValidationOptions{
		Now:                  now.Add(time.Minute),
		ExpectedIntentDigest: baseResult.Token.IntentDigest,
		ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
		RequiredScope:        []string{"tool:tool.write"},
		TargetCount:          2,
		OperationCount:       3,
	}); err != nil {
		t.Fatalf("expected bounded approval token validation to pass: %v", err)
	}
	if err := ValidateApprovalToken(baseResult.Token, keyPair.Public, ApprovalValidationOptions{
		Now:                  now.Add(time.Minute),
		ExpectedIntentDigest: baseResult.Token.IntentDigest,
		ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
		RequiredScope:        []string{"tool:tool.write"},
		TargetCount:          3,
		OperationCount:       3,
	}); err == nil {
		t.Fatalf("expected max_targets validation failure")
	} else {
		var tokenErr *ApprovalTokenError
		if !errors.As(err, &tokenErr) || tokenErr.Code != ApprovalCodeTargetsExceeded {
			t.Fatalf("expected max_targets error code %q, got %v", ApprovalCodeTargetsExceeded, err)
		}
	}
	if err := ValidateApprovalToken(baseResult.Token, keyPair.Public, ApprovalValidationOptions{
		Now:                  now.Add(time.Minute),
		ExpectedIntentDigest: baseResult.Token.IntentDigest,
		ExpectedPolicyDigest: baseResult.Token.PolicyDigest,
		RequiredScope:        []string{"tool:tool.write"},
		TargetCount:          2,
		OperationCount:       4,
	}); err == nil {
		t.Fatalf("expected max_ops validation failure")
	} else {
		var tokenErr *ApprovalTokenError
		if !errors.As(err, &tokenErr) || tokenErr.Code != ApprovalCodeOpsExceeded {
			t.Fatalf("expected max_ops error code %q, got %v", ApprovalCodeOpsExceeded, err)
		}
	}
}

func TestApprovalContext(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`default_verdict: require_approval`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "TOOL.WRITE"

	policyDigest, intentDigest, scope, err := ApprovalContext(policy, intent)
	if err != nil {
		t.Fatalf("approval context: %v", err)
	}
	if policyDigest == "" || intentDigest == "" {
		t.Fatalf("expected non-empty digests: policy=%s intent=%s", policyDigest, intentDigest)
	}
	expectedScope := []string{"tool:tool.write"}
	if !reflect.DeepEqual(scope, expectedScope) {
		t.Fatalf("unexpected approval scope: got=%#v want=%#v", scope, expectedScope)
	}
}

func TestApprovalTokenErrorHelpers(t *testing.T) {
	baseErr := errors.New("base")
	tokenErr := &ApprovalTokenError{Code: ApprovalCodeExpired, Err: baseErr}
	if got := tokenErr.Error(); got != "approval_token_expired: base" {
		t.Fatalf("unexpected error text: %s", got)
	}
	if !errors.Is(tokenErr, baseErr) {
		t.Fatalf("expected wrapped base error")
	}

	codeOnly := &ApprovalTokenError{Code: ApprovalCodeScopeMismatch}
	if got := codeOnly.Error(); got != ApprovalCodeScopeMismatch {
		t.Fatalf("unexpected code-only error text: %s", got)
	}
}

func TestApprovalContextDestructiveApplyScope(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`default_verdict: require_approval`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "tool.delete"
	intent.Context.Phase = "apply"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "path", Value: "/tmp/demo.txt", Operation: "delete"},
	}
	_, _, scope, err := ApprovalContext(policy, intent)
	if err != nil {
		t.Fatalf("approval context: %v", err)
	}
	if !reflect.DeepEqual(scope, []string{"destructive:apply", "phase:apply", "tool:tool.delete"}) {
		t.Fatalf("unexpected destructive apply scope: %#v", scope)
	}
}

func TestMintApprovalTokenInputValidation(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	tests := []struct {
		name string
		opts MintApprovalTokenOptions
	}{
		{
			name: "missing_signing_key",
			opts: MintApprovalTokenOptions{
				ApproverIdentity: "alice",
				ReasonCode:       "rc",
				IntentDigest:     "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:     "2222222222222222222222222222222222222222222222222222222222222222",
				Scope:            []string{"tool:tool.write"},
				TTL:              time.Minute,
			},
		},
		{
			name: "invalid_ttl",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ApproverIdentity:  "alice",
				ReasonCode:        "rc",
				IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
				Scope:             []string{"tool:tool.write"},
				TTL:               0,
			},
		},
		{
			name: "invalid_intent_digest",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ApproverIdentity:  "alice",
				ReasonCode:        "rc",
				IntentDigest:      "bad",
				PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
				Scope:             []string{"tool:tool.write"},
				TTL:               time.Minute,
			},
		},
		{
			name: "invalid_policy_digest",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ApproverIdentity:  "alice",
				ReasonCode:        "rc",
				IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:      "bad",
				Scope:             []string{"tool:tool.write"},
				TTL:               time.Minute,
			},
		},
		{
			name: "missing_approver",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ReasonCode:        "rc",
				IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
				Scope:             []string{"tool:tool.write"},
				TTL:               time.Minute,
			},
		},
		{
			name: "missing_reason_code",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ApproverIdentity:  "alice",
				IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
				Scope:             []string{"tool:tool.write"},
				TTL:               time.Minute,
			},
		},
		{
			name: "missing_scope",
			opts: MintApprovalTokenOptions{
				SigningPrivateKey: keyPair.Private,
				ApproverIdentity:  "alice",
				ReasonCode:        "rc",
				IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
				PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
				TTL:               time.Minute,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := MintApprovalToken(testCase.opts); err == nil {
				t.Fatalf("expected mint validation to fail")
			}
		})
	}
}

func TestApprovalTokenReadWriteErrorsAndScopeMatching(t *testing.T) {
	workDir := t.TempDir()
	tokenPath := filepath.Join(workDir, "approval.json")
	token := schemagate.ApprovalToken{
		SchemaID:         approvalTokenSchemaID,
		SchemaVersion:    approvalTokenSchemaV1,
		CreatedAt:        time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion:  "test",
		TokenID:          "token_demo",
		ApproverIdentity: "alice",
		ReasonCode:       "rc",
		IntentDigest:     "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:     "2222222222222222222222222222222222222222222222222222222222222222",
		Scope:            []string{"tool:tool.write", "*"},
		ExpiresAt:        time.Date(2026, time.February, 5, 1, 0, 0, 0, time.UTC),
	}

	if err := WriteApprovalToken(tokenPath, token); err != nil {
		t.Fatalf("write approval token: %v", err)
	}
	readBack, err := ReadApprovalToken(tokenPath)
	if err != nil {
		t.Fatalf("read approval token: %v", err)
	}
	if readBack.TokenID != token.TokenID {
		t.Fatalf("unexpected read token: %#v", readBack)
	}
	if !matchesApprovalScope([]string{"tool:tool.delete"}, readBack.Scope) {
		t.Fatalf("expected wildcard scope to match")
	}

	if _, err := ReadApprovalToken(filepath.Join(workDir, "missing.json")); err == nil {
		t.Fatalf("expected missing file read error")
	}

	invalidPath := filepath.Join(workDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid token file: %v", err)
	}
	if _, err := ReadApprovalToken(invalidPath); err == nil {
		t.Fatalf("expected invalid json read failure")
	}
}

func TestValidateApprovalTokenMissingVerifyKeyAndInvalidContext(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)
	workDir := t.TempDir()
	result, err := MintApprovalToken(MintApprovalTokenOptions{
		ProducerVersion:   "test",
		ApproverIdentity:  "alice",
		ReasonCode:        "incident_hotfix",
		IntentDigest:      "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyDigest:      "2222222222222222222222222222222222222222222222222222222222222222",
		Scope:             []string{"tool:tool.write"},
		TTL:               time.Minute,
		Now:               now,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         filepath.Join(workDir, "approval.json"),
	})
	if err != nil {
		t.Fatalf("mint approval token: %v", err)
	}

	err = ValidateApprovalToken(result.Token, nil, ApprovalValidationOptions{
		Now:                  now.Add(30 * time.Second),
		ExpectedIntentDigest: result.Token.IntentDigest,
		ExpectedPolicyDigest: result.Token.PolicyDigest,
		RequiredScope:        []string{"tool:tool.write"},
	})
	var tokenErr *ApprovalTokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("expected approval token error, got: %v", err)
	}
	if tokenErr.Code != ApprovalCodeSignatureFailed {
		t.Fatalf("unexpected missing-key error code: %s", tokenErr.Code)
	}

	if _, _, _, err := ApprovalContext(Policy{SchemaID: "bad"}, baseIntent()); err == nil {
		t.Fatalf("expected approval context to fail for invalid policy")
	}
}
