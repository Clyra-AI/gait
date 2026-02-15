package runpack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/zipx"
)

func TestReadRunpackSuccess(t *testing.T) {
	path := writeTestRunpack(t, "run_read", buildIntents("intent_1"), buildResults("intent_1"))

	pack, err := ReadRunpack(path)
	if err != nil {
		t.Fatalf("read runpack: %v", err)
	}
	if pack.Run.RunID != "run_read" {
		t.Fatalf("expected run_id")
	}
	if len(pack.Intents) != 1 || len(pack.Results) != 1 {
		t.Fatalf("expected intents and results")
	}
}

func TestReadRunpackMissingFile(t *testing.T) {
	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		RunID:           "run_missing",
		CaptureMode:     "reference",
		Files: []schemarunpack.ManifestFile{
			{Path: "run.json", SHA256: "1111111111111111111111111111111111111111111111111111111111111111"},
		},
		ManifestDigest: "2222222222222222222222222222222222222222222222222222222222222222",
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buf, []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	path := filepath.Join(t.TempDir(), "runpack_missing.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if _, err := ReadRunpack(path); err == nil {
		t.Fatalf("expected error for missing files")
	}
}

func TestReadRunpackManifestDigestMismatch(t *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest.ManifestDigest = "deadbeef"
	tamperedManifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: tamperedManifestBytes, Mode: 0o644},
	}, runpackFiles...)
	path := writeRunpackZip(t, archiveFiles)

	if _, err := ReadRunpack(path); err == nil {
		t.Fatalf("expected manifest digest mismatch to fail read")
	} else if !strings.Contains(err.Error(), "runpack hash mismatch") {
		t.Fatalf("expected runpack hash mismatch error, got %v", err)
	}
}

func TestReadRunpackManifestSchemaValidation(t *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	baseManifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}

	cases := []struct {
		name      string
		mutate    func(*schemarunpack.Manifest)
		errSubstr string
	}{
		{
			name: "invalid schema id",
			mutate: func(manifest *schemarunpack.Manifest) {
				manifest.SchemaID = "bad.schema"
			},
			errSubstr: "schema_id must be gait.runpack.manifest",
		},
		{
			name: "invalid schema version",
			mutate: func(manifest *schemarunpack.Manifest) {
				manifest.SchemaVersion = "9.9.9"
			},
			errSubstr: "schema_version must be 1.0.0",
		},
		{
			name: "missing run id",
			mutate: func(manifest *schemarunpack.Manifest) {
				manifest.RunID = ""
			},
			errSubstr: "manifest missing run_id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var manifest schemarunpack.Manifest
			if err := json.Unmarshal(baseManifestBytes, &manifest); err != nil {
				t.Fatalf("decode manifest: %v", err)
			}
			tc.mutate(&manifest)
			mutatedBytes, err := json.Marshal(manifest)
			if err != nil {
				t.Fatalf("encode manifest: %v", err)
			}
			archiveFiles := append([]zipx.File{
				{Path: "manifest.json", Data: mutatedBytes, Mode: 0o644},
			}, runpackFiles...)
			path := writeRunpackZip(t, archiveFiles)

			if _, err := ReadRunpack(path); err == nil {
				t.Fatalf("expected validation error")
			} else if !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("expected %q, got %v", tc.errSubstr, err)
			}
		})
	}
}

func TestReadRunpackFileHashMismatch(t *testing.T) {
	manifestFiles, runpackFiles := buildCompleteRunpackFixture()
	manifestBytes, err := buildManifestBytes("run_test", manifestFiles, nil)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	for index := range runpackFiles {
		if runpackFiles[index].Path == "run.json" {
			runpackFiles[index].Data = []byte("{\"run\":\"tampered\"}\n")
		}
	}
	archiveFiles := append([]zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
	}, runpackFiles...)
	path := writeRunpackZip(t, archiveFiles)

	if _, err := ReadRunpack(path); err == nil {
		t.Fatalf("expected hash mismatch error")
	} else if !strings.Contains(err.Error(), "runpack hash mismatch") {
		t.Fatalf("expected runpack hash mismatch, got %v", err)
	}
}

