//go:build scenario

package scenarios

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

type scenarioFlags struct {
	Simulate             bool     `yaml:"simulate"`
	Concurrency          int      `yaml:"concurrency"`
	DelegationChainFiles []string `yaml:"delegation_chain_files"`
}

type expectedYAML struct {
	ExitCode               int      `yaml:"exit_code"`
	Verdict                string   `yaml:"verdict"`
	SimulateMode           bool     `yaml:"simulate_mode"`
	SuccessfulRuns         int      `yaml:"successful_runs"`
	VerifySignatureStatus  string   `yaml:"verify_signature_status"`
	SourceRef              string   `yaml:"source_ref"`
	ValidDelegations       int      `yaml:"valid_delegations"`
	ReasonCodes            []string `yaml:"reason_codes"`
	ReasonCodesMustInclude []string `yaml:"reason_codes_must_include"`
}

type expectedVerdictLine struct {
	Index    int    `json:"index"`
	ToolName string `json:"tool_name"`
	Verdict  string `json:"verdict"`
	ExitCode int    `json:"exit_code"`
}

type gateEvalOutput struct {
	Verdict          string   `json:"verdict"`
	ReasonCodes      []string `json:"reason_codes"`
	SimulateMode     bool     `json:"simulate_mode"`
	ValidDelegations int      `json:"valid_delegations"`
}

type packVerifyOutput struct {
	SourceRef string `json:"source_ref"`
	Verify    struct {
		SignatureStatus string `json:"signature_status"`
		SourceRef       string `json:"source_ref"`
	} `json:"verify"`
}

func TestTier11Scenarios(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	repoRoot, err := findRepoRoot(cwd)
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	scenarioRoot := filepath.Join(repoRoot, scenarioRootRelativePath)
	binaryPath := buildGaitBinary(t, repoRoot)

	scenarioNames := make([]string, 0, len(requiredScenarioMinimumFiles))
	for name := range requiredScenarioMinimumFiles {
		scenarioNames = append(scenarioNames, name)
	}
	sort.Strings(scenarioNames)

	for _, name := range scenarioNames {
		name := name
		t.Run(name, func(t *testing.T) {
			scenarioPath := filepath.Join(scenarioRoot, name)
			runScenario(t, repoRoot, binaryPath, name, scenarioPath)
		})
	}
}

func runScenario(t *testing.T, repoRoot string, binaryPath string, name string, scenarioPath string) {
	switch name {
	case "policy-block-destructive", "policy-allow-safe-tools":
		runPolicyVerdictScenario(t, repoRoot, binaryPath, scenarioPath)
	case "dry-run-no-side-effects":
		runDryRunScenario(t, repoRoot, binaryPath, scenarioPath)
	case "concurrent-evaluation-10":
		runConcurrentScenario(t, repoRoot, binaryPath, scenarioPath)
	case "pack-integrity-round-trip":
		runPackScenario(t, repoRoot, binaryPath, scenarioPath)
	case "delegation-chain-depth-3":
		runDelegationScenario(t, repoRoot, binaryPath, scenarioPath)
	case "approval-expiry-1s-past", "approval-token-valid":
		runApprovalScenario(t, repoRoot, binaryPath, scenarioPath)
	default:
		t.Fatalf("unsupported scenario: %s", name)
	}
}

func runPolicyVerdictScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedVerdicts(t, filepath.Join(scenarioPath, "expected-verdicts.jsonl"))
	intents := readJSONLLines(t, filepath.Join(scenarioPath, "intents.jsonl"))
	policyPath := filepath.Join(scenarioPath, "policy.yaml")

	if len(expected) != len(intents) {
		t.Fatalf("expected/intents length mismatch: expected=%d intents=%d", len(expected), len(intents))
	}

	workDir := t.TempDir()
	for i := range expected {
		intentPath := filepath.Join(workDir, fmt.Sprintf("intent_%02d.json", i+1))
		if err := os.WriteFile(intentPath, []byte(intents[i]), 0o600); err != nil {
			t.Fatalf("write intent fixture %s: %v", intentPath, err)
		}
		output, code := mustRunCommand(t, workDir, binaryPath,
			"gate", "eval",
			"--policy", policyPath,
			"--intent", intentPath,
			"--json",
		)
		if code != expected[i].ExitCode {
			t.Fatalf("unexpected exit code for index=%d tool=%s: got=%d want=%d output=%s", expected[i].Index, expected[i].ToolName, code, expected[i].ExitCode, output)
		}
		var got gateEvalOutput
		if err := json.Unmarshal([]byte(output), &got); err != nil {
			t.Fatalf("parse gate output for index=%d: %v output=%s", expected[i].Index, err, output)
		}
		if got.Verdict != expected[i].Verdict {
			t.Fatalf("unexpected verdict for index=%d tool=%s: got=%s want=%s output=%s", expected[i].Index, expected[i].ToolName, got.Verdict, expected[i].Verdict, output)
		}
	}
}

func runDryRunScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedYAML(t, filepath.Join(scenarioPath, "expected.yaml"))
	flags := readScenarioFlags(t, filepath.Join(scenarioPath, "flags.yaml"))
	intentPath := filepath.Join(scenarioPath, "intent.json")
	policyPath := filepath.Join(scenarioPath, "policy.yaml")

	args := []string{"gate", "eval", "--policy", policyPath, "--intent", intentPath, "--json"}
	if flags.Simulate {
		args = append(args, "--simulate")
	}
	output, code := mustRunCommand(t, t.TempDir(), binaryPath, args...)
	if code != expected.ExitCode {
		t.Fatalf("unexpected exit code: got=%d want=%d output=%s", code, expected.ExitCode, output)
	}
	var got gateEvalOutput
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("parse gate output: %v output=%s", err, output)
	}
	if got.Verdict != expected.Verdict {
		t.Fatalf("unexpected verdict: got=%s want=%s output=%s", got.Verdict, expected.Verdict, output)
	}
	if got.SimulateMode != expected.SimulateMode {
		t.Fatalf("unexpected simulate_mode: got=%v want=%v output=%s", got.SimulateMode, expected.SimulateMode, output)
	}
	for _, required := range expected.ReasonCodes {
		if !contains(got.ReasonCodes, required) {
			t.Fatalf("missing required reason code %q in %v", required, got.ReasonCodes)
		}
	}
}

func runConcurrentScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedYAML(t, filepath.Join(scenarioPath, "expected.yaml"))
	flags := readScenarioFlags(t, filepath.Join(scenarioPath, "flags.yaml"))
	if flags.Concurrency <= 0 {
		flags.Concurrency = 1
	}

	intentPath := filepath.Join(scenarioPath, "intent.json")
	policyPath := filepath.Join(scenarioPath, "policy.yaml")
	baseWorkDir := t.TempDir()

	var wg sync.WaitGroup
	errCh := make(chan error, flags.Concurrency)
	for i := 0; i < flags.Concurrency; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			workDir := filepath.Join(baseWorkDir, fmt.Sprintf("run_%02d", i+1))
			if err := os.MkdirAll(workDir, 0o755); err != nil {
				errCh <- fmt.Errorf("mkdir workdir: %w", err)
				return
			}
			output, code, err := runCommand(workDir, binaryPath,
				"gate", "eval",
				"--policy", policyPath,
				"--intent", intentPath,
				"--json",
			)
			if err != nil {
				errCh <- fmt.Errorf("run gate eval run=%d: %w", i+1, err)
				return
			}
			if code != expected.ExitCode {
				errCh <- fmt.Errorf("unexpected exit code run=%d got=%d want=%d output=%s", i+1, code, expected.ExitCode, output)
				return
			}
			var got gateEvalOutput
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				errCh <- fmt.Errorf("parse gate output run=%d: %w output=%s", i+1, err, output)
				return
			}
			if got.Verdict != expected.Verdict {
				errCh <- fmt.Errorf("unexpected verdict run=%d got=%s want=%s", i+1, got.Verdict, expected.Verdict)
				return
			}
		}()
	}
	wg.Wait()
	close(errCh)

	failures := []string{}
	for err := range errCh {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		t.Fatalf("concurrent evaluation failures: %v", failures)
	}
	if expected.SuccessfulRuns > 0 && flags.Concurrency != expected.SuccessfulRuns {
		t.Fatalf("unexpected concurrency run count: got=%d want=%d", flags.Concurrency, expected.SuccessfulRuns)
	}
}

func runPackScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedYAML(t, filepath.Join(scenarioPath, "expected.yaml"))
	workDir := t.TempDir()

	_, demoCode := mustRunCommand(t, workDir, binaryPath, "demo", "--json")
	if demoCode != 0 {
		t.Fatalf("demo failed with exit code %d", demoCode)
	}
	packPath := filepath.Join(workDir, "scenario-pack.zip")
	buildOutput, buildCode := mustRunCommand(t, workDir, binaryPath,
		"pack", "build",
		"--type", "run",
		"--from", "run_demo",
		"--out", packPath,
		"--json",
	)
	if buildCode != 0 {
		t.Fatalf("pack build failed: code=%d output=%s", buildCode, buildOutput)
	}
	verifyOutput, verifyCode := mustRunCommand(t, workDir, binaryPath,
		"pack", "verify",
		packPath,
		"--json",
	)
	if verifyCode != expected.ExitCode {
		t.Fatalf("pack verify exit mismatch: got=%d want=%d output=%s", verifyCode, expected.ExitCode, verifyOutput)
	}

	var got packVerifyOutput
	if err := json.Unmarshal([]byte(verifyOutput), &got); err != nil {
		t.Fatalf("parse pack verify output: %v output=%s", err, verifyOutput)
	}
	if expected.VerifySignatureStatus != "" && got.Verify.SignatureStatus != expected.VerifySignatureStatus {
		t.Fatalf("unexpected signature status: got=%s want=%s", got.Verify.SignatureStatus, expected.VerifySignatureStatus)
	}
	sourceRef := got.SourceRef
	if sourceRef == "" {
		sourceRef = got.Verify.SourceRef
	}
	if expected.SourceRef != "" && sourceRef != expected.SourceRef {
		t.Fatalf("unexpected source_ref: got=%s want=%s", sourceRef, expected.SourceRef)
	}
}

func runDelegationScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedYAML(t, filepath.Join(scenarioPath, "expected.yaml"))
	flags := readScenarioFlags(t, filepath.Join(scenarioPath, "flags.yaml"))

	chain := make([]string, 0, len(flags.DelegationChainFiles))
	for _, rel := range flags.DelegationChainFiles {
		chain = append(chain, filepath.Join(scenarioPath, rel))
	}
	output, code := mustRunCommand(t, t.TempDir(), binaryPath,
		"gate", "eval",
		"--policy", filepath.Join(scenarioPath, "policy.yaml"),
		"--intent", filepath.Join(scenarioPath, "intent.json"),
		"--delegation-token", filepath.Join(scenarioPath, "delegation-token-1.json"),
		"--delegation-token-chain", strings.Join(chain, ","),
		"--delegation-public-key", filepath.Join(scenarioPath, "delegation_public.key"),
		"--json",
	)
	if code != expected.ExitCode {
		t.Fatalf("delegation scenario exit mismatch: got=%d want=%d output=%s", code, expected.ExitCode, output)
	}
	var got gateEvalOutput
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("parse delegation output: %v output=%s", err, output)
	}
	if got.Verdict != expected.Verdict {
		t.Fatalf("unexpected delegation verdict: got=%s want=%s output=%s", got.Verdict, expected.Verdict, output)
	}
	if expected.ValidDelegations > 0 && got.ValidDelegations != expected.ValidDelegations {
		t.Fatalf("unexpected valid_delegations: got=%d want=%d", got.ValidDelegations, expected.ValidDelegations)
	}
	for _, required := range expected.ReasonCodesMustInclude {
		if !contains(got.ReasonCodes, required) {
			t.Fatalf("missing required reason code %q in %v", required, got.ReasonCodes)
		}
	}
}

