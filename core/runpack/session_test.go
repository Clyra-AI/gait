package runpack

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestSessionJournalLifecycleAndCheckpointChain(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "demo.journal.jsonl")
	startedAt := time.Date(2026, time.February, 11, 0, 0, 0, 0, time.UTC)

	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID:       "sess_demo",
		RunID:           "run_demo",
		ProducerVersion: "test",
		Now:             startedAt,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_demo",
		RunID:     "run_demo",
		Now:       startedAt,
	}); err != nil {
		t.Fatalf("start existing session should succeed: %v", err)
	}

	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:    startedAt.Add(1 * time.Second),
		IntentID:     "intent_1",
		ToolName:     "tool.read",
		IntentDigest: strings.Repeat("a", 64),
		PolicyDigest: strings.Repeat("b", 64),
		TraceID:      "trace_1",
		TracePath:    filepath.Join(workDir, "traces", "trace_1.json"),
		Verdict:      "allow",
		ReasonCodes:  []string{"ok", "ok", "policy_allow"},
		Violations:   []string{"", "none"},
	}); err != nil {
		t.Fatalf("append session event 1: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:      startedAt.Add(2 * time.Second),
		IntentID:       "intent_2",
		ToolName:       "tool.write",
		IntentDigest:   strings.Repeat("c", 64),
		PolicyDigest:   strings.Repeat("d", 64),
		PolicyID:       "gait.gate.policy",
		PolicyVersion:  "1.0.0",
		MatchedRuleIDs: []string{"allow-write", "allow-write"},
		TraceID:        "trace_2",
		TracePath:      filepath.Join(workDir, "traces", "trace_2.json"),
		Verdict:        "block",
		ReasonCodes:    []string{"policy_block"},
		Violations:     []string{"unauthorized_target"},
	}); err != nil {
		t.Fatalf("append session event 2: %v", err)
	}

	journalAfterAppend, err := ReadSessionJournal(journalPath)
	if err != nil {
		t.Fatalf("read session journal after append: %v", err)
	}
	if len(journalAfterAppend.Events) != 2 {
		t.Fatalf("expected two session events after append, got %d", len(journalAfterAppend.Events))
	}
	secondEvent := journalAfterAppend.Events[1]
	if secondEvent.PolicyID != "gait.gate.policy" || secondEvent.PolicyVersion != "1.0.0" {
		t.Fatalf("expected policy lineage on session event, got id=%q version=%q", secondEvent.PolicyID, secondEvent.PolicyVersion)
	}
	if len(secondEvent.MatchedRuleIDs) != 1 || secondEvent.MatchedRuleIDs[0] != "allow-write" {
		t.Fatalf("expected normalized matched_rule_ids on session event, got %#v", secondEvent.MatchedRuleIDs)
	}
	if secondEvent.Relationship == nil || secondEvent.Relationship.ParentRef == nil || secondEvent.Relationship.ParentRef.Kind != "session" {
		t.Fatalf("expected relationship envelope with session parent on session event: %#v", secondEvent.Relationship)
	}

	status, err := GetSessionStatus(journalPath)
	if err != nil {
		t.Fatalf("get session status: %v", err)
	}
	if status.EventCount != 2 || status.LastSequence != 2 {
		t.Fatalf("unexpected status after append: %#v", status)
	}

	checkpoint1Path := filepath.Join(workDir, "gait-out", "session_cp_0001.zip")
	checkpoint1, chainPath, err := SessionCheckpointAndWriteChain(journalPath, checkpoint1Path, SessionCheckpointOptions{
		Now:             startedAt.Add(5 * time.Second),
		ProducerVersion: "test",
	})
	if err != nil {
		t.Fatalf("checkpoint 1: %v", err)
	}
	if checkpoint1.Checkpoint.CheckpointIndex != 1 {
		t.Fatalf("unexpected first checkpoint index: %d", checkpoint1.Checkpoint.CheckpointIndex)
	}
	if checkpoint1.Checkpoint.Relationship == nil || checkpoint1.Checkpoint.Relationship.ParentRef == nil || checkpoint1.Checkpoint.Relationship.ParentRef.Kind != "session" {
		t.Fatalf("expected relationship envelope on checkpoint: %#v", checkpoint1.Checkpoint.Relationship)
	}
	if !ContainsSessionChainPath(chainPath) {
		t.Fatalf("expected json session chain path, got %s", chainPath)
	}
	firstChainPath := filepath.Join(workDir, "sessions", "chain_first.json")
	if err := WriteSessionChain(firstChainPath, checkpoint1.Chain); err != nil {
		t.Fatalf("write first chain snapshot: %v", err)
	}

	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:    startedAt.Add(7 * time.Second),
		IntentID:     "intent_3",
		ToolName:     "tool.delete",
		IntentDigest: strings.Repeat("e", 64),
		PolicyDigest: strings.Repeat("f", 64),
		TraceID:      "trace_3",
		Verdict:      "require_approval",
		ReasonCodes:  []string{"approval_required"},
	}); err != nil {
		t.Fatalf("append session event 3: %v", err)
	}
	checkpoint2Path := filepath.Join(workDir, "gait-out", "session_cp_0002.zip")
	checkpoint2, _, err := SessionCheckpointAndWriteChain(journalPath, checkpoint2Path, SessionCheckpointOptions{
		Now:             startedAt.Add(10 * time.Second),
		ProducerVersion: "test",
	})
	if err != nil {
		t.Fatalf("checkpoint 2: %v", err)
	}
	if checkpoint2.Checkpoint.CheckpointIndex != 2 {
		t.Fatalf("unexpected second checkpoint index: %d", checkpoint2.Checkpoint.CheckpointIndex)
	}
	if checkpoint2.Checkpoint.PrevCheckpointDigest != checkpoint1.Checkpoint.CheckpointDigest {
		t.Fatalf("checkpoint digest linkage mismatch")
	}

	chain, err := ReadSessionChain(chainPath)
	if err != nil {
		t.Fatalf("read session chain: %v", err)
	}
	if len(chain.Checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints in chain, got %d", len(chain.Checkpoints))
	}

	verifyResult, err := VerifySessionChain(chainPath, SessionChainVerifyOptions{})
	if err != nil {
		t.Fatalf("verify session chain: %v", err)
	}
	if len(verifyResult.LinkageErrors) != 0 || len(verifyResult.CheckpointErrors) != 0 {
		t.Fatalf("expected clean session chain verify result, got %#v", verifyResult)
	}

	latest, err := ResolveSessionCheckpointRunpack(chainPath, "latest")
	if err != nil {
		t.Fatalf("resolve latest checkpoint: %v", err)
	}
	if latest.CheckpointIndex != 2 {
		t.Fatalf("expected latest checkpoint index 2, got %d", latest.CheckpointIndex)
	}
	first, err := ResolveSessionCheckpointRunpack(chainPath, "1")
	if err != nil {
		t.Fatalf("resolve checkpoint 1: %v", err)
	}
	if first.CheckpointIndex != 1 {
		t.Fatalf("expected checkpoint index 1, got %d", first.CheckpointIndex)
	}

	if _, err := EmitSessionCheckpoint(journalPath, filepath.Join(workDir, "gait-out", "session_cp_0003.zip"), SessionCheckpointOptions{
		Now:             startedAt.Add(11 * time.Second),
		ProducerVersion: "test",
	}); err == nil || !strings.Contains(err.Error(), "no new session events") {
		t.Fatalf("expected no new events error, got %v", err)
	}

	diff := DiffSessionChains(checkpoint1.Chain, checkpoint2.Chain)
	if !diff.ChangedCheckpoints || len(diff.RightOnlyIndexes) != 1 || diff.RightOnlyIndexes[0] != 2 {
		t.Fatalf("unexpected session chain diff summary: %#v", diff)
	}

	compareResult, err := CompareRunpackOrSessionChain(firstChainPath, chainPath, DiffPrivacyMetadata)
	if err != nil {
		t.Fatalf("compare session chains: %v", err)
	}
	if !compareResult.Summary.ManifestChanged {
		t.Fatalf("expected manifest changed in session chain compare summary")
	}
}

