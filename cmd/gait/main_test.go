package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/doctor"
	gatecore "github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/jcs"
	"github.com/davidahmann/gait/core/runpack"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	schemascout "github.com/davidahmann/gait/core/schema/v1/scout"
	"github.com/davidahmann/gait/core/sign"
)

func TestRunDispatch(t *testing.T) {
	if code := run([]string{"gait"}); code != exitOK {
		t.Fatalf("run without args: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "--explain"}); code != exitOK {
		t.Fatalf("run explain: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "version"}); code != exitOK {
		t.Fatalf("run version: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "unknown"}); code != exitInvalidInput {
		t.Fatalf("run unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := run([]string{"gait", "approve", "--help"}); code != exitOK {
		t.Fatalf("run approve help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "delegate", "mint", "--help"}); code != exitOK {
		t.Fatalf("run delegate mint help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "delegate", "verify", "--help"}); code != exitOK {
		t.Fatalf("run delegate verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "gate", "eval", "--help"}); code != exitOK {
		t.Fatalf("run gate help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "policy", "test", "--help"}); code != exitOK {
		t.Fatalf("run policy help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "policy", "validate", "--help"}); code != exitOK {
		t.Fatalf("run policy validate help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "policy", "fmt", "--help"}); code != exitOK {
		t.Fatalf("run policy fmt help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "policy", "simulate", "--help"}); code != exitOK {
		t.Fatalf("run policy simulate help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "keys", "--help"}); code != exitOK {
		t.Fatalf("run keys help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "keys", "init", "--help"}); code != exitOK {
		t.Fatalf("run keys init help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "keys", "rotate", "--help"}); code != exitOK {
		t.Fatalf("run keys rotate help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "keys", "verify", "--help"}); code != exitOK {
		t.Fatalf("run keys verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "trace", "verify", "--help"}); code != exitOK {
		t.Fatalf("run trace help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "regress", "init", "--help"}); code != exitOK {
		t.Fatalf("run regress init help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "regress", "run", "--help"}); code != exitOK {
		t.Fatalf("run regress run help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "regress", "bootstrap", "--help"}); code != exitOK {
		t.Fatalf("run regress bootstrap help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "replay", "--help"}); code != exitOK {
		t.Fatalf("run replay help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "diff", "--help"}); code != exitOK {
		t.Fatalf("run diff help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "record", "--help"}); code != exitOK {
		t.Fatalf("run record help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "inspect", "--help"}); code != exitOK {
		t.Fatalf("run inspect help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "reduce", "--help"}); code != exitOK {
		t.Fatalf("run reduce help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "run", "receipt", "--help"}); code != exitOK {
		t.Fatalf("run receipt help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "job", "submit", "--help"}); code != exitOK {
		t.Fatalf("run job submit help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "pack", "build", "--help"}); code != exitOK {
		t.Fatalf("run pack build help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "pack", "verify", "--help"}); code != exitOK {
		t.Fatalf("run pack verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "pack", "inspect", "--help"}); code != exitOK {
		t.Fatalf("run pack inspect help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "pack", "diff", "--help"}); code != exitOK {
		t.Fatalf("run pack diff help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "report", "top", "--help"}); code != exitOK {
		t.Fatalf("run report top help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "scout", "snapshot", "--help"}); code != exitOK {
		t.Fatalf("run scout snapshot help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "scout", "diff", "--help"}); code != exitOK {
		t.Fatalf("run scout diff help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "scout", "signal", "--help"}); code != exitOK {
		t.Fatalf("run scout signal help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "guard", "pack", "--help"}); code != exitOK {
		t.Fatalf("run guard pack help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "guard", "verify", "--help"}); code != exitOK {
		t.Fatalf("run guard verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "guard", "retain", "--help"}); code != exitOK {
		t.Fatalf("run guard retain help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "guard", "encrypt", "--help"}); code != exitOK {
		t.Fatalf("run guard encrypt help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "guard", "decrypt", "--help"}); code != exitOK {
		t.Fatalf("run guard decrypt help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "incident", "pack", "--help"}); code != exitOK {
		t.Fatalf("run incident pack help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "registry", "install", "--help"}); code != exitOK {
		t.Fatalf("run registry install help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "registry", "list", "--help"}); code != exitOK {
		t.Fatalf("run registry list help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "registry", "verify", "--help"}); code != exitOK {
		t.Fatalf("run registry verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "migrate", "--help"}); code != exitOK {
		t.Fatalf("run migrate help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "mcp", "proxy", "--help"}); code != exitOK {
		t.Fatalf("run mcp proxy help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "mcp", "bridge", "--help"}); code != exitOK {
		t.Fatalf("run mcp bridge help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "verify", "--help"}); code != exitOK {
		t.Fatalf("run verify help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "verify", "chain", "--help"}); code != exitOK {
		t.Fatalf("run verify chain help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "verify", "session-chain", "--help"}); code != exitOK {
		t.Fatalf("run verify session-chain help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "doctor", "--help"}); code != exitOK {
		t.Fatalf("run doctor help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "doctor", "adoption", "--help"}); code != exitOK {
		t.Fatalf("run doctor adoption help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "tour", "--help"}); code != exitOK {
		t.Fatalf("run tour help: expected %d got %d", exitOK, code)
	}
	if code := run([]string{"gait", "ui", "--help"}); code != exitOK {
		t.Fatalf("run ui help: expected %d got %d", exitOK, code)
	}
}

func TestMainEntrypoint(t *testing.T) {
	if os.Getenv("GAIT_TEST_MAIN") == "1" {
		os.Args = []string{"gait", "version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainEntrypoint")
	cmd.Env = append(os.Environ(), "GAIT_TEST_MAIN=1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run child process: %v", err)
	}
}

func TestRunpackPathHelpers(t *testing.T) {
	workDir := t.TempDir()
	runpackDir := filepath.Join(workDir, "gait-out")
	if err := os.MkdirAll(runpackDir, 0o750); err != nil {
		t.Fatalf("mkdir runpack dir: %v", err)
	}
	runpackPath := filepath.Join(runpackDir, "runpack_run_demo.zip")
	if err := os.WriteFile(runpackPath, []byte("zip"), 0o600); err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	withWorkingDir(t, workDir)
	resolved, err := resolveRunpackPath("run_demo")
	if err != nil {
		t.Fatalf("resolve run id: %v", err)
	}
	if resolved != filepath.Join(".", "gait-out", "runpack_run_demo.zip") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}

	resolved, err = resolveRunpackPath(runpackPath)
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if resolved != runpackPath {
		t.Fatalf("unexpected resolved runpack path: %s", resolved)
	}

	if !looksLikePath("a/b") {
		t.Fatalf("expected path with separator to match")
	}
	if !looksLikePath("thing.zip") {
		t.Fatalf("expected .zip to match")
	}
	if looksLikePath("run_demo") {
		t.Fatalf("unexpected path detection")
	}
	if !fileExists(runpackPath) {
		t.Fatalf("expected file to exist")
	}
	if fileExists(runpackDir) {
		t.Fatalf("directory should not be treated as file")
	}
}

func TestDemoVerifyReplayDiffAndRegressFlow(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo: expected %d got %d", exitOK, code)
	}

	if code := runVerify([]string{"--json", "run_demo"}); code != exitOK {
		t.Fatalf("verify: expected %d got %d", exitOK, code)
	}
	if code := runReplay([]string{"--json", "run_demo"}); code != exitOK {
		t.Fatalf("replay: expected %d got %d", exitOK, code)
	}
	if code := runInspect([]string{"--json", "--from", "run_demo"}); code != exitOK {
		t.Fatalf("run inspect: expected %d got %d", exitOK, code)
	}
	if code := runReplay([]string{"--real-tools", "run_demo"}); code != exitUnsafeReplay {
		t.Fatalf("unsafe replay guard: expected %d got %d", exitUnsafeReplay, code)
	}

	diffPath := filepath.Join(workDir, "diff.json")
	if code := runDiff([]string{"--json", "--output", diffPath, "run_demo", "run_demo"}); code != exitOK {
		t.Fatalf("diff: expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(diffPath); err != nil {
		t.Fatalf("diff output: %v", err)
	}

	if code := runRegressInit([]string{"--from", "run_demo", "--json"}); code != exitOK {
		t.Fatalf("regress init: expected %d got %d", exitOK, code)
	}
	if code := runRegressRun([]string{"--json", "--junit", "junit.xml"}); code != exitOK {
		t.Fatalf("regress run pass: expected %d got %d", exitOK, code)
	}
	if code := runReceipt([]string{"--json", "--from", "run_demo"}); code != exitOK {
		t.Fatalf("run receipt pass: expected %d got %d", exitOK, code)
	}

	fixturePath := filepath.Join(workDir, "fixtures", "run_demo", "fixture.json")
	rawFixture, err := os.ReadFile(fixturePath) // #nosec G304
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(rawFixture, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	fixture["expected_replay_exit_code"] = float64(exitVerifyFailed)
	encodedFixture, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(fixturePath, append(encodedFixture, '\n'), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if code := runRegressRun([]string{"--json"}); code != exitRegressFailed {
		t.Fatalf("regress run fail: expected %d got %d", exitRegressFailed, code)
	}
}

func TestJobAndPackFlow(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("demo: expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"submit", "--id", "job_test", "--root", filepath.Join(workDir, "jobs"), "--json"}); code != exitOK {
		t.Fatalf("job submit: expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"checkpoint", "add", "--id", "job_test", "--root", filepath.Join(workDir, "jobs"), "--type", "decision-needed", "--summary", "need approval", "--required-action", "approve", "--json"}); code != exitOK {
		t.Fatalf("job checkpoint add: expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"approve", "--id", "job_test", "--root", filepath.Join(workDir, "jobs"), "--actor", "alice", "--json"}); code != exitOK {
		t.Fatalf("job approve: expected %d got %d", exitOK, code)
	}
	if code := runJob([]string{"resume", "--id", "job_test", "--root", filepath.Join(workDir, "jobs"), "--allow-env-mismatch", "--env-fingerprint", "envfp:override", "--json"}); code != exitOK {
		t.Fatalf("job resume: expected %d got %d", exitOK, code)
	}

	runPackPath := filepath.Join(workDir, "run_pack.zip")
	jobPackPath := filepath.Join(workDir, "job_pack.zip")

	if code := runPack([]string{"build", "--type", "run", "--from", "run_demo", "--out", runPackPath, "--json"}); code != exitOK {
		t.Fatalf("pack build run: expected %d got %d", exitOK, code)
	}
	if code := runPack([]string{"build", "--type", "job", "--from", "job_test", "--job-root", filepath.Join(workDir, "jobs"), "--out", jobPackPath, "--json"}); code != exitOK {
		t.Fatalf("pack build job: expected %d got %d", exitOK, code)
	}
	if code := runPack([]string{"verify", runPackPath, "--json"}); code != exitOK {
		t.Fatalf("pack verify run: expected %d got %d", exitOK, code)
	}
	if code := runPack([]string{"inspect", jobPackPath, "--json"}); code != exitOK {
		t.Fatalf("pack inspect job: expected %d got %d", exitOK, code)
	}
	code := runPack([]string{"diff", runPackPath, jobPackPath, "--json"})
	if code != exitOK && code != exitVerifyFailed {
		t.Fatalf("pack diff: expected %d or %d got %d", exitOK, exitVerifyFailed, code)
	}
}

func TestGatePolicyTraceApproveAndDoctor(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")

	policyAllowPath := filepath.Join(workDir, "policy_allow.yaml")
	mustWriteFile(t, policyAllowPath, strings.Join([]string{
		"schema_id: gait.gate.policy",
		"schema_version: 1.0.0",
		"default_verdict: allow",
		"rules:",
		"  - name: allow-write",
		"    priority: 1",
		"    effect: allow",
		"    match:",
		"      tool_names: [tool.write]",
		"      target_kinds: [path]",
		"    reason_codes: [allow_rule]",
	}, "\n")+"\n")

	tracePath := filepath.Join(workDir, "trace_allow.json")
	if code := runGateEval([]string{
		"--policy", policyAllowPath,
		"--intent", intentPath,
		"--trace-out", tracePath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("gate eval allow: expected %d got %d", exitOK, code)
	}
	if code := runTraceVerify([]string{"--path", tracePath, "--private-key", privateKeyPath, "--json"}); code != exitOK {
		t.Fatalf("trace verify: expected %d got %d", exitOK, code)
	}

	if code := runPolicyTest([]string{"--json", policyAllowPath, intentPath}); code != exitOK {
		t.Fatalf("policy test allow: expected %d got %d", exitOK, code)
	}

	policyBlockPath := filepath.Join(workDir, "policy_block.yaml")
	mustWriteFile(t, policyBlockPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: block-write",
		"    priority: 1",
		"    effect: block",
		"    match:",
		"      tool_names: [tool.write]",
		"    reason_codes: [blocked_write]",
	}, "\n")+"\n")
	if code := runPolicyTest([]string{policyBlockPath, intentPath, "--json"}); code != exitPolicyBlocked {
		t.Fatalf("policy test block: expected %d got %d", exitPolicyBlocked, code)
	}

	policyApprovalPath := filepath.Join(workDir, "policy_approval.yaml")
	mustWriteFile(t, policyApprovalPath, "default_verdict: require_approval\n")
	if code := runGateEval([]string{
		"--policy", policyApprovalPath,
		"--intent", intentPath,
		"--trace-out", filepath.Join(workDir, "trace_approval.json"),
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitApprovalRequired {
		t.Fatalf("gate eval approval: expected %d got %d", exitApprovalRequired, code)
	}

	tokenPath := filepath.Join(workDir, "approval_token.json")
	if code := runApprove([]string{
		"--intent-digest", strings.Repeat("a", 64),
		"--policy-digest", strings.Repeat("b", 64),
		"--ttl", "1h",
		"--scope", "tool:tool.write",
		"--approver", "alice",
		"--reason-code", "change-ticket",
		"--out", tokenPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("approve: expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("approval token path: %v", err)
	}
	if code := runApprove([]string{"--ttl", "bad", "--json"}); code != exitInvalidInput {
		t.Fatalf("approve invalid: expected %d got %d", exitInvalidInput, code)
	}

	repoRoot := repoRootFromPackageDir(t)
	if code := runDoctor([]string{
		"--workdir", repoRoot,
		"--output-dir", filepath.Join(workDir, "gait-out"),
		"--json",
	}); code != exitOK {
		t.Fatalf("doctor valid: expected %d got %d", exitOK, code)
	}
	if code := runDoctor([]string{
		"--workdir", workDir,
		"--output-dir", filepath.Join(workDir, "gait-out"),
		"--json",
	}); code != exitMissingDependency {
		t.Fatalf("doctor missing schemas: expected %d got %d", exitMissingDependency, code)
	}
	if code := runDoctor([]string{
		"--workdir", workDir,
		"--output-dir", filepath.Join(workDir, "gait-out"),
		"--production-readiness",
		"--json",
	}); code != exitVerifyFailed {
		t.Fatalf("doctor production readiness failure: expected %d got %d", exitVerifyFailed, code)
	}
}

func TestPolicySimulateCommand(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	baselinePolicyPath := filepath.Join(workDir, "policy_baseline.yaml")
	mustWriteFile(t, baselinePolicyPath, "default_verdict: allow\n")

	candidatePolicyPath := filepath.Join(workDir, "policy_candidate.yaml")
	mustWriteFile(t, candidatePolicyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: block-write",
		"    priority: 1",
		"    effect: block",
		"    match:",
		"      tool_names: [tool.write]",
		"    reason_codes: [blocked_write]",
	}, "\n")+"\n")

	fixtureDir := filepath.Join(workDir, "fixtures")
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	writeIntentFixture(t, filepath.Join(fixtureDir, "intent_write.json"), "tool.write")
	writeIntentFixture(t, filepath.Join(fixtureDir, "intent_read.json"), "tool.read")

	var code int
	raw := captureStdout(t, func() {
		code = runPolicySimulate([]string{
			"--baseline", baselinePolicyPath,
			"--policy", candidatePolicyPath,
			"--fixtures", fixtureDir,
			"--json",
		})
	})
	if code != exitOK {
		t.Fatalf("runPolicySimulate expected %d got %d", exitOK, code)
	}

	var output policySimulateOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode policy simulate output: %v (%s)", err, raw)
	}
	if !output.OK {
		t.Fatalf("expected successful policy simulate output: %#v", output)
	}
	if output.FixturesTotal != 2 {
		t.Fatalf("expected fixtures_total=2 got %d", output.FixturesTotal)
	}
	if output.ChangedFixtures != 1 {
		t.Fatalf("expected changed_fixtures=1 got %d", output.ChangedFixtures)
	}
	if output.Recommendation != "require_approval" {
		t.Fatalf("unexpected recommendation: %s", output.Recommendation)
	}
	if len(output.Changed) != 1 {
		t.Fatalf("expected one changed fixture entry got %#v", output.Changed)
	}
	if output.Changed[0].FixturePath != filepath.Join(fixtureDir, "intent_write.json") {
		t.Fatalf("unexpected changed fixture path: %s", output.Changed[0].FixturePath)
	}
}

func TestKeysLifecycleCommands(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runKeys(nil); code != exitInvalidInput {
		t.Fatalf("runKeys without args expected %d got %d", exitInvalidInput, code)
	}
	if code := runKeys([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runKeys unknown subcommand expected %d got %d", exitInvalidInput, code)
	}
	if code := runKeys([]string{"--explain"}); code != exitOK {
		t.Fatalf("runKeys explain expected %d got %d", exitOK, code)
	}

	keysDir := filepath.Join(workDir, "keys")
	var initCode int
	initRaw := captureStdout(t, func() {
		initCode = runKeys([]string{
			"init",
			"--out-dir", keysDir,
			"--prefix", "gaittest",
			"--json",
		})
	})
	if initCode != exitOK {
		t.Fatalf("runKeys init expected %d got %d", exitOK, initCode)
	}
	var initOutput keysInitOutput
	if err := json.Unmarshal([]byte(initRaw), &initOutput); err != nil {
		t.Fatalf("decode keys init output: %v (%s)", err, initRaw)
	}
	if !initOutput.OK {
		t.Fatalf("expected keys init ok output: %#v", initOutput)
	}
	if _, err := os.Stat(initOutput.PrivateKeyPath); err != nil {
		t.Fatalf("expected private key file: %v", err)
	}
	if _, err := os.Stat(initOutput.PublicKeyPath); err != nil {
		t.Fatalf("expected public key file: %v", err)
	}

	var verifyCode int
	verifyRaw := captureStdout(t, func() {
		verifyCode = runKeys([]string{
			"verify",
			"--private-key", initOutput.PrivateKeyPath,
			"--public-key", initOutput.PublicKeyPath,
			"--json",
		})
	})
	if verifyCode != exitOK {
		t.Fatalf("runKeys verify expected %d got %d", exitOK, verifyCode)
	}
	var verifyOutput keysVerifyOutput
	if err := json.Unmarshal([]byte(verifyRaw), &verifyOutput); err != nil {
		t.Fatalf("decode keys verify output: %v (%s)", err, verifyRaw)
	}
	if !verifyOutput.OK {
		t.Fatalf("expected keys verify ok output: %#v", verifyOutput)
	}
	if verifyOutput.KeyID == "" {
		t.Fatalf("expected key id in keys verify output")
	}

	if code := runKeys([]string{"verify", "--json"}); code != exitInvalidInput {
		t.Fatalf("runKeys verify missing private key expected %d got %d", exitInvalidInput, code)
	}

	var rotateCode int
	rotateRaw := captureStdout(t, func() {
		rotateCode = runKeys([]string{
			"rotate",
			"--out-dir", keysDir,
			"--prefix", "gaittest",
			"--json",
		})
	})
	if rotateCode != exitOK {
		t.Fatalf("runKeys rotate expected %d got %d", exitOK, rotateCode)
	}
	var rotateOutput keysInitOutput
	if err := json.Unmarshal([]byte(rotateRaw), &rotateOutput); err != nil {
		t.Fatalf("decode keys rotate output: %v (%s)", err, rotateRaw)
	}
	if !rotateOutput.OK {
		t.Fatalf("expected keys rotate ok output: %#v", rotateOutput)
	}
	if !strings.HasPrefix(rotateOutput.Prefix, "gaittest_") {
		t.Fatalf("expected rotated prefix with timestamp, got %s", rotateOutput.Prefix)
	}

	if _, err := createSigningKeypair(keysDir, "gaittest", false); err == nil {
		t.Fatalf("expected createSigningKeypair duplicate paths error")
	}
	if _, err := createSigningKeypair("", "x", false); err == nil {
		t.Fatalf("expected createSigningKeypair out-dir error")
	}
	if _, err := createSigningKeypair(keysDir, "", false); err == nil {
		t.Fatalf("expected createSigningKeypair prefix error")
	}
	if _, err := createSigningKeypair(keysDir, "gaittest", true); err != nil {
		t.Fatalf("expected createSigningKeypair force overwrite success: %v", err)
	}

	if got := keySourceLabel(initOutput.PublicKeyPath, ""); !strings.HasPrefix(got, "path:") {
		t.Fatalf("expected path key source label, got %s", got)
	}
	if got := keySourceLabel("", "GAIT_KEY_ENV"); got != "env:GAIT_KEY_ENV" {
		t.Fatalf("expected env key source label, got %s", got)
	}
	if got := keySourceLabel("", ""); got != "derived" {
		t.Fatalf("expected derived key source label, got %s", got)
	}
}

func TestRunRecordAndMigrateFlow(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	run, intents, results, refs, err := buildDemoRunpack()
	if err != nil {
		t.Fatalf("build demo runpack: %v", err)
	}

	inputPath := filepath.Join(workDir, "record_input.json")
	payload := runRecordInput{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal record input: %v", err)
	}
	if err := os.WriteFile(inputPath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatalf("write record input: %v", err)
	}

	if code := runRecord([]string{
		"--input", inputPath,
		"--out-dir", "./gait-out",
		"--run-id", "run_recorded",
		"--json",
	}); code != exitOK {
		t.Fatalf("run record: expected %d got %d", exitOK, code)
	}

	recordedPath := filepath.Join(workDir, "gait-out", "runpack_run_recorded.zip")
	if _, err := os.Stat(recordedPath); err != nil {
		t.Fatalf("recorded runpack missing: %v", err)
	}
	if code := runVerify([]string{"--json", recordedPath}); code != exitOK {
		t.Fatalf("verify recorded path: expected %d got %d", exitOK, code)
	}

	if code := runMigrate([]string{"--input", recordedPath, "--json"}); code != exitOK {
		t.Fatalf("migrate runpack: expected %d got %d", exitOK, code)
	}

	migratedPath := filepath.Join(workDir, "gait-out", "runpack_run_recorded_migrated.zip")
	if _, err := os.Stat(migratedPath); err != nil {
		t.Fatalf("migrated runpack missing: %v", err)
	}
	if code := runVerify([]string{"--json", migratedPath}); code != exitOK {
		t.Fatalf("verify migrated path: expected %d got %d", exitOK, code)
	}
}

func TestScoutGuardRegistryAndReduceFlow(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	pythonPath := filepath.Join(workDir, "agent.py")
	mustWriteFile(t, pythonPath, strings.Join([]string{
		"from langchain.tools import tool",
		"@tool",
		"def delete_user():",
		"    return \"ok\"",
	}, "\n")+"\n")
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: block",
		"rules:",
		"  - name: allow-delete",
		"    priority: 1",
		"    effect: allow",
		"    match:",
		"      tool_names: [delete_user]",
	}, "\n")+"\n")
	snapshotPath := filepath.Join(workDir, "inventory_snapshot.json")
	coveragePath := filepath.Join(workDir, "inventory_coverage.json")
	if code := runScoutSnapshot([]string{
		"--roots", workDir,
		"--policy", policyPath,
		"--out", snapshotPath,
		"--coverage-out", coveragePath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runScoutSnapshot: expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("snapshot output missing: %v", err)
	}
	if _, err := os.Stat(coveragePath); err != nil {
		t.Fatalf("coverage output missing: %v", err)
	}

	runpackPath := filepath.Join(workDir, "runpack_run_reduce_flow.zip")
	_, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run: schemarunpack.Run{
			RunID:           "run_reduce_flow",
			CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			ProducerVersion: "0.0.0-dev",
		},
		Intents: []schemarunpack.IntentRecord{{
			IntentID:   "intent_missing",
			RunID:      "run_reduce_flow",
			ToolName:   "tool.delete",
			ArgsDigest: strings.Repeat("a", 64),
		}},
		Results: nil,
		Refs:    schemarunpack.Refs{RunID: "run_reduce_flow"},
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}
	reducedPath := filepath.Join(workDir, "reduced.zip")
	reportPath := filepath.Join(workDir, "reduce_report.json")
	if code := runReduce([]string{
		"--from", runpackPath,
		"--predicate", "missing_result",
		"--out", reducedPath,
		"--report-out", reportPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runReduce: expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(reducedPath); err != nil {
		t.Fatalf("reduced runpack missing: %v", err)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("reduce report missing: %v", err)
	}

	packPath := filepath.Join(workDir, "evidence_pack.zip")
	if code := runGuardPack([]string{
		"--run", runpackPath,
		"--inventory", snapshotPath,
		"--out", packPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardPack: expected %d got %d", exitOK, code)
	}
	if code := runGuardVerify([]string{packPath, "--json"}); code != exitOK {
		t.Fatalf("runGuardVerify: expected %d got %d", exitOK, code)
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	registryManifestPath := filepath.Join(workDir, "registry_pack.json")
	manifest := map[string]any{
		"schema_id":        "gait.registry.pack",
		"schema_version":   "1.0.0",
		"created_at":       "2026-01-01T00:00:00Z",
		"producer_version": "0.0.0-dev",
		"pack_name":        "baseline-highrisk",
		"pack_version":     "1.1.0",
		"artifacts": []map[string]string{
			{"path": "policy.yaml", "sha256": strings.Repeat("a", 64)},
		},
	}
	signableRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal signable manifest: %v", err)
	}
	digest, err := jcs.DigestJCS(signableRaw)
	if err != nil {
		t.Fatalf("digest signable manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest["signatures"] = []map[string]string{{
		"alg":           signature.Alg,
		"key_id":        signature.KeyID,
		"sig":           signature.Sig,
		"signed_digest": signature.SignedDigest,
	}}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal signed manifest: %v", err)
	}
	if err := os.WriteFile(registryManifestPath, manifestRaw, 0o600); err != nil {
		t.Fatalf("write registry manifest: %v", err)
	}
	publicKeyPath := filepath.Join(workDir, "public.key")
	if err := os.WriteFile(publicKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	cacheDir := filepath.Join(workDir, "registry_cache")
	if code := runRegistryInstall([]string{
		"--source", registryManifestPath,
		"--cache-dir", cacheDir,
		"--public-key", publicKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runRegistryInstall: expected %d got %d", exitOK, code)
	}
	if code := runRegistryList([]string{"--cache-dir", cacheDir, "--json"}); code != exitOK {
		t.Fatalf("runRegistryList: expected %d got %d", exitOK, code)
	}
	installedMetadataPath := filepath.Join(cacheDir, "baseline-highrisk", "1.1.0", digest, "registry_pack.json")
	if code := runRegistryVerify([]string{
		"--path", installedMetadataPath,
		"--cache-dir", cacheDir,
		"--public-key", publicKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runRegistryVerify success: expected %d got %d", exitOK, code)
	}
	otherPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alternate key pair: %v", err)
	}
	otherPublicKeyPath := filepath.Join(workDir, "public_other.key")
	if err := os.WriteFile(otherPublicKeyPath, []byte(base64.StdEncoding.EncodeToString(otherPair.Public)), 0o600); err != nil {
		t.Fatalf("write alternate public key: %v", err)
	}
	if code := runRegistryVerify([]string{
		"--path", installedMetadataPath,
		"--cache-dir", cacheDir,
		"--public-key", otherPublicKeyPath,
		"--json",
	}); code != exitVerifyFailed {
		t.Fatalf("runRegistryVerify invalid key: expected %d got %d", exitVerifyFailed, code)
	}
}

func TestRunRecordAndMigrateBranches(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runRecord([]string{"--explain"}); code != exitOK {
		t.Fatalf("runRecord explain: expected %d got %d", exitOK, code)
	}
	if code := runInspect([]string{"--explain"}); code != exitOK {
		t.Fatalf("runInspect explain: expected %d got %d", exitOK, code)
	}
	if code := runMigrate([]string{"--explain"}); code != exitOK {
		t.Fatalf("runMigrate explain: expected %d got %d", exitOK, code)
	}

	run, intents, results, refs, err := buildDemoRunpack()
	if err != nil {
		t.Fatalf("build demo runpack: %v", err)
	}
	inputPath := filepath.Join(workDir, "record_input.json")
	payload := runRecordInput{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal record input: %v", err)
	}
	if err := os.WriteFile(inputPath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatalf("write record input: %v", err)
	}

	if code := runRecord([]string{"--input", inputPath, "--capture-mode", "invalid"}); code != exitInvalidInput {
		t.Fatalf("runRecord invalid capture mode: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRecord([]string{"--json", inputPath, "--run-id", "run_shifted"}); code != exitOK {
		t.Fatalf("runRecord interspersed flags: expected %d got %d", exitOK, code)
	}
	if _, err := os.Stat(filepath.Join(workDir, "gait-out", "runpack_run_shifted.zip")); err != nil {
		t.Fatalf("runRecord interspersed output missing: %v", err)
	}

	if code := runRecord([]string{"--input", filepath.Join(workDir, "missing.json")}); code != exitInvalidInput {
		t.Fatalf("runRecord missing input file: expected %d got %d", exitInvalidInput, code)
	}
	if code := runInspect([]string{"--from", filepath.Join(workDir, "missing.zip")}); code != exitInvalidInput {
		t.Fatalf("runInspect missing runpack: expected %d got %d", exitInvalidInput, code)
	}

	invalidJSONPath := filepath.Join(workDir, "invalid.json")
	mustWriteFile(t, invalidJSONPath, "{\n")
	if _, err := readRunRecordInput(invalidJSONPath); err == nil {
		t.Fatalf("readRunRecordInput invalid json: expected error")
	}

	emptyRunPath := filepath.Join(workDir, "empty_run.json")
	mustWriteFile(t, emptyRunPath, `{"run":{"schema_id":"gait.runpack.run"},"intents":[],"results":[],"refs":{"schema_id":"gait.runpack.refs","schema_version":"1.0.0","created_at":"2026-02-05T00:00:00Z","producer_version":"test","run_id":"","receipts":[]}}`+"\n")
	if code := runRecord([]string{"--input", emptyRunPath}); code != exitInvalidInput {
		t.Fatalf("runRecord missing run_id: expected %d got %d", exitInvalidInput, code)
	}

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup for migrate branches: expected %d got %d", exitOK, code)
	}
	if code := runMigrate([]string{"--json", "run_demo", "--target", "v1"}); code != exitOK {
		t.Fatalf("runMigrate interspersed flags: expected %d got %d", exitOK, code)
	}
	runpackPath := filepath.Join(workDir, "gait-out", "runpack_run_demo.zip")
	if code := runMigrate([]string{"--input", runpackPath, "--out", runpackPath}); code != exitInvalidInput {
		t.Fatalf("runMigrate out==input: expected %d got %d", exitInvalidInput, code)
	}

	unsupportedArtifact := filepath.Join(workDir, "artifact.txt")
	mustWriteFile(t, unsupportedArtifact, "artifact\n")
	if code := runMigrate([]string{"--input", unsupportedArtifact}); code != exitInvalidInput {
		t.Fatalf("runMigrate unsupported artifact: expected %d got %d", exitInvalidInput, code)
	}

	if got := displayOutputPath("gait-out/runpack.zip"); got != "./gait-out/runpack.zip" {
		t.Fatalf("displayOutputPath relative: got %s", got)
	}
	if got := displayOutputPath("./gait-out/runpack.zip"); got != "./gait-out/runpack.zip" {
		t.Fatalf("displayOutputPath dot-prefixed: got %s", got)
	}
	if got := defaultMigratedRunpackPath(filepath.FromSlash("/tmp/input.zip"), ""); got != filepath.FromSlash("/tmp/input_migrated.zip") {
		t.Fatalf("defaultMigratedRunpackPath fallback: got %s", got)
	}
}

func TestCommandRoutersAndHelpers(t *testing.T) {
	if code := runGate(nil); code != exitInvalidInput {
		t.Fatalf("runGate no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGate([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runGate unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicy(nil); code != exitInvalidInput {
		t.Fatalf("runPolicy no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicy([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runPolicy unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runTrace(nil); code != exitInvalidInput {
		t.Fatalf("runTrace no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runTrace([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runTrace unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegress(nil); code != exitInvalidInput {
		t.Fatalf("runRegress no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegress([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runRegress unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runCommand(nil); code != exitInvalidInput {
		t.Fatalf("runCommand no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runCommand([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runCommand unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runScout(nil); code != exitInvalidInput {
		t.Fatalf("runScout no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runScout([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runScout unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuard(nil); code != exitInvalidInput {
		t.Fatalf("runGuard no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuard([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runGuard unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegistry(nil); code != exitInvalidInput {
		t.Fatalf("runRegistry no args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegistry([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runRegistry unknown: expected %d got %d", exitInvalidInput, code)
	}
	if code := runMigrate(nil); code != exitInvalidInput {
		t.Fatalf("runMigrate no args: expected %d got %d", exitInvalidInput, code)
	}

	if joined := joinCSV([]string{"a", "b", "c"}); joined != "a,b,c" {
		t.Fatalf("joinCSV mismatch: %s", joined)
	}
	if values := parseCSV("a, b, ,c"); len(values) != 3 || values[1] != "b" {
		t.Fatalf("parseCSV mismatch: %#v", values)
	}
	merged := mergeUniqueSorted([]string{"b", "a", "a"}, []string{"c", "b", " "})
	if strings.Join(merged, ",") != "a,b,c" {
		t.Fatalf("mergeUniqueSorted mismatch: %#v", merged)
	}

	if !hasAnyKeySource(sign.KeyConfig{PrivateKeyPath: "x"}) {
		t.Fatalf("expected key source detection")
	}
	if hasAnyKeySource(sign.KeyConfig{}) {
		t.Fatalf("unexpected key source detection")
	}

	normalized := reorderInterspersedFlags([]string{
		"run_demo",
		"--json",
		"--output", "diff.json",
		"run_other",
	}, map[string]bool{"output": true})
	joined := strings.Join(normalized, " ")
	if joined != "--json --output diff.json run_demo run_other" {
		t.Fatalf("reorderInterspersedFlags mismatch: %s", joined)
	}
}

func TestValidationBranches(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo([]string{"--help"}); code != exitOK {
		t.Fatalf("runDemo help: expected %d got %d", exitOK, code)
	}
	if code := runPolicyInit([]string{"--help"}); code != exitOK {
		t.Fatalf("runPolicyInit help: expected %d got %d", exitOK, code)
	}
	if code := runGateEval([]string{}); code != exitInvalidInput {
		t.Fatalf("runGateEval missing args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicyInit([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runPolicyInit missing template: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicyTest([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runPolicyTest missing positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicyValidate([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runPolicyValidate missing positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runPolicyFmt([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runPolicyFmt missing positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runTraceVerify([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runTraceVerify missing path: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runVerify missing path: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"chain", "--json"}); code != exitInvalidInput {
		t.Fatalf("runVerify chain missing --run: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegressInit([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runRegressInit missing --from: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegressRun([]string{"--json", "extra"}); code != exitInvalidInput {
		t.Fatalf("runRegressRun positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegressBootstrap([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runRegressBootstrap missing --from: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegressBootstrap([]string{"--from", "run_demo", "--json", "extra"}); code != exitInvalidInput {
		t.Fatalf("runRegressBootstrap positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRecord([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runRecord missing input: expected %d got %d", exitInvalidInput, code)
	}
	if code := runMigrate([]string{"--target", "v2", "--json"}); code != exitInvalidInput {
		t.Fatalf("runMigrate invalid target: expected %d got %d", exitInvalidInput, code)
	}
	if code := runReduce([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runReduce missing from: expected %d got %d", exitInvalidInput, code)
	}
	if code := runScoutSnapshot([]string{"--roots", filepath.Join(workDir, "missing"), "--json"}); code != exitInvalidInput {
		t.Fatalf("runScoutSnapshot missing root: expected %d got %d", exitInvalidInput, code)
	}
	if code := runScoutSignal([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runScoutSignal missing runs: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuardPack([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runGuardPack missing run: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuardVerify([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runGuardVerify missing path: expected %d got %d", exitInvalidInput, code)
	}
	if code := runRegistryInstall([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("runRegistryInstall missing source: expected %d got %d", exitInvalidInput, code)
	}
	if code := runDiff([]string{"--json", "left-only"}); code != exitInvalidInput {
		t.Fatalf("runDiff positional args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}
	if code := runDemo([]string{"--json"}); code != exitOK {
		t.Fatalf("runDemo json: expected %d got %d", exitOK, code)
	}
	if code := runVerify([]string{"run_demo", "--json"}); code != exitOK {
		t.Fatalf("runVerify trailing json flag: expected %d got %d", exitOK, code)
	}
	if code := runDiff([]string{"run_demo", "run_demo", "--json"}); code != exitOK {
		t.Fatalf("runDiff trailing json flag: expected %d got %d", exitOK, code)
	}
	if code := runReplay([]string{"run_demo", "--json"}); code != exitOK {
		t.Fatalf("runReplay trailing json flag: expected %d got %d", exitOK, code)
	}
	if code := runVerify([]string{"--json", "--require-signature", "run_demo"}); code != exitVerifyFailed {
		t.Fatalf("runVerify require signature: expected %d got %d", exitVerifyFailed, code)
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	intentPath := filepath.Join(workDir, "intent.json")
	mustWriteFile(t, policyPath, "default_verdict: require_approval\n")
	writeIntentFixture(t, intentPath, "tool.write")

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--approval-token", filepath.Join(workDir, "missing-token.json"),
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval missing approval token path: expected %d got %d", exitInvalidInput, code)
	}

	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)
	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--approval-public-key-env", "MISSING_PUBLIC_KEY",
		"--approval-token", filepath.Join(workDir, "missing-token.json"),
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval missing verify key env: expected %d got %d", exitInvalidInput, code)
	}

	if code := runVerify([]string{"--json", "--public-key-env", "MISSING_PUBLIC_KEY", "run_demo"}); code != exitInvalidInput {
		t.Fatalf("runVerify missing env key: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"chain", "--json", "--run", "run_demo", "--trace", "trace.json"}); code != exitInvalidInput {
		t.Fatalf("runVerify chain trace without key: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"chain", "--json", "--run", "run_demo"}); code != exitOK {
		t.Fatalf("runVerify chain run-only: expected %d got %d", exitOK, code)
	}

	gaitOutAsFile := filepath.Join(workDir, "gait-out")
	if err := os.RemoveAll(gaitOutAsFile); err != nil {
		t.Fatalf("remove gait-out dir: %v", err)
	}
	mustWriteFile(t, gaitOutAsFile, "not-a-dir\n")
	if code := runDemo(nil); code != exitInvalidInput {
		t.Fatalf("runDemo with invalid output dir: expected %d got %d", exitInvalidInput, code)
	}
}

func TestPolicyInitScaffolds(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy-highrisk.yaml")
	var initCode int
	initOutputRaw := captureStdout(t, func() {
		initCode = runPolicyInit([]string{"baseline-highrisk", "--out", policyPath, "--json"})
	})
	if initCode != exitOK {
		t.Fatalf("runPolicyInit: expected %d got %d", exitOK, initCode)
	}

	var initOutput policyInitOutput
	if err := json.Unmarshal([]byte(initOutputRaw), &initOutput); err != nil {
		t.Fatalf("decode policy init output: %v", err)
	}
	if !initOutput.OK {
		t.Fatalf("policy init returned ok=false: %#v", initOutput)
	}
	if initOutput.Template != "baseline-highrisk" {
		t.Fatalf("unexpected policy template: %s", initOutput.Template)
	}
	if initOutput.PolicyPath != policyPath {
		t.Fatalf("unexpected policy path: %s", initOutput.PolicyPath)
	}

	loadedPolicy, err := gatecore.LoadPolicyFile(policyPath)
	if err != nil {
		t.Fatalf("load generated policy: %v", err)
	}
	if loadedPolicy.DefaultVerdict != "block" {
		t.Fatalf("unexpected default verdict: %s", loadedPolicy.DefaultVerdict)
	}

	intentPath := filepath.Join(workDir, "intent_write.json")
	writeIntentFixture(t, intentPath, "tool.write")
	if code := runPolicyTest([]string{policyPath, intentPath, "--json"}); code != exitApprovalRequired {
		t.Fatalf("generated policy should require approval for write: expected %d got %d", exitApprovalRequired, code)
	}

	if code := runPolicyInit([]string{"baseline-highrisk", "--out", policyPath, "--json"}); code != exitInvalidInput {
		t.Fatalf("expected existing output guard to fail without force, got %d", code)
	}
	if code := runPolicyInit([]string{"baseline_high_risk", "--out", policyPath, "--force", "--json"}); code != exitOK {
		t.Fatalf("expected alias + force overwrite to succeed, got %d", code)
	}
}

func TestPolicyValidateFmtAndMatchedRuleOutput(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: block",
		"rules:",
		"  - name: allow-write",
		"    priority: 10",
		"    effect: allow",
		"    match:",
		"      tool_names: [tool.write]",
		"    reason_codes: [allow_write]",
	}, "\n")+"\n")
	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")

	var validateCode int
	validateRaw := captureStdout(t, func() {
		validateCode = runPolicyValidate([]string{"--json", policyPath})
	})
	if validateCode != exitOK {
		t.Fatalf("runPolicyValidate: expected %d got %d", exitOK, validateCode)
	}
	var validateOutput policyValidateOutput
	if err := json.Unmarshal([]byte(validateRaw), &validateOutput); err != nil {
		t.Fatalf("decode policy validate output: %v", err)
	}
	if !validateOutput.OK {
		t.Fatalf("policy validate returned ok=false: %#v", validateOutput)
	}
	if validateOutput.RuleCount != 1 {
		t.Fatalf("policy validate expected 1 rule, got %d", validateOutput.RuleCount)
	}
	if validateOutput.DefaultVerdict != "block" {
		t.Fatalf("policy validate default verdict mismatch: %s", validateOutput.DefaultVerdict)
	}
	if strings.TrimSpace(validateOutput.PolicyDigest) == "" {
		t.Fatalf("policy validate expected policy digest")
	}
	var validateTextCode int
	validateText := captureStdout(t, func() {
		validateTextCode = runPolicyValidate([]string{policyPath})
	})
	if validateTextCode != exitOK {
		t.Fatalf("runPolicyValidate text: expected %d got %d", exitOK, validateTextCode)
	}
	if !strings.Contains(validateText, "policy validate ok:") {
		t.Fatalf("policy validate text output mismatch: %s", validateText)
	}

	var fmtWriteCode int
	fmtWriteRaw := captureStdout(t, func() {
		fmtWriteCode = runPolicyFmt([]string{"--write", "--json", policyPath})
	})
	if fmtWriteCode != exitOK {
		t.Fatalf("runPolicyFmt write: expected %d got %d", exitOK, fmtWriteCode)
	}
	var fmtWriteOutput policyFmtOutput
	if err := json.Unmarshal([]byte(fmtWriteRaw), &fmtWriteOutput); err != nil {
		t.Fatalf("decode policy fmt write output: %v", err)
	}
	if !fmtWriteOutput.OK {
		t.Fatalf("policy fmt write returned ok=false: %#v", fmtWriteOutput)
	}

	var fmtIdempotentCode int
	fmtIdempotentRaw := captureStdout(t, func() {
		fmtIdempotentCode = runPolicyFmt([]string{"--write", "--json", policyPath})
	})
	if fmtIdempotentCode != exitOK {
		t.Fatalf("runPolicyFmt second write: expected %d got %d", exitOK, fmtIdempotentCode)
	}
	var fmtIdempotent policyFmtOutput
	if err := json.Unmarshal([]byte(fmtIdempotentRaw), &fmtIdempotent); err != nil {
		t.Fatalf("decode policy fmt second write output: %v", err)
	}
	if fmtIdempotent.Changed {
		t.Fatalf("policy fmt expected idempotent output on second write")
	}

	var fmtJSONCode int
	fmtJSONRaw := captureStdout(t, func() {
		fmtJSONCode = runPolicyFmt([]string{"--json", policyPath})
	})
	if fmtJSONCode != exitOK {
		t.Fatalf("runPolicyFmt json: expected %d got %d", exitOK, fmtJSONCode)
	}
	var fmtJSONOutput policyFmtOutput
	if err := json.Unmarshal([]byte(fmtJSONRaw), &fmtJSONOutput); err != nil {
		t.Fatalf("decode policy fmt json output: %v", err)
	}
	if strings.TrimSpace(fmtJSONOutput.Formatted) == "" {
		t.Fatalf("expected non-empty formatted payload in policy fmt json output")
	}

	var fmtStdoutCode int
	fmtStdout := captureStdout(t, func() {
		fmtStdoutCode = runPolicyFmt([]string{policyPath})
	})
	if fmtStdoutCode != exitOK {
		t.Fatalf("runPolicyFmt stdout: expected %d got %d", exitOK, fmtStdoutCode)
	}
	if !strings.Contains(fmtStdout, "schema_id: gait.gate.policy") {
		t.Fatalf("policy fmt stdout missing schema metadata: %s", fmtStdout)
	}

	var testCode int
	testRaw := captureStdout(t, func() {
		testCode = runPolicyTest([]string{policyPath, intentPath, "--json"})
	})
	if testCode != exitOK {
		t.Fatalf("runPolicyTest matched-rule: expected %d got %d", exitOK, testCode)
	}
	var testOutput policyTestOutput
	if err := json.Unmarshal([]byte(testRaw), &testOutput); err != nil {
		t.Fatalf("decode policy test output: %v", err)
	}
	if testOutput.MatchedRule != "allow-write" {
		t.Fatalf("expected matched_rule allow-write, got %q", testOutput.MatchedRule)
	}

	invalidPolicyPath := filepath.Join(workDir, "policy_invalid.yaml")
	mustWriteFile(t, invalidPolicyPath, "default_verdit: allow\n")
	if code := runPolicyValidate([]string{"--json", invalidPolicyPath}); code != exitInvalidInput {
		t.Fatalf("runPolicyValidate unknown field: expected %d got %d", exitInvalidInput, code)
	}
}

func TestDemoJSONOutput(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	var demoCode int
	demoOutputRaw := captureStdout(t, func() {
		demoCode = runDemo([]string{"--json"})
	})
	if demoCode != exitOK {
		t.Fatalf("runDemo json: expected %d got %d", exitOK, demoCode)
	}

	var decodedDemo demoOutput
	if err := json.Unmarshal([]byte(demoOutputRaw), &decodedDemo); err != nil {
		t.Fatalf("decode demo output: %v", err)
	}
	if !decodedDemo.OK {
		t.Fatalf("demo json output returned ok=false: %#v", decodedDemo)
	}
	if decodedDemo.RunID != "run_demo" {
		t.Fatalf("unexpected demo run_id: %s", decodedDemo.RunID)
	}
	if decodedDemo.Verify != "ok" {
		t.Fatalf("unexpected demo verify status: %s", decodedDemo.Verify)
	}
	if decodedDemo.Bundle != "./gait-out/runpack_run_demo.zip" {
		t.Fatalf("unexpected demo bundle path: %s", decodedDemo.Bundle)
	}
	if decodedDemo.DurationMS < 0 {
		t.Fatalf("unexpected negative demo duration: %d", decodedDemo.DurationMS)
	}
	if decodedDemo.Mode != string(demoModeStandard) {
		t.Fatalf("unexpected demo mode: %s", decodedDemo.Mode)
	}
	if len(decodedDemo.NextCommands) == 0 {
		t.Fatalf("expected guided next commands in demo output")
	}
	if decodedDemo.MetricsOptIn == "" {
		t.Fatalf("expected metrics opt-in hint in demo output")
	}
}

func TestDemoDurableAndPolicyModes(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	var durableCode int
	durableRaw := captureStdout(t, func() {
		durableCode = runDemo([]string{"--durable", "--json"})
	})
	if durableCode != exitOK {
		t.Fatalf("runDemo durable: expected %d got %d", exitOK, durableCode)
	}
	var durableOut demoOutput
	if err := json.Unmarshal([]byte(durableRaw), &durableOut); err != nil {
		t.Fatalf("decode durable demo output: %v", err)
	}
	if !durableOut.OK {
		t.Fatalf("durable demo expected ok=true: %#v", durableOut)
	}
	if durableOut.Mode != string(demoModeDurable) {
		t.Fatalf("unexpected durable mode: %s", durableOut.Mode)
	}
	if durableOut.JobID != demoDurableJobID {
		t.Fatalf("unexpected durable job id: %s", durableOut.JobID)
	}
	if durableOut.JobStatus != "completed" {
		t.Fatalf("expected durable job status completed, got %s", durableOut.JobStatus)
	}
	if durableOut.PackPath == "" {
		t.Fatalf("expected durable pack path")
	}

	if code := runDemo([]string{"--durable", "--policy", "--json"}); code != exitInvalidInput {
		t.Fatalf("runDemo conflicting modes expected %d got %d", exitInvalidInput, code)
	}

	var policyCode int
	policyRaw := captureStdout(t, func() {
		policyCode = runDemo([]string{"--policy", "--json"})
	})
	if policyCode != exitOK {
		t.Fatalf("runDemo policy: expected %d got %d", exitOK, policyCode)
	}
	var policyOut demoOutput
	if err := json.Unmarshal([]byte(policyRaw), &policyOut); err != nil {
		t.Fatalf("decode policy demo output: %v", err)
	}
	if !policyOut.OK {
		t.Fatalf("policy demo expected ok=true: %#v", policyOut)
	}
	if policyOut.Mode != string(demoModePolicy) {
		t.Fatalf("unexpected policy mode: %s", policyOut.Mode)
	}
	if policyOut.PolicyVerdict != "block" {
		t.Fatalf("expected policy verdict block, got %s", policyOut.PolicyVerdict)
	}
	if policyOut.MatchedRule != "block-destructive-tool-delete" {
		t.Fatalf("unexpected matched rule: %s", policyOut.MatchedRule)
	}
	if !strings.Contains(strings.Join(policyOut.ReasonCodes, ","), "destructive_tool_blocked") {
		t.Fatalf("missing destructive_tool_blocked reason code: %#v", policyOut.ReasonCodes)
	}
}

func TestTourJSONOutput(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	var tourCode int
	raw := captureStdout(t, func() {
		tourCode = runTour([]string{"--json"})
	})
	if tourCode != exitOK {
		t.Fatalf("runTour json expected %d got %d", exitOK, tourCode)
	}

	var output tourOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode tour output: %v", err)
	}
	if !output.OK {
		t.Fatalf("expected ok=true from tour output: %#v", output)
	}
	if output.Mode != "activation" {
		t.Fatalf("unexpected tour mode: %s", output.Mode)
	}
	if output.RunID != demoRunID {
		t.Fatalf("unexpected tour run id: %s", output.RunID)
	}
	if output.RegressStatus != regressStatusPass {
		t.Fatalf("unexpected tour regress status: %s", output.RegressStatus)
	}
	if len(output.NextCommands) == 0 {
		t.Fatalf("expected tour next commands")
	}
}

func TestVerifyJSONIncludesGuidance(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup expected %d got %d", exitOK, code)
	}

	var verifyCode int
	raw := captureStdout(t, func() {
		verifyCode = runVerify([]string{"--json", demoRunID})
	})
	if verifyCode != exitOK {
		t.Fatalf("runVerify expected %d got %d", exitOK, verifyCode)
	}

	var output verifyOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode verify output: %v", err)
	}
	if output.SignatureStatus == "" {
		t.Fatalf("expected signature_status in verify output")
	}
	if output.SignatureNote == "" {
		t.Fatalf("expected signature note in verify output")
	}
	if len(output.NextCommands) == 0 {
		t.Fatalf("expected verify next commands")
	}
}

func TestDoctorSummaryMode(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	repoRoot := repoRootFromPackageDir(t)

	var code int
	raw := captureStdout(t, func() {
		code = runDoctor([]string{
			"--workdir", repoRoot,
			"--output-dir", filepath.Join(workDir, "gait-out"),
			"--summary",
			"--json",
		})
	})
	if code != exitOK {
		t.Fatalf("runDoctor summary expected %d got %d", exitOK, code)
	}
	var output doctorOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode doctor summary output: %v", err)
	}
	if !output.SummaryMode {
		t.Fatalf("expected summary_mode=true")
	}
	if output.Summary == "" {
		t.Fatalf("expected summary text in doctor output")
	}
}

func TestGateEvalApprovalChainAndSimulation(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")

	policyPath := filepath.Join(workDir, "policy_chain.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: approval-chain",
		"    effect: require_approval",
		"    min_approvals: 2",
		"    match:",
		"      tool_names: [tool.write]",
	}, "\n")+"\n")

	policy, err := gatecore.LoadPolicyFile(policyPath)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	intent, err := readIntentRequest(intentPath)
	if err != nil {
		t.Fatalf("read intent: %v", err)
	}
	policyDigest, intentDigest, _, err := gatecore.ApprovalContext(policy, intent)
	if err != nil {
		t.Fatalf("approval context: %v", err)
	}
	privateKeyPath := filepath.Join(workDir, "approval_private.key")
	writePrivateKey(t, privateKeyPath)

	tokenAPath := filepath.Join(workDir, "approval_a.json")
	if code := runApprove([]string{
		"--intent-digest", intentDigest,
		"--policy-digest", policyDigest,
		"--ttl", "1h",
		"--scope", "tool:tool.write",
		"--approver", "alice",
		"--reason-code", "ticket-123",
		"--out", tokenAPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runApprove token A: expected %d got %d", exitOK, code)
	}
	tokenBPath := filepath.Join(workDir, "approval_b.json")
	if code := runApprove([]string{
		"--intent-digest", intentDigest,
		"--policy-digest", policyDigest,
		"--ttl", "1h",
		"--scope", "tool:tool.write",
		"--approver", "bob",
		"--reason-code", "ticket-123",
		"--out", tokenBPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runApprove token B: expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--approval-token", tokenAPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--approval-private-key", privateKeyPath,
		"--json",
	}); code != exitApprovalRequired {
		t.Fatalf("runGateEval single token: expected %d got %d", exitApprovalRequired, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--approval-token", tokenAPath,
		"--approval-token-chain", tokenBPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--approval-private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval token chain: expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--simulate",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval simulate mode: expected %d got %d", exitOK, code)
	}
}

func TestGateEvalOSSProdProfile(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")
	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)

	allowPolicyPath := filepath.Join(workDir, "policy_allow.yaml")
	mustWriteFile(t, allowPolicyPath, "default_verdict: allow\n")

	if code := runGateEval([]string{
		"--policy", allowPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--simulate",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod simulate: expected %d got %d", exitInvalidInput, code)
	}

	if code := runGateEval([]string{
		"--policy", allowPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod missing key: expected %d got %d", exitInvalidInput, code)
	}

	if code := runGateEval([]string{
		"--policy", allowPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "dev",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod dev key mode: expected %d got %d", exitInvalidInput, code)
	}

	if code := runGateEval([]string{
		"--policy", allowPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval oss-prod allow: expected %d got %d", exitOK, code)
	}

	highRiskNoBrokerPolicyPath := filepath.Join(workDir, "policy_high_risk_no_broker.yaml")
	mustWriteFile(t, highRiskNoBrokerPolicyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: high-risk-allow",
		"    effect: allow",
		"    match:",
		"      risk_classes: [high]",
	}, "\n")+"\n")
	if code := runGateEval([]string{
		"--policy", highRiskNoBrokerPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod high-risk without broker requirement: expected %d got %d", exitInvalidInput, code)
	}

	highRiskWithBrokerPolicyPath := filepath.Join(workDir, "policy_high_risk_with_broker.yaml")
	mustWriteFile(t, highRiskWithBrokerPolicyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: high-risk-allow",
		"    effect: allow",
		"    require_broker_credential: true",
		"    match:",
		"      risk_classes: [high]",
	}, "\n")+"\n")
	if code := runGateEval([]string{
		"--policy", highRiskWithBrokerPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod high-risk missing broker runtime: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGateEval([]string{
		"--policy", highRiskWithBrokerPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--credential-broker", "stub",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval oss-prod high-risk with broker runtime: expected %d got %d", exitOK, code)
	}

	approvalPolicyPath := filepath.Join(workDir, "policy_approval.yaml")
	mustWriteFile(t, approvalPolicyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: needs-approval",
		"    effect: require_approval",
		"    match:",
		"      tool_names: [tool.write]",
	}, "\n")+"\n")

	if code := runGateEval([]string{
		"--policy", approvalPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval oss-prod missing approval verify key: expected %d got %d", exitInvalidInput, code)
	}

	if code := runGateEval([]string{
		"--policy", approvalPolicyPath,
		"--intent", intentPath,
		"--profile", "oss-prod",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--approval-private-key", privateKeyPath,
		"--json",
	}); code != exitApprovalRequired {
		t.Fatalf("runGateEval oss-prod approval path: expected %d got %d", exitApprovalRequired, code)
	}
}

func TestGateEvalProjectConfigDefaults(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")
	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)
	policyPath := filepath.Join(workDir, "policy_allow.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	if err := os.MkdirAll(filepath.Join(workDir, ".gait"), 0o750); err != nil {
		t.Fatalf("mkdir .gait: %v", err)
	}
	mustWriteFile(t, filepath.Join(workDir, ".gait", "config.yaml"), strings.Join([]string{
		"gate:",
		"  policy: " + policyPath,
		"  profile: oss-prod",
		"  key_mode: prod",
		"  private_key: " + privateKeyPath,
		"  credential_broker: stub",
	}, "\n")+"\n")

	if code := runGateEval([]string{
		"--intent", intentPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval config defaults: expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--intent", intentPath,
		"--no-config",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval no-config should require policy: expected %d got %d", exitInvalidInput, code)
	}

	if code := runGateEval([]string{
		"--intent", intentPath,
		"--config", filepath.Join(workDir, "missing.yaml"),
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval missing explicit config: expected %d got %d", exitInvalidInput, code)
	}
}

func TestVerifyChainRunTracePack(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}

	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)
	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")
	policyPath := filepath.Join(workDir, "policy_allow.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	tracePath := filepath.Join(workDir, "trace_chain.json")
	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--trace-out", tracePath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval trace for chain: expected %d got %d", exitOK, code)
	}

	packPath := filepath.Join(workDir, "evidence_chain.zip")
	if code := runGuardPack([]string{
		"--run", "run_demo",
		"--out", packPath,
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardPack for chain: expected %d got %d", exitOK, code)
	}

	if code := runVerify([]string{
		"chain",
		"--run", "run_demo",
		"--trace", tracePath,
		"--pack", packPath,
		"--private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runVerify chain success: expected %d got %d", exitOK, code)
	}

	if code := runVerify([]string{
		"chain",
		"--run", "run_demo",
		"--trace", tracePath,
		"--pack", packPath,
		"--private-key", privateKeyPath,
		"--require-signature",
		"--json",
	}); code != exitVerifyFailed {
		t.Fatalf("runVerify chain require-signature: expected %d got %d", exitVerifyFailed, code)
	}
}

func TestStrictVerifyProfiles(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}

	privateKeyPath := filepath.Join(workDir, "private.key")
	writePrivateKey(t, privateKeyPath)

	if code := runVerify([]string{"--profile", "strict", "--json", "run_demo"}); code != exitInvalidInput {
		t.Fatalf("runVerify strict missing key: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"--profile", "strict", "--private-key", privateKeyPath, "--json", "run_demo"}); code != exitVerifyFailed {
		t.Fatalf("runVerify strict signature enforcement: expected %d got %d", exitVerifyFailed, code)
	}
	if code := runVerify([]string{"chain", "--run", "run_demo", "--profile", "strict", "--json"}); code != exitInvalidInput {
		t.Fatalf("runVerify chain strict missing key: expected %d got %d", exitInvalidInput, code)
	}
	if code := runVerify([]string{"chain", "--run", "run_demo", "--profile", "strict", "--private-key", privateKeyPath, "--json"}); code != exitVerifyFailed {
		t.Fatalf("runVerify chain strict signature enforcement: expected %d got %d", exitVerifyFailed, code)
	}

	packPath := filepath.Join(workDir, "evidence_pack_strict.zip")
	if code := runGuardPack([]string{
		"--run", "run_demo",
		"--out", packPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGuardPack strict profile fixture: expected %d got %d", exitOK, code)
	}
	if code := runGuardVerify([]string{packPath, "--profile", "strict", "--json"}); code != exitInvalidInput {
		t.Fatalf("runGuardVerify strict missing key: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGuardVerify([]string{packPath, "--profile", "strict", "--private-key", privateKeyPath, "--json"}); code != exitVerifyFailed {
		t.Fatalf("runGuardVerify strict signature enforcement: expected %d got %d", exitVerifyFailed, code)
	}
}

func TestRunReplayUnsafeRequiresAllowToolsAndEnv(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	if code := runDemo(nil); code != exitOK {
		t.Fatalf("runDemo setup: expected %d got %d", exitOK, code)
	}

	if code := runReplay([]string{"--json", "--real-tools", "--unsafe-real-tools", "run_demo"}); code != exitUnsafeReplay {
		t.Fatalf("runReplay missing --allow-tools: expected %d got %d", exitUnsafeReplay, code)
	}

	if code := runReplay([]string{"--json", "--real-tools", "--unsafe-real-tools", "--allow-tools", "tool.write", "run_demo"}); code != exitUnsafeReplay {
		t.Fatalf("runReplay missing env interlock: expected %d got %d", exitUnsafeReplay, code)
	}

	t.Setenv("GAIT_ALLOW_REAL_REPLAY", "1")
	if code := runReplay([]string{"--json", "--real-tools", "--unsafe-real-tools", "--allow-tools", "tool.write", "run_demo"}); code != exitOK {
		t.Fatalf("runReplay with env interlock: expected %d got %d", exitOK, code)
	}
}

func TestGateEvalCredentialCommandBrokerAndRateLimit(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")

	policyPath := filepath.Join(workDir, "policy_gate_v12.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: protected-write",
		"    effect: allow",
		"    require_broker_credential: true",
		"    broker_reference: egress",
		"    broker_scopes: [export]",
		"    rate_limit:",
		"      requests: 1",
		"      scope: tool_identity",
		"      window: minute",
		"    match:",
		"      tool_names: [tool.write]",
	}, "\n")+"\n")

	brokerPath := filepath.Join(workDir, "broker.sh")
	brokerScript := "#!/bin/sh\necho '{\"issued_by\":\"command\",\"credential_ref\":\"cmd:token\"}'\n"
	if runtime.GOOS == "windows" {
		brokerPath = filepath.Join(workDir, "broker.cmd")
		brokerScript = "@echo {\"issued_by\":\"command\",\"credential_ref\":\"cmd:token\"}\r\n"
	}
	mustWriteFile(t, brokerPath, brokerScript)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(brokerPath, 0o700); err != nil {
			t.Fatalf("chmod broker script: %v", err)
		}
	}

	rateStatePath := filepath.Join(workDir, "rate_state.json")
	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--credential-broker", "command",
		"--credential-command", brokerPath,
		"--rate-limit-state", rateStatePath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval first call: expected %d got %d", exitOK, code)
	}
	credentialEvidence, err := filepath.Glob(filepath.Join(workDir, "credential_evidence_*.json"))
	if err != nil {
		t.Fatalf("glob credential evidence: %v", err)
	}
	if len(credentialEvidence) != 1 {
		t.Fatalf("expected one credential evidence artifact, got %d", len(credentialEvidence))
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--credential-broker", "command",
		"--credential-command", brokerPath,
		"--rate-limit-state", rateStatePath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval second call: expected %d got %d", exitOK, code)
	}
	credentialEvidence, err = filepath.Glob(filepath.Join(workDir, "credential_evidence_*.json"))
	if err != nil {
		t.Fatalf("glob credential evidence after rate limit: %v", err)
	}
	if len(credentialEvidence) != 1 {
		t.Fatalf("expected rate-limited run to avoid new credential artifact, got %d", len(credentialEvidence))
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--credential-broker", "unknown",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval invalid broker: expected %d got %d", exitInvalidInput, code)
	}
	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--credential-broker", "command",
		"--json",
	}); code != exitInvalidInput {
		t.Fatalf("runGateEval command broker missing command: expected %d got %d", exitInvalidInput, code)
	}
}

func TestGateEvalCredentialCommandBrokerFailureDoesNotLeakSecrets(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.write")

	policyPath := filepath.Join(workDir, "policy_gate_v12.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: protected-write",
		"    effect: allow",
		"    require_broker_credential: true",
		"    broker_reference: egress",
		"    broker_scopes: [export]",
		"    match:",
		"      tool_names: [tool.write]",
	}, "\n")+"\n")

	brokerPath := filepath.Join(workDir, "broker_fail.sh")
	brokerScript := "#!/bin/sh\necho 'forced failure token=secret-broker-token' 1>&2\nexit 2\n"
	if runtime.GOOS == "windows" {
		brokerPath = filepath.Join(workDir, "broker_fail.cmd")
		brokerScript = "@echo forced failure token=secret-broker-token 1>&2\r\n@exit /b 2\r\n"
	}
	mustWriteFile(t, brokerPath, brokerScript)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(brokerPath, 0o700); err != nil {
			t.Fatalf("chmod broker script: %v", err)
		}
	}

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = runGateEval([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--credential-broker", "command",
			"--credential-command", brokerPath,
			"--json",
		})
	})
	if exitCode != exitOK {
		t.Fatalf("runGateEval broker failure expected %d got %d", exitOK, exitCode)
	}
	if strings.Contains(output, "secret-broker-token") {
		t.Fatalf("gate output leaked broker token: %s", output)
	}
	if !strings.Contains(output, `"verdict":"block"`) {
		t.Fatalf("expected blocked verdict in output, got: %s", output)
	}
	if !strings.Contains(output, "broker_credential_missing") {
		t.Fatalf("expected broker_credential_missing reason in output, got: %s", output)
	}
}

func TestOutputWritersAndUsagePrinters(t *testing.T) {
	if code := writeApproveOutput(true, approveOutput{OK: true, TokenPath: "token.json"}, exitOK); code != exitOK {
		t.Fatalf("writeApproveOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeApproveOutput(false, approveOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeApproveOutput text: expected %d got %d", exitInvalidInput, code)
	}

	if code := writeGateEvalOutput(true, gateEvalOutput{OK: true, Verdict: "allow"}, exitOK); code != exitOK {
		t.Fatalf("writeGateEvalOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeGateEvalOutput(false, gateEvalOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeGateEvalOutput text: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeGateEvalOutput(false, gateEvalOutput{
		OK:          true,
		Verdict:     "allow",
		TracePath:   "trace.json",
		ReasonCodes: []string{"r1"},
		Violations:  []string{"v1"},
		Warnings:    []string{"w1"},
	}, exitOK); code != exitOK {
		t.Fatalf("writeGateEvalOutput text ok: expected %d got %d", exitOK, code)
	}

	if code := writePolicyTestOutput(true, policyTestOutput{OK: true, Summary: "ok"}, exitOK); code != exitOK {
		t.Fatalf("writePolicyTestOutput json: expected %d got %d", exitOK, code)
	}
	if code := writePolicyTestOutput(false, policyTestOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writePolicyTestOutput text: expected %d got %d", exitInvalidInput, code)
	}
	if code := writePolicyInitOutput(false, policyInitOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writePolicyInitOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writePolicyInitOutput(false, policyInitOutput{
		OK:           true,
		Template:     "baseline-highrisk",
		PolicyPath:   "gait.policy.yaml",
		NextCommands: []string{"gait policy validate gait.policy.yaml --json"},
	}, exitOK); code != exitOK {
		t.Fatalf("writePolicyInitOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writePolicyValidateOutput(false, policyValidateOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writePolicyValidateOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writePolicyValidateOutput(false, policyValidateOutput{OK: true, Summary: "ok"}, exitOK); code != exitOK {
		t.Fatalf("writePolicyValidateOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writePolicyFmtOutput(false, policyFmtOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writePolicyFmtOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writePolicyFmtOutput(false, policyFmtOutput{
		OK:      true,
		Path:    "gait.policy.yaml",
		Changed: true,
		Written: true,
	}, exitOK); code != exitOK {
		t.Fatalf("writePolicyFmtOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writePolicySimulateOutput(false, policySimulateOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writePolicySimulateOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writePolicySimulateOutput(false, policySimulateOutput{OK: true, Summary: "policy simulate ok"}, exitOK); code != exitOK {
		t.Fatalf("writePolicySimulateOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeKeysInitOutput(false, keysInitOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeKeysInitOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeKeysInitOutput(false, keysInitOutput{
		OK:             true,
		KeyID:          "kid",
		PublicKeyPath:  "pub.key",
		PrivateKeyPath: "priv.key",
	}, exitOK); code != exitOK {
		t.Fatalf("writeKeysInitOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeKeysVerifyOutput(false, keysVerifyOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeKeysVerifyOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeKeysVerifyOutput(false, keysVerifyOutput{OK: true, KeyID: "kid"}, exitOK); code != exitOK {
		t.Fatalf("writeKeysVerifyOutput text ok: expected %d got %d", exitOK, code)
	}

	if code := writeRegressInitOutput(true, regressInitOutput{OK: true, FixtureName: "f"}, exitOK); code != exitOK {
		t.Fatalf("writeRegressInitOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRegressInitOutput(false, regressInitOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeRegressInitOutput text: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeRegressInitOutput(false, regressInitOutput{
		OK:           true,
		FixtureName:  "fixture",
		RunID:        "run_demo",
		ConfigPath:   "gait.yaml",
		RunpackPath:  "fixtures/run_demo/runpack.zip",
		NextCommands: []string{"gait regress run --json"},
	}, exitOK); code != exitOK {
		t.Fatalf("writeRegressInitOutput text ok: expected %d got %d", exitOK, code)
	}

	if code := writeRegressRunOutput(true, regressRunOutput{OK: true, FixtureSet: "default"}, exitOK); code != exitOK {
		t.Fatalf("writeRegressRunOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRegressRunOutput(false, regressRunOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeRegressRunOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeRegressRunOutput(false, regressRunOutput{
		OK:         true,
		FixtureSet: "default",
		Graders:    3,
		Output:     "regress_result.json",
		JUnit:      "junit.xml",
	}, exitOK); code != exitOK {
		t.Fatalf("writeRegressRunOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeRegressRunOutput(false, regressRunOutput{OK: false, FixtureSet: "default", Failed: 1}, exitRegressFailed); code != exitRegressFailed {
		t.Fatalf("writeRegressRunOutput text fail: expected %d got %d", exitRegressFailed, code)
	}
	if code := writeRegressBootstrapOutput(true, regressBootstrapOutput{OK: true, FixtureName: "fixture"}, exitOK); code != exitOK {
		t.Fatalf("writeRegressBootstrapOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRegressBootstrapOutput(false, regressBootstrapOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeRegressBootstrapOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeRegressBootstrapOutput(false, regressBootstrapOutput{
		OK:               false,
		RunID:            "run_demo",
		FixtureName:      "run_demo",
		Failed:           1,
		TopFailureReason: "unexpected_exit_code",
		NextCommand:      "gait regress run --json",
		ArtifactPaths:    []string{"regress_result.json", "junit.xml"},
	}, exitRegressFailed); code != exitRegressFailed {
		t.Fatalf("writeRegressBootstrapOutput text fail: expected %d got %d", exitRegressFailed, code)
	}

	if code := writeDiffOutput(true, diffOutput{OK: true}, exitOK); code != exitOK {
		t.Fatalf("writeDiffOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeDiffOutput(false, diffOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeDiffOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeDiffOutput(false, diffOutput{
		OK: false,
		Summary: runpack.DiffSummary{
			RunIDLeft:    "left",
			RunIDRight:   "right",
			FilesChanged: []string{"manifest.json"},
		},
	}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeDiffOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeReplayOutput(true, replayOutput{OK: true}, exitOK); code != exitOK {
		t.Fatalf("writeReplayOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeReplayOutput(false, replayOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeReplayOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeReplayOutput(false, replayOutput{
		OK:             false,
		RunID:          "run_demo",
		MissingResults: []string{"intent_2"},
		Warnings:       []string{"warn"},
	}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeReplayOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeRunRecordOutput(true, runRecordOutput{OK: true, RunID: "run_demo"}, exitOK); code != exitOK {
		t.Fatalf("writeRunRecordOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRunRecordOutput(false, runRecordOutput{OK: true, RunID: "run_demo", Bundle: "b", TicketFooter: "t"}, exitOK); code != exitOK {
		t.Fatalf("writeRunRecordOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeRunRecordOutput(false, runRecordOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeRunRecordOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeRunReceiptOutput(true, runReceiptOutput{OK: true, TicketFooter: "x"}, exitOK); code != exitOK {
		t.Fatalf("writeRunReceiptOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRunReceiptOutput(false, runReceiptOutput{OK: true, TicketFooter: "x"}, exitOK); code != exitOK {
		t.Fatalf("writeRunReceiptOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeRunReceiptOutput(false, runReceiptOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeRunReceiptOutput text err: expected %d got %d", exitInvalidInput, code)
	}

	if code := writeMigrateOutput(true, migrateOutput{OK: true, Input: "a", Output: "b"}, exitOK); code != exitOK {
		t.Fatalf("writeMigrateOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeMigrateOutput(false, migrateOutput{OK: true, Input: "a", Output: "b", Status: "migrated"}, exitOK); code != exitOK {
		t.Fatalf("writeMigrateOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeMigrateOutput(false, migrateOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeMigrateOutput text err: expected %d got %d", exitInvalidInput, code)
	}

	if code := writeTraceVerifyOutput(true, traceVerifyOutput{OK: true, Path: "trace.json"}, exitOK); code != exitOK {
		t.Fatalf("writeTraceVerifyOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeTraceVerifyOutput(false, traceVerifyOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeTraceVerifyOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeTraceVerifyOutput(false, traceVerifyOutput{OK: false, Path: "trace.json", SignatureStatus: "failed"}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeTraceVerifyOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeDoctorOutput(true, doctorOutput{OK: true, Summary: "ok"}, exitOK); code != exitOK {
		t.Fatalf("writeDoctorOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeDoctorOutput(false, doctorOutput{
		OK:      true,
		Summary: "doctor summary",
		Checks: []doctor.Check{
			{Name: "check1", Status: "pass", Message: "ok"},
		},
	}, exitOK); code != exitOK {
		t.Fatalf("writeDoctorOutput text ok: expected %d got %d", exitOK, code)
	}
	if code := writeDoctorOutput(false, doctorOutput{Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeDoctorOutput text error: expected %d got %d", exitInvalidInput, code)
	}

	if code := writeVerifyOutput(true, verifyOutput{OK: true, Path: "runpack.zip"}, exitOK); code != exitOK {
		t.Fatalf("writeVerifyOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeVerifyOutput(false, verifyOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeVerifyOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeVerifyOutput(false, verifyOutput{
		OK:              false,
		Path:            "runpack.zip",
		MissingFiles:    []string{"manifest.json"},
		HashMismatches:  []runpack.HashMismatch{{Path: "run.json"}},
		SignatureStatus: "failed",
		SignatureErrors: []string{"bad sig"},
	}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeVerifyOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}
	if code := writeVerifyChainOutput(true, verifyChainOutput{
		OK:  true,
		Run: verifyOutput{OK: true, Path: "runpack.zip"},
	}, exitOK); code != exitOK {
		t.Fatalf("writeVerifyChainOutput json ok: expected %d got %d", exitOK, code)
	}
	if code := writeVerifyChainOutput(false, verifyChainOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeVerifyChainOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeVerifyChainOutput(false, verifyChainOutput{
		OK:  false,
		Run: verifyOutput{Path: "runpack.zip"},
		Trace: &traceVerifyOutput{
			Path:            "trace.json",
			SignatureStatus: "failed",
		},
		Pack: &guardVerifyOutput{
			Path:            "pack.zip",
			SignatureStatus: "failed",
		},
	}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeVerifyChainOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeGuardPackOutput(true, guardPackOutput{OK: true, PackPath: "evidence.zip"}, exitOK); code != exitOK {
		t.Fatalf("writeGuardPackOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeGuardPackOutput(false, guardPackOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeGuardPackOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeGuardVerifyOutput(true, guardVerifyOutput{OK: true, Path: "evidence.zip"}, exitOK); code != exitOK {
		t.Fatalf("writeGuardVerifyOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeGuardVerifyOutput(false, guardVerifyOutput{OK: false, Path: "evidence.zip"}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeGuardVerifyOutput text fail: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeRegistryInstallOutput(true, registryInstallOutput{OK: true, PackName: "pack", PackVersion: "1.0.0"}, exitOK); code != exitOK {
		t.Fatalf("writeRegistryInstallOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeRegistryInstallOutput(false, registryInstallOutput{OK: false, Error: "bad"}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeRegistryInstallOutput text err: expected %d got %d", exitVerifyFailed, code)
	}

	if code := writeScoutSnapshotOutput(true, scoutSnapshotOutput{OK: true, SnapshotPath: "snapshot.json"}, exitOK); code != exitOK {
		t.Fatalf("writeScoutSnapshotOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeScoutSnapshotOutput(false, scoutSnapshotOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeScoutSnapshotOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeScoutDiffOutput(true, scoutDiffOutput{OK: true, Left: "a", Right: "b"}, exitOK); code != exitOK {
		t.Fatalf("writeScoutDiffOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeScoutDiffOutput(false, scoutDiffOutput{OK: false, Left: "a", Right: "b"}, exitVerifyFailed); code != exitVerifyFailed {
		t.Fatalf("writeScoutDiffOutput text changed: expected %d got %d", exitVerifyFailed, code)
	}
	if code := writeScoutSignalOutput(true, scoutSignalOutput{OK: true, OutputPath: "signal.json"}, exitOK); code != exitOK {
		t.Fatalf("writeScoutSignalOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeScoutSignalOutput(false, scoutSignalOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
		t.Fatalf("writeScoutSignalOutput text err: expected %d got %d", exitInvalidInput, code)
	}
	if code := writeScoutSignalOutput(false, scoutSignalOutput{
		OK:          true,
		OutputPath:  "signal.json",
		RunCount:    2,
		FamilyCount: 1,
		TopIssues:   1,
		Report: &schemascout.SignalReport{
			TopIssues: []schemascout.SignalIssue{{
				FamilyID:         "fam_123",
				SeverityLevel:    "high",
				SeverityScore:    120,
				TopFailureReason: "unexpected_diff",
			}},
		},
	}, exitOK); code != exitOK {
		t.Fatalf("writeScoutSignalOutput text ok: expected %d got %d", exitOK, code)
	}

	printUsage()
	printApproveUsage()
	printDemoUsage()
	printTourUsage()
	printDoctorUsage()
	printGateUsage()
	printGateEvalUsage()
	printPolicyUsage()
	printPolicyInitUsage()
	printPolicyValidateUsage()
	printPolicyFmtUsage()
	printPolicySimulateUsage()
	printPolicyTestUsage()
	printKeysUsage()
	printKeysInitUsage()
	printKeysRotateUsage()
	printKeysVerifyUsage()
	printTraceUsage()
	printTraceVerifyUsage()
	printRegressUsage()
	printRegressInitUsage()
	printRegressRunUsage()
	printRegressBootstrapUsage()
	printRunUsage()
	printRecordUsage()
	printRunReceiptUsage()
	printReplayUsage()
	printDiffUsage()
	printMigrateUsage()
	printVerifyUsage()
	printVerifyChainUsage()
	printScoutSignalUsage()
}

func TestTicketFooterContract(t *testing.T) {
	footer := formatTicketFooter("run_demo", strings.Repeat("a", 64))
	if !ticketFooterMatchesContract(footer) {
		t.Fatalf("expected ticket footer to match contract: %s", footer)
	}
	if ticketFooterMatchesContract("GAIT run_id=run_demo manifest=sha256:abc verify=\"gait verify run_demo\"") {
		t.Fatalf("expected short digest to fail contract")
	}
	if ticketFooterMatchesContract(
		"GAIT run_id=run_demo manifest=sha256:" + strings.Repeat("a", 64) + " verify=\"gait verify run_other\"",
	) {
		t.Fatalf("expected mismatched run_id verify target to fail contract")
	}
}

func TestTelemetryHealthSnapshotTracksWriteOutcomes(t *testing.T) {
	healthPath := filepath.Join(t.TempDir(), "telemetry_health.json")
	t.Setenv("GAIT_TELEMETRY_HEALTH_PATH", healthPath)
	t.Setenv("GAIT_TELEMETRY_WARN", "off")

	telemetryState.Lock()
	telemetryState.streams = map[string]telemetryStreamHealth{}
	telemetryState.Unlock()

	recordTelemetryWriteOutcome("adoption", nil)
	recordTelemetryWriteOutcome("operational_end", os.ErrPermission)
	recordTelemetryWriteOutcome("operational_end", nil)

	raw, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatalf("read telemetry health snapshot: %v", err)
	}
	var snapshot telemetryHealthSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode telemetry health snapshot: %v", err)
	}
	if snapshot.SchemaID != "gait.scout.telemetry_health" {
		t.Fatalf("unexpected telemetry schema id: %s", snapshot.SchemaID)
	}
	adoption := snapshot.Streams["adoption"]
	if adoption.Attempts != 1 || adoption.Success != 1 || adoption.Failed != 0 {
		t.Fatalf("unexpected adoption telemetry counters: %#v", adoption)
	}
	operationalEnd := snapshot.Streams["operational_end"]
	if operationalEnd.Attempts != 2 || operationalEnd.Success != 1 || operationalEnd.Failed != 1 {
		t.Fatalf("unexpected operational_end telemetry counters: %#v", operationalEnd)
	}
}

func withWorkingDir(t *testing.T, path string) {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(current)
	})
}

func writePrivateKey(t *testing.T, path string) {
	t.Helper()
	kp, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(kp.Private)
	if err := os.WriteFile(path, []byte(encoded+"\n"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
}

func writeIntentFixture(t *testing.T, path, toolName string) {
	t.Helper()
	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		ToolName:        toolName,
		Args: map[string]any{
			"path": "/tmp/out.txt",
		},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt"},
		},
		ArgProvenance: []schemagate.IntentArgProvenance{
			{ArgPath: "$.path", Source: "user"},
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/tmp",
			RiskClass: "high",
		},
	}
	raw, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatalf("marshal intent: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write intent: %v", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func repoRootFromPackageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
