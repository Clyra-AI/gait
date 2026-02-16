package pack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/runpack"
	schemapack "github.com/davidahmann/gait/core/schema/v1/pack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/zipx"
)

func TestBuildExportRecordAndWriteOTelJSONL(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixtureWithTelemetrySignals(t, workDir)

	buildResult, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "pack_run_metrics.zip"),
	})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	record, err := BuildExportRecord(buildResult.Path)
	if err != nil {
		t.Fatalf("build export record: %v", err)
	}
	if record.PackType != string(BuildTypeRun) {
		t.Fatalf("expected run pack type, got %q", record.PackType)
	}
	if record.Metrics.ToolCallsTotal != 4 {
		t.Fatalf("expected tool_calls_total=4, got %d", record.Metrics.ToolCallsTotal)
	}
	if record.Metrics.ToolCallsSuccess != 1 {
		t.Fatalf("expected tool_calls_success=1, got %d", record.Metrics.ToolCallsSuccess)
	}
	if record.Metrics.PolicyBlocked != 1 {
		t.Fatalf("expected policy_blocked=1, got %d", record.Metrics.PolicyBlocked)
	}
	if record.Metrics.ApprovalRequired != 1 {
		t.Fatalf("expected approval_required=1, got %d", record.Metrics.ApprovalRequired)
	}
	if record.Metrics.RegressFailures != 1 {
		t.Fatalf("expected regress_failures=1, got %d", record.Metrics.RegressFailures)
	}
	if record.Metrics.ToolCallsSuccessRate <= 0 || record.Metrics.ToolCallsSuccessRate >= 1 {
		t.Fatalf("expected success rate between 0 and 1, got %f", record.Metrics.ToolCallsSuccessRate)
	}

	otelPath := filepath.Join(workDir, "pack_export.otel.jsonl")
	if err := WriteOTelJSONL(otelPath, record); err != nil {
		t.Fatalf("write otel jsonl: %v", err)
	}
	raw, err := os.ReadFile(otelPath)
	if err != nil {
		t.Fatalf("read otel output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 8 {
		t.Fatalf("expected 8 otel records, got %d", len(lines))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("decode first otel line: %v", err)
	}
	if first["record_type"] != "span" {
		t.Fatalf("expected first otel record to be span, got %#v", first["record_type"])
	}
	attrs, ok := first["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected attributes map in first otel record: %#v", first)
	}
	if attrs["gait.pack_id"] != record.PackID {
		t.Fatalf("expected gait.pack_id=%q got %#v", record.PackID, attrs["gait.pack_id"])
	}
}

func TestBuildPostgresIndexSQL(t *testing.T) {
	record := ExportRecord{
		PackID:          "abc123",
		PackType:        "run",
		SourceRef:       "run_'demo",
		ManifestDigest:  "digest_1",
		FilesChecked:    4,
		HashMismatches:  0,
		MissingFiles:    0,
		UndeclaredFiles: 0,
		SignatureStatus: "verified",
		SignaturesTotal: 1,
		SignaturesValid: 1,
		CreatedAt:       time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC),
		Metrics: ExportMetrics{
			ToolCallsTotal:       2,
			ToolCallsSuccess:     1,
			ToolCallsSuccessRate: 0.5,
			PolicyBlocked:        1,
			ApprovalRequired:     0,
			RegressFailures:      0,
		},
	}

	sqlPayload, err := BuildPostgresIndexSQL(record, PostgresSQLOptions{
		Table:      "public.gait_pack_index",
		IncludeDDL: true,
	})
	if err != nil {
		t.Fatalf("build postgres sql: %v", err)
	}
	sqlText := string(sqlPayload)
	if !strings.Contains(sqlText, `CREATE TABLE IF NOT EXISTS "public"."gait_pack_index"`) {
		t.Fatalf("expected create table statement, got:\n%s", sqlText)
	}
	if !strings.Contains(sqlText, `CREATE INDEX IF NOT EXISTS "idx_gait_pack_index_pack_type_created_at"`) {
		t.Fatalf("expected pack_type index statement, got:\n%s", sqlText)
	}
	if !strings.Contains(sqlText, `ON CONFLICT (pack_id) DO UPDATE`) {
		t.Fatalf("expected upsert statement, got:\n%s", sqlText)
	}
	if !strings.Contains(sqlText, `'run_''demo'`) {
		t.Fatalf("expected escaped source_ref literal, got:\n%s", sqlText)
	}
}

