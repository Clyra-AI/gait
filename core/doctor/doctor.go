package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/sign"
)

const (
	statusPass = "pass"
	statusWarn = "warn"
	statusFail = "fail"
)

type Options struct {
	WorkDir         string
	OutputDir       string
	ProducerVersion string
	KeyMode         sign.KeyMode
	KeyConfig       sign.KeyConfig
}

type Result struct {
	SchemaID        string   `json:"schema_id"`
	SchemaVersion   string   `json:"schema_version"`
	CreatedAt       string   `json:"created_at"`
	ProducerVersion string   `json:"producer_version"`
	Status          string   `json:"status"`
	NonFixable      bool     `json:"non_fixable"`
	Summary         string   `json:"summary"`
	FixCommands     []string `json:"fix_commands"`
	Checks          []Check  `json:"checks"`
}

type Check struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	FixCommand string `json:"fix_command,omitempty"`
	NonFixable bool   `json:"non_fixable,omitempty"`
}

var requiredSchemaPaths = []string{
	"schemas/v1/runpack/manifest.schema.json",
	"schemas/v1/runpack/run.schema.json",
	"schemas/v1/runpack/intent.schema.json",
	"schemas/v1/runpack/result.schema.json",
	"schemas/v1/runpack/refs.schema.json",
	"schemas/v1/gate/intent_request.schema.json",
	"schemas/v1/gate/gate_result.schema.json",
	"schemas/v1/gate/trace_record.schema.json",
	"schemas/v1/gate/approval_token.schema.json",
	"schemas/v1/gate/approval_audit_record.schema.json",
	"schemas/v1/gate/broker_credential_record.schema.json",
	"schemas/v1/policytest/policy_test_result.schema.json",
	"schemas/v1/regress/regress_result.schema.json",
	"schemas/v1/scout/inventory_snapshot.schema.json",
	"schemas/v1/guard/pack_manifest.schema.json",
	"schemas/v1/registry/registry_pack.schema.json",
	"schemas/v1/scout/adoption_event.schema.json",
	"schemas/v1/scout/operational_event.schema.json",
}

var requiredOnboardingPaths = []string{
	"scripts/quickstart.sh",
	"examples/integrations/openai_agents/quickstart.py",
	"examples/integrations/langchain/quickstart.py",
	"examples/integrations/autogen/quickstart.py",
}

func Run(opts Options) Result {
	workDir := strings.TrimSpace(opts.WorkDir)
	if workDir == "" {
		workDir = "."
	}
	outputDir := strings.TrimSpace(opts.OutputDir)
	if outputDir == "" {
		outputDir = "./gait-out"
	}
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(workDir, outputDir)
	}

	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	checks := []Check{
		checkWorkDirWritable(workDir),
		checkOutputDir(outputDir),
		checkTempDirWritable(),
		checkSchemaFiles(workDir),
		checkHooksPath(workDir),
		checkRegistryCacheHealth(),
		checkRateLimitLock(outputDir),
		checkOnboardingBinary(workDir),
		checkOnboardingAssets(workDir),
		checkKeySourceAmbiguity(opts.KeyConfig),
		checkKeyFilePermissions(opts.KeyConfig),
		checkKeyConfig(opts.KeyMode, opts.KeyConfig),
	}

	failed := 0
	warned := 0
	nonFixable := false
	fixCommands := make([]string, 0, len(checks))
	seenFixes := map[string]struct{}{}
	for _, check := range checks {
		switch check.Status {
		case statusFail:
			failed++
		case statusWarn:
			warned++
		}
		if check.NonFixable {
			nonFixable = true
		}
		if check.FixCommand != "" {
			if _, ok := seenFixes[check.FixCommand]; !ok {
				seenFixes[check.FixCommand] = struct{}{}
				fixCommands = append(fixCommands, check.FixCommand)
			}
		}
	}

	status := statusPass
	if failed > 0 {
		status = statusFail
	} else if warned > 0 {
		status = statusWarn
	}

	sort.Strings(fixCommands)
	summary := fmt.Sprintf("doctor: status=%s failed=%d warned=%d non_fixable=%t", status, failed, warned, nonFixable)

	return Result{
		SchemaID:        "gait.doctor.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		ProducerVersion: producerVersion,
		Status:          status,
		NonFixable:      nonFixable,
		Summary:         summary,
		FixCommands:     fixCommands,
		Checks:          checks,
	}
}

