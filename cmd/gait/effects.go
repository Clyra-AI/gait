package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Clyra-AI/gait/core/effects"
)

type effectsOutput struct {
	OK     bool                `json:"ok"`
	Result effects.GradeResult `json:"result"`
	JUnit  string              `json:"junit,omitempty"`
	Error  string              `json:"error,omitempty"`
}

func runEffects(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Grade a deterministic effect contract against a bounded before/after effect snapshot without executing external effects.")
	}
	if len(arguments) == 0 || (len(arguments) == 1 && isTopLevelHelp(arguments[0])) {
		printEffectsUsage()
		if len(arguments) == 0 {
			return exitInvalidInput
		}
		return exitOK
	}
	if arguments[0] != "grade" {
		printEffectsUsage()
		return exitInvalidInput
	}
	return runEffectsGrade(arguments[1:])
}

func runEffectsGrade(arguments []string) int {
	flags := flag.NewFlagSet("effects-grade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var snapshotPath, contractPath, junitPath, trustedCollectorKeyPath string
	var jsonOutput, help, allowFixtureTestProvenance bool
	flags.StringVar(&snapshotPath, "snapshot", "", "effect snapshot evidence JSON")
	flags.StringVar(&contractPath, "contract", "", "effect contract JSON")
	flags.StringVar(&junitPath, "junit", "", "optional deterministic JUnit output path")
	flags.StringVar(&trustedCollectorKeyPath, "trusted-collector-key", "", "trusted Ed25519 collector public key path (required for pass)")
	flags.BoolVar(&allowFixtureTestProvenance, "allow-fixture-test-provenance", false, "TEST ONLY: allow fixture_test_only provenance")
	flags.BoolVar(&jsonOutput, "json", false, "emit deterministic JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printEffectsGradeUsage()
		return exitOK
	}
	if strings.TrimSpace(snapshotPath) == "" || strings.TrimSpace(contractPath) == "" || strings.TrimSpace(trustedCollectorKeyPath) == "" || len(flags.Args()) > 0 {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: "--snapshot, --contract, and --trusted-collector-key are required; positional arguments are not accepted"}, exitInvalidInput)
	}
	snapshot, err := effects.LoadSnapshot(snapshotPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	contract, err := effects.LoadContract(contractPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	trustedKey, err := effects.LoadPublicKey(trustedCollectorKeyPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	result := effects.GradeWithOptions(snapshot, contract, effects.GradeOptions{TrustedCollectorPublicKey: trustedKey, AllowFixtureTestProvenance: allowFixtureTestProvenance})
	if strings.TrimSpace(junitPath) != "" {
		if err := effects.WriteJUnit(junitPath, result); err != nil {
			return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Result: result, Error: err.Error()}, exitInternalFailure)
		}
	}
	code := exitOK
	if result.Status != effects.GradePass {
		code = exitVerifyFailed
	}
	return writeEffectsOutput(jsonOutput, effectsOutput{OK: code == exitOK, Result: result, JUnit: junitPath}, code)
}

func writeEffectsOutput(jsonOutput bool, output effectsOutput, code int) int {
	if jsonOutput {
		encoded, err := marshalJSON(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode effects output"}`)
			return exitInternalFailure
		}
		fmt.Println(string(encoded))
		return code
	}
	if output.Error != "" {
		fmt.Printf("effects grade error: %s\n", output.Error)
	} else {
		fmt.Printf("effects grade: status=%s\n", output.Result.Status)
	}
	return code
}

func printEffectsUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait effects grade --snapshot <effect_snapshot.json> --contract <effect_contract.json> --trusted-collector-key <public-key> [--allow-fixture-test-provenance] [--junit <junit.xml>] [--json] [--explain]")
}

func printEffectsGradeUsage() {
	fmt.Println("Usage: gait effects grade --snapshot <effect_snapshot.json> --contract <effect_contract.json> --trusted-collector-key <public-key> [--allow-fixture-test-provenance] [--junit <junit.xml>] [--json] [--explain]")
}
