package gateway

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	gaitjcs "github.com/Clyra-AI/proof/canon"
	proofrecord "github.com/Clyra-AI/proof/core/record"
	proofschema "github.com/Clyra-AI/proof/core/schema"
	sign "github.com/Clyra-AI/proof/signing"
)

const (
	SourceKong    = "kong"
	SourceDocker  = "docker"
	SourceMintMCP = "mintmcp"
)

var supportedSources = map[string]struct{}{
	SourceKong:    {},
	SourceDocker:  {},
	SourceMintMCP: {},
}

var deterministicGatewayEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

type IngestOptions struct {
	Source            string
	LogPath           string
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
}

type IngestResult struct {
	Source          string `json:"source"`
	LogPath         string `json:"log_path"`
	ProofRecordsOut string `json:"proof_records_out"`
	InputEvents     int    `json:"input_events"`
	OutputRecords   int    `json:"output_records"`
}

type gatewayEvent struct {
	Timestamp    time.Time
	ToolName     string
	Verdict      string
	PolicyDigest string
	ReasonCodes  []string
	RequestID    string
	Identity     string
	Path         string
	StatusCode   int
	RawDigest    string
}

func IngestLogs(opts IngestOptions) (IngestResult, error) {
	source := strings.ToLower(strings.TrimSpace(opts.Source))
	if _, ok := supportedSources[source]; !ok {
		return IngestResult{}, fmt.Errorf("unsupported gateway source: %s", opts.Source)
	}
	logPath, err := normalizePath(opts.LogPath)
	if err != nil {
		return IngestResult{}, fmt.Errorf("log path: %w", err)
	}
	outputPath, err := resolveOutputPath(opts.OutputPath, logPath, source)
	if err != nil {
		return IngestResult{}, fmt.Errorf("proof output path: %w", err)
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	// #nosec G304 -- ingest path is explicit local user input.
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return IngestResult{}, fmt.Errorf("read gateway logs: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lines := make([][]byte, 0)
	previousHash := ""
	inputEvents := 0
	outputRecords := 0
	for lineNo := 1; scanner.Scan(); lineNo++ {
		trimmedLine := strings.TrimSpace(scanner.Text())
		if trimmedLine == "" {
			continue
		}
		inputEvents++
		event, err := parseGatewayEvent(source, trimmedLine, lineNo)
		if err != nil {
			return IngestResult{}, fmt.Errorf("parse gateway log line %d: %w", lineNo, err)
		}
		recordItem, err := proofrecord.New(proofrecord.RecordOpts{
			RecordVersion: "1.0",
			Timestamp:     event.Timestamp,
			Source:        "gait.gateway." + source,
			SourceProduct: "gait",
			AgentID:       nonEmptyOrDefault(event.Identity, source),
			Type:          "policy_enforcement",
			Event: map[string]any{
				"gateway_source":      source,
				"gateway_log_digest":  event.RawDigest,
				"gateway_request_id":  event.RequestID,
				"gateway_status_code": event.StatusCode,
				"path":                event.Path,
				"policy_digest":       event.PolicyDigest,
				"producer_version":    producerVersion,
				"reason_codes":        event.ReasonCodes,
				"tool_name":           event.ToolName,
				"verdict":             event.Verdict,
			},
			Controls: proofrecord.Controls{
				PermissionsEnforced: event.Verdict == "block" || event.Verdict == "require_approval",
			},
			Metadata: map[string]any{
				"artifact_kind":      "gait.gateway.policy_enforcement",
				"gateway_input_line": lineNo,
				"gateway_log_path":   logPath,
			},
		})
		if err != nil {
			return IngestResult{}, fmt.Errorf("build proof record line %d: %w", lineNo, err)
		}
		if previousHash != "" {
			recordItem.Integrity.PreviousRecordHash = previousHash
			recordHash, hashErr := proofrecord.ComputeHash(recordItem)
			if hashErr != nil {
				return IngestResult{}, fmt.Errorf("compute chained record hash line %d: %w", lineNo, hashErr)
			}
			recordItem.Integrity.RecordHash = recordHash
		}
		if len(opts.SigningPrivateKey) > 0 {
			recordSig := sign.SignBytes(opts.SigningPrivateKey, []byte(recordItem.Integrity.RecordHash))
			recordItem.Integrity.SigningKeyID = recordSig.KeyID
			recordItem.Integrity.Signature = "base64:" + recordSig.Sig
		}
		canonicalLine, err := canonicalJSON(recordItem)
		if err != nil {
			return IngestResult{}, fmt.Errorf("encode proof record line %d: %w", lineNo, err)
		}
		if err := proofschema.ValidateRecord(canonicalLine, recordItem.RecordType); err != nil {
			return IngestResult{}, fmt.Errorf("validate proof record line %d: %w", lineNo, err)
		}
		lines = append(lines, canonicalLine)
		previousHash = recordItem.Integrity.RecordHash
		outputRecords++
	}
	if err := scanner.Err(); err != nil {
		return IngestResult{}, fmt.Errorf("read gateway logs: %w", err)
	}

	payload := bytes.Join(lines, []byte{'\n'})
	if len(payload) > 0 {
		payload = append(payload, '\n')
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return IngestResult{}, fmt.Errorf("create proof output directory: %w", err)
	}
	if err := fsx.WriteFileAtomic(outputPath, payload, 0o644); err != nil {
		return IngestResult{}, fmt.Errorf("write proof records: %w", err)
	}
	return IngestResult{
		Source:          source,
		LogPath:         logPath,
		ProofRecordsOut: outputPath,
		InputEvents:     inputEvents,
		OutputRecords:   outputRecords,
	}, nil
}

func parseGatewayEvent(source string, line string, lineNo int) (gatewayEvent, error) {
	payload, err := decodeLinePayload(source, line)
	if err != nil {
		return gatewayEvent{}, err
	}
	timestamp := extractEventTimestamp(payload, lineNo)
	statusCode, _ := extractInt(payload, "status", "status_code", "response.status", "http.status")
	verdict := normalizeVerdict(firstNonEmpty(
		extractString(payload, "verdict", "decision", "action", "outcome"),
		verdictFromStatus(statusCode),
	))
	toolName := firstNonEmpty(extractString(payload, "tool_name", "tool", "route.name", "service.name", "request.path", "request.uri", "path"), "gateway.unknown")
	reasonCodes := extractReasonCodes(payload)
	rawDigest := sha256Hex([]byte(line))
	return gatewayEvent{
		Timestamp:    timestamp,
		ToolName:     toolName,
		Verdict:      verdict,
		PolicyDigest: resolvePolicyDigest(payload, rawDigest),
		ReasonCodes:  reasonCodes,
		RequestID:    extractString(payload, "request_id", "request.id", "correlation_id", "trace_id"),
		Identity:     extractString(payload, "identity", "consumer.username", "user", "actor"),
		Path:         extractString(payload, "path", "request.path", "request.uri"),
		StatusCode:   statusCode,
		RawDigest:    rawDigest,
	}, nil
}

func decodeLinePayload(source string, line string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		if source == SourceDocker {
			return map[string]any{
				"log": line,
			}, nil
		}
		return nil, fmt.Errorf("line is not valid JSON")
	}
	if source == SourceDocker {
		if nestedRaw, ok := payload["log"].(string); ok {
			trimmedNested := strings.TrimSpace(nestedRaw)
			if trimmedNested != "" {
				var nestedPayload map[string]any
				if err := json.Unmarshal([]byte(trimmedNested), &nestedPayload); err == nil {
					for key, value := range payload {
						if key == "log" {
							continue
						}
						if _, exists := nestedPayload[key]; !exists {
							nestedPayload[key] = value
						}
					}
					payload = nestedPayload
				}
			}
		}
	}
	return payload, nil
}

func resolveOutputPath(rawOutputPath string, logPath string, source string) (string, error) {
	trimmed := strings.TrimSpace(rawOutputPath)
	if trimmed == "" {
		baseDir := filepath.Dir(logPath)
		return filepath.Join(baseDir, fmt.Sprintf("gateway_%s_policy_enforcement.jsonl", source)), nil
	}
	return normalizePath(trimmed)
}

func normalizePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	cleanPath := filepath.Clean(trimmed)
	if cleanPath == "." {
		return "", fmt.Errorf("path must not resolve to current directory")
	}
	return cleanPath, nil
}

