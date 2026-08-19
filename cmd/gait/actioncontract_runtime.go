package main

import (
	"crypto/ed25519"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

type repeatStringFlag []string

func (f *repeatStringFlag) String() string { return strings.Join(*f, ",") }
func (f *repeatStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type actionContractRuntimeOutput struct {
	SchemaID       string                               `json:"schema_id"`
	SchemaVersion  string                               `json:"schema_version"`
	OK             bool                                 `json:"ok"`
	Operation      string                               `json:"operation"`
	Classification *actioncontract.ClassificationResult `json:"classification,omitempty"`
	Readiness      *actioncontract.ReadinessResult      `json:"readiness,omitempty"`
	ReasonCodes    []string                             `json:"reason_codes,omitempty"`
	Error          string                               `json:"error,omitempty"`
}

func runtimeActionContractFlags(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func runActionContractClassify(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "action": true, "input": true})
	flags := runtimeActionContractFlags("contract-classify")
	var proposalPath, actionPath, inputPath string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed action contract artifact")
	flags.StringVar(&actionPath, "action", "", "explicit path to one schema-validated runtime-action artifact")
	flags.StringVar(&inputPath, "input", "", "explicit path to raw ClassificationInput JSON (heuristic input)")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractClassifyUsage()
		return exitOK
	}
	selected := 0
	if strings.TrimSpace(proposalPath) != "" {
		selected++
	}
	if strings.TrimSpace(actionPath) != "" {
		selected++
	}
	if strings.TrimSpace(inputPath) != "" {
		selected++
	}
	if len(flags.Args()) > 0 {
		if selected > 0 || len(flags.Args()) != 1 {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "exactly one input must be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
		}
		inputPath = flags.Args()[0]
		selected++
	}
	if selected != 1 {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "--proposal or --action is required", ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}, exitInvalidInput)
	}
	var result actioncontract.ClassificationResult
	if proposalPath != "" {
		artifact, raw, err := actioncontract.ReadArtifact(proposalPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
		}
		_, validation := actioncontract.ValidateArtifactBytes(raw, actioncontract.ValidationOptions{})
		if !validation.Valid {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Classification: &actioncontract.ClassificationResult{Valid: false, ReasonCodes: validation.Reasons}, Error: "proposal validation failed", ReasonCodes: validation.Reasons}, exitVerifyFailed)
		}
		result = actioncontract.ClassifyArtifact(artifact)
	} else if actionPath != "" {
		raw, err := actioncontract.ReadRuntimeInput(actionPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "runtime input is unreadable", ReasonCodes: []string{"runtime_input_unreadable"}}, exitInvalidInput)
		}
		action, err := actioncontract.ParseRuntimeAction(raw)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "runtime action JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
		}
		result = actioncontract.ClassificationResult{Action: action, Valid: true}
	} else {
		raw, err := actioncontract.ReadRuntimeInput(inputPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "runtime input is unreadable", ReasonCodes: []string{"runtime_input_unreadable"}}, exitInvalidInput)
		}
		input, err := actioncontract.ParseClassificationInput(raw)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "classification input JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
		}
		result = actioncontract.ClassifyAction(input)
	}
	out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeActionSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: result.Valid, Operation: "classify", Classification: &result, ReasonCodes: result.ReasonCodes}
	code := exitOK
	if !result.Valid {
		code = exitVerifyFailed
	}
	return writeRuntimeOutput(jsonOutput, out, code)
}