func TestSessionJournalValidationErrors(t *testing.T) {
	workDir := t.TempDir()

	parsePath := filepath.Join(workDir, "parse_error.journal.jsonl")
	if err := os.WriteFile(parsePath, []byte("{\n"), 0o600); err != nil {
		t.Fatalf("write parse error journal: %v", err)
	}
	if _, err := ReadSessionJournal(parsePath); err == nil || !strings.Contains(err.Error(), "parse line") {
		t.Fatalf("expected parse line error, got %v", err)
	}

	eventBeforeHeaderPath := filepath.Join(workDir, "event_before_header.journal.jsonl")
	content := `{"record_type":"event","event":{"session_id":"sess","run_id":"run","sequence":1}}
`
	if err := os.WriteFile(eventBeforeHeaderPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write event-before-header journal: %v", err)
	}
	if _, err := ReadSessionJournal(eventBeforeHeaderPath); err == nil || !strings.Contains(err.Error(), "event encountered before header") {
		t.Fatalf("expected event-before-header error, got %v", err)
	}

	unsupportedRecordPath := filepath.Join(workDir, "unsupported_record.journal.jsonl")
	unsupportedRecord := `{"record_type":"header","header":{"session_id":"sess","run_id":"run","created_at":"2026-02-11T00:00:00Z","started_at":"2026-02-11T00:00:00Z"}}
{"record_type":"unsupported"}
`
	if err := os.WriteFile(unsupportedRecordPath, []byte(unsupportedRecord), 0o600); err != nil {
		t.Fatalf("write unsupported record journal: %v", err)
	}
	if _, err := ReadSessionJournal(unsupportedRecordPath); err == nil || !strings.Contains(err.Error(), "unsupported record_type") {
		t.Fatalf("expected unsupported record_type error, got %v", err)
	}

	duplicateHeaderPath := filepath.Join(workDir, "duplicate_header.journal.jsonl")
	duplicateHeader := `{"record_type":"header","header":{"session_id":"sess","run_id":"run","created_at":"2026-02-11T00:00:00Z","started_at":"2026-02-11T00:00:00Z"}}
{"record_type":"header","header":{"session_id":"sess","run_id":"run","created_at":"2026-02-11T00:00:00Z","started_at":"2026-02-11T00:00:00Z"}}
`
	if err := os.WriteFile(duplicateHeaderPath, []byte(duplicateHeader), 0o600); err != nil {
		t.Fatalf("write duplicate header journal: %v", err)
	}
	if _, err := ReadSessionJournal(duplicateHeaderPath); err == nil || !strings.Contains(err.Error(), "duplicate header") {
		t.Fatalf("expected duplicate header error, got %v", err)
	}
}

