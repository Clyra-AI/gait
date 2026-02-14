package regress

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
)

const (
	configFileName    = "gait.yaml"
	fixturesDirName   = "fixtures"
	fixtureFileName   = "fixture.json"
	fixtureRunpack    = "runpack.zip"
	configSchemaID    = "gait.regress.config"
	configSchemaV1    = "1.0.0"
	fixtureSchemaID   = "gait.regress.fixture"
	fixtureSchemaV1   = "1.0.0"
	defaultFixtureSet = "default"
)

var (
	fixtureNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	digestPattern      = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

type InitOptions struct {
	SourceRunpackPath string
	SessionChainPath  string
	CheckpointRef     string
	FixtureName       string
	WorkDir           string
}

type InitResult struct {
	RunID        string
	FixtureName  string
	FixtureDir   string
	RunpackPath  string
	ConfigPath   string
	NextCommands []string
}

type fixtureMeta struct {
	SchemaID                 string   `json:"schema_id"`
	SchemaVersion            string   `json:"schema_version"`
	Name                     string   `json:"name"`
	RunID                    string   `json:"run_id"`
	Runpack                  string   `json:"runpack"`
	ExpectedReplayExitCode   int      `json:"expected_replay_exit_code"`
	CandidateRunpack         string   `json:"candidate_runpack,omitempty"`
	ContextConformance       string   `json:"context_conformance,omitempty"`
	AllowContextRuntimeDrift bool     `json:"allow_context_runtime_drift,omitempty"`
	ExpectedContextSetDigest string   `json:"expected_context_set_digest,omitempty"`
	DiffAllowChangedFiles    []string `json:"diff_allow_changed_files,omitempty"`
	SessionChain             string   `json:"session_chain,omitempty"`
	CheckpointIndex          int      `json:"checkpoint_index,omitempty"`
}

type configFile struct {
	SchemaID      string          `json:"schema_id"`
	SchemaVersion string          `json:"schema_version"`
	FixtureSet    string          `json:"fixture_set"`
	Fixtures      []configFixture `json:"fixtures"`
}

type configFixture struct {
	Name    string `json:"name"`
	RunID   string `json:"run_id"`
	Runpack string `json:"runpack"`
}

func InitFixture(opts InitOptions) (InitResult, error) {
	if opts.SourceRunpackPath == "" && opts.SessionChainPath == "" {
		return InitResult{}, fmt.Errorf("source runpack path is required")
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}

	sourceRunpackPath := strings.TrimSpace(opts.SourceRunpackPath)
	sessionChainPath := strings.TrimSpace(opts.SessionChainPath)
	checkpointIndex := 0
	if sessionChainPath != "" {
		checkpoint, resolveErr := runpack.ResolveSessionCheckpointRunpack(sessionChainPath, strings.TrimSpace(opts.CheckpointRef))
		if resolveErr != nil {
			return InitResult{}, fmt.Errorf("resolve session checkpoint: %w", resolveErr)
		}
		sourceRunpackPath = checkpoint.RunpackPath
		checkpointIndex = checkpoint.CheckpointIndex
	}
	sourceRunpackPath, cleanupSourcePath, err := materializeRunpackSource(sourceRunpackPath)
	if err != nil {
		return InitResult{}, err
	}
	defer cleanupSourcePath()

	verifyResult, err := runpack.VerifyZip(sourceRunpackPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		return InitResult{}, fmt.Errorf("verify source runpack: %w", err)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 {
		return InitResult{}, fmt.Errorf("source runpack failed integrity checks")
	}
	if verifyResult.RunID == "" {
		return InitResult{}, fmt.Errorf("source runpack missing run_id")
	}

	fixtureName := opts.FixtureName
	if fixtureName == "" {
		fixtureName = sanitizeFixtureName(verifyResult.RunID)
	}
	if !isValidFixtureName(fixtureName) {
		return InitResult{}, fmt.Errorf("invalid fixture name: %s", fixtureName)
	}

	fixturesRoot := filepath.Join(workDir, fixturesDirName)
	fixtureDir := filepath.Join(fixturesRoot, fixtureName)
	if err := os.MkdirAll(fixtureDir, 0o750); err != nil {
		return InitResult{}, fmt.Errorf("create fixture directory: %w", err)
	}

	destinationRunpack := filepath.Join(fixtureDir, fixtureRunpack)
	if err := copyRunpack(sourceRunpackPath, destinationRunpack); err != nil {
		return InitResult{}, err
	}

	meta := fixtureMeta{
		SchemaID:               fixtureSchemaID,
		SchemaVersion:          fixtureSchemaV1,
		Name:                   fixtureName,
		RunID:                  verifyResult.RunID,
		Runpack:                fixtureRunpack,
		ExpectedReplayExitCode: 0,
		SessionChain:           sessionChainPath,
		CheckpointIndex:        checkpointIndex,
	}
	if sourcePack, readErr := runpack.ReadRunpack(sourceRunpackPath); readErr == nil {
		if strings.TrimSpace(sourcePack.Refs.ContextSetDigest) != "" {
			meta.ContextConformance = "required"
			meta.ExpectedContextSetDigest = strings.TrimSpace(sourcePack.Refs.ContextSetDigest)
		}
	}
	if err := writeJSON(filepath.Join(fixtureDir, fixtureFileName), meta); err != nil {
		return InitResult{}, fmt.Errorf("write fixture metadata: %w", err)
	}

	if _, err := writeConfig(workDir); err != nil {
		return InitResult{}, err
	}

	return InitResult{
		RunID:        verifyResult.RunID,
		FixtureName:  fixtureName,
		FixtureDir:   slashPath(filepath.Join(fixturesDirName, fixtureName)),
		RunpackPath:  slashPath(filepath.Join(fixturesDirName, fixtureName, fixtureRunpack)),
		ConfigPath:   configFileName,
		NextCommands: []string{"gait regress run --json"},
	}, nil
}

func writeConfig(workDir string) (string, error) {
	fixturesRoot := filepath.Join(workDir, fixturesDirName)
	entries, err := loadFixtureEntries(fixturesRoot)
	if err != nil {
		return "", err
	}

	cfg := configFile{
		SchemaID:      configSchemaID,
		SchemaVersion: configSchemaV1,
		FixtureSet:    defaultFixtureSet,
		Fixtures:      entries,
	}

	configPath := filepath.Join(workDir, configFileName)
	if err := writeJSON(configPath, cfg); err != nil {
		return "", fmt.Errorf("write %s: %w", configFileName, err)
	}
	return configPath, nil
}

func loadFixtureEntries(fixturesRoot string) ([]configFixture, error) {
	dirEntries, err := os.ReadDir(fixturesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []configFixture{}, nil
		}
		return nil, fmt.Errorf("list fixtures: %w", err)
	}

	fixtures := make([]configFixture, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		name := dirEntry.Name()
		if !isValidFixtureName(name) {
			continue
		}
		metaPath := filepath.Join(fixturesRoot, name, fixtureFileName)
		if _, err := os.Stat(metaPath); err != nil {
			if os.IsNotExist(err) {
				// Skip non-regression fixture folders (for example, other fixture families)
				// unless they look like an incomplete regress fixture directory.
				runpackCandidate := filepath.Join(fixturesRoot, name, fixtureRunpack)
				if _, runpackErr := os.Stat(runpackCandidate); runpackErr == nil {
					return nil, fmt.Errorf("fixture metadata missing for %s", name)
				} else if runpackErr != nil && !os.IsNotExist(runpackErr) {
					return nil, fmt.Errorf("stat fixture runpack for %s: %w", name, runpackErr)
				}
				continue
			}
			return nil, fmt.Errorf("stat fixture metadata for %s: %w", name, err)
		}
		meta, err := readFixtureMeta(metaPath)
		if err != nil {
			return nil, err
		}
		if meta.Name != name {
			return nil, fmt.Errorf("fixture metadata name mismatch for %s", name)
		}
		if filepath.Base(meta.Runpack) != meta.Runpack {
			return nil, fmt.Errorf("fixture runpack path must be a filename for %s", name)
		}

		runpackPath := filepath.Join(fixturesRoot, name, meta.Runpack)
		if _, err := os.Stat(runpackPath); err != nil {
			return nil, fmt.Errorf("fixture runpack missing for %s: %w", name, err)
		}

		fixtures = append(fixtures, configFixture{
			Name:    meta.Name,
			RunID:   meta.RunID,
			Runpack: slashPath(filepath.Join(fixturesDirName, name, meta.Runpack)),
		})
	}

	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].Name < fixtures[j].Name
	})
	return fixtures, nil
}

