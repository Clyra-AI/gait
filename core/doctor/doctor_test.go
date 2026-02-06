package doctor

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

	if result.Status == statusFail {
		t.Fatalf("expected non-failing status, got: %s (%s)", result.Status, result.Summary)
	}
	if result.NonFixable {
		t.Fatalf("expected non-fixable to be false")
	}
	if len(result.Checks) != 12 {
		t.Fatalf("unexpected checks count: %d", len(result.Checks))
	}
	if !checkStatus(result.Checks, "key_permissions", statusPass) {
		t.Fatalf("expected key_permissions pass check")
	}
	if !checkStatus(result.Checks, "onboarding_binary", statusPass) &&
		!checkStatus(result.Checks, "onboarding_binary", statusWarn) {
		t.Fatalf("expected onboarding_binary pass or warn check")
	}
	if !checkStatus(result.Checks, "onboarding_assets", statusPass) {
		t.Fatalf("expected onboarding_assets pass check")
	}
	if !checkStatus(result.Checks, "key_source_ambiguity", statusPass) {
		t.Fatalf("expected key_source_ambiguity pass check")
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
	check = checkKeySourceAmbiguity(sign.KeyConfig{PrivateKeyPath: "a", PrivateKeyEnv: "KEY"})
	if check.Status != statusFail {
		t.Fatalf("key source ambiguity should fail: %#v", check)
	}
	check = checkTempDirWritable()
	if check.Status != statusPass {
		t.Fatalf("temp dir check should pass: %#v", check)
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
	if runtime.GOOS == "windows" && check.Status != statusPass && check.Status != statusWarn {
		t.Fatalf("expected key permission pass/warn on windows for writable key file: %#v", check)
	}
	if runtime.GOOS != "windows" && check.Status != statusWarn {
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
	if runtime.GOOS == "windows" {
		if check.Status != statusPass {
			t.Fatalf("expected onboarding assets pass on windows, got %#v", check)
		}
	} else if check.Status != statusWarn || !strings.Contains(check.FixCommand, "chmod +x scripts/quickstart.sh") {
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

func TestHardeningDoctorChecks(t *testing.T) {
	outputDir := t.TempDir()
	lockPath := filepath.Join(outputDir, "gate_rate_limits.json.lock")
	if err := os.WriteFile(lockPath, []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	staleTime := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("set stale lock time: %v", err)
	}
	lockCheck := checkRateLimitLock(outputDir)
	if lockCheck.Status != statusWarn {
		t.Fatalf("expected stale lock warning, got %#v", lockCheck)
	}

	hookCheck := checkHooksPath(t.TempDir())
	if hookCheck.Status != statusWarn && hookCheck.Status != statusPass {
		t.Fatalf("unexpected hooks check status: %#v", hookCheck)
	}

	registryCheck := checkRegistryCacheHealth()
	if registryCheck.Status != statusWarn && registryCheck.Status != statusPass {
		t.Fatalf("unexpected registry cache check status: %#v", registryCheck)
	}
}

func TestCheckHooksPathScenarios(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...) // #nosec G204 -- static test command.
		command.Dir = repoDir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(output))
		}
	}
	run("init")
	run("config", "core.hooksPath", ".githooks")
	if err := os.MkdirAll(filepath.Join(repoDir, ".githooks"), 0o750); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	check := checkHooksPath(repoDir)
	if check.Status != statusPass {
		t.Fatalf("expected hooks path pass, got %#v", check)
	}
	run("config", "core.hooksPath", "hooks")
	check = checkHooksPath(repoDir)
	if check.Status != statusWarn {
		t.Fatalf("expected hooks path warning for mismatch, got %#v", check)
	}
}