func TestSessionInputAndChainValidation(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "input.journal.jsonl")
	now := time.Date(2026, time.February, 11, 2, 0, 0, 0, time.UTC)

	if _, err := StartSession("", SessionStartOptions{
		SessionID: "sess_input",
		RunID:     "run_input",
	}); err == nil {
		t.Fatalf("expected missing journal path error")
	}
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "",
		RunID:     "run_input",
		Now:       now,
	}); err == nil {
		t.Fatalf("expected missing session_id error")
	}
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_input",
		RunID:     "run_input",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_other",
		RunID:     "run_input",
		Now:       now,
	}); err == nil || !strings.Contains(err.Error(), "different session/run") {
		t.Fatalf("expected mismatched session/run error, got %v", err)
	}

	orphanJournal := filepath.Join(workDir, "sessions", "orphan.journal.jsonl")
	if _, err := AppendSessionEvent(orphanJournal, SessionAppendOptions{
		IntentID: "intent_orphan",
	}); err == nil {
		t.Fatalf("expected append without header to fail")
	}

	chainPath := filepath.Join(workDir, "sessions", "chain_defaults.json")
	chain := schemarunpack.SessionChain{
		SessionID: "sess_input",
		RunID:     "run_input",
		Checkpoints: []schemarunpack.SessionCheckpoint{
			{
				CheckpointIndex:  1,
				SequenceStart:    1,
				SequenceEnd:      1,
				RunpackPath:      filepath.Join(workDir, "runpack.zip"),
				ManifestDigest:   strings.Repeat("a", 64),
				CheckpointDigest: computeCheckpointDigest(strings.Repeat("a", 64), "", 1, 1, 1),
			},
		},
	}
	if err := WriteSessionChain(chainPath, chain); err != nil {
		t.Fatalf("write chain with defaults: %v", err)
	}
	loaded, err := ReadSessionChain(chainPath)
	if err != nil {
		t.Fatalf("read chain written with defaults: %v", err)
	}
	if loaded.SchemaID != sessionChainSchemaID || loaded.SchemaVersion != sessionChainSchemaVersion {
		t.Fatalf("expected default schema fields to be applied, got %#v", loaded)
	}
}

func TestReadSessionChainValidationErrors(t *testing.T) {
	workDir := t.TempDir()

	unsupportedSchemaPath := filepath.Join(workDir, "unsupported_chain.json")
	unsupportedSchema := `{
  "schema_id":"unsupported",
  "schema_version":"1.0.0",
  "session_id":"sess",
  "run_id":"run",
  "checkpoints":[{"checkpoint_index":1,"sequence_start":1,"sequence_end":1,"runpack_path":"runpack.zip","manifest_digest":"abc","checkpoint_digest":"def"}]
}`
	if err := os.WriteFile(unsupportedSchemaPath, []byte(unsupportedSchema), 0o600); err != nil {
		t.Fatalf("write unsupported schema chain: %v", err)
	}
	if _, err := ReadSessionChain(unsupportedSchemaPath); err == nil || !strings.Contains(err.Error(), "unsupported session chain schema") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}

	noCheckpointsPath := filepath.Join(workDir, "no_checkpoints_chain.json")
	noCheckpoints := `{
  "schema_id":"gait.runpack.session_chain",
  "schema_version":"1.0.0",
  "session_id":"sess",
  "run_id":"run",
  "checkpoints":[]
}`
	if err := os.WriteFile(noCheckpointsPath, []byte(noCheckpoints), 0o600); err != nil {
		t.Fatalf("write no checkpoints chain: %v", err)
	}
	if _, err := ReadSessionChain(noCheckpointsPath); err == nil || !strings.Contains(err.Error(), "has no checkpoints") {
		t.Fatalf("expected no checkpoints validation error, got %v", err)
	}

	missingIdentityPath := filepath.Join(workDir, "missing_identity_chain.json")
	missingIdentity := `{
  "schema_id":"gait.runpack.session_chain",
  "schema_version":"1.0.0",
  "session_id":"",
  "run_id":"",
  "checkpoints":[{"checkpoint_index":1,"sequence_start":1,"sequence_end":1,"runpack_path":"runpack.zip","manifest_digest":"abc","checkpoint_digest":"def"}]
}`
	if err := os.WriteFile(missingIdentityPath, []byte(missingIdentity), 0o600); err != nil {
		t.Fatalf("write missing identity chain: %v", err)
	}
	if _, err := ReadSessionChain(missingIdentityPath); err == nil || !strings.Contains(err.Error(), "missing session_id/run_id") {
		t.Fatalf("expected missing identity validation error, got %v", err)
	}
}

func TestSessionChainVerifyDetectsTamper(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "tamper.journal.jsonl")
	now := time.Date(2026, time.February, 11, 1, 0, 0, 0, time.UTC)

	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_tamper",
		RunID:     "run_tamper",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:    now.Add(1 * time.Second),
		IntentID:     "intent_tamper",
		ToolName:     "tool.write",
		IntentDigest: strings.Repeat("1", 64),
		PolicyDigest: strings.Repeat("2", 64),
		Verdict:      "allow",
	}); err != nil {
		t.Fatalf("append session event: %v", err)
	}
	checkpoint, chainPath, err := SessionCheckpointAndWriteChain(journalPath, filepath.Join(workDir, "gait-out", "tamper_cp_0001.zip"), SessionCheckpointOptions{
		Now: now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("checkpoint session: %v", err)
	}

	tampered := checkpoint.Chain
	tampered.Checkpoints[0].CheckpointDigest = strings.Repeat("0", 64)
	tampered.Checkpoints[0].RunpackPath = filepath.Join(workDir, "missing", "checkpoint.zip")
	tamperedPath := filepath.Join(workDir, "sessions", "tampered_chain.json")
	if err := WriteSessionChain(tamperedPath, tampered); err != nil {
		t.Fatalf("write tampered chain: %v", err)
	}

	result, err := VerifySessionChain(tamperedPath, SessionChainVerifyOptions{})
	if err != nil {
		t.Fatalf("verify tampered session chain should return structured result: %v", err)
	}
	if len(result.LinkageErrors) == 0 {
		t.Fatalf("expected linkage errors for tampered chain")
	}
	if len(result.CheckpointErrors) == 0 {
		t.Fatalf("expected checkpoint verification errors for tampered chain")
	}

	if _, err := ResolveSessionCheckpointRunpack(chainPath, "invalid"); err == nil {
		t.Fatalf("expected invalid checkpoint ref error")
	}
	if _, err := ResolveSessionCheckpointRunpack(chainPath, "99"); err == nil {
		t.Fatalf("expected missing checkpoint error")
	}
}

