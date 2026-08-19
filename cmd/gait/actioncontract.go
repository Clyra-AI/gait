package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

type actionContractOutput struct {
	SchemaID      string                            `json:"schema_id"`
	SchemaVersion string                            `json:"schema_version"`
	OK            bool                              `json:"ok"`
	Operation     string                            `json:"operation"`
	Proposal      *actioncontract.ValidationResult  `json:"proposal,omitempty"`
	Activated     *actioncontract.ActivatedArtifact `json:"activated,omitempty"`
	Receipt       *actionContractReceipt            `json:"receipt,omitempty"`
	Error         string                            `json:"error,omitempty"`
	ReasonCodes   []string                          `json:"reason_codes,omitempty"`
}

type actionContractReceipt struct {
	Consumer             string                                    `json:"consumer"`
	Version              string                                    `json:"version"`
	ScenarioID           string                                    `json:"scenario_id"`
	ArtifactSHA256       string                                    `json:"artifact_sha256"`
	Status               string                                    `json:"status"`
	SelfAttestation      bool                                      `json:"self_attestation"`
	ProposalArtifactID   string                                    `json:"proposal_artifact_id,omitempty"`
	ContractID           string                                    `json:"contract_id,omitempty"`
	ContractFamilyID     string                                    `json:"contract_family_id,omitempty"`
	Revision             int                                       `json:"revision,omitempty"`
	ResolutionKey        string                                    `json:"resolution_key,omitempty"`
	SchemaVersions       actionContractSchemaVersions              `json:"schema_versions"`
	SupportedConstraints actioncontract.SupportedConstraintSummary `json:"supported_constraints"`
	SemanticResult       actionContractSemanticResult              `json:"semantic_result"`
}

type actionContractSchemaVersions struct {
	Artifact string `json:"artifact"`
	Contract string `json:"contract"`
}

type actionContractSemanticResult struct {
	ProposalValid   bool     `json:"proposal_valid"`
	ActivationReady bool     `json:"activation_ready"`
	ReasonCodes     []string `json:"reason_codes,omitempty"`
	ExecutionClaim  bool     `json:"execution_claim"`
	EffectClaim     bool     `json:"effect_claim"`
}

func runActionContract(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Validate explicitly selected Wrkr proposed Action Contract artifacts and create signed Gait activation artifacts at an explicit authority boundary.")
	}
	if len(arguments) == 0 {
		printActionContractUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "validate", "ingest":
		return runActionContractValidate(arguments[1:])
	case "activate":
		return runActionContractActivate(arguments[1:])
	case "consume", "consumer":
		return runActionContractConsume(arguments[1:])
	default:
		printActionContractUsage()
		return exitInvalidInput
	}
}

func actionContractFlags(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func runActionContractValidate(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "artifact": true, "from": true, "evaluation-time": true})
	flags := actionContractFlags("contract-validate")
	var proposalPath, evaluationTime string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed_action_contract artifact")
	flags.StringVar(&proposalPath, "artifact", "", "alias for --proposal")
	flags.StringVar(&proposalPath, "from", "", "alias for --proposal")
	flags.StringVar(&evaluationTime, "evaluation-time", "", "fixed RFC3339 time for stale checks")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "validate", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractValidateUsage()
		return exitOK
	}
	if actionContractSelectionFlagCount(arguments) > 1 {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "validate", Error: "exactly one proposal flag may be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
	}
	if len(flags.Args()) > 0 {
		if proposalPath != "" {
			return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "validate", Error: "exactly one proposal path must be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
		}
		proposalPath = flags.Args()[0]
	}
	if strings.TrimSpace(proposalPath) == "" {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "validate", Error: "--proposal is required", ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}, exitInvalidInput)
	}
	artifact, _, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "validate", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
	}
	validation := actioncontract.ValidateArtifact(artifact, actioncontract.ValidationOptions{Now: parseEvaluationTime(evaluationTime)})
	out := actionContractOutput{SchemaID: actioncontract.ProposedSchemaID, SchemaVersion: actioncontract.ProposedSchemaVersion, OK: validation.Valid, Operation: "validate", Proposal: &validation, ReasonCodes: validation.Reasons}
	code := exitOK
	if !validation.Valid {
		code = exitVerifyFailed
	}
	return writeActionContractOutput(jsonOutput, out, code)
}

