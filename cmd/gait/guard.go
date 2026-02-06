package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/guard"
)

type guardPackOutput struct {
	OK           bool   `json:"ok"`
	PackPath     string `json:"pack_path,omitempty"`
	PackID       string `json:"pack_id,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
	Error        string `json:"error,omitempty"`
}

type guardVerifyOutput struct {
	OK             bool                 `json:"ok"`
	Path           string               `json:"path,omitempty"`
	PackID         string               `json:"pack_id,omitempty"`
	RunID          string               `json:"run_id,omitempty"`
	FilesChecked   int                  `json:"files_checked,omitempty"`
	MissingFiles   []string             `json:"missing_files,omitempty"`
	HashMismatches []guard.HashMismatch `json:"hash_mismatches,omitempty"`
	Error          string               `json:"error,omitempty"`
}

func runGuard(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build deterministic evidence packs from run artifacts and verify them offline for tampering.")
	}
	if len(arguments) == 0 {
		printGuardUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "pack":
		return runGuardPack(arguments[1:])
	case "verify":
		return runGuardVerify(arguments[1:])
	default:
		printGuardUsage()
		return exitInvalidInput
	}
}

func runGuardPack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build an evidence_pack zip with a canonical pack_manifest.json and evidence summaries.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"run":                 true,
		"out":                 true,
		"case-id":             true,
		"inventory":           true,
		"trace":               true,
		"regress":             true,
		"approval-audit":      true,
		"credential-evidence": true,
	})
	flagSet := flag.NewFlagSet("guard-pack", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var runPath string
	var outPath string
	var caseID string
	var inventoryCSV string
	var traceCSV string
	var regressCSV string
	var approvalAuditCSV string
	var credentialEvidenceCSV string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&runPath, "run", "", "runpack path or run_id")
	flagSet.StringVar(&outPath, "out", "", "output evidence_pack zip path")
	flagSet.StringVar(&caseID, "case-id", "", "optional case identifier")
	flagSet.StringVar(&inventoryCSV, "inventory", "", "comma-separated inventory snapshot paths")
	flagSet.StringVar(&traceCSV, "trace", "", "comma-separated gate trace paths")
	flagSet.StringVar(&regressCSV, "regress", "", "comma-separated regress result paths")
	flagSet.StringVar(&approvalAuditCSV, "approval-audit", "", "comma-separated approval audit record paths")
	flagSet.StringVar(&credentialEvidenceCSV, "credential-evidence", "", "comma-separated broker credential evidence paths")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printGuardPackUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(runPath) == "" && len(remaining) > 0 {
		runPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(runPath) == "" || len(remaining) > 0 {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: "expected --run <run_id|path>"}, exitInvalidInput)
	}

	resolvedRunPath, err := resolveRunpackPath(runPath)
	if err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	result, err := guard.BuildPack(guard.BuildOptions{
		RunpackPath:             resolvedRunPath,
		OutputPath:              outPath,
		CaseID:                  caseID,
		InventoryPaths:          parseCSVList(inventoryCSV),
		TracePaths:              parseCSVList(traceCSV),
		RegressPaths:            parseCSVList(regressCSV),
		ApprovalAuditPaths:      parseCSVList(approvalAuditCSV),
		CredentialEvidencePaths: parseCSVList(credentialEvidenceCSV),
		AutoDiscoverV12:         true,
		ProducerVersion:         version,
	})
	if err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	manifestPath := result.PackPath + "#pack_manifest.json"
	return writeGuardPackOutput(jsonOutput, guardPackOutput{
		OK:           true,
		PackPath:     result.PackPath,
		PackID:       result.Manifest.PackID,
		RunID:        result.Manifest.RunID,
		ManifestPath: manifestPath,
	}, exitOK)
}

func runGuardVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify an evidence_pack zip offline by checking pack manifest hashes deterministically.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path": true,
	})
	flagSet := flag.NewFlagSet("guard-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var pathValue string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&pathValue, "path", "", "path to evidence_pack zip")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printGuardVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(pathValue) == "" && len(remaining) > 0 {
		pathValue = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(pathValue) == "" || len(remaining) > 0 {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: "expected <evidence_pack.zip>"}, exitInvalidInput)
	}

	result, err := guard.VerifyPack(pathValue)
	if err != nil {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{
		OK:             ok,
		Path:           pathValue,
		PackID:         result.PackID,
		RunID:          result.RunID,
		FilesChecked:   result.FilesChecked,
		MissingFiles:   result.MissingFiles,
		HashMismatches: result.HashMismatches,
	}, exitCode)
}

func writeGuardPackOutput(jsonOutput bool, output guardPackOutput, exitCode int) int {
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
		fmt.Printf("guard pack ok: %s\n", output.PackPath)
		return exitCode
	}
	fmt.Printf("guard pack error: %s\n", output.Error)
	return exitCode
}

func writeGuardVerifyOutput(jsonOutput bool, output guardVerifyOutput, exitCode int) int {
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
		fmt.Printf("guard verify ok: %s\n", output.Path)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("guard verify error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("guard verify failed: %s\n", output.Path)
	return exitCode
}

func printGuardUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard pack --run <run_id|path> [--inventory <csv>] [--trace <csv>] [--regress <csv>] [--approval-audit <csv>] [--credential-evidence <csv>] [--out <evidence_pack.zip>] [--case-id <id>] [--json] [--explain]")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--json] [--explain]")
}

func printGuardPackUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard pack --run <run_id|path> [--inventory <csv>] [--trace <csv>] [--regress <csv>] [--approval-audit <csv>] [--credential-evidence <csv>] [--out <evidence_pack.zip>] [--case-id <id>] [--json] [--explain]")
}

func printGuardVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--json] [--explain]")
}
