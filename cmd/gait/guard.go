package main

import (
	"crypto/ed25519"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/guard"
	"github.com/davidahmann/gait/core/sign"
)

type guardPackOutput struct {
	OK           bool     `json:"ok"`
	PackPath     string   `json:"pack_path,omitempty"`
	PackID       string   `json:"pack_id,omitempty"`
	RunID        string   `json:"run_id,omitempty"`
	TemplateID   string   `json:"template_id,omitempty"`
	Controls     int      `json:"controls,omitempty"`
	Rendered     int      `json:"rendered_artifacts,omitempty"`
	ManifestPath string   `json:"manifest_path,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type guardVerifyOutput struct {
	OK              bool                 `json:"ok"`
	Path            string               `json:"path,omitempty"`
	PackID          string               `json:"pack_id,omitempty"`
	RunID           string               `json:"run_id,omitempty"`
	FilesChecked    int                  `json:"files_checked,omitempty"`
	MissingFiles    []string             `json:"missing_files,omitempty"`
	HashMismatches  []guard.HashMismatch `json:"hash_mismatches,omitempty"`
	SignatureStatus string               `json:"signature_status,omitempty"`
	SignatureErrors []string             `json:"signature_errors,omitempty"`
	SignaturesTotal int                  `json:"signatures_total,omitempty"`
	SignaturesValid int                  `json:"signatures_valid,omitempty"`
	Error           string               `json:"error,omitempty"`
}

type guardRetainOutput struct {
	OK           bool                       `json:"ok"`
	RootPath     string                     `json:"root_path,omitempty"`
	ScannedFiles int                        `json:"scanned_files,omitempty"`
	DeletedFiles []guard.RetentionFileEvent `json:"deleted_files,omitempty"`
	KeptFiles    []guard.RetentionFileEvent `json:"kept_files,omitempty"`
	DryRun       bool                       `json:"dry_run,omitempty"`
	Error        string                     `json:"error,omitempty"`
}

type guardEncryptOutput struct {
	OK        bool   `json:"ok"`
	Path      string `json:"path,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
	PlainSHA  string `json:"plain_sha256,omitempty"`
	PlainSize int    `json:"plain_size,omitempty"`
	KeyMode   string `json:"key_mode,omitempty"`
	KeyRef    string `json:"key_ref,omitempty"`
	Error     string `json:"error,omitempty"`
}