func TestReplayStubSuccess(t *testing.T) {
	path := writeTestRunpack(t, "run_replay", buildIntents("intent_1", "intent_2"), buildResults("intent_1", "intent_2"))

	result, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub: %v", err)
	}
	if result.RunID != "run_replay" {
		t.Fatalf("unexpected run_id")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps")
	}
	if len(result.MissingResults) != 0 {
		t.Fatalf("expected no missing results")
	}
}

func TestReplayStubMissingResult(t *testing.T) {
	path := writeTestRunpack(t, "run_missing_result", buildIntents("intent_1"), nil)

	result, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub: %v", err)
	}
	if len(result.MissingResults) != 1 {
		t.Fatalf("expected missing results")
	}
	if result.Steps[0].Status != "missing_result" {
		t.Fatalf("expected missing_result status")
	}
}

func TestReplayStubUsesDeterministicFidelityStubForKnownTools(t *testing.T) {
	intents := []schemarunpack.IntentRecord{{
		IntentID:   "intent_http_1",
		ToolName:   "tool.http.fetch",
		ArgsDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Args:       map[string]any{"url": "https://example.local"},
	}}
	path := writeTestRunpackWithIntents(t, "run_stubbed", intents, nil)

	first, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub first run: %v", err)
	}
	second, err := ReplayStub(path)
	if err != nil {
		t.Fatalf("replay stub second run: %v", err)
	}

	if len(first.MissingResults) != 0 {
		t.Fatalf("expected no missing results for known stub type, got %#v", first.MissingResults)
	}
	if first.Steps[0].Status != "stubbed" || first.Steps[0].StubType != "http" {
		t.Fatalf("expected stubbed http step, got %#v", first.Steps[0])
	}
	if first.Steps[0].ResultDigest == "" || first.Steps[0].ResultDigest != second.Steps[0].ResultDigest {
		t.Fatalf("expected deterministic stub digest, first=%s second=%s", first.Steps[0].ResultDigest, second.Steps[0].ResultDigest)
	}
}

func TestReplayStubDuplicateIntent(t *testing.T) {
	intents := []schemarunpack.IntentRecord{
		buildIntent("intent_dup"),
		buildIntent("intent_dup"),
	}
	path := writeTestRunpackWithIntents(t, "run_dup_intent", intents, buildResults("intent_dup"))
	if _, err := ReplayStub(path); err == nil {
		t.Fatalf("expected duplicate intent error")
	}
}

func TestReplayStubDuplicateResult(t *testing.T) {
	results := []schemarunpack.ResultRecord{
		buildResult("intent_dup"),
		buildResult("intent_dup"),
	}
	path := writeTestRunpackWithResults(t, "run_dup_result", buildIntents("intent_dup"), results)
	if _, err := ReplayStub(path); err == nil {
		t.Fatalf("expected duplicate result error")
	}
}

func TestReplayRealRequiresAllowlist(t *testing.T) {
	path := writeTestRunpack(t, "run_replay_real_allowlist", buildIntents("intent_1"), buildResults("intent_1"))
	if _, err := ReplayReal(path, RealReplayOptions{}); err == nil {
		t.Fatalf("expected replay real to require a non-empty allowlist")
	}
	if _, err := ReplayReal(path, RealReplayOptions{AllowTools: []string{" ", "\t"}}); err == nil {
		t.Fatalf("expected replay real to reject whitespace-only allowlist entries")
	}
}

