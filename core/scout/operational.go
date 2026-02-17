package scout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
	"github.com/Clyra-AI/gait/core/fsx"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

const (
	operationalEventSchemaID = "gait.scout.operational_event"
	operationalEventSchemaV1 = "1.0.0"
	maxOperationalLineBytes  = 1024 * 1024
)

func NewOperationalStartEvent(
	command string,
	correlationID string,
	producerVersion string,
	now time.Time,
) schemascout.OperationalEvent {
	return newOperationalEvent(command, correlationID, producerVersion, "start", 0, "none", false, 0, now)
}

func NewOperationalEndEvent(
	command string,
	correlationID string,
	producerVersion string,
	exitCode int,
	errorCategory string,
	retryable bool,
	elapsed time.Duration,
	now time.Time,
) schemascout.OperationalEvent {
	elapsedMS := elapsed.Milliseconds()
	if elapsedMS < 0 {
		elapsedMS = 0
	}
	return newOperationalEvent(
		command,
		correlationID,
		producerVersion,
		"end",
		exitCode,
		errorCategory,
		retryable,
		elapsedMS,
		now,
	)
}

func AppendOperationalEvent(path string, event schemascout.OperationalEvent) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("operational log path is required")
	}
	normalized, err := normalizeOperationalEvent(event)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal operational event: %w", err)
	}
	if err := fsx.AppendLineLocked(trimmedPath, encoded, 0o600); err != nil {
		return fmt.Errorf("append operational log: %w", err)
	}
	return nil
}

func LoadOperationalEvents(path string) ([]schemascout.OperationalEvent, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, fmt.Errorf("operational log path is required")
	}
	// #nosec G304 -- operational log path is explicit local user input.
	file, err := os.Open(trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("open operational log: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	events := make([]schemascout.OperationalEvent, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxOperationalLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var event schemascout.OperationalEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return nil, fmt.Errorf("parse operational log line %d: %w", line, err)
		}
		normalized, err := normalizeOperationalEvent(event)
		if err != nil {
			return nil, fmt.Errorf("validate operational log line %d: %w", line, err)
		}
		events = append(events, normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan operational log: %w", err)
	}
	return events, nil
}

func normalizeOperationalEvent(event schemascout.OperationalEvent) (schemascout.OperationalEvent, error) {
	if strings.TrimSpace(event.SchemaID) != operationalEventSchemaID {
		return schemascout.OperationalEvent{}, fmt.Errorf("invalid schema_id %q", event.SchemaID)
	}
	if strings.TrimSpace(event.SchemaVersion) != operationalEventSchemaV1 {
		return schemascout.OperationalEvent{}, fmt.Errorf("invalid schema_version %q", event.SchemaVersion)
	}
	if event.CreatedAt.IsZero() {
		return schemascout.OperationalEvent{}, fmt.Errorf("created_at is required")
	}
	if strings.TrimSpace(event.ProducerVersion) == "" {
		return schemascout.OperationalEvent{}, fmt.Errorf("producer_version is required")
	}
	if strings.TrimSpace(event.CorrelationID) == "" {
		return schemascout.OperationalEvent{}, fmt.Errorf("correlation_id is required")
	}
	if strings.TrimSpace(event.Command) == "" {
		return schemascout.OperationalEvent{}, fmt.Errorf("command is required")
	}
	phase := strings.ToLower(strings.TrimSpace(event.Phase))
	if phase != "start" && phase != "end" {
		return schemascout.OperationalEvent{}, fmt.Errorf("phase must be start or end")
	}
	if event.ExitCode < 0 || event.ExitCode > 255 {
		return schemascout.OperationalEvent{}, fmt.Errorf("exit_code out of range")
	}
	if event.ElapsedMS < 0 {
		return schemascout.OperationalEvent{}, fmt.Errorf("elapsed_ms out of range")
	}
	category := strings.ToLower(strings.TrimSpace(event.ErrorCategory))
	if category == "" {
		return schemascout.OperationalEvent{}, fmt.Errorf("error_category is required")
	}
	if category != "none" {
		switch coreerrors.Category(category) {
		case coreerrors.CategoryInvalidInput,
			coreerrors.CategoryVerification,
			coreerrors.CategoryPolicyBlocked,
			coreerrors.CategoryApprovalRequired,
			coreerrors.CategoryDependencyMissing,
			coreerrors.CategoryIOFailure,
			coreerrors.CategoryStateContention,
			coreerrors.CategoryNetworkTransient,
			coreerrors.CategoryNetworkPermanent,
			coreerrors.CategoryInternalFailure:
		default:
			return schemascout.OperationalEvent{}, fmt.Errorf("unsupported error_category %q", event.ErrorCategory)
		}
	}
	if strings.TrimSpace(event.Environment.OS) == "" || strings.TrimSpace(event.Environment.Arch) == "" {
		return schemascout.OperationalEvent{}, fmt.Errorf("environment os/arch are required")
	}

	return schemascout.OperationalEvent{
		SchemaID:        operationalEventSchemaID,
		SchemaVersion:   operationalEventSchemaV1,
		CreatedAt:       event.CreatedAt.UTC(),
		ProducerVersion: strings.TrimSpace(event.ProducerVersion),
		CorrelationID:   strings.TrimSpace(event.CorrelationID),
		Command:         strings.TrimSpace(event.Command),
		Phase:           phase,
		ExitCode:        event.ExitCode,
		ErrorCategory:   category,
		Retryable:       event.Retryable,
		ElapsedMS:       event.ElapsedMS,
		Environment: schemascout.AdoptionEnvContext{
			OS:   strings.TrimSpace(event.Environment.OS),
			Arch: strings.TrimSpace(event.Environment.Arch),
		},
	}, nil
}

func newOperationalEvent(
	command string,
	correlationID string,
	producerVersion string,
	phase string,
	exitCode int,
	errorCategory string,
	retryable bool,
	elapsedMS int64,
	now time.Time,
) schemascout.OperationalEvent {
	createdAt := now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand == "" {
		trimmedCommand = "unknown"
	}
	trimmedCorrelationID := strings.TrimSpace(correlationID)
	if trimmedCorrelationID == "" {
		trimmedCorrelationID = "unknown"
	}
	trimmedProducerVersion := strings.TrimSpace(producerVersion)
	if trimmedProducerVersion == "" {
		trimmedProducerVersion = "0.0.0-dev"
	}
	trimmedPhase := strings.ToLower(strings.TrimSpace(phase))
	if trimmedPhase == "" {
		trimmedPhase = "end"
	}
	trimmedCategory := strings.ToLower(strings.TrimSpace(errorCategory))
	if trimmedCategory == "" {
		trimmedCategory = "none"
	}
	return schemascout.OperationalEvent{
		SchemaID:        operationalEventSchemaID,
		SchemaVersion:   operationalEventSchemaV1,
		CreatedAt:       createdAt,
		ProducerVersion: trimmedProducerVersion,
		CorrelationID:   trimmedCorrelationID,
		Command:         trimmedCommand,
		Phase:           trimmedPhase,
		ExitCode:        exitCode,
		ErrorCategory:   trimmedCategory,
		Retryable:       retryable,
		ElapsedMS:       elapsedMS,
		Environment: schemascout.AdoptionEnvContext{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}
}