func extractEventTimestamp(payload map[string]any, lineNo int) time.Time {
	for _, key := range []string{"timestamp", "time", "ts", "started_at", "request.timestamp", "request.time"} {
		if value, ok := extractValue(payload, key); ok {
			if timestamp, parsed := parseTimestamp(value); parsed {
				return timestamp
			}
		}
	}
	return deterministicGatewayEpoch.Add(time.Duration(lineNo) * time.Second)
}

func parseTimestamp(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}, false
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05",
		}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, trimmed); err == nil {
				return parsed.UTC(), true
			}
		}
		if seconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return parseUnixTimestamp(seconds), true
		}
	case float64:
		return parseUnixTimestamp(int64(typed)), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parseUnixTimestamp(parsed), true
		}
	case int:
		return parseUnixTimestamp(int64(typed)), true
	case int64:
		return parseUnixTimestamp(typed), true
	}
	return time.Time{}, false
}

func parseUnixTimestamp(value int64) time.Time {
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func normalizeVerdict(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "allowed", "permit", "permitted", "ok", "success":
		return "allow"
	case "block", "blocked", "deny", "denied", "reject", "rejected", "forbidden", "error":
		return "block"
	case "require_approval", "approval_required", "needs_approval":
		return "require_approval"
	case "dry_run", "observe", "simulate":
		return "dry_run"
	default:
		return "unknown"
	}
}

func verdictFromStatus(statusCode int) string {
	if statusCode >= 400 && statusCode != 0 {
		return "block"
	}
	if statusCode >= 200 && statusCode < 400 {
		return "allow"
	}
	return ""
}

func extractReasonCodes(payload map[string]any) []string {
	codes := normalizeStringSlice(
		extractStrings(payload, "reason_codes", "reasons", "policy.reason_codes"),
	)
	if reasonCode := extractString(payload, "reason_code", "policy.reason_code"); reasonCode != "" {
		codes = append(codes, reasonCode)
	}
	return normalizeStringSlice(codes)
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractStrings(payload map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := extractValue(payload, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return typed
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				switch text := item.(type) {
				case string:
					out = append(out, text)
				case fmt.Stringer:
					out = append(out, text.String())
				}
			}
			return out
		case string:
			return []string{typed}
		}
	}
	return nil
}

func extractString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := extractValue(payload, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func extractInt(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := extractValue(payload, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed), true
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func extractValue(payload map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := asMap[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return gaitjcs.CanonicalizeJSON(raw)
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nonEmptyOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func resolvePolicyDigest(payload map[string]any, fallbackDigest string) string {
	candidate := strings.TrimSpace(extractString(payload, "policy_digest", "policy.hash", "policy.id", "policy_version"))
	if candidate != "" {
		return candidate
	}
	// proof v0.4.5 requires non-empty policy_digest for policy_enforcement records.
	// Use deterministic source-event digest when upstream policy metadata is absent.
	return strings.TrimSpace(fallbackDigest)
}