func runActionContractActivate(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "artifact": true, "from": true, "policy-digest": true, "principal": true, "activating-principal": true, "authority-refs": true, "authority-ref": true, "target": true, "environment": true, "mode": true, "valid-from": true, "valid-until": true, "exceptions": true, "exception": true, "out": true, "private-key": true, "private-key-env": true})
	flags := actionContractFlags("contract-activate")
	var proposalPath, policyDigest, principal, authorityCSV, authorityRef, target, environment, mode, validFrom, validUntil, exceptionsCSV, exception, outPath, privateKeyPath, privateKeyEnv string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed_action_contract artifact")
	flags.StringVar(&proposalPath, "artifact", "", "alias for --proposal")
	flags.StringVar(&proposalPath, "from", "", "alias for --proposal")
	flags.StringVar(&policyDigest, "policy-digest", "", "JCS SHA-256 digest of the Gait policy")
	flags.StringVar(&principal, "principal", "", "activating principal reference")
	flags.StringVar(&principal, "activating-principal", "", "alias for --principal")
	flags.StringVar(&authorityCSV, "authority-refs", "", "comma-separated authority references")
	flags.StringVar(&authorityRef, "authority-ref", "", "one authority reference (repeatable through --authority-refs)")
	flags.StringVar(&target, "target", "", "explicit activation target")
	flags.StringVar(&environment, "environment", "", "explicit target environment")
	flags.StringVar(&mode, "mode", "", "activation mode: context_only|enforce_floor|required")
	flags.StringVar(&validFrom, "valid-from", "", "RFC3339 validity start")
	flags.StringVar(&validUntil, "valid-until", "", "RFC3339 validity end")
	flags.StringVar(&exceptionsCSV, "exceptions", "", "comma-separated explicit exceptions")
	flags.StringVar(&exception, "exception", "", "one explicit exception")
	flags.StringVar(&outPath, "out", "", "optional activation artifact output path")
	flags.StringVar(&privateKeyPath, "private-key", "", "base64 Ed25519 private key path")
	flags.StringVar(&privateKeyEnv, "private-key-env", "", "environment variable containing base64 Ed25519 private key")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractActivateUsage()
		return exitOK
	}
	if actionContractSelectionFlagCount(arguments) > 1 {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: "exactly one proposal flag may be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
	}
	if len(flags.Args()) > 0 {
		if proposalPath != "" {
			return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: "exactly one proposal path must be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
		}
		proposalPath = flags.Args()[0]
	}
	if strings.TrimSpace(proposalPath) == "" {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: "--proposal is required", ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}, exitInvalidInput)
	}
	artifact, _, err := actioncontract.ReadArtifact(proposalPath)
	if err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
	}
	privateKey, err := loadActionContractPrivateKey(privateKeyPath, privateKeyEnv)
	if err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: err.Error()}, exitInvalidInput)
	}
	authorityRefs := parseActionContractCSV(authorityCSV)
	if strings.TrimSpace(authorityRef) != "" {
		authorityRefs = append(authorityRefs, authorityRef)
	}
	exceptions := parseActionContractCSV(exceptionsCSV)
	if strings.TrimSpace(exception) != "" {
		exceptions = append(exceptions, exception)
	}
	activated, validation, err := actioncontract.Activate(artifact, actioncontract.ActivationOptions{PolicyDigest: policyDigest, ActivatingPrincipal: principal, AuthorityRefs: authorityRefs, Target: target, Environment: environment, Mode: actioncontract.ActivationMode(strings.TrimSpace(mode)), ValidFrom: validFrom, ValidUntil: validUntil, ExplicitExceptions: exceptions, SigningPrivateKey: privateKey, EvaluationTime: parseEvaluationTime("")})
	if err != nil {
		return writeActionContractOutput(jsonOutput, actionContractOutput{SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, Operation: "activate", Proposal: &validation, Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitVerifyFailed)
	}
	if outPath != "" {
		encoded, marshalErr := json.MarshalIndent(activated, "", "  ")
		if marshalErr != nil {
			return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: marshalErr.Error()}, exitInternalFailure)
		}
		encoded = append(encoded, '\n')
		if writeErr := os.WriteFile(outPath, encoded, 0o600); writeErr != nil {
			return writeActionContractOutput(jsonOutput, actionContractOutput{Operation: "activate", Error: writeErr.Error()}, exitInternalFailure)
		}
	}
	out := actionContractOutput{SchemaID: actioncontract.ActivatedSchemaID, SchemaVersion: actioncontract.ActivatedSchemaVersion, OK: true, Operation: "activate", Proposal: &validation, Activated: &activated}
	return writeActionContractOutput(jsonOutput, out, exitOK)
}

