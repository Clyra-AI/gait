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

	"github.com/davidahmann/gait/core/credential"
	"github.com/davidahmann/gait/core/gate"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	"github.com/davidahmann/gait/core/sign"
)

type gateEvalOutput struct {
	OK                     bool     `json:"ok"`
	Profile                string   `json:"profile,omitempty"`
	Verdict                string   `json:"verdict,omitempty"`
	ReasonCodes            []string `json:"reason_codes,omitempty"`
	Violations             []string `json:"violations,omitempty"`
	ApprovalRef            string   `json:"approval_ref,omitempty"`
	RequiredApprovals      int      `json:"required_approvals,omitempty"`
	ValidApprovals         int      `json:"valid_approvals,omitempty"`
	ApprovalAuditPath      string   `json:"approval_audit_path,omitempty"`
	TraceID                string   `json:"trace_id,omitempty"`
	TracePath              string   `json:"trace_path,omitempty"`
	PolicyDigest           string   `json:"policy_digest,omitempty"`
	IntentDigest           string   `json:"intent_digest,omitempty"`
	MatchedRule            string   `json:"matched_rule,omitempty"`
	RateLimitScope         string   `json:"rate_limit_scope,omitempty"`
	RateLimitKey           string   `json:"rate_limit_key,omitempty"`
	RateLimitUsed          int      `json:"rate_limit_used,omitempty"`
	RateLimitRemaining     int      `json:"rate_limit_remaining,omitempty"`
	CredentialIssuer       string   `json:"credential_issuer,omitempty"`
	CredentialRef          string   `json:"credential_ref,omitempty"`
	CredentialEvidencePath string   `json:"credential_evidence_path,omitempty"`
	SimulateMode           bool     `json:"simulate_mode,omitempty"`
	WouldHaveBlocked       bool     `json:"would_have_blocked,omitempty"`
	SimulatedVerdict       string   `json:"simulated_verdict,omitempty"`
	SimulatedReasonCodes   []string `json:"simulated_reason_codes,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
	Error                  string   `json:"error,omitempty"`
}

type gateEvalProfile string

const (
	gateProfileStandard gateEvalProfile = "standard"
	gateProfileOSSProd  gateEvalProfile = "oss-prod"
)

func runGate(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Evaluate structured tool intents against policy, enforce approval flows, and emit signed trace records.")
	}
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
	if hasExplainFlag(arguments) {
		return writeExplain("Run policy evaluation for a single intent request, optionally simulate non-enforcing rollout, and write signed gate artifacts.")
	}
	flagSet := flag.NewFlagSet("gate-eval", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var intentPath string
	var tracePath string
	var approvalTokenRef string
	var approvalTokenPath string
	var approvalTokenChain string
	var approvalAuditPath string
	var approvalPublicKeyPath string
	var approvalPublicKeyEnv string
	var approvalPrivateKeyPath string
	var approvalPrivateKeyEnv string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var profile string
	var rateLimitState string
	var credentialBroker string
	var credentialEnvPrefix string
	var credentialRef string
	var credentialScopesCSV string
	var credentialCommand string
	var credentialCommandArgsCSV string
	var credentialEvidencePath string
	var simulate bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml")
	flagSet.StringVar(&intentPath, "intent", "", "path to intent request json")
	flagSet.StringVar(&tracePath, "trace-out", "", "path to emitted trace JSON (default trace_<trace_id>.json)")
	flagSet.StringVar(&approvalTokenRef, "approval-token-ref", "", "optional approval token reference")
	flagSet.StringVar(&approvalTokenPath, "approval-token", "", "path to signed approval token")
	flagSet.StringVar(&approvalTokenChain, "approval-token-chain", "", "comma-separated paths to additional signed approval tokens")
	flagSet.StringVar(&approvalAuditPath, "approval-audit-out", "", "path to emitted approval audit JSON (default approval_audit_<trace_id>.json)")
	flagSet.StringVar(&approvalPublicKeyPath, "approval-public-key", "", "path to base64 approval verify key")
	flagSet.StringVar(&approvalPublicKeyEnv, "approval-public-key-env", "", "env var containing base64 approval verify key")
	flagSet.StringVar(&approvalPrivateKeyPath, "approval-private-key", "", "path to base64 approval private key (derive public)")
	flagSet.StringVar(&approvalPrivateKeyEnv, "approval-private-key-env", "", "env var containing base64 approval private key (derive public)")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.StringVar(&profile, "profile", string(gateProfileStandard), "runtime profile: standard|oss-prod")
	flagSet.StringVar(&rateLimitState, "rate-limit-state", ".gait-out/gate_rate_limits.json", "path to persisted rate limit state")
	flagSet.StringVar(&credentialBroker, "credential-broker", "off", "credential broker: off|stub|env|command")
	flagSet.StringVar(&credentialEnvPrefix, "credential-env-prefix", "GAIT_BROKER_TOKEN_", "env broker key prefix")
	flagSet.StringVar(&credentialRef, "credential-ref", "", "credential broker reference override")
	flagSet.StringVar(&credentialScopesCSV, "credential-scopes", "", "comma-separated broker scopes override")
	flagSet.StringVar(&credentialCommand, "credential-command", "", "command to execute when --credential-broker=command")
	flagSet.StringVar(&credentialCommandArgsCSV, "credential-command-args", "", "comma-separated args for --credential-command")
	flagSet.StringVar(&credentialEvidencePath, "credential-evidence-out", "", "path to emitted broker credential evidence JSON")
	flagSet.BoolVar(&simulate, "simulate", false, "non-enforcing simulation mode; report what would have been blocked")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
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
	resolvedProfile, err := parseGateEvalProfile(profile)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	keyMode = strings.ToLower(strings.TrimSpace(keyMode))
	if resolvedProfile == gateProfileOSSProd {
		if simulate {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: "oss-prod profile does not allow --simulate"}, exitInvalidInput)
		}
		if sign.KeyMode(keyMode) != sign.ModeProd {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: "oss-prod profile requires --key-mode prod"}, exitInvalidInput)
		}
		if !hasAnyKeySource(sign.KeyConfig{PrivateKeyPath: privateKeyPath, PrivateKeyEnv: privateKeyEnv}) {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: "oss-prod profile requires --private-key or --private-key-env"}, exitInvalidInput)
		}
	}

	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	intent, err := readIntentRequest(intentPath)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	resolvedBroker, err := credential.ResolveBroker(
		credentialBroker,
		credentialEnvPrefix,
		credentialCommand,
		parseCSV(credentialCommandArgsCSV),
	)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if resolvedProfile == gateProfileOSSProd {
		if gate.PolicyHasHighRiskUnbrokeredActions(policy) {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{
				OK:    false,
				Error: "oss-prod profile requires high-risk allow/approval rules to set require_broker_credential: true",
			}, exitInvalidInput)
		}
		if gate.PolicyRequiresBrokerForHighRisk(policy) && resolvedBroker == nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{
				OK:    false,
				Error: "oss-prod profile requires --credential-broker for high-risk policies",
			}, exitInvalidInput)
		}
	}

	evalStart := time.Now()
	outcome, err := gate.EvaluatePolicyDetailed(policy, intent, gate.EvalOptions{ProducerVersion: version})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	result := outcome.Result
	evalLatencyMS := time.Since(evalStart).Seconds() * 1000

	var rateDecision gate.RateLimitDecision
	if outcome.RateLimit.Requests > 0 {
		rateDecision, err = gate.EnforceRateLimit(rateLimitState, outcome.RateLimit, intent, time.Now().UTC())
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		if !rateDecision.Allowed {
			result.Verdict = "block"
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"rate_limit_exceeded"})
			result.Violations = mergeUniqueSorted(result.Violations, []string{"rate_limit_exceeded"})
		}
	}

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(keyMode),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	exitCode := exitOK
	resolvedApprovalRef := strings.TrimSpace(approvalTokenRef)
	requiredApprovals := outcome.MinApprovals
	validApprovals := 0
	approvalEntries := make([]schemagate.ApprovalAuditEntry, 0)
	approvalTokenPaths := gatherApprovalTokenPaths(approvalTokenPath, approvalTokenChain)

	if result.Verdict == "require_approval" {
		policyDigest, intentDigest, requiredScope, err := gate.ApprovalContext(policy, intent)
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		if requiredApprovals <= 0 {
			requiredApprovals = 1
		}

		verifyKey := keyPair.Public
		verifyConfig := sign.KeyConfig{
			PublicKeyPath:  approvalPublicKeyPath,
			PublicKeyEnv:   approvalPublicKeyEnv,
			PrivateKeyPath: approvalPrivateKeyPath,
			PrivateKeyEnv:  approvalPrivateKeyEnv,
		}
		if resolvedProfile == gateProfileOSSProd && !hasAnyKeySource(verifyConfig) {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{
				OK:    false,
				Error: "oss-prod profile requires explicit approval verify key (--approval-public-key/--approval-public-key-env or approval private key source)",
			}, exitInvalidInput)
		}
		if hasAnyKeySource(verifyConfig) {
			verifyKey, err = sign.LoadVerifyKey(verifyConfig)
			if err != nil {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
			}
		}

		validApproverSet := map[string]struct{}{}
		validTokenRefs := make([]string, 0, len(approvalTokenPaths))
		if len(approvalTokenPaths) == 0 {
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonMissingToken})
			result.Violations = mergeUniqueSorted(result.Violations, []string{"approval_not_granted"})
			exitCode = exitApprovalRequired
		} else {
			for _, tokenPath := range approvalTokenPaths {
				token, err := gate.ReadApprovalToken(tokenPath)
				if err != nil {
					return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
				}

				entry := schemagate.ApprovalAuditEntry{
					TokenID:          token.TokenID,
					ApproverIdentity: token.ApproverIdentity,
					ReasonCode:       token.ReasonCode,
					Scope:            mergeUniqueSorted(nil, token.Scope),
					ExpiresAt:        token.ExpiresAt.UTC(),
					Valid:            false,
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
					entry.ErrorCode = reasonCode
					result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{reasonCode})
					approvalEntries = append(approvalEntries, entry)
					continue
				}

				entry.Valid = true
				approvalEntries = append(approvalEntries, entry)
				validApprovals++
				if token.TokenID != "" {
					validTokenRefs = append(validTokenRefs, token.TokenID)
				}
				if token.ApproverIdentity != "" {
					validApproverSet[token.ApproverIdentity] = struct{}{}
				}
			}

			if validApprovals < requiredApprovals {
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonChainInsufficient})
				result.Violations = mergeUniqueSorted(result.Violations, []string{"approval_not_granted"})
				exitCode = exitApprovalRequired
			}
			if outcome.RequireDistinctApprovers && requiredApprovals > 1 && len(validApproverSet) < requiredApprovals {
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonDistinctApprovers})
				result.Violations = mergeUniqueSorted(result.Violations, []string{"approval_not_granted"})
				exitCode = exitApprovalRequired
			}

			if exitCode == exitOK {
				result.Verdict = "allow"
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{gate.ApprovalReasonGranted})
				if len(validTokenRefs) > 0 {
					resolvedApprovalRef = strings.Join(mergeUniqueSorted(nil, validTokenRefs), ",")
				}
			}
		}
	}

	credentialIssuer := ""
	credentialRefOut := ""
	credentialReferenceUsed := ""
	credentialScopesUsed := []string{}
	if outcome.RequireBrokerCredential && result.Verdict == "allow" {
		if resolvedBroker == nil {
			result.Verdict = "block"
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"broker_credential_required"})
			result.Violations = mergeUniqueSorted(result.Violations, []string{"broker_credential_missing"})
		} else {
			scope := mergeUniqueSorted(outcome.BrokerScopes, parseCSV(credentialScopesCSV))
			reference := strings.TrimSpace(credentialRef)
			if reference == "" {
				reference = outcome.BrokerReference
			}
			credentialReferenceUsed = reference
			credentialScopesUsed = scope
			issued, issueErr := credential.Issue(resolvedBroker, credential.Request{
				ToolName:  intent.ToolName,
				Identity:  intent.Context.Identity,
				Workspace: intent.Context.Workspace,
				SessionID: intent.Context.SessionID,
				RequestID: intent.Context.RequestID,
				Reference: reference,
				Scope:     scope,
			})
			if issueErr != nil {
				result.Verdict = "block"
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"broker_credential_missing"})
				result.Violations = mergeUniqueSorted(result.Violations, []string{"broker_credential_missing"})
			} else {
				credentialIssuer = issued.IssuedBy
				credentialRefOut = issued.CredentialRef
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"broker_credential_present"})
			}
		}
	}

	simulatedVerdict := ""
	simulatedReasonCodes := []string{}
	wouldHaveBlocked := false
	if simulate {
		simulatedVerdict = result.Verdict
		simulatedReasonCodes = mergeUniqueSorted(nil, result.ReasonCodes)
		wouldHaveBlocked = result.Verdict == "block" || result.Verdict == "require_approval"
		result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"simulate_mode_non_enforcing"})
		if wouldHaveBlocked {
			result.Verdict = "allow"
			result.Violations = []string{}
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"simulated_" + simulatedVerdict})
		}
		exitCode = exitOK
	}

	traceResult, err := gate.EmitSignedTrace(policy, intent, result, gate.EmitTraceOptions{
		ProducerVersion:   version,
		CorrelationID:     currentCorrelationID(),
		ApprovalTokenRef:  resolvedApprovalRef,
		LatencyMS:         evalLatencyMS,
		SigningPrivateKey: keyPair.Private,
		TracePath:         tracePath,
	})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	resolvedApprovalAuditPath := ""
	resolvedCredentialEvidencePath := ""
	if requiredApprovals > 0 || len(approvalEntries) > 0 {
		resolvedApprovalAuditPath = strings.TrimSpace(approvalAuditPath)
		if resolvedApprovalAuditPath == "" {
			resolvedApprovalAuditPath = fmt.Sprintf("approval_audit_%s.json", traceResult.Trace.TraceID)
		}
		audit := gate.BuildApprovalAuditRecord(gate.BuildApprovalAuditOptions{
			CreatedAt:         result.CreatedAt,
			ProducerVersion:   version,
			TraceID:           traceResult.Trace.TraceID,
			ToolName:          traceResult.Trace.ToolName,
			IntentDigest:      traceResult.IntentDigest,
			PolicyDigest:      traceResult.PolicyDigest,
			RequiredApprovals: requiredApprovals,
			Entries:           approvalEntries,
		})
		if err := gate.WriteApprovalAuditRecord(resolvedApprovalAuditPath, audit); err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		validApprovals = audit.ValidApprovals
	}
	if credentialRefOut != "" {
		resolvedCredentialEvidencePath = strings.TrimSpace(credentialEvidencePath)
		if resolvedCredentialEvidencePath == "" {
			resolvedCredentialEvidencePath = fmt.Sprintf("credential_evidence_%s.json", traceResult.Trace.TraceID)
		}
		credentialRecord := gate.BuildBrokerCredentialRecord(gate.BuildBrokerCredentialRecordOptions{
			CreatedAt:       result.CreatedAt,
			ProducerVersion: version,
			TraceID:         traceResult.Trace.TraceID,
			ToolName:        traceResult.Trace.ToolName,
			Identity:        intent.Context.Identity,
			Broker:          credentialIssuer,
			Reference:       credentialReferenceUsed,
			Scope:           credentialScopesUsed,
			CredentialRef:   credentialRefOut,
		})
		if err := gate.WriteBrokerCredentialRecord(resolvedCredentialEvidencePath, credentialRecord); err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}

	return writeGateEvalOutput(jsonOutput, gateEvalOutput{
		OK:                     true,
		Profile:                string(resolvedProfile),
		Verdict:                result.Verdict,
		ReasonCodes:            result.ReasonCodes,
		Violations:             result.Violations,
		ApprovalRef:            resolvedApprovalRef,
		RequiredApprovals:      requiredApprovals,
		ValidApprovals:         validApprovals,
		ApprovalAuditPath:      resolvedApprovalAuditPath,
		TraceID:                traceResult.Trace.TraceID,
		TracePath:              traceResult.TracePath,
		PolicyDigest:           traceResult.PolicyDigest,
		IntentDigest:           traceResult.IntentDigest,
		MatchedRule:            outcome.MatchedRule,
		RateLimitScope:         rateDecision.Scope,
		RateLimitKey:           rateDecision.Key,
		RateLimitUsed:          rateDecision.Used,
		RateLimitRemaining:     rateDecision.Remaining,
		CredentialIssuer:       credentialIssuer,
		CredentialRef:          credentialRefOut,
		CredentialEvidencePath: resolvedCredentialEvidencePath,
		SimulateMode:           simulate,
		WouldHaveBlocked:       wouldHaveBlocked,
		SimulatedVerdict:       simulatedVerdict,
		SimulatedReasonCodes:   simulatedReasonCodes,
		Warnings:               warnings,
	}, exitCode)
}

func gatherApprovalTokenPaths(primaryPath, chainCSV string) []string {
	paths := make([]string, 0, 1)
	if strings.TrimSpace(primaryPath) != "" {
		paths = append(paths, strings.TrimSpace(primaryPath))
	}
	paths = append(paths, parseCSV(chainCSV)...)
	return mergeUniqueSorted(nil, paths)
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
		return writeJSONOutput(output, exitCode)
	}

	if output.OK {
		fmt.Printf("gate eval: verdict=%s\n", output.Verdict)
		fmt.Printf("trace: %s\n", output.TracePath)
		if output.SimulateMode {
			fmt.Printf("simulate: would_have_blocked=%t simulated_verdict=%s\n", output.WouldHaveBlocked, output.SimulatedVerdict)
		}
		if len(output.ReasonCodes) > 0 {
			fmt.Printf("reasons: %s\n", joinCSV(output.ReasonCodes))
		}
		if len(output.Violations) > 0 {
			fmt.Printf("violations: %s\n", joinCSV(output.Violations))
		}
		if output.RequiredApprovals > 0 {
			fmt.Printf("approvals: %d/%d\n", output.ValidApprovals, output.RequiredApprovals)
		}
		if output.ApprovalAuditPath != "" {
			fmt.Printf("approval audit: %s\n", output.ApprovalAuditPath)
		}
		if output.CredentialRef != "" {
			fmt.Printf("credential: %s (%s)\n", output.CredentialRef, output.CredentialIssuer)
		}
		if output.CredentialEvidencePath != "" {
			fmt.Printf("credential evidence: %s\n", output.CredentialEvidencePath)
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
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--profile standard|oss-prod] [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--approval-audit-out audit.json] [--credential-broker off|stub|env|command] [--credential-command <path>] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printGateEvalUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--profile standard|oss-prod] [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--approval-token-ref token] [--approval-public-key <path>|--approval-public-key-env <VAR>] [--approval-audit-out audit.json] [--rate-limit-state state.json] [--credential-broker off|stub|env|command] [--credential-env-prefix GAIT_BROKER_TOKEN_] [--credential-command <path>] [--credential-command-args csv] [--credential-ref ref] [--credential-scopes csv] [--credential-evidence-out path] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func parseGateEvalProfile(value string) (gateEvalProfile, error) {
	profile := strings.ToLower(strings.TrimSpace(value))
	if profile == "" {
		return gateProfileStandard, nil
	}
	switch gateEvalProfile(profile) {
	case gateProfileStandard, gateProfileOSSProd:
		return gateEvalProfile(profile), nil
	default:
		return "", fmt.Errorf("unsupported --profile value %q (expected standard or oss-prod)", value)
	}
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
