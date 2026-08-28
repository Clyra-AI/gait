package effects

// ComposeRunner is a constrained, opt-in local integration runner. It is an
// observation harness, not a general Docker executor: project/file scope is
// explicit, commands are bounded, and no dependency is installed or pulled.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var composeProjectPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

type ComposeRunOptions struct {
	ComposeFile              string
	Project                  string
	WorkingDir               string
	DockerBinary             string
	Service                  string
	MountedPath              string
	ProxyURL                 string
	Reset                    []string
	Seed                     []string
	Run                      []string
	Diff                     []string
	Accept                   []string
	Repeat                   []string
	Timeout                  time.Duration
	Now                      time.Time
	Capture                  func(CaptureRequest) (CaptureResult, error)
	PostgresCollector        func(context.Context) (CaptureResult, error)
	PostgresReference        string
	PostgresCollectorCommand []string
}

type ComposeRunResult struct {
	Status       string          `json:"status"`
	ReasonCodes  []string        `json:"reason_codes,omitempty"`
	Project      string          `json:"project"`
	ComposeFile  string          `json:"compose_file"`
	Observations []CaptureResult `json:"observations,omitempty"`
	Before       []CaptureResult `json:"before,omitempty"`
	After        []CaptureResult `json:"after,omitempty"`
	Changed      bool            `json:"changed,omitempty"`
	Commands     []string        `json:"commands,omitempty"`
}