func runActionContractConsume(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"artifact": true, "scenario-id": true})
	flags := actionContractFlags("contract-consume")
	var artifactPath, scenarioID string
	var jsonOutput, help bool
	flags.StringVar(&artifactPath, "artifact", "", "explicit path to one proposed_action_contract artifact")
	flags.StringVar(&scenarioID, "scenario-id", "", "fixture scenario identifier (otherwise inferred from parent directory)")
	flags.BoolVar(&jsonOutput, "json", true, "emit deterministic JSON receipt")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeActionContractReceipt(actionContractReceipt{Consumer: "gait", Version: currentVersion(), Status: "reject", SelfAttestation: false, SemanticResult: actionContractSemanticResult{ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}}, exitInvalidInput)
	}
	if help {
		printActionContractConsumeUsage()
		return exitOK
	}
	if len(flags.Args()) > 0 {
		if artifactPath != "" || len(flags.Args()) != 1 {
			return writeActionContractReceipt(actionContractReceipt{Consumer: "gait", Version: currentVersion(), Status: "reject", SelfAttestation: false, SemanticResult: actionContractSemanticResult{ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}}, exitInvalidInput)
		}
		artifactPath = flags.Args()[0]
	}
	if strings.TrimSpace(artifactPath) == "" {
		return writeActionContractReceipt(actionContractReceipt{Consumer: "gait", Version: currentVersion(), Status: "reject", SelfAttestation: false, SemanticResult: actionContractSemanticResult{ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}}, exitInvalidInput)
	}
	artifact, raw, err := actioncontract.ReadArtifact(artifactPath)
	if err != nil {
		return writeActionContractReceipt(actionContractReceipt{Consumer: "gait", Version: currentVersion(), Status: "reject", SelfAttestation: false, SemanticResult: actionContractSemanticResult{ReasonCodes: actionContractReasonCodes(err)}}, exitVerifyFailed)
	}
	validation := actioncontract.ValidateArtifact(artifact, actioncontract.ValidationOptions{})
	if scenarioID == "" {
		scenarioID = filepath.Base(filepath.Dir(artifactPath))
	}
	structuralReasons := make([]string, 0, len(validation.Reasons))
	for _, reason := range validation.Reasons {
		if reason != actioncontract.ReasonStaleProposal && reason != actioncontract.ReasonContradictoryProposal && !strings.HasPrefix(reason, actioncontract.ReasonSupersededProposal) {
			structuralReasons = append(structuralReasons, reason)
		}
	}
	receipt := actionContractReceipt{Consumer: "gait", Version: currentVersion(), ScenarioID: scenarioID, ArtifactSHA256: actioncontract.RawDigest(raw), Status: "pass", SelfAttestation: false, ProposalArtifactID: artifact.ArtifactID, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID, Revision: artifact.Revision, ResolutionKey: artifact.ResolutionKey, SchemaVersions: actionContractSchemaVersions{Artifact: artifact.SchemaVersion, Contract: artifact.Producer.ContractSchemaVersion}, SupportedConstraints: validation.SupportedConstraints, SemanticResult: actionContractSemanticResult{ProposalValid: len(structuralReasons) == 0, ActivationReady: validation.Valid, ReasonCodes: validation.Reasons, ExecutionClaim: false, EffectClaim: false}}
	code := exitOK
	if len(structuralReasons) > 0 {
		receipt.Status = "reject"
		code = exitVerifyFailed
	}
	return writeActionContractReceipt(receipt, code)
}

