package pack

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

type ExportMetrics struct {
	ToolCallsTotal       int     `json:"tool_calls_total"`
	ToolCallsSuccess     int     `json:"tool_calls_success"`
	ToolCallsSuccessRate float64 `json:"tool_calls_success_rate"`
	PolicyBlocked        int     `json:"policy_blocked"`
	ApprovalRequired     int     `json:"approval_required"`
	RegressFailures      int     `json:"regress_failures"`
}

type ExportRecord struct {
	PackID          string        `json:"pack_id,omitempty"`
	PackType        string        `json:"pack_type,omitempty"`
	SourceRef       string        `json:"source_ref,omitempty"`
	ManifestDigest  string        `json:"manifest_digest,omitempty"`
	FilesChecked    int           `json:"files_checked"`
	HashMismatches  int           `json:"hash_mismatches"`
	MissingFiles    int           `json:"missing_files"`
	UndeclaredFiles int           `json:"undeclared_files"`
	SignatureStatus string        `json:"signature_status,omitempty"`
	SignaturesTotal int           `json:"signatures_total"`
	SignaturesValid int           `json:"signatures_valid"`
	CreatedAt       time.Time     `json:"created_at"`
	Metrics         ExportMetrics `json:"metrics"`
}

type PostgresSQLOptions struct {
	Table      string
	IncludeDDL bool
}

func BuildExportRecord(path string) (ExportRecord, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ExportRecord{}, fmt.Errorf("pack path is required")
	}

	verifyResult, err := Verify(trimmedPath, VerifyOptions{})
	if err != nil {
		return ExportRecord{}, err
	}
	inspectResult, err := Inspect(trimmedPath)
	if err != nil {
		return ExportRecord{}, err
	}

	record := ExportRecord{
		PackID:          strings.TrimSpace(verifyResult.PackID),
		PackType:        strings.TrimSpace(verifyResult.PackType),
		SourceRef:       strings.TrimSpace(verifyResult.SourceRef),
		FilesChecked:    verifyResult.FilesChecked,
		HashMismatches:  len(verifyResult.HashMismatches),
		MissingFiles:    len(verifyResult.MissingFiles),
		UndeclaredFiles: len(verifyResult.UndeclaredFiles),
		SignatureStatus: strings.TrimSpace(verifyResult.SignatureStatus),
		SignaturesTotal: verifyResult.SignaturesTotal,
		SignaturesValid: verifyResult.SignaturesValid,
		CreatedAt:       resolveExportCreatedAt(inspectResult),
		Metrics:         buildExportMetrics(trimmedPath, inspectResult),
	}
	if record.PackType == "" {
		record.PackType = strings.TrimSpace(inspectResult.PackType)
	}
	if record.SourceRef == "" {
		record.SourceRef = strings.TrimSpace(inspectResult.SourceRef)
	}
	if record.PackID == "" {
		record.PackID = strings.TrimSpace(inspectResult.PackID)
	}

	manifestDigest, digestErr := readManifestDigest(trimmedPath, inspectResult)
	if digestErr == nil {
		record.ManifestDigest = manifestDigest
	}
	if record.ManifestDigest == "" {
		record.ManifestDigest = strings.TrimSpace(record.PackID)
	}

	return record, nil
}

