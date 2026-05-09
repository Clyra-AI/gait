package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
)

func TestRunPackLifecycleCommands(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	jobsRoot := filepath.Join(workDir, "jobs")

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"submit", "--id", "job_pack_cli", "--root", jobsRoot, "--json"}); code != exitOK {
		t.Fatalf("job submit expected %d got %d", exitOK, code)
	}

	runBuildCode, runBuildOut := runPackJSON(t, []string{"build", "--type", "run", "--from", "run_demo", "--json"})
	if runBuildCode != exitOK {
		t.Fatalf("pack build run expected %d got %d output=%#v", exitOK, runBuildCode, runBuildOut)
	}
	if runBuildOut.Path == "" || runBuildOut.PackType != "run" {
		t.Fatalf("unexpected run pack build output: %#v", runBuildOut)
	}

	jobBuildCode, jobBuildOut := runPackJSON(t, []string{
		"build",
		"--type", "job",
		"--from", "job_pack_cli",
		"--job-root", jobsRoot,
		"--json",
	})
	if jobBuildCode != exitOK {
		t.Fatalf("pack build job expected %d got %d output=%#v", exitOK, jobBuildCode, jobBuildOut)
	}
	if jobBuildOut.Path == "" || jobBuildOut.PackType != "job" {
		t.Fatalf("unexpected job pack build output: %#v", jobBuildOut)
	}

	runpackPath, err := resolveRunpackPath("run_demo")
	if err != nil {
		t.Fatalf("resolve runpack path: %v", err)
	}
	callRecordPath := writeVoiceCallRecordForPackCLI(t, workDir, runpackPath, "call_pack_cli")
	callBuildCode, callBuildOut := runPackJSON(t, []string{
		"build",
		"--type", "call",
		"--from", callRecordPath,
		"--json",
	})
	if callBuildCode != exitOK {
		t.Fatalf("pack build call expected %d got %d output=%#v", exitOK, callBuildCode, callBuildOut)
	}
	if callBuildOut.Path == "" || callBuildOut.PackType != "call" {
		t.Fatalf("unexpected call pack build output: %#v", callBuildOut)
	}

	verifyCode, verifyOut := runPackJSON(t, []string{"verify", runBuildOut.Path, "--json"})
	if verifyCode != exitOK {
		t.Fatalf("pack verify expected %d got %d output=%#v", exitOK, verifyCode, verifyOut)
	}
	if verifyOut.Verify == nil || verifyOut.Verify.PackID == "" {
		t.Fatalf("unexpected verify output: %#v", verifyOut)
	}

	inspectCode, inspectOut := runPackJSON(t, []string{"inspect", runBuildOut.Path, "--json"})
	if inspectCode != exitOK {
		t.Fatalf("pack inspect expected %d got %d output=%#v", exitOK, inspectCode, inspectOut)
	}
	if inspectOut.Inspect == nil || inspectOut.Inspect.PackType != "run" {
		t.Fatalf("unexpected inspect output: %#v", inspectOut)
	}

	callInspectCode, callInspectOut := runPackJSON(t, []string{"inspect", callBuildOut.Path, "--json"})
	if callInspectCode != exitOK {
		t.Fatalf("pack inspect call expected %d got %d output=%#v", exitOK, callInspectCode, callInspectOut)
	}
	if callInspectOut.Inspect == nil || callInspectOut.Inspect.PackType != "call" || callInspectOut.Inspect.CallPayload == nil {
		t.Fatalf("unexpected call inspect output: %#v", callInspectOut)
	}

	diffPath := filepath.Join(workDir, "pack_diff.json")
	diffCode, diffOut := runPackJSON(t, []string{"diff", runBuildOut.Path, jobBuildOut.Path, "--output", diffPath, "--json"})
	if diffCode != exitVerifyFailed {
		t.Fatalf("pack diff expected %d got %d output=%#v", exitVerifyFailed, diffCode, diffOut)
	}
	if diffOut.Diff == nil || !diffOut.Diff.Result.Summary.Changed {
		t.Fatalf("expected changed diff output: %#v", diffOut)
	}
	if _, err := os.Stat(diffPath); err != nil {
		t.Fatalf("expected diff output file: %v", err)
	}

	otelExportPath := filepath.Join(workDir, "pack_otel.jsonl")
	postgresSQLPath := filepath.Join(workDir, "pack_index.sql")
	exportCode, exportOut := runPackJSON(t, []string{
		"export",
		runBuildOut.Path,
		"--otel-out", otelExportPath,
		"--postgres-sql-out", postgresSQLPath,
		"--postgres-table", "public.gait_pack_index",
		"--json",
	})
	if exportCode != exitOK {
		t.Fatalf("pack export expected %d got %d output=%#v", exitOK, exportCode, exportOut)
	}
	if exportOut.Export == nil || exportOut.Export.PackID == "" {
		t.Fatalf("unexpected pack export output: %#v", exportOut)
	}
	if _, err := os.Stat(otelExportPath); err != nil {
		t.Fatalf("expected otel export output: %v", err)
	}
	if _, err := os.Stat(postgresSQLPath); err != nil {
		t.Fatalf("expected postgres sql output: %v", err)
	}

	var verifyTextCode int
	verifyText := captureStdout(t, func() {
		verifyTextCode = runPack([]string{"verify", runBuildOut.Path})
	})
	if verifyTextCode != exitOK {
		t.Fatalf("pack verify text expected %d got %d", exitOK, verifyTextCode)
	}
	if !strings.Contains(verifyText, "pack verify ok:") {
		t.Fatalf("expected pack verify text output, got %q", verifyText)
	}

	var diffTextCode int
	diffText := captureStdout(t, func() {
		diffTextCode = runPack([]string{"diff", runBuildOut.Path, runBuildOut.Path})
	})
	if diffTextCode != exitOK {
		t.Fatalf("pack diff text expected %d got %d", exitOK, diffTextCode)
	}
	if !strings.Contains(diffText, "pack diff ok") {
		t.Fatalf("expected pack diff text output, got %q", diffText)
	}
}

