package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/gate"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	"github.com/davidahmann/gait/core/sign"
)

type gateEvalOutput struct {
	OK           bool     `json:"ok"`
	Verdict      string   `json:"verdict,omitempty"`
	ReasonCodes  []string `json:"reason_codes,omitempty"`
	Violations   []string `json:"violations,omitempty"`
	ApprovalRef  string   `json:"approval_ref,omitempty"`
	TraceID      string   `json:"trace_id,omitempty"`
	TracePath    string   `json:"trace_path,omitempty"`
	PolicyDigest string   `json:"policy_digest,omitempty"`
	IntentDigest string   `json:"intent_digest,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func runGate(arguments []string) int {
	if len(arguments) == 0 {
		printGateUsage()
		return exitInvalidInput
	}

	switch arguments[0] {
	case "eval":
		return runGateEval(arguments[1:])
	default:
		printGateUsage()
		return exitInvalidInput
	}
}

func runGateEval(arguments []string) int {
	flagSet := flag.NewFlagSet("gate-eval", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var intentPath string
	var tracePath string
	var approvalTokenRef string
	var approvalTokenPath string
	var approvalPublicKeyPath string
	var approvalPublicKeyEnv string
	var approvalPrivateKeyPath string
	var approvalPrivateKeyEnv string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml")
	flagSet.StringVar(&intentPath, "intent", "", "path to intent request json")
	flagSet.StringVar(&tracePath, "trace-out", "", "path to emitted trace JSON (default trace_<trace_id>.json)")
	flagSet.StringVar(&approvalTokenRef, "approval-token-ref", "", "optional approval token reference")
	flagSet.StringVar(&approvalTokenPath, "approval-token", "", "path to signed approval token")
	flagSet.StringVar(&approvalPublicKeyPath, "approval-public-key", "", "path to base64 approval verify key")
	flagSet.StringVar(&approvalPublicKeyEnv, "approval-public-key-env", "", "env var containing base64 approval verify key")
	flagSet.StringVar(&approvalPrivateKeyPath, "approval-private-key", "", "path to base64 approval private key (derive public)")
	flagSet.StringVar(&approvalPrivateKeyEnv, "approval-private-key-env", "", "env var containing base64 approval private key (derive public)")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printGateEvalUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if policyPath == "" || intentPath == "" {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: "both --policy and --intent are required"}, exitInvalidInput)
	}

	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	intent, err := readIntentRequest(intentPath)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	evalStart := time.Now()
	result, err := gate.EvaluatePolicy(policy, intent, gate.EvalOptions{ProducerVersion: version})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	evalLatencyMS := time.Since(evalStart).Seconds() * 1000

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	exitCode := exitOK
	resolvedApprovalRef := strings.TrimSpace(approvalTokenRef)
	if result.Verdict == "require_approval" {
		policyDigest, intentDigest, requiredScope, err := gate.ApprovalContext(policy, intent)
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
		}
		if strings.TrimSpace(approvalTokenPath) == "" {
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonMissingToken})
			exitCode = exitApprovalRequired
		} else {
			token, err := gate.ReadApprovalToken(approvalTokenPath)
			if err != nil {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
			}
			if token.TokenID != "" {
				resolvedApprovalRef = token.TokenID
			}

			verifyKey := keyPair.Public
			verifyConfig := sign.KeyConfig{
				PublicKeyPath:  approvalPublicKeyPath,
				PublicKeyEnv:   approvalPublicKeyEnv,
				PrivateKeyPath: approvalPrivateKeyPath,
				PrivateKeyEnv:  approvalPrivateKeyEnv,
			}
			if hasAnyKeySource(verifyConfig) {
				verifyKey, err = sign.LoadVerifyKey(verifyConfig)
				if err != nil {
					return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
				}
			}

			err = gate.ValidateApprovalToken(token, verifyKey, gate.ApprovalValidationOptions{
				Now:                  time.Now().UTC(),
				ExpectedIntentDigest: intentDigest,
				ExpectedPolicyDigest: policyDigest,
				RequiredScope:        requiredScope,
			})
			if err != nil {
				reasonCode := gate.ApprovalCodeSchemaInvalid
				var tokenErr *gate.ApprovalTokenError
				if errors.As(err, &tokenErr) && tokenErr.Code != "" {
					reasonCode = tokenErr.Code
				}
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{reasonCode})
				result.Violations = mergeUniqueSorted(result.Violations, []string{"approval_not_granted"})
				exitCode = exitApprovalRequired
			} else {
				result.Verdict = "allow"
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonGranted})
			}
		}
	}

	traceResult, err := gate.EmitSignedTrace(policy, intent, result, gate.EmitTraceOptions{
		ProducerVersion:   version,
		ApprovalTokenRef:  resolvedApprovalRef,
		LatencyMS:         evalLatencyMS,
		SigningPrivateKey: keyPair.Private,
		TracePath:         tracePath,
	})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	return writeGateEvalOutput(jsonOutput, gateEvalOutput{
		OK:           true,
		Verdict:      result.Verdict,
		ReasonCodes:  result.ReasonCodes,
		Violations:   result.Violations,
		ApprovalRef:  resolvedApprovalRef,
		TraceID:      traceResult.Trace.TraceID,
		TracePath:    traceResult.TracePath,
		PolicyDigest: traceResult.PolicyDigest,
		IntentDigest: traceResult.IntentDigest,
		Warnings:     warnings,
	}, exitCode)
}

func readIntentRequest(path string) (schemagate.IntentRequest, error) {
	// #nosec G304 -- intent path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemagate.IntentRequest{}, fmt.Errorf("read intent: %w", err)
	}
	var intent schemagate.IntentRequest
	if err := json.Unmarshal(content, &intent); err != nil {
		return schemagate.IntentRequest{}, fmt.Errorf("parse intent json: %w", err)
	}
	return intent, nil
}

func writeGateEvalOutput(jsonOutput bool, output gateEvalOutput, exitCode int) int {
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
		fmt.Printf("gate eval: verdict=%s\n", output.Verdict)
		fmt.Printf("trace: %s\n", output.TracePath)
		if len(output.ReasonCodes) > 0 {
			fmt.Printf("reasons: %s\n", joinCSV(output.ReasonCodes))
		}
		if len(output.Violations) > 0 {
			fmt.Printf("violations: %s\n", joinCSV(output.Violations))
		}
		for _, warning := range output.Warnings {
			fmt.Printf("warning: %s\n", warning)
		}
		return exitCode
	}
	fmt.Printf("gate eval error: %s\n", output.Error)
	return exitCode
}

func printGateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--approval-token <token.json>] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json]")
}

func printGateEvalUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--approval-token <token.json>] [--approval-token-ref token] [--approval-public-key <path>|--approval-public-key-env <VAR>] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json]")
}

func joinCSV(values []string) string {
	return strings.Join(values, ",")
}

func mergeUniqueSorted(current []string, extra []string) []string {
	merged := make([]string, 0, len(current)+len(extra))
	merged = append(merged, current...)
	merged = append(merged, extra...)
	seen := make(map[string]struct{}, len(merged))
	out := make([]string, 0, len(merged))
	for _, value := range merged {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
