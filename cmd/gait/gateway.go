package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Clyra-AI/gait/core/gateway"
	sign "github.com/Clyra-AI/proof/signing"
)

const (
	gatewayOutputSchemaID      = "gait.gateway.output"
	gatewayOutputSchemaVersion = "1.0.0"
)

type gatewayOutput struct {
	SchemaID        string   `json:"schema_id"`
	SchemaVersion   string   `json:"schema_version"`
	OK              bool     `json:"ok"`
	Operation       string   `json:"operation,omitempty"`
	Source          string   `json:"source,omitempty"`
	LogPath         string   `json:"log_path,omitempty"`
	ProofRecordsOut string   `json:"proof_records_out,omitempty"`
	InputEvents     int      `json:"input_events,omitempty"`
	OutputRecords   int      `json:"output_records,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Error           string   `json:"error,omitempty"`
}

func runGateway(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Ingest gateway audit logs and emit signed policy_enforcement proof records for compliance evidence.")
	}
	if len(arguments) == 0 {
		printGatewayUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "ingest":
		return runGatewayIngest(arguments[1:])
	default:
		printGatewayUsage()
		return exitInvalidInput
	}
}

func runGatewayIngest(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"source":          true,
		"log-path":        true,
		"proof-out":       true,
		"key-mode":        true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("gateway-ingest", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var source string
	var logPath string
	var proofOut string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&source, "source", "", "gateway source: kong|docker|mintmcp")
	flagSet.StringVar(&logPath, "log-path", "", "path to gateway log file")
	flagSet.StringVar(&proofOut, "proof-out", "", "optional output path for policy_enforcement proof record JSONL")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGatewayOutput(jsonOutput, gatewayOutput{OK: false, Operation: "ingest", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGatewayIngestUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeGatewayOutput(jsonOutput, gatewayOutput{OK: false, Operation: "ingest", Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(logPath) == "" {
		return writeGatewayOutput(jsonOutput, gatewayOutput{OK: false, Operation: "ingest", Error: "expected --source <kong|docker|mintmcp> and --log-path <path>"}, exitInvalidInput)
	}

	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeGatewayOutput(jsonOutput, gatewayOutput{OK: false, Operation: "ingest", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := gateway.IngestLogs(gateway.IngestOptions{
		Source:            strings.TrimSpace(source),
		LogPath:           strings.TrimSpace(logPath),
		OutputPath:        strings.TrimSpace(proofOut),
		ProducerVersion:   version,
		SigningPrivateKey: keyPair.Private,
	})
	if err != nil {
		return writeGatewayOutput(jsonOutput, gatewayOutput{OK: false, Operation: "ingest", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeGatewayOutput(jsonOutput, gatewayOutput{
		OK:              true,
		Operation:       "ingest",
		Source:          result.Source,
		LogPath:         result.LogPath,
		ProofRecordsOut: result.ProofRecordsOut,
		InputEvents:     result.InputEvents,
		OutputRecords:   result.OutputRecords,
		Warnings:        warnings,
	}, exitOK)
}

func writeGatewayOutput(jsonOutput bool, output gatewayOutput, exitCode int) int {
	output.SchemaID = gatewayOutputSchemaID
	output.SchemaVersion = gatewayOutputSchemaVersion
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("gateway %s error: %s\n", output.Operation, output.Error)
		return exitCode
	}
	fmt.Printf("gateway %s: source=%s input=%d output=%d proof=%s\n", output.Operation, output.Source, output.InputEvents, output.OutputRecords, output.ProofRecordsOut)
	if len(output.Warnings) > 0 {
		fmt.Printf("warnings: %s\n", strings.Join(output.Warnings, "; "))
	}
	return exitCode
}

func printGatewayUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gateway ingest --source <kong|docker|mintmcp> --log-path <path> [--proof-out <policy_enforcement.jsonl>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printGatewayIngestUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait gateway ingest --source <kong|docker|mintmcp> --log-path <path> [--proof-out <policy_enforcement.jsonl>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}
