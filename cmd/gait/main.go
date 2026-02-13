package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/scout"
)

const version = "0.0.0-dev"

type telemetryStreamHealth struct {
	Attempts int64 `json:"attempts"`
	Success  int64 `json:"success"`
	Failed   int64 `json:"failed"`
}

type telemetryHealthSnapshot struct {
	SchemaID        string                           `json:"schema_id"`
	SchemaVersion   string                           `json:"schema_version"`
	CreatedAt       string                           `json:"created_at"`
	ProducerVersion string                           `json:"producer_version"`
	Streams         map[string]telemetryStreamHealth `json:"streams"`
}

var telemetryState struct {
	sync.Mutex
	streams map[string]telemetryStreamHealth
}

func main() {
	os.Exit(run(os.Args))
}

func run(arguments []string) int {
	startedAt := time.Now()
	correlationID := newCorrelationID(arguments)
	setCurrentCorrelationID(correlationID)
	command := normalizeAdoptionCommand(arguments)
	writeOperationalEventStart(command, correlationID, startedAt.UTC())
	exitCode := runDispatch(arguments)
	finishedAt := time.Now().UTC()
	elapsed := time.Since(startedAt)
	writeAdoptionEvent(command, exitCode, elapsed, finishedAt)
	writeOperationalEventEnd(command, correlationID, exitCode, elapsed, finishedAt)
	setCurrentCorrelationID("")
	return exitCode
}

func runDispatch(arguments []string) int {
	if len(arguments) < 2 {
		fmt.Println("gait", version)
		return exitOK
	}
	if arguments[1] == "--explain" {
		return writeExplain("Gait is an offline-first CLI for deterministic run artifacts, policy-gated tool execution, and reproducible debugging workflows.")
	}

	switch arguments[1] {
	case "approve":
		return runApprove(arguments[2:])
	case "delegate":
		return runDelegate(arguments[2:])
	case "demo":
		return runDemo(arguments[2:])
	case "doctor":
		return runDoctor(arguments[2:])
	case "gate":
		return runGate(arguments[2:])
	case "policy":
		return runPolicy(arguments[2:])
	case "keys":
		return runKeys(arguments[2:])
	case "trace":
		return runTrace(arguments[2:])
	case "regress":
		return runRegress(arguments[2:])
	case "run":
		return runCommand(arguments[2:])
	case "report":
		return runReport(arguments[2:])
	case "scout":
		return runScout(arguments[2:])
	case "guard":
		return runGuard(arguments[2:])
	case "incident":
		return runIncident(arguments[2:])
	case "registry":
		return runRegistry(arguments[2:])
	case "migrate":
		return runMigrate(arguments[2:])
	case "mcp":
		return runMCP(arguments[2:])
	case "verify":
		return runVerify(arguments[2:])
	case "ui":
		return runUI(arguments[2:])
	case "version", "--version", "-v":
		if hasExplainFlag(arguments[2:]) {
			return writeExplain("Print the CLI version.")
		}
		fmt.Println("gait", version)
		return exitOK
	default:
		printUsage()
		return exitInvalidInput
	}
}

func normalizeAdoptionCommand(arguments []string) string {
	if len(arguments) < 2 {
		return "version"
	}
	command := strings.TrimSpace(arguments[1])
	if command == "" {
		return "unknown"
	}
	switch command {
	case "--version", "-v", "version":
		return "version"
	case "--explain":
		return "explain"
	case "gate", "policy", "keys", "trace", "regress", "run", "report", "scout", "guard", "incident", "registry", "mcp", "doctor", "delegate", "ui":
		if len(arguments) > 2 {
			subcommand := strings.TrimSpace(arguments[2])
			if subcommand != "" && !strings.HasPrefix(subcommand, "-") {
				return command + " " + subcommand
			}
		}
	}
	return command
}

func writeAdoptionEvent(command string, exitCode int, elapsed time.Duration, now time.Time) {
	adoptionPath := strings.TrimSpace(os.Getenv("GAIT_ADOPTION_LOG"))
	if adoptionPath == "" {
		return
	}
	workflowID := strings.TrimSpace(os.Getenv("GAIT_ADOPTION_WORKFLOW"))
	event := scout.NewAdoptionEvent(command, exitCode, elapsed, version, now, workflowID)
	recordTelemetryWriteOutcome("adoption", scout.AppendAdoptionEvent(adoptionPath, event))
}

func writeOperationalEventStart(command string, correlationID string, now time.Time) {
	operationalPath := strings.TrimSpace(os.Getenv("GAIT_OPERATIONAL_LOG"))
	if operationalPath == "" {
		return
	}
	event := scout.NewOperationalStartEvent(command, correlationID, version, now)
	recordTelemetryWriteOutcome("operational_start", scout.AppendOperationalEvent(operationalPath, event))
}

func writeOperationalEventEnd(command string, correlationID string, exitCode int, elapsed time.Duration, now time.Time) {
	operationalPath := strings.TrimSpace(os.Getenv("GAIT_OPERATIONAL_LOG"))
	if operationalPath == "" {
		return
	}
	category := "none"
	retryable := false
	if exitCode != exitOK {
		resolvedCategory := defaultErrorCategory(exitCode)
		category = string(resolvedCategory)
		retryable = defaultRetryable(resolvedCategory)
	}
	event := scout.NewOperationalEndEvent(command, correlationID, version, exitCode, category, retryable, elapsed, now)
	recordTelemetryWriteOutcome("operational_end", scout.AppendOperationalEvent(operationalPath, event))
}

func reportTelemetryWriteFailure(stream string, err error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GAIT_TELEMETRY_WARN")), "off") {
		return
	}
	fmt.Fprintf(os.Stderr, "gait warning: telemetry stream=%s write failed: %v\n", stream, err)
}

func recordTelemetryWriteOutcome(stream string, err error) {
	trimmedStream := strings.TrimSpace(stream)
	if trimmedStream == "" {
		trimmedStream = "unknown"
	}
	telemetryState.Lock()
	if telemetryState.streams == nil {
		telemetryState.streams = map[string]telemetryStreamHealth{}
	}
	stats := telemetryState.streams[trimmedStream]
	stats.Attempts++
	if err == nil {
		stats.Success++
	} else {
		stats.Failed++
	}
	telemetryState.streams[trimmedStream] = stats
	snapshot := telemetryHealthSnapshot{
		SchemaID:        "gait.scout.telemetry_health",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		ProducerVersion: version,
		Streams:         make(map[string]telemetryStreamHealth, len(telemetryState.streams)),
	}
	for key, value := range telemetryState.streams {
		snapshot.Streams[key] = value
	}
	telemetryState.Unlock()

	if err != nil {
		reportTelemetryWriteFailure(trimmedStream, err)
	}
	writeTelemetryHealthSnapshot(snapshot)
}

func writeTelemetryHealthSnapshot(snapshot telemetryHealthSnapshot) {
	healthPath := strings.TrimSpace(os.Getenv("GAIT_TELEMETRY_HEALTH_PATH"))
	if healthPath == "" {
		return
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		reportTelemetryWriteFailure("telemetry_health", err)
		return
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(healthPath, encoded, 0o600); err != nil {
		reportTelemetryWriteFailure("telemetry_health", err)
	}
}
