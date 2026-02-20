package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
)

type listScriptsEntry struct {
	PatternID        string   `json:"pattern_id"`
	ApproverIdentity string   `json:"approver_identity"`
	PolicyDigest     string   `json:"policy_digest"`
	ScriptHash       string   `json:"script_hash"`
	ToolSequence     []string `json:"tool_sequence"`
	Scope            []string `json:"scope,omitempty"`
	ExpiresAt        string   `json:"expires_at"`
	Expired          bool     `json:"expired"`
}

type listScriptsOutput struct {
	OK       bool               `json:"ok"`
	Registry string             `json:"registry,omitempty"`
	Count    int                `json:"count,omitempty"`
	Entries  []listScriptsEntry `json:"entries,omitempty"`
	Error    string             `json:"error,omitempty"`
}

func runListScripts(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("List approved-script registry entries with expiry status and deterministic ordering.")
	}
	flagSet := flag.NewFlagSet("list-scripts", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	var registryPath string
	var jsonOutput bool
	var helpFlag bool
	flagSet.StringVar(&registryPath, "registry", "", "path to approved script registry json")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")
	if err := flagSet.Parse(arguments); err != nil {
		return writeListScriptsOutput(jsonOutput, listScriptsOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printListScriptsUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeListScriptsOutput(jsonOutput, listScriptsOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(registryPath) == "" {
		return writeListScriptsOutput(jsonOutput, listScriptsOutput{OK: false, Error: "--registry is required"}, exitInvalidInput)
	}
	entries, err := gate.ReadApprovedScriptRegistry(registryPath)
	if err != nil {
		return writeListScriptsOutput(jsonOutput, listScriptsOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	nowUTC := time.Now().UTC()
	outputEntries := make([]listScriptsEntry, 0, len(entries))
	for _, entry := range entries {
		expiresAt := entry.ExpiresAt.UTC()
		outputEntries = append(outputEntries, listScriptsEntry{
			PatternID:        entry.PatternID,
			ApproverIdentity: entry.ApproverIdentity,
			PolicyDigest:     entry.PolicyDigest,
			ScriptHash:       entry.ScriptHash,
			ToolSequence:     entry.ToolSequence,
			Scope:            entry.Scope,
			ExpiresAt:        expiresAt.Format(time.RFC3339Nano),
			Expired:          !expiresAt.After(nowUTC),
		})
	}
	return writeListScriptsOutput(jsonOutput, listScriptsOutput{
		OK:       true,
		Registry: strings.TrimSpace(registryPath),
		Count:    len(outputEntries),
		Entries:  outputEntries,
	}, exitOK)
}

func writeListScriptsOutput(jsonOutput bool, output listScriptsOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("list-scripts error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("list-scripts: %d entries in %s\n", output.Count, output.Registry)
	for _, entry := range output.Entries {
		status := "active"
		if entry.Expired {
			status = "expired"
		}
		fmt.Printf("- %s (%s) approver=%s expires=%s\n", entry.PatternID, status, entry.ApproverIdentity, entry.ExpiresAt)
	}
	return exitCode
}

func printListScriptsUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait list-scripts --registry <registry.json> [--json] [--explain]")
}