func TestBuildPostgresIndexSQLRejectsInvalidIdentifier(t *testing.T) {
	_, err := BuildPostgresIndexSQL(ExportRecord{}, PostgresSQLOptions{
		Table:      "bad-table",
		IncludeDDL: true,
	})
	if err == nil {
		t.Fatalf("expected invalid postgres identifier error")
	}
}

func TestBuildExportRecordRejectsEmptyPath(t *testing.T) {
	_, err := BuildExportRecord("  ")
	if err == nil {
		t.Fatalf("expected empty path error")
	}
}

func TestWriteOTelJSONLRejectsEmptyPath(t *testing.T) {
	err := WriteOTelJSONL("   ", ExportRecord{})
	if err == nil {
		t.Fatalf("expected empty otel path error")
	}
}

func TestBuildPostgresIndexSQLDefaultsToBaseTableWithoutDDL(t *testing.T) {
	sqlPayload, err := BuildPostgresIndexSQL(ExportRecord{
		PackID:          "pack-1",
		PackType:        "run",
		SourceRef:       "run_demo",
		ManifestDigest:  strings.Repeat("a", 64),
		SignatureStatus: "verified",
		CreatedAt:       time.Date(2026, time.February, 16, 1, 2, 3, 0, time.UTC),
	}, PostgresSQLOptions{})
	if err != nil {
		t.Fatalf("build postgres sql with defaults: %v", err)
	}
	sqlText := string(sqlPayload)
	if strings.Contains(sqlText, "CREATE TABLE") {
		t.Fatalf("did not expect ddl when IncludeDDL=false:\n%s", sqlText)
	}
	if !strings.Contains(sqlText, `INSERT INTO "gait_pack_index"`) {
		t.Fatalf("expected default table insert, got:\n%s", sqlText)
	}
}

func TestNormalizePostgresTableIdentifier(t *testing.T) {
	quoted, prefix, err := normalizePostgresTableIdentifier("public.pack_export")
	if err != nil {
		t.Fatalf("normalize schema table: %v", err)
	}
	if quoted != `"public"."pack_export"` {
		t.Fatalf("unexpected quoted table: %s", quoted)
	}
	if prefix != "pack_export" {
		t.Fatalf("unexpected index prefix: %s", prefix)
	}

	_, _, err = normalizePostgresTableIdentifier("public.schema.pack_export")
	if err == nil {
		t.Fatalf("expected invalid nested table identifier")
	}
}

type exportStringer string

func (value exportStringer) String() string {
	return string(value)
}

func TestExportValueHelpers(t *testing.T) {
	for _, candidate := range []struct {
		name  string
		value any
		want  int
		ok    bool
	}{
		{name: "int", value: int(7), want: 7, ok: true},
		{name: "int8", value: int8(8), want: 8, ok: true},
		{name: "int16", value: int16(9), want: 9, ok: true},
		{name: "int32", value: int32(10), want: 10, ok: true},
		{name: "int64", value: int64(11), want: 11, ok: true},
		{name: "uint", value: uint(12), want: 12, ok: true},
		{name: "uint8", value: uint8(13), want: 13, ok: true},
		{name: "uint16", value: uint16(14), want: 14, ok: true},
		{name: "uint32", value: uint32(15), want: 15, ok: true},
		{name: "uint64", value: uint64(16), want: 16, ok: true},
		{name: "float32", value: float32(17.9), want: 17, ok: true},
		{name: "float64", value: float64(18.9), want: 18, ok: true},
		{name: "json_int", value: json.Number("19"), want: 19, ok: true},
		{name: "json_float", value: json.Number("20.5"), want: 20, ok: true},
		{name: "invalid", value: json.Number("nope"), want: 0, ok: false},
		{name: "unsupported", value: "21", want: 0, ok: false},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			got, ok := intFromAny(candidate.value)
			if ok != candidate.ok {
				t.Fatalf("expected ok=%t got %t", candidate.ok, ok)
			}
			if got != candidate.want {
				t.Fatalf("expected value=%d got %d", candidate.want, got)
			}
		})
	}

	if got := toString(exportStringer("stringer_value")); got != "stringer_value" {
		t.Fatalf("unexpected stringer value: %q", got)
	}
	if got := toString(struct{}{}); got != "" {
		t.Fatalf("expected empty string for unsupported type, got %q", got)
	}
	if got := toStringSlice([]string{" one ", "", "two"}); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("unexpected []string normalization: %#v", got)
	}
	if got := toStringSlice([]any{"alpha", exportStringer("beta"), "  "}); len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("unexpected []any normalization: %#v", got)
	}
	if got := toStringSlice(123); got != nil {
		t.Fatalf("expected nil for unsupported slice input, got %#v", got)
	}
}