func checkWorkDirWritable(workDir string) Check {
	info, err := os.Stat(workDir)
	if err != nil {
		return Check{
			Name:       "workdir",
			Status:     statusFail,
			Message:    fmt.Sprintf("workdir not accessible: %v", err),
			FixCommand: fmt.Sprintf("mkdir -p %s", shellQuote(workDir)),
		}
	}
	if !info.IsDir() {
		return Check{
			Name:       "workdir",
			Status:     statusFail,
			Message:    "workdir is not a directory",
			FixCommand: fmt.Sprintf("mkdir -p %s", shellQuote(workDir)),
		}
	}
	testPath := filepath.Join(workDir, ".gait-doctor-writecheck")
	if err := os.WriteFile(testPath, []byte("ok"), 0o600); err != nil {
		return Check{
			Name:       "workdir",
			Status:     statusFail,
			Message:    fmt.Sprintf("workdir not writable: %v", err),
			FixCommand: fmt.Sprintf("chmod u+w %s", shellQuote(workDir)),
		}
	}
	_ = os.Remove(testPath)
	return Check{
		Name:    "workdir",
		Status:  statusPass,
		Message: "workdir is writable",
	}
}

func checkOutputDir(outputDir string) Check {
	info, err := os.Stat(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:       "output_dir",
				Status:     statusWarn,
				Message:    "output directory does not exist",
				FixCommand: fmt.Sprintf("mkdir -p %s", shellQuote(outputDir)),
			}
		}
		return Check{
			Name:    "output_dir",
			Status:  statusFail,
			Message: fmt.Sprintf("output directory check failed: %v", err),
		}
	}
	if !info.IsDir() {
		return Check{
			Name:    "output_dir",
			Status:  statusFail,
			Message: "output path is not a directory",
		}
	}
	testPath := filepath.Join(outputDir, ".gait-doctor-writecheck")
	if err := os.WriteFile(testPath, []byte("ok"), 0o600); err != nil {
		return Check{
			Name:       "output_dir",
			Status:     statusFail,
			Message:    fmt.Sprintf("output directory not writable: %v", err),
			FixCommand: fmt.Sprintf("chmod u+w %s", shellQuote(outputDir)),
		}
	}
	_ = os.Remove(testPath)
	return Check{
		Name:    "output_dir",
		Status:  statusPass,
		Message: "output directory is writable",
	}
}

func checkSchemaFiles(workDir string) Check {
	missing := make([]string, 0, len(requiredSchemaPaths))
	for _, relativePath := range requiredSchemaPaths {
		fullPath := filepath.Join(workDir, filepath.FromSlash(relativePath))
		if _, err := os.Stat(fullPath); err != nil {
			missing = append(missing, relativePath)
		}
	}
	if len(missing) > 0 {
		return Check{
			Name:       "schema_files",
			Status:     statusFail,
			Message:    fmt.Sprintf("missing required schema files: %s", strings.Join(missing, ",")),
			NonFixable: true,
		}
	}
	return Check{
		Name:    "schema_files",
		Status:  statusPass,
		Message: "required schema files are present",
	}
}

func checkHooksPath(workDir string) Check {
	if _, err := exec.LookPath("git"); err != nil {
		return Check{
			Name:       "hooks_path",
			Status:     statusWarn,
			Message:    "git is not available; cannot verify core.hooksPath",
			FixCommand: "make hooks",
		}
	}
	command := exec.Command("git", "-C", workDir, "config", "--get", "core.hooksPath") // #nosec G204 -- fixed executable and arguments.
	output, err := command.Output()
	if err != nil {
		return Check{
			Name:       "hooks_path",
			Status:     statusWarn,
			Message:    "git core.hooksPath is not configured",
			FixCommand: "make hooks",
		}
	}
	configured := filepath.Clean(strings.TrimSpace(string(output)))
	if configured == ".githooks" {
		return Check{
			Name:    "hooks_path",
			Status:  statusPass,
			Message: "git hooks path is configured",
		}
	}
	return Check{
		Name:       "hooks_path",
		Status:     statusWarn,
		Message:    fmt.Sprintf("git core.hooksPath is %q (expected .githooks)", configured),
		FixCommand: "make hooks",
	}
}