func TestCheckRegistryCacheHealthScenarios(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cacheDir := filepath.Join(home, ".gait", "registry")

	check := checkRegistryCacheHealth()
	if check.Status != statusWarn {
		t.Fatalf("expected uninitialized cache warning, got %#v", check)
	}

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o750); err != nil {
		t.Fatalf("mkdir cache parent: %v", err)
	}
	if err := os.WriteFile(cacheDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file cache path: %v", err)
	}
	check = checkRegistryCacheHealth()
	if check.Status != statusFail {
		t.Fatalf("expected non-directory cache failure, got %#v", check)
	}
	if err := os.Remove(cacheDir); err != nil {
		t.Fatalf("remove file cache path: %v", err)
	}

	pinDigest := strings.Repeat("a", 64)
	pinDir := filepath.Join(cacheDir, "pins")
	if err := os.MkdirAll(pinDir, 0o750); err != nil {
		t.Fatalf("mkdir pins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pinDir, "pack.pin"), []byte("sha256:"+pinDigest+"\n"), 0o600); err != nil {
		t.Fatalf("write pin: %v", err)
	}
	check = checkRegistryCacheHealth()
	if check.Status != statusWarn {
		t.Fatalf("expected inconsistent pin warning, got %#v", check)
	}

	metadataPath := filepath.Join(cacheDir, "pack", "1.0.0", pinDigest, "registry_pack.json")
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o750); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metadataPath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	check = checkRegistryCacheHealth()
	if check.Status != statusPass {
		t.Fatalf("expected healthy cache pass, got %#v", check)
	}
}

func TestCheckRateLimitLockBranches(t *testing.T) {
	outputDir := t.TempDir()
	check := checkRateLimitLock(outputDir)
	if check.Status != statusPass {
		t.Fatalf("expected pass when no lock exists, got %#v", check)
	}
	lockPath := filepath.Join(outputDir, "gate_rate_limits.json.lock")
	if err := os.MkdirAll(lockPath, 0o750); err != nil {
		t.Fatalf("mkdir lock path: %v", err)
	}
	check = checkRateLimitLock(outputDir)
	if check.Status != statusWarn {
		t.Fatalf("expected warning for lock directory, got %#v", check)
	}
}

func TestCheckTempDirWritableFailure(t *testing.T) {
	tempDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		t.Fatalf("mkdir temp dir: %v", err)
	}
	if err := os.Chmod(tempDir, 0o500); err != nil {
		t.Fatalf("chmod temp dir readonly: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(tempDir, 0o700)
	})
	t.Setenv("TMPDIR", tempDir)
	t.Setenv("TMP", tempDir)
	t.Setenv("TEMP", tempDir)

	check := checkTempDirWritable()
	if runtime.GOOS != "windows" && check.Status != statusFail {
		t.Fatalf("expected temp dir writable failure, got %#v", check)
	}
	if runtime.GOOS == "windows" && check.Status != statusFail && check.Status != statusPass {
		t.Fatalf("unexpected temp dir check status on windows: %#v", check)
	}
}

func TestCheckHooksPathGitUnavailable(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".githooks"), 0o750); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	originalPath := os.Getenv("PATH")
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)
	check := checkHooksPath(repoDir)
	if check.Status != statusWarn {
		t.Fatalf("expected hooks warning without git, got %#v", check)
	}
	if originalPath != "" {
		t.Logf("original PATH length=%d", len(originalPath))
	}
}

func TestCheckRegistryCacheHealthGlobErrorPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cacheDir := filepath.Join(home, ".gait", "registry")
	pinsDir := filepath.Join(cacheDir, "pins")
	if err := os.MkdirAll(pinsDir, 0o750); err != nil {
		t.Fatalf("mkdir pins dir: %v", err)
	}
	badPinPath := filepath.Join(pinsDir, "bad.pin")
	if err := os.WriteFile(badPinPath, []byte(fmt.Sprintf("sha256:%s\n", strings.Repeat("z", 64))), 0o600); err != nil {
		t.Fatalf("write bad pin: %v", err)
	}
	check := checkRegistryCacheHealth()
	if check.Status != statusWarn {
		t.Fatalf("expected warning for bad pin digest, got %#v", check)
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
