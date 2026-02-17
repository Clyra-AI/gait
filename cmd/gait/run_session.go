package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Clyra-AI/gait/core/runpack"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

type runSessionOutput struct {
	OK         bool                             `json:"ok"`
	Operation  string                           `json:"operation,omitempty"`
	Journal    string                           `json:"journal,omitempty"`
	ChainPath  string                           `json:"chain_path,omitempty"`
	Compaction *runpack.SessionCompactionResult `json:"compaction,omitempty"`
	Status     *runpack.SessionStatus           `json:"status,omitempty"`
	Event      *schemarunpack.SessionEvent      `json:"event,omitempty"`
	Checkpoint *schemarunpack.SessionCheckpoint `json:"checkpoint,omitempty"`
	Error      string                           `json:"error,omitempty"`
}

func runSession(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Manage append-only long-running session journals and emit verifiable checkpoint runpacks with chain linkage.")
	}
	if len(arguments) == 0 {
		printRunSessionUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "start":
		return runSessionStart(arguments[1:])
	case "append":
		return runSessionAppend(arguments[1:])
	case "status":
		return runSessionStatus(arguments[1:])
	case "checkpoint":
		return runSessionCheckpoint(arguments[1:])
	case "compact":
		return runSessionCompact(arguments[1:])
	default:
		printRunSessionUsage()
		return exitInvalidInput
	}
}

func runSessionStart(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"journal":    true,
		"session-id": true,
		"run-id":     true,
	})
	flagSet := flag.NewFlagSet("run-session-start", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var journal string
	var sessionID string
	var runID string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&journal, "journal", "", "path to session journal JSONL")
	flagSet.StringVar(&sessionID, "session-id", "", "session identifier")
	flagSet.StringVar(&runID, "run-id", "", "run identifier")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "start",
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunSessionStartUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "start",
			Error:     "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(journal) == "" || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(runID) == "" {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "start",
			Error:     "--journal, --session-id, and --run-id are required",
		}, exitInvalidInput)
	}

	status, err := runpack.StartSession(journal, runpack.SessionStartOptions{
		SessionID:       sessionID,
		RunID:           runID,
		ProducerVersion: version,
	})
	if err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "start",
			Journal:   strings.TrimSpace(journal),
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}

	return writeRunSessionOutput(jsonOutput, runSessionOutput{
		OK:        true,
		Operation: "start",
		Journal:   strings.TrimSpace(journal),
		Status:    &status,
	}, exitOK)
}

func runSessionAppend(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"journal":       true,
		"tool":          true,
		"verdict":       true,
		"intent-id":     true,
		"trace-id":      true,
		"trace-path":    true,
		"intent-digest": true,
		"policy-digest": true,
		"reason-codes":  true,
		"violations":    true,
	})
	flagSet := flag.NewFlagSet("run-session-append", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var journal string
	var toolName string
	var verdict string
	var intentID string
	var traceID string
	var tracePath string
	var intentDigest string
	var policyDigest string
	var reasonCodesCSV string
	var violationsCSV string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&journal, "journal", "", "path to session journal JSONL")
	flagSet.StringVar(&toolName, "tool", "", "tool name")
	flagSet.StringVar(&verdict, "verdict", "", "decision verdict: allow|block|dry_run|require_approval")
	flagSet.StringVar(&intentID, "intent-id", "", "optional intent id")
	flagSet.StringVar(&traceID, "trace-id", "", "optional trace id")
	flagSet.StringVar(&tracePath, "trace-path", "", "optional trace artifact path")
	flagSet.StringVar(&intentDigest, "intent-digest", "", "optional intent digest (sha256)")
	flagSet.StringVar(&policyDigest, "policy-digest", "", "optional policy digest (sha256)")
	flagSet.StringVar(&reasonCodesCSV, "reason-codes", "", "comma-separated reason codes")
	flagSet.StringVar(&violationsCSV, "violations", "", "comma-separated violations")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "append",
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunSessionAppendUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "append",
			Error:     "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(journal) == "" || strings.TrimSpace(toolName) == "" || strings.TrimSpace(verdict) == "" {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "append",
			Error:     "--journal, --tool, and --verdict are required",
		}, exitInvalidInput)
	}
	if !isSessionVerdictSupported(verdict) {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "append",
			Error:     "unsupported --verdict value (expected allow|block|dry_run|require_approval)",
		}, exitInvalidInput)
	}

	event, err := runpack.AppendSessionEvent(journal, runpack.SessionAppendOptions{
		ProducerVersion: version,
		IntentID:        intentID,
		ToolName:        toolName,
		IntentDigest:    intentDigest,
		PolicyDigest:    policyDigest,
		TraceID:         traceID,
		TracePath:       tracePath,
		Verdict:         strings.ToLower(strings.TrimSpace(verdict)),
		ReasonCodes:     parseCSV(reasonCodesCSV),
		Violations:      parseCSV(violationsCSV),
	})
	if err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "append",
			Journal:   strings.TrimSpace(journal),
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}

	return writeRunSessionOutput(jsonOutput, runSessionOutput{
		OK:        true,
		Operation: "append",
		Journal:   strings.TrimSpace(journal),
		Event:     &event,
	}, exitOK)
}

