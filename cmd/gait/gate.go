package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/credential"
	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/projectconfig"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
)

type gateEvalOutput struct {
	OK                     bool                          `json:"ok"`
	Profile                string                        `json:"profile,omitempty"`
	Verdict                string                        `json:"verdict,omitempty"`
	ReasonCodes            []string                      `json:"reason_codes,omitempty"`
	Violations             []string                      `json:"violations,omitempty"`
	ApprovalRef            string                        `json:"approval_ref,omitempty"`
	RequiredApprovals      int                           `json:"required_approvals,omitempty"`
	ValidApprovals         int                           `json:"valid_approvals,omitempty"`
	ApprovalAuditPath      string                        `json:"approval_audit_path,omitempty"`
	DelegationRef          string                        `json:"delegation_ref,omitempty"`
	DelegationRequired     bool                          `json:"delegation_required,omitempty"`
	ValidDelegations       int                           `json:"valid_delegations,omitempty"`
	DelegationAuditPath    string                        `json:"delegation_audit_path,omitempty"`
	TraceID                string                        `json:"trace_id,omitempty"`
	TracePath              string                        `json:"trace_path,omitempty"`
	PolicyDigest           string                        `json:"policy_digest,omitempty"`
	IntentDigest           string                        `json:"intent_digest,omitempty"`
	ContextSetDigest       string                        `json:"context_set_digest,omitempty"`
	ContextEvidenceMode    string                        `json:"context_evidence_mode,omitempty"`
	ContextRefCount        int                           `json:"context_ref_count,omitempty"`
	ContextSource          string                        `json:"context_source,omitempty"`
	Script                 bool                          `json:"script,omitempty"`
	StepCount              int                           `json:"step_count,omitempty"`
	ScriptHash             string                        `json:"script_hash,omitempty"`
	CompositeRiskClass     string                        `json:"composite_risk_class,omitempty"`
	StepVerdicts           []schemagate.TraceStepVerdict `json:"step_verdicts,omitempty"`
	PreApproved            bool                          `json:"pre_approved,omitempty"`
	PatternID              string                        `json:"pattern_id,omitempty"`
	RegistryReason         string                        `json:"registry_reason,omitempty"`
	MatchedRule            string                        `json:"matched_rule,omitempty"`
	Phase                  string                        `json:"phase,omitempty"`
	RateLimitScope         string                        `json:"rate_limit_scope,omitempty"`
	RateLimitKey           string                        `json:"rate_limit_key,omitempty"`
	RateLimitUsed          int                           `json:"rate_limit_used,omitempty"`
	RateLimitRemaining     int                           `json:"rate_limit_remaining,omitempty"`
	DestructiveBudgetScope     string                    `json:"destructive_budget_scope,omitempty"`
	DestructiveBudgetKey       string                    `json:"destructive_budget_key,omitempty"`
	DestructiveBudgetUsed      int                       `json:"destructive_budget_used,omitempty"`
	DestructiveBudgetRemaining int                       `json:"destructive_budget_remaining,omitempty"`
	CredentialIssuer       string                        `json:"credential_issuer,omitempty"`
	CredentialRef          string                        `json:"credential_ref,omitempty"`
	CredentialEvidencePath string                        `json:"credential_evidence_path,omitempty"`
	SimulateMode           bool                          `json:"simulate_mode,omitempty"`
	WouldHaveBlocked       bool                          `json:"would_have_blocked,omitempty"`
	SimulatedVerdict       string                        `json:"simulated_verdict,omitempty"`
	SimulatedReasonCodes   []string                      `json:"simulated_reason_codes,omitempty"`
	Warnings               []string                      `json:"warnings,omitempty"`
	Error                  string                        `json:"error,omitempty"`
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
	var delegationTokenPath string
	var delegationTokenChain string
	var delegationAuditPath string
	var approvalPublicKeyPath string
	var approvalPublicKeyEnv string
	var approvalPrivateKeyPath string
	var approvalPrivateKeyEnv string
	var delegationPublicKeyPath string
	var delegationPublicKeyEnv string
	var delegationPrivateKeyPath string
	var delegationPrivateKeyEnv string
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
	var wrkrInventoryPath string
	var approvedScriptRegistryPath string
	var approvedScriptPublicKeyPath string
	var approvedScriptPublicKeyEnv string
	var configPath string
	var disableConfig bool
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
	flagSet.StringVar(&delegationTokenPath, "delegation-token", "", "path to signed delegation token")
	flagSet.StringVar(&delegationTokenChain, "delegation-token-chain", "", "comma-separated paths to additional signed delegation tokens")
	flagSet.StringVar(&delegationAuditPath, "delegation-audit-out", "", "path to emitted delegation audit JSON (default delegation_audit_<trace_id>.json)")
	flagSet.StringVar(&approvalPublicKeyPath, "approval-public-key", "", "path to base64 approval verify key")
	flagSet.StringVar(&approvalPublicKeyEnv, "approval-public-key-env", "", "env var containing base64 approval verify key")
	flagSet.StringVar(&approvalPrivateKeyPath, "approval-private-key", "", "path to base64 approval private key (derive public)")
	flagSet.StringVar(&approvalPrivateKeyEnv, "approval-private-key-env", "", "env var containing base64 approval private key (derive public)")
	flagSet.StringVar(&delegationPublicKeyPath, "delegation-public-key", "", "path to base64 delegation verify key")
	flagSet.StringVar(&delegationPublicKeyEnv, "delegation-public-key-env", "", "env var containing base64 delegation verify key")
	flagSet.StringVar(&delegationPrivateKeyPath, "delegation-private-key", "", "path to base64 delegation private key (derive public)")
	flagSet.StringVar(&delegationPrivateKeyEnv, "delegation-private-key-env", "", "env var containing base64 delegation private key (derive public)")
	flagSet.StringVar(&keyMode, "key-mode", "", "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.StringVar(&profile, "profile", "", "runtime profile: standard|oss-prod")
	flagSet.StringVar(&rateLimitState, "rate-limit-state", "", "path to persisted rate limit state")
	flagSet.StringVar(&credentialBroker, "credential-broker", "", "credential broker: off|stub|env|command")
	flagSet.StringVar(&credentialEnvPrefix, "credential-env-prefix", "", "env broker key prefix")
	flagSet.StringVar(&credentialRef, "credential-ref", "", "credential broker reference override")
	flagSet.StringVar(&credentialScopesCSV, "credential-scopes", "", "comma-separated broker scopes override")
	flagSet.StringVar(&credentialCommand, "credential-command", "", "command to execute when --credential-broker=command")
	flagSet.StringVar(&credentialCommandArgsCSV, "credential-command-args", "", "comma-separated args for --credential-command")
	flagSet.StringVar(&credentialEvidencePath, "credential-evidence-out", "", "path to emitted broker credential evidence JSON")
	flagSet.StringVar(&wrkrInventoryPath, "wrkr-inventory", "", "path to local Wrkr inventory JSON")
	flagSet.StringVar(&approvedScriptRegistryPath, "approved-script-registry", "", "path to approved script registry JSON")
	flagSet.StringVar(&approvedScriptPublicKeyPath, "approved-script-public-key", "", "path to base64 approved-script verify key")
	flagSet.StringVar(&approvedScriptPublicKeyEnv, "approved-script-public-key-env", "", "env var containing base64 approved-script verify key")
	flagSet.StringVar(&configPath, "config", projectconfig.DefaultPath, "path to project defaults yaml")
	flagSet.BoolVar(&disableConfig, "no-config", false, "disable project defaults file lookup")
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
	if !disableConfig {
		allowMissing := isDefaultProjectConfigPath(configPath)
		configuration, err := projectconfig.Load(configPath, allowMissing)
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
		}
		applyGateConfigDefaults(configuration.Gate, &policyPath, &profile, &keyMode, &privateKeyPath, &privateKeyEnv, &approvalPublicKeyPath, &approvalPublicKeyEnv, &approvalPrivateKeyPath, &approvalPrivateKeyEnv, &rateLimitState, &credentialBroker, &credentialEnvPrefix, &credentialRef, &credentialScopesCSV, &credentialCommand, &credentialCommandArgsCSV, &credentialEvidencePath, &tracePath, &wrkrInventoryPath)
	}
	if profile == "" {
		profile = string(gateProfileStandard)
	}
	if keyMode == "" {
		keyMode = string(sign.ModeDev)
	}
	if rateLimitState == "" {
		rateLimitState = ".gait-out/gate_rate_limits.json"
	}
	if credentialBroker == "" {
		credentialBroker = "off"
	}
	if credentialEnvPrefix == "" {
		credentialEnvPrefix = "GAIT_BROKER_TOKEN_" // #nosec G101 -- env var prefix identifier, not credential material.
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
	startupWarnings := []string{}
	wrkrInventory := map[string]gate.WrkrToolMetadata{}
	wrkrSource := ""
	if strings.TrimSpace(wrkrInventoryPath) != "" {
		inventory, loadErr := gate.LoadWrkrInventory(wrkrInventoryPath)
		if loadErr != nil {
			riskClass := strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))
			if resolvedProfile == gateProfileOSSProd || riskClass == "high" || riskClass == "critical" {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{
					OK:    false,
					Error: "wrkr inventory unavailable in fail-closed mode: " + loadErr.Error(),
				}, exitPolicyBlocked)
			}
			startupWarnings = append(startupWarnings, "wrkr inventory unavailable; continuing without context enrichment")
		} else {
			wrkrInventory = inventory.Tools
			wrkrSource = inventory.Path
		}
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

	approvedRegistryConfigured := strings.TrimSpace(approvedScriptRegistryPath) != ""
	approvedRegistryEntries := []schemagate.ApprovedScriptEntry{}
	if approvedRegistryConfigured {
		entries, readErr := gate.ReadApprovedScriptRegistry(approvedScriptRegistryPath)
		if readErr != nil {
			riskClass := strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))
			if resolvedProfile == gateProfileOSSProd || riskClass == "high" || riskClass == "critical" {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{
					OK:    false,
					Error: "approved script registry unavailable in fail-closed mode: " + readErr.Error(),
				}, exitPolicyBlocked)
			}
			startupWarnings = append(startupWarnings, "approved script registry unavailable; continuing without fast-path pre-approval")
		} else {
			approvedVerifyConfig := sign.KeyConfig{
				PublicKeyPath: approvedScriptPublicKeyPath,
				PublicKeyEnv:  approvedScriptPublicKeyEnv,
			}
			if !hasAnyKeySource(approvedVerifyConfig) {
				approvedVerifyConfig = sign.KeyConfig{
					PublicKeyPath:  approvalPublicKeyPath,
					PublicKeyEnv:   approvalPublicKeyEnv,
					PrivateKeyPath: approvalPrivateKeyPath,
					PrivateKeyEnv:  approvalPrivateKeyEnv,
				}
			}
			if resolvedProfile == gateProfileOSSProd && !hasAnyKeySource(approvedVerifyConfig) {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{
					OK:    false,
					Error: "oss-prod profile requires approved-script verify key when --approved-script-registry is set",
				}, exitInvalidInput)
			}
			if hasAnyKeySource(approvedVerifyConfig) {
				verifyKey, verifyErr := sign.LoadVerifyKey(approvedVerifyConfig)
				if verifyErr != nil {
					return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: verifyErr.Error()}, exitCodeForError(verifyErr, exitInvalidInput))
				}
				nowUTC := time.Now().UTC()
				for index, entry := range entries {
					if verifyErr := gate.VerifyApprovedScriptEntry(entry, verifyKey, nowUTC); verifyErr != nil {
						riskClass := strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))
						if resolvedProfile == gateProfileOSSProd || riskClass == "high" || riskClass == "critical" {
							return writeGateEvalOutput(jsonOutput, gateEvalOutput{
								OK:    false,
								Error: fmt.Sprintf("approved script registry verification failed at entry %d: %v", index, verifyErr),
							}, exitPolicyBlocked)
						}
						startupWarnings = append(startupWarnings, "approved script registry entry failed verification; fast-path disabled")
						entries = []schemagate.ApprovedScriptEntry{}
						break
					}
				}
			}
			approvedRegistryEntries = entries
		}
	}

	evalStart := time.Now()
	outcome := gate.EvalOutcome{}
	registryReason := ""
	preApprovedFastPath := false
	if approvedRegistryConfigured && len(approvedRegistryEntries) > 0 {
		policyDigestForRegistry, digestErr := gate.PolicyDigest(policy)
		if digestErr != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: digestErr.Error()}, exitCodeForError(digestErr, exitInvalidInput))
		}
		match, matchErr := gate.MatchApprovedScript(intent, policyDigestForRegistry, approvedRegistryEntries, time.Now().UTC())
		if matchErr != nil {
			riskClass := strings.ToLower(strings.TrimSpace(intent.Context.RiskClass))
			if resolvedProfile == gateProfileOSSProd || riskClass == "high" || riskClass == "critical" {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{
					OK:    false,
					Error: "approved script fast-path evaluation failed in fail-closed mode: " + matchErr.Error(),
				}, exitPolicyBlocked)
			}
			startupWarnings = append(startupWarnings, "approved script fast-path evaluation failed; continuing with policy evaluation")
		} else if match.Matched {
			preApprovedOutcome, preApproveErr := buildPreApprovedOutcome(intent, version, match)
			if preApproveErr != nil {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: preApproveErr.Error()}, exitCodeForError(preApproveErr, exitInvalidInput))
			}
			outcome = preApprovedOutcome
			preApprovedFastPath = true
		} else {
			registryReason = match.Reason
		}
	}
	if !preApprovedFastPath {
		outcome, err = gate.EvaluatePolicyDetailed(policy, intent, gate.EvalOptions{
			ProducerVersion: version,
			WrkrInventory:   wrkrInventory,
			WrkrSource:      wrkrSource,
		})
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		if registryReason != "" {
			outcome.RegistryReason = registryReason
		}
	}
	result := outcome.Result
	evalLatencyMS := time.Since(evalStart).Seconds() * 1000
	policyDigestForContext, intentDigestForContext, requiredApprovalScope, err := gate.ApprovalContext(policy, intent)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	delegationBindingDigest, err := gate.DelegationBindingDigest(intent)
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	var rateDecision gate.RateLimitDecision
	var destructiveBudgetDecision gate.RateLimitDecision
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
	if outcome.DestructiveBudget.Requests > 0 && gate.IntentContainsDestructiveTarget(intent.Targets) {
		budgetIntent := intent
		budgetIntent.ToolName = "destructive_budget|" + strings.TrimSpace(intent.ToolName)
		destructiveBudgetDecision, err = gate.EnforceRateLimit(rateLimitState, outcome.DestructiveBudget, budgetIntent, time.Now().UTC())
		if err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		if !destructiveBudgetDecision.Allowed {
			result.Verdict = "block"
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"destructive_budget_exceeded"})
			result.Violations = mergeUniqueSorted(result.Violations, []string{"destructive_budget_exceeded"})
		}
	}

	keyPair, signingWarnings, err := sign.LoadSigningKey(sign.KeyConfig{
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
	delegationRequired := outcome.RequireDelegation
	resolvedDelegationRef := ""
	validDelegations := 0
	delegationEntries := make([]schemagate.DelegationAuditEntry, 0)
	delegationTokenPaths := gatherDelegationTokenPaths(delegationTokenPath, delegationTokenChain)

	if delegationRequired {
		verifyKey := keyPair.Public
		delegationVerifyConfig := sign.KeyConfig{
			PublicKeyPath:  delegationPublicKeyPath,
			PublicKeyEnv:   delegationPublicKeyEnv,
			PrivateKeyPath: delegationPrivateKeyPath,
			PrivateKeyEnv:  delegationPrivateKeyEnv,
		}
		if !hasAnyKeySource(delegationVerifyConfig) {
			delegationVerifyConfig = sign.KeyConfig{
				PublicKeyPath:  approvalPublicKeyPath,
				PublicKeyEnv:   approvalPublicKeyEnv,
				PrivateKeyPath: approvalPrivateKeyPath,
				PrivateKeyEnv:  approvalPrivateKeyEnv,
			}
		}
		if resolvedProfile == gateProfileOSSProd && !hasAnyKeySource(delegationVerifyConfig) {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{
				OK:    false,
				Error: "oss-prod profile requires explicit delegation verify key (--delegation-public-key/--delegation-public-key-env or private key source)",
			}, exitInvalidInput)
		}
		if hasAnyKeySource(delegationVerifyConfig) {
			verifyKey, err = sign.LoadVerifyKey(delegationVerifyConfig)
			if err != nil {
				return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
			}
		}
		if len(delegationTokenPaths) == 0 {
			result.Verdict = "block"
			result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"delegation_token_missing"})
			result.Violations = mergeUniqueSorted(result.Violations, []string{"delegation_not_granted"})
			exitCode = exitPolicyBlocked
		} else {
			expectedDelegator := ""
			expectedDelegate := ""
			if intent.Delegation != nil {
				expectedDelegate = intent.Delegation.RequesterIdentity
				if len(intent.Delegation.Chain) > 0 {
					last := intent.Delegation.Chain[len(intent.Delegation.Chain)-1]
					expectedDelegator = last.DelegatorIdentity
					if expectedDelegate == "" {
						expectedDelegate = last.DelegateIdentity
					}
				}
			}
			validTokenRefs := make([]string, 0, len(delegationTokenPaths))
			for _, tokenPath := range delegationTokenPaths {
				token, readErr := gate.ReadDelegationToken(tokenPath)
				if readErr != nil {
					return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: readErr.Error()}, exitCodeForError(readErr, exitInvalidInput))
				}
				entry := schemagate.DelegationAuditEntry{
					TokenID:           token.TokenID,
					DelegatorIdentity: token.DelegatorIdentity,
					DelegateIdentity:  token.DelegateIdentity,
					Scope:             mergeUniqueSorted(nil, token.Scope),
					ExpiresAt:         token.ExpiresAt.UTC(),
					Valid:             false,
				}
				validateErr := gate.ValidateDelegationToken(token, verifyKey, gate.DelegationValidationOptions{
					Now:                  time.Now().UTC(),
					ExpectedDelegator:    expectedDelegator,
					ExpectedDelegate:     expectedDelegate,
					ExpectedIntentDigest: intentDigestForContext,
					ExpectedPolicyDigest: policyDigestForContext,
				})
				if validateErr != nil {
					reasonCode := gate.DelegationCodeSchemaInvalid
					var tokenErr *gate.DelegationTokenError
					if errors.As(validateErr, &tokenErr) && tokenErr.Code != "" {
						reasonCode = tokenErr.Code
					}
					entry.ErrorCode = reasonCode
					result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{reasonCode})
					delegationEntries = append(delegationEntries, entry)
					continue
				}
				entry.Valid = true
				delegationEntries = append(delegationEntries, entry)
				validDelegations++
				if token.TokenID != "" {
					validTokenRefs = append(validTokenRefs, token.TokenID)
				}
			}
			if validDelegations == 0 {
				result.Verdict = "block"
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"delegation_chain_insufficient"})
				result.Violations = mergeUniqueSorted(result.Violations, []string{"delegation_not_granted"})
				exitCode = exitPolicyBlocked
			} else {
				result.ReasonCodes = mergeUniqueSorted(result.ReasonCodes, []string{"delegation_granted"})
				if len(validTokenRefs) > 0 {
					resolvedDelegationRef = strings.Join(mergeUniqueSorted(nil, validTokenRefs), ",")
				}
			}
		}
	}

	if result.Verdict == "require_approval" {
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
					Now:                             time.Now().UTC(),
					ExpectedIntentDigest:            intentDigestForContext,
					ExpectedPolicyDigest:            policyDigestForContext,
					ExpectedDelegationBindingDigest: delegationBindingDigest,
					RequiredScope:                   requiredApprovalScope,
					TargetCount:                     gateIntentTargetCount(intent),
					OperationCount:                  gateIntentOperationCount(intent),
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
	credentialIssuedAt := time.Time{}
	credentialExpiresAt := time.Time{}
	credentialTTLSeconds := int64(0)
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
				credentialIssuedAt = issued.IssuedAt
				credentialExpiresAt = issued.ExpiresAt
				credentialTTLSeconds = issued.TTLSeconds
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
	exitCode = gateEvalExitCodeForVerdict(result.Verdict, exitCode)

	traceResult, err := gate.EmitSignedTrace(policy, intent, result, gate.EmitTraceOptions{
		ProducerVersion:       version,
		CorrelationID:         currentCorrelationID(),
		ApprovalTokenRef:      resolvedApprovalRef,
		DelegationTokenRef:    resolvedDelegationRef,
		DelegationReasonCodes: mergeUniqueSorted(nil, filterReasonsByPrefix(result.ReasonCodes, "delegation_")),
		LatencyMS:             evalLatencyMS,
		ContextSource:         outcome.ContextSource,
		CompositeRiskClass:    outcome.CompositeRiskClass,
		StepVerdicts:          outcome.StepVerdicts,
		PreApproved:           outcome.PreApproved,
		PatternID:             outcome.PatternID,
		RegistryReason:        outcome.RegistryReason,
		SigningPrivateKey:     keyPair.Private,
		TracePath:             tracePath,
	})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	resolvedApprovalAuditPath := ""
	resolvedDelegationAuditPath := ""
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
			IssuedAt:        credentialIssuedAt,
			ExpiresAt:       credentialExpiresAt,
			TTLSeconds:      credentialTTLSeconds,
		})
		if err := gate.WriteBrokerCredentialRecord(resolvedCredentialEvidencePath, credentialRecord); err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}
	if delegationRequired || len(delegationEntries) > 0 {
		resolvedDelegationAuditPath = strings.TrimSpace(delegationAuditPath)
		if resolvedDelegationAuditPath == "" {
			resolvedDelegationAuditPath = fmt.Sprintf("delegation_audit_%s.json", traceResult.Trace.TraceID)
		}
		audit := gate.BuildDelegationAuditRecord(gate.BuildDelegationAuditOptions{
			CreatedAt:          result.CreatedAt,
			ProducerVersion:    version,
			TraceID:            traceResult.Trace.TraceID,
			ToolName:           traceResult.Trace.ToolName,
			IntentDigest:       traceResult.IntentDigest,
			PolicyDigest:       traceResult.PolicyDigest,
			DelegationRequired: delegationRequired,
			DelegationRef:      resolvedDelegationRef,
			Entries:            delegationEntries,
		})
		if err := gate.WriteDelegationAuditRecord(resolvedDelegationAuditPath, audit); err != nil {
			return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		validDelegations = audit.ValidDelegations
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
		DelegationRef:          resolvedDelegationRef,
		DelegationRequired:     delegationRequired,
		ValidDelegations:       validDelegations,
		DelegationAuditPath:    resolvedDelegationAuditPath,
		TraceID:                traceResult.Trace.TraceID,
		TracePath:              traceResult.TracePath,
		PolicyDigest:           traceResult.PolicyDigest,
		IntentDigest:           traceResult.IntentDigest,
		ContextSetDigest:       intent.Context.ContextSetDigest,
		ContextEvidenceMode:    intent.Context.ContextEvidenceMode,
		ContextRefCount:        len(intent.Context.ContextRefs),
		ContextSource:          outcome.ContextSource,
		Script:                 outcome.Script,
		StepCount:              outcome.StepCount,
		ScriptHash:             outcome.ScriptHash,
		CompositeRiskClass:     outcome.CompositeRiskClass,
		StepVerdicts:           outcome.StepVerdicts,
		PreApproved:            outcome.PreApproved,
		PatternID:              outcome.PatternID,
		RegistryReason:         outcome.RegistryReason,
		MatchedRule:            outcome.MatchedRule,
		Phase:                  intent.Context.Phase,
		RateLimitScope:         rateDecision.Scope,
		RateLimitKey:           rateDecision.Key,
		RateLimitUsed:          rateDecision.Used,
		RateLimitRemaining:     rateDecision.Remaining,
		DestructiveBudgetScope:     destructiveBudgetDecision.Scope,
		DestructiveBudgetKey:       destructiveBudgetDecision.Key,
		DestructiveBudgetUsed:      destructiveBudgetDecision.Used,
		DestructiveBudgetRemaining: destructiveBudgetDecision.Remaining,
		CredentialIssuer:       credentialIssuer,
		CredentialRef:          credentialRefOut,
		CredentialEvidencePath: resolvedCredentialEvidencePath,
		SimulateMode:           simulate,
		WouldHaveBlocked:       wouldHaveBlocked,
		SimulatedVerdict:       simulatedVerdict,
		SimulatedReasonCodes:   simulatedReasonCodes,
		Warnings:               mergeUniqueSorted(startupWarnings, signingWarnings),
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

func gatherDelegationTokenPaths(primaryPath, chainCSV string) []string {
	paths := make([]string, 0, 1)
	if strings.TrimSpace(primaryPath) != "" {
		paths = append(paths, strings.TrimSpace(primaryPath))
	}
	paths = append(paths, parseCSV(chainCSV)...)
	return mergeUniqueSorted(nil, paths)
}

func gateIntentTargetCount(intent schemagate.IntentRequest) int {
	if intent.Script != nil && len(intent.Script.Steps) > 0 {
		total := 0
		for _, step := range intent.Script.Steps {
			total += len(step.Targets)
		}
		if total > 0 {
			return total
		}
	}
	return len(intent.Targets)
}

func gateIntentOperationCount(intent schemagate.IntentRequest) int {
	if intent.Script != nil && len(intent.Script.Steps) > 0 {
		return len(intent.Script.Steps)
	}
	if len(intent.Targets) == 0 {
		return 1
	}
	return len(intent.Targets)
}

func buildPreApprovedOutcome(intent schemagate.IntentRequest, producerVersion string, match gate.ApprovedScriptMatch) (gate.EvalOutcome, error) {
	normalizedIntent, err := gate.NormalizeIntent(intent)
	if err != nil {
		return gate.EvalOutcome{}, fmt.Errorf("normalize intent for approved-script fast-path: %w", err)
	}
	nowUTC := time.Now().UTC()
	outcome := gate.EvalOutcome{
		Result: schemagate.GateResult{
			SchemaID:        "gait.gate.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       nowUTC,
			ProducerVersion: producerVersion,
			Verdict:         "allow",
			ReasonCodes:     []string{match.Reason},
			Violations:      []string{},
		},
		PreApproved:    true,
		PatternID:      match.PatternID,
		RegistryReason: match.Reason,
	}
	if normalizedIntent.Script == nil {
		return outcome, nil
	}
	outcome.Script = true
	outcome.StepCount = len(normalizedIntent.Script.Steps)
	outcome.ScriptHash = normalizedIntent.ScriptHash
	stepVerdicts := make([]schemagate.TraceStepVerdict, 0, len(normalizedIntent.Script.Steps))
	riskClasses := make([]string, 0, len(normalizedIntent.Script.Steps))
	for index, step := range normalizedIntent.Script.Steps {
		stepVerdicts = append(stepVerdicts, schemagate.TraceStepVerdict{
			Index:       index,
			ToolName:    step.ToolName,
			Verdict:     "allow",
			ReasonCodes: []string{match.Reason},
			Violations:  []string{},
		})
		riskClasses = append(riskClasses, classifyPreApprovedStepRisk(step.Targets))
	}
	outcome.StepVerdicts = stepVerdicts
	outcome.CompositeRiskClass = compositePreApprovedRiskClass(riskClasses)
	return outcome, nil
}

func classifyPreApprovedStepRisk(targets []schemagate.IntentTarget) string {
	risk := "low"
	for _, target := range targets {
		switch target.EndpointClass {
		case "fs.delete", "proc.exec":
			return "high"
		case "fs.write", "net.http", "net.dns":
			if risk == "low" {
				risk = "medium"
			}
		}
		if target.Destructive {
			return "high"
		}
	}
	return risk
}

func compositePreApprovedRiskClass(riskClasses []string) string {
	hasMedium := false
	for _, riskClass := range riskClasses {
		switch riskClass {
		case "high":
			return "high"
		case "medium":
			hasMedium = true
		}
	}
	if hasMedium {
		return "medium"
	}
	return "low"
}

func gateEvalExitCodeForVerdict(verdict string, current int) int {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "block":
		return exitPolicyBlocked
	case "require_approval":
		if current == exitOK {
			return exitApprovalRequired
		}
	}
	return current
}

func isDefaultProjectConfigPath(path string) bool {
	return filepath.Clean(strings.TrimSpace(path)) == filepath.Clean(projectconfig.DefaultPath)
}

func applyGateConfigDefaults(
	defaults projectconfig.GateDefaults,
	policyPath *string,
	profile *string,
	keyMode *string,
	privateKeyPath *string,
	privateKeyEnv *string,
	approvalPublicKeyPath *string,
	approvalPublicKeyEnv *string,
	approvalPrivateKeyPath *string,
	approvalPrivateKeyEnv *string,
	rateLimitState *string,
	credentialBroker *string,
	credentialEnvPrefix *string,
	credentialRef *string,
	credentialScopesCSV *string,
	credentialCommand *string,
	credentialCommandArgsCSV *string,
	credentialEvidencePath *string,
	tracePath *string,
	wrkrInventoryPath *string,
) {
	if strings.TrimSpace(*policyPath) == "" {
		*policyPath = defaults.Policy
	}
	if strings.TrimSpace(*profile) == "" {
		*profile = defaults.Profile
	}
	if strings.TrimSpace(*keyMode) == "" {
		*keyMode = defaults.KeyMode
	}
	if strings.TrimSpace(*privateKeyPath) == "" {
		*privateKeyPath = defaults.PrivateKey
	}
	if strings.TrimSpace(*privateKeyEnv) == "" {
		*privateKeyEnv = defaults.PrivateKeyEnv
	}
	if strings.TrimSpace(*approvalPublicKeyPath) == "" {
		*approvalPublicKeyPath = defaults.ApprovalPublicKey
	}
	if strings.TrimSpace(*approvalPublicKeyEnv) == "" {
		*approvalPublicKeyEnv = defaults.ApprovalPublicKeyEnv
	}
	if strings.TrimSpace(*approvalPrivateKeyPath) == "" {
		*approvalPrivateKeyPath = defaults.ApprovalPrivateKey
	}
	if strings.TrimSpace(*approvalPrivateKeyEnv) == "" {
		*approvalPrivateKeyEnv = defaults.ApprovalPrivateKeyEnv
	}
	if strings.TrimSpace(*rateLimitState) == "" {
		*rateLimitState = defaults.RateLimitState
	}
	if strings.TrimSpace(*credentialBroker) == "" {
		*credentialBroker = defaults.CredentialBroker
	}
	if strings.TrimSpace(*credentialEnvPrefix) == "" {
		*credentialEnvPrefix = defaults.CredentialEnvPrefix
	}
	if strings.TrimSpace(*credentialRef) == "" {
		*credentialRef = defaults.CredentialRef
	}
	if strings.TrimSpace(*credentialScopesCSV) == "" {
		*credentialScopesCSV = defaults.CredentialScopes
	}
	if strings.TrimSpace(*credentialCommand) == "" {
		*credentialCommand = defaults.CredentialCommand
	}
	if strings.TrimSpace(*credentialCommandArgsCSV) == "" {
		*credentialCommandArgsCSV = defaults.CredentialCommandArgs
	}
	if strings.TrimSpace(*credentialEvidencePath) == "" {
		*credentialEvidencePath = defaults.CredentialEvidencePath
	}
	if strings.TrimSpace(*tracePath) == "" {
		*tracePath = defaults.TracePath
	}
	if strings.TrimSpace(*wrkrInventoryPath) == "" {
		*wrkrInventoryPath = defaults.WrkrInventoryPath
	}
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
		if output.DelegationRequired {
			fmt.Printf("delegations: %d\n", output.ValidDelegations)
		}
		if output.DelegationAuditPath != "" {
			fmt.Printf("delegation audit: %s\n", output.DelegationAuditPath)
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
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--config .gait/config.yaml] [--no-config] [--profile standard|oss-prod] [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--delegation-token <token.json>] [--delegation-token-chain <csv>] [--approval-audit-out audit.json] [--delegation-audit-out audit.json] [--credential-broker off|stub|env|command] [--credential-command <path>] [--wrkr-inventory <inventory.json>] [--approved-script-registry <registry.json>] [--approved-script-public-key <path>|--approved-script-public-key-env <VAR>] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("Rollout path:")
	fmt.Println("  observe: gait gate eval ... --simulate --json")
	fmt.Println("  enforce: gait gate eval ... --json")
}

func printGateEvalUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--config .gait/config.yaml] [--no-config] [--profile standard|oss-prod] [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--delegation-token <token.json>] [--delegation-token-chain <csv>] [--approval-token-ref token] [--approval-public-key <path>|--approval-public-key-env <VAR>] [--delegation-public-key <path>|--delegation-public-key-env <VAR>] [--approval-audit-out audit.json] [--delegation-audit-out audit.json] [--rate-limit-state state.json] [--credential-broker off|stub|env|command] [--credential-env-prefix GAIT_BROKER_TOKEN_] [--credential-command <path>] [--credential-command-args csv] [--credential-ref ref] [--credential-scopes csv] [--credential-evidence-out path] [--wrkr-inventory <inventory.json>] [--approved-script-registry <registry.json>] [--approved-script-public-key <path>|--approved-script-public-key-env <VAR>] [--trace-out trace.json] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  observe first: add --simulate while tuning")
	fmt.Println("  enforce later: remove --simulate once fixtures are stable")
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
	merged := append([]string{}, current...)
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

func filterReasonsByPrefix(reasons []string, prefix string) []string {
	if strings.TrimSpace(prefix) == "" || len(reasons) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if strings.HasPrefix(strings.TrimSpace(reason), prefix) {
			filtered = append(filtered, reason)
		}
	}
	return filtered
}
