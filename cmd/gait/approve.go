package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/sign"
)

type approveOutput struct {
	OK          bool     `json:"ok"`
	TokenID     string   `json:"token_id,omitempty"`
	TokenPath   string   `json:"token_path,omitempty"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
	ReasonCode  string   `json:"reason_code,omitempty"`
	Scope       []string `json:"scope,omitempty"`
	KeyID       string   `json:"key_id,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Error       string   `json:"error,omitempty"`
	Description string   `json:"description,omitempty"`
}

func runApprove(arguments []string) int {
	flagSet := flag.NewFlagSet("approve", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var intentDigest string
	var policyDigest string
	var ttl string
	var scope string
	var approver string
	var reasonCode string
	var outputPath string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&intentDigest, "intent-digest", "", "sha256 hex digest of normalized intent")
	flagSet.StringVar(&policyDigest, "policy-digest", "", "sha256 hex digest of normalized policy")
	flagSet.StringVar(&ttl, "ttl", "", "approval token ttl (for example 1h or 30m)")
	flagSet.StringVar(&scope, "scope", "", "comma-separated approval scope values (for example tool:tool.write)")
	flagSet.StringVar(&approver, "approver", "", "approver identity")
	flagSet.StringVar(&reasonCode, "reason-code", "", "approval reason code")
	flagSet.StringVar(&outputPath, "out", "", "path to emitted approval token (default approval_<token_id>.json)")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printApproveUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	ttlDuration, err := time.ParseDuration(strings.TrimSpace(ttl))
	if err != nil || ttlDuration <= 0 {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: "invalid --ttl, expected positive duration"}, exitInvalidInput)
	}
	scopeValues := parseCSV(scope)
	if len(scopeValues) == 0 {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: "scope is required"}, exitInvalidInput)
	}

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	result, err := gate.MintApprovalToken(gate.MintApprovalTokenOptions{
		ProducerVersion:   version,
		ApproverIdentity:  approver,
		ReasonCode:        reasonCode,
		IntentDigest:      intentDigest,
		PolicyDigest:      policyDigest,
		Scope:             scopeValues,
		TTL:               ttlDuration,
		SigningPrivateKey: keyPair.Private,
		TokenPath:         outputPath,
	})
	if err != nil {
		return writeApproveOutput(jsonOutput, approveOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	keyID := ""
	if result.Token.Signature != nil {
		keyID = result.Token.Signature.KeyID
	}
	return writeApproveOutput(jsonOutput, approveOutput{
		OK:          true,
		TokenID:     result.Token.TokenID,
		TokenPath:   result.TokenPath,
		ExpiresAt:   result.Token.ExpiresAt.UTC().Format(time.RFC3339),
		ReasonCode:  result.Token.ReasonCode,
		Scope:       result.Token.Scope,
		KeyID:       keyID,
		Warnings:    warnings,
		Description: "signed approval token created",
	}, exitOK)
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}

func writeApproveOutput(jsonOutput bool, output approveOutput, exitCode int) int {
	if jsonOutput {
		encoded, err := json.Marshal(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode output"}`)
			return exitInvalidInput
		}
		fmt.Println(string(encoded))
		return exitCode
	}

	if output.OK {
		fmt.Printf("approval token created: %s\n", output.TokenPath)
		return exitCode
	}
	fmt.Printf("approve error: %s\n", output.Error)
	return exitCode
}

func printApproveUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait approve --intent-digest <sha256> --policy-digest <sha256> --ttl <duration> --scope <csv> --approver <identity> --reason-code <code> [--out token.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json]")
}