func TestReplayRealExecutesAllowedToolsAndKeepsRecordedFallback(t *testing.T) {
	workDir := t.TempDir()
	outputPath := filepath.Join(workDir, "out.txt")
	intents := []schemarunpack.IntentRecord{
		{
			IntentID: "intent_echo",
			ToolName: "tool.echo",
			Args:     map[string]any{"message": "hello replay"},
		},
		{
			IntentID: "intent_write",
			ToolName: "fs.write",
			Args: map[string]any{
				"path":    outputPath,
				"content": "payload",
			},
		},
		{
			IntentID: "intent_unsupported",
			ToolName: "tool.unknown",
			Args:     map[string]any{},
		},
		{
			IntentID: "intent_recorded",
			ToolName: "tool.recorded",
			Args:     map[string]any{},
		},
		{
			IntentID: "intent_stubbed",
			ToolName: "queue.publish",
			Args:     map[string]any{},
		},
	}
	results := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_recorded",
			Status:       "ok",
			ResultDigest: strings.Repeat("c", 64),
			Result:       map[string]any{"ok": true},
		},
	}
	path := writeTestRunpackWithIntents(t, "run_replay_real", intents, results)

	replayResult, err := ReplayReal(path, RealReplayOptions{
		AllowTools: []string{" tool.echo ", "FS.WRITE", "tool.unknown"},
	})
	if err != nil {
		t.Fatalf("replay real: %v", err)
	}
	if replayResult.Mode != ReplayModeReal {
		t.Fatalf("expected real replay mode, got %s", replayResult.Mode)
	}
	if len(replayResult.Steps) != len(intents) {
		t.Fatalf("unexpected step count: got=%d want=%d", len(replayResult.Steps), len(intents))
	}

	byID := map[string]ReplayStep{}
	for _, step := range replayResult.Steps {
		byID[step.IntentID] = step
	}

	echoStep := byID["intent_echo"]
	if echoStep.Status != "ok" || echoStep.Execution != "executed" || echoStep.ResultDigest == "" {
		t.Fatalf("unexpected echo replay step: %#v", echoStep)
	}

	writeStep := byID["intent_write"]
	if writeStep.Status != "ok" || writeStep.Execution != "executed" || writeStep.ResultDigest == "" {
		t.Fatalf("unexpected write replay step: %#v", writeStep)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read replay output file: %v", err)
	}
	if string(written) != "payload" {
		t.Fatalf("unexpected replay output file content: %q", string(written))
	}

	unsupportedStep := byID["intent_unsupported"]
	if unsupportedStep.Status != "error" || unsupportedStep.Execution != "executed" || unsupportedStep.ResultDigest == "" {
		t.Fatalf("unexpected unsupported replay step: %#v", unsupportedStep)
	}

	recordedStep := byID["intent_recorded"]
	if recordedStep.Status != "ok" || recordedStep.Execution != "recorded" || recordedStep.ResultDigest != strings.Repeat("c", 64) {
		t.Fatalf("unexpected recorded fallback step: %#v", recordedStep)
	}

	stubbedStep := byID["intent_stubbed"]
	if stubbedStep.Status != "stubbed" || stubbedStep.Execution != "stubbed" || stubbedStep.StubType != "queue" || stubbedStep.ResultDigest == "" {
		t.Fatalf("unexpected stubbed step: %#v", stubbedStep)
	}
}

func TestReplayRealHelperFunctions(t *testing.T) {
	if isAllowedRealTool(nil, "tool.echo") {
		t.Fatalf("expected empty allow set to deny tool")
	}
	if !isAllowedRealTool(map[string]struct{}{"tool.echo": {}}, " TOOL.ECHO ") {
		t.Fatalf("expected normalized allow-set match")
	}
	if isAllowedRealTool(map[string]struct{}{"tool.echo": {}}, "tool.other") {
		t.Fatalf("did not expect non-allowlisted tool to match")
	}

	if got := readStringArg(nil, "message"); got != "" {
		t.Fatalf("expected empty readStringArg for nil args, got %q", got)
	}
	if got := readStringArg(map[string]any{"message": "  hi  "}, "message"); got != "hi" {
		t.Fatalf("unexpected trimmed readStringArg value: %q", got)
	}
	if got := readStringArg(map[string]any{"message": 42}, "message"); got != "" {
		t.Fatalf("expected empty readStringArg for non-string value, got %q", got)
	}
	if got := digestString("hello"); got != digestString("hello") || len(got) != 64 {
		t.Fatalf("expected stable 64-char digest, got %q", got)
	}
	for _, tc := range []struct {
		toolName string
		expected string
	}{
		{toolName: "tool.http.fetch", expected: "http"},
		{toolName: "write_file", expected: "file"},
		{toolName: "db.query", expected: "db"},
		{toolName: "queue.publish", expected: "queue"},
		{toolName: "tool.demo", expected: ""},
	} {
		if got := classifyStubType(tc.toolName); got != tc.expected {
			t.Fatalf("unexpected stub type for %q: got=%q want=%q", tc.toolName, got, tc.expected)
		}
	}
}