func WriteOTelJSONL(path string, record ExportRecord) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("otel output path is required")
	}

	createdAt := normalizeExportCreatedAt(record.CreatedAt)
	traceID, spanID := deriveOTelIDs(record)
	integrityOK := record.HashMismatches == 0 && record.MissingFiles == 0 && record.UndeclaredFiles == 0

	commonAttributes := map[string]any{
		"gait.pack_id":           strings.TrimSpace(record.PackID),
		"gait.pack_type":         strings.TrimSpace(record.PackType),
		"gait.source_ref":        strings.TrimSpace(record.SourceRef),
		"gait.manifest_digest":   strings.TrimSpace(record.ManifestDigest),
		"gait.files_checked":     record.FilesChecked,
		"gait.hash_mismatches":   record.HashMismatches,
		"gait.missing_files":     record.MissingFiles,
		"gait.undeclared_files":  record.UndeclaredFiles,
		"gait.signature_status":  strings.TrimSpace(record.SignatureStatus),
		"gait.signatures_total":  record.SignaturesTotal,
		"gait.signatures_valid":  record.SignaturesValid,
		"gait.integrity_ok":      integrityOK,
		"gait.tool_calls_total":  record.Metrics.ToolCallsTotal,
		"gait.tool_calls_ok":     record.Metrics.ToolCallsSuccess,
		"gait.policy_blocked":    record.Metrics.PolicyBlocked,
		"gait.approval_required": record.Metrics.ApprovalRequired,
		"gait.regress_failures":  record.Metrics.RegressFailures,
	}

	records := []map[string]any{
		{
			"record_type":           "span",
			"trace_id":              traceID,
			"span_id":               spanID,
			"name":                  "gait.pack.verify",
			"start_time_unix_nano":  createdAt.UnixNano(),
			"end_time_unix_nano":    createdAt.UnixNano(),
			"status":                map[string]any{"code": map[bool]string{true: "ok", false: "error"}[integrityOK]},
			"attributes":            commonAttributes,
			"instrumentation_scope": "gait.pack.export",
		},
		{
			"record_type":    "event",
			"time_unix_nano": createdAt.UnixNano(),
			"severity_text":  "INFO",
			"body":           "gait.pack.export.metadata",
			"trace_id":       traceID,
			"span_id":        spanID,
			"attributes":     commonAttributes,
		},
	}

	metrics := []struct {
		name       string
		valueInt   *int
		valueFloat *float64
		unit       string
	}{
		{name: "gait.pack.tool_calls_total", valueInt: intPointer(record.Metrics.ToolCallsTotal), unit: "1"},
		{name: "gait.pack.tool_calls_success_total", valueInt: intPointer(record.Metrics.ToolCallsSuccess), unit: "1"},
		{name: "gait.pack.tool_calls_success_rate", valueFloat: floatPointer(record.Metrics.ToolCallsSuccessRate), unit: "ratio"},
		{name: "gait.pack.policy_blocked_total", valueInt: intPointer(record.Metrics.PolicyBlocked), unit: "1"},
		{name: "gait.pack.approval_required_total", valueInt: intPointer(record.Metrics.ApprovalRequired), unit: "1"},
		{name: "gait.pack.regress_failures_total", valueInt: intPointer(record.Metrics.RegressFailures), unit: "1"},
	}
	for _, metric := range metrics {
		payload := map[string]any{
			"record_type":    "metric",
			"time_unix_nano": createdAt.UnixNano(),
			"name":           metric.name,
			"unit":           metric.unit,
			"attributes":     commonAttributes,
		}
		if metric.valueInt != nil {
			payload["value_int"] = *metric.valueInt
		}
		if metric.valueFloat != nil {
			payload["value_double"] = *metric.valueFloat
		}
		records = append(records, payload)
	}

	for _, recordLine := range records {
		encoded, err := json.Marshal(recordLine)
		if err != nil {
			return fmt.Errorf("encode otel export record: %w", err)
		}
		if err := fsx.AppendLineLocked(trimmedPath, encoded, 0o600); err != nil {
			return fmt.Errorf("write otel export record: %w", err)
		}
	}
	return nil
}

