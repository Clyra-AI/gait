package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/registry"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	mcpTrustSnapshotSchemaID      = "gait.mcp.trust_snapshot"
	mcpTrustSnapshotSchemaVersion = "1.0.0"
)

type trustSnapshotErrorCode string

const (
	trustSnapshotErrorUnavailable trustSnapshotErrorCode = "unavailable"
	trustSnapshotErrorInvalid     trustSnapshotErrorCode = "invalid"
)

type trustSnapshotError struct {
	code trustSnapshotErrorCode
	err  error
}

func (e *trustSnapshotError) Error() string {
	if e == nil || e.err == nil {
		return "trust snapshot error"
	}
	return e.err.Error()
}

func (e *trustSnapshotError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type TrustSnapshot struct {
	SchemaID        string               `json:"schema_id"`
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	ProducerVersion string               `json:"producer_version"`
	Entries         []TrustSnapshotEntry `json:"entries"`
}

type TrustSnapshotEntry struct {
	ServerID             string                 `json:"server_id"`
	ServerName           string                 `json:"server_name,omitempty"`
	Publisher            string                 `json:"publisher,omitempty"`
	Source               string                 `json:"source,omitempty"`
	Endpoint             string                 `json:"endpoint,omitempty"`
	Status               string                 `json:"status,omitempty"`
	UpdatedAt            time.Time              `json:"updated_at"`
	Score                float64                `json:"score,omitempty"`
	EvidencePath         string                 `json:"evidence_path,omitempty"`
	RegistryVerification *registry.VerifyResult `json:"registry_verification,omitempty"`
}

func ApplyTrustPolicy(policy gate.MCPTrustPolicy, call ToolCall, outcome gate.EvalOutcome, now time.Time) gate.EvalOutcome {
	if !policy.Enabled {
		return outcome
	}
	decision := EvaluateServerTrust(policy, call.Server, call.Context.RiskClass, now)
	if decision == nil {
		return outcome
	}
	outcome.MCPTrust = decision
	outcome.Result.ReasonCodes = mergeUniqueSorted(outcome.Result.ReasonCodes, decision.ReasonCodes)
	if decision.Enforced {
		outcome.Result.Verdict = moreRestrictiveVerdict(outcome.Result.Verdict, policy.Action)
		outcome.Result.Violations = mergeUniqueSorted(outcome.Result.Violations, []string{"mcp_trust_policy"})
	}
	return outcome
}

func EvaluateServerTrust(policy gate.MCPTrustPolicy, server *ServerInfo, riskClass string, now time.Time) *schemagate.MCPTrustDecision {
	if !policy.Enabled {
		return nil
	}
	decision := &schemagate.MCPTrustDecision{
		Threshold:      policy.MinScore,
		DecisionSource: strings.TrimSpace(policy.SnapshotPath),
		MaxAgeSeconds:  mcpTrustMaxAgeSeconds(policy.MaxAge),
		Required:       containsFold(policy.RequiredRiskClasses, riskClass),
	}
	if server != nil {
		decision.ServerID = strings.TrimSpace(server.ServerID)
		decision.ServerName = strings.TrimSpace(server.ServerName)
		decision.Publisher = strings.ToLower(strings.TrimSpace(server.Publisher))
		decision.Source = strings.ToLower(strings.TrimSpace(server.Source))
	}

	serverKey := normalizedServerKey(decision.ServerID, decision.ServerName)
	if serverKey == "" {
		decision.Status = "missing"
		decision.ReasonCodes = []string{"mcp_trust_identifier_missing"}
		decision.Enforced = decision.Required
		return decision
	}

	snapshot, err := LoadTrustSnapshot(policy.SnapshotPath)
	if err != nil {
		switch trustSnapshotErrorCodeOf(err) {
		case trustSnapshotErrorInvalid:
			decision.Status = "invalid"
			decision.ReasonCodes = []string{"mcp_trust_snapshot_invalid"}
		default:
			decision.Status = "missing"
			decision.ReasonCodes = []string{"mcp_trust_snapshot_unavailable"}
		}
		decision.Enforced = decision.Required
		return decision
	}

	entry, ok := findTrustSnapshotEntry(snapshot.Entries, serverKey)
	if !ok {
		decision.Status = "unknown"
		decision.ReasonCodes = []string{"mcp_trust_server_unknown"}
		decision.Enforced = decision.Required
		return decision
	}

	decision.ServerID = strings.TrimSpace(entry.ServerID)
	decision.ServerName = strings.TrimSpace(entry.ServerName)
	decision.Publisher = strings.ToLower(strings.TrimSpace(entry.Publisher))
	decision.Source = strings.ToLower(strings.TrimSpace(entry.Source))
	decision.Score = entry.Score
	decision.UpdatedAt = entry.UpdatedAt.UTC()
	if strings.TrimSpace(entry.EvidencePath) != "" {
		decision.DecisionSource = strings.TrimSpace(entry.EvidencePath)
	}

	reasons := make([]string, 0, 4)
	if status := strings.ToLower(strings.TrimSpace(entry.Status)); status != "" && status != "trusted" && status != "allow" && status != "pass" {
		reasons = append(reasons, "mcp_trust_status_untrusted")
	}
	if len(policy.PublisherAllowlist) > 0 {
		decision.PublisherAllowed = containsFold(policy.PublisherAllowlist, entry.Publisher)
		if !decision.PublisherAllowed {
			reasons = append(reasons, "mcp_trust_publisher_untrusted")
		}
	}
	if policy.RequireRegistry {
		registryOK := entry.RegistryVerification != nil &&
			entry.RegistryVerification.SignatureVerified &&
			entry.RegistryVerification.PublisherAllowed &&
			(!entry.RegistryVerification.PinPresent || entry.RegistryVerification.PinVerified)
		decision.RegistryVerified = registryOK
		if !registryOK {
			reasons = append(reasons, "mcp_trust_registry_unverified")
		}
	}
	if decision.MaxAgeSeconds > 0 {
		if entry.UpdatedAt.IsZero() || entry.UpdatedAt.UTC().Add(time.Duration(decision.MaxAgeSeconds)*time.Second).Before(now.UTC()) {
			reasons = append(reasons, "mcp_trust_stale")
		}
	}
	if policy.MinScore > 0 && entry.Score < policy.MinScore {
		reasons = append(reasons, "mcp_trust_below_threshold")
	}

	if len(reasons) == 0 {
		decision.Status = "trusted"
		decision.ReasonCodes = []string{"mcp_trust_verified"}
		return decision
	}

	decision.Status = classifyTrustStatus(reasons)
	decision.ReasonCodes = mergeUniqueSorted(nil, reasons)
	decision.Enforced = decision.Required
	return decision
}

func LoadTrustSnapshot(path string) (TrustSnapshot, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("trust snapshot path is required"))
	}
	// #nosec G304 -- trust snapshot path is explicit local user input from policy.
	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorUnavailable, fmt.Errorf("read trust snapshot: %w", err))
	}
	var snapshot TrustSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("parse trust snapshot: %w", err))
	}
	if strings.TrimSpace(snapshot.SchemaID) == "" {
		snapshot.SchemaID = mcpTrustSnapshotSchemaID
	}
	if snapshot.SchemaID != mcpTrustSnapshotSchemaID {
		return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("unsupported trust snapshot schema_id %q", snapshot.SchemaID))
	}
	if strings.TrimSpace(snapshot.SchemaVersion) == "" {
		snapshot.SchemaVersion = mcpTrustSnapshotSchemaVersion
	}
	if snapshot.SchemaVersion != mcpTrustSnapshotSchemaVersion {
		return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("unsupported trust snapshot schema_version %q", snapshot.SchemaVersion))
	}
	seenKeys := make(map[string]int, len(snapshot.Entries))
	for index := range snapshot.Entries {
		snapshot.Entries[index].ServerID = strings.TrimSpace(snapshot.Entries[index].ServerID)
		snapshot.Entries[index].ServerName = strings.TrimSpace(snapshot.Entries[index].ServerName)
		snapshot.Entries[index].Publisher = strings.ToLower(strings.TrimSpace(snapshot.Entries[index].Publisher))
		snapshot.Entries[index].Source = strings.ToLower(strings.TrimSpace(snapshot.Entries[index].Source))
		snapshot.Entries[index].Status = strings.ToLower(strings.TrimSpace(snapshot.Entries[index].Status))
		snapshot.Entries[index].EvidencePath = strings.TrimSpace(snapshot.Entries[index].EvidencePath)
		key := normalizedServerKey(snapshot.Entries[index].ServerID, snapshot.Entries[index].ServerName)
		if key == "" {
			return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("trust snapshot entry[%d] requires server_id or server_name", index))
		}
		if previousIndex, ok := seenKeys[key]; ok {
			return TrustSnapshot{}, wrapTrustSnapshotError(trustSnapshotErrorInvalid, fmt.Errorf("trust snapshot contains duplicate server identity %q at entries[%d] and entries[%d]", key, previousIndex, index))
		}
		seenKeys[key] = index
	}
	sort.Slice(snapshot.Entries, func(i, j int) bool {
		return normalizedServerKey(snapshot.Entries[i].ServerID, snapshot.Entries[i].ServerName) <
			normalizedServerKey(snapshot.Entries[j].ServerID, snapshot.Entries[j].ServerName)
	})
	return snapshot, nil
}