func writeActionContractReceipt(receipt actionContractReceipt, code int) int {
	encoded, err := marshalJSON(receipt)
	if err != nil {
		fmt.Println(`{"consumer":"gait","version":"unknown","status":"reject","self_attestation":false}`)
		return exitInternalFailure
	}
	fmt.Println(string(encoded))
	return code
}

func loadActionContractPrivateKey(path, env string) (ed25519.PrivateKey, error) {
	if path == "" && env == "" {
		return nil, nil
	}
	if path != "" && env != "" {
		return nil, fmt.Errorf("set only one private key source")
	}
	if path != "" {
		return proofsign.LoadPrivateKeyBase64(path)
	}
	encoded, ok := os.LookupEnv(env)
	if !ok {
		return nil, fmt.Errorf("private key env not set: %s", env)
	}
	return proofsign.ParsePrivateKeyBase64(encoded)
}

func parseEvaluationTime(value string) (result time.Time) {
	if strings.TrimSpace(value) == "" {
		return result
	}
	result, _ = time.Parse(time.RFC3339, strings.TrimSpace(value))
	return result
}

func parseActionContractCSV(value string) []string {
	values := strings.Split(value, ",")
	result := make([]string, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			result = append(result, strings.TrimSpace(item))
		}
	}
	return result
}

func actionContractSelectionFlagCount(arguments []string) int {
	count := 0
	for _, argument := range arguments {
		if argument == "--proposal" || strings.HasPrefix(argument, "--proposal=") || argument == "--artifact" || strings.HasPrefix(argument, "--artifact=") || argument == "--from" || strings.HasPrefix(argument, "--from=") {
			count++
		}
	}
	return count
}

func actionContractReasonCodes(err error) []string {
	var validationErr *actioncontract.ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Reasons
	}
	return nil
}

func writeActionContractOutput(jsonOutput bool, output actionContractOutput, code int) int {
	if jsonOutput {
		// Contract output is a conformance surface: do not add the process
		// correlation ID or wall-clock metadata used by general CLI telemetry.
		encoded, err := marshalJSON(output)
		if err != nil {
			fmt.Println(`{"ok":false,"operation":"contract","error":"failed to encode output","reason_codes":["artifact_malformed"]}`)
			return exitInternalFailure
		}
		fmt.Println(string(encoded))
		return code
	}
	if output.Error != "" {
		fmt.Printf("contract %s error: %s\n", output.Operation, output.Error)
	} else {
		fmt.Printf("contract %s: ok=%t\n", output.Operation, output.OK)
	}
	return code
}

func printActionContractUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait contract validate --proposal <artifact.json> [--evaluation-time <rfc3339>] [--json]")
	fmt.Println("  gait contract activate --proposal <artifact.json> --policy-digest sha256:<hex> --principal <ref> --authority-ref <ref> --target <target> --environment <env> --mode context_only|enforce_floor|required [--valid-from <rfc3339>] [--valid-until <rfc3339>] [--out <activated.json>] [--json]")
	fmt.Println("  gait contract consume <artifact.json>")
}
func printActionContractValidateUsage() {
	fmt.Println("Usage: gait contract validate --proposal <artifact.json> [--evaluation-time <rfc3339>] [--json]")
}
func printActionContractActivateUsage() {
	fmt.Println("Usage: gait contract activate --proposal <artifact.json> --policy-digest sha256:<hex> --principal <ref> --authority-ref <ref> --target <target> --environment <env> --mode context_only|enforce_floor|required [--valid-from <rfc3339>] [--valid-until <rfc3339>] [--out <activated.json>] [--json]")
}
func printActionContractConsumeUsage() { fmt.Println("Usage: gait contract consume <artifact.json>") }
