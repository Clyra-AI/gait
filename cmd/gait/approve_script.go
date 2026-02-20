package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
)

type approveScriptOutput struct {
	OK           bool     `json:"ok"`
	PatternID    string   `json:"pattern_id,omitempty"`
	PolicyDigest string   `json:"policy_digest,omitempty"`
	ScriptHash   string   `json:"script_hash,omitempty"`
	ToolSequence []string `json:"tool_sequence,omitempty"`
	RegistryPath string   `json:"registry_path,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func runApproveScript(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Create a signed approved-script registry entry bound to a policy digest and script hash.")
	}
	flagSet := flag.NewFlagSet("approve-script", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var intentPath string
	var registryPath string
	var patternID string
	var approver string
	var ttlRaw string
	var scopeCSV string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", "", "path to policy yaml")
	flagSet.StringVar(&intentPath, "intent", "", "path to script intent json")
	flagSet.StringVar(&registryPath, "registry", "", "path to approved script registry json")
	flagSet.StringVar(&patternID, "pattern-id", "", "approved script pattern id")
	flagSet.StringVar(&approver, "approver", "", "approver identity")
	flagSet.StringVar(&ttlRaw, "ttl", "168h", "entry validity duration")
	flagSet.StringVar(&scopeCSV, "scope", "", "comma-separated scope values")
	flagSet.StringVar(&keyMode, "key-mode", "dev", "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printApproveScriptUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(policyPath) == "" || strings.TrimSpace(intentPath) == "" || strings.TrimSpace(registryPath) == "" || strings.TrimSpace(approver) == "" {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{
			OK:    false,
			Error: "--policy, --intent, --registry, and --approver are required",
		}, exitInvalidInput)
	}

	ttl, err := time.ParseDuration(strings.TrimSpace(ttlRaw))
	if err != nil || ttl <= 0 {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: "invalid --ttl duration"}, exitInvalidInput)
	}
	policy, err := gate.LoadPolicyFile(policyPath)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	policyDigest, err := gate.PolicyDigest(policy)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	intent, err := readIntentRequest(intentPath)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	normalizedIntent, err := gate.NormalizeIntent(intent)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if normalizedIntent.Script == nil || len(normalizedIntent.Script.Steps) == 0 {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: "intent must include script.steps"}, exitInvalidInput)
	}
	scriptHash, err := gate.ScriptHash(normalizedIntent)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	toolSequence := make([]string, 0, len(normalizedIntent.Script.Steps))
	for _, step := range normalizedIntent.Script.Steps {
		toolSequence = append(toolSequence, step.ToolName)
	}

	if strings.TrimSpace(patternID) == "" {
		patternID = "pattern_" + scriptHash[:12]
	}
	nowUTC := time.Now().UTC()
	keyPair, _, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	entry, err := gate.SignApprovedScriptEntry(schemagate.ApprovedScriptEntry{
		SchemaID:         "gait.gate.approved_script_entry",
		SchemaVersion:    "1.0.0",
		CreatedAt:        nowUTC,
		ProducerVersion:  version,
		PatternID:        strings.TrimSpace(patternID),
		PolicyDigest:     policyDigest,
		ScriptHash:       scriptHash,
		ToolSequence:     toolSequence,
		Scope:            parseCSV(scopeCSV),
		ApproverIdentity: strings.TrimSpace(approver),
		ExpiresAt:        nowUTC.Add(ttl),
	}, keyPair.Private)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	existing, err := gate.ReadApprovedScriptRegistry(registryPath)
	if err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	updated := make([]schemagate.ApprovedScriptEntry, 0, len(existing)+1)
	replaced := false
	for _, candidate := range existing {
		if candidate.PatternID == entry.PatternID {
			updated = append(updated, entry)
			replaced = true
			continue
		}
		updated = append(updated, candidate)
	}
	if !replaced {
		updated = append(updated, entry)
	}
	if err := gate.WriteApprovedScriptRegistry(registryPath, updated); err != nil {
		return writeApproveScriptOutput(jsonOutput, approveScriptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeApproveScriptOutput(jsonOutput, approveScriptOutput{
		OK:           true,
		PatternID:    entry.PatternID,
		PolicyDigest: entry.PolicyDigest,
		ScriptHash:   entry.ScriptHash,
		ToolSequence: entry.ToolSequence,
		RegistryPath: strings.TrimSpace(registryPath),
		ExpiresAt:    entry.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, exitOK)
}

func writeApproveScriptOutput(jsonOutput bool, output approveScriptOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("approve-script error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("approve-script: pattern=%s registry=%s\n", output.PatternID, output.RegistryPath)
	fmt.Printf("policy_digest=%s script_hash=%s\n", output.PolicyDigest, output.ScriptHash)
	return exitCode
}

func printApproveScriptUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait approve-script --policy <policy.yaml> --intent <script_intent.json> --registry <registry.json> --approver <identity> [--pattern-id <id>] [--ttl 168h] [--scope <csv>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}