func TestExecuteRealToolBranches(t *testing.T) {
	if _, status, err := executeRealTool(schemarunpack.IntentRecord{ToolName: "tool.echo"}); err != nil || status != "ok" {
		t.Fatalf("expected tool.echo to succeed without message, status=%q err=%v", status, err)
	}

	if _, status, err := executeRealTool(schemarunpack.IntentRecord{
		ToolName: "fs.write",
		Args:     map[string]any{"content": "x"},
	}); err == nil || status != "error" {
		t.Fatalf("expected fs.write without path to fail, status=%q err=%v", status, err)
	}

	if _, status, err := executeRealTool(schemarunpack.IntentRecord{
		ToolName: "shell.exec",
		Args:     map[string]any{},
	}); err == nil || status != "error" {
		t.Fatalf("expected shell.exec without command to fail, status=%q err=%v", status, err)
	}

	if runtime.GOOS == "windows" {
		t.Skip("shell.exec branch uses sh and is covered on non-windows systems")
	}

	okDigest, okStatus, okErr := executeRealTool(schemarunpack.IntentRecord{
		ToolName: "shell.exec",
		Args:     map[string]any{"command": "printf ok"},
	})
	if okErr != nil || okStatus != "ok" || okDigest == "" {
		t.Fatalf("expected shell.exec success, digest=%q status=%q err=%v", okDigest, okStatus, okErr)
	}

	errDigest, errStatus, errExec := executeRealTool(schemarunpack.IntentRecord{
		ToolName: "shell.exec",
		Args:     map[string]any{"command": "echo fail >&2; exit 3"},
	})
	if errExec == nil || errStatus != "error" || errDigest == "" {
		t.Fatalf("expected shell.exec failure to return error status and digest, digest=%q status=%q err=%v", errDigest, errStatus, errExec)
	}
}

func writeTestRunpack(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	return writeTestRunpackWithIntents(t, runID, intents, results)
}

func writeTestRunpackWithIntents(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	run := schemarunpack.Run{
		RunID:     runID,
		CreatedAt: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		Env:       schemarunpack.RunEnv{OS: "linux", Arch: "amd64", Runtime: "go"},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)},
		},
	}
	path := filepath.Join(t.TempDir(), "runpack.zip")
	_, err := WriteRunpack(path, RecordOptions{
		Run:     run,
		Intents: intents,
		Results: results,
		Refs: schemarunpack.Refs{
			RunID: runID,
		},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}
	return path
}

func writeTestRunpackWithResults(t *testing.T, runID string, intents []schemarunpack.IntentRecord, results []schemarunpack.ResultRecord) string {
	return writeTestRunpackWithIntents(t, runID, intents, results)
}

func buildIntents(intentIDs ...string) []schemarunpack.IntentRecord {
	intents := make([]schemarunpack.IntentRecord, len(intentIDs))
	for i, id := range intentIDs {
		intents[i] = buildIntent(id)
	}
	return intents
}

func buildResults(intentIDs ...string) []schemarunpack.ResultRecord {
	results := make([]schemarunpack.ResultRecord, len(intentIDs))
	for i, id := range intentIDs {
		results[i] = buildResult(id)
	}
	return results
}

func buildIntent(intentID string) schemarunpack.IntentRecord {
	return schemarunpack.IntentRecord{
		IntentID:   intentID,
		ToolName:   "tool.demo",
		ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
		Args:       map[string]any{"foo": "bar"},
	}
}

func buildResult(intentID string) schemarunpack.ResultRecord {
	return schemarunpack.ResultRecord{
		IntentID:     intentID,
		Status:       "ok",
		ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
		Result:       map[string]any{"ok": true},
	}
}