func TestSessionLockRecoveryAndHelpers(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "lock.journal.jsonl")
	lockPath := journalPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		t.Fatalf("create lock dir: %v", err)
	}
	staleCreatedAt := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	if err := os.WriteFile(lockPath, []byte(`{"created_at":"`+staleCreatedAt+`"}`), 0o600); err != nil {
		t.Fatalf("write stale lock file: %v", err)
	}
	if !shouldRecoverStaleSessionLock(lockPath, time.Now().UTC()) {
		t.Fatalf("expected stale lock to be recoverable")
	}

	called := false
	if err := withSessionLock(journalPath, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withSessionLock should recover stale lock: %v", err)
	}
	if !called {
		t.Fatalf("expected lock callback to run")
	}

	freshLockPath := filepath.Join(workDir, "sessions", "fresh.lock")
	if err := os.WriteFile(freshLockPath, []byte(`{"created_at":"`+time.Now().UTC().Format(time.RFC3339)+`"}`), 0o600); err != nil {
		t.Fatalf("write fresh lock file: %v", err)
	}
	if shouldRecoverStaleSessionLock(freshLockPath, time.Now().UTC()) {
		t.Fatalf("did not expect fresh lock to be recoverable")
	}

	if got := sessionChainFromJournalPath(journalPath); !strings.HasSuffix(got, "_chain.json") {
		t.Fatalf("unexpected chain path: %s", got)
	}
	if sessionChainLooksLike(filepath.Join(workDir, "chain.zip")) {
		t.Fatalf("expected non-json path not to look like session chain")
	}
	if maybe, ok := maybeReadSessionChain(filepath.Join(workDir, "does_not_exist.json")); ok || len(maybe.Checkpoints) != 0 {
		t.Fatalf("expected non-readable chain probe to return false")
	}
}

func TestIsSessionLockContention(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "session.lock")
	permissionErr := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrPermission}
	accessDeniedErr := &os.PathError{Op: "open", Path: lockPath, Err: errors.New("Access is denied.")}

	if !isSessionLockContention(os.ErrExist, lockPath) {
		t.Fatalf("expected os.ErrExist to be lock contention")
	}
	if isSessionLockContention(permissionErr, lockPath) {
		t.Fatalf("expected permission error without existing lock to be non-contention")
	}
	if isSessionLockContention(accessDeniedErr, lockPath) {
		t.Fatalf("expected access denied without existing lock to be non-contention")
	}
	if err := os.WriteFile(lockPath, []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	if !isSessionLockContention(permissionErr, lockPath) {
		t.Fatalf("expected permission error with lock present to be contention")
	}
	if !isSessionLockContention(accessDeniedErr, lockPath) {
		t.Fatalf("expected access denied with lock present to be contention")
	}
	missingLockPath := filepath.Join(t.TempDir(), "missing.lock")
	if isSessionLockContention(os.ErrNotExist, missingLockPath) {
		t.Fatalf("expected non-contention error")
	}
}

func TestIsWindowsAccessDeniedLockError(t *testing.T) {
	t.Parallel()

	deniedErr := &os.PathError{Op: "open", Path: "session.lock", Err: errors.New("Access is denied.")}
	expected := runtime.GOOS == "windows"
	if got := isWindowsAccessDeniedLockError(deniedErr); got != expected {
		t.Fatalf("unexpected access denied classification: got=%v want=%v", got, expected)
	}
	if isWindowsAccessDeniedLockError(nil) {
		t.Fatalf("expected nil error to be non-contention")
	}
}

func TestVerifySessionChainRequireSignature(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "require_signature.journal.jsonl")
	now := time.Date(2026, time.February, 11, 4, 0, 0, 0, time.UTC)

	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_require_signature",
		RunID:     "run_require_signature",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:    now.Add(1 * time.Second),
		IntentID:     "intent_signature",
		ToolName:     "tool.write",
		IntentDigest: strings.Repeat("1", 64),
		PolicyDigest: strings.Repeat("2", 64),
		Verdict:      "allow",
	}); err != nil {
		t.Fatalf("append session event: %v", err)
	}
	_, chainPath, err := SessionCheckpointAndWriteChain(journalPath, filepath.Join(workDir, "gait-out", "require_signature_cp_0001.zip"), SessionCheckpointOptions{
		Now: now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("checkpoint session: %v", err)
	}

	result, err := VerifySessionChain(chainPath, SessionChainVerifyOptions{
		RequireSignature: true,
	})
	if err != nil {
		t.Fatalf("verify session chain require signature: %v", err)
	}
	if len(result.CheckpointErrors) == 0 {
		t.Fatalf("expected signature verification failure when signatures are required")
	}
}

func TestCompareRunpackOrSessionChainFallsBackToRunpackDiff(t *testing.T) {
	leftPath := writeTestRunpack(t, "run_compare_left", buildIntents("intent_1"), buildResults("intent_1"))
	rightPath := writeTestRunpack(t, "run_compare_right", buildIntents("intent_1"), buildResults("intent_1"))

	result, err := CompareRunpackOrSessionChain(leftPath, rightPath, DiffPrivacyMetadata)
	if err != nil {
		t.Fatalf("compare runpack fallback: %v", err)
	}
	if result.Summary.RunIDLeft == "" || result.Summary.RunIDRight == "" {
		t.Fatalf("expected run ids in compare result summary: %#v", result.Summary)
	}
}