func runApprovalScenario(t *testing.T, repoRoot string, binaryPath string, scenarioPath string) {
	expected := readExpectedYAML(t, filepath.Join(scenarioPath, "expected.yaml"))
	output, code := mustRunCommand(t, t.TempDir(), binaryPath,
		"gate", "eval",
		"--policy", filepath.Join(scenarioPath, "policy.yaml"),
		"--intent", filepath.Join(scenarioPath, "intent.json"),
		"--approval-token", filepath.Join(scenarioPath, "approval-token.json"),
		"--approval-public-key", filepath.Join(scenarioPath, "approval_public.key"),
		"--json",
	)
	if code != expected.ExitCode {
		t.Fatalf("approval scenario exit mismatch: got=%d want=%d output=%s", code, expected.ExitCode, output)
	}
	var got gateEvalOutput
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("parse approval output: %v output=%s", err, output)
	}
	if got.Verdict != expected.Verdict {
		t.Fatalf("unexpected approval verdict: got=%s want=%s output=%s", got.Verdict, expected.Verdict, output)
	}
	for _, required := range expected.ReasonCodesMustInclude {
		if !contains(got.ReasonCodes, required) {
			t.Fatalf("missing required reason code %q in %v", required, got.ReasonCodes)
		}
	}
}

func buildGaitBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	if prebuilt := strings.TrimSpace(os.Getenv("GAIT_SCENARIO_BIN")); prebuilt != "" {
		if info, err := os.Stat(prebuilt); err == nil && !info.IsDir() {
			return prebuilt
		}
		t.Fatalf("GAIT_SCENARIO_BIN does not point to a valid file: %s", prebuilt)
	}
	binaryPath := filepath.Join(t.TempDir(), "gait")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gait")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build gait binary: %v output=%s", err, string(output))
	}
	return binaryPath
}

func mustRunCommand(t *testing.T, workDir string, binaryPath string, args ...string) (string, int) {
	t.Helper()
	output, code, err := runCommand(workDir, binaryPath, args...)
	if err != nil {
		t.Fatalf("run command %v: %v", args, err)
	}
	return output, code
}

func runCommand(workDir string, binaryPath string, args ...string) (string, int, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return strings.TrimSpace(string(output)), exitErr.ExitCode(), nil
	}
	return strings.TrimSpace(string(output)), -1, fmt.Errorf("%w output=%s", err, string(output))
}

func readScenarioFlags(t *testing.T, path string) scenarioFlags {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read flags file %s: %v", path, err)
	}
	var flags scenarioFlags
	if err := yaml.Unmarshal(payload, &flags); err != nil {
		t.Fatalf("parse flags file %s: %v", path, err)
	}
	return flags
}

func readExpectedYAML(t *testing.T, path string) expectedYAML {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read expected yaml %s: %v", path, err)
	}
	var expected expectedYAML
	if err := yaml.Unmarshal(payload, &expected); err != nil {
		t.Fatalf("parse expected yaml %s: %v", path, err)
	}
	return expected
}

func readExpectedVerdicts(t *testing.T, path string) []expectedVerdictLine {
	t.Helper()
	lines := readJSONLLines(t, path)
	out := make([]expectedVerdictLine, 0, len(lines))
	for _, line := range lines {
		var item expectedVerdictLine
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("parse expected verdict line in %s: %v line=%s", path, err, line)
		}
		out = append(out, item)
	}
	return out
}

func readJSONLLines(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl file %s: %v", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl file %s: %v", path, err)
	}
	return lines
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}
