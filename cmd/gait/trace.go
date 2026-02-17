package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/sign"
)

type traceVerifyOutput struct {
	OK              bool   `json:"ok"`
	Path            string `json:"path,omitempty"`
	TraceID         string `json:"trace_id,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	SignatureStatus string `json:"signature_status,omitempty"`
	KeyID           string `json:"key_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

func runTrace(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify signed gate trace records for offline auditability.")
	}
	if len(arguments) == 0 {
		printTraceUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "verify":
		return runTraceVerify(arguments[1:])
	default:
		printTraceUsage()
		return exitInvalidInput
	}
}

func runTraceVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Validate a gate trace signature and report deterministic verification status.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path":            true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		arguments = append([]string{"--path", arguments[0]}, arguments[1:]...)
	}

	flagSet := flag.NewFlagSet("trace-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var pathValue string
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.StringVar(&pathValue, "path", "", "path to trace record")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printTraceVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if pathValue == "" {
		if len(remaining) != 1 {
			return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{OK: false, Error: "expected trace path"}, exitInvalidInput)
		}
		pathValue = remaining[0]
	} else if len(remaining) > 0 {
		return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{OK: false, Error: "expected trace path"}, exitInvalidInput)
	}

	record, err := gate.ReadTraceRecord(pathValue)
	if err != nil {
		return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	publicKey, err := sign.LoadVerifyKey(sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok, err := gate.VerifyTraceRecordSignature(record, publicKey)
	if err != nil {
		return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{
			OK:              false,
			Path:            pathValue,
			TraceID:         record.TraceID,
			Verdict:         record.Verdict,
			SignatureStatus: "failed",
			Error:           err.Error(),
		}, exitVerifyFailed)
	}

	status := "failed"
	exitCode := exitVerifyFailed
	if ok {
		status = "verified"
		exitCode = exitOK
	}
	keyID := ""
	if record.Signature != nil {
		keyID = record.Signature.KeyID
	}
	return writeTraceVerifyOutput(jsonOutput, traceVerifyOutput{
		OK:              ok,
		Path:            pathValue,
		TraceID:         record.TraceID,
		Verdict:         record.Verdict,
		SignatureStatus: status,
		KeyID:           keyID,
	}, exitCode)
}

func writeTraceVerifyOutput(jsonOutput bool, output traceVerifyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("trace verify ok: %s\n", output.Path)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("trace verify error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("trace verify failed: %s\n", output.Path)
	if output.SignatureStatus != "" {
		fmt.Printf("signature status: %s\n", output.SignatureStatus)
	}
	return exitCode
}

func printTraceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait trace verify <path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
}

func printTraceVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait trace verify <path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
}