func TestSessionConcurrentAppendsAreDeterministic(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "concurrent.journal.jsonl")
	now := time.Date(2026, time.February, 11, 5, 0, 0, 0, time.UTC)
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_concurrent",
		RunID:     "run_concurrent",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}

	const workers = 12
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		worker := i
		go func() {
			defer wg.Done()
			_, err := AppendSessionEvent(journalPath, SessionAppendOptions{
				CreatedAt:    now.Add(time.Duration(worker+1) * time.Second),
				IntentID:     "intent_" + strconv.Itoa(worker),
				ToolName:     "tool.write",
				IntentDigest: strings.Repeat("a", 64),
				PolicyDigest: strings.Repeat("b", 64),
				Verdict:      "allow",
			})
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("append session event in concurrent worker: %v", err)
		}
	}

	journal, err := ReadSessionJournal(journalPath)
	if err != nil {
		t.Fatalf("read session journal: %v", err)
	}
	if len(journal.Events) != workers {
		t.Fatalf("expected %d events, got %d", workers, len(journal.Events))
	}
	for i, event := range journal.Events {
		want := int64(i + 1)
		if event.Sequence != want {
			t.Fatalf("sequence mismatch at index %d: got %d want %d", i, event.Sequence, want)
		}
	}
}

func TestSessionAppendLatencyDriftBudget(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "latency.journal.jsonl")
	now := time.Date(2026, time.February, 11, 5, 30, 0, 0, time.UTC)
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_latency",
		RunID:     "run_latency",
		Now:       now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}

	const samples = 240
	durations := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		_, err := AppendSessionEvent(journalPath, SessionAppendOptions{
			CreatedAt: now.Add(time.Duration(i+1) * time.Second),
			IntentID:  "intent_latency_" + strconv.Itoa(i+1),
			ToolName:  "tool.write",
			Verdict:   "allow",
		})
		if err != nil {
			t.Fatalf("append latency sample %d: %v", i+1, err)
		}
		durations = append(durations, time.Since(start))
	}

	var firstWindowTotal time.Duration
	var lastWindowTotal time.Duration
	for i := 0; i < 40; i++ {
		firstWindowTotal += durations[i]
		lastWindowTotal += durations[len(durations)-1-i]
	}
	firstAvg := firstWindowTotal / 40
	lastAvg := lastWindowTotal / 40
	// Keep a generous gate to avoid noisy CI while still detecting runaway growth.
	if lastAvg > firstAvg*8 {
		t.Fatalf("append latency drift exceeded budget: first_avg=%s last_avg=%s", firstAvg, lastAvg)
	}
}

func TestSessionRelativePathGuards(t *testing.T) {
	workDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	journalPath := filepath.Join("sessions", "relative.journal.jsonl")
	now := time.Date(2026, time.February, 11, 6, 0, 0, 0, time.UTC)
	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID: "sess_relative",
		RunID:     "run_relative",
		Now:       now,
	}); err != nil {
		t.Fatalf("start relative session: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt:    now.Add(1 * time.Second),
		IntentID:     "intent_relative",
		ToolName:     "tool.read",
		IntentDigest: strings.Repeat("1", 64),
		PolicyDigest: strings.Repeat("2", 64),
		Verdict:      "allow",
	}); err != nil {
		t.Fatalf("append relative event: %v", err)
	}
	if _, err := ReadSessionJournal(journalPath); err != nil {
		t.Fatalf("read relative journal: %v", err)
	}
	if err := withSessionLock(journalPath, func() error { return nil }); err != nil {
		t.Fatalf("lock relative journal: %v", err)
	}

	lockPath := journalPath + ".lock"
	staleCreatedAt := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	if err := os.WriteFile(lockPath, []byte(`{"created_at":"`+staleCreatedAt+`"}`), 0o600); err != nil {
		t.Fatalf("write stale relative lock: %v", err)
	}
	if !shouldRecoverStaleSessionLock(lockPath, time.Now().UTC()) {
		t.Fatalf("expected stale relative lock to be recoverable")
	}
}

func TestSessionRejectsParentTraversalPaths(t *testing.T) {
	now := time.Date(2026, time.February, 11, 7, 0, 0, 0, time.UTC)

	if _, err := StartSession("../outside.journal.jsonl", SessionStartOptions{
		SessionID: "sess_invalid",
		RunID:     "run_invalid",
		Now:       now,
	}); err == nil {
		t.Fatalf("expected start session to reject parent traversal path")
	}

	record := sessionJournalRecord{
		RecordType: "header",
		Header: &schemarunpack.SessionJournal{
			SessionID: "sess_invalid",
			RunID:     "run_invalid",
			CreatedAt: now,
			StartedAt: now,
		},
	}
	if err := appendJournalRecord("../outside.journal.jsonl", record); err == nil || !strings.Contains(err.Error(), "session journal directory must be local relative or absolute") {
		t.Fatalf("expected appendJournalRecord traversal guard error, got %v", err)
	}

	if err := withSessionLock("../outside.journal.jsonl", func() error { return nil }); err == nil || !strings.Contains(err.Error(), "session lock directory must be local relative or absolute") {
		t.Fatalf("expected withSessionLock traversal guard error, got %v", err)
	}

	if shouldRecoverStaleSessionLock("../outside.lock", now) {
		t.Fatalf("expected traversal lock path not to be recoverable")
	}
}

func TestSessionPathHelpersEnforceLocalOrAbsolute(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "session_state.json")
	content := []byte(`{"ok":true}`)
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("write helper fixture: %v", err)
	}

	info, err := statLocalOrAbsolutePath(filePath)
	if err != nil {
		t.Fatalf("statLocalOrAbsolutePath absolute path: %v", err)
	}
	if info.Size() != int64(len(content)) {
		t.Fatalf("unexpected stat size: got=%d want=%d", info.Size(), len(content))
	}

	read, err := readFileLocalOrAbsolutePath(filePath)
	if err != nil {
		t.Fatalf("readFileLocalOrAbsolutePath absolute path: %v", err)
	}
	if string(read) != string(content) {
		t.Fatalf("unexpected helper read content: %q", string(read))
	}

	if _, err := statLocalOrAbsolutePath(filepath.Join("..", "outside.json")); err == nil {
		t.Fatalf("expected traversal stat path to be rejected")
	}
	if _, err := readFileLocalOrAbsolutePath(filepath.Join("..", "outside.json")); err == nil {
		t.Fatalf("expected traversal read path to be rejected")
	}
}

