package regress

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/contextproof"
	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/runpack"
	schemaregress "github.com/davidahmann/gait/core/schema/v1/regress"
)

const (
	regressStatusPass     = "pass"
	regressStatusFail     = "fail"
	replayMissingExitCode = 2
	defaultRegressOutFile = "regress_result.json"
)

type RunOptions struct {
	ConfigPath               string
	OutputPath               string
	JUnitPath                string
	WorkDir                  string
	ProducerVersion          string
	AllowNondeterministic    bool
	ContextConformance       bool
	AllowContextRuntimeDrift bool
}

type RunResult struct {
	Result        schemaregress.RegressResult
	OutputPath    string
	JUnitPath     string
	FailedGraders int
}

type junitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     string           `xml:"time,attr"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

type Grader interface {
	Name() string
	Deterministic() bool
	Grade(ctx FixtureContext) (schemaregress.GraderResult, error)
}

type FixtureContext struct {
	Fixture fixtureSpec
}

type fixtureSpec struct {
	Name         string
	RunID        string
	FixtureDir   string
	RunpackPath  string
	RunCreatedAt time.Time
	Meta         fixtureMeta
}

func Run(opts RunOptions) (RunResult, error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = configFileName
	}
	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = defaultRegressOutFile
	}
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(configPath)
		if workDir == "" {
			workDir = "."
		}
	}
	producerVersion := opts.ProducerVersion
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	cfg, err := readRegressConfig(configPath)
	if err != nil {
		return RunResult{}, err
	}
	fixtures, err := loadFixtureSpecs(cfg, workDir)
	if err != nil {
		return RunResult{}, err
	}
	if len(fixtures) == 0 {
		return RunResult{}, fmt.Errorf("no fixtures configured")
	}

	graders := []Grader{
		schemaValidationGrader{},
		expectedReplayExitCodeGrader{},
		trajectoryGrader{},
		diffGrader{},
	}
	if shouldRunContextConformance(opts, fixtures) {
		graders = append(graders, contextConformanceGrader{
			enforce:           opts.ContextConformance,
			allowRuntimeDrift: opts.AllowContextRuntimeDrift,
		})
	}
	for _, grader := range graders {
		if !grader.Deterministic() && !opts.AllowNondeterministic {
			return RunResult{}, fmt.Errorf("non-deterministic grader blocked: %s", grader.Name())
		}
	}

	graderResults := make([]schemaregress.GraderResult, 0, len(fixtures)*len(graders))
	failedGraders := 0
	for _, fixture := range fixtures {
		ctx := FixtureContext{Fixture: fixture}
		for _, grader := range graders {
			graderResult, gradeErr := grader.Grade(ctx)
			if gradeErr != nil {
				graderResult = schemaregress.GraderResult{
					Name:        grader.Name(),
					Status:      regressStatusFail,
					ReasonCodes: []string{"grader_error"},
					Details: map[string]any{
						"error": gradeErr.Error(),
					},
				}
			}
			if graderResult.Name == "" {
				graderResult.Name = grader.Name()
			}
			graderResult.Name = fmt.Sprintf("%s/%s", fixture.Name, graderResult.Name)
			if graderResult.Status == "" {
				graderResult.Status = regressStatusFail
			}
			graderResult.ReasonCodes = uniqueSortedStrings(graderResult.ReasonCodes)
			if graderResult.Details == nil {
				graderResult.Details = map[string]any{}
			}
			graderResult.Details["fixture"] = fixture.Name
			graderResult.Details["run_id"] = fixture.RunID
			if graderResult.Status == regressStatusFail {
				failedGraders++
			}
			graderResults = append(graderResults, graderResult)
		}
	}

	sort.Slice(graderResults, func(i, j int) bool {
		return graderResults[i].Name < graderResults[j].Name
	})

	status := regressStatusPass
	if failedGraders > 0 {
		status = regressStatusFail
	}
	createdAt := fixtures[0].RunCreatedAt
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	result := schemaregress.RegressResult{
		SchemaID:        "gait.regress.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt.UTC(),
		ProducerVersion: producerVersion,
		FixtureSet:      cfg.FixtureSet,
		Status:          status,
		Graders:         graderResults,
	}

	if err := writeJSON(outputPath, result); err != nil {
		return RunResult{}, fmt.Errorf("write regress result: %w", err)
	}

	junitPath := strings.TrimSpace(opts.JUnitPath)
	if junitPath != "" {
		if err := writeJUnitReport(junitPath, result); err != nil {
			return RunResult{}, fmt.Errorf("write junit report: %w", err)
		}
	}

	return RunResult{
		Result:        result,
		OutputPath:    outputPath,
		JUnitPath:     junitPath,
		FailedGraders: failedGraders,
	}, nil
}

func readRegressConfig(configPath string) (configFile, error) {
	// #nosec G304 -- config path is provided by the caller.
	content, err := os.ReadFile(configPath)
	if err != nil {
		return configFile{}, fmt.Errorf("read config: %w", err)
	}
	var cfg configFile
	if err := json.Unmarshal(content, &cfg); err != nil {
		return configFile{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.SchemaID != "" && cfg.SchemaID != configSchemaID {
		return configFile{}, fmt.Errorf("unsupported config schema_id: %s", cfg.SchemaID)
	}
	if cfg.SchemaVersion != "" && cfg.SchemaVersion != configSchemaV1 {
		return configFile{}, fmt.Errorf("unsupported config schema_version: %s", cfg.SchemaVersion)
	}
	if cfg.FixtureSet == "" {
		cfg.FixtureSet = defaultFixtureSet
	}
	sort.Slice(cfg.Fixtures, func(i, j int) bool {
		return cfg.Fixtures[i].Name < cfg.Fixtures[j].Name
	})
	for _, fixture := range cfg.Fixtures {
		if strings.TrimSpace(fixture.Name) == "" {
			return configFile{}, fmt.Errorf("fixture name is required")
		}
		if strings.TrimSpace(fixture.Runpack) == "" {
			return configFile{}, fmt.Errorf("fixture runpack is required for %s", fixture.Name)
		}
	}
	return cfg, nil
}

func loadFixtureSpecs(cfg configFile, workDir string) ([]fixtureSpec, error) {
	specs := make([]fixtureSpec, 0, len(cfg.Fixtures))
	for _, fixture := range cfg.Fixtures {
		runpackPath := fixture.Runpack
		if !filepath.IsAbs(runpackPath) {
			runpackPath = filepath.Join(workDir, filepath.FromSlash(runpackPath))
		}
		if _, err := os.Stat(runpackPath); err != nil {
			return nil, fmt.Errorf("fixture runpack missing for %s: %w", fixture.Name, err)
		}

		pack, err := runpack.ReadRunpack(runpackPath)
		if err != nil {
			return nil, fmt.Errorf("read fixture runpack for %s: %w", fixture.Name, err)
		}

		fixtureDir := filepath.Dir(runpackPath)
		metaPath := filepath.Join(fixtureDir, fixtureFileName)
		meta, err := readFixtureMeta(metaPath)
		if err != nil {
			return nil, fmt.Errorf("read fixture metadata for %s: %w", fixture.Name, err)
		}
		if meta.Name != fixture.Name {
			return nil, fmt.Errorf("fixture metadata name mismatch for %s", fixture.Name)
		}
		if fixture.RunID != "" && meta.RunID != fixture.RunID {
			return nil, fmt.Errorf("fixture run_id mismatch for %s", fixture.Name)
		}
		if pack.Run.RunID != meta.RunID {
			return nil, fmt.Errorf("runpack run_id mismatch for %s", fixture.Name)
		}
		meta.DiffAllowChangedFiles = uniqueSortedStrings(meta.DiffAllowChangedFiles)

		specs = append(specs, fixtureSpec{
			Name:         fixture.Name,
			RunID:        meta.RunID,
			FixtureDir:   fixtureDir,
			RunpackPath:  runpackPath,
			RunCreatedAt: pack.Run.CreatedAt,
			Meta:         meta,
		})
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs, nil
}

func shouldRunContextConformance(opts RunOptions, fixtures []fixtureSpec) bool {
	if opts.ContextConformance {
		return true
	}
	for _, fixture := range fixtures {
		if strings.TrimSpace(fixture.Meta.ContextConformance) != "" {
			return true
		}
		if strings.TrimSpace(fixture.Meta.ExpectedContextSetDigest) != "" {
			return true
		}
		if fixture.Meta.AllowContextRuntimeDrift {
			return true
		}
	}
	return false
}

type schemaValidationGrader struct{}

func (schemaValidationGrader) Name() string { return "schema_validation" }

func (schemaValidationGrader) Deterministic() bool { return true }

func (schemaValidationGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	verifyResult, err := runpack.VerifyZip(ctx.Fixture.RunpackPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		return failResult("schema_validation", "schema_validation_error", map[string]any{
			"error": err.Error(),
		}), nil
	}

	reasonCodes := []string{}
	if len(verifyResult.MissingFiles) > 0 {
		reasonCodes = append(reasonCodes, "missing_files")
	}
	if len(verifyResult.HashMismatches) > 0 {
		reasonCodes = append(reasonCodes, "hash_mismatch")
	}

	pack, err := runpack.ReadRunpack(ctx.Fixture.RunpackPath)
	if err != nil {
		reasonCodes = append(reasonCodes, "runpack_parse_error")
		return schemaregress.GraderResult{
			Name:        "schema_validation",
			Status:      regressStatusFail,
			ReasonCodes: uniqueSortedStrings(reasonCodes),
			Details: map[string]any{
				"error": err.Error(),
			},
		}, nil
	}

	if pack.Manifest.SchemaID != "gait.runpack.manifest" || pack.Manifest.SchemaVersion != "1.0.0" {
		reasonCodes = append(reasonCodes, "manifest_schema_invalid")
	}
	if pack.Run.SchemaID != "gait.runpack.run" || pack.Run.SchemaVersion != "1.0.0" {
		reasonCodes = append(reasonCodes, "run_schema_invalid")
	}
	if pack.Refs.SchemaID != "gait.runpack.refs" || pack.Refs.SchemaVersion != "1.0.0" {
		reasonCodes = append(reasonCodes, "refs_schema_invalid")
	}
	for _, intent := range pack.Intents {
		if intent.SchemaID != "gait.runpack.intent" || intent.SchemaVersion != "1.0.0" {
			reasonCodes = append(reasonCodes, "intent_schema_invalid")
			break
		}
	}
	for _, result := range pack.Results {
		if result.SchemaID != "gait.runpack.result" || result.SchemaVersion != "1.0.0" {
			reasonCodes = append(reasonCodes, "result_schema_invalid")
			break
		}
	}

	details := map[string]any{
		"missing_files":      verifyResult.MissingFiles,
		"hash_mismatches":    len(verifyResult.HashMismatches),
		"intent_records":     len(pack.Intents),
		"result_records":     len(pack.Results),
		"reference_receipts": len(pack.Refs.Receipts),
	}

	if len(reasonCodes) > 0 {
		return schemaregress.GraderResult{
			Name:        "schema_validation",
			Status:      regressStatusFail,
			ReasonCodes: uniqueSortedStrings(reasonCodes),
			Details:     details,
		}, nil
	}
	return schemaregress.GraderResult{
		Name:        "schema_validation",
		Status:      regressStatusPass,
		ReasonCodes: []string{},
		Details:     details,
	}, nil
}

type expectedReplayExitCodeGrader struct{}

func (expectedReplayExitCodeGrader) Name() string { return "expected_exit_code" }

func (expectedReplayExitCodeGrader) Deterministic() bool { return true }

func (expectedReplayExitCodeGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	replayResult, err := runpack.ReplayStub(ctx.Fixture.RunpackPath)
	if err != nil {
		return failResult("expected_exit_code", "replay_error", map[string]any{
			"error": err.Error(),
		}), nil
	}

	expected := ctx.Fixture.Meta.ExpectedReplayExitCode
	actual := 0
	if len(replayResult.MissingResults) > 0 {
		actual = replayMissingExitCode
	}
	details := map[string]any{
		"expected_exit_code": expected,
		"actual_exit_code":   actual,
		"missing_results":    replayResult.MissingResults,
	}

	if actual != expected {
		return schemaregress.GraderResult{
			Name:        "expected_exit_code",
			Status:      regressStatusFail,
			ReasonCodes: []string{"unexpected_exit_code"},
			Details:     details,
		}, nil
	}
	return schemaregress.GraderResult{
		Name:        "expected_exit_code",
		Status:      regressStatusPass,
		ReasonCodes: []string{},
		Details:     details,
	}, nil
}

type trajectoryGrader struct{}

func (trajectoryGrader) Name() string { return "trajectory" }

func (trajectoryGrader) Deterministic() bool { return true }

func (trajectoryGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	candidatePath, err := resolveCandidatePath(ctx.Fixture)
	if err != nil {
		return failResult("trajectory", "candidate_path_invalid", map[string]any{
			"error": err.Error(),
		}), nil
	}

	candidatePack, err := runpack.ReadRunpack(candidatePath)
	if err != nil {
		return failResult("trajectory", "candidate_runpack_invalid", map[string]any{
			"error": err.Error(),
		}), nil
	}

	expectedTools := normalizeTrajectorySequence(ctx.Fixture.Meta.ExpectedToolSequence)
	expectedVerdicts := ctx.Fixture.Meta.ExpectedVerdictSequence
	actualTools := toolSequenceFromRunpack(candidatePack)
	actualVerdicts := verdictSequenceFromRunpack(candidatePack)

	details := map[string]any{
		"candidate_runpack":         candidatePath,
		"expected_tool_sequence":    expectedTools,
		"actual_tool_sequence":      actualTools,
		"expected_verdict_sequence": expectedVerdicts,
		"actual_verdict_sequence":   actualVerdicts,
	}

	reasonCodes := []string{}
	if len(expectedTools) > 0 && !equalOrderedSequence(expectedTools, actualTools) {
		reasonCodes = append(reasonCodes, "unexpected_tool_sequence")
		index, expectedValue, actualValue := firstSequenceMismatch(expectedTools, actualTools)
		details["tool_sequence_mismatch_index"] = index
		details["tool_sequence_expected_at_mismatch"] = expectedValue
		details["tool_sequence_actual_at_mismatch"] = actualValue
	}
	if len(expectedVerdicts) > 0 && !equalOrderedSequence(expectedVerdicts, actualVerdicts) {
		reasonCodes = append(reasonCodes, "unexpected_verdict_sequence")
		index, expectedValue, actualValue := firstSequenceMismatch(expectedVerdicts, actualVerdicts)
		details["verdict_sequence_mismatch_index"] = index
		details["verdict_sequence_expected_at_mismatch"] = expectedValue
		details["verdict_sequence_actual_at_mismatch"] = actualValue
	}

	if len(reasonCodes) > 0 {
		return schemaregress.GraderResult{
			Name:        "trajectory",
			Status:      regressStatusFail,
			ReasonCodes: uniqueSortedStrings(reasonCodes),
			Details:     details,
		}, nil
	}

	passReasons := []string{}
	if len(expectedTools) == 0 && len(expectedVerdicts) == 0 {
		passReasons = append(passReasons, "trajectory_not_required")
	} else {
		passReasons = append(passReasons, "trajectory_match")
	}
	return schemaregress.GraderResult{
		Name:        "trajectory",
		Status:      regressStatusPass,
		ReasonCodes: passReasons,
		Details:     details,
	}, nil
}

type diffGrader struct{}

func (diffGrader) Name() string { return "diff" }

func (diffGrader) Deterministic() bool { return true }

func (diffGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	candidatePath, err := resolveCandidatePath(ctx.Fixture)
	if err != nil {
		return failResult("diff", "candidate_path_invalid", map[string]any{
			"error": err.Error(),
		}), nil
	}

	diffResult, err := runpack.DiffRunpacks(ctx.Fixture.RunpackPath, candidatePath, runpack.DiffPrivacy("full"))
	if err != nil {
		return failResult("diff", "diff_error", map[string]any{
			"error": err.Error(),
		}), nil
	}

	changedFiles := summarizeChangedFiles(diffResult.Summary)
	allowedFiles := uniqueSortedStrings(ctx.Fixture.Meta.DiffAllowChangedFiles)
	allowedSet := make(map[string]struct{}, len(allowedFiles))
	for _, allowed := range allowedFiles {
		allowedSet[allowed] = struct{}{}
	}
	unexpected := []string{}
	for _, changed := range changedFiles {
		if _, ok := allowedSet[changed]; !ok {
			unexpected = append(unexpected, changed)
		}
	}

	details := map[string]any{
		"candidate_runpack": candidatePath,
		"changed_files":     changedFiles,
		"allowed_files":     allowedFiles,
	}

	if len(unexpected) > 0 {
		details["unexpected_files"] = unexpected
		return schemaregress.GraderResult{
			Name:        "diff",
			Status:      regressStatusFail,
			ReasonCodes: []string{"unexpected_diff"},
			Details:     details,
		}, nil
	}

	return schemaregress.GraderResult{
		Name:        "diff",
		Status:      regressStatusPass,
		ReasonCodes: []string{},
		Details:     details,
	}, nil
}

type contextConformanceGrader struct {
	enforce           bool
	allowRuntimeDrift bool
}

func (contextConformanceGrader) Name() string { return "context_conformance" }

func (contextConformanceGrader) Deterministic() bool { return true }

func (grader contextConformanceGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	candidatePath, err := resolveCandidatePath(ctx.Fixture)
	if err != nil {
		result := failResult("context_conformance", "candidate_path_invalid", map[string]any{
			"error": err.Error(),
		})
		result.ContextConformance = "missing"
		return result, nil
	}
	sourcePack, err := runpack.ReadRunpack(ctx.Fixture.RunpackPath)
	if err != nil {
		result := failResult("context_conformance", "source_runpack_invalid", map[string]any{
			"error": err.Error(),
		})
		result.ContextConformance = "missing"
		return result, nil
	}
	candidatePack, err := runpack.ReadRunpack(candidatePath)
	if err != nil {
		result := failResult("context_conformance", "candidate_runpack_invalid", map[string]any{
			"error": err.Error(),
		})
		result.ContextConformance = "missing"
		return result, nil
	}

	classification, changed, runtimeOnly, err := contextproof.ClassifyRefsDrift(sourcePack.Refs, candidatePack.Refs)
	if err != nil {
		result := failResult("context_conformance", "context_diff_error", map[string]any{
			"error": err.Error(),
		})
		result.ContextConformance = "missing"
		return result, nil
	}

	details := map[string]any{
		"candidate_runpack":            candidatePath,
		"context_set_digest_fixture":   sourcePack.Refs.ContextSetDigest,
		"context_set_digest_candidate": candidatePack.Refs.ContextSetDigest,
		"context_changed":              changed,
		"context_runtime_only":         runtimeOnly,
	}

	enforce := grader.enforce || strings.TrimSpace(ctx.Fixture.Meta.ContextConformance) == "required" || strings.TrimSpace(ctx.Fixture.Meta.ExpectedContextSetDigest) != ""
	allowRuntime := grader.allowRuntimeDrift || ctx.Fixture.Meta.AllowContextRuntimeDrift
	expectedDigest := strings.TrimSpace(ctx.Fixture.Meta.ExpectedContextSetDigest)
	if expectedDigest != "" && !strings.EqualFold(candidatePack.Refs.ContextSetDigest, expectedDigest) && classification != "runtime_only" {
		result := failResult("context_conformance", "context_set_digest_mismatch", details)
		result.ContextConformance = "semantic"
		return result, nil
	}
	if !enforce {
		return schemaregress.GraderResult{
			Name:               "context_conformance",
			Status:             regressStatusPass,
			ReasonCodes:        []string{"context_conformance_not_required"},
			ContextConformance: classification,
			Details:            details,
		}, nil
	}
	if strings.TrimSpace(candidatePack.Refs.ContextSetDigest) == "" {
		result := failResult("context_conformance", "context_evidence_missing", details)
		result.ContextConformance = "missing"
		return result, nil
	}
	if classification == "semantic" {
		result := failResult("context_conformance", "context_semantic_drift", details)
		result.ContextConformance = "semantic"
		return result, nil
	}
	if classification == "runtime_only" && !allowRuntime {
		result := failResult("context_conformance", "context_runtime_drift_blocked", details)
		result.ContextConformance = "runtime_only"
		return result, nil
	}

	return schemaregress.GraderResult{
		Name:               "context_conformance",
		Status:             regressStatusPass,
		ReasonCodes:        []string{"context_conformance_pass"},
		ContextConformance: classification,
		Details:            details,
	}, nil
}

func summarizeChangedFiles(summary runpack.DiffSummary) []string {
	changed := make([]string, 0, 4+len(summary.FilesChanged))
	changed = append(changed, summary.FilesChanged...)
	if summary.ManifestChanged {
		changed = append(changed, "manifest.json")
	}
	if summary.IntentsChanged {
		changed = append(changed, "intents.jsonl")
	}
	if summary.ResultsChanged {
		changed = append(changed, "results.jsonl")
	}
	if summary.RefsChanged {
		changed = append(changed, "refs.json")
	}
	return uniqueSortedStrings(changed)
}

func resolveCandidatePath(fixture fixtureSpec) (string, error) {
	candidate := strings.TrimSpace(fixture.Meta.CandidateRunpack)
	if candidate == "" {
		return fixture.RunpackPath, nil
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(fixture.FixtureDir, filepath.FromSlash(candidate))
	}
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("candidate runpack not found: %w", err)
	}
	return candidate, nil
}

func failResult(name, reason string, details map[string]any) schemaregress.GraderResult {
	return schemaregress.GraderResult{
		Name:        name,
		Status:      regressStatusFail,
		ReasonCodes: []string{reason},
		Details:     details,
	}
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func equalOrderedSequence(expected []string, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return false
		}
	}
	return true
}

func firstSequenceMismatch(expected []string, actual []string) (int, string, string) {
	shared := len(expected)
	if len(actual) < shared {
		shared = len(actual)
	}
	for index := 0; index < shared; index++ {
		if expected[index] != actual[index] {
			return index, expected[index], actual[index]
		}
	}
	switch {
	case len(expected) > len(actual):
		return len(actual), expected[len(actual)], "<missing>"
	case len(actual) > len(expected):
		return len(expected), "<missing>", actual[len(expected)]
	default:
		return -1, "<none>", "<none>"
	}
}

func writeJUnitReport(path string, result schemaregress.RegressResult) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create junit directory: %w", err)
		}
	}

	suites := buildJUnit(result)
	encoded, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return err
	}
	document := append([]byte(xml.Header), encoded...)
	document = append(document, '\n')
	return fsx.WriteFileAtomic(path, document, 0o600)
}

func buildJUnit(result schemaregress.RegressResult) junitTestSuites {
	testCases := make([]junitTestCase, 0, len(result.Graders))
	failureCount := 0
	for _, grader := range result.Graders {
		fixtureName, graderName := splitFixtureGraderName(grader.Name)
		testCase := junitTestCase{
			Name:      graderName,
			ClassName: fixtureName,
			Time:      "0",
		}
		if grader.Status == regressStatusFail {
			failureCount++
			reasons := uniqueSortedStrings(grader.ReasonCodes)
			reasonText := strings.Join(reasons, ",")
			if reasonText == "" {
				reasonText = "failed"
			}
			testCase.Failure = &junitFailure{
				Message: reasonText,
				Type:    "regress_failure",
				Body:    reasonText,
			}
		}
		testCases = append(testCases, testCase)
	}

	suiteName := "gait.regress"
	if result.FixtureSet != "" {
		suiteName += "." + result.FixtureSet
	}
	suite := junitTestSuite{
		Name:      suiteName,
		Tests:     len(testCases),
		Failures:  failureCount,
		Errors:    0,
		Skipped:   0,
		Time:      "0",
		TestCases: testCases,
	}
	return junitTestSuites{
		Tests:    suite.Tests,
		Failures: suite.Failures,
		Errors:   0,
		Skipped:  0,
		Time:     "0",
		Suites:   []junitTestSuite{suite},
	}
}

func splitFixtureGraderName(value string) (string, string) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "regress", value
}