func RunCompose(ctx context.Context, options ComposeRunOptions) ComposeRunResult {
	workingDir := strings.TrimSpace(options.WorkingDir)
	if workingDir != "" {
		if abs, err := filepath.Abs(workingDir); err == nil {
			workingDir = filepath.Clean(abs)
		}
	}
	composeFile := strings.TrimSpace(options.ComposeFile)
	if composeFile != "" && !filepath.IsAbs(composeFile) {
		if workingDir != "" {
			composeFile = filepath.Join(workingDir, composeFile)
		} else if abs, err := filepath.Abs(composeFile); err == nil {
			composeFile = abs
		}
	}
	composeFile = filepath.Clean(composeFile)
	result := ComposeRunResult{Status: "failed", Project: strings.TrimSpace(options.Project), ComposeFile: composeFile}
	fail := func(code string) ComposeRunResult {
		result.ReasonCodes = append(result.ReasonCodes, code)
		return result
	}
	if !composeProjectPattern.MatchString(result.Project) {
		return fail("compose_project_invalid")
	}
	if result.ComposeFile == "" {
		return fail("compose_file_required")
	}
	info, err := os.Lstat(result.ComposeFile)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fail("compose_file_unavailable")
	}
	if workingDir != "" && !composePathWithin(workingDir, result.ComposeFile) {
		return fail("compose_file_out_of_scope")
	}
	if workingDir != "" && strings.TrimSpace(options.MountedPath) != "" && !composePathWithin(workingDir, options.MountedPath) {
		return fail("compose_mounted_path_out_of_scope")
	}
	docker := strings.TrimSpace(options.DockerBinary)
	if docker == "" {
		docker = "docker"
	}
	if _, err := exec.LookPath(docker); err != nil {
		return fail("compose_dependency_missing")
	}
	timeout := options.Timeout
	if timeout <= 0 || timeout > 10*time.Minute {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := func(command []string) error {
		if len(command) == 0 {
			return nil
		}
		if strings.TrimSpace(command[0]) == "" {
			return errors.New("compose hook command missing")
		}
		if workingDir != "" && filepath.IsAbs(command[0]) && !composePathWithin(workingDir, command[0]) {
			return errors.New("compose hook out of scope")
		}
		cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204 -- hooks are explicit local test configuration.
		if workingDir != "" {
			cmd.Dir = workingDir
		}
		if err := cmd.Run(); err != nil {
			return err
		}
		result.Commands = append(result.Commands, strings.Join(command, " "))
		return nil
	}
	compose := func(args ...string) error {
		if len(args) > 0 && args[len(args)-1] == "" {
			args = args[:len(args)-1]
		}
		full := append([]string{"compose", "--project-name", result.Project, "--file", result.ComposeFile}, args...)
		return run(append([]string{docker}, full...))
	}
	// Cleanup is always project-scoped and never targets unrelated containers.
	// Use a fresh bounded context so an expired run context cannot strand the
	// explicitly selected Compose project.
	defer func() {
		cleanupTimeout := timeout
		if cleanupTimeout <= 0 || cleanupTimeout > 30*time.Second {
			cleanupTimeout = 30 * time.Second
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cleanupCancel()
		cleanupArgs := []string{"compose", "--project-name", result.Project, "--file", result.ComposeFile, "down", "--remove-orphans"}
		cleanupCmd := exec.CommandContext(cleanupCtx, docker, cleanupArgs...) // #nosec G204 -- docker binary is explicit operator configuration.
		if workingDir != "" {
			cleanupCmd.Dir = workingDir
		}
		_ = cleanupCmd.Run()
	}()
	if err := compose("down", "--remove-orphans"); err != nil {
		return fail("compose_reset_failed")
	}
	if err := compose("up", "--detach", "--no-build", options.Service); err != nil {
		return fail("compose_start_failed")
	}
	if err := run(options.Reset); err != nil {
		return fail("compose_reset_hook_failed")
	}
	if err := run(options.Seed); err != nil {
		return fail("compose_seed_hook_failed")
	}
	capture := options.Capture
	if capture == nil {
		capture = CaptureLocal
	}
	observe := func(before bool) error {
		appendObservation := func(observation CaptureResult) {
			result.Observations = append(result.Observations, observation)
			if before {
				result.Before = append(result.Before, observation)
			} else {
				result.After = append(result.After, observation)
			}
		}
		if len(options.PostgresCollectorCommand) > 0 || options.PostgresCollector != nil || (strings.TrimSpace(options.Service) == "postgres" && options.Capture != nil) {
			var observation CaptureResult
			var captureErr error
			if options.PostgresCollector != nil {
				observation, captureErr = options.PostgresCollector(ctx)
			} else if len(options.PostgresCollectorCommand) > 0 {
				observation, captureErr = runComposeCollector(ctx, options.PostgresCollectorCommand, workingDir)
			} else {
				// Capture is injectable for hermetic tests and bounded local
				// collectors; CaptureLocal deliberately has no Postgres driver.
				observation, captureErr = capture(CaptureRequest{ResourceKind: ResourcePostgres, Reference: options.PostgresReference, Now: options.Now})
			}
			if captureErr != nil {
				return captureErr
			}
			appendObservation(observation)
		}
		if strings.TrimSpace(options.MountedPath) != "" {
			observation, captureErr := capture(CaptureRequest{ResourceKind: ResourceFilesystem, Path: options.MountedPath, Now: options.Now})
			if captureErr != nil {
				return captureErr
			}
			appendObservation(observation)
		}
		if strings.TrimSpace(options.ProxyURL) != "" {
			observation, captureErr := capture(CaptureRequest{ResourceKind: ResourceHTTP, URL: options.ProxyURL, AllowUnsafeLocal: true, Now: options.Now})
			if captureErr != nil {
				return captureErr
			}
			appendObservation(observation)
		}
		return nil
	}
	if err := observe(true); err != nil {
		return fail("compose_before_observation_failed")
	}
	if err := run(options.Run); err != nil {
		return fail("compose_run_failed")
	}
	if err := observe(false); err != nil {
		return fail("compose_after_observation_failed")
	}
	if len(result.Observations) == 0 {
		return fail("compose_observation_missing")
	}
	result.Changed = composeObservationsChanged(result.Before, result.After)
	if err := run(options.Diff); err != nil {
		return fail("compose_diff_failed")
	}
	if err := run(options.Accept); err != nil {
		return fail("compose_accept_failed")
	}
	if err := run(options.Repeat); err != nil {
		return fail("compose_repeat_failed")
	}
	result.Status = "pass"
	return result
}

func composeObservationsChanged(before, after []CaptureResult) bool {
	if len(before) != len(after) {
		return true
	}
	for i := range before {
		if before[i].Observation.Identity != after[i].Observation.Identity || before[i].Observation.State != after[i].Observation.State || before[i].Observation.Digest != after[i].Observation.Digest {
			return true
		}
	}
	return false
}

const maxComposeCollectorBytes = 1 << 20

func runComposeCollector(ctx context.Context, command []string, workingDir string) (CaptureResult, error) {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return CaptureResult{}, errors.New("postgres collector command required")
	}
	if workingDir != "" && filepath.IsAbs(command[0]) && !composePathWithin(workingDir, command[0]) {
		return CaptureResult{}, errors.New("postgres collector command out of scope")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204 -- explicit bounded local collector command.
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	var out bytes.Buffer
	cmd.Stdout = &limitedComposeBuffer{Buffer: &out, Limit: maxComposeCollectorBytes}
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return CaptureResult{}, err
	}
	var capture CaptureResult
	if err := decodeStrict(out.Bytes(), &capture); err != nil {
		return CaptureResult{}, errors.New("postgres collector response malformed")
	}
	if capture.Observation.Identity == "" || capture.Observation.ObservedAt == "" {
		return CaptureResult{}, errors.New("postgres collector response incomplete")
	}
	return capture, nil
}

type limitedComposeBuffer struct {
	*bytes.Buffer
	Limit int
}

func (b *limitedComposeBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.Limit {
		return 0, errors.New("collector output exceeds size limit")
	}
	return b.Buffer.Write(p)
}

func MarshalComposeCollectorResult(result CaptureResult) ([]byte, error) { return json.Marshal(result) }

func ValidateComposeOptions(options ComposeRunOptions) error {
	if !composeProjectPattern.MatchString(strings.TrimSpace(options.Project)) {
		return fmt.Errorf("compose project must match %s", composeProjectPattern.String())
	}
	if strings.TrimSpace(options.ComposeFile) == "" {
		return errors.New("compose file required")
	}
	return nil
}

func ComposeProjectValid(project string) bool {
	return composeProjectPattern.MatchString(strings.TrimSpace(project))
}

func ComposeDependencyAvailable(binary string) bool {
	_, err := exec.LookPath(strings.TrimSpace(binary))
	return err == nil
}

func composePathWithin(root, path string) bool {
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(r, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