func TestRunPackAuthorizationLifecycle(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	specPath := writeAuthorizationBundleFixtureForPackCLI(t, workDir, "allow")
	buildCode, buildOut := runPackJSON(t, []string{
		"build",
		"--type", "authorization",
		"--from", specPath,
		"--json",
	})
	if buildCode != exitOK {
		t.Fatalf("pack build authorization expected %d got %d output=%#v", exitOK, buildCode, buildOut)
	}
	if buildOut.Path == "" || buildOut.PackType != "authorization" {
		t.Fatalf("unexpected authorization pack build output: %#v", buildOut)
	}

	verifyCode, verifyOut := runPackJSON(t, []string{"verify", buildOut.Path, "--json"})
	if verifyCode != exitOK {
		t.Fatalf("pack verify authorization expected %d got %d output=%#v", exitOK, verifyCode, verifyOut)
	}
	if verifyOut.Verify == nil || verifyOut.Verify.PackType != "authorization" {
		t.Fatalf("unexpected authorization verify output: %#v", verifyOut)
	}

	inspectCode, inspectOut := runPackJSON(t, []string{"inspect", buildOut.Path, "--json"})
	if inspectCode != exitOK {
		t.Fatalf("pack inspect authorization expected %d got %d output=%#v", exitOK, inspectCode, inspectOut)
	}
	if inspectOut.Inspect == nil || inspectOut.Inspect.AuthorizationBundle == nil || inspectOut.Inspect.AuthorizationMeta == nil {
		t.Fatalf("unexpected authorization inspect output: %#v", inspectOut)
	}
}

