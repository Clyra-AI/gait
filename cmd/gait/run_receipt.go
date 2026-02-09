package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/runpack"
)

type runReceiptOutput struct {
	OK             bool   `json:"ok"`
	RunID          string `json:"run_id,omitempty"`
	Path           string `json:"path,omitempty"`
	ManifestDigest string `json:"manifest_digest,omitempty"`
	TicketFooter   string `json:"ticket_footer,omitempty"`
	Error          string `json:"error,omitempty"`
}

func runReceipt(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Extract the deterministic ticket footer from an existing runpack artifact without rerunning the original scenario.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"from": true,
	})

	flagSet := flag.NewFlagSet("receipt", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id or runpack path")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunReceiptUsage()
		return exitOK
	}

	remaining := flagSet.Args()
	if strings.TrimSpace(from) == "" && len(remaining) == 1 {
		from = remaining[0]
		remaining = nil
	}
	if strings.TrimSpace(from) == "" {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: "missing required --from <run_id|path>"}, exitInvalidInput)
	}
	if len(remaining) > 0 {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	resolvedPath, err := resolveRunpackPath(from)
	if err != nil {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	verifyResult, err := runpack.VerifyZip(resolvedPath, runpack.VerifyOptions{RequireSignature: false})
	if err != nil {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 || verifyResult.SignatureStatus == "failed" {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: "runpack verification failed"}, exitVerifyFailed)
	}

	ticketFooter := formatTicketFooter(verifyResult.RunID, verifyResult.ManifestDigest)
	if !ticketFooterMatchesContract(ticketFooter) {
		return writeRunReceiptOutput(jsonOutput, runReceiptOutput{OK: false, Error: "ticket footer contract validation failed"}, exitInternalFailure)
	}

	return writeRunReceiptOutput(jsonOutput, runReceiptOutput{
		OK:             true,
		RunID:          verifyResult.RunID,
		Path:           resolvedPath,
		ManifestDigest: verifyResult.ManifestDigest,
		TicketFooter:   ticketFooter,
	}, exitOK)
}

func writeRunReceiptOutput(jsonOutput bool, output runReceiptOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Println(output.TicketFooter)
		return exitCode
	}
	fmt.Printf("receipt error: %s\n", output.Error)
	return exitCode
}

func printRunReceiptUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run receipt --from <run_id|path> [--json] [--explain]")
	fmt.Println("  gait run receipt <run_id|path> [--json] [--explain]")
}