type guardDecryptOutput struct {
	OK        bool   `json:"ok"`
	Path      string `json:"path,omitempty"`
	PlainSHA  string `json:"plain_sha256,omitempty"`
	PlainSize int    `json:"plain_size,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runGuard(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build, verify, retain, and optionally encrypt evidence artifacts for audit and incident workflows.")
	}
	if len(arguments) == 0 {
		printGuardUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "pack":
		return runGuardPack(arguments[1:])
	case "verify":
		return runGuardVerify(arguments[1:])
	case "retain":
		return runGuardRetain(arguments[1:])
	case "encrypt":
		return runGuardEncrypt(arguments[1:])
	case "decrypt":
		return runGuardDecrypt(arguments[1:])
	default:
		printGuardUsage()
		return exitInvalidInput
	}
}

func runGuardPack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build an evidence_pack zip with a canonical pack_manifest.json and evidence summaries.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"run":                 true,
		"out":                 true,
		"case-id":             true,
		"inventory":           true,
		"trace":               true,
		"regress":             true,
		"approval-audit":      true,
		"credential-evidence": true,
		"template":            true,
		"key-mode":            true,
		"private-key":         true,
		"private-key-env":     true,
	})
	flagSet := flag.NewFlagSet("guard-pack", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var runPath string
	var outPath string
	var caseID string
	var inventoryCSV string
	var traceCSV string
	var regressCSV string
	var approvalAuditCSV string
	var credentialEvidenceCSV string
	var templateID string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var renderPDF bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&runPath, "run", "", "runpack path or run_id")
	flagSet.StringVar(&outPath, "out", "", "output evidence_pack zip path")
	flagSet.StringVar(&caseID, "case-id", "", "optional case identifier")
	flagSet.StringVar(&inventoryCSV, "inventory", "", "comma-separated inventory snapshot paths")
	flagSet.StringVar(&traceCSV, "trace", "", "comma-separated gate trace paths")
	flagSet.StringVar(&regressCSV, "regress", "", "comma-separated regress result paths")
	flagSet.StringVar(&approvalAuditCSV, "approval-audit", "", "comma-separated approval audit record paths")
	flagSet.StringVar(&credentialEvidenceCSV, "credential-evidence", "", "comma-separated broker credential evidence paths")
	flagSet.StringVar(&templateID, "template", "soc2", "audit template id: soc2|pci|incident_response")
	flagSet.StringVar(&keyMode, "key-mode", string(sign.ModeDev), "signing key mode: dev or prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&renderPDF, "render-pdf", false, "include optional summary.pdf convenience artifact")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGuardPackUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(runPath) == "" && len(remaining) > 0 {
		runPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(runPath) == "" || len(remaining) > 0 {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: "expected --run <run_id|path>"}, exitInvalidInput)
	}

	resolvedRunPath, err := resolveRunpackPath(runPath)
	if err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	keyPair, warnings, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.KeyMode(strings.ToLower(strings.TrimSpace(keyMode))),
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	result, err := guard.BuildPack(guard.BuildOptions{
		RunpackPath:             resolvedRunPath,
		OutputPath:              outPath,
		CaseID:                  caseID,
		InventoryPaths:          parseCSVList(inventoryCSV),
		TracePaths:              parseCSVList(traceCSV),
		RegressPaths:            parseCSVList(regressCSV),
		ApprovalAuditPaths:      parseCSVList(approvalAuditCSV),
		CredentialEvidencePaths: parseCSVList(credentialEvidenceCSV),
		TemplateID:              templateID,
		RenderPDF:               renderPDF,
		AutoDiscoverV12:         true,
		ProducerVersion:         version,
		SignKey:                 keyPair.Private,
	})
	if err != nil {
		return writeGuardPackOutput(jsonOutput, guardPackOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	manifestPath := result.PackPath + "#pack_manifest.json"
	return writeGuardPackOutput(jsonOutput, guardPackOutput{
		OK:           true,
		PackPath:     result.PackPath,
		PackID:       result.Manifest.PackID,
		RunID:        result.Manifest.RunID,
		TemplateID:   result.Manifest.TemplateID,
		Controls:     len(result.Manifest.ControlIndex),
		Rendered:     len(result.Manifest.Rendered),
		ManifestPath: manifestPath,
		Warnings:     warnings,
	}, exitOK)
}

func runGuardVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify an evidence_pack zip offline by checking pack manifest hashes deterministically.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path":            true,
		"profile":         true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("guard-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var pathValue string
	var profile string
	var requireSignature bool
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&pathValue, "path", "", "path to evidence_pack zip")
	flagSet.StringVar(&profile, "profile", string(verifyProfileStandard), "verify profile: standard|strict")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid pack manifest signatures")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGuardVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(pathValue) == "" && len(remaining) > 0 {
		pathValue = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(pathValue) == "" || len(remaining) > 0 {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: "expected <evidence_pack.zip>"}, exitInvalidInput)
	}
	resolvedProfile, err := parseArtifactVerifyProfile(profile)
	if err != nil {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if resolvedProfile == verifyProfileStrict {
		requireSignature = true
	}

	var publicKey ed25519.PublicKey
	keyConfig := sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	}
	if resolvedProfile == verifyProfileStrict && !hasAnyKeySource(keyConfig) {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{
			OK:    false,
			Error: "strict verify profile requires --public-key/--public-key-env or private key source",
		}, exitInvalidInput)
	}
	if hasAnyKeySource(keyConfig) {
		loadedKey, keyErr := sign.LoadVerifyKey(keyConfig)
		if keyErr != nil {
			return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: keyErr.Error()}, exitCodeForError(keyErr, exitInvalidInput))
		}
		publicKey = loadedKey
	}

	result, err := guard.VerifyPackWithOptions(pathValue, guard.VerifyOptions{
		PublicKey:        publicKey,
		RequireSignature: requireSignature,
	})
	if err != nil {
		return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0
	if requireSignature {
		ok = ok && result.SignatureStatus == "verified"
	} else {
		ok = ok && result.SignatureStatus != "failed"
	}
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeGuardVerifyOutput(jsonOutput, guardVerifyOutput{
		OK:              ok,
		Path:            pathValue,
		PackID:          result.PackID,
		RunID:           result.RunID,
		FilesChecked:    result.FilesChecked,
		MissingFiles:    result.MissingFiles,
		HashMismatches:  result.HashMismatches,
		SignatureStatus: result.SignatureStatus,
		SignatureErrors: result.SignatureErrors,
		SignaturesTotal: result.SignaturesTotal,
		SignaturesValid: result.SignaturesValid,
	}, exitCode)
}

func runGuardRetain(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Apply deterministic retention policies to trace and evidence pack artifacts and emit a deletion report.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"root":       true,
		"trace-ttl":  true,
		"pack-ttl":   true,
		"report-out": true,
	})
	flagSet := flag.NewFlagSet("guard-retain", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var rootPath string
	var traceTTL string
	var packTTL string
	var reportPath string
	var dryRun bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&rootPath, "root", ".", "artifact root path")
	flagSet.StringVar(&traceTTL, "trace-ttl", "168h", "retention window for trace_*.json")
	flagSet.StringVar(&packTTL, "pack-ttl", "720h", "retention window for evidence_pack_*.zip")
	flagSet.StringVar(&reportPath, "report-out", "", "optional retention report path")
	flagSet.BoolVar(&dryRun, "dry-run", false, "calculate retention actions without deleting files")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardRetainOutput(jsonOutput, guardRetainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGuardRetainUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeGuardRetainOutput(jsonOutput, guardRetainOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	parsedTraceTTL, err := time.ParseDuration(strings.TrimSpace(traceTTL))
	if err != nil {
		return writeGuardRetainOutput(jsonOutput, guardRetainOutput{OK: false, Error: fmt.Sprintf("parse --trace-ttl: %v", err)}, exitInvalidInput)
	}
	parsedPackTTL, err := time.ParseDuration(strings.TrimSpace(packTTL))
	if err != nil {
		return writeGuardRetainOutput(jsonOutput, guardRetainOutput{OK: false, Error: fmt.Sprintf("parse --pack-ttl: %v", err)}, exitInvalidInput)
	}

	result, err := guard.ApplyRetention(guard.RetentionOptions{
		RootPath:        rootPath,
		TraceTTL:        parsedTraceTTL,
		PackTTL:         parsedPackTTL,
		DryRun:          dryRun,
		ReportOutput:    reportPath,
		ProducerVersion: version,
	})
	if err != nil {
		return writeGuardRetainOutput(jsonOutput, guardRetainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeGuardRetainOutput(jsonOutput, guardRetainOutput{
		OK:           true,
		RootPath:     result.RootPath,
		ScannedFiles: result.ScannedFiles,
		DeletedFiles: result.DeletedFiles,
		KeptFiles:    result.KeptFiles,
		DryRun:       result.DryRun,
	}, exitOK)
}

func runGuardEncrypt(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Encrypt an artifact for local-at-rest storage using env or command key hooks.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"in":               true,
		"out":              true,
		"key-env":          true,
		"key-command":      true,
		"key-command-args": true,
	})
	flagSet := flag.NewFlagSet("guard-encrypt", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var inputPath string
	var outputPath string
	var keyEnv string
	var keyCommand string
	var keyCommandArgs string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&inputPath, "in", "", "input artifact path")
	flagSet.StringVar(&outputPath, "out", "", "output encrypted artifact path")
	flagSet.StringVar(&keyEnv, "key-env", "", "env var containing base64 32-byte key")
	flagSet.StringVar(&keyCommand, "key-command", "", "command that prints base64 32-byte key")
	flagSet.StringVar(&keyCommandArgs, "key-command-args", "", "comma-separated args for key command")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardEncryptOutput(jsonOutput, guardEncryptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGuardEncryptUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(inputPath) == "" && len(remaining) > 0 {
		inputPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(inputPath) == "" || len(remaining) > 0 {
		return writeGuardEncryptOutput(jsonOutput, guardEncryptOutput{OK: false, Error: "expected --in <artifact>"}, exitInvalidInput)
	}

	result, err := guard.EncryptArtifact(guard.EncryptOptions{
		InputPath:       inputPath,
		OutputPath:      outputPath,
		KeyEnv:          keyEnv,
		KeyCommand:      keyCommand,
		KeyCommandArgs:  parseCSVList(keyCommandArgs),
		ProducerVersion: version,
	})
	if err != nil {
		return writeGuardEncryptOutput(jsonOutput, guardEncryptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeGuardEncryptOutput(jsonOutput, guardEncryptOutput{
		OK:        true,
		Path:      result.Path,
		Algorithm: result.Artifact.Algorithm,
		PlainSHA:  result.Artifact.PlainSHA256,
		PlainSize: result.Artifact.PlainSize,
		KeyMode:   result.KeySource.Mode,
		KeyRef:    result.KeySource.Ref,
	}, exitOK)
}

func runGuardDecrypt(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Decrypt a guard encrypted artifact using the configured key hook and verify payload digest.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"in":               true,
		"out":              true,
		"key-env":          true,
		"key-command":      true,
		"key-command-args": true,
	})
	flagSet := flag.NewFlagSet("guard-decrypt", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var inputPath string
	var outputPath string
	var keyEnv string
	var keyCommand string
	var keyCommandArgs string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&inputPath, "in", "", "encrypted artifact path")
	flagSet.StringVar(&outputPath, "out", "", "decrypted output path")
	flagSet.StringVar(&keyEnv, "key-env", "", "env var containing base64 32-byte key")
	flagSet.StringVar(&keyCommand, "key-command", "", "command that prints base64 32-byte key")
	flagSet.StringVar(&keyCommandArgs, "key-command-args", "", "comma-separated args for key command")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeGuardDecryptOutput(jsonOutput, guardDecryptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printGuardDecryptUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(inputPath) == "" && len(remaining) > 0 {
		inputPath = remaining[0]
		remaining = remaining[1:]
	}
	if strings.TrimSpace(inputPath) == "" || len(remaining) > 0 {
		return writeGuardDecryptOutput(jsonOutput, guardDecryptOutput{OK: false, Error: "expected --in <artifact.gaitenc>"}, exitInvalidInput)
	}

	result, err := guard.DecryptArtifact(guard.DecryptOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		KeyEnv:         keyEnv,
		KeyCommand:     keyCommand,
		KeyCommandArgs: parseCSVList(keyCommandArgs),
	})
	if err != nil {
		return writeGuardDecryptOutput(jsonOutput, guardDecryptOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeGuardDecryptOutput(jsonOutput, guardDecryptOutput{
		OK:        true,
		Path:      result.Path,
		PlainSHA:  result.PlainSHA256,
		PlainSize: result.PlainSize,
	}, exitOK)
}

func writeGuardPackOutput(jsonOutput bool, output guardPackOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("guard pack ok: %s\n", output.PackPath)
		return exitCode
	}
	fmt.Printf("guard pack error: %s\n", output.Error)
	return exitCode
}

func writeGuardVerifyOutput(jsonOutput bool, output guardVerifyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("guard verify ok: %s\n", output.Path)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("guard verify error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("guard verify failed: %s\n", output.Path)
	return exitCode
}

func writeGuardRetainOutput(jsonOutput bool, output guardRetainOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("guard retain ok: scanned=%d deleted=%d\n", output.ScannedFiles, len(output.DeletedFiles))
		return exitCode
	}
	fmt.Printf("guard retain error: %s\n", output.Error)
	return exitCode
}

func writeGuardEncryptOutput(jsonOutput bool, output guardEncryptOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("guard encrypt ok: %s\n", output.Path)
		return exitCode
	}
	fmt.Printf("guard encrypt error: %s\n", output.Error)
	return exitCode
}

func writeGuardDecryptOutput(jsonOutput bool, output guardDecryptOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("guard decrypt ok: %s\n", output.Path)
		return exitCode
	}
	fmt.Printf("guard decrypt error: %s\n", output.Error)
	return exitCode
}

func printGuardUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard pack --run <run_id|path> [--inventory <csv>] [--trace <csv>] [--regress <csv>] [--approval-audit <csv>] [--credential-evidence <csv>] [--template soc2|pci|incident_response] [--render-pdf] [--out <evidence_pack.zip>] [--case-id <id>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait guard retain [--root <dir>] [--trace-ttl <duration>] [--pack-ttl <duration>] [--dry-run] [--report-out <path>] [--json] [--explain]")
	fmt.Println("  gait guard encrypt --in <artifact> [--out <artifact.gaitenc>] [--key-env <ENV>|--key-command <cmd> --key-command-args <csv>] [--json] [--explain]")
	fmt.Println("  gait guard decrypt --in <artifact.gaitenc> [--out <artifact>] [--key-env <ENV>|--key-command <cmd> --key-command-args <csv>] [--json] [--explain]")
}

func printGuardPackUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard pack --run <run_id|path> [--inventory <csv>] [--trace <csv>] [--regress <csv>] [--approval-audit <csv>] [--credential-evidence <csv>] [--template soc2|pci|incident_response] [--render-pdf] [--out <evidence_pack.zip>] [--case-id <id>] [--key-mode dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printGuardVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
}

func printGuardRetainUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard retain [--root <dir>] [--trace-ttl <duration>] [--pack-ttl <duration>] [--dry-run] [--report-out <path>] [--json] [--explain]")
}

func printGuardEncryptUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard encrypt --in <artifact> [--out <artifact.gaitenc>] [--key-env <ENV>|--key-command <cmd> --key-command-args <csv>] [--json] [--explain]")
}

func printGuardDecryptUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait guard decrypt --in <artifact.gaitenc> [--out <artifact>] [--key-env <ENV>|--key-command <cmd> --key-command-args <csv>] [--json] [--explain]")
}