func TestExportVerdictAndRegressFailureDetection(t *testing.T) {
	for _, candidate := range []struct {
		name   string
		result map[string]any
		want   string
	}{
		{name: "allow", result: map[string]any{"verdict": "allow"}, want: "allow"},
		{name: "block", result: map[string]any{"verdict": "BLOCK"}, want: "block"},
		{name: "approval_alias", result: map[string]any{"verdict": "needs_approval"}, want: "require_approval"},
		{name: "empty", result: map[string]any{}, want: ""},
		{name: "unknown", result: map[string]any{"verdict": "maybe"}, want: ""},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			if got := normalizeExportVerdict(candidate.result); got != candidate.want {
				t.Fatalf("expected %q got %q", candidate.want, got)
			}
		})
	}

	for _, candidate := range []struct {
		name   string
		record schemarunpack.ResultRecord
		want   bool
	}{
		{name: "regress_failures_int", record: schemarunpack.ResultRecord{Result: map[string]any{"regress_failures": 1}}, want: true},
		{name: "failed_float", record: schemarunpack.ResultRecord{Result: map[string]any{"failed": 2.0}}, want: true},
		{name: "status_regress_failed", record: schemarunpack.ResultRecord{Result: map[string]any{"status": "regress_failed"}}, want: true},
		{name: "reason_code_drift", record: schemarunpack.ResultRecord{Result: map[string]any{"reason_codes": []any{"regress_drift_detected"}}}, want: true},
		{name: "reason_failure", record: schemarunpack.ResultRecord{Result: map[string]any{"failure_reason": "regress fail due to mismatch"}}, want: true},
		{name: "non_failure", record: schemarunpack.ResultRecord{Result: map[string]any{"verdict": "allow"}}, want: false},
		{name: "empty", record: schemarunpack.ResultRecord{}, want: false},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			if got := isRegressFailureResult(candidate.record); got != candidate.want {
				t.Fatalf("expected %t got %t", candidate.want, got)
			}
		})
	}
}

func TestDeriveOTelIDsFallbackOrder(t *testing.T) {
	packID := strings.Repeat("a", 64)
	traceID, spanID := deriveOTelIDs(ExportRecord{PackID: packID})
	if traceID != packID[:32] || spanID != packID[32:48] {
		t.Fatalf("expected ids from pack id, got trace=%s span=%s", traceID, spanID)
	}

	manifestDigest := strings.Repeat("b", 64)
	traceID, spanID = deriveOTelIDs(ExportRecord{PackID: "not-a-sha", ManifestDigest: manifestDigest})
	if traceID != manifestDigest[:32] || spanID != manifestDigest[32:48] {
		t.Fatalf("expected ids from manifest digest, got trace=%s span=%s", traceID, spanID)
	}

	record := ExportRecord{PackType: "run", SourceRef: "run_demo"}
	traceID, spanID = deriveOTelIDs(record)
	if len(traceID) != 32 || len(spanID) != 16 {
		t.Fatalf("unexpected trace/span lengths: %d/%d", len(traceID), len(spanID))
	}
	if !isHexString(traceID) || !isHexString(spanID) {
		t.Fatalf("expected hex trace/span ids, got %q / %q", traceID, spanID)
	}
}

