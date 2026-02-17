package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
)

type ExportEvent struct {
	CreatedAt       time.Time `json:"created_at"`
	ProducerVersion string    `json:"producer_version"`
	RunID           string    `json:"run_id"`
	SessionID       string    `json:"session_id,omitempty"`
	TraceID         string    `json:"trace_id"`
	TracePath       string    `json:"trace_path,omitempty"`
	ToolName        string    `json:"tool_name"`
	Verdict         string    `json:"verdict"`
	ReasonCodes     []string  `json:"reason_codes,omitempty"`
	PolicyDigest    string    `json:"policy_digest"`
	IntentDigest    string    `json:"intent_digest"`
	DecisionLatency int64     `json:"decision_latency_ms,omitempty"`
	DelegationRef   string    `json:"delegation_ref,omitempty"`
	DelegationDepth int       `json:"delegation_depth,omitempty"`
}

func ExportLogEvent(path string, event ExportEvent) error {
	entry := event
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	entry.CreatedAt = entry.CreatedAt.UTC()
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode log export event: %w", err)
	}
	return appendJSONL(path, encoded)
}

func ExportOTelEvent(path string, event ExportEvent) error {
	createdAt := event.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	payload := map[string]any{
		"time_unix_nano": createdAt.UnixNano(),
		"severity_text":  "INFO",
		"body":           "gait.mcp.proxy.decision",
		"trace_id":       strings.TrimSpace(event.TraceID),
		"attributes": map[string]any{
			"gait.run_id":              strings.TrimSpace(event.RunID),
			"gait.session_id":          strings.TrimSpace(event.SessionID),
			"gait.trace_path":          strings.TrimSpace(event.TracePath),
			"gait.tool_name":           strings.TrimSpace(event.ToolName),
			"gait.verdict":             strings.TrimSpace(event.Verdict),
			"gait.policy_digest":       strings.TrimSpace(event.PolicyDigest),
			"gait.intent_digest":       strings.TrimSpace(event.IntentDigest),
			"gait.decision_latency_ms": event.DecisionLatency,
			"gait.delegation_ref":      strings.TrimSpace(event.DelegationRef),
			"gait.delegation_depth":    event.DelegationDepth,
			"gait.reason_codes":        event.ReasonCodes,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode otel export event: %w", err)
	}
	return appendJSONL(path, encoded)
}

func appendJSONL(path string, line []byte) error {
	if err := fsx.AppendLineLocked(path, line, 0o600); err != nil {
		return fmt.Errorf("append export file: %w", err)
	}
	return nil
}