func TestRunPackHelpAndErrorPaths(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runPack([]string{}); code != exitInvalidInput {
		t.Fatalf("pack root usage expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("pack unknown command expected %d got %d", exitInvalidInput, code)
	}

	helpCases := [][]string{
		{"build", "--help"},
		{"verify", "--help"},
		{"inspect", "--help"},
		{"diff", "--help"},
		{"export", "--help"},
	}
	for _, args := range helpCases {
		if code := runPack(args); code != exitOK {
			t.Fatalf("help path %v expected %d got %d", args, exitOK, code)
		}
	}

	if code := runPack([]string{"build", "--type", "run", "--json"}); code != exitInvalidInput {
		t.Fatalf("build missing --from expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"build", "--type", "bad", "--from", "x", "--json"}); code != exitInvalidInput {
		t.Fatalf("build invalid --type expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"verify", "--json"}); code != exitInvalidInput {
		t.Fatalf("verify missing path expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"verify", "--profile", "strict", "missing.zip", "--json"}); code != exitInvalidInput {
		t.Fatalf("verify strict without keys expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"inspect", "--json"}); code != exitInvalidInput {
		t.Fatalf("inspect missing path expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"diff", "--json"}); code != exitInvalidInput {
		t.Fatalf("diff missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"export", "missing.zip", "--json"}); code != exitInvalidInput {
		t.Fatalf("export missing output flags expected %d got %d", exitInvalidInput, code)
	}
	if code := runPack([]string{"export", "--postgres-sql-out", "out.sql", "--postgres-table", "bad-table", "--json"}); code != exitInvalidInput {
		t.Fatalf("export invalid postgres table expected %d got %d", exitInvalidInput, code)
	}
}

func TestResolveJobSourceVariants(t *testing.T) {
	workDir := t.TempDir()
	jobsRoot := filepath.Join(workDir, "jobs")
	jobDir := filepath.Join(jobsRoot, "job_from_dir")
	if err := os.MkdirAll(jobDir, 0o750); err != nil {
		t.Fatalf("mkdir job dir: %v", err)
	}
	statePath := filepath.Join(jobDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write state json: %v", err)
	}

	root, jobID, err := resolveJobSource(jobDir, "")
	if err != nil {
		t.Fatalf("resolve from directory: %v", err)
	}
	if root != jobsRoot || jobID != "job_from_dir" {
		t.Fatalf("unexpected resolved directory source root=%s job_id=%s", root, jobID)
	}

	root, jobID, err = resolveJobSource(statePath, "")
	if err != nil {
		t.Fatalf("resolve from state.json path: %v", err)
	}
	if root != jobsRoot || jobID != "job_from_dir" {
		t.Fatalf("unexpected resolved state source root=%s job_id=%s", root, jobID)
	}

	root, jobID, err = resolveJobSource("job_from_id", "")
	if err != nil {
		t.Fatalf("resolve from id: %v", err)
	}
	if root != "./gait-out/jobs" || jobID != "job_from_id" {
		t.Fatalf("unexpected resolved id source root=%s job_id=%s", root, jobID)
	}

	if _, _, err := resolveJobSource(" ", ""); err == nil {
		t.Fatalf("expected empty source error")
	}
}

func TestRunPackBuildDeterministicByDefault(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo expected %d got %d", exitOK, code)
	}

	firstPath := filepath.Join(workDir, "pack_first.zip")
	secondPath := filepath.Join(workDir, "pack_second.zip")

	if code := runPack([]string{"build", "--type", "run", "--from", "run_demo", "--out", firstPath, "--json"}); code != exitOK {
		t.Fatalf("first pack build expected %d got %d", exitOK, code)
	}
	if code := runPack([]string{"build", "--type", "run", "--from", "run_demo", "--out", secondPath, "--json"}); code != exitOK {
		t.Fatalf("second pack build expected %d got %d", exitOK, code)
	}

	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first pack: %v", err)
	}
	secondBytes, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second pack: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("expected deterministic pack bytes for repeated build")
	}
}

func TestRunPackVerifySchemaFailureExitCode(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo expected %d got %d", exitOK, code)
	}

	packPath := filepath.Join(workDir, "pack_run.zip")
	if code := runPack([]string{"build", "--type", "run", "--from", "run_demo", "--out", packPath, "--json"}); code != exitOK {
		t.Fatalf("pack build expected %d got %d", exitOK, code)
	}

	mutatedPath := filepath.Join(workDir, "pack_run_invalid_payload.zip")
	if err := mutateRunPackPayloadSchema(packPath, mutatedPath); err != nil {
		t.Fatalf("mutate run pack payload: %v", err)
	}

	if code := runPack([]string{"verify", mutatedPath, "--json"}); code != exitVerifyFailed {
		t.Fatalf("pack verify invalid payload expected %d got %d", exitVerifyFailed, code)
	}
}

func TestRunPackVerifyWrongKeyFailsInStandardMode(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo expected %d got %d", exitOK, code)
	}

	signingPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate signing key pair: %v", err)
	}
	signingPrivateKeyPath := filepath.Join(workDir, "signing_private.key")
	if err := os.WriteFile(signingPrivateKeyPath, []byte(base64.StdEncoding.EncodeToString(signingPair.Private)+"\n"), 0o600); err != nil {
		t.Fatalf("write signing private key: %v", err)
	}

	wrongPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate wrong key pair: %v", err)
	}
	wrongPublicKeyPath := filepath.Join(workDir, "wrong_public.key")
	if err := os.WriteFile(wrongPublicKeyPath, []byte(base64.StdEncoding.EncodeToString(wrongPair.Public)+"\n"), 0o600); err != nil {
		t.Fatalf("write wrong public key: %v", err)
	}

	packPath := filepath.Join(workDir, "signed_pack.zip")
	if code := runPack([]string{
		"build",
		"--type", "run",
		"--from", "run_demo",
		"--out", packPath,
		"--key-mode", "prod",
		"--private-key", signingPrivateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("signed pack build expected %d got %d", exitOK, code)
	}

	verifyCode, verifyOut := runPackJSON(t, []string{"verify", packPath, "--public-key", wrongPublicKeyPath, "--json"})
	if verifyCode != exitVerifyFailed {
		t.Fatalf("pack verify wrong-key standard mode expected %d got %d output=%#v", exitVerifyFailed, verifyCode, verifyOut)
	}
	if verifyOut.OK {
		t.Fatalf("expected verify output ok=false, got %#v", verifyOut)
	}
	if verifyOut.Verify == nil || verifyOut.Verify.SignatureStatus != "failed" {
		t.Fatalf("expected signature_status=failed, got %#v", verifyOut)
	}
}

func mutateRunPackPayloadSchema(srcPath string, dstPath string) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	payloadByName := map[string][]byte{}
	for _, file := range reader.File {
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(fileReader)
		_ = fileReader.Close()
		if err != nil {
			return err
		}
		payloadByName[file.Name] = content
	}
	payloadByName["run_payload.json"] = []byte(`{"schema_id":"gait.pack.run","schema_version":"1.0.0","created_at":"2026-02-14T00:00:00Z","run_id":"run_demo","capture_mode":"reference","manifest_digest":"` + strings.Repeat("a", 64) + `","intents_count":1,"results_count":1,"refs_count":1,"unexpected":"field"}`)

	// Build a valid manifest for the mutated payload so verify reaches schema/contract checks.
	manifestRaw, ok := payloadByName["pack_manifest.json"]
	if !ok {
		return os.ErrNotExist
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return err
	}
	delete(manifest, "signatures")
	contents, ok := manifest["contents"].([]any)
	if !ok {
		return os.ErrInvalid
	}
	for _, item := range contents {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pathValue, _ := entry["path"].(string)
		content, exists := payloadByName[pathValue]
		if !exists {
			continue
		}
		sum := sha256.Sum256(content)
		entry["sha256"] = hex.EncodeToString(sum[:])
	}
	manifest["pack_id"] = ""
	signable, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	canonicalSignable, err := jcs.CanonicalizeJSON(signable)
	if err != nil {
		return err
	}
	packID, err := jcs.DigestJCS(canonicalSignable)
	if err != nil {
		return err
	}
	manifest["pack_id"] = packID
	finalManifest, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	canonicalManifest, err := jcs.CanonicalizeJSON(finalManifest)
	if err != nil {
		return err
	}
	payloadByName["pack_manifest.json"] = canonicalManifest

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(payloadByName))
	for name := range payloadByName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		zipEntry, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := zipEntry.Write(payloadByName[name]); err != nil {
			_ = writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(dstPath, buffer.Bytes(), 0o600)
}

