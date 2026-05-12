package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

type killSwitchOutput struct {
	OK      bool                         `json:"ok"`
	Action  string                       `json:"action,omitempty"`
	State   *schemagate.KillSwitchState  `json:"state,omitempty"`
	Entry   *schemagate.KillSwitchEntry  `json:"entry,omitempty"`
	Entries []schemagate.KillSwitchEntry `json:"entries,omitempty"`
	Error   string                       `json:"error,omitempty"`
}

func runKillSwitch(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Manage generalized kill-switch state for Gate and MCP enforcement.")
	}
	if len(arguments) == 0 {
		printKillSwitchUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "add":
		return runKillSwitchAdd(arguments[1:])
	case "list":
		return runKillSwitchList(arguments[1:])
	case "disable":
		return runKillSwitchDisable(arguments[1:])
	case "expire":
		return runKillSwitchExpire(arguments[1:])
	default:
		printKillSwitchUsage()
		return exitInvalidInput
	}
}

func runKillSwitchAdd(arguments []string) int {
	flagSet := flag.NewFlagSet("kill-switch-add", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	var statePath string
	var entryID string
	var agentID string
	var identity string
	var toolName string
	var targetKind string
	var targetValue string
	var environment string
	var pathPrefixesCSV string
	var workspacePrefixesCSV string
	var reason string
	var actor string
	var expiresAtText string
	var jsonOutput bool
	flagSet.StringVar(&statePath, "state", "./.gait-out/kill_switch_state.json", "path to kill-switch state JSON")
	flagSet.StringVar(&entryID, "entry-id", "", "entry identifier override")
	flagSet.StringVar(&agentID, "agent-id", "", "agent id selector")
	flagSet.StringVar(&identity, "identity", "", "identity selector")
	flagSet.StringVar(&toolName, "tool-name", "", "tool name selector")
	flagSet.StringVar(&targetKind, "target-kind", "", "target kind selector")
	flagSet.StringVar(&targetValue, "target-value", "", "target value selector")
	flagSet.StringVar(&environment, "environment", "", "environment selector")
	flagSet.StringVar(&pathPrefixesCSV, "path-prefixes", "", "comma-separated path prefixes")
	flagSet.StringVar(&workspacePrefixesCSV, "workspace-prefixes", "", "comma-separated workspace prefixes")
	flagSet.StringVar(&reason, "reason", "", "operator-visible reason")
	flagSet.StringVar(&actor, "actor", "", "actor recording the entry")
	flagSet.StringVar(&expiresAtText, "expires-at", "", "optional RFC3339 expiry time")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	if err := flagSet.Parse(arguments); err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	now := time.Now().UTC()
	state, err := loadOrCreateKillSwitchState(strings.TrimSpace(statePath), now)
	if err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	entry := schemagate.KillSwitchEntry{
		EntryID:           entryID,
		Enabled:           true,
		AgentID:           agentID,
		Identity:          identity,
		ToolName:          toolName,
		TargetKind:        targetKind,
		TargetValue:       targetValue,
		Environment:       environment,
		PathPrefixes:      parseCSV(pathPrefixesCSV),
		WorkspacePrefixes: parseCSV(workspacePrefixesCSV),
		Reason:            reason,
		Actor:             actor,
	}
	if strings.TrimSpace(expiresAtText) != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(expiresAtText))
		if parseErr != nil {
			return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: fmt.Sprintf("invalid --expires-at: %v", parseErr)}, exitInvalidInput)
		}
		entry.ExpiresAt = expiresAt.UTC()
	}
	normalizedEntry, err := gate.NewKillSwitchEntry(now, entry)
	if err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	state.Entries = append(state.Entries, normalizedEntry)
	state.UpdatedAt = now
	if err := gate.WriteKillSwitchState(statePath, state); err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: true, Action: "add", Entry: &normalizedEntry, State: &state}, exitOK)
}

func runKillSwitchList(arguments []string) int {
	flagSet := flag.NewFlagSet("kill-switch-list", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	var statePath string
	var jsonOutput bool
	flagSet.StringVar(&statePath, "state", "./.gait-out/kill_switch_state.json", "path to kill-switch state JSON")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	if err := flagSet.Parse(arguments); err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	state, err := loadOrCreateKillSwitchState(strings.TrimSpace(statePath), time.Now().UTC())
	if err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: true, Action: "list", State: &state, Entries: append([]schemagate.KillSwitchEntry(nil), state.Entries...)}, exitOK)
}

func runKillSwitchDisable(arguments []string) int {
	return mutateKillSwitchEntry(arguments, "disable", func(entry *schemagate.KillSwitchEntry, now time.Time) {
		entry.Enabled = false
	})
}

