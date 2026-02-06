package doctor

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/davidahmann/gait/core/sign"
)

func TestRunDetectsMissingSchemasAsNonFixable(t *testing.T) {
	workDir := t.TempDir()
	result := Run(Options{
		WorkDir:         workDir,
		OutputDir:       filepath.Join(workDir, "gait-out"),
		ProducerVersion: "test",
		KeyMode:         sign.ModeDev,
	})

	if result.Status != statusFail {
		t.Fatalf("expected fail status, got: %s", result.Status)
	}
	if !result.NonFixable {
		t.Fatalf("expected non-fixable result")
	}
	if !checkStatus(result.Checks, "schema_files", statusFail) {
		t.Fatalf("expected schema_files fail check")
	}
}

func TestRunPassesWithValidWorkspaceAndSchemas(t *testing.T) {
	root := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "gait-out")
	if err := ensureDir(outputDir); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	result := Run(Options{
		WorkDir:         root,
		OutputDir:       outputDir,
		ProducerVersion: "test",
		KeyMode:         sign.ModeDev,
	})

	if result.Status != statusPass {
		t.Fatalf("expected pass status, got: %s (%s)", result.Status, result.Summary)
	}
	if result.NonFixable {
		t.Fatalf("expected non-fixable to be false")
	}
	if len(result.Checks) != 7 {
		t.Fatalf("unexpected checks count: %d", len(result.Checks))
	}
	if !checkStatus(result.Checks, "key_permissions", statusPass) {
		t.Fatalf("expected key_permissions pass check")
	}
	if !checkStatus(result.Checks, "onboarding_binary", statusPass) {
		t.Fatalf("expected onboarding_binary pass check")
	}
	if !checkStatus(result.Checks, "onboarding_assets", statusPass) {
		t.Fatalf("expected onboarding_assets pass check")
	}
}

func TestRunDetectsProdKeyConfigFailure(t *testing.T) {
	root := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "gait-out")
	if err := ensureDir(outputDir); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	result := Run(Options{
		WorkDir:         root,
		OutputDir:       outputDir,
		ProducerVersion: "test",
		KeyMode:         sign.ModeProd,
	})

	if result.Status != statusFail {
		t.Fatalf("expected fail status for prod key failure, got: %s", result.Status)
	}
	if result.NonFixable {
		t.Fatalf("expected fixable failure for key config")
	}
	if !checkStatus(result.Checks, "key_config", statusFail) {
		t.Fatalf("expected key_config fail check")
	}
}