func writeVoiceCallRecordForPackCLI(t *testing.T, dir string, runpackPath string, callID string) string {
	t.Helper()
	createdAt := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	record := map[string]any{
		"schema_id":        "gait.voice.call_record",
		"schema_version":   "1.0.0",
		"created_at":       createdAt.Format(time.RFC3339Nano),
		"producer_version": "test",
		"call_id":          callID,
		"runpack_path":     runpackPath,
		"privacy_mode":     "hash_only",
		"events": []map[string]any{
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Format(time.RFC3339Nano), "call_id": callID, "call_seq": 1, "turn_index": 1, "event_type": "asr.final", "payload_digest": strings.Repeat("a", 64)},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(1 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 2, "turn_index": 1, "event_type": "commitment.declared", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(2 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 3, "turn_index": 1, "event_type": "gate.decision", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(3 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 4, "turn_index": 1, "event_type": "tts.request", "commitment_class": "quote"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(4 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 5, "turn_index": 1, "event_type": "tts.emitted", "commitment_class": "quote", "say_token_id": "say_demo"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(5 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 6, "turn_index": 1, "event_type": "tool.intent"},
			{"schema_id": "gait.voice.call_event", "schema_version": "1.0.0", "created_at": createdAt.Add(6 * time.Second).Format(time.RFC3339Nano), "call_id": callID, "call_seq": 7, "turn_index": 1, "event_type": "tool.result"},
		},
		"commitments": []map[string]any{
			{
				"schema_id":        "gait.voice.commitment_intent",
				"schema_version":   "1.0.0",
				"created_at":       createdAt.Add(1 * time.Second).Format(time.RFC3339Nano),
				"producer_version": "test",
				"call_id":          callID,
				"turn_index":       1,
				"call_seq":         2,
				"commitment_class": "quote",
				"context": map[string]any{
					"identity":   "agent.voice",
					"workspace":  "/srv/voice",
					"risk_class": "high",
				},
				"quote_min_cents": 1000,
				"quote_max_cents": 1200,
			},
		},
		"gate_decisions": []map[string]any{
			{
				"call_id":          callID,
				"call_seq":         3,
				"turn_index":       1,
				"commitment_class": "quote",
				"verdict":          "allow",
			},
		},
		"speak_receipts": []map[string]any{
			{
				"call_id":          callID,
				"call_seq":         5,
				"turn_index":       1,
				"commitment_class": "quote",
				"say_token_id":     "say_demo",
				"spoken_digest":    strings.Repeat("a", 64),
				"emitted_at":       createdAt.Add(4 * time.Second).Format(time.RFC3339Nano),
			},
		},
	}
	path := filepath.Join(dir, "call_record_"+callID+".json")
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal call record: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write call record: %v", err)
	}
	return path
}

func writeAuthorizationBundleFixtureForPackCLI(t *testing.T, dir string, outcomeStatus string) string {
	t.Helper()
	writeAuthorizationFixtureJSONForPackCLI(t, filepath.Join(dir, "trace.json"), map[string]any{
		"schema_id":        "gait.gate.trace",
		"schema_version":   "1.0.0",
		"created_at":       "2026-05-09T00:00:00Z",
		"producer_version": "test",
		"trace_id":         "trace-auth-demo",
		"tool_name":        "tool.write",
		"args_digest":      strings.Repeat("c", 64),
		"intent_digest":    strings.Repeat("b", 64),
		"policy_digest":    strings.Repeat("a", 64),
		"verdict":          outcomeStatus,
	})
	writeAuthorizationFixtureJSONForPackCLI(t, filepath.Join(dir, "approval_audit.json"), map[string]any{"approved": outcomeStatus == "allow"})
	writeAuthorizationFixtureJSONForPackCLI(t, filepath.Join(dir, "credential_evidence.json"), map[string]any{"credential_ref": "stub:1234"})
	writeAuthorizationFixtureJSONForPackCLI(t, filepath.Join(dir, "outcome_receipt.json"), map[string]any{"status": outcomeStatus, "executed": outcomeStatus == "allow"})

	bundle := map[string]any{
		"schema_id":                "gait.gate.authorization_bundle",
		"schema_version":           "1.0.0",
		"created_at":               "2026-05-09T00:00:00Z",
		"producer_version":         "test",
		"trace_id":                 "trace-auth-demo",
		"policy_digest":            strings.Repeat("a", 64),
		"intent_digest":            strings.Repeat("b", 64),
		"trace_path":               "trace.json",
		"approval_audit_path":      "approval_audit.json",
		"credential_evidence_path": "credential_evidence.json",
		"outcome_path":             "outcome_receipt.json",
		"outcome_status":           outcomeStatus,
		"freeze_window":            map[string]any{"status": "inactive"},
		"kill_switch":              map[string]any{"status": "inactive"},
		"sandbox":                  map[string]any{"status": "valid", "evidence_digest": strings.Repeat("d", 64)},
	}
	specPath := filepath.Join(dir, "authorization_bundle.json")
	writeAuthorizationFixtureJSONForPackCLI(t, specPath, bundle)
	return specPath
}

func writeAuthorizationFixtureJSONForPackCLI(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal authorization fixture: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write authorization fixture: %v", err)
	}
}

func runPackJSON(t *testing.T, args []string) (int, packOutput) {
	t.Helper()
	var code int
	raw := captureStdout(t, func() {
		code = runPack(args)
	})
	var output packOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode pack output: %v raw=%q", err, raw)
	}
	return code, output
}

