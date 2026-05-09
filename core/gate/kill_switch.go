package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	killSwitchStateSchemaID      = "gait.gate.kill_switch_state"
	killSwitchStateSchemaVersion = "1.0.0"
	killSwitchJournalSchemaID    = "gait.gate.kill_switch_journal"
)

type KillSwitchJournalRecord struct {
	SchemaID        string    `json:"schema_id"`
	SchemaVersion   string    `json:"schema_version"`
	CreatedAt       time.Time `json:"created_at"`
	ProducerVersion string    `json:"producer_version"`
	Source          string    `json:"source"`
	TraceID         string    `json:"trace_id,omitempty"`
	JobID           string    `json:"job_id,omitempty"`
	ToolName        string    `json:"tool_name,omitempty"`
	AgentID         string    `json:"agent_id,omitempty"`
	Identity        string    `json:"identity,omitempty"`
	ReasonCodes     []string  `json:"reason_codes,omitempty"`
	MatchedEntryIDs []string  `json:"matched_entry_ids,omitempty"`
}

func LoadKillSwitchState(path string) (schemagate.KillSwitchState, error) {
	// #nosec G304 -- explicit local state path input.
	payload, err := os.ReadFile(path)
	if err != nil {
		return schemagate.KillSwitchState{}, fmt.Errorf("read kill switch state: %w", err)
	}
	var state schemagate.KillSwitchState
	if err := json.Unmarshal(payload, &state); err != nil {
		return schemagate.KillSwitchState{}, fmt.Errorf("parse kill switch state: %w", err)
	}
	return normalizeKillSwitchState(state)
}

func WriteKillSwitchState(path string, state schemagate.KillSwitchState) error {
	normalized, err := normalizeKillSwitchState(state)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kill switch state: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := fsx.WriteFileAtomic(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write kill switch state: %w", err)
	}
	return nil
}

func MatchKillSwitch(state schemagate.KillSwitchState, intent schemagate.IntentRequest, now time.Time) *schemagate.KillSwitchDecision {
	now = now.UTC()
	decision := &schemagate.KillSwitchDecision{
		Status:      "inactive",
		EvaluatedAt: now,
	}
	reasonCodes := []string{}
	matchedEntryIDs := []string{}
	for _, entry := range state.Entries {
		if !entry.Enabled {
			continue
		}
		if !entry.ExpiresAt.IsZero() && !entry.ExpiresAt.After(now) {
			continue
		}
		matched, entryReasonCodes := killSwitchEntryMatches(entry, intent)
		if !matched {
			continue
		}
		decision.Status = "active"
		matchedEntryIDs = append(matchedEntryIDs, entry.EntryID)
		reasonCodes = mergeUniqueSorted(reasonCodes, append([]string{"kill_switch_active"}, entryReasonCodes...))
	}
	if decision.Status != "active" {
		return decision
	}
	decision.ReasonCodes = uniqueSorted(reasonCodes)
	decision.MatchedEntryIDs = uniqueSorted(matchedEntryIDs)
	for _, reasonCode := range decision.ReasonCodes {
		if reasonCode != "kill_switch_active" {
			decision.ReasonCode = reasonCode
			break
		}
	}
	if decision.ReasonCode == "" {
		decision.ReasonCode = "kill_switch_active"
	}
	return decision
}

func AppendKillSwitchJournal(path string, record KillSwitchJournalRecord) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("kill switch journal path is required")
	}
	if strings.TrimSpace(record.SchemaID) == "" {
		record.SchemaID = killSwitchJournalSchemaID
	}
	if strings.TrimSpace(record.SchemaVersion) == "" {
		record.SchemaVersion = "1.0.0"
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	record.Source = strings.TrimSpace(record.Source)
	record.TraceID = strings.TrimSpace(record.TraceID)
	record.JobID = strings.TrimSpace(record.JobID)
	record.ToolName = strings.TrimSpace(record.ToolName)
	record.AgentID = strings.TrimSpace(record.AgentID)
	record.Identity = strings.TrimSpace(record.Identity)
	record.ReasonCodes = uniqueSorted(record.ReasonCodes)
	record.MatchedEntryIDs = uniqueSorted(record.MatchedEntryIDs)

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal kill switch journal record: %w", err)
	}
	if err := fsx.AppendLineLocked(path, payload, 0o600); err != nil {
		return fmt.Errorf("append kill switch journal record: %w", err)
	}
	return nil
}

func KillSwitchJournalPath(statePath string) string {
	trimmed := strings.TrimSpace(statePath)
	if trimmed == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(trimmed), "kill_switch_journal.jsonl")
}