func runActionContractReadiness(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "input": true, "trusted-validators": true, "trusted-validator": true, "trusted-validator-key": true, "evaluation-time": true, "policy-digest": true})
	flags := runtimeActionContractFlags("contract-readiness")
	var proposalPath, inputPath, trustedCSV, evaluationTimeValue, policyDigest string
	var trustedKeyFlags repeatStringFlag
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed action contract artifact")
	flags.StringVar(&inputPath, "input", "", "explicit path to a runtime readiness input JSON")
	flags.StringVar(&trustedCSV, "trusted-validators", "", "comma-separated policy-named trusted validator references")
	flags.StringVar(&trustedCSV, "trusted-validator", "", "alias for --trusted-validators")
	flags.StringVar(&evaluationTimeValue, "evaluation-time", "", "required fixed UTC RFC3339 evaluation time")
	flags.StringVar(&policyDigest, "policy-digest", "", "policy digest bound into validator claims")
	flags.Var(&trustedKeyFlags, "trusted-validator-key", "authoritative validator key as producer=public-key-path (repeatable)")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractReadinessUsage()
		return exitOK
	}
	evaluationTime, evaluationErr := parseEvaluationTime(evaluationTimeValue)
	if evaluationErr != nil || evaluationTime.IsZero() {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "--evaluation-time is required and must be RFC3339", ReasonCodes: []string{actioncontract.ReasonEvaluationTimeInvalid}}, exitInvalidInput)
	}
	if proposalPath != "" && inputPath != "" || proposalPath == "" && inputPath == "" {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "exactly one of --proposal or --input is required", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
	}
	var result actioncontract.ReadinessResult
	trusted := parseActionContractCSV(trustedCSV)
	trustedKeys, keyErr := loadTrustedValidatorKeys(trustedKeyFlags)
	if keyErr != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "trusted validator key input is invalid", ReasonCodes: []string{"validator_key_invalid"}}, exitInvalidInput)
	}
	if proposalPath != "" {
		artifact, raw, err := actioncontract.ReadArtifact(proposalPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
		}
		_, validation := actioncontract.ValidateArtifactBytes(raw, actioncontract.ValidationOptions{})
		if !validation.Valid {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "proposal validation failed", ReasonCodes: validation.Reasons}, exitVerifyFailed)
		}
		result = actioncontract.ReadinessFromArtifact(artifact, actioncontract.ReadinessInput{Now: evaluationTime, PolicyDigest: policyDigest, TrustedValidatorRefs: trusted, TrustedValidatorKeys: trustedKeys})
	} else {
		raw, err := actioncontract.ReadRuntimeInput(inputPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "runtime input is unreadable", ReasonCodes: []string{"runtime_input_unreadable"}}, exitInvalidInput)
		}
		var input actioncontract.ReadinessInput
		if err := actioncontract.DecodeStrictRuntimeJSON(raw, &input); err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "readiness JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
		}
		if len(trusted) > 0 {
			input.TrustedValidatorRefs = trusted
		}
		if policyDigest != "" {
			input.PolicyDigest = policyDigest
		}
		input.TrustedValidatorKeys = trustedKeys
		input.Now = evaluationTime
		result = actioncontract.EvaluateContractReadiness(input)
	}
	out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: result.Ready, Operation: "readiness", Readiness: &result, ReasonCodes: result.ReasonCodes}
	code := exitOK
	if !result.Ready {
		code = exitVerifyFailed
	}
	return writeRuntimeOutput(jsonOutput, out, code)
}

func loadTrustedValidatorKeys(values []string) (map[string]ed25519.PublicKey, error) {
	keys := make(map[string]ed25519.PublicKey, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("trusted validator key must be producer=path")
		}
		raw, err := actioncontract.ReadRuntimeInput(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
		key, err := proofsign.ParsePublicKeyBase64(string(raw))
		if err != nil {
			return nil, err
		}
		keys[normalizeRuntimeKey(parts[0])] = key
	}
	return keys, nil
}

