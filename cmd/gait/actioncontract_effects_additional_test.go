package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/actioncontract"
)

const cliTestDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func writeCLIJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCLIKey(t *testing.T, dir string) (string, string) {
	t.Helper()
	seed := sha256.Sum256([]byte("gait-cli-additional-test-key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	privatePath := filepath.Join(dir, "private.key")
	publicPath := filepath.Join(dir, "public.key")
	if err := os.WriteFile(privatePath, []byte(base64.StdEncoding.EncodeToString(private)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))), 0o600); err != nil {
		t.Fatal(err)
	}
	return privatePath, publicPath
}

func TestActionContractAdvisoryCLIEvaluateVerifyAndErrors(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	privatePath, publicPath := writeCLIKey(t, dir)
	inputPath := filepath.Join(dir, "advisory-input.json")
	reportPath := filepath.Join(dir, "advisory-report.json")
	writeCLIJSON(t, inputPath, actioncontract.AdvisoryInput{Claims: []string{"review"}})
	output, code := captureEffectsOutput(t, func() int {
		return runActionContractAdvisory([]string{"evaluate", "--input", inputPath, "--out", reportPath, "--private-key", privatePath, "--action-id", "action:cli", "--contract-digest", cliTestDigest, "--correlation-digest", cliTestDigest, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("advisory evaluate: code=%d output=%s", code, output)
	}
	output, code = captureEffectsOutput(t, func() int {
		return runActionContractAdvisory([]string{"verify", "--report", reportPath, "--trusted-key", publicPath, "--expected-contract-digest", cliTestDigest, "--expected-correlation-digest", cliTestDigest, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("advisory verify: code=%d output=%s", code, output)
	}
	if code := runActionContractAdvisory([]string{"verify", "--report", reportPath, "--trusted-key", publicPath, "--expected-contract-digest", "sha256:" + strings.Repeat("a", 64)}); code != exitVerifyFailed {
		t.Fatalf("advisory binding mismatch: got %d", code)
	}
	if code := runActionContractAdvisory([]string{"evaluate", "--input", filepath.Join(dir, "missing"), "--out", reportPath, "--private-key", privatePath, "--action-id", "action:cli"}); code != exitInvalidInput {
		t.Fatalf("advisory missing input: got %d", code)
	}
	badInput := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badInput, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractAdvisory([]string{"evaluate", "--input", badInput, "--out", reportPath, "--private-key", privatePath, "--action-id", "action:cli", "--json"}); code != exitInvalidInput {
		t.Fatalf("advisory malformed input: got %d", code)
	}
	if code := runActionContractAdvisory([]string{"--help"}); code != exitOK {
		t.Fatalf("advisory help: got %d", code)
	}
}

func TestEffectsObserveCaptureCLIAndErrors(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resourcePath := filepath.Join(dir, "resource.txt")
	if err := os.WriteFile(resourcePath, []byte("effect"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforePath := filepath.Join(dir, "before.json")
	afterPath := filepath.Join(dir, "after.json")
	for _, tc := range []struct {
		out string
		at  string
	}{
		{beforePath, "2026-08-20T00:00:00Z"},
		{afterPath, "2026-08-20T00:00:01Z"},
	} {
		output, code := captureEffectsOutput(t, func() int {
			return runEffects([]string{"observe", "--resource", "filesystem", "--path", resourcePath, "--out", tc.out, "--observed-at", tc.at, "--json"})
		})
		if code != exitOK || !strings.Contains(output, `"ok":true`) {
			t.Fatalf("effects observe: code=%d output=%s", code, output)
		}
	}
	privatePath, _ := writeCLIKey(t, dir)
	snapshotPath := filepath.Join(dir, "snapshot.json")
	output, code := captureEffectsOutput(t, func() int {
		return runEffects([]string{"capture", "--resource", "filesystem", "--path", resourcePath, "--before-observation", beforePath, "--after-observation", afterPath, "--private-key", privatePath, "--out", snapshotPath, "--action-digest", cliTestDigest, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"ok":true`) {
		t.Fatalf("effects capture: code=%d output=%s", code, output)
	}
	if code := runEffects([]string{"observe", "--resource", "filesystem", "--out", filepath.Join(dir, "missing.json")}); code != exitInvalidInput {
		t.Fatalf("effects observe missing path: got %d", code)
	}
	if code := runEffects([]string{"observe", "--resource", "filesystem", "--path", resourcePath, "--out", filepath.Join(dir, "bad-time.json"), "--observed-at", "bad"}); code != exitInvalidInput {
		t.Fatalf("effects observe bad timestamp: got %d", code)
	}
	if code := runEffects([]string{"capture", "--resource", "filesystem", "--path", resourcePath, "--before-observation", beforePath, "--after-observation", afterPath, "--private-key", privatePath, "--out", snapshotPath, "--action-digest", cliTestDigest}); code != exitInternalFailure {
		t.Fatalf("effects capture existing output: got %d", code)
	}
}

func TestActionContractOTelCLIValidationAndOutputBranches(t *testing.T) {
	if code := runActionContractOTel([]string{"--json"}); code != exitInvalidInput {
		t.Fatalf("otel missing flags: got %d", code)
	}
	if code := writeOTelCLI(false, "", exitOK); code != exitOK {
		t.Fatalf("otel success output: got %d", code)
	}
	if code := writeOTelCLI(true, "output failed", exitInternalFailure); code != exitInternalFailure {
		t.Fatalf("otel error output: got %d", code)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lifecyclePath := filepath.Join(dir, "lifecycle.json")
	if err := os.WriteFile(lifecyclePath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractOTel([]string{"--lifecycle", lifecyclePath, "--otel-out", filepath.Join(dir, "otel.jsonl"), "--trusted-key", filepath.Join(dir, "missing.key"), "--json"}); code != exitVerifyFailed {
		t.Fatalf("otel missing key: got %d", code)
	}
	if code := runActionContractOTel([]string{"--lifecycle", lifecyclePath, "--otel-out", filepath.Join(dir, "otel.jsonl"), "--trusted-key", filepath.Join(dir, "missing.key"), "--source-version", ""}); code != exitInvalidInput {
		t.Fatalf("otel empty version: got %d", code)
	}
}

func TestActionContractChainCLIBranches(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(dir, "policy.json")
	statePath := filepath.Join(dir, "state.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	writeCLIJSON(t, policyPath, actioncontract.ChainPolicy{MaxSteps: 2})
	writeCLIJSON(t, statePath, actioncontract.ChainState{StepIDs: []string{}, Classes: []string{}, Targets: []string{}})
	writeCLIJSON(t, candidatePath, actioncontract.ChainStep{ID: "step-1", Classes: []string{"read"}, Target: "repo"})
	outputPath := filepath.Join(dir, "decision.json")
	output, code := captureEffectsOutput(t, func() int {
		return runActionContractChain([]string{"evaluate", "--policy", policyPath, "--state", statePath, "--candidate", candidatePath, "--out", outputPath, "--json", "--explain"})
	})
	if code != exitOK || !strings.Contains(output, `"allowed":true`) {
		t.Fatalf("chain allow: code=%d output=%s", code, output)
	}
	writeCLIJSON(t, policyPath, actioncontract.ChainPolicy{ForbiddenClasses: []string{"delete"}})
	writeCLIJSON(t, candidatePath, actioncontract.ChainStep{ID: "step-2", Classes: []string{"delete"}, Target: "repo"})
	if code := runActionContractChain([]string{"evaluate", "--policy", policyPath, "--state", statePath, "--candidate", candidatePath, "--json"}); code != exitVerifyFailed {
		t.Fatalf("chain deny: got %d", code)
	}
	if code := runActionContractChain([]string{"evaluate", "--help"}); code != exitOK {
		t.Fatalf("chain help: got %d", code)
	}
	if code := runActionContractChain([]string{"evaluate", policyPath}); code != exitInvalidInput {
		t.Fatalf("chain positional: got %d", code)
	}
}