func BuildPostgresIndexSQL(record ExportRecord, options PostgresSQLOptions) ([]byte, error) {
	quotedTable, indexNamePrefix, err := normalizePostgresTableIdentifier(options.Table)
	if err != nil {
		return nil, err
	}

	includeDDL := options.IncludeDDL
	builder := strings.Builder{}
	if includeDDL {
		builder.WriteString("CREATE TABLE IF NOT EXISTS ")
		builder.WriteString(quotedTable)
		builder.WriteString(" (\n")
		builder.WriteString("  pack_id TEXT PRIMARY KEY,\n")
		builder.WriteString("  pack_type TEXT NOT NULL,\n")
		builder.WriteString("  source_ref TEXT NOT NULL,\n")
		builder.WriteString("  manifest_digest TEXT NOT NULL,\n")
		builder.WriteString("  files_checked INTEGER NOT NULL,\n")
		builder.WriteString("  hash_mismatches INTEGER NOT NULL,\n")
		builder.WriteString("  missing_files INTEGER NOT NULL,\n")
		builder.WriteString("  undeclared_files INTEGER NOT NULL,\n")
		builder.WriteString("  signature_status TEXT NOT NULL,\n")
		builder.WriteString("  signatures_total INTEGER NOT NULL,\n")
		builder.WriteString("  signatures_valid INTEGER NOT NULL,\n")
		builder.WriteString("  tool_calls_total INTEGER NOT NULL,\n")
		builder.WriteString("  tool_calls_success INTEGER NOT NULL,\n")
		builder.WriteString("  tool_calls_success_rate DOUBLE PRECISION NOT NULL,\n")
		builder.WriteString("  policy_blocked INTEGER NOT NULL,\n")
		builder.WriteString("  approval_required INTEGER NOT NULL,\n")
		builder.WriteString("  regress_failures INTEGER NOT NULL,\n")
		builder.WriteString("  created_at TIMESTAMPTZ NOT NULL\n")
		builder.WriteString(");\n")

		builder.WriteString("CREATE INDEX IF NOT EXISTS ")
		builder.WriteString(quotePostgresIdentifier("idx_" + indexNamePrefix + "_pack_type_created_at"))
		builder.WriteString(" ON ")
		builder.WriteString(quotedTable)
		builder.WriteString(" (pack_type, created_at DESC);\n")

		builder.WriteString("CREATE INDEX IF NOT EXISTS ")
		builder.WriteString(quotePostgresIdentifier("idx_" + indexNamePrefix + "_source_ref"))
		builder.WriteString(" ON ")
		builder.WriteString(quotedTable)
		builder.WriteString(" (source_ref);\n\n")
	}

	createdAt := normalizeExportCreatedAt(record.CreatedAt).UTC().Format(time.RFC3339Nano)
	builder.WriteString("INSERT INTO ")
	builder.WriteString(quotedTable)
	builder.WriteString(" (\n")
	builder.WriteString("  pack_id,\n")
	builder.WriteString("  pack_type,\n")
	builder.WriteString("  source_ref,\n")
	builder.WriteString("  manifest_digest,\n")
	builder.WriteString("  files_checked,\n")
	builder.WriteString("  hash_mismatches,\n")
	builder.WriteString("  missing_files,\n")
	builder.WriteString("  undeclared_files,\n")
	builder.WriteString("  signature_status,\n")
	builder.WriteString("  signatures_total,\n")
	builder.WriteString("  signatures_valid,\n")
	builder.WriteString("  tool_calls_total,\n")
	builder.WriteString("  tool_calls_success,\n")
	builder.WriteString("  tool_calls_success_rate,\n")
	builder.WriteString("  policy_blocked,\n")
	builder.WriteString("  approval_required,\n")
	builder.WriteString("  regress_failures,\n")
	builder.WriteString("  created_at\n")
	builder.WriteString(") VALUES (\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(strings.TrimSpace(record.PackID)))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(strings.TrimSpace(record.PackType)))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(strings.TrimSpace(record.SourceRef)))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(strings.TrimSpace(record.ManifestDigest)))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.FilesChecked))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.HashMismatches))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.MissingFiles))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.UndeclaredFiles))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(strings.TrimSpace(record.SignatureStatus)))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.SignaturesTotal))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.SignaturesValid))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.Metrics.ToolCallsTotal))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.Metrics.ToolCallsSuccess))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.FormatFloat(record.Metrics.ToolCallsSuccessRate, 'f', 6, 64))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.Metrics.PolicyBlocked))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.Metrics.ApprovalRequired))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(strconv.Itoa(record.Metrics.RegressFailures))
	builder.WriteString(",\n")
	builder.WriteString("  ")
	builder.WriteString(sqlStringLiteral(createdAt))
	builder.WriteString("::timestamptz\n")
	builder.WriteString(")\n")
	builder.WriteString("ON CONFLICT (pack_id) DO UPDATE SET\n")
	builder.WriteString("  pack_type = EXCLUDED.pack_type,\n")
	builder.WriteString("  source_ref = EXCLUDED.source_ref,\n")
	builder.WriteString("  manifest_digest = EXCLUDED.manifest_digest,\n")
	builder.WriteString("  files_checked = EXCLUDED.files_checked,\n")
	builder.WriteString("  hash_mismatches = EXCLUDED.hash_mismatches,\n")
	builder.WriteString("  missing_files = EXCLUDED.missing_files,\n")
	builder.WriteString("  undeclared_files = EXCLUDED.undeclared_files,\n")
	builder.WriteString("  signature_status = EXCLUDED.signature_status,\n")
	builder.WriteString("  signatures_total = EXCLUDED.signatures_total,\n")
	builder.WriteString("  signatures_valid = EXCLUDED.signatures_valid,\n")
	builder.WriteString("  tool_calls_total = EXCLUDED.tool_calls_total,\n")
	builder.WriteString("  tool_calls_success = EXCLUDED.tool_calls_success,\n")
	builder.WriteString("  tool_calls_success_rate = EXCLUDED.tool_calls_success_rate,\n")
	builder.WriteString("  policy_blocked = EXCLUDED.policy_blocked,\n")
	builder.WriteString("  approval_required = EXCLUDED.approval_required,\n")
	builder.WriteString("  regress_failures = EXCLUDED.regress_failures,\n")
	builder.WriteString("  created_at = EXCLUDED.created_at;\n")
	return []byte(builder.String()), nil
}

func buildExportMetrics(path string, inspectResult InspectResult) ExportMetrics {
	metrics := ExportMetrics{}

	data, err := readRunpackForExport(path)
	if err == nil && data != nil {
		total := len(data.Results)
		if total == 0 {
			total = len(data.Intents)
		}
		success := 0
		blocked := 0
		approvalRequired := 0
		regressFailures := 0
		for _, result := range data.Results {
			if strings.EqualFold(strings.TrimSpace(result.Status), "ok") {
				success++
			}
			switch normalizeExportVerdict(result.Result) {
			case "block":
				blocked++
			case "require_approval":
				approvalRequired++
			}
			if isRegressFailureResult(result) {
				regressFailures++
			}
		}
		metrics.ToolCallsTotal = total
		metrics.ToolCallsSuccess = success
		metrics.PolicyBlocked = blocked
		metrics.ApprovalRequired = approvalRequired
		metrics.RegressFailures = regressFailures
	} else if inspectResult.RunLineage != nil {
		metrics.ToolCallsTotal = len(inspectResult.RunLineage.IntentResults)
		success := 0
		for _, link := range inspectResult.RunLineage.IntentResults {
			if strings.EqualFold(strings.TrimSpace(link.Status), "ok") {
				success++
			}
		}
		metrics.ToolCallsSuccess = success
	}

	if metrics.ToolCallsTotal > 0 {
		metrics.ToolCallsSuccessRate = float64(metrics.ToolCallsSuccess) / float64(metrics.ToolCallsTotal)
	}
	return metrics
}

func readRunpackForExport(path string) (*runpack.Runpack, error) {
	bundle, err := openZip(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if sourceRunpack, ok := bundle.Files["source/runpack.zip"]; ok {
		payload, readErr := readZipFile(sourceRunpack)
		if readErr != nil {
			return nil, fmt.Errorf("read source runpack: %w", readErr)
		}
		runData, decodeErr := readRunpackFromBytes(payload)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode source runpack: %w", decodeErr)
		}
		return &runData, nil
	}
	if _, ok := bundle.Files["manifest.json"]; ok {
		runData, readErr := runpack.ReadRunpack(path)
		if readErr != nil {
			return nil, readErr
		}
		return &runData, nil
	}
	return nil, nil
}

func readManifestDigest(path string, inspectResult InspectResult) (string, error) {
	bundle, err := openZip(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = bundle.Close()
	}()

	if manifestFile, ok := bundle.Files[manifestFileName]; ok {
		manifestBytes, readErr := readZipFile(manifestFile)
		if readErr != nil {
			return "", readErr
		}
		digest, digestErr := jcs.DigestJCS(manifestBytes)
		if digestErr != nil {
			return "", digestErr
		}
		return digest, nil
	}
	if legacyManifest, ok := bundle.Files["manifest.json"]; ok {
		manifestBytes, readErr := readZipFile(legacyManifest)
		if readErr != nil {
			return "", readErr
		}
		digest, digestErr := jcs.DigestJCS(manifestBytes)
		if digestErr != nil {
			return "", digestErr
		}
		return digest, nil
	}
	if inspectResult.RunPayload != nil && strings.TrimSpace(inspectResult.RunPayload.ManifestDigest) != "" {
		return strings.TrimSpace(inspectResult.RunPayload.ManifestDigest), nil
	}
	if inspectResult.Manifest != nil {
		manifestBytes, encodeErr := canonicalJSON(*inspectResult.Manifest)
		if encodeErr == nil {
			digest, digestErr := jcs.DigestJCS(manifestBytes)
			if digestErr == nil {
				return digest, nil
			}
		}
	}
	return "", fmt.Errorf("manifest digest unavailable")
}

func resolveExportCreatedAt(inspectResult InspectResult) time.Time {
	switch {
	case inspectResult.Manifest != nil && !inspectResult.Manifest.CreatedAt.IsZero():
		return inspectResult.Manifest.CreatedAt.UTC()
	case inspectResult.RunPayload != nil && !inspectResult.RunPayload.CreatedAt.IsZero():
		return inspectResult.RunPayload.CreatedAt.UTC()
	case inspectResult.JobPayload != nil && !inspectResult.JobPayload.CreatedAt.IsZero():
		return inspectResult.JobPayload.CreatedAt.UTC()
	case inspectResult.CallPayload != nil && !inspectResult.CallPayload.CreatedAt.IsZero():
		return inspectResult.CallPayload.CreatedAt.UTC()
	default:
		return deterministicTimestamp
	}
}

func normalizeExportCreatedAt(value time.Time) time.Time {
	if value.IsZero() {
		return deterministicTimestamp
	}
	return value.UTC()
}

func normalizeExportVerdict(result map[string]any) string {
	if len(result) == 0 {
		return ""
	}
	raw := toString(result["verdict"])
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "allow", "block", "require_approval":
		return strings.ToLower(strings.TrimSpace(raw))
	case "approval_required", "needs_approval", "needsapproval":
		return "require_approval"
	default:
		return ""
	}
}