func checkRegistryCacheHealth() Check {
	home, err := os.UserHomeDir()
	if err != nil {
		return Check{
			Name:       "registry_cache",
			Status:     statusWarn,
			Message:    fmt.Sprintf("unable to resolve user home for registry cache: %v", err),
			FixCommand: "mkdir -p ~/.gait/registry",
		}
	}
	cacheDir := filepath.Join(home, ".gait", "registry")
	info, err := os.Stat(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:       "registry_cache",
				Status:     statusWarn,
				Message:    "registry cache is not initialized",
				FixCommand: "mkdir -p ~/.gait/registry",
			}
		}
		return Check{
			Name:       "registry_cache",
			Status:     statusWarn,
			Message:    fmt.Sprintf("registry cache check failed: %v", err),
			FixCommand: "mkdir -p ~/.gait/registry",
		}
	}
	if !info.IsDir() {
		return Check{
			Name:    "registry_cache",
			Status:  statusFail,
			Message: "registry cache path is not a directory",
		}
	}
	pinsDir := filepath.Join(cacheDir, "pins")
	pinFiles, err := filepath.Glob(filepath.Join(pinsDir, "*.pin"))
	if err != nil {
		return Check{
			Name:    "registry_cache",
			Status:  statusWarn,
			Message: fmt.Sprintf("unable to inspect registry pins: %v", err),
		}
	}
	if len(pinFiles) == 0 {
		return Check{
			Name:    "registry_cache",
			Status:  statusPass,
			Message: "registry cache is accessible",
		}
	}
	brokenPins := 0
	for _, pinPath := range pinFiles {
		// #nosec G304 -- pin files come from local cache glob.
		raw, readErr := os.ReadFile(pinPath)
		if readErr != nil {
			brokenPins++
			continue
		}
		digest := strings.ToLower(strings.TrimSpace(string(raw)))
		digest = strings.TrimPrefix(digest, "sha256:")
		if len(digest) != 64 {
			brokenPins++
			continue
		}
		metadataMatches, globErr := filepath.Glob(filepath.Join(cacheDir, "*", "*", digest, "registry_pack.json"))
		if globErr != nil || len(metadataMatches) == 0 {
			brokenPins++
		}
	}
	if brokenPins > 0 {
		return Check{
			Name:       "registry_cache",
			Status:     statusWarn,
			Message:    fmt.Sprintf("registry cache has %d inconsistent pin entries", brokenPins),
			FixCommand: fmt.Sprintf("gait registry list --cache-dir %s", shellQuote(cacheDir)),
		}
	}
	return Check{
		Name:    "registry_cache",
		Status:  statusPass,
		Message: fmt.Sprintf("registry cache healthy (%d pinned pack(s))", len(pinFiles)),
	}
}

func checkRateLimitLock(outputDir string) Check {
	lockPath := filepath.Join(outputDir, "gate_rate_limits.json.lock")
	info, err := os.Stat(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:    "gate_rate_limit_lock",
				Status:  statusPass,
				Message: "no stale gate rate-limit lock detected",
			}
		}
		return Check{
			Name:    "gate_rate_limit_lock",
			Status:  statusWarn,
			Message: fmt.Sprintf("unable to inspect gate lock file: %v", err),
		}
	}
	if info.IsDir() {
		return Check{
			Name:    "gate_rate_limit_lock",
			Status:  statusWarn,
			Message: "gate rate-limit lock path is a directory",
		}
	}
	age := time.Since(info.ModTime().UTC())
	if age > 30*time.Second {
		return Check{
			Name:       "gate_rate_limit_lock",
			Status:     statusWarn,
			Message:    fmt.Sprintf("stale gate rate-limit lock detected (%s old)", age.Truncate(time.Second)),
			FixCommand: fmt.Sprintf("rm -f %s", shellQuote(lockPath)),
		}
	}
	return Check{
		Name:    "gate_rate_limit_lock",
		Status:  statusPass,
		Message: "gate rate-limit lock is not stale",
	}
}

func checkTempDirWritable() Check {
	tempDir := strings.TrimSpace(os.TempDir())
	if tempDir == "" {
		return Check{
			Name:    "temp_dir",
			Status:  statusFail,
			Message: "temporary directory is not configured",
		}
	}
	testPath := filepath.Join(tempDir, fmt.Sprintf("gait-doctor-%d.tmp", time.Now().UTC().UnixNano()))
	if err := os.WriteFile(testPath, []byte("ok"), 0o600); err != nil {
		return Check{
			Name:       "temp_dir",
			Status:     statusFail,
			Message:    fmt.Sprintf("temporary directory is not writable: %v", err),
			FixCommand: fmt.Sprintf("chmod u+w %s", shellQuote(tempDir)),
		}
	}
	_ = os.Remove(testPath)
	return Check{
		Name:    "temp_dir",
		Status:  statusPass,
		Message: "temporary directory is writable",
	}
}

