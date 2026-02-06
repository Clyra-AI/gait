package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		checkSchemaFiles(workDir),
		checkOnboardingBinary(workDir),
		checkOnboardingAssets(workDir),
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

func checkOnboardingBinary(workDir string) Check {
	if _, err := exec.LookPath("gait"); err == nil {
		return Check{
			Name:    "onboarding_binary",
			Status:  statusPass,
			Message: "gait binary is available on PATH",
		}
	}

	localBinaryPath := filepath.Join(workDir, "gait")
	info, err := os.Stat(localBinaryPath)
	if err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
		return Check{
			Name:    "onboarding_binary",
			Status:  statusPass,
			Message: "local gait binary is executable",
		}
	}

	return Check{
		Name:       "onboarding_binary",
		Status:     statusWarn,
		Message:    "gait binary not found; onboarding commands may fail",
		FixCommand: "go build -o ./gait ./cmd/gait",
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
		if relativePath == "scripts/quickstart.sh" && info.Mode().Perm()&0o111 == 0 {
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

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