func isRegressFailureResult(result schemarunpack.ResultRecord) bool {
	payload := result.Result
	if len(payload) == 0 {
		return false
	}

	if value, ok := intFromAny(payload["regress_failures"]); ok && value > 0 {
		return true
	}
	if value, ok := intFromAny(payload["failed"]); ok && value > 0 {
		return true
	}

	statusTokens := []string{
		toString(payload["regress_status"]),
		toString(payload["status"]),
		toString(payload["result_status"]),
	}
	for _, token := range statusTokens {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "fail" || normalized == "failed" || normalized == "regress_failed" {
			return true
		}
	}

	reasons := append(toStringSlice(payload["reason_codes"]), toString(payload["reason"]), toString(payload["failure_reason"]))
	for _, reason := range reasons {
		normalized := strings.ToLower(strings.TrimSpace(reason))
		if normalized == "" {
			continue
		}
		if strings.Contains(normalized, "regress") && (strings.Contains(normalized, "fail") || strings.Contains(normalized, "drift")) {
			return true
		}
	}
	return false
}

func deriveOTelIDs(record ExportRecord) (string, string) {
	seed := strings.ToLower(strings.TrimSpace(record.PackID))
	if !isSHA256Hex(seed) {
		seed = strings.ToLower(strings.TrimSpace(record.ManifestDigest))
	}
	if !isSHA256Hex(seed) {
		seed = sha256Hex([]byte(strings.TrimSpace(record.PackType) + "|" + strings.TrimSpace(record.SourceRef)))
	}
	seed = strings.ToLower(strings.TrimSpace(seed))
	if len(seed) < 64 || !isHexString(seed) {
		seed = sha256Hex([]byte(seed))
	}
	return seed[:32], seed[32:48]
}

