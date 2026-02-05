package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/davidahmann/gait/core/gate"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

type gateEvalOutput struct {
	OK          bool     `json:"ok"`
	Verdict     string   `json:"verdict,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Violations  []string `json:"violations,omitempty"`
	Error       string   `json:"error,omitempty"`
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
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml")
	flagSet.StringVar(&intentPath, "intent", "", "path to intent request json")
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

	result, err := gate.EvaluatePolicy(policy, intent, gate.EvalOptions{ProducerVersion: version})
	if err != nil {
		return writeGateEvalOutput(jsonOutput, gateEvalOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	return writeGateEvalOutput(jsonOutput, gateEvalOutput{
		OK:          true,
		Verdict:     result.Verdict,
		ReasonCodes: result.ReasonCodes,
		Violations:  result.Violations,
	}, exitOK)
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
		if len(output.ReasonCodes) > 0 {
			fmt.Printf("reasons: %s\n", joinCSV(output.ReasonCodes))
		}
		if len(output.Violations) > 0 {
			fmt.Printf("violations: %s\n", joinCSV(output.Violations))
		}
		return exitCode
	}
	fmt.Printf("gate eval error: %s\n", output.Error)
	return exitCode
}

func printGateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--json]")
}

func printGateEvalUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--json]")
}

func joinCSV(values []string) string {
	return strings.Join(values, ",")
}
