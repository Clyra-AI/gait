package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/guard"
)

type incidentPackOutput struct {
	OK                      bool     `json:"ok"`
	PackPath                string   `json:"pack_path,omitempty"`
	PackID                  string   `json:"pack_id,omitempty"`
	RunID                   string   `json:"run_id,omitempty"`
	TemplateID              string   `json:"template_id,omitempty"`
	TraceCount              int      `json:"trace_count,omitempty"`
	RegressCount            int      `json:"regress_count,omitempty"`
	ApprovalAuditCount      int      `json:"approval_audit_count,omitempty"`
	CredentialEvidenceCount int      `json:"credential_evidence_count,omitempty"`
	PolicyDigests           []string `json:"policy_digests,omitempty"`
	WindowFrom              string   `json:"window_from,omitempty"`
	WindowTo                string   `json:"window_to,omitempty"`
	Error                   string   `json:"error,omitempty"`
}

func runIncident(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build an incident evidence pack from a runpack and a deterministic artifact time window.")
	}
	if len(arguments) == 0 {
		printIncidentUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "pack":
		return runIncidentPack(arguments[1:])
	default:
		printIncidentUsage()
		return exitInvalidInput
	}
}

func runIncidentPack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Bundle runpack, traces, regress outputs, policy digests, and approval artifacts into one deterministic incident pack.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"from":     true,
		"window":   true,
		"out":      true,
		"case-id":  true,
		"template": true,
	})
	flagSet := flag.NewFlagSet("incident-pack", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var window string
	var outPath string
	var caseID string
	var templateID string
	var renderPDF bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or runpack path")
	flagSet.StringVar(&window, "window", "24h", "incident selection window duration around run created_at")
	flagSet.StringVar(&outPath, "out", "", "output incident pack path")
	flagSet.StringVar(&caseID, "case-id", "", "optional incident case id")
	flagSet.StringVar(&templateID, "template", "incident_response", "template id: incident_response|soc2|pci")
	flagSet.BoolVar(&renderPDF, "render-pdf", false, "include summary.pdf convenience artifact")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeIncidentPackOutput(jsonOutput, incidentPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printIncidentPackUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(from) == "" && len(remaining) > 0 {
		from = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(from) == "" || len(remaining) > 0 {
		return writeIncidentPackOutput(jsonOutput, incidentPackOutput{OK: false, Error: "expected --from <run_id|path>"}, exitInvalidInput)
	}

	resolvedRunPath, err := resolveRunpackPath(from)
	if err != nil {
		return writeIncidentPackOutput(jsonOutput, incidentPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	parsedWindow, err := time.ParseDuration(strings.TrimSpace(window))
	if err != nil {
		return writeIncidentPackOutput(jsonOutput, incidentPackOutput{OK: false, Error: fmt.Sprintf("parse --window: %v", err)}, exitInvalidInput)
	}

	result, err := guard.BuildIncidentPack(guard.IncidentPackOptions{
		RunpackPath:     resolvedRunPath,
		OutputPath:      outPath,
		CaseID:          caseID,
		Window:          parsedWindow,
		TemplateID:      templateID,
		RenderPDF:       renderPDF,
		ProducerVersion: version,
	})
	if err != nil {
		return writeIncidentPackOutput(jsonOutput, incidentPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeIncidentPackOutput(jsonOutput, incidentPackOutput{
		OK:                      true,
		PackPath:                result.BuildResult.PackPath,
		PackID:                  result.BuildResult.Manifest.PackID,
		RunID:                   result.BuildResult.Manifest.RunID,
		TemplateID:              result.BuildResult.Manifest.TemplateID,
		TraceCount:              result.TraceCount,
		RegressCount:            result.RegressCount,
		ApprovalAuditCount:      result.ApprovalAuditCount,
		CredentialEvidenceCount: result.CredentialEvidenceCount,
		PolicyDigests:           result.PolicyDigests,
		WindowFrom:              result.WindowFrom.Format(time.RFC3339),
		WindowTo:                result.WindowTo.Format(time.RFC3339),
	}, exitOK)
}

func writeIncidentPackOutput(jsonOutput bool, output incidentPackOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("incident pack ok: %s\n", output.PackPath)
		return exitCode
	}
	fmt.Printf("incident pack error: %s\n", output.Error)
	return exitCode
}

func printIncidentUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait incident pack --from <run_id|path> [--window <duration>] [--template incident_response|soc2|pci] [--render-pdf] [--out <incident_pack.zip>] [--case-id <id>] [--json] [--explain]")
}

func printIncidentPackUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait incident pack --from <run_id|path> [--window <duration>] [--template incident_response|soc2|pci] [--render-pdf] [--out <incident_pack.zip>] [--case-id <id>] [--json] [--explain]")
}