func TestReadManifestDigestSources(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixtureWithTelemetrySignals(t, workDir)

	buildResult, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "pack_export_digest.zip"),
	})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}

	packInspect, err := Inspect(buildResult.Path)
	if err != nil {
		t.Fatalf("inspect pack: %v", err)
	}
	packDigest, err := readManifestDigest(buildResult.Path, packInspect)
	if err != nil {
		t.Fatalf("read manifest digest from pack: %v", err)
	}
	if len(packDigest) != 64 || !isHexString(packDigest) {
		t.Fatalf("expected sha256 digest from pack, got %q", packDigest)
	}

	legacyDigest, err := readManifestDigest(runpackPath, InspectResult{})
	if err != nil {
		t.Fatalf("read manifest digest from runpack: %v", err)
	}
	if len(legacyDigest) != 64 || !isHexString(legacyDigest) {
		t.Fatalf("expected sha256 digest from runpack manifest, got %q", legacyDigest)
	}

	manualZipPath := filepath.Join(workDir, "manual_export_source.zip")
	var archive bytes.Buffer
	if err := zipx.WriteDeterministicZip(&archive, []zipx.File{
		{Path: "notes.txt", Data: []byte("hello"), Mode: 0o644},
	}); err != nil {
		t.Fatalf("build manual zip: %v", err)
	}
	if err := os.WriteFile(manualZipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatalf("write manual zip: %v", err)
	}
	inspectFallback := InspectResult{
		RunPayload: &schemapack.RunPayload{ManifestDigest: "from_run_payload"},
	}
	fallbackDigest, err := readManifestDigest(manualZipPath, inspectFallback)
	if err != nil {
		t.Fatalf("read fallback digest from inspect payload: %v", err)
	}
	if fallbackDigest != "from_run_payload" {
		t.Fatalf("expected fallback digest from inspect payload, got %q", fallbackDigest)
	}
}

func TestReadManifestDigestFromInspectManifest(t *testing.T) {
	workDir := t.TempDir()
	manualZipPath := filepath.Join(workDir, "manual_manifest_source.zip")
	var archive bytes.Buffer
	if err := zipx.WriteDeterministicZip(&archive, []zipx.File{
		{Path: "payload.json", Data: []byte(`{"ok":true}`), Mode: 0o644},
	}); err != nil {
		t.Fatalf("build manual zip: %v", err)
	}
	if err := os.WriteFile(manualZipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatalf("write manual zip: %v", err)
	}
	inspectFallback := InspectResult{
		Manifest: &schemapack.Manifest{
			SchemaID:      "gait.pack.manifest",
			SchemaVersion: "1.0.0",
			CreatedAt:     time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC),
			PackID:        strings.Repeat("c", 64),
			PackType:      "run",
			SourceRef:     "run_id",
		},
	}
	digest, err := readManifestDigest(manualZipPath, inspectFallback)
	if err != nil {
		t.Fatalf("read digest from inspect manifest fallback: %v", err)
	}
	if len(digest) != 64 || !isHexString(digest) {
		t.Fatalf("expected sha256 digest from inspect manifest fallback, got %q", digest)
	}
}

func TestResolveAndNormalizeExportCreatedAt(t *testing.T) {
	manifestTime := time.Date(2026, time.February, 16, 1, 0, 0, 0, time.UTC)
	runTime := manifestTime.Add(1 * time.Hour)
	jobTime := manifestTime.Add(2 * time.Hour)
	callTime := manifestTime.Add(3 * time.Hour)

	resolved := resolveExportCreatedAt(InspectResult{
		Manifest:    &schemapack.Manifest{CreatedAt: manifestTime},
		RunPayload:  &schemapack.RunPayload{CreatedAt: runTime},
		JobPayload:  &schemapack.JobPayload{CreatedAt: jobTime},
		CallPayload: &schemapack.CallPayload{CreatedAt: callTime},
	})
	if !resolved.Equal(manifestTime) {
		t.Fatalf("expected manifest time precedence, got %s", resolved.Format(time.RFC3339))
	}

	if got := resolveExportCreatedAt(InspectResult{RunPayload: &schemapack.RunPayload{CreatedAt: runTime}}); !got.Equal(runTime) {
		t.Fatalf("expected run payload created_at, got %s", got.Format(time.RFC3339))
	}
	if got := resolveExportCreatedAt(InspectResult{JobPayload: &schemapack.JobPayload{CreatedAt: jobTime}}); !got.Equal(jobTime) {
		t.Fatalf("expected job payload created_at, got %s", got.Format(time.RFC3339))
	}
	if got := resolveExportCreatedAt(InspectResult{CallPayload: &schemapack.CallPayload{CreatedAt: callTime}}); !got.Equal(callTime) {
		t.Fatalf("expected call payload created_at, got %s", got.Format(time.RFC3339))
	}

	if got := resolveExportCreatedAt(InspectResult{}); !got.Equal(deterministicTimestamp) {
		t.Fatalf("expected deterministic timestamp fallback, got %s", got.Format(time.RFC3339))
	}
	if got := normalizeExportCreatedAt(time.Time{}); !got.Equal(deterministicTimestamp) {
		t.Fatalf("expected deterministic timestamp for zero time, got %s", got.Format(time.RFC3339))
	}
	if got := normalizeExportCreatedAt(runTime.In(time.FixedZone("Offset", -5*3600))); !got.Equal(runTime) {
		t.Fatalf("expected normalized UTC time to match original instant, got %s", got.Format(time.RFC3339))
	}
}