func readFixtureMeta(path string) (fixtureMeta, error) {
	// #nosec G304 -- fixture metadata path is derived from local workspace fixture directories.
	content, err := os.ReadFile(path)
	if err != nil {
		return fixtureMeta{}, fmt.Errorf("read fixture metadata: %w", err)
	}
	var meta fixtureMeta
	if err := json.Unmarshal(content, &meta); err != nil {
		return fixtureMeta{}, fmt.Errorf("parse fixture metadata: %w", err)
	}
	if meta.Name == "" || meta.RunID == "" || meta.Runpack == "" {
		return fixtureMeta{}, fmt.Errorf("fixture metadata incomplete: %s", slashPath(path))
	}
	if meta.ExpectedReplayExitCode < 0 {
		return fixtureMeta{}, fmt.Errorf("fixture expected_replay_exit_code must be >= 0: %s", slashPath(path))
	}
	if meta.CheckpointIndex < 0 {
		return fixtureMeta{}, fmt.Errorf("fixture checkpoint_index must be >= 0: %s", slashPath(path))
	}
	meta.ContextConformance = strings.ToLower(strings.TrimSpace(meta.ContextConformance))
	if meta.ContextConformance != "" && meta.ContextConformance != "required" && meta.ContextConformance != "none" {
		return fixtureMeta{}, fmt.Errorf("fixture context_conformance must be one of required|none: %s", slashPath(path))
	}
	meta.ExpectedContextSetDigest = strings.TrimSpace(meta.ExpectedContextSetDigest)
	if meta.ExpectedContextSetDigest != "" && !digestPattern.MatchString(meta.ExpectedContextSetDigest) {
		return fixtureMeta{}, fmt.Errorf("fixture expected_context_set_digest must be sha256 hex: %s", slashPath(path))
	}
	return meta, nil
}

