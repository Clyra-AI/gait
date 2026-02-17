package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clyra-AI/gait/core/contextproof"
	"github.com/Clyra-AI/gait/core/runpack"
	schemacontext "github.com/Clyra-AI/gait/core/schema/v1/context"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/sign"
)

type runRecordInput struct {
	Run         schemarunpack.Run            `json:"run"`
	Intents     []schemarunpack.IntentRecord `json:"intents"`
	Results     []schemarunpack.ResultRecord `json:"results"`
	Refs        schemarunpack.Refs           `json:"refs"`
	CaptureMode string                       `json:"capture_mode"`
}

type runRecordOutput struct {
	OK              bool     `json:"ok"`
	RunID           string   `json:"run_id,omitempty"`
	Bundle          string   `json:"bundle,omitempty"`
	ManifestDigest  string   `json:"manifest_digest,omitempty"`
	SignatureStatus string   `json:"signature_status,omitempty"`
	SignatureKeyID  string   `json:"signature_key_id,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	TicketFooter    string   `json:"ticket_footer,omitempty"`
	Error           string   `json:"error,omitempty"`
}

func runRecord(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Create a signed and verifiable runpack zip from normalized run, intent, result, and reference receipt records.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"input":                 true,
		"out-dir":               true,
		"run-id":                true,
		"capture-mode":          true,
		"context-envelope":      true,
		"context-evidence-mode": true,
		"unsafe-context-raw":    false,
		"key-mode":              true,
		"private-key":           true,
		"private-key-env":       true,
	})

	flagSet := flag.NewFlagSet("record", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var inputPath string
	var outDir string
	var runIDOverride string
	var captureMode string
	var jsonOutput bool
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var contextEnvelopePath string
	var contextEvidenceMode string
	var unsafeContextRaw bool
	var helpFlag bool

	flagSet.StringVar(&inputPath, "input", "", "path to run record JSON input")
	flagSet.StringVar(&outDir, "out-dir", "./gait-out", "directory for generated runpack")
	flagSet.StringVar(&runIDOverride, "run-id", "", "optional run_id override")
	flagSet.StringVar(&captureMode, "capture-mode", "", "capture mode override: reference|raw")
	flagSet.StringVar(&keyMode, "key-mode", "", "signing key mode: dev|prod (optional)")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key (optional)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key (optional)")
	flagSet.StringVar(&contextEnvelopePath, "context-envelope", "", "path to context evidence envelope JSON (optional)")
	flagSet.StringVar(&contextEvidenceMode, "context-evidence-mode", "", "context evidence mode: best_effort|required (optional)")
	flagSet.BoolVar(&unsafeContextRaw, "unsafe-context-raw", false, "allow context receipts with redaction_mode=raw (unsafe)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRecordUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(inputPath) == "" && len(remaining) == 1 {
		inputPath = remaining[0]
		remaining = nil
	}
	if len(remaining) > 0 {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(inputPath) == "" {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "missing required --input <run_record.json>",
		}, exitInvalidInput)
	}

	recordInput, err := readRunRecordInput(inputPath)
	if err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if strings.TrimSpace(runIDOverride) != "" {
		recordInput.Run.RunID = strings.TrimSpace(runIDOverride)
		recordInput.Refs.RunID = recordInput.Run.RunID
		for i := range recordInput.Intents {
			recordInput.Intents[i].RunID = recordInput.Run.RunID
		}
		for i := range recordInput.Results {
			recordInput.Results[i].RunID = recordInput.Run.RunID
		}
	}
	if strings.TrimSpace(recordInput.Run.RunID) == "" {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "input run.run_id is required (or set --run-id)",
		}, exitInvalidInput)
	}

	resolvedCaptureMode := strings.TrimSpace(captureMode)
	if resolvedCaptureMode == "" {
		resolvedCaptureMode = strings.TrimSpace(recordInput.CaptureMode)
	}
	if resolvedCaptureMode == "" {
		resolvedCaptureMode = "reference"
	}
	if resolvedCaptureMode != "reference" && resolvedCaptureMode != "raw" {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "capture mode must be one of: reference, raw",
		}, exitInvalidInput)
	}
	resolvedContextEvidenceMode := strings.TrimSpace(contextEvidenceMode)
	if resolvedContextEvidenceMode == "" {
		resolvedContextEvidenceMode = strings.TrimSpace(recordInput.Refs.ContextEvidenceMode)
	}
	normalizedContextEvidenceMode, err := contextproof.NormalizeEvidenceMode(resolvedContextEvidenceMode)
	if err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: err.Error(),
		}, exitInvalidInput)
	}
	if normalizedContextEvidenceMode == "" {
		normalizedContextEvidenceMode = contextproof.EvidenceModeBestEffort
	}
	recordInput.Refs.ContextEvidenceMode = normalizedContextEvidenceMode

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	zipPath := filepath.Join(outDir, fmt.Sprintf("runpack_%s.zip", recordInput.Run.RunID))

	signingRequested := strings.TrimSpace(keyMode) != "" || strings.TrimSpace(privateKeyPath) != "" || strings.TrimSpace(privateKeyEnv) != ""
	signingKey := sign.KeyPair{}
	signingWarnings := []string{}
	if strings.TrimSpace(contextEnvelopePath) != "" {
		envelope, loadErr := contextproof.LoadEnvelope(strings.TrimSpace(contextEnvelopePath))
		if loadErr != nil {
			if normalizedContextEvidenceMode == contextproof.EvidenceModeRequired {
				return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: loadErr.Error()}, exitInvalidInput)
			}
			signingWarnings = append(signingWarnings, "context envelope ignored: "+loadErr.Error())
		} else {
			envelope.EvidenceMode = normalizedContextEvidenceMode
			if !unsafeContextRaw && hasRawContextRecord(envelope.Records) {
				return writeRunRecordOutput(jsonOutput, runRecordOutput{
					OK:    false,
					Error: "context envelope includes redaction_mode=raw; re-run with --unsafe-context-raw to allow unsafe context capture",
				}, exitInvalidInput)
			}
			contextproof.ApplyEnvelopeToRefs(&recordInput.Refs, envelope)
		}
	}
	if !unsafeContextRaw && hasRawContextReceipts(recordInput.Refs.Receipts) {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "refs receipts include redaction_mode=raw; re-run with --unsafe-context-raw to allow unsafe context capture",
		}, exitInvalidInput)
	}
	if recordInput.Refs.ContextRefCount == 0 && len(recordInput.Refs.Receipts) > 0 {
		recordInput.Refs.ContextRefCount = len(recordInput.Refs.Receipts)
	}
	if normalizedContextEvidenceMode == contextproof.EvidenceModeRequired && strings.TrimSpace(recordInput.Refs.ContextSetDigest) == "" {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "context evidence mode required but context_set_digest is missing",
		}, exitInvalidInput)
	}
	if signingRequested {
		resolvedMode := strings.ToLower(strings.TrimSpace(keyMode))
		if resolvedMode == "" {
			resolvedMode = string(sign.ModeProd)
		}
		loadedKey, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
			Mode:           sign.KeyMode(resolvedMode),
			PrivateKeyPath: strings.TrimSpace(privateKeyPath),
			PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
		})
		if err != nil {
			return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
		signingKey = loadedKey
		signingWarnings = append(signingWarnings, warnings...)
	}

	result, err := runpack.WriteRunpack(zipPath, runpack.RecordOptions{
		Run:         recordInput.Run,
		Intents:     recordInput.Intents,
		Results:     recordInput.Results,
		Refs:        recordInput.Refs,
		CaptureMode: resolvedCaptureMode,
		SignKey:     signingKey.Private,
	})
	if err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	verifyResult, err := runpack.VerifyZip(zipPath, runpack.VerifyOptions{RequireSignature: false})
	if err != nil {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return writeRunRecordOutput(jsonOutput, runRecordOutput{
			OK:    false,
			Error: "recorded runpack failed verification",
		}, exitVerifyFailed)
	}

	ticketFooter := formatTicketFooter(recordInput.Run.RunID, result.Manifest.ManifestDigest)
	signatureStatus := "missing"
	signatureKeyID := ""
	if len(result.Manifest.Signatures) > 0 {
		signatureStatus = "signed"
		signatureKeyID = strings.TrimSpace(result.Manifest.Signatures[0].KeyID)
	}

	return writeRunRecordOutput(jsonOutput, runRecordOutput{
		OK:              true,
		RunID:           recordInput.Run.RunID,
		Bundle:          displayOutputPath(zipPath),
		ManifestDigest:  result.Manifest.ManifestDigest,
		SignatureStatus: signatureStatus,
		SignatureKeyID:  signatureKeyID,
		Warnings:        signingWarnings,
		TicketFooter:    ticketFooter,
	}, exitOK)
}

func readRunRecordInput(path string) (runRecordInput, error) {
	// #nosec G304 -- explicit user-supplied local file path.
	content, err := os.ReadFile(path)
	if err != nil {
		return runRecordInput{}, fmt.Errorf("read input: %w", err)
	}
	var input runRecordInput
	if err := json.Unmarshal(content, &input); err != nil {
		return runRecordInput{}, fmt.Errorf("parse input json: %w", err)
	}
	return input, nil
}

func displayOutputPath(path string) string {
	if filepath.IsAbs(path) || strings.HasPrefix(path, ".") {
		return path
	}
	return "./" + path
}

func writeRunRecordOutput(jsonOutput bool, output runRecordOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("run_id=%s\n", output.RunID)
		fmt.Printf("bundle=%s\n", output.Bundle)
		fmt.Printf("signature_status=%s\n", output.SignatureStatus)
		if output.SignatureKeyID != "" {
			fmt.Printf("signature_key_id=%s\n", output.SignatureKeyID)
		}
		if len(output.Warnings) > 0 {
			fmt.Printf("warnings=%s\n", strings.Join(output.Warnings, ";"))
		}
		fmt.Printf("ticket_footer=%s\n", output.TicketFooter)
		return exitCode
	}
	fmt.Printf("record error: %s\n", output.Error)
	return exitCode
}

func printRecordUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run record --input <run_record.json> [--out-dir gait-out] [--run-id <run_id>] [--capture-mode reference|raw] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--context-envelope <path>] [--context-evidence-mode best_effort|required] [--unsafe-context-raw] [--json] [--explain]")
	fmt.Println("  gait run record <run_record.json> [--out-dir gait-out] [--run-id <run_id>] [--capture-mode reference|raw] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--context-envelope <path>] [--context-evidence-mode best_effort|required] [--unsafe-context-raw] [--json] [--explain]")
}

func hasRawContextRecord(records []schemacontext.ReferenceRecord) bool {
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.RedactionMode), "raw") {
			return true
		}
	}
	return false
}

func hasRawContextReceipts(receipts []schemarunpack.RefReceipt) bool {
	for _, receipt := range receipts {
		if strings.EqualFold(strings.TrimSpace(receipt.RedactionMode), "raw") {
			return true
		}
	}
	return false
}