func TestWritePackOutputTextBranches(t *testing.T) {
	successCases := []struct {
		name      string
		output    packOutput
		expectSub string
	}{
		{
			name:      "build",
			output:    packOutput{OK: true, Operation: "build", Path: "pack.zip", PackType: "run"},
			expectSub: "pack build ok:",
		},
		{
			name:      "verify",
			output:    packOutput{OK: true, Operation: "verify", Path: "pack.zip"},
			expectSub: "pack verify ok:",
		},
		{
			name:      "inspect",
			output:    packOutput{OK: true, Operation: "inspect", PackID: "abc", PackType: "run"},
			expectSub: "pack inspect ok:",
		},
		{
			name:      "diff",
			output:    packOutput{OK: true, Operation: "diff"},
			expectSub: "pack diff ok",
		},
		{
			name:      "export",
			output:    packOutput{OK: true, Operation: "export", Path: "pack.zip", OTelPath: "pack.otel.jsonl", PostgresSQL: "pack_index.sql"},
			expectSub: "pack export ok:",
		},
	}

	for _, testCase := range successCases {
		t.Run(testCase.name, func(t *testing.T) {
			var code int
			text := captureStdout(t, func() {
				code = writePackOutput(false, testCase.output, exitOK)
			})
			if code != exitOK {
				t.Fatalf("writePackOutput expected %d got %d", exitOK, code)
			}
			if !strings.Contains(text, testCase.expectSub) {
				t.Fatalf("expected output to contain %q, got %q", testCase.expectSub, text)
			}
		})
	}

	var warningCode int
	warningText := captureStdout(t, func() {
		warningCode = writePackOutput(false, packOutput{
			OK:        true,
			Operation: "build",
			Path:      "pack.zip",
			PackType:  "run",
			Warnings:  []string{"warn-1", "warn-2"},
		}, exitOK)
	})
	if warningCode != exitOK {
		t.Fatalf("writePackOutput warnings expected %d got %d", exitOK, warningCode)
	}
	if !strings.Contains(warningText, "warnings:") {
		t.Fatalf("expected warnings output, got %q", warningText)
	}

	var errorCode int
	errorText := captureStdout(t, func() {
		errorCode = writePackOutput(false, packOutput{
			OK:        false,
			Operation: "verify",
			Error:     "bad verify",
		}, exitVerifyFailed)
	})
	if errorCode != exitVerifyFailed {
		t.Fatalf("writePackOutput error expected %d got %d", exitVerifyFailed, errorCode)
	}
	if !strings.Contains(errorText, "pack verify error: bad verify") {
		t.Fatalf("expected error output, got %q", errorText)
	}
}