func TestSessionJournalAndLockErrorPaths(t *testing.T) {
	workDir := t.TempDir()
	now := time.Date(2026, time.February, 11, 8, 0, 0, 0, time.UTC)

	malformed := filepath.Join(workDir, "sessions", "malformed.journal.jsonl")
	if err := os.MkdirAll(filepath.Dir(malformed), 0o750); err != nil {
		t.Fatalf("mkdir malformed journal dir: %v", err)
	}
	if err := os.WriteFile(malformed, []byte("{\n"), 0o600); err != nil {
		t.Fatalf("write malformed journal: %v", err)
	}
	if _, err := StartSession(malformed, SessionStartOptions{
		SessionID: "sess_malformed",
		RunID:     "run_malformed",
		Now:       now,
	}); err == nil || !strings.Contains(err.Error(), "parse line") {
		t.Fatalf("expected start session parse error from malformed journal, got %v", err)
	}

	validJournal := filepath.Join(workDir, "sessions", "lock_error.journal.jsonl")
	if err := withSessionLock(validJournal, func() error { return os.ErrPermission }); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected lock callback error to propagate, got %v", err)
	}

	badJSONLock := filepath.Join(workDir, "sessions", "bad_json.lock")
	if err := os.WriteFile(badJSONLock, []byte("{"), 0o600); err != nil {
		t.Fatalf("write bad json lock: %v", err)
	}
	if shouldRecoverStaleSessionLock(badJSONLock, time.Now().UTC()) {
		t.Fatalf("expected bad json lock not to be recoverable")
	}

	badTimeLock := filepath.Join(workDir, "sessions", "bad_time.lock")
	if err := os.WriteFile(badTimeLock, []byte(`{"created_at":"not-a-time"}`), 0o600); err != nil {
		t.Fatalf("write bad time lock: %v", err)
	}
	if shouldRecoverStaleSessionLock(badTimeLock, time.Now().UTC()) {
		t.Fatalf("expected bad timestamp lock not to be recoverable")
	}
}

func TestWithSessionLockTimeoutOnFreshLock(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "timeout.journal.jsonl")
	lockPath := journalPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		t.Fatalf("create lock directory: %v", err)
	}
	freshCreatedAt := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(lockPath, []byte(`{"created_at":"`+freshCreatedAt+`"}`), 0o600); err != nil {
		t.Fatalf("write fresh lock: %v", err)
	}
	start := time.Now()
	err := withSessionLock(journalPath, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "lock timeout") {
		t.Fatalf("expected lock timeout error, got %v", err)
	}
	if time.Since(start) < sessionLockTimeout {
		t.Fatalf("expected lock acquisition to wait for timeout window")
	}
}

func TestSessionStateIndexRebuildsFromJournalDrift(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "state.journal.jsonl")
	now := time.Date(2026, time.February, 11, 9, 0, 0, 0, time.UTC)

	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID:       "sess_state",
		RunID:           "run_state",
		ProducerVersion: "test",
		Now:             now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt: now.Add(1 * time.Second),
		IntentID:  "intent_1",
		ToolName:  "tool.read",
		Verdict:   "allow",
	}); err != nil {
		t.Fatalf("append event 1: %v", err)
	}
	statePath := sessionStatePath(journalPath)
	state, err := readSessionState(statePath)
	if err != nil {
		t.Fatalf("read session state: %v", err)
	}
	if state.LastSequence != 1 || state.EventCount != 1 {
		t.Fatalf("unexpected session state after first append: %#v", state)
	}

	// Simulate drift from crash between journal append and state update.
	state.LastSequence = 0
	state.EventCount = 0
	state.JournalSizeBytes = 1
	if err := writeSessionState(statePath, state); err != nil {
		t.Fatalf("write drifted session state: %v", err)
	}

	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt: now.Add(2 * time.Second),
		IntentID:  "intent_2",
		ToolName:  "tool.read",
		Verdict:   "allow",
	}); err != nil {
		t.Fatalf("append event 2 with drifted state: %v", err)
	}

	journal, err := ReadSessionJournal(journalPath)
	if err != nil {
		t.Fatalf("read session journal: %v", err)
	}
	if len(journal.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(journal.Events))
	}
	if journal.Events[0].Sequence != 1 || journal.Events[1].Sequence != 2 {
		t.Fatalf("unexpected event sequences after rebuild: %#v", journal.Events)
	}
	rebuilt, err := readSessionState(statePath)
	if err != nil {
		t.Fatalf("read rebuilt session state: %v", err)
	}
	if rebuilt.LastSequence != 2 || rebuilt.EventCount != 2 {
		t.Fatalf("unexpected rebuilt session state: %#v", rebuilt)
	}
}