func TestReadRunpackForExportPaths(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := createRunpackFixtureWithTelemetrySignals(t, workDir)

	buildResult, err := BuildRunPack(BuildRunOptions{
		RunpackPath: runpackPath,
		OutputPath:  filepath.Join(workDir, "pack_for_read_runpack.zip"),
	})
	if err != nil {
		t.Fatalf("build run pack: %v", err)
	}
	data, err := readRunpackForExport(buildResult.Path)
	if err != nil {
		t.Fatalf("read runpack from pack source: %v", err)
	}
	if data == nil || data.Run.RunID == "" {
		t.Fatalf("expected runpack data from source runpack entry")
	}

	data, err = readRunpackForExport(runpackPath)
	if err != nil {
		t.Fatalf("read runpack from legacy runpack: %v", err)
	}
	if data == nil || data.Run.RunID == "" {
		t.Fatalf("expected runpack data from legacy manifest path")
	}

	emptyZipPath := filepath.Join(workDir, "empty_source.zip")
	var archive bytes.Buffer
	if err := zipx.WriteDeterministicZip(&archive, []zipx.File{
		{Path: "noop.txt", Data: []byte(strconv.Itoa(1)), Mode: 0o644},
	}); err != nil {
		t.Fatalf("build empty zip: %v", err)
	}
	if err := os.WriteFile(emptyZipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatalf("write empty zip: %v", err)
	}
	data, err = readRunpackForExport(emptyZipPath)
	if err != nil {
		t.Fatalf("read runpack from empty zip: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil runpack data when zip has no runpack sources")
	}
}

func createRunpackFixtureWithTelemetrySignals(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "runpack_export_telemetry.zip")
	createdAt := time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC)

	_, err := runpack.WriteRunpack(path, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:     "run_export_metrics",
			CreatedAt: createdAt,
			Env:       schemarunpack.RunEnv{OS: "darwin", Arch: "arm64", Runtime: "go"},
			Timeline: []schemarunpack.TimelineEvt{
				{Event: "start", TS: createdAt},
				{Event: "finish", TS: createdAt},
			},
		},
		Intents: []schemarunpack.IntentRecord{
			{IntentID: "intent_allow", ToolName: "tool.search", ArgsDigest: strings.Repeat("a", 64)},
			{IntentID: "intent_block", ToolName: "tool.write", ArgsDigest: strings.Repeat("b", 64)},
			{IntentID: "intent_approval", ToolName: "tool.delete", ArgsDigest: strings.Repeat("c", 64)},
			{IntentID: "intent_regress", ToolName: "tool.replay", ArgsDigest: strings.Repeat("d", 64)},
		},
		Results: []schemarunpack.ResultRecord{
			{
				IntentID:     "intent_allow",
				Status:       "ok",
				ResultDigest: strings.Repeat("1", 64),
				Result:       map[string]any{"verdict": "allow"},
			},
			{
				IntentID:     "intent_block",
				Status:       "error",
				ResultDigest: strings.Repeat("2", 64),
				Result: map[string]any{
					"verdict":      "block",
					"reason_codes": []any{"matched_rule_block_write_host"},
				},
			},
			{
				IntentID:     "intent_approval",
				Status:       "error",
				ResultDigest: strings.Repeat("3", 64),
				Result: map[string]any{
					"verdict":      "require_approval",
					"reason_codes": []any{"high_risk_requires_approval"},
				},
			},
			{
				IntentID:     "intent_regress",
				Status:       "error",
				ResultDigest: strings.Repeat("4", 64),
				Result: map[string]any{
					"regress_status": "fail",
					"reason_codes":   []any{"regress_drift_detected"},
				},
			},
		},
		Refs: schemarunpack.Refs{
			RunID: "run_export_metrics",
		},
	})
	if err != nil {
		t.Fatalf("write runpack fixture: %v", err)
	}
	return path
}