func NewKillSwitchState(now time.Time, producerVersion string) schemagate.KillSwitchState {
	now = now.UTC()
	producerVersion = strings.TrimSpace(producerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	return schemagate.KillSwitchState{
		SchemaID:        killSwitchStateSchemaID,
		SchemaVersion:   killSwitchStateSchemaVersion,
		CreatedAt:       now,
		UpdatedAt:       now,
		ProducerVersion: producerVersion,
		Entries:         []schemagate.KillSwitchEntry{},
	}
}

func NewKillSwitchEntry(now time.Time, entry schemagate.KillSwitchEntry) (schemagate.KillSwitchEntry, error) {
	now = now.UTC()
	entry.EntryID = strings.TrimSpace(entry.EntryID)
	entry.Enabled = true
	entry.CreatedAt = now
	normalized, err := normalizeKillSwitchEntry(entry)
	if err != nil {
		return schemagate.KillSwitchEntry{}, err
	}
	if normalized.EntryID == "" {
		normalized.EntryID = computeKillSwitchEntryID(normalized)
	}
	return normalized, nil
}

func computeKillSwitchEntryID(entry schemagate.KillSwitchEntry) string {
	parts := []string{
		entry.AgentID,
		entry.Identity,
		entry.ToolName,
		entry.TargetKind,
		entry.TargetValue,
		entry.Environment,
		strings.Join(entry.PathPrefixes, ","),
		strings.Join(entry.WorkspacePrefixes, ","),
		entry.Reason,
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:12])
}

func normalizeKillSwitchState(state schemagate.KillSwitchState) (schemagate.KillSwitchState, error) {
	if strings.TrimSpace(state.SchemaID) == "" {
		state.SchemaID = killSwitchStateSchemaID
	}
	if strings.TrimSpace(state.SchemaVersion) == "" {
		state.SchemaVersion = killSwitchStateSchemaVersion
	}
	if state.SchemaID != killSwitchStateSchemaID {
		return schemagate.KillSwitchState{}, fmt.Errorf("kill switch state schema_id must be %s", killSwitchStateSchemaID)
	}
	if state.SchemaVersion != killSwitchStateSchemaVersion {
		return schemagate.KillSwitchState{}, fmt.Errorf("kill switch state schema_version must be %s", killSwitchStateSchemaVersion)
	}
	if state.CreatedAt.IsZero() {
		return schemagate.KillSwitchState{}, fmt.Errorf("kill switch state created_at is required")
	}
	if state.UpdatedAt.IsZero() {
		return schemagate.KillSwitchState{}, fmt.Errorf("kill switch state updated_at is required")
	}
	state.CreatedAt = state.CreatedAt.UTC()
	state.UpdatedAt = state.UpdatedAt.UTC()
	state.ProducerVersion = strings.TrimSpace(state.ProducerVersion)
	if state.ProducerVersion == "" {
		state.ProducerVersion = "0.0.0-dev"
	}
	normalizedEntries := make([]schemagate.KillSwitchEntry, 0, len(state.Entries))
	for _, entry := range state.Entries {
		normalized, err := normalizeKillSwitchEntry(entry)
		if err != nil {
			return schemagate.KillSwitchState{}, err
		}
		normalizedEntries = append(normalizedEntries, normalized)
	}
	sort.Slice(normalizedEntries, func(i, j int) bool {
		return normalizedEntries[i].EntryID < normalizedEntries[j].EntryID
	})
	state.Entries = normalizedEntries
	return state, nil
}

func normalizeKillSwitchEntry(entry schemagate.KillSwitchEntry) (schemagate.KillSwitchEntry, error) {
	entry.EntryID = strings.TrimSpace(entry.EntryID)
	entry.AgentID = strings.TrimSpace(entry.AgentID)
	entry.Identity = strings.TrimSpace(entry.Identity)
	entry.ToolName = strings.ToLower(strings.TrimSpace(entry.ToolName))
	entry.TargetKind = strings.ToLower(strings.TrimSpace(entry.TargetKind))
	entry.TargetValue = strings.TrimSpace(entry.TargetValue)
	entry.Environment = strings.ToLower(strings.TrimSpace(entry.Environment))
	entry.Reason = strings.TrimSpace(entry.Reason)
	entry.Actor = strings.TrimSpace(entry.Actor)
	if entry.CreatedAt.IsZero() {
		return schemagate.KillSwitchEntry{}, fmt.Errorf("kill switch entry created_at is required")
	}
	entry.CreatedAt = entry.CreatedAt.UTC()
	entry.ExpiresAt = entry.ExpiresAt.UTC()
	pathPrefixes := normalizeKillSwitchPrefixes(entry.PathPrefixes)
	workspacePrefixes := normalizeKillSwitchPrefixes(entry.WorkspacePrefixes)
	entry.PathPrefixes = pathPrefixes
	entry.WorkspacePrefixes = workspacePrefixes
	if entry.TargetKind != "" {
		if _, ok := allowedTargetKinds[entry.TargetKind]; !ok {
			return schemagate.KillSwitchEntry{}, fmt.Errorf("kill switch entry target_kind is unsupported: %s", entry.TargetKind)
		}
	}
	if entry.EntryID == "" &&
		entry.AgentID == "" &&
		entry.Identity == "" &&
		entry.ToolName == "" &&
		entry.TargetKind == "" &&
		entry.TargetValue == "" &&
		entry.Environment == "" &&
		len(entry.PathPrefixes) == 0 &&
		len(entry.WorkspacePrefixes) == 0 {
		return schemagate.KillSwitchEntry{}, fmt.Errorf("kill switch entry requires at least one scope selector")
	}
	return entry, nil
}

