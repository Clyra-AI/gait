package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunJobLifecycleCommands(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_lifecycle"

	submitCode, submitOut := runJobJSON(t, []string{"submit", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if submitCode != exitOK {
		t.Fatalf("job submit expected %d got %d output=%#v", exitOK, submitCode, submitOut)
	}
	if submitOut.Job == nil || submitOut.Job.JobID != jobID {
		t.Fatalf("unexpected submit output: %#v", submitOut)
	}

	statusCode, statusOut := runJobJSON(t, []string{"status", "--id", jobID, "--root", root, "--json"})
	if statusCode != exitOK {
		t.Fatalf("job status expected %d got %d output=%#v", exitOK, statusCode, statusOut)
	}
	if statusOut.Job == nil || statusOut.Job.Status != "running" {
		t.Fatalf("unexpected status output: %#v", statusOut)
	}

	checkpointCode, checkpointOut := runJobJSON(t, []string{
		"checkpoint", "add",
		"--id", jobID,
		"--root", root,
		"--type", "progress",
		"--summary", "checkpoint summary",
		"--actor", "alice",
		"--json",
	})
	if checkpointCode != exitOK {
		t.Fatalf("checkpoint add expected %d got %d output=%#v", exitOK, checkpointCode, checkpointOut)
	}
	if checkpointOut.Checkpoint == nil || checkpointOut.Checkpoint.CheckpointID == "" {
		t.Fatalf("unexpected checkpoint add output: %#v", checkpointOut)
	}

	listCode, listOut := runJobJSON(t, []string{"checkpoint", "list", "--id", jobID, "--root", root, "--json"})
	if listCode != exitOK {
		t.Fatalf("checkpoint list expected %d got %d output=%#v", exitOK, listCode, listOut)
	}
	if len(listOut.Checkpoints) == 0 {
		t.Fatalf("expected checkpoint list entries")
	}

	showCode, showOut := runJobJSON(t, []string{
		"checkpoint", "show",
		"--id", jobID,
		"--root", root,
		"--checkpoint", listOut.Checkpoints[0].CheckpointID,
		"--json",
	})
	if showCode != exitOK {
		t.Fatalf("checkpoint show expected %d got %d output=%#v", exitOK, showCode, showOut)
	}
	if showOut.Checkpoint == nil || showOut.Checkpoint.CheckpointID != listOut.Checkpoints[0].CheckpointID {
		t.Fatalf("unexpected checkpoint show output: %#v", showOut)
	}

	pauseCode, pauseOut := runJobJSON(t, []string{"pause", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if pauseCode != exitOK {
		t.Fatalf("pause expected %d got %d output=%#v", exitOK, pauseCode, pauseOut)
	}
	if pauseOut.Job == nil || pauseOut.Job.Status != "paused" {
		t.Fatalf("unexpected pause output: %#v", pauseOut)
	}

	stopCode, stopOut := runJobJSON(t, []string{"stop", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if stopCode != exitOK {
		t.Fatalf("stop expected %d got %d output=%#v", exitOK, stopCode, stopOut)
	}
	if stopOut.Job == nil || stopOut.Job.Status != "emergency_stopped" || stopOut.Job.StatusReasonCode != "emergency_stop_preempted" {
		t.Fatalf("unexpected stop output: %#v", stopOut)
	}

	inspectCode, inspectOut := runJobJSON(t, []string{"inspect", "--id", jobID, "--root", root, "--json"})
	if inspectCode != exitOK {
		t.Fatalf("inspect expected %d got %d output=%#v", exitOK, inspectCode, inspectOut)
	}
	if inspectOut.Job == nil || len(inspectOut.Events) == 0 {
		t.Fatalf("unexpected inspect output: %#v", inspectOut)
	}

	cancelCode, cancelOut := runJobJSON(t, []string{"cancel", "--id", jobID, "--root", root, "--actor", "alice", "--json"})
	if cancelCode != exitOK {
		t.Fatalf("cancel expected %d got %d output=%#v", exitOK, cancelCode, cancelOut)
	}
	if cancelOut.Job == nil || cancelOut.Job.Status != "cancelled" {
		t.Fatalf("unexpected cancel output: %#v", cancelOut)
	}

	var textCode int
	textOut := captureStdout(t, func() {
		textCode = runJob([]string{"status", "--id", jobID, "--root", root})
	})
	if textCode != exitOK {
		t.Fatalf("text status expected %d got %d", exitOK, textCode)
	}
	if !strings.Contains(textOut, "job status:") {
		t.Fatalf("expected text status output, got %q", textOut)
	}
}

func TestRunJobHelpAndErrorPaths(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_errors"

	if code := runJob([]string{}); code != exitInvalidInput {
		t.Fatalf("job root usage expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("job unknown command expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint"}); code != exitInvalidInput {
		t.Fatalf("job checkpoint usage expected %d got %d", exitInvalidInput, code)
	}

	helpCases := [][]string{
		{"submit", "--help"},
		{"status", "--help"},
		{"checkpoint", "add", "--help"},
		{"checkpoint", "list", "--help"},
		{"checkpoint", "show", "--help"},
		{"pause", "--help"},
		{"stop", "--help"},
		{"approve", "--help"},
		{"resume", "--help"},
		{"cancel", "--help"},
		{"inspect", "--help"},
	}
	for _, args := range helpCases {
		if code := runJob(args); code != exitOK {
			t.Fatalf("help path %v expected %d got %d", args, exitOK, code)
		}
	}

	if code := runJob([]string{"submit", "--id", jobID, "--root", root, "--json"}); code != exitOK {
		t.Fatalf("submit expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"status", "--id", jobID, "--root", root, "extra", "--json"}); code != exitInvalidInput {
		t.Fatalf("status positional-arg validation expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint", "list", "--id", jobID, "--root", root, "extra", "--json"}); code != exitInvalidInput {
		t.Fatalf("checkpoint list positional-arg validation expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"checkpoint", "show", "--id", jobID, "--root", root, "--checkpoint", "missing", "--json"}); code != exitInvalidInput {
		t.Fatalf("checkpoint show missing checkpoint expected %d got %d", exitInvalidInput, code)
	}
	if code := runJob([]string{"pause", "--id", "missing", "--root", root, "--json"}); code != exitInternalFailure {
		t.Fatalf("pause missing job expected %d got %d", exitInternalFailure, code)
	}
	if code := runJob([]string{"cancel", "--id", "missing", "--root", root, "--json"}); code != exitInternalFailure {
		t.Fatalf("cancel missing job expected %d got %d", exitInternalFailure, code)
	}
	if code := runJob([]string{"inspect", "--id", "missing", "--root", root, "--json"}); code != exitInvalidInput {
		t.Fatalf("inspect missing job expected %d got %d", exitInvalidInput, code)
	}
}

func TestRunJobResumeReevaluatesPolicyAndIdentity(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_policy_resume"

	policyA := filepath.Join(workDir, "policy_a.yaml")
	policyB := filepath.Join(workDir, "policy_b.yaml")
	if err := os.WriteFile(policyA, []byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: block
rules:
  - name: allow_read
    effect: allow
    match:
      tool_names: ["tool.read"]
`), 0o600); err != nil {
		t.Fatalf("write policy A: %v", err)
	}
	if err := os.WriteFile(policyB, []byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: block
rules:
  - name: allow_read_then_write
    effect: allow
    match:
      tool_names: ["tool.read", "tool.write"]
`), 0o600); err != nil {
		t.Fatalf("write policy B: %v", err)
	}

	submitCode, submitOut := runJobJSON(t, []string{
		"submit",
		"--id", jobID,
		"--root", root,
		"--identity", "agent.alice",
		"--policy", policyA,
		"--json",
	})
	if submitCode != exitOK {
		t.Fatalf("submit expected %d got %d output=%#v", exitOK, submitCode, submitOut)
	}
	if submitOut.Job == nil || strings.TrimSpace(submitOut.Job.PolicyDigest) == "" {
		t.Fatalf("expected submit output to include policy digest: %#v", submitOut)
	}

	pauseCode, pauseOut := runJobJSON(t, []string{"pause", "--id", jobID, "--root", root, "--json"})
	if pauseCode != exitOK || pauseOut.Job == nil || pauseOut.Job.Status != "paused" {
		t.Fatalf("pause before resume expected paused: code=%d output=%#v", pauseCode, pauseOut)
	}

	missingPolicyCode, missingPolicyOut := runJobJSON(t, []string{"resume", "--id", jobID, "--root", root, "--json"})
	if missingPolicyCode != exitInvalidInput {
		t.Fatalf("resume without policy expected %d got %d output=%#v", exitInvalidInput, missingPolicyCode, missingPolicyOut)
	}
	if !strings.Contains(missingPolicyOut.Error, "policy evaluation required") {
		t.Fatalf("expected policy evaluation required error, got %#v", missingPolicyOut)
	}

	resumeCode, resumeOut := runJobJSON(t, []string{
		"resume",
		"--id", jobID,
		"--root", root,
		"--policy", policyB,
		"--identity-validation-source", "revocation_list",
		"--json",
	})
	if resumeCode != exitOK {
		t.Fatalf("resume expected %d got %d output=%#v", exitOK, resumeCode, resumeOut)
	}
	if resumeOut.Job == nil || resumeOut.Job.StatusReasonCode != "resumed_with_policy_transition" {
		t.Fatalf("expected resumed_with_policy_transition, got %#v", resumeOut)
	}
	if submitOut.Job.PolicyDigest == resumeOut.Job.PolicyDigest {
		t.Fatalf("expected policy digest transition on resume: submit=%s resume=%s", submitOut.Job.PolicyDigest, resumeOut.Job.PolicyDigest)
	}

	pauseAgainCode, pauseAgainOut := runJobJSON(t, []string{"pause", "--id", jobID, "--root", root, "--json"})
	if pauseAgainCode != exitOK || pauseAgainOut.Job == nil || pauseAgainOut.Job.Status != "paused" {
		t.Fatalf("pause before revoked identity check expected paused: code=%d output=%#v", pauseAgainCode, pauseAgainOut)
	}
	mismatchCode, mismatchOut := runJobJSON(t, []string{
		"resume",
		"--id", jobID,
		"--root", root,
		"--policy", policyB,
		"--identity", "agent.bob",
		"--json",
	})
	if mismatchCode != exitInvalidInput {
		t.Fatalf("resume with identity mismatch expected %d got %d output=%#v", exitInvalidInput, mismatchCode, mismatchOut)
	}
	if !strings.Contains(mismatchOut.Error, "identity binding mismatch") {
		t.Fatalf("expected identity binding mismatch error, got %#v", mismatchOut)
	}

	revocationsPath := filepath.Join(workDir, "revoked_identities.txt")
	if err := os.WriteFile(revocationsPath, []byte("agent.alice\n"), 0o600); err != nil {
		t.Fatalf("write revoked identities: %v", err)
	}
	revokedCode, revokedOut := runJobJSON(t, []string{
		"resume",
		"--id", jobID,
		"--root", root,
		"--policy", policyB,
		"--identity-revocations", revocationsPath,
		"--json",
	})
	if revokedCode != exitInvalidInput {
		t.Fatalf("resume revoked identity expected %d got %d output=%#v", exitInvalidInput, revokedCode, revokedOut)
	}
	if !strings.Contains(revokedOut.Error, "identity revoked") {
		t.Fatalf("expected identity revoked error, got %#v", revokedOut)
	}
}

func TestRunJobSubmitRejectsPolicyDigestMismatch(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: allow-read
    effect: allow
    match:
      tool_names: [tool.read]
`), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	code, out := runJobJSON(t, []string{
		"submit",
		"--id", "job_digest_mismatch",
		"--root", root,
		"--policy", policyPath,
		"--policy-digest", "sha256:deadbeef",
		"--json",
	})
	if code != exitInvalidInput {
		t.Fatalf("submit mismatch expected %d got %d output=%#v", exitInvalidInput, code, out)
	}
	if !strings.Contains(out.Error, "policy digest mismatch") {
		t.Fatalf("expected policy digest mismatch error, got %#v", out)
	}
}

func TestLoadRevokedIdentitiesFormats(t *testing.T) {
	workDir := t.TempDir()

	arrayPath := filepath.Join(workDir, "revoked_array.json")
	if err := os.WriteFile(arrayPath, []byte(`["agent.a","agent.b"]`), 0o600); err != nil {
		t.Fatalf("write array revocations: %v", err)
	}
	revoked, err := loadRevokedIdentities(arrayPath)
	if err != nil {
		t.Fatalf("load array revocations: %v", err)
	}
	expected := map[string]struct{}{"agent.a": {}, "agent.b": {}}
	if !reflect.DeepEqual(revoked, expected) {
		t.Fatalf("unexpected array revocations: %#v", revoked)
	}

	objectPath := filepath.Join(workDir, "revoked_object.json")
	if err := os.WriteFile(objectPath, []byte(`{"revoked_identities":["agent.c"],"identities":["agent.d"]}`), 0o600); err != nil {
		t.Fatalf("write object revocations: %v", err)
	}
	revoked, err = loadRevokedIdentities(objectPath)
	if err != nil {
		t.Fatalf("load object revocations: %v", err)
	}
	expected = map[string]struct{}{"agent.c": {}, "agent.d": {}}
	if !reflect.DeepEqual(revoked, expected) {
		t.Fatalf("unexpected object revocations: %#v", revoked)
	}

	linesPath := filepath.Join(workDir, "revoked_lines.txt")
	if err := os.WriteFile(linesPath, []byte("# comment\nagent.e\n\nagent.f\n"), 0o600); err != nil {
		t.Fatalf("write line revocations: %v", err)
	}
	revoked, err = loadRevokedIdentities(linesPath)
	if err != nil {
		t.Fatalf("load line revocations: %v", err)
	}
	expected = map[string]struct{}{"agent.e": {}, "agent.f": {}}
	if !reflect.DeepEqual(revoked, expected) {
		t.Fatalf("unexpected line revocations: %#v", revoked)
	}
}

func TestLoadRevokedIdentitiesErrorsAndIdentityLookup(t *testing.T) {
	if _, err := loadRevokedIdentities("."); err == nil {
		t.Fatalf("expected path validation error")
	}
	if _, err := loadRevokedIdentities(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatalf("expected read error for missing file")
	}

	revoked := map[string]struct{}{"agent.revoked": {}}
	if !identityIsRevoked(revoked, "agent.revoked") {
		t.Fatalf("expected identityIsRevoked positive match")
	}
	if identityIsRevoked(revoked, "agent.ok") {
		t.Fatalf("expected identityIsRevoked negative match")
	}
	if identityIsRevoked(nil, "agent.revoked") {
		t.Fatalf("expected empty revoked set to return false")
	}
}

func TestResolveJobPolicyMetadata(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: allow-read
    effect: allow
    match:
      tool_names: [tool.read]
`), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	digest, ref, err := resolveJobPolicyMetadata(policyPath, "", "")
	if err != nil {
		t.Fatalf("resolve policy metadata: %v", err)
	}
	if strings.TrimSpace(digest) == "" {
		t.Fatalf("expected computed policy digest")
	}
	if ref != filepath.Clean(policyPath) {
		t.Fatalf("expected default policy ref to be cleaned path, got %q", ref)
	}

	digest2, ref2, err := resolveJobPolicyMetadata("", "sha256:abc", "ref-1")
	if err != nil {
		t.Fatalf("resolve metadata without policy path: %v", err)
	}
	if digest2 != "sha256:abc" || ref2 != "ref-1" {
		t.Fatalf("expected passthrough policy metadata, got digest=%q ref=%q", digest2, ref2)
	}
}

func TestRunJobResumeWithIdentityRevokedFlag(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	root := filepath.Join(workDir, "jobs")
	jobID := "job_cli_revoked_flag"

	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: allow-read
    effect: allow
    match:
      tool_names: [tool.read]
`), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	if code := runJob([]string{
		"submit",
		"--id", jobID,
		"--root", root,
		"--identity", "agent.revoked",
		"--policy", policyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("submit expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"pause", "--id", jobID, "--root", root, "--json"}); code != exitOK {
		t.Fatalf("pause expected %d got %d", exitOK, code)
	}

	code, out := runJobJSON(t, []string{
		"resume",
		"--id", jobID,
		"--root", root,
		"--policy", policyPath,
		"--identity", "agent.revoked",
		"--identity-revoked",
		"--json",
	})
	if code != exitInvalidInput {
		t.Fatalf("resume with identity revoked flag expected %d got %d output=%#v", exitInvalidInput, code, out)
	}
	if !strings.Contains(out.Error, "identity revoked") {
		t.Fatalf("expected identity revoked error, got %#v", out)
	}
}

func runJobJSON(t *testing.T, args []string) (int, jobOutput) {
	t.Helper()
	var code int
	raw := captureStdout(t, func() {
		code = runJob(args)
	})
	var output jobOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode job output: %v raw=%q", err, raw)
	}
	return code, output
}