func normalizeRuntimeKey(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func runActionContractExplain(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "action": true, "input": true})
	flags := runtimeActionContractFlags("contract-explain")
	var proposalPath, actionPath, inputPath string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed action contract artifact")
	flags.StringVar(&actionPath, "action", "", "explicit path to one schema-validated runtime-action artifact")
	flags.StringVar(&inputPath, "input", "", "explicit path to raw ClassificationInput JSON (heuristic input)")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractExplainUsage()
		return exitOK
	}
	selected := 0
	if strings.TrimSpace(proposalPath) != "" {
		selected++
	}
	if strings.TrimSpace(actionPath) != "" {
		selected++
	}
	if strings.TrimSpace(inputPath) != "" {
		selected++
	}
	if len(flags.Args()) > 0 {
		selected += len(flags.Args())
	}
	if selected > 1 {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "exactly one input must be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
	}
	if proposalPath == "" && actionPath == "" && inputPath == "" && len(flags.Args()) == 0 {
		if jsonOutput {
			return writeRuntimeOutput(true, actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeActionSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: true, Operation: "explain"}, exitOK)
		}
		fmt.Println("contract explain: classification is deterministic and pre-execution; readiness fails closed for required missing or inconclusive evidence")
		return exitOK
	}
	if proposalPath != "" {
		code := runActionContractExplainProposal(proposalPath, jsonOutput)
		return code
	}
	if actionPath == "" && inputPath == "" && len(flags.Args()) == 1 {
		inputPath = flags.Args()[0]
	}
	if actionPath == "" && inputPath == "" {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "one input must be selected", ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}, exitInvalidInput)
	}
	if actionPath != "" {
		raw, err := actioncontract.ReadRuntimeInput(actionPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "runtime input is unreadable", ReasonCodes: []string{"runtime_input_unreadable"}}, exitInvalidInput)
		}
		action, parseErr := actioncontract.ParseRuntimeAction(raw)
		if parseErr != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "runtime action JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
		}
		classification := actioncontract.ClassificationResult{Action: action, Valid: true}
		out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeActionSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: true, Operation: "explain", Classification: &classification}
		return writeRuntimeOutput(jsonOutput, out, exitOK)
	}
	raw, err := actioncontract.ReadRuntimeInput(inputPath)
	if err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "runtime input is unreadable", ReasonCodes: []string{"runtime_input_unreadable"}}, exitInvalidInput)
	}
	input, err := actioncontract.ParseClassificationInput(raw)
	if err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "classification input JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
	}
	classification := actioncontract.ClassifyAction(input)
	out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeActionSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: classification.Valid, Operation: "explain", Classification: &classification, ReasonCodes: classification.ReasonCodes}
	code := exitOK
	if !classification.Valid {
		code = exitVerifyFailed
	}
	return writeRuntimeOutput(jsonOutput, out, code)
}

func runActionContractExplainProposal(path string, jsonOutput bool) int {
	artifact, raw, err := actioncontract.ReadArtifact(path)
	if err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
	}
	_, validation := actioncontract.ValidateArtifactBytes(raw, actioncontract.ValidationOptions{})
	if !validation.Valid {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "proposal validation failed", ReasonCodes: validation.Reasons}, exitVerifyFailed)
	}
	classification := actioncontract.ClassifyArtifact(artifact)
	readiness := actioncontract.ReadinessFromArtifact(artifact, actioncontract.ReadinessInput{})
	reasons := append(append([]string(nil), classification.ReasonCodes...), readiness.ReasonCodes...)
	reasons = sortedRuntimeReasonCodes(reasons)
	out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeActionSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: classification.Valid && readiness.Ready, Operation: "explain", Classification: &classification, Readiness: &readiness, ReasonCodes: reasons}
	code := exitOK
	if !out.OK {
		code = exitVerifyFailed
	}
	return writeRuntimeOutput(jsonOutput, out, code)
}

func sortedRuntimeReasonCodes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func writeRuntimeOutput(jsonOutput bool, output actionContractRuntimeOutput, code int) int {
	if jsonOutput {
		encoded, err := marshalJSON(output)
		if err != nil {
			fmt.Println(`{"ok":false,"operation":"runtime","reason_codes":["artifact_malformed"]}`)
			return exitInternalFailure
		}
		fmt.Println(string(encoded))
		return code
	}
	if output.Error != "" {
		fmt.Printf("contract %s error: %s\n", output.Operation, output.Error)
		return code
	}
	fmt.Printf("contract %s: ok=%t\n", output.Operation, output.OK)
	if len(output.ReasonCodes) > 0 {
		fmt.Printf("reason codes: %s\n", strings.Join(output.ReasonCodes, ","))
	}
	return code
}