func normalizeKillSwitchPrefixes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		value := filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw)))
		if value == "" || value == "." {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

func killSwitchEntryMatches(entry schemagate.KillSwitchEntry, intent schemagate.IntentRequest) (bool, []string) {
	reasonCodes := []string{}
	if entry.AgentID != "" {
		if strings.TrimSpace(intent.Context.AgentID) != entry.AgentID {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_agent_id_active")
	}
	if entry.Identity != "" {
		if strings.TrimSpace(intent.Context.Identity) != entry.Identity {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_identity_active")
	}
	if entry.ToolName != "" {
		if strings.ToLower(strings.TrimSpace(intent.ToolName)) != entry.ToolName {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_tool_name_active")
	}
	if entry.Environment != "" {
		if strings.ToLower(strings.TrimSpace(intent.Context.Environment)) != entry.Environment {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_environment_active")
	}
	if entry.TargetKind != "" || entry.TargetValue != "" {
		if !killSwitchMatchesTargets(entry, intent) {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_target_active")
	}
	if len(entry.PathPrefixes) > 0 {
		if !killSwitchMatchesPathPrefixes(entry.PathPrefixes, intent) {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_path_active")
	}
	if len(entry.WorkspacePrefixes) > 0 {
		if !killSwitchMatchesWorkspacePrefixes(entry.WorkspacePrefixes, intent) {
			return false, nil
		}
		reasonCodes = append(reasonCodes, "kill_switch_workspace_active")
	}
	return true, uniqueSorted(reasonCodes)
}

func killSwitchMatchesTargets(entry schemagate.KillSwitchEntry, intent schemagate.IntentRequest) bool {
	for _, target := range allIntentTargets(intent) {
		if entry.TargetKind != "" && strings.ToLower(strings.TrimSpace(target.Kind)) != entry.TargetKind {
			continue
		}
		if entry.TargetValue != "" && strings.TrimSpace(target.Value) != entry.TargetValue {
			continue
		}
		return true
	}
	return false
}

func killSwitchMatchesPathPrefixes(prefixes []string, intent schemagate.IntentRequest) bool {
	for _, target := range allIntentTargets(intent) {
		if strings.ToLower(strings.TrimSpace(target.Kind)) != "path" {
			continue
		}
		pathValue := filepath.ToSlash(filepath.Clean(strings.TrimSpace(target.Value)))
		for _, prefix := range prefixes {
			if killSwitchPrefixMatches(pathValue, prefix) {
				return true
			}
		}
	}
	return false
}

func killSwitchMatchesWorkspacePrefixes(prefixes []string, intent schemagate.IntentRequest) bool {
	workspace := filepath.ToSlash(filepath.Clean(strings.TrimSpace(intent.Context.Workspace)))
	for _, prefix := range prefixes {
		if killSwitchPrefixMatches(workspace, prefix) {
			return true
		}
	}
	return false
}

func killSwitchPrefixMatches(value string, prefix string) bool {
	if prefix == "" {
		return false
	}
	if value == prefix {
		return true
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(value, prefix)
	}
	return strings.HasPrefix(value, prefix+"/")
}

func allIntentTargets(intent schemagate.IntentRequest) []schemagate.IntentTarget {
	if intent.Script == nil || len(intent.Script.Steps) == 0 {
		return intent.Targets
	}
	targets := make([]schemagate.IntentTarget, 0, len(intent.Targets))
	targets = append(targets, intent.Targets...)
	for _, step := range intent.Script.Steps {
		targets = append(targets, step.Targets...)
	}
	return targets
}