func runKillSwitchExpire(arguments []string) int {
	flagSet := flag.NewFlagSet("kill-switch-expire", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	var statePath string
	var entryID string
	var expiresAtText string
	var jsonOutput bool
	flagSet.StringVar(&statePath, "state", "./.gait-out/kill_switch_state.json", "path to kill-switch state JSON")
	flagSet.StringVar(&entryID, "entry-id", "", "entry id to expire")
	flagSet.StringVar(&expiresAtText, "expires-at", "", "optional RFC3339 expiry time (default now)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	if err := flagSet.Parse(arguments); err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if strings.TrimSpace(entryID) == "" {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: "--entry-id is required"}, exitInvalidInput)
	}
	now := time.Now().UTC()
	expiresAt := now
	if strings.TrimSpace(expiresAtText) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAtText))
		if err != nil {
			return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: fmt.Sprintf("invalid --expires-at: %v", err)}, exitInvalidInput)
		}
		expiresAt = parsed.UTC()
	}
	state, entry, err := mutateKillSwitchState(strings.TrimSpace(statePath), now, strings.TrimSpace(entryID), func(candidate *schemagate.KillSwitchEntry, _ time.Time) {
		candidate.ExpiresAt = expiresAt
	})
	if err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: true, Action: "expire", State: &state, Entry: entry}, exitOK)
}

func mutateKillSwitchEntry(arguments []string, action string, mutate func(entry *schemagate.KillSwitchEntry, now time.Time)) int {
	flagSet := flag.NewFlagSet("kill-switch-"+action, flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	var statePath string
	var entryID string
	var jsonOutput bool
	flagSet.StringVar(&statePath, "state", "./.gait-out/kill_switch_state.json", "path to kill-switch state JSON")
	flagSet.StringVar(&entryID, "entry-id", "", "entry id to mutate")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	if err := flagSet.Parse(arguments); err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if strings.TrimSpace(entryID) == "" {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: "--entry-id is required"}, exitInvalidInput)
	}
	now := time.Now().UTC()
	state, entry, err := mutateKillSwitchState(strings.TrimSpace(statePath), now, strings.TrimSpace(entryID), mutate)
	if err != nil {
		return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	return writeKillSwitchOutput(jsonOutput, killSwitchOutput{OK: true, Action: action, State: &state, Entry: entry}, exitOK)
}

func mutateKillSwitchState(statePath string, now time.Time, entryID string, mutate func(entry *schemagate.KillSwitchEntry, now time.Time)) (schemagate.KillSwitchState, *schemagate.KillSwitchEntry, error) {
	state, err := gate.LoadKillSwitchState(statePath)
	if err != nil {
		return schemagate.KillSwitchState{}, nil, err
	}
	for index := range state.Entries {
		if state.Entries[index].EntryID != entryID {
			continue
		}
		mutate(&state.Entries[index], now)
		state.UpdatedAt = now
		if err := gate.WriteKillSwitchState(statePath, state); err != nil {
			return schemagate.KillSwitchState{}, nil, err
		}
		entry := state.Entries[index]
		return state, &entry, nil
	}
	return schemagate.KillSwitchState{}, nil, fmt.Errorf("kill switch entry not found: %s", entryID)
}

func loadOrCreateKillSwitchState(statePath string, now time.Time) (schemagate.KillSwitchState, error) {
	state, err := gate.LoadKillSwitchState(statePath)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, fs.ErrNotExist) && !os.IsNotExist(err) && !strings.Contains(strings.ToLower(err.Error()), "no such file or directory") {
		return schemagate.KillSwitchState{}, err
	}
	return gate.NewKillSwitchState(now, currentVersion()), nil
}

func writeKillSwitchOutput(jsonOutput bool, output killSwitchOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.Error != "" {
		fmt.Fprintln(os.Stderr, output.Error)
		return exitCode
	}
	if output.Entry != nil {
		fmt.Printf("%s: %s\n", output.Action, output.Entry.EntryID)
		return exitCode
	}
	if output.State != nil {
		fmt.Printf("%s: %d entries\n", output.Action, len(output.State.Entries))
	}
	return exitCode
}

func printKillSwitchUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait kill-switch add --state <path> [--entry-id <id>] [--agent-id <id>] [--identity <id>] [--tool-name <name>] [--target-kind <kind>] [--target-value <value>] [--environment <env>] [--path-prefixes <csv>] [--workspace-prefixes <csv>] [--reason <text>] [--actor <id>] [--expires-at <rfc3339>] [--json]")
	fmt.Println("  gait kill-switch list --state <path> [--json]")
	fmt.Println("  gait kill-switch disable --state <path> --entry-id <id> [--json]")
	fmt.Println("  gait kill-switch expire --state <path> --entry-id <id> [--expires-at <rfc3339>] [--json]")
}