func normalizePostgresTableIdentifier(raw string) (string, string, error) {
	table := strings.TrimSpace(raw)
	if table == "" {
		table = "gait_pack_index"
	}
	parts := strings.Split(table, ".")
	if len(parts) == 0 || len(parts) > 2 {
		return "", "", fmt.Errorf("postgres table must be <table> or <schema.table>")
	}
	quotedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if !postgresIdentifierPattern.MatchString(trimmed) {
			return "", "", fmt.Errorf("postgres identifier must match %s", postgresIdentifierPattern.String())
		}
		quotedParts = append(quotedParts, quotePostgresIdentifier(trimmed))
	}
	indexNamePrefix := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	return strings.Join(quotedParts, "."), indexNamePrefix, nil
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func intPointer(value int) *int {
	return &value
}

func floatPointer(value float64) *float64 {
	return &value
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return intFromInt64(typed)
	case uint:
		return intFromUint64(uint64(typed))
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return intFromUint64(typed)
	case float32:
		return intFromFloat64(float64(typed))
	case float64:
		return intFromFloat64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return intFromInt64(parsed)
		}
		floatParsed, floatErr := typed.Float64()
		if floatErr != nil {
			return 0, false
		}
		return intFromFloat64(floatParsed)
	default:
		return 0, false
	}
}

func intFromInt64(value int64) (int, bool) {
	const maxInt = int64(^uint(0) >> 1)
	const minInt = -maxInt - 1
	if value < minInt || value > maxInt {
		return 0, false
	}
	return int(value), true
}

func intFromUint64(value uint64) (int, bool) {
	const maxInt = uint64(^uint(0) >> 1)
	if value > maxInt {
		return 0, false
	}
	return int(value), true
}

func intFromFloat64(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	truncated := math.Trunc(value)
	positiveLimit := math.Ldexp(1, strconv.IntSize-1)
	if truncated < -positiveLimit || truncated >= positiveLimit {
		return 0, false
	}
	return intFromInt64(int64(truncated))
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func toStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(toString(item))
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}

func isHexString(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

var postgresIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