func copyRunpack(sourcePath, destinationPath string) error {
	// #nosec G304 -- source runpack path is explicit CLI user input.
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source runpack: %w", err)
	}
	if err := fsx.WriteFileAtomic(destinationPath, content, 0o600); err != nil {
		return fmt.Errorf("write fixture runpack: %w", err)
	}
	return nil
}

func materializeRunpackSource(sourcePath string) (string, func(), error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", func() {}, fmt.Errorf("source runpack path is required")
	}
	_, verifyErr := runpack.VerifyZip(sourcePath, runpack.VerifyOptions{RequireSignature: false})
	if verifyErr == nil {
		return sourcePath, func() {}, nil
	}
	extracted, extractErr := pack.ExtractRunpack(sourcePath)
	if extractErr != nil {
		return "", func() {}, fmt.Errorf("verify source runpack: %w", verifyErr)
	}
	tempFile, err := os.CreateTemp("", "gait-regress-source-*.zip")
	if err != nil {
		return "", func() {}, fmt.Errorf("materialize source runpack: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(extracted); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", func() {}, fmt.Errorf("materialize source runpack: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", func() {}, fmt.Errorf("materialize source runpack: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	return tempPath, cleanup, nil
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return fsx.WriteFileAtomic(path, encoded, 0o600)
}

func sanitizeFixtureName(value string) string {
	lower := strings.ToLower(value)
	var out strings.Builder
	out.Grow(len(lower))
	lastDash := false
	for _, char := range lower {
		switch {
		case char >= 'a' && char <= 'z':
			out.WriteRune(char)
			lastDash = false
		case char >= '0' && char <= '9':
			out.WriteRune(char)
			lastDash = false
		case char == '.' || char == '_' || char == '-':
			out.WriteRune(char)
			lastDash = false
		default:
			if !lastDash {
				out.WriteRune('-')
				lastDash = true
			}
		}
	}
	candidate := strings.Trim(out.String(), "-._")
	if candidate == "" {
		return "fixture"
	}
	return candidate
}

func isValidFixtureName(value string) bool {
	return fixtureNamePattern.MatchString(value)
}

func slashPath(path string) string {
	return filepath.ToSlash(path)
}
