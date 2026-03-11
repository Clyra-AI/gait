package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/projectconfig"
	"github.com/Clyra-AI/gait/core/runpack"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
	"github.com/Clyra-AI/gait/core/scout"
)

const (
	captureReceiptSchemaID      = "gait.capture.receipt"
	captureReceiptSchemaVersion = "1.0.0"
)

type surfaceContract struct {
	RepoPolicyPath           string `json:"repo_policy_path"`
	ProjectDefaultsPath      string `json:"project_defaults_path"`
	RegressConfigPath        string `json:"regress_config_path"`
	LegacyPolicyScaffoldPath string `json:"legacy_policy_scaffold_path"`
}

type repoDetection struct {
	Frameworks []string `json:"frameworks,omitempty"`
	ToolNames  []string `json:"tool_names,omitempty"`
}

type initOutput struct {
	OK           bool            `json:"ok"`
	Template     string          `json:"template,omitempty"`
	PolicyPath   string          `json:"policy_path,omitempty"`
	Detection    repoDetection   `json:"detection,omitempty"`
	Contract     surfaceContract `json:"contract"`
	NextCommands []string        `json:"next_commands,omitempty"`
	Error        string          `json:"error,omitempty"`
}

type checkOutput struct {
	OK             bool            `json:"ok"`
	PolicyPath     string          `json:"policy_path,omitempty"`
	DefaultVerdict string          `json:"default_verdict,omitempty"`
	RuleCount      int             `json:"rule_count,omitempty"`
	Detection      repoDetection   `json:"detection,omitempty"`
	Contract       surfaceContract `json:"contract"`
	GapWarnings    []string        `json:"gap_warnings,omitempty"`
	Summary        string          `json:"summary,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type captureOutput struct {
	SchemaID         string `json:"schema_id"`
	SchemaVersion    string `json:"schema_version"`
	OK               bool   `json:"ok"`
	From             string `json:"from,omitempty"`
	ArtifactType     string `json:"artifact_type,omitempty"`
	ArtifactPath     string `json:"artifact_path,omitempty"`
	SessionChainPath string `json:"session_chain_path,omitempty"`
	RunID            string `json:"run_id,omitempty"`
	OutputPath       string `json:"output_path,omitempty"`
	Error            string `json:"error,omitempty"`
}

func runInit(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Write a deterministic starter repo policy at .gait.yaml and report local framework hints without changing existing regress or project-defaults files.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"template": true,
		"out":      true,
	})

	flagSet := flag.NewFlagSet("init", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var templateName string
	var outPath string
	var force bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&templateName, "template", "baseline-highrisk", "starter template: baseline-lowrisk|baseline-mediumrisk|baseline-highrisk")
	flagSet.StringVar(&outPath, "out", projectconfig.RepoPolicyPath, "output path for generated repo policy")
	flagSet.BoolVar(&force, "force", false, "overwrite existing output path")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeInitOutput(jsonOutput, initOutput{OK: false, Contract: currentSurfaceContract(), Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printInitUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeInitOutput(jsonOutput, initOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "unexpected positional arguments",
		}, exitInvalidInput)
	}

	templateKey := strings.ToLower(strings.TrimSpace(templateName))
	resolvedTemplate, ok := policyTemplateAliases[templateKey]
	if !ok {
		return writeInitOutput(jsonOutput, initOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "unknown template: " + templateKey,
		}, exitInvalidInput)
	}
	templateBody, ok := policyTemplates[resolvedTemplate]
	if !ok {
		return writeInitOutput(jsonOutput, initOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "template is not available: " + resolvedTemplate,
		}, exitInvalidInput)
	}

	detection, err := detectRepoSurface(".")
	if err != nil {
		return writeInitOutput(jsonOutput, initOutput{OK: false, Contract: currentSurfaceContract(), Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	trimmedOutPath := strings.TrimSpace(outPath)
	if trimmedOutPath == "" {
		return writeInitOutput(jsonOutput, initOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "output path must not be empty",
		}, exitInvalidInput)
	}
	if !force {
		if _, err := os.Stat(trimmedOutPath); err == nil {
			return writeInitOutput(jsonOutput, initOutput{
				OK:       false,
				Contract: currentSurfaceContract(),
				Error:    "output path already exists (use --force to overwrite): " + trimmedOutPath,
			}, exitInvalidInput)
		}
	}

	parentDir := filepath.Dir(trimmedOutPath)
	if parentDir != "." && parentDir != "" {
		if err := os.MkdirAll(parentDir, 0o750); err != nil {
			return writeInitOutput(jsonOutput, initOutput{OK: false, Contract: currentSurfaceContract(), Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}

	rendered := renderInitPolicy(templateBody, detection)
	if err := os.WriteFile(trimmedOutPath, []byte(rendered), 0o600); err != nil {
		return writeInitOutput(jsonOutput, initOutput{OK: false, Contract: currentSurfaceContract(), Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeInitOutput(jsonOutput, initOutput{
		OK:         true,
		Template:   resolvedTemplate,
		PolicyPath: trimmedOutPath,
		Detection:  detection,
		Contract:   currentSurfaceContract(),
		NextCommands: []string{
			fmt.Sprintf("gait check --policy %s --json", trimmedOutPath),
			fmt.Sprintf("gait policy validate %s --json", trimmedOutPath),
			fmt.Sprintf("gait gate eval --policy %s --intent examples/policy/intents/intent_delete.json --json", trimmedOutPath),
		},
	}, exitOK)
}

func runCheck(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Validate a repo policy file, then report deterministic gap warnings against local tool and framework hints without mutating the workspace.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"policy": true,
	})

	flagSet := flag.NewFlagSet("check", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var policyPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&policyPath, "policy", projectconfig.RepoPolicyPath, "path to repo policy")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeCheckOutput(jsonOutput, checkOutput{OK: false, Contract: currentSurfaceContract(), Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printCheckUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(policyPath) == "" && len(remaining) == 1 {
		policyPath = remaining[0]
		remaining = nil
	}
	if len(remaining) > 0 {
		return writeCheckOutput(jsonOutput, checkOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "unexpected positional arguments",
		}, exitInvalidInput)
	}

	trimmedPolicyPath := strings.TrimSpace(policyPath)
	if trimmedPolicyPath == "" {
		return writeCheckOutput(jsonOutput, checkOutput{
			OK:       false,
			Contract: currentSurfaceContract(),
			Error:    "policy path must not be empty",
		}, exitInvalidInput)
	}

	policy, err := gate.LoadPolicyFile(trimmedPolicyPath)
	if err != nil {
		return writeCheckOutput(jsonOutput, checkOutput{
			OK:         false,
			PolicyPath: trimmedPolicyPath,
			Contract:   currentSurfaceContract(),
			Error:      err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}

	detection, err := detectRepoSurface(".")
	if err != nil {
		return writeCheckOutput(jsonOutput, checkOutput{
			OK:         false,
			PolicyPath: trimmedPolicyPath,
			Contract:   currentSurfaceContract(),
			Error:      err.Error(),
		}, exitCodeForError(err, exitInvalidInput))
	}

	gapWarnings := buildPolicyGapWarnings(policy, detection)
	summary := fmt.Sprintf(
		"policy ok: default_verdict=%s rules=%d gap_warnings=%d",
		policy.DefaultVerdict,
		len(policy.Rules),
		len(gapWarnings),
	)
	return writeCheckOutput(jsonOutput, checkOutput{
		OK:             true,
		PolicyPath:     trimmedPolicyPath,
		DefaultVerdict: policy.DefaultVerdict,
		RuleCount:      len(policy.Rules),
		Detection:      detection,
		Contract:       currentSurfaceContract(),
		GapWarnings:    gapWarnings,
		Summary:        summary,
	}, exitOK)
}

func runCapture(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Resolve an explicit runpack, trace, or session-chain source and persist a portable capture receipt for later regress import.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"from":       true,
		"out":        true,
		"checkpoint": true,
	})

	flagSet := flag.NewFlagSet("capture", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var from string
	var outPath string
	var checkpointRef string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&from, "from", "", "run_id, runpack path, trace path, or session_chain.json path")
	flagSet.StringVar(&outPath, "out", projectconfig.DefaultCapturePath, "path to write capture receipt JSON")
	flagSet.StringVar(&checkpointRef, "checkpoint", "latest", "checkpoint index or latest when --from is session_chain.json")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeCaptureOutput(jsonOutput, captureOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printCaptureUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeCaptureOutput(jsonOutput, captureOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(from) == "" {
		return writeCaptureOutput(jsonOutput, captureOutput{OK: false, Error: "missing required --from <run_id|path>"}, exitInvalidInput)
	}

	output, exitCode := resolveCaptureOutput(from, checkpointRef)
	if exitCode != exitOK {
		return writeCaptureOutput(jsonOutput, output, exitCode)
	}
	output.OutputPath = strings.TrimSpace(outPath)
	if err := writeCaptureReceipt(output.OutputPath, output); err != nil {
		return writeCaptureOutput(jsonOutput, captureOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeCaptureOutput(jsonOutput, output, exitOK)
}

func resolveCaptureOutput(from string, checkpointRef string) (captureOutput, int) {
	trimmedFrom := strings.TrimSpace(from)
	if trimmedFrom == "" {
		return captureOutput{OK: false, Error: "missing required --from <run_id|path>"}, exitInvalidInput
	}

	if strings.HasSuffix(strings.ToLower(trimmedFrom), ".json") && fileExists(trimmedFrom) {
		if checkpoint, err := runpack.ResolveSessionCheckpointRunpack(trimmedFrom, strings.TrimSpace(checkpointRef)); err == nil {
			return captureOutput{
				SchemaID:         captureReceiptSchemaID,
				SchemaVersion:    captureReceiptSchemaVersion,
				OK:               true,
				From:             trimmedFrom,
				ArtifactType:     "runpack",
				ArtifactPath:     checkpoint.RunpackPath,
				SessionChainPath: trimmedFrom,
				RunID:            checkpoint.RunID,
			}, exitOK
		}
		if _, err := gate.ReadTraceRecord(trimmedFrom); err == nil {
			return captureOutput{
				SchemaID:      captureReceiptSchemaID,
				SchemaVersion: captureReceiptSchemaVersion,
				OK:            true,
				From:          trimmedFrom,
				ArtifactType:  "trace",
				ArtifactPath:  trimmedFrom,
			}, exitOK
		}
	}

	runpackPath, sessionChainPath, err := resolveRegressSource(trimmedFrom, checkpointRef)
	if err != nil {
		return captureOutput{
			SchemaID:      captureReceiptSchemaID,
			SchemaVersion: captureReceiptSchemaVersion,
			OK:            false,
			From:          trimmedFrom,
			Error:         err.Error(),
		}, exitCodeForError(err, exitInvalidInput)
	}
	verifyResult, err := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{RequireSignature: false})
	if err != nil {
		return captureOutput{
			SchemaID:      captureReceiptSchemaID,
			SchemaVersion: captureReceiptSchemaVersion,
			OK:            false,
			From:          trimmedFrom,
			Error:         err.Error(),
		}, exitCodeForError(err, exitInvalidInput)
	}
	return captureOutput{
		SchemaID:         captureReceiptSchemaID,
		SchemaVersion:    captureReceiptSchemaVersion,
		OK:               true,
		From:             trimmedFrom,
		ArtifactType:     "runpack",
		ArtifactPath:     runpackPath,
		SessionChainPath: sessionChainPath,
		RunID:            verifyResult.RunID,
	}, exitOK
}

func currentSurfaceContract() surfaceContract {
	return surfaceContract{
		RepoPolicyPath:           projectconfig.RepoPolicyPath,
		ProjectDefaultsPath:      projectconfig.DefaultPath,
		RegressConfigPath:        projectconfig.RegressConfigPath,
		LegacyPolicyScaffoldPath: projectconfig.LegacyPolicyScaffoldPath,
	}
}

func detectRepoSurface(root string) (repoDetection, error) {
	provider := scout.DefaultProvider{Options: scout.SnapshotOptions{ProducerVersion: version}}
	snapshot, err := provider.Snapshot(context.Background(), scout.SnapshotRequest{Roots: []string{root}})
	if err != nil {
		return repoDetection{}, fmt.Errorf("detect repo surface: %w", err)
	}
	return summarizeRepoDetection(snapshot), nil
}

func summarizeRepoDetection(snapshot schemascout.InventorySnapshot) repoDetection {
	frameworks := map[string]struct{}{}
	toolNames := map[string]struct{}{}
	for _, item := range snapshot.Items {
		if item.Kind != "tool" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name != "" {
			toolNames[name] = struct{}{}
		}
		for _, tag := range item.Tags {
			trimmed := strings.TrimSpace(tag)
			if strings.HasPrefix(trimmed, "framework:") {
				frameworks[strings.TrimPrefix(trimmed, "framework:")] = struct{}{}
			}
		}
	}
	return repoDetection{
		Frameworks: sortedKeys(frameworks),
		ToolNames:  sortedKeys(toolNames),
	}
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func renderInitPolicy(templateBody string, detection repoDetection) string {
	lines := []string{
		"# gait init starter policy",
		fmt.Sprintf("# repo_policy_path: %s", projectconfig.RepoPolicyPath),
		fmt.Sprintf("# project_defaults_path: %s", projectconfig.DefaultPath),
		fmt.Sprintf("# regress_config_path: %s", projectconfig.RegressConfigPath),
	}
	if len(detection.Frameworks) > 0 {
		lines = append(lines, fmt.Sprintf("# detected_frameworks: %s", strings.Join(detection.Frameworks, ", ")))
	}
	if toolSummary := summarizeDetectedToolsForComment(detection.ToolNames); toolSummary != "" {
		lines = append(lines, fmt.Sprintf("# detected_tools: %s", toolSummary))
	}
	body := strings.TrimLeft(templateBody, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return strings.Join(lines, "\n") + "\n\n" + body
}

func summarizeDetectedToolsForComment(toolNames []string) string {
	if len(toolNames) == 0 {
		return ""
	}
	limit := len(toolNames)
	if limit > 8 {
		limit = 8
	}
	selected := append([]string(nil), toolNames[:limit]...)
	summary := strings.Join(selected, ", ")
	if len(toolNames) > limit {
		summary = fmt.Sprintf("%s (+%d more)", summary, len(toolNames)-limit)
	}
	return summary
}

func buildPolicyGapWarnings(policy gate.Policy, detection repoDetection) []string {
	warnings := []string{}
	if len(policy.Rules) == 0 {
		warnings = append(warnings, "policy defines no explicit rules; only default_verdict will apply")
	}
	if strings.EqualFold(strings.TrimSpace(policy.DefaultVerdict), "allow") {
		warnings = append(warnings, "default_verdict=allow is permissive; high-risk repos usually prefer require_approval or block")
	}

	hasNonAllowRule := false
	matchedToolNames := map[string]struct{}{}
	for _, rule := range policy.Rules {
		effect := strings.ToLower(strings.TrimSpace(rule.Effect))
		if effect == "" {
			effect = strings.ToLower(strings.TrimSpace(rule.Action))
		}
		if effect != "" && effect != "allow" {
			hasNonAllowRule = true
		}
		if toolName := strings.TrimSpace(rule.Match.ToolName); toolName != "" {
			matchedToolNames[toolName] = struct{}{}
		}
		for _, toolName := range rule.Match.ToolNames {
			trimmed := strings.TrimSpace(toolName)
			if trimmed != "" {
				matchedToolNames[trimmed] = struct{}{}
			}
		}
	}
	if !hasNonAllowRule {
		warnings = append(warnings, "policy has no non-allow rules; enforce paths will not block or require approval")
	}
	if len(detection.Frameworks) == 0 {
		warnings = append(warnings, "no supported framework tools were detected locally; review whether this repo has an explicit Gait interception seam yet")
	}
	if len(detection.ToolNames) > 0 && len(matchedToolNames) == 0 {
		warnings = append(warnings, "detected tool inventory but no explicit tool_name/tool_names coverage was found; review policy coverage manually")
	}
	if len(detection.ToolNames) > 0 && len(matchedToolNames) > 0 {
		covered := false
		for _, detectedTool := range detection.ToolNames {
			if _, ok := matchedToolNames[strings.TrimSpace(detectedTool)]; ok {
				covered = true
				break
			}
		}
		if !covered {
			warnings = append(warnings, "detected tool inventory names do not match explicit policy tool_name/tool_names entries; normalize tool naming before rollout")
		}
	}
	return warnings
}

func writeCaptureReceipt(path string, output captureOutput) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("capture output path must not be empty")
	}
	parentDir := filepath.Dir(trimmedPath)
	if parentDir != "." && parentDir != "" {
		if err := os.MkdirAll(parentDir, 0o750); err != nil {
			return fmt.Errorf("create capture output directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("encode capture output: %w", err)
	}
	if err := os.WriteFile(trimmedPath, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write capture output: %w", err)
	}
	return nil
}

func loadCaptureReceipt(path string) (captureOutput, error) {
	// #nosec G304 -- capture receipt path is explicit local user input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return captureOutput{}, fmt.Errorf("read capture receipt: %w", err)
	}
	var output captureOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return captureOutput{}, fmt.Errorf("decode capture receipt: %w", err)
	}
	if output.SchemaID != captureReceiptSchemaID || output.SchemaVersion != captureReceiptSchemaVersion {
		return captureOutput{}, fmt.Errorf("unexpected capture receipt schema")
	}
	if strings.TrimSpace(output.ArtifactPath) == "" {
		return captureOutput{}, fmt.Errorf("capture receipt missing artifact_path")
	}
	return output, nil
}

func writeInitOutput(jsonOutput bool, output initOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("init ok: policy=%s template=%s\n", output.PolicyPath, output.Template)
		if len(output.Detection.Frameworks) > 0 {
			fmt.Printf("detected frameworks: %s\n", strings.Join(output.Detection.Frameworks, ", "))
		}
		return exitCode
	}
	fmt.Printf("init error: %s\n", output.Error)
	return exitCode
}

func writeCheckOutput(jsonOutput bool, output checkOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("check ok: policy=%s rules=%d warnings=%d\n", output.PolicyPath, output.RuleCount, len(output.GapWarnings))
		return exitCode
	}
	fmt.Printf("check error: %s\n", output.Error)
	return exitCode
}

func writeCaptureOutput(jsonOutput bool, output captureOutput, exitCode int) int {
	output.SchemaID = captureReceiptSchemaID
	output.SchemaVersion = captureReceiptSchemaVersion
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("capture ok: type=%s path=%s\n", output.ArtifactType, output.ArtifactPath)
		if output.OutputPath != "" {
			fmt.Printf("receipt: %s\n", output.OutputPath)
		}
		return exitCode
	}
	fmt.Printf("capture error: %s\n", output.Error)
	return exitCode
}

func printInitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait init [--template baseline-lowrisk|baseline-mediumrisk|baseline-highrisk] [--out .gait.yaml] [--force] [--json] [--explain]")
}

func printCheckUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait check [--policy .gait.yaml] [--json] [--explain]")
}

func printCaptureUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait capture --from <run_id|runpack.zip|trace.json|session_chain.json> [--checkpoint latest|<index>] [--out ./gait-out/capture.json] [--json] [--explain]")
}