func runSessionStatus(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"journal": true,
	})
	flagSet := flag.NewFlagSet("run-session-status", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var journal string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&journal, "journal", "", "path to session journal JSONL")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "status",
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunSessionStatusUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "status",
			Error:     "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(journal) == "" {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "status",
			Error:     "--journal is required",
		}, exitInvalidInput)
	}

	status, err := runpack.GetSessionStatus(journal)
	if err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "status",
			Journal:   strings.TrimSpace(journal),
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	return writeRunSessionOutput(jsonOutput, runSessionOutput{
		OK:        true,
		Operation: "status",
		Journal:   strings.TrimSpace(journal),
		Status:    &status,
	}, exitOK)
}

func runSessionCheckpoint(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"journal":   true,
		"out":       true,
		"chain-out": true,
	})
	flagSet := flag.NewFlagSet("run-session-checkpoint", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var journal string
	var outPath string
	var chainOut string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&journal, "journal", "", "path to session journal JSONL")
	flagSet.StringVar(&outPath, "out", "", "path to emitted checkpoint runpack")
	flagSet.StringVar(&chainOut, "chain-out", "", "optional path to session chain JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "checkpoint",
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunSessionCheckpointUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "checkpoint",
			Error:     "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(journal) == "" || strings.TrimSpace(outPath) == "" {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "checkpoint",
			Error:     "--journal and --out are required",
		}, exitInvalidInput)
	}

	result, chainPath, err := runpack.SessionCheckpointAndWriteChain(journal, outPath, runpack.SessionCheckpointOptions{
		ProducerVersion: version,
	})
	if err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "checkpoint",
			Journal:   strings.TrimSpace(journal),
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if strings.TrimSpace(chainOut) != "" {
		if err := runpack.WriteSessionChain(chainOut, result.Chain); err != nil {
			return writeRunSessionOutput(jsonOutput, runSessionOutput{
				OK:        false,
				Operation: "checkpoint",
				Journal:   strings.TrimSpace(journal),
				Error:     err.Error(),
			}, exitCodeForError(err, exitInvalidInput))
		}
		chainPath = strings.TrimSpace(chainOut)
	}

	return writeRunSessionOutput(jsonOutput, runSessionOutput{
		OK:         true,
		Operation:  "checkpoint",
		Journal:    strings.TrimSpace(journal),
		ChainPath:  chainPath,
		Checkpoint: &result.Checkpoint,
	}, exitOK)
}