func TestSessionCompactionPreservesCheckpointVerification(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "compact.journal.jsonl")
	now := time.Date(2026, time.February, 11, 10, 0, 0, 0, time.UTC)

	if _, err := StartSession(journalPath, SessionStartOptions{
		SessionID:       "sess_compact",
		RunID:           "run_compact",
		ProducerVersion: "test",
		Now:             now,
	}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
			CreatedAt: now.Add(time.Duration(i+1) * time.Second),
			IntentID:  "intent_pre_" + strconv.Itoa(i+1),
			ToolName:  "tool.write",
			Verdict:   "allow",
		}); err != nil {
			t.Fatalf("append pre-checkpoint event %d: %v", i+1, err)
		}
	}
	_, chainPath, err := SessionCheckpointAndWriteChain(journalPath, filepath.Join(workDir, "gait-out", "compact_cp_0001.zip"), SessionCheckpointOptions{
		Now:             now.Add(3 * time.Second),
		ProducerVersion: "test",
	})
	if err != nil {
		t.Fatalf("checkpoint 1: %v", err)
	}
	if _, err := AppendSessionEvent(journalPath, SessionAppendOptions{
		CreatedAt: now.Add(4 * time.Second),
		IntentID:  "intent_post_1",
		ToolName:  "tool.write",
		Verdict:   "allow",
	}); err != nil {
		t.Fatalf("append post-checkpoint event: %v", err)
	}

	dryRun, err := CompactSessionJournal(journalPath, SessionCompactionOptions{
		DryRun: true,
		Now:    now.Add(5 * time.Second),
	})
	if err != nil {
		t.Fatalf("dry-run compact: %v", err)
	}
	if !dryRun.Compacted || dryRun.EventsBefore != 3 || dryRun.EventsAfter != 1 {
		t.Fatalf("unexpected dry-run compaction result: %#v", dryRun)
	}

	applied, err := CompactSessionJournal(journalPath, SessionCompactionOptions{
		Now:             now.Add(6 * time.Second),
		ProducerVersion: "test",
	})
	if err != nil {
		t.Fatalf("apply compact: %v", err)
	}
	if !applied.Compacted || applied.EventsAfter != 1 {
		t.Fatalf("unexpected applied compaction result: %#v", applied)
	}

	journal, err := ReadSessionJournal(journalPath)
	if err != nil {
		t.Fatalf("read compacted journal: %v", err)
	}
	if len(journal.Checkpoints) != 1 || len(journal.Events) != 1 {
		t.Fatalf("unexpected compacted journal shape: checkpoints=%d events=%d", len(journal.Checkpoints), len(journal.Events))
	}
	if journal.Events[0].Sequence != 3 {
		t.Fatalf("expected retained event sequence 3, got %d", journal.Events[0].Sequence)
	}

	status, err := GetSessionStatus(journalPath)
	if err != nil {
		t.Fatalf("session status after compaction: %v", err)
	}
	if status.EventCount != 1 || status.LastSequence != 3 || status.CheckpointCount != 1 {
		t.Fatalf("unexpected status after compaction: %#v", status)
	}

	verifyBefore, err := VerifySessionChain(chainPath, SessionChainVerifyOptions{})
	if err != nil {
		t.Fatalf("verify checkpoint chain after compaction: %v", err)
	}
	if len(verifyBefore.LinkageErrors) != 0 || len(verifyBefore.CheckpointErrors) != 0 {
		t.Fatalf("unexpected chain verification issues after compaction: %#v", verifyBefore)
	}

	if _, _, err := SessionCheckpointAndWriteChain(journalPath, filepath.Join(workDir, "gait-out", "compact_cp_0002.zip"), SessionCheckpointOptions{
		Now:             now.Add(7 * time.Second),
		ProducerVersion: "test",
	}); err != nil {
		t.Fatalf("checkpoint 2 after compaction: %v", err)
	}
}

func TestWithSessionLockTimeoutIncludesDiagnosticsAndEnvOverrides(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "sessions", "timeout_diagnostics.journal.jsonl")
	lockPath := journalPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		t.Fatalf("create lock directory: %v", err)
	}
	freshCreatedAt := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(lockPath, []byte(`{"created_at":"`+freshCreatedAt+`"}`), 0o600); err != nil {
		t.Fatalf("write fresh lock: %v", err)
	}

	t.Setenv("GAIT_SESSION_LOCK_PROFILE", "swarm")
	t.Setenv("GAIT_SESSION_LOCK_TIMEOUT", "200ms")
	t.Setenv("GAIT_SESSION_LOCK_RETRY", "20ms")
	t.Setenv("GAIT_SESSION_LOCK_STALE_AFTER", "2m")

	start := time.Now()
	err := withSessionLock(journalPath, func() error { return nil })
	if err == nil {
		t.Fatalf("expected lock timeout")
	}
	var contentionErr SessionLockContentionError
	if !errors.As(err, &contentionErr) {
		t.Fatalf("expected SessionLockContentionError, got %T: %v", err, err)
	}
	if contentionErr.Profile != "swarm" {
		t.Fatalf("expected swarm profile, got %q", contentionErr.Profile)
	}
	if contentionErr.Timeout != 200*time.Millisecond || contentionErr.Retry != 20*time.Millisecond {
		t.Fatalf("unexpected timeout/retry in diagnostics: %#v", contentionErr)
	}
	if contentionErr.Attempts < 2 {
		t.Fatalf("expected multiple retry attempts, got %d", contentionErr.Attempts)
	}
	if time.Since(start) < 180*time.Millisecond {
		t.Fatalf("expected lock wait close to configured timeout")
	}
}

