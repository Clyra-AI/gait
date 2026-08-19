package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/actioncontract"
)

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
	flags.StringVar(&actionPath, "action", "", "explicit path to one runtime action classification JSON")
	flags.StringVar(&inputPath, "input", "", "alias for --action")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractClassifyUsage()
		return exitOK
	}
	if inputPath != "" {
		actionPath = inputPath
	}
	selected := 0
	if strings.TrimSpace(proposalPath) != "" {
		selected++
	}
	if strings.TrimSpace(actionPath) != "" {
		selected++
	}
	if len(flags.Args()) > 0 {
		if selected > 0 || len(flags.Args()) != 1 {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "exactly one input must be selected", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
		}
		actionPath = flags.Args()[0]
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
	} else {
		raw, err := os.ReadFile(actionPath) // #nosec G304 -- explicit operator-selected runtime action path.
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: err.Error()}, exitInvalidInput)
		}
		var input actioncontract.ClassificationInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "classify", Error: "runtime action JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
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
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "input": true, "trusted-validators": true, "trusted-validator": true})
	flags := runtimeActionContractFlags("contract-readiness")
	var proposalPath, inputPath, trustedCSV string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed action contract artifact")
	flags.StringVar(&inputPath, "input", "", "explicit path to a runtime readiness input JSON")
	flags.StringVar(&trustedCSV, "trusted-validators", "", "comma-separated policy-named trusted validator references")
	flags.StringVar(&trustedCSV, "trusted-validator", "", "alias for --trusted-validators")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractReadinessUsage()
		return exitOK
	}
	if proposalPath != "" && inputPath != "" || proposalPath == "" && inputPath == "" {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "exactly one of --proposal or --input is required", ReasonCodes: []string{actioncontract.ReasonAmbiguousSelection}}, exitInvalidInput)
	}
	var result actioncontract.ReadinessResult
	trusted := parseActionContractCSV(trustedCSV)
	if proposalPath != "" {
		artifact, raw, err := actioncontract.ReadArtifact(proposalPath)
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: err.Error(), ReasonCodes: actionContractReasonCodes(err)}, exitInvalidInput)
		}
		_, validation := actioncontract.ValidateArtifactBytes(raw, actioncontract.ValidationOptions{})
		if !validation.Valid {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "proposal validation failed", ReasonCodes: validation.Reasons}, exitVerifyFailed)
		}
		result = actioncontract.ReadinessFromArtifact(artifact, actioncontract.ReadinessInput{TrustedValidatorRefs: trusted})
	} else {
		raw, err := os.ReadFile(inputPath) // #nosec G304 -- explicit operator-selected readiness input path.
		if err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: err.Error()}, exitInvalidInput)
		}
		var input actioncontract.ReadinessInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "readiness", Error: "readiness JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
		}
		if len(trusted) > 0 {
			input.TrustedValidatorRefs = trusted
		}
		result = actioncontract.EvaluateContractReadiness(input)
	}
	out := actionContractRuntimeOutput{SchemaID: actioncontract.RuntimeReadinessSchemaID, SchemaVersion: actioncontract.RuntimeActionSchemaVersion, OK: result.Ready, Operation: "readiness", Readiness: &result, ReasonCodes: result.ReasonCodes}
	code := exitOK
	if !result.Ready {
		code = exitVerifyFailed
	}
	return writeRuntimeOutput(jsonOutput, out, code)
}

func runActionContractExplain(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"proposal": true, "action": true, "input": true})
	flags := runtimeActionContractFlags("contract-explain")
	var proposalPath, actionPath, inputPath string
	var jsonOutput, help bool
	flags.StringVar(&proposalPath, "proposal", "", "explicit path to one proposed action contract artifact")
	flags.StringVar(&actionPath, "action", "", "explicit path to one runtime action classification JSON")
	flags.StringVar(&inputPath, "input", "", "alias for --action")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printActionContractExplainUsage()
		return exitOK
	}
	if inputPath != "" {
		actionPath = inputPath
	}
	if proposalPath == "" && actionPath == "" && len(flags.Args()) == 0 {
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
	if actionPath == "" && len(flags.Args()) == 1 {
		actionPath = flags.Args()[0]
	}
	if actionPath == "" {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "one input must be selected", ReasonCodes: []string{actioncontract.ReasonSelectionRequired}}, exitInvalidInput)
	}
	raw, err := os.ReadFile(actionPath) // #nosec G304 -- explicit operator-selected runtime action path.
	if err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: err.Error()}, exitInvalidInput)
	}
	var input actioncontract.ClassificationInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return writeRuntimeOutput(jsonOutput, actionContractRuntimeOutput{Operation: "explain", Error: "runtime action JSON is malformed", ReasonCodes: []string{actioncontract.ReasonMalformedArtifact}}, exitInvalidInput)
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
