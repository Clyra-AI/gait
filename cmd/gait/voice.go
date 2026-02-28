package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemavoice "github.com/Clyra-AI/gait/core/schema/v1/voice"
	sign "github.com/Clyra-AI/proof/signing"
)

type voiceTokenOutput struct {
	OK              bool     `json:"ok"`
	Operation       string   `json:"operation,omitempty"`
	Verdict         string   `json:"verdict,omitempty"`
	ReasonCodes     []string `json:"reason_codes,omitempty"`
	Violations      []string `json:"violations,omitempty"`
	TokenID         string   `json:"token_id,omitempty"`
	TokenPath       string   `json:"token_path,omitempty"`
	CallID          string   `json:"call_id,omitempty"`
	TurnIndex       int      `json:"turn_index,omitempty"`
	CallSeq         int      `json:"call_seq,omitempty"`
	CommitmentClass string   `json:"commitment_class,omitempty"`
	TraceID         string   `json:"trace_id,omitempty"`
	TracePath       string   `json:"trace_path,omitempty"`
	PolicyDigest    string   `json:"policy_digest,omitempty"`
	IntentDigest    string   `json:"intent_digest,omitempty"`
	ErrorCode       string   `json:"error_code,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Error           string   `json:"error,omitempty"`
}

func runVoice(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Enforce pre-utterance commitment gating and emit signed callpacks and say tokens for voice agent boundaries.")
	}
	if len(arguments) == 0 {
		printVoiceUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "pack":
		return runVoicePack(arguments[1:])
	case "token":
		return runVoiceToken(arguments[1:])
	default:
		printVoiceUsage()
		return exitInvalidInput
	}
}

func runVoicePack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build, inspect, verify, and diff voice callpacks on the existing PackSpec pipeline.")
	}
	if len(arguments) == 0 {
		printVoicePackUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "build":
		args := append([]string{"--type", "call"}, arguments[1:]...)
		return runPackBuild(args)
	case "verify":
		return runPackVerify(arguments[1:])
	case "inspect":
		return runPackInspect(arguments[1:])
	case "diff":
		return runPackDiff(arguments[1:])
	default:
		printVoicePackUsage()
		return exitInvalidInput
	}
}

func runVoiceToken(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Mint and verify signed SayToken capability tokens bound to commitment intent, policy digest, and call coordinates.")
	}
	if len(arguments) == 0 {
		printVoiceTokenUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "mint":
		return runVoiceTokenMint(arguments[1:])
	case "verify":
		return runVoiceTokenVerify(arguments[1:])
	default:
		printVoiceTokenUsage()
		return exitInvalidInput
	}
}

func runVoiceTokenMint(arguments []string) int {
	flagSet := flag.NewFlagSet("voice-token-mint", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var intentPath string
	var policyPath string
	var ttl string
	var outPath string
	var tracePath string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&intentPath, "intent", "", "path to voice commitment intent json")
	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml")
	flagSet.StringVar(&ttl, "ttl", "2m", "say token ttl (for example 2m or 30s)")
	flagSet.StringVar(&outPath, "out", "", "path to emitted say token")
	flagSet.StringVar(&tracePath, "trace-out", "", "path to emitted gate trace JSON")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printVoiceTokenMintUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(intentPath) == "" || strings.TrimSpace(policyPath) == "" {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: "both --intent and --policy are required"}, exitInvalidInput)
	}
	ttlDuration, err := time.ParseDuration(strings.TrimSpace(ttl))
	if err != nil || ttlDuration <= 0 {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: "invalid --ttl, expected positive duration"}, exitInvalidInput)
	}

	intent, err := readVoiceCommitmentIntent(intentPath)
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	normalizedCommitment, err := gate.NormalizeCommitmentIntent(intent)
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	normalizedIntent, err := gate.CommitmentIntentToIntent(normalizedCommitment)
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	outcome, err := gate.EvaluatePolicyDetailed(policy, normalizedIntent, gate.EvalOptions{ProducerVersion: version})
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	policyDigest, intentDigest, _, err := gate.ApprovalContext(policy, normalizedIntent)
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
	})
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	traceResult, traceErr := gate.EmitSignedTrace(policy, normalizedIntent, outcome.Result, gate.EmitTraceOptions{
		ProducerVersion:    version,
		CorrelationID:      currentCorrelationID(),
		ContextSource:      outcome.ContextSource,
		CompositeRiskClass: outcome.CompositeRiskClass,
		StepVerdicts:       outcome.StepVerdicts,
		PreApproved:        outcome.PreApproved,
		PatternID:          outcome.PatternID,
		RegistryReason:     outcome.RegistryReason,
		SigningPrivateKey:  keyPair.Private,
		TracePath:          strings.TrimSpace(tracePath),
	})
	if traceErr != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: traceErr.Error()}, exitCodeForError(traceErr, exitInvalidInput))
	}
	if outcome.Result.Verdict != "allow" {
		exitCode := exitPolicyBlocked
		if outcome.Result.Verdict == "require_approval" {
			exitCode = exitApprovalRequired
		}
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{
			OK:              false,
			Operation:       "mint",
			Verdict:         outcome.Result.Verdict,
			ReasonCodes:     outcome.Result.ReasonCodes,
			Violations:      outcome.Result.Violations,
			TraceID:         traceResult.Trace.TraceID,
			TracePath:       traceResult.TracePath,
			PolicyDigest:    policyDigest,
			IntentDigest:    intentDigest,
			CallID:          normalizedCommitment.CallID,
			TurnIndex:       normalizedCommitment.TurnIndex,
			CallSeq:         normalizedCommitment.CallSeq,
			CommitmentClass: normalizedCommitment.CommitmentClass,
			Error:           "commitment intent was not allowed by policy",
		}, exitCode)
	}
	tokenResult, err := gate.MintSayToken(gate.MintSayTokenOptions{
		ProducerVersion:    version,
		CommitmentClass:    normalizedCommitment.CommitmentClass,
		IntentDigest:       intentDigest,
		PolicyDigest:       policyDigest,
		CallID:             normalizedCommitment.CallID,
		TurnIndex:          normalizedCommitment.TurnIndex,
		CallSeq:            normalizedCommitment.CallSeq,
		Currency:           normalizedCommitment.Currency,
		QuoteMinCents:      normalizedCommitment.QuoteMinCents,
		QuoteMaxCents:      normalizedCommitment.QuoteMaxCents,
		RefundCeilingCents: normalizedCommitment.RefundCeilingCents,
		TTL:                ttlDuration,
		SigningPrivateKey:  keyPair.Private,
		TokenPath:          strings.TrimSpace(outPath),
	})
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "mint", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{
		OK:              true,
		Operation:       "mint",
		Verdict:         outcome.Result.Verdict,
		ReasonCodes:     outcome.Result.ReasonCodes,
		Violations:      outcome.Result.Violations,
		TokenID:         tokenResult.Token.TokenID,
		TokenPath:       tokenResult.TokenPath,
		CallID:          tokenResult.Token.CallID,
		TurnIndex:       tokenResult.Token.TurnIndex,
		CallSeq:         tokenResult.Token.CallSeq,
		CommitmentClass: tokenResult.Token.CommitmentClass,
		TraceID:         traceResult.Trace.TraceID,
		TracePath:       traceResult.TracePath,
		PolicyDigest:    policyDigest,
		IntentDigest:    intentDigest,
		Warnings:        warnings,
	}, exitOK)
}

func runVoiceTokenVerify(arguments []string) int {
	flagSet := flag.NewFlagSet("voice-token-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var tokenPath string
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var expectedIntentDigest string
	var expectedPolicyDigest string
	var expectedCallID string
	var expectedTurnIndex int
	var expectedCallSeq int
	var expectedClass string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&tokenPath, "token", "", "path to say token")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 verify key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 verify key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive verify key)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive verify key)")
	flagSet.StringVar(&expectedIntentDigest, "intent-digest", "", "expected intent digest")
	flagSet.StringVar(&expectedPolicyDigest, "policy-digest", "", "expected policy digest")
	flagSet.StringVar(&expectedCallID, "call-id", "", "expected call_id binding")
	flagSet.IntVar(&expectedTurnIndex, "turn-index", -1, "expected turn_index binding (>=0 enables check)")
	flagSet.IntVar(&expectedCallSeq, "call-seq", 0, "expected call_seq binding (>0 enables check)")
	flagSet.StringVar(&expectedClass, "commitment-class", "", "expected commitment class binding")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printVoiceTokenVerifyUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "verify", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(tokenPath) == "" {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "verify", Error: "--token is required"}, exitInvalidInput)
	}
	verifyKey, err := sign.LoadVerifyKey(sign.KeyConfig{
		PublicKeyPath:  strings.TrimSpace(publicKeyPath),
		PublicKeyEnv:   strings.TrimSpace(publicKeyEnv),
		PrivateKeyPath: strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
	})
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	token, err := gate.ReadSayToken(strings.TrimSpace(tokenPath))
	if err != nil {
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if err := gate.ValidateSayToken(token, verifyKey, gate.SayTokenValidationOptions{
		ExpectedIntentDigest:    expectedIntentDigest,
		ExpectedPolicyDigest:    expectedPolicyDigest,
		ExpectedCallID:          expectedCallID,
		ExpectedTurnIndex:       expectedTurnIndex,
		ExpectedCallSeq:         expectedCallSeq,
		ExpectedCommitmentClass: expectedClass,
	}); err != nil {
		errorCode := gate.SayTokenCodeSchemaInvalid
		var tokenErr *gate.SayTokenError
		if errors.As(err, &tokenErr) && tokenErr.Code != "" {
			errorCode = tokenErr.Code
		}
		return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{
			OK:              false,
			Operation:       "verify",
			TokenID:         token.TokenID,
			TokenPath:       strings.TrimSpace(tokenPath),
			CallID:          token.CallID,
			TurnIndex:       token.TurnIndex,
			CallSeq:         token.CallSeq,
			CommitmentClass: token.CommitmentClass,
			ErrorCode:       errorCode,
			Error:           err.Error(),
		}, exitVerifyFailed)
	}
	return writeVoiceTokenOutput(jsonOutput, voiceTokenOutput{
		OK:              true,
		Operation:       "verify",
		TokenID:         token.TokenID,
		TokenPath:       strings.TrimSpace(tokenPath),
		CallID:          token.CallID,
		TurnIndex:       token.TurnIndex,
		CallSeq:         token.CallSeq,
		CommitmentClass: token.CommitmentClass,
		PolicyDigest:    token.PolicyDigest,
		IntentDigest:    token.IntentDigest,
	}, exitOK)
}

func writeVoiceTokenOutput(jsonOutput bool, output voiceTokenOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("voice token %s ok: %s\n", output.Operation, output.TokenPath)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("voice token %s error: %s\n", output.Operation, output.Error)
	}
	return exitCode
}

func readVoiceCommitmentIntent(path string) (schemavoice.CommitmentIntent, error) {
	// #nosec G304 -- intent path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("read voice intent: %w", err)
	}
	var intent schemavoice.CommitmentIntent
	if err := json.Unmarshal(content, &intent); err != nil {
		return schemavoice.CommitmentIntent{}, fmt.Errorf("parse voice intent json: %w", err)
	}
	return intent, nil
}

func printVoiceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait voice pack build --from <call_record.json> [--out <callpack.zip>] [--key-mode none|dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait voice pack verify <callpack.zip> [--profile standard|strict] [--json] [--explain]")
	fmt.Println("  gait voice pack inspect <callpack.zip> [--json] [--explain]")
	fmt.Println("  gait voice pack diff <left.zip> <right.zip> [--json] [--explain]")
	fmt.Println("  gait voice token mint --intent <commitment_intent.json> --policy <policy.yaml> [--ttl <duration>] [--out <say_token.json>] [--trace-out <trace.json>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait voice token verify --token <say_token.json> [--intent-digest <sha256>] [--policy-digest <sha256>] [--call-id <id>] [--turn-index <n>] [--call-seq <n>] [--commitment-class <class>] [--public-key <path>|--public-key-env <VAR>|--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printVoicePackUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait voice pack build --from <call_record.json> [--out <callpack.zip>] [--key-mode none|dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait voice pack verify <callpack.zip> [--profile standard|strict] [--json] [--explain]")
	fmt.Println("  gait voice pack inspect <callpack.zip> [--json] [--explain]")
	fmt.Println("  gait voice pack diff <left.zip> <right.zip> [--json] [--explain]")
}

func printVoiceTokenUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait voice token mint --intent <commitment_intent.json> --policy <policy.yaml> [--ttl <duration>] [--out <say_token.json>] [--trace-out <trace.json>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait voice token verify --token <say_token.json> [--intent-digest <sha256>] [--policy-digest <sha256>] [--call-id <id>] [--turn-index <n>] [--call-seq <n>] [--commitment-class <class>] [--public-key <path>|--public-key-env <VAR>|--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printVoiceTokenMintUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait voice token mint --intent <commitment_intent.json> --policy <policy.yaml> [--ttl <duration>] [--out <say_token.json>] [--trace-out <trace.json>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printVoiceTokenVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait voice token verify --token <say_token.json> [--intent-digest <sha256>] [--policy-digest <sha256>] [--call-id <id>] [--turn-index <n>] [--call-seq <n>] [--commitment-class <class>] [--public-key <path>|--public-key-env <VAR>|--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}
