package actioncontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OTLPExporter is opt-in. Local signed lifecycle evidence is always produced
// first; transport failure is returned as telemetry status and is never a
// verdict or authorization failure.
type OTLPExporter struct {
	Endpoint string
	Timeout  time.Duration
	MaxBytes int64
	Redact   bool
	AllowRaw bool
	Client   *http.Client
}

type OTLPExportResult struct {
	Attempted bool   `json:"attempted"`
	Sent      bool   `json:"sent"`
	Deduped   bool   `json:"deduped"`
	Error     string `json:"error,omitempty"`
}

// ExportOTLP emits one already-created local evidence object. It is useful for
// trace/runpack producers that do not use LifecycleRecord directly.
func ExportOTLP(exporter OTLPExporter, evidence any) OTLPExportResult {
	if strings.TrimSpace(exporter.Endpoint) == "" {
		return OTLPExportResult{}
	}
	result := OTLPExportResult{Attempted: true}
	maxBytes := exporter.MaxBytes
	if maxBytes <= 0 || maxBytes > 4<<20 {
		maxBytes = 1 << 20
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if int64(len(raw)) > maxBytes {
		result.Error = "OTLP payload exceeds size limit"
		return result
	}
	// The local object is wrapped as a bounded opaque OTLP event. The exporter
	// never changes the source object or its verdict.
	value := any(digestOTLP(string(raw)))
	if exporter.AllowRaw && !exporter.Redact {
		value = string(raw)
	}
	payload, err := json.Marshal(otlpPayload([]map[string]any{otlpSpan("gait.evidence", "", map[string]any{"gait.evidence_reference": value})}))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	return sendOTLP(exporter, payload, result)
}

func ExportLifecycleOTLP(exporter OTLPExporter, records []LifecycleRecord) OTLPExportResult {
	if strings.TrimSpace(exporter.Endpoint) == "" {
		return OTLPExportResult{}
	}
	result := OTLPExportResult{Attempted: true}
	if len(records) == 0 {
		result.Error = "no lifecycle records to export"
		return result
	}
	maxBytes := exporter.MaxBytes
	if maxBytes <= 0 || maxBytes > 4<<20 {
		maxBytes = 1 << 20
	}
	seen := map[string]struct{}{}
	spans := make([]map[string]any, 0, len(records))
	for _, record := range records {
		key := strings.TrimSpace(record.RecordID)
		if key == "" {
			key = string(record.Kind) + "|" + record.OccurredAt
		}
		if _, ok := seen[key]; ok {
			result.Deduped = true
			continue
		}
		seen[key] = struct{}{}
		attrs := map[string]any{"gait.lifecycle.kind": record.Kind, "gait.lifecycle.record_id": record.RecordID, "gait.contract_family_id": record.ContractFamilyID, "gait.revision": record.Revision}
		if exporter.AllowRaw && !exporter.Redact {
			attrs["gait.contract_id"] = record.ContractRef.ID
			attrs["gait.contract_digest"] = record.ContractRef.Digest
			if record.ActivationRef != nil {
				attrs["gait.activation_digest"] = record.ActivationRef.Digest
			}
		} else {
			attrs["gait.contract_id_digest"] = digestOTLP(record.ContractRef.ID)
		}
		spans = append(spans, otlpSpan("gait.lifecycle."+string(record.Kind), record.OccurredAt, attrs))
	}
	payload, err := json.Marshal(otlpPayload(spans))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if int64(len(payload)) > maxBytes {
		result.Error = "OTLP payload exceeds size limit"
		return result
	}
	timeout := exporter.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	_ = timeout
	return sendOTLP(exporter, payload, result)
}

func sendOTLP(exporter OTLPExporter, payload []byte, result OTLPExportResult) OTLPExportResult {
	timeout := exporter.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	client := exporter.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequest(http.MethodPost, exporter.Endpoint, bytes.NewReader(payload))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req) // #nosec G704 -- endpoint is explicit opt-in configuration.
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("OTLP exporter returned HTTP %d", resp.StatusCode)
		return result
	}
	result.Sent = true
	return result
}

func digestOTLP(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func otlpPayload(spans []map[string]any) map[string]any {
	return map[string]any{"resourceSpans": []any{map[string]any{"scopeSpans": []any{map[string]any{"spans": spans}}}}}
}

func otlpSpan(name, occurredAt string, attrs map[string]any) map[string]any {
	span := map[string]any{"name": name, "attributes": otlpAttributes(attrs)}
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(occurredAt)); err == nil {
		span["startTimeUnixNano"] = fmt.Sprintf("%d", parsed.UnixNano())
	}
	return span
}

func otlpAttributes(attrs map[string]any) []map[string]any {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, map[string]any{"key": key, "value": map[string]any{"stringValue": fmt.Sprint(attrs[key])}})
	}
	return out
}
