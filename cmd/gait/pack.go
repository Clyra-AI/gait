package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidahmann/gait/core/pack"
	"github.com/davidahmann/gait/core/runpack"
	"github.com/davidahmann/gait/core/sign"
)

const (
	packOutputSchemaID      = "gait.pack.output"
	packOutputSchemaVersion = "1.0.0"
)

type packOutput struct {
	SchemaID      string              `json:"schema_id"`
	SchemaVersion string              `json:"schema_version"`
	OK            bool                `json:"ok"`
	Operation     string              `json:"operation,omitempty"`
	Path          string              `json:"path,omitempty"`
	PackID        string              `json:"pack_id,omitempty"`
	PackType      string              `json:"pack_type,omitempty"`
	SourceRef     string              `json:"source_ref,omitempty"`
	OTelPath      string              `json:"otel_path,omitempty"`
	PostgresSQL   string              `json:"postgres_sql_path,omitempty"`
	Diff          *pack.DiffResult    `json:"diff,omitempty"`
	Inspect       *pack.InspectResult `json:"inspect,omitempty"`
	Verify        *pack.VerifyResult  `json:"verify,omitempty"`
	Export        *pack.ExportRecord  `json:"export,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
	Error         string              `json:"error,omitempty"`
}

func runPack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Build, verify, inspect, and diff PackSpec artifacts for run and job evidence while preserving legacy compatibility.")
	}
	if len(arguments) == 0 {
		printPackUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "build":
		return runPackBuild(arguments[1:])
	case "verify":
		return runPackVerify(arguments[1:])
	case "diff":
		return runPackDiff(arguments[1:])
	case "inspect":
		return runPackInspect(arguments[1:])
	case "export":
		return runPackExport(arguments[1:])
	default:
		printPackUsage()
		return exitInvalidInput
	}
}

func runPackBuild(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"type":            true,
		"from":            true,
		"out":             true,
		"job-root":        true,
		"key-mode":        true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("pack-build", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var packType string
	var from string
	var outPath string
	var jobRoot string
	var keyMode string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&packType, "type", "run", "pack type: run|job|call")
	flagSet.StringVar(&from, "from", "", "run_id|path for run packs; job_id|path for job packs")
	flagSet.StringVar(&outPath, "out", "", "output path for pack_<id>.zip")
	flagSet.StringVar(&jobRoot, "job-root", "./gait-out/jobs", "job runtime root (used for job_id sources)")
	flagSet.StringVar(&keyMode, "key-mode", "none", "signing key mode: none|dev|prod")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private signing key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private signing key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printPackBuildUsage()
		return exitOK
	}
	if strings.TrimSpace(from) == "" && len(flagSet.Args()) > 0 {
		from = flagSet.Args()[0]
	}
	if strings.TrimSpace(from) == "" {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: "--from is required"}, exitInvalidInput)
	}

	mode := strings.ToLower(strings.TrimSpace(keyMode))
	privateSource := strings.TrimSpace(privateKeyPath) != "" || strings.TrimSpace(privateKeyEnv) != ""
	keyPair := sign.KeyPair{}
	warnings := []string{}
	switch mode {
	case "", "none":
		if privateSource {
			loaded, loadedWarnings, loadErr := sign.LoadSigningKey(sign.KeyConfig{
				Mode:           sign.ModeProd,
				PrivateKeyPath: strings.TrimSpace(privateKeyPath),
				PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
			})
			if loadErr != nil {
				return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: loadErr.Error()}, exitCodeForError(loadErr, exitInvalidInput))
			}
			keyPair = loaded
			warnings = append(warnings, loadedWarnings...)
		}
	case string(sign.ModeDev), string(sign.ModeProd):
		loaded, loadedWarnings, loadErr := sign.LoadSigningKey(sign.KeyConfig{
			Mode:           sign.KeyMode(mode),
			PrivateKeyPath: strings.TrimSpace(privateKeyPath),
			PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
		})
		if loadErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: loadErr.Error()}, exitCodeForError(loadErr, exitInvalidInput))
		}
		keyPair = loaded
		warnings = append(warnings, loadedWarnings...)
	default:
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: "--key-mode must be none, dev, or prod"}, exitInvalidInput)
	}

	resolvedType := strings.ToLower(strings.TrimSpace(packType))
	switch resolvedType {
	case string(pack.BuildTypeRun):
		runSource := strings.TrimSpace(from)
		runPath := ""
		if runpack.ContainsSessionChainPath(runSource) {
			checkpoint, chainErr := runpack.ResolveSessionCheckpointRunpack(runSource, "latest")
			if chainErr != nil {
				return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: chainErr.Error()}, exitCodeForError(chainErr, exitInvalidInput))
			}
			runPath = checkpoint.RunpackPath
		} else {
			resolvedPath, resolveErr := resolveRunpackPath(runSource)
			if resolveErr != nil {
				return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: resolveErr.Error()}, exitCodeForError(resolveErr, exitInvalidInput))
			}
			runPath = resolvedPath
		}
		result, buildErr := pack.BuildRunPack(pack.BuildRunOptions{
			RunpackPath:       runPath,
			OutputPath:        strings.TrimSpace(outPath),
			ProducerVersion:   version,
			SigningPrivateKey: keyPair.Private,
		})
		if buildErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: buildErr.Error()}, exitCodeForError(buildErr, exitInvalidInput))
		}
		return writePackOutput(jsonOutput, packOutput{OK: true, Operation: "build", Path: result.Path, PackID: result.Manifest.PackID, PackType: result.Manifest.PackType, SourceRef: result.Manifest.SourceRef, Warnings: warnings}, exitOK)
	case string(pack.BuildTypeJob):
		root, jobID, resolveErr := resolveJobSource(strings.TrimSpace(from), strings.TrimSpace(jobRoot))
		if resolveErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: resolveErr.Error()}, exitCodeForError(resolveErr, exitInvalidInput))
		}
		result, buildErr := pack.BuildJobPackFromPath(root, jobID, strings.TrimSpace(outPath), version, keyPair.Private)
		if buildErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: buildErr.Error()}, exitCodeForError(buildErr, exitInvalidInput))
		}
		return writePackOutput(jsonOutput, packOutput{OK: true, Operation: "build", Path: result.Path, PackID: result.Manifest.PackID, PackType: result.Manifest.PackType, SourceRef: result.Manifest.SourceRef, Warnings: warnings}, exitOK)
	case string(pack.BuildTypeCall):
		result, buildErr := pack.BuildCallPack(pack.BuildCallOptions{
			CallRecordPath:    strings.TrimSpace(from),
			OutputPath:        strings.TrimSpace(outPath),
			ProducerVersion:   version,
			SigningPrivateKey: keyPair.Private,
		})
		if buildErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: buildErr.Error()}, exitCodeForError(buildErr, exitInvalidInput))
		}
		return writePackOutput(jsonOutput, packOutput{OK: true, Operation: "build", Path: result.Path, PackID: result.Manifest.PackID, PackType: result.Manifest.PackType, SourceRef: result.Manifest.SourceRef, Warnings: warnings}, exitOK)
	default:
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "build", Error: "--type must be run, job, or call"}, exitInvalidInput)
	}
}

func runPackVerify(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path":            true,
		"profile":         true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("pack-verify", flag.ContinueOnError)
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

	flagSet.StringVar(&pathValue, "path", "", "path to pack artifact zip")
	flagSet.StringVar(&profile, "profile", string(verifyProfileStandard), "verify profile: standard|strict")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid signatures")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printPackVerifyUsage()
		return exitOK
	}
	if strings.TrimSpace(pathValue) == "" && len(flagSet.Args()) > 0 {
		pathValue = flagSet.Args()[0]
	}
	if strings.TrimSpace(pathValue) == "" {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: "expected <pack.zip>"}, exitInvalidInput)
	}
	resolvedProfile, err := parseArtifactVerifyProfile(profile)
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: err.Error()}, exitInvalidInput)
	}
	if resolvedProfile == verifyProfileStrict {
		requireSignature = true
	}
	keyConfig := sign.KeyConfig{
		PublicKeyPath:  strings.TrimSpace(publicKeyPath),
		PublicKeyEnv:   strings.TrimSpace(publicKeyEnv),
		PrivateKeyPath: strings.TrimSpace(privateKeyPath),
		PrivateKeyEnv:  strings.TrimSpace(privateKeyEnv),
	}
	if resolvedProfile == verifyProfileStrict && !hasAnyKeySource(keyConfig) {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: "strict verify profile requires --public-key/--public-key-env or private key source"}, exitInvalidInput)
	}
	publicKey, err := loadOptionalVerifyKey(keyConfig)
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := pack.Verify(strings.TrimSpace(pathValue), pack.VerifyOptions{PublicKey: publicKey, RequireSignature: requireSignature})
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "verify", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0 && len(result.UndeclaredFiles) == 0
	if requireSignature {
		ok = ok && result.SignatureStatus == "verified"
	}
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writePackOutput(jsonOutput, packOutput{OK: ok, Operation: "verify", Path: strings.TrimSpace(pathValue), PackID: result.PackID, PackType: result.PackType, SourceRef: result.SourceRef, Verify: &result}, exitCode)
}

func runPackDiff(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"output": true})
	flagSet := flag.NewFlagSet("pack-diff", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var outputPath string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&outputPath, "output", "", "write diff JSON to path")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "diff", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printPackDiffUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 2 {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "diff", Error: "expected <left> <right>"}, exitInvalidInput)
	}
	result, err := pack.Diff(strings.TrimSpace(remaining[0]), strings.TrimSpace(remaining[1]))
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "diff", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if strings.TrimSpace(outputPath) != "" {
		payload, marshalErr := json.Marshal(result.Result)
		if marshalErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "diff", Error: marshalErr.Error()}, exitCodeForError(marshalErr, exitInvalidInput))
		}
		if writeErr := os.WriteFile(strings.TrimSpace(outputPath), payload, 0o600); writeErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "diff", Error: writeErr.Error()}, exitCodeForError(writeErr, exitInvalidInput))
		}
	}
	ok := !result.Result.Summary.Changed
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writePackOutput(jsonOutput, packOutput{OK: ok, Operation: "diff", Path: strings.TrimSpace(outputPath), Diff: &result}, exitCode)
}

func runPackInspect(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{"path": true})
	flagSet := flag.NewFlagSet("pack-inspect", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var pathValue string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&pathValue, "path", "", "path to pack artifact zip")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "inspect", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printPackInspectUsage()
		return exitOK
	}
	if strings.TrimSpace(pathValue) == "" && len(flagSet.Args()) > 0 {
		pathValue = flagSet.Args()[0]
	}
	if strings.TrimSpace(pathValue) == "" {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "inspect", Error: "expected <pack.zip>"}, exitInvalidInput)
	}
	result, err := pack.Inspect(strings.TrimSpace(pathValue))
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "inspect", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writePackOutput(jsonOutput, packOutput{OK: true, Operation: "inspect", Path: strings.TrimSpace(pathValue), PackID: result.PackID, PackType: result.PackType, SourceRef: result.SourceRef, Inspect: &result}, exitOK)
}

func runPackExport(arguments []string) int {
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path":                 true,
		"otel-out":             true,
		"postgres-sql-out":     true,
		"postgres-table":       true,
		"postgres-include-ddl": false,
	})
	flagSet := flag.NewFlagSet("pack-export", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var pathValue string
	var otelOut string
	var postgresSQLOut string
	var postgresTable string
	var postgresIncludeDDL bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&pathValue, "path", "", "path to pack artifact zip")
	flagSet.StringVar(&otelOut, "otel-out", "", "optional OTEL-style JSONL export path")
	flagSet.StringVar(&postgresSQLOut, "postgres-sql-out", "", "optional path to emit PostgreSQL upsert SQL for pack metadata index")
	flagSet.StringVar(&postgresTable, "postgres-table", "gait_pack_index", "PostgreSQL table name for metadata index SQL (<table> or <schema.table>)")
	flagSet.BoolVar(&postgresIncludeDDL, "postgres-include-ddl", true, "include CREATE TABLE and index DDL in PostgreSQL SQL output")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printPackExportUsage()
		return exitOK
	}
	if strings.TrimSpace(pathValue) == "" && len(flagSet.Args()) > 0 {
		pathValue = flagSet.Args()[0]
	}
	if strings.TrimSpace(pathValue) == "" {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: "expected <pack.zip>"}, exitInvalidInput)
	}

	resolvedOTelOut := strings.TrimSpace(otelOut)
	resolvedPostgresOut := strings.TrimSpace(postgresSQLOut)
	if resolvedOTelOut == "" && resolvedPostgresOut == "" {
		return writePackOutput(jsonOutput, packOutput{
			OK:        false,
			Operation: "export",
			Error:     "at least one export output is required (--otel-out or --postgres-sql-out)",
		}, exitInvalidInput)
	}

	record, err := pack.BuildExportRecord(strings.TrimSpace(pathValue))
	if err != nil {
		return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	if resolvedOTelOut != "" {
		if err := pack.WriteOTelJSONL(resolvedOTelOut, record); err != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
		}
	}
	if resolvedPostgresOut != "" {
		sqlPayload, sqlErr := pack.BuildPostgresIndexSQL(record, pack.PostgresSQLOptions{
			Table:      strings.TrimSpace(postgresTable),
			IncludeDDL: postgresIncludeDDL,
		})
		if sqlErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: sqlErr.Error()}, exitCodeForError(sqlErr, exitInvalidInput))
		}
		if writeErr := os.WriteFile(resolvedPostgresOut, sqlPayload, 0o600); writeErr != nil {
			return writePackOutput(jsonOutput, packOutput{OK: false, Operation: "export", Error: writeErr.Error()}, exitCodeForError(writeErr, exitInvalidInput))
		}
	}

	return writePackOutput(jsonOutput, packOutput{
		OK:          true,
		Operation:   "export",
		Path:        strings.TrimSpace(pathValue),
		PackID:      record.PackID,
		PackType:    record.PackType,
		SourceRef:   record.SourceRef,
		OTelPath:    resolvedOTelOut,
		PostgresSQL: resolvedPostgresOut,
		Export:      &record,
	}, exitOK)
}

func writePackOutput(jsonOutput bool, output packOutput, exitCode int) int {
	output.SchemaID = packOutputSchemaID
	output.SchemaVersion = packOutputSchemaVersion
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		switch output.Operation {
		case "build":
			fmt.Printf("pack build ok: %s (%s)\n", output.Path, output.PackType)
		case "verify":
			fmt.Printf("pack verify ok: %s\n", output.Path)
		case "inspect":
			fmt.Printf("pack inspect ok: %s (%s)\n", output.PackID, output.PackType)
		case "diff":
			fmt.Printf("pack diff ok\n")
		case "export":
			fmt.Printf("pack export ok: %s\n", output.Path)
			if strings.TrimSpace(output.OTelPath) != "" {
				fmt.Printf("otel: %s\n", output.OTelPath)
			}
			if strings.TrimSpace(output.PostgresSQL) != "" {
				fmt.Printf("postgres_sql: %s\n", output.PostgresSQL)
			}
		}
		if len(output.Warnings) > 0 {
			fmt.Printf("warnings: %s\n", strings.Join(output.Warnings, "; "))
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("pack %s error: %s\n", output.Operation, output.Error)
	}
	return exitCode
}

func resolveJobSource(from string, defaultRoot string) (string, string, error) {
	trimmed := strings.TrimSpace(from)
	if trimmed == "" {
		return "", "", fmt.Errorf("job source is required")
	}
	if info, err := os.Stat(trimmed); err == nil {
		if info.IsDir() {
			return filepath.Dir(trimmed), filepath.Base(trimmed), nil
		}
		if strings.EqualFold(filepath.Base(trimmed), "state.json") {
			jobDir := filepath.Dir(trimmed)
			return filepath.Dir(jobDir), filepath.Base(jobDir), nil
		}
	}
	root := strings.TrimSpace(defaultRoot)
	if root == "" {
		root = "./gait-out/jobs"
	}
	return root, trimmed, nil
}

func printPackUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack build --type <run|job|call> --from <run_id|path|job_id|job_path|call_record.json> [--out <pack.zip>] [--job-root ./gait-out/jobs] [--key-mode none|dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait pack verify <pack.zip> [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait pack inspect <pack.zip> [--json] [--explain]")
	fmt.Println("  gait pack diff <left.zip> <right.zip> [--output <diff.json>] [--json] [--explain]")
	fmt.Println("  gait pack export <pack.zip> [--otel-out <otel.jsonl>] [--postgres-sql-out <pack_index.sql>] [--postgres-table <table|schema.table>] [--postgres-include-ddl=true|false] [--json] [--explain]")
}

func printPackBuildUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack build --type <run|job|call> --from <run_id|path|job_id|job_path|call_record.json> [--out <pack.zip>] [--job-root ./gait-out/jobs] [--key-mode none|dev|prod] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printPackVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack verify <pack.zip> [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--private-key <path>|--private-key-env <VAR>] [--json] [--explain]")
}

func printPackInspectUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack inspect <pack.zip> [--json] [--explain]")
}

func printPackDiffUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack diff <left.zip> <right.zip> [--output <diff.json>] [--json] [--explain]")
}

func printPackExportUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait pack export <pack.zip> [--otel-out <otel.jsonl>] [--postgres-sql-out <pack_index.sql>] [--postgres-table <table|schema.table>] [--postgres-include-ddl=true|false] [--json] [--explain]")
}