func TestSessionRelationshipBuildersNormalizeAndSort(t *testing.T) {
	eventRelationship := buildSessionEventRelationship(
		"sess_demo",
		"run_demo",
		"tool.write",
		"trace_demo",
		"gait.gate.policy",
		"1.0.0",
		strings.Repeat("b", 64),
		[]string{"rule_b", "rule_a", "rule_a"},
		"agent.exec",
		[]schemacommon.AgentLink{
			{Identity: "agent.b", Role: "requester"},
			{Identity: "agent.a", Role: "requester"},
			{Identity: "agent.ignore", Role: "approver"},
		},
	)
	if eventRelationship == nil {
		t.Fatalf("expected session event relationship")
	}
	if eventRelationship.ParentRef == nil || eventRelationship.ParentRef.Kind != "session" || eventRelationship.ParentRef.ID != "sess_demo" {
		t.Fatalf("unexpected session parent_ref: %#v", eventRelationship.ParentRef)
	}
	if eventRelationship.PolicyRef == nil || eventRelationship.PolicyRef.PolicyID != "gait.gate.policy" || eventRelationship.PolicyRef.PolicyVersion != "1.0.0" {
		t.Fatalf("expected policy lineage in relationship: %#v", eventRelationship.PolicyRef)
	}
	if got := eventRelationship.PolicyRef.MatchedRuleIDs; len(got) != 2 || got[0] != "rule_a" || got[1] != "rule_b" {
		t.Fatalf("expected matched rules to be deduplicated/sorted, got %#v", got)
	}
	if len(eventRelationship.AgentChain) != 2 || eventRelationship.AgentChain[0].Identity != "agent.a" {
		t.Fatalf("expected normalized agent chain ordering, got %#v", eventRelationship.AgentChain)
	}
	if len(eventRelationship.Edges) == 0 {
		t.Fatalf("expected relationship edges for session event")
	}
	foundCallsEdge := false
	for _, edge := range eventRelationship.Edges {
		if edge.Kind == "calls" && edge.From.Kind == "agent" && edge.From.ID == "agent.exec" {
			foundCallsEdge = true
			break
		}
	}
	if !foundCallsEdge {
		t.Fatalf("expected calls edge to use explicit actor identity, got %#v", eventRelationship.Edges)
	}

	checkpointRelationship := buildSessionCheckpointRelationship(
		"sess_demo",
		"run_demo",
		"run_demo_cp_0001",
		strings.Repeat("c", 64),
		strings.Repeat("d", 64),
	)
	if checkpointRelationship == nil {
		t.Fatalf("expected checkpoint relationship")
	}
	if checkpointRelationship.ParentRef == nil || checkpointRelationship.ParentRef.Kind != "session" {
		t.Fatalf("unexpected checkpoint parent_ref: %#v", checkpointRelationship.ParentRef)
	}
	if len(checkpointRelationship.EntityRefs) < 2 {
		t.Fatalf("expected checkpoint entity refs, got %#v", checkpointRelationship.EntityRefs)
	}

	timelineRelationship := buildRunTimelineRelationship(
		"session_event",
		"run_demo_cp_0001",
		"sess_demo",
		"trace_demo",
		"tool.write",
		strings.Repeat("e", 64),
	)
	if timelineRelationship == nil {
		t.Fatalf("expected timeline relationship")
	}
	if timelineRelationship.ParentRef == nil || timelineRelationship.ParentRef.Kind != "run" {
		t.Fatalf("unexpected timeline parent_ref: %#v", timelineRelationship.ParentRef)
	}
	if len(timelineRelationship.Edges) == 0 {
		t.Fatalf("expected timeline relationship edges")
	}

	checkpointTimelineRelationship := buildRunTimelineRelationship(
		"session_checkpoint_emitted",
		"run_demo_cp_0001",
		"checkpoint:1",
		"",
		"",
		"",
	)
	if checkpointTimelineRelationship == nil {
		t.Fatalf("expected checkpoint timeline relationship")
	}
	foundEvidenceEmission := false
	for _, edge := range checkpointTimelineRelationship.Edges {
		if edge.Kind == "emits_evidence" && edge.To.Kind == "evidence" && edge.To.ID == "checkpoint:1" {
			foundEvidenceEmission = true
			break
		}
	}
	if !foundEvidenceEmission {
		t.Fatalf("expected checkpoint emits_evidence edge to target checkpoint ref, got %#v", checkpointTimelineRelationship.Edges)
	}
}

func TestSelectRunpackEventActor(t *testing.T) {
	t.Run("explicit actor wins", func(t *testing.T) {
		got := selectRunpackEventActor("agent.explicit", []schemacommon.AgentLink{
			{Identity: "agent.delegate", Role: "delegate"},
			{Identity: "agent.requester", Role: "requester"},
		})
		if got != "agent.explicit" {
			t.Fatalf("expected explicit actor, got %q", got)
		}
	})

	t.Run("delegate role preferred", func(t *testing.T) {
		got := selectRunpackEventActor("", []schemacommon.AgentLink{
			{Identity: "agent.requester", Role: "requester"},
			{Identity: "agent.delegator", Role: "delegator"},
			{Identity: "agent.delegate", Role: "delegate"},
		})
		if got != "agent.delegate" {
			t.Fatalf("expected delegate actor, got %q", got)
		}
	})

	t.Run("returns empty for unusable chain", func(t *testing.T) {
		got := selectRunpackEventActor("", []schemacommon.AgentLink{
			{Identity: "", Role: "delegate"},
			{Identity: "agent.invalid", Role: "approver"},
		})
		if got != "" {
			t.Fatalf("expected empty actor, got %q", got)
		}
	})
}

func TestNormalizeRunpackRelationshipEnvelopeDropsInvalidEntries(t *testing.T) {
	invalid := &schemacommon.RelationshipEnvelope{
		ParentRef: &schemacommon.RelationshipNodeRef{Kind: "invalid", ID: "root"},
		EntityRefs: []schemacommon.RelationshipRef{
			{Kind: "invalid", ID: "x"},
		},
		AgentChain: []schemacommon.AgentLink{
			{Identity: "agent.demo", Role: "approver"},
		},
		Edges: []schemacommon.RelationshipEdge{
			{
				Kind: "invalid",
				From: schemacommon.RelationshipNodeRef{Kind: "tool", ID: "tool.write"},
				To:   schemacommon.RelationshipNodeRef{Kind: "policy", ID: strings.Repeat("f", 64)},
			},
		},
	}
	if normalized := normalizeRunpackRelationshipEnvelope(invalid); normalized != nil {
		t.Fatalf("expected invalid relationship envelope to collapse to nil, got %#v", normalized)
	}
}