func checkKeySourceAmbiguity(cfg sign.KeyConfig) Check {
	privatePath := strings.TrimSpace(cfg.PrivateKeyPath)
	privateEnv := strings.TrimSpace(cfg.PrivateKeyEnv)
	publicPath := strings.TrimSpace(cfg.PublicKeyPath)
	publicEnv := strings.TrimSpace(cfg.PublicKeyEnv)

	if privatePath != "" && privateEnv != "" {
		return Check{
			Name:       "key_source_ambiguity",
			Status:     statusFail,
			Message:    "private key path and env are both set",
			FixCommand: "set only one of --private-key or --private-key-env",
		}
	}
	if publicPath != "" && publicEnv != "" {
		return Check{
			Name:       "key_source_ambiguity",
			Status:     statusFail,
			Message:    "public key path and env are both set",
			FixCommand: "set only one of --public-key or --public-key-env",
		}
	}
	return Check{
		Name:    "key_source_ambiguity",
		Status:  statusPass,
		Message: "key source configuration is unambiguous",
	}
}

func checkOnboardingBinary(workDir string) Check {
	binaryPath, err := findGaitBinaryPath(workDir)
	if err != nil {
		return Check{
			Name:       "onboarding_binary",
			Status:     statusWarn,
			Message:    "gait binary not found; onboarding commands may fail",
			FixCommand: "go build -o ./gait ./cmd/gait",
		}
	}

	info, err := os.Stat(binaryPath)
	if err != nil || info.IsDir() {
		return Check{
			Name:       "onboarding_binary",
			Status:     statusWarn,
			Message:    "gait binary path is not accessible",
			FixCommand: "go build -o ./gait ./cmd/gait",
		}
	}
	if !isExecutableBinary(binaryPath, info) {
		return Check{
			Name:       "onboarding_binary",
			Status:     statusWarn,
			Message:    fmt.Sprintf("gait binary is not executable: %s", binaryPath),
			FixCommand: fmt.Sprintf("chmod +x %s", shellQuote(binaryPath)),
		}
	}

	versionOutput, versionErr := readGaitVersion(binaryPath)
	if versionErr != nil {
		return Check{
			Name:       "onboarding_binary",
			Status:     statusWarn,
			Message:    fmt.Sprintf("gait binary version check failed (%s): %v", binaryPath, versionErr),
			FixCommand: "go build -o ./gait ./cmd/gait",
		}
	}

	return Check{
		Name:    "onboarding_binary",
		Status:  statusPass,
		Message: fmt.Sprintf("gait binary ready (path=%s version=%s)", binaryPath, versionOutput),
	}
}

func checkOnboardingAssets(workDir string) Check {
	missing := make([]string, 0, len(requiredOnboardingPaths))
	for _, relativePath := range requiredOnboardingPaths {
		fullPath := filepath.Join(workDir, filepath.FromSlash(relativePath))
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			missing = append(missing, relativePath)
			continue
		}
		if runtime.GOOS != "windows" && relativePath == "scripts/quickstart.sh" && info.Mode().Perm()&0o111 == 0 {
			return Check{
				Name:       "onboarding_assets",
				Status:     statusWarn,
				Message:    "scripts/quickstart.sh is not executable",
				FixCommand: "chmod +x scripts/quickstart.sh",
			}
		}
	}
	if len(missing) > 0 {
		return Check{
			Name:       "onboarding_assets",
			Status:     statusWarn,
			Message:    fmt.Sprintf("missing onboarding assets: %s", strings.Join(missing, ",")),
			FixCommand: "git restore --source=HEAD -- scripts/quickstart.sh examples/integrations",
		}
	}
	return Check{
		Name:    "onboarding_assets",
		Status:  statusPass,
		Message: "onboarding assets are present",
	}
}