func runSessionCompact(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"journal": true,
		"out":     true,
	})
	flagSet := flag.NewFlagSet("run-session-compact", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var journal string
	var outPath string
	var dryRun bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&journal, "journal", "", "path to session journal JSONL")
	flagSet.StringVar(&outPath, "out", "", "optional output path for compacted journal (default in-place)")
	flagSet.BoolVar(&dryRun, "dry-run", false, "compute compaction outcome without writing changes")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "compact",
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRunSessionCompactUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "compact",
			Error:     "unexpected positional arguments",
		}, exitInvalidInput)
	}
	if strings.TrimSpace(journal) == "" {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "compact",
			Error:     "--journal is required",
		}, exitInvalidInput)
	}

	result, err := runpack.CompactSessionJournal(journal, runpack.SessionCompactionOptions{
		ProducerVersion: version,
		OutputPath:      outPath,
		DryRun:          dryRun,
	})
	if err != nil {
		return writeRunSessionOutput(jsonOutput, runSessionOutput{
			OK:        false,
			Operation: "compact",
			Journal:   strings.TrimSpace(journal),
			Error:     err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}
	return writeRunSessionOutput(jsonOutput, runSessionOutput{
		OK:         true,
		Operation:  "compact",
		Journal:    strings.TrimSpace(journal),
		Compaction: &result,
	}, exitOK)
}

func writeRunSessionOutput(jsonOutput bool, output runSessionOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if !output.OK {
		fmt.Printf("run session %s error: %s\n",
			sanitizeSessionLogField(fallbackValue(output.Operation, "command")),
			sanitizeSessionLogField(output.Error),
		)
		return exitCode
	}
	switch output.Operation {
	case "start", "status":
		if output.Status != nil {
			fmt.Printf("session %s: session_id=%s run_id=%s events=%d checkpoints=%d last_sequence=%d\n",
				sanitizeSessionLogField(output.Operation),
				sanitizeSessionLogField(output.Status.SessionID),
				sanitizeSessionLogField(output.Status.RunID),
				output.Status.EventCount,
				output.Status.CheckpointCount,
				output.Status.LastSequence,
			)
		}
		if output.Journal != "" {
			fmt.Printf("journal: %s\n", sanitizeSessionLogField(output.Journal))
		}
	case "append":
		if output.Event != nil {
			toolLabel := "redacted"
			if strings.TrimSpace(output.Event.ToolName) == "" {
				toolLabel = "unset"
			}
			fmt.Printf("session append: sequence=%d tool=%s verdict=%s\n",
				output.Event.Sequence,
				toolLabel,
				sessionVerdictLabel(output.Event.Verdict),
			)
		}
		if output.Journal != "" {
			fmt.Printf("journal: %s\n", sanitizeSessionLogField(output.Journal))
		}
	case "checkpoint":
		if output.Checkpoint != nil {
			runpackLabel := "redacted"
			if strings.TrimSpace(output.Checkpoint.RunpackPath) == "" {
				runpackLabel = "unset"
			}
			fmt.Printf("session checkpoint: index=%d runpack=%s range=%d..%d\n",
				output.Checkpoint.CheckpointIndex,
				runpackLabel,
				output.Checkpoint.SequenceStart,
				output.Checkpoint.SequenceEnd,
			)
		}
		if output.ChainPath != "" {
			fmt.Printf("session chain: %s\n", sanitizeSessionLogField(output.ChainPath))
		}
	case "compact":
		if output.Compaction != nil {
			fmt.Printf("session compact: compacted=%t dry_run=%t events=%d->%d checkpoints=%d bytes=%d->%d\n",
				output.Compaction.Compacted,
				output.Compaction.DryRun,
				output.Compaction.EventsBefore,
				output.Compaction.EventsAfter,
				output.Compaction.Checkpoints,
				output.Compaction.BytesBefore,
				output.Compaction.BytesAfter,
			)
			fmt.Printf("journal: %s\n", sanitizeSessionLogField(output.Compaction.JournalPath))
			fmt.Printf("output: %s\n", sanitizeSessionLogField(output.Compaction.OutputPath))
		}
	default:
		fmt.Printf("run session %s: ok\n", sanitizeSessionLogField(fallbackValue(output.Operation, "command")))
	}
	return exitCode
}

func sanitizeSessionLogField(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, trimmed)
	return strings.Join(strings.Fields(cleaned), " ")
}

func sessionVerdictLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow":
		return "allow"
	case "block":
		return "block"
	case "dry_run":
		return "dry_run"
	case "require_approval":
		return "require_approval"
	default:
		return "unknown"
	}
}

func isSessionVerdictSupported(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "block", "dry_run", "require_approval":
		return true
	default:
		return false
	}
}

func printRunSessionUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session start --journal <path> --session-id <id> --run-id <run_id> [--json] [--explain]")
	fmt.Println("  gait run session append --journal <path> --tool <name> --verdict <allow|block|dry_run|require_approval> [--intent-id <id>] [--trace-id <id>] [--trace-path <path>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--reason-codes <csv>] [--violations <csv>] [--json] [--explain]")
	fmt.Println("  gait run session status --journal <path> [--json] [--explain]")
	fmt.Println("  gait run session checkpoint --journal <path> --out <runpack.zip> [--chain-out <session_chain.json>] [--json] [--explain]")
	fmt.Println("  gait run session compact --journal <path> [--out <journal.jsonl>] [--dry-run] [--json] [--explain]")
}

func printRunSessionStartUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session start --journal <path> --session-id <id> --run-id <run_id> [--json] [--explain]")
}

func printRunSessionAppendUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session append --journal <path> --tool <name> --verdict <allow|block|dry_run|require_approval> [--intent-id <id>] [--trace-id <id>] [--trace-path <path>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--reason-codes <csv>] [--violations <csv>] [--json] [--explain]")
}

func printRunSessionStatusUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session status --journal <path> [--json] [--explain]")
}

func printRunSessionCheckpointUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session checkpoint --journal <path> --out <runpack.zip> [--chain-out <session_chain.json>] [--json] [--explain]")
}

func printRunSessionCompactUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait run session compact --journal <path> [--out <journal.jsonl>] [--dry-run] [--json] [--explain]")
}