func findTrustSnapshotEntry(entries []TrustSnapshotEntry, serverKey string) (TrustSnapshotEntry, bool) {
	for _, entry := range entries {
		if normalizedServerKey(entry.ServerID, entry.ServerName) == serverKey {
			return entry, true
		}
	}
	return TrustSnapshotEntry{}, false
}

func wrapTrustSnapshotError(code trustSnapshotErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &trustSnapshotError{code: code, err: err}
}

func trustSnapshotErrorCodeOf(err error) trustSnapshotErrorCode {
	var snapshotErr *trustSnapshotError
	if errors.As(err, &snapshotErr) {
		return snapshotErr.code
	}
	return ""
}

func normalizedServerKey(serverID string, serverName string) string {
	if trimmed := strings.ToLower(strings.TrimSpace(serverID)); trimmed != "" {
		return trimmed
	}
	return strings.ToLower(strings.TrimSpace(serverName))
}

func mcpTrustMaxAgeSeconds(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	duration, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || duration <= 0 {
		return 0
	}
	return int64(duration / time.Second)
}

func classifyTrustStatus(reasons []string) string {
	for _, reason := range reasons {
		switch reason {
		case "mcp_trust_snapshot_unavailable", "mcp_trust_identifier_missing":
			return "missing"
		case "mcp_trust_server_unknown":
			return "unknown"
		case "mcp_trust_publisher_untrusted", "mcp_trust_status_untrusted":
			return "untrusted"
		case "mcp_trust_registry_unverified":
			return "invalid"
		case "mcp_trust_stale":
			return "stale"
		case "mcp_trust_below_threshold":
			return "below_threshold"
		}
	}
	return "untrusted"
}

func moreRestrictiveVerdict(current string, proposed string) string {
	current = strings.ToLower(strings.TrimSpace(current))
	proposed = strings.ToLower(strings.TrimSpace(proposed))
	if current == "block" || current == "dry_run" {
		return current
	}
	if proposed == "block" {
		return "block"
	}
	if current == "require_approval" {
		return current
	}
	if proposed == "require_approval" {
		return "require_approval"
	}
	return current
}

func containsFold(values []string, value string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range values {
		if strings.ToLower(strings.TrimSpace(candidate)) == normalizedValue {
			return true
		}
	}
	return false
}

func mergeUniqueSorted(values []string, extra []string) []string {
	combined := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, group := range [][]string{values, extra} {
		for _, value := range group {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			combined = append(combined, trimmed)
		}
	}
	sort.Strings(combined)
	return combined
}