func TestDoctorHelperBranches(t *testing.T) {
	workDir := t.TempDir()
	if got := shellQuote(""); got != "''" {
		t.Fatalf("shellQuote empty mismatch: %s", got)
	}
	if got := shellQuote("a'b"); got != "'a'\\''b'" {
		t.Fatalf("shellQuote quote mismatch: %s", got)
	}
	if hasAnyKeySource(sign.KeyConfig{}) {
		t.Fatalf("expected no key sources")
	}
	if !hasAnyKeySource(sign.KeyConfig{PrivateKeyPath: "key"}) {
		t.Fatalf("expected key source detection")
	}
	if hasAnyVerifySource(sign.KeyConfig{}) {
		t.Fatalf("expected no verify sources")
	}
	if !hasAnyVerifySource(sign.KeyConfig{PublicKeyEnv: "KEY"}) {
		t.Fatalf("expected verify source detection")
	}

	filePath := filepath.Join(workDir, "out-file")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file path: %v", err)
	}
	check := checkOutputDir(filePath)
	if check.Status != statusFail {
		t.Fatalf("checkOutputDir file should fail, got %s", check.Status)
	}

	missingDir := filepath.Join(workDir, "missing")
	check = checkOutputDir(missingDir)
	if check.Status != statusWarn || !strings.Contains(check.FixCommand, "mkdir -p") {
		t.Fatalf("checkOutputDir missing should warn with mkdir command: %#v", check)
	}

	check = checkWorkDirWritable(filepath.Join(workDir, "missing-workdir"))
	if check.Status != statusFail {
		t.Fatalf("checkWorkDirWritable missing should fail: %#v", check)
	}

	check = checkKeyConfig(sign.KeyMode("unknown"), sign.KeyConfig{})
	if check.Status != statusFail {
		t.Fatalf("checkKeyConfig unknown mode should fail: %#v", check)
	}

	check = checkKeyConfig(sign.ModeDev, sign.KeyConfig{PrivateKeyPath: "x"})
	if check.Status != statusWarn {
		t.Fatalf("dev mode with key source should warn: %#v", check)
	}

	check = checkKeyConfig(sign.ModeProd, sign.KeyConfig{PrivateKeyPath: "/missing"})
	if check.Status != statusFail {
		t.Fatalf("prod mode invalid key should fail: %#v", check)
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	privateKeyPath := filepath.Join(workDir, "private.key")
	publicKeyPath := filepath.Join(workDir, "public.key")
	if err := os.WriteFile(privateKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Private)), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(publicKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	check = checkKeyConfig(sign.ModeProd, sign.KeyConfig{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
	})
	if check.Status != statusPass {
		t.Fatalf("prod mode valid keys should pass: %#v", check)
	}

	if err := os.Chmod(privateKeyPath, 0o666); err != nil {
		t.Fatalf("chmod private key: %v", err)
	}
	check = checkKeyFilePermissions(sign.KeyConfig{PrivateKeyPath: privateKeyPath})
	if check.Status != statusWarn {
		t.Fatalf("expected key permission warning for writable key file: %#v", check)
	}
	if err := os.Chmod(privateKeyPath, 0o600); err != nil {
		t.Fatalf("restore private key mode: %v", err)
	}
	check = checkKeyFilePermissions(sign.KeyConfig{PrivateKeyPath: privateKeyPath})
	if check.Status != statusPass {
		t.Fatalf("expected strict key permissions to pass: %#v", check)
	}
}

func TestOnboardingChecks(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("PATH", "")

	check := checkOnboardingBinary(workDir)
	if check.Status != statusWarn {
		t.Fatalf("expected onboarding binary warning, got %#v", check)
	}
	if !strings.Contains(check.FixCommand, "go build -o ./gait ./cmd/gait") {
		t.Fatalf("unexpected binary fix command: %#v", check)
	}

	check = checkOnboardingAssets(workDir)
	if check.Status != statusWarn {
		t.Fatalf("expected onboarding assets warning, got %#v", check)
	}
	if !strings.Contains(check.FixCommand, "git restore --source=HEAD -- scripts/quickstart.sh examples/integrations") {
		t.Fatalf("unexpected assets fix command: %#v", check)
	}

	quickstartPath := filepath.Join(workDir, "scripts", "quickstart.sh")
	if err := os.MkdirAll(filepath.Dir(quickstartPath), 0o750); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	if err := os.WriteFile(quickstartPath, []byte("#!/usr/bin/env bash\n"), 0o600); err != nil {
		t.Fatalf("write quickstart: %v", err)
	}
	for _, relativePath := range []string{
		"examples/integrations/openai_agents/quickstart.py",
		"examples/integrations/langchain/quickstart.py",
		"examples/integrations/autogen/quickstart.py",
	} {
		fullPath := filepath.Join(workDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			t.Fatalf("mkdir integration dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("print('ok')\n"), 0o600); err != nil {
			t.Fatalf("write integration quickstart: %v", err)
		}
	}

	check = checkOnboardingAssets(workDir)
	if check.Status != statusWarn || !strings.Contains(check.FixCommand, "chmod +x scripts/quickstart.sh") {
		t.Fatalf("expected onboarding quickstart chmod warning, got %#v", check)
	}

	if err := os.Chmod(quickstartPath, 0o755); err != nil {
		t.Fatalf("chmod quickstart: %v", err)
	}
	check = checkOnboardingAssets(workDir)
	if check.Status != statusPass {
		t.Fatalf("expected onboarding assets pass, got %#v", check)
	}
}

func checkStatus(checks []Check, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o750)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	dir := filepath.Dir(filename)
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}
