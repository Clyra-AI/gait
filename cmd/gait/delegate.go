package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/sign"
)

type delegateOutput struct {
	OK          bool     `json:"ok"`
	TokenID     string   `json:"token_id,omitempty"`
	TokenPath   string   `json:"token_path,omitempty"`
	Delegator   string   `json:"delegator_identity,omitempty"`
	Delegate    string   `json:"delegate_identity,omitempty"`
	Scope       []string `json:"scope,omitempty"`
	ScopeClass  string   `json:"scope_class,omitempty"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
	KeyID       string   `json:"key_id,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	ErrorCode   string   `json:"error_code,omitempty"`
	Error       string   `json:"error,omitempty"`
	Description string   `json:"description,omitempty"`
}

func runDelegate(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Mint and verify signed delegation tokens that bind delegated authority by identity, scope, and optional intent/policy digests.")
	}
	if len(arguments) == 0 {
		printDelegateUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "mint":
		return runDelegateMint(arguments[1:])
	case "verify":
		return runDelegateVerify(arguments[1:])
	default:
		printDelegateUsage()
		return exitInvalidInput
	}
}

func runDelegateMint(arguments []string) int {
	flagSet := flag.NewFlagSet("delegate-mint", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var delegator string
	var delegate string
	var scope string
	var scopeClass string
	var ttl string
	var intentDigest string
	var policyDigest string
	var outputPath string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&delegator, "delegator", "", "delegator identity")
	flagSet.StringVar(&delegate, "delegate", "", "delegate identity")
	flagSet.StringVar(&scope, "scope", "", "comma-separated delegation scope values")
	flagSet.StringVar(&scopeClass, "scope-class", "", "delegation scope class")
	flagSet.StringVar(&ttl, "ttl", "", "delegation token ttl (for example 1h or 30m)")
	flagSet.StringVar(&intentDigest, "intent-digest", "", "optional intent digest binding")
	flagSet.StringVar(&policyDigest, "policy-digest", "", "optional policy digest binding")
	flagSet.StringVar(&outputPath, "out", "", "path to emitted delegation token (default delegation_<token_id>.json)")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDelegateMintUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	ttlDuration, err := time.ParseDuration(strings.TrimSpace(ttl))
	if err != nil || ttlDuration <= 0 {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: "invalid --ttl, expected positive duration"}, exitInvalidInput)
	}
	scopeValues := parseCSV(scope)
	if len(scopeValues) == 0 {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: "scope is required"}, exitInvalidInput)
	}

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := gate.MintDelegationToken(gate.MintDelegationTokenOptions{
		ProducerVersion:   version,
		DelegatorIdentity: delegator,
		DelegateIdentity:  delegate,
		Scope:             scopeValues,
		ScopeClass:        scopeClass,
		IntentDigest:      intentDigest,
		PolicyDigest:      policyDigest,
		TTL:               ttlDuration,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         outputPath,
	})
	if err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	keyID := ""
	if result.Token.Signature != nil {
		keyID = result.Token.Signature.KeyID
	}
	return writeDelegateOutput(jsonOutput, delegateOutput{
		OK:          true,
		TokenID:     result.Token.TokenID,
		TokenPath:   result.TokenPath,
		Delegator:   result.Token.DelegatorIdentity,
		Delegate:    result.Token.DelegateIdentity,
		Scope:       result.Token.Scope,
		ScopeClass:  result.Token.ScopeClass,
		ExpiresAt:   result.Token.ExpiresAt.UTC().Format(time.RFC3339),
		KeyID:       keyID,
		Warnings:    warnings,
		Description: "signed delegation token created",
	}, exitOK)
}

func runDelegateVerify(arguments []string) int {
	flagSet := flag.NewFlagSet("delegate-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var tokenPath string
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var expectedDelegator string
	var expectedDelegate string
	var requiredScope string
	var expectedIntentDigest string
	var expectedPolicyDigest string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&tokenPath, "token", "", "path to delegation token")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 verify key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 verify key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive verify key)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive verify key)")
	flagSet.StringVar(&expectedDelegator, "delegator", "", "expected delegator identity")
	flagSet.StringVar(&expectedDelegate, "delegate", "", "expected delegate identity")
	flagSet.StringVar(&requiredScope, "scope", "", "required scope csv")
	flagSet.StringVar(&expectedIntentDigest, "intent-digest", "", "expected intent digest")
	flagSet.StringVar(&expectedPolicyDigest, "policy-digest", "", "expected policy digest")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printDelegateVerifyUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(tokenPath) == "" {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: "--token is required"}, exitInvalidInput)
	}

	verifyKey, err := sign.LoadVerifyKey(sign.KeyConfig{
		PublicKeyPath:  strings.TrimSpace(publicKeyPath),
		PublicKeyEnv:   strings.TrimSpace(publicKeyEnv),
		PrivateKeyPath: strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
	})
	if err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	token, err := gate.ReadDelegationToken(tokenPath)
	if err != nil {
		return writeDelegateOutput(jsonOutput, delegateOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if err := gate.ValidateDelegationToken(token, verifyKey, gate.DelegationValidationOptions{
		ExpectedDelegator:    expectedDelegator,
		ExpectedDelegate:     expectedDelegate,
		RequiredScope:        parseCSV(requiredScope),
		ExpectedIntentDigest: expectedIntentDigest,
		ExpectedPolicyDigest: expectedPolicyDigest,
	}); err != nil {
		errorCode := gate.DelegationCodeSchemaInvalid
		var tokenErr *gate.DelegationTokenError
		if errors.As(err, &tokenErr) && tokenErr.Code != "" {
			errorCode = tokenErr.Code
		}
		return writeDelegateOutput(jsonOutput, delegateOutput{
			OK:        false,
			TokenID:   token.TokenID,
			TokenPath: tokenPath,
			ErrorCode: errorCode,
			Error:     err.Error(),
		}, exitVerifyFailed)
	}

	keyID := ""
	if token.Signature != nil {
		keyID = token.Signature.KeyID
	}
	return writeDelegateOutput(jsonOutput, delegateOutput{
		OK:          true,
		TokenID:     token.TokenID,
		TokenPath:   tokenPath,
		Delegator:   token.DelegatorIdentity,
		Delegate:    token.DelegateIdentity,
		Scope:       token.Scope,
		ScopeClass:  token.ScopeClass,
		ExpiresAt:   token.ExpiresAt.UTC().Format(time.RFC3339),
		KeyID:       keyID,
		Description: "delegation token verified",
	}, exitOK)
}

func writeDelegateOutput(jsonOutput bool, output delegateOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("delegation token ok: %s\n", output.TokenPath)
		return exitCode
	}
	fmt.Printf("delegate error: %s\n", output.Error)
	return exitCode
}

func printDelegateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait delegate mint --delegator <identity> --delegate <identity> --scope <csv> --ttl <duration> [--scope-class <value>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--out token.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait delegate verify --token <token.json> [--delegator <identity>] [--delegate <identity>] [--scope <csv>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--public-key <path>|--public-key-env <VAR>|--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printDelegateMintUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait delegate mint --delegator <identity> --delegate <identity> --scope <csv> --ttl <duration> [--scope-class <value>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--out token.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printDelegateVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait delegate verify --token <token.json> [--delegator <identity>] [--delegate <identity>] [--scope <csv>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--public-key <path>|--public-key-env <VAR>|--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}