func checkKeyConfig(mode sign.KeyMode, cfg sign.KeyConfig) Check {
	keyMode := mode
	if keyMode == "" {
		keyMode = sign.ModeDev
	}

	switch keyMode {
	case sign.ModeDev:
		if hasAnyKeySource(cfg) {
			return Check{
				Name:       "key_config",
				Status:     statusWarn,
				Message:    "dev mode ignores explicit key sources",
				FixCommand: "remove explicit key flags/env or use --key-mode prod",
			}
		}
		return Check{
			Name:    "key_config",
			Status:  statusPass,
			Message: "dev key mode is configured",
		}
	case sign.ModeProd:
		loadCfg := cfg
		loadCfg.Mode = sign.ModeProd
		if _, _, err := sign.LoadSigningKey(loadCfg); err != nil {
			return Check{
				Name:       "key_config",
				Status:     statusFail,
				Message:    fmt.Sprintf("invalid prod signing key config: %v", err),
				FixCommand: "set --private-key <path> or --private-key-env <VAR> for prod mode",
			}
		}
		if hasAnyVerifySource(cfg) {
			if _, err := sign.LoadVerifyKey(cfg); err != nil {
				return Check{
					Name:       "key_config",
					Status:     statusFail,
					Message:    fmt.Sprintf("invalid verify key config: %v", err),
					FixCommand: "set a valid --public-key/--public-key-env or matching private key source",
				}
			}
		}
		return Check{
			Name:    "key_config",
			Status:  statusPass,
			Message: "prod key configuration is valid",
		}
	default:
		return Check{
			Name:       "key_config",
			Status:     statusFail,
			Message:    fmt.Sprintf("unsupported key mode: %s", keyMode),
			FixCommand: "use --key-mode dev or --key-mode prod",
		}
	}
}

func hasAnyKeySource(cfg sign.KeyConfig) bool {
	return strings.TrimSpace(cfg.PrivateKeyPath) != "" ||
		strings.TrimSpace(cfg.PrivateKeyEnv) != "" ||
		strings.TrimSpace(cfg.PublicKeyPath) != "" ||
		strings.TrimSpace(cfg.PublicKeyEnv) != ""
}

func hasAnyVerifySource(cfg sign.KeyConfig) bool {
	return strings.TrimSpace(cfg.PublicKeyPath) != "" ||
		strings.TrimSpace(cfg.PublicKeyEnv) != "" ||
		strings.TrimSpace(cfg.PrivateKeyPath) != "" ||
		strings.TrimSpace(cfg.PrivateKeyEnv) != ""
}

func checkKeyFilePermissions(cfg sign.KeyConfig) Check {
	keyPaths := []string{
		strings.TrimSpace(cfg.PrivateKeyPath),
		strings.TrimSpace(cfg.PublicKeyPath),
	}
	requestedPaths := make([]string, 0, len(keyPaths))
	for _, path := range keyPaths {
		if path != "" {
			requestedPaths = append(requestedPaths, path)
		}
	}
	if len(requestedPaths) == 0 {
		return Check{
			Name:    "key_permissions",
			Status:  statusPass,
			Message: "no key file paths configured",
		}
	}

	for _, path := range requestedPaths {
		info, err := os.Stat(path)
		if err != nil {
			return Check{
				Name:       "key_permissions",
				Status:     statusWarn,
				Message:    fmt.Sprintf("key file not accessible: %s (%v)", path, err),
				FixCommand: fmt.Sprintf("ls -l %s", shellQuote(path)),
			}
		}
		if info.IsDir() {
			return Check{
				Name:       "key_permissions",
				Status:     statusWarn,
				Message:    fmt.Sprintf("key path is a directory: %s", path),
				FixCommand: fmt.Sprintf("set key path to a file: %s", shellQuote(path)),
			}
		}
		if runtime.GOOS == "windows" {
			continue
		}
		if info.Mode().Perm()&0o022 != 0 {
			return Check{
				Name:       "key_permissions",
				Status:     statusWarn,
				Message:    fmt.Sprintf("key file is writable by group/others: %s", path),
				FixCommand: fmt.Sprintf("chmod go-w %s", shellQuote(path)),
			}
		}
	}

	return Check{
		Name:    "key_permissions",
		Status:  statusPass,
		Message: "key file permissions are strict",
	}
}

func findGaitBinaryPath(workDir string) (string, error) {
	if path, err := exec.LookPath("gait"); err == nil {
		return path, nil
	}

	candidates := []string{
		filepath.Join(workDir, "gait"),
		filepath.Join(workDir, "gait.exe"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("gait binary not found")
}

func isExecutableBinary(path string, info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		lowerPath := strings.ToLower(path)
		return strings.HasSuffix(lowerPath, ".exe") ||
			strings.HasSuffix(lowerPath, ".cmd") ||
			strings.HasSuffix(lowerPath, ".bat")
	}
	return info.Mode().Perm()&0o111 != 0
}

func readGaitVersion(binaryPath string) (string, error) {
	command := exec.Command(binaryPath, "version") // #nosec G204 -- controlled binary path from local workspace/PATH.
	output, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, "gait ") {
		return "", fmt.Errorf("unexpected version output: %s", version)
	}
	return version, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
