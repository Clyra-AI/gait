package main

import (
	"crypto/ed25519"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/gate"
	"github.com/davidahmann/gait/core/guard"
	"github.com/davidahmann/gait/core/runpack"
	"github.com/davidahmann/gait/core/sign"
)

const (
	exitOK                = 0
	exitInternalFailure   = 1
	exitPolicyBlocked     = 3
	exitApprovalRequired  = 4
	exitRegressFailed     = 5
	exitVerifyFailed      = 2
	exitInvalidInput      = 6
	exitMissingDependency = 7
	exitUnsafeReplay      = 8
)

type verifyOutput struct {
	OK              bool                   `json:"ok"`
	Path            string                 `json:"path,omitempty"`
	RunID           string                 `json:"run_id,omitempty"`
	ManifestDigest  string                 `json:"manifest_digest,omitempty"`
	FilesChecked    int                    `json:"files_checked,omitempty"`
	MissingFiles    []string               `json:"missing_files,omitempty"`
	HashMismatches  []runpack.HashMismatch `json:"hash_mismatches,omitempty"`
	SignatureStatus string                 `json:"signature_status,omitempty"`
	SignatureErrors []string               `json:"signature_errors,omitempty"`
	SignaturesTotal int                    `json:"signatures_total,omitempty"`
	SignaturesValid int                    `json:"signatures_valid,omitempty"`
	Error           string                 `json:"error,omitempty"`
}

type verifyChainOutput struct {
	OK    bool               `json:"ok"`
	Run   verifyOutput       `json:"run"`
	Trace *traceVerifyOutput `json:"trace,omitempty"`
	Pack  *guardVerifyOutput `json:"pack,omitempty"`
	Error string             `json:"error,omitempty"`
}

type verifySessionChainOutput struct {
	OK                 bool     `json:"ok"`
	ChainPath          string   `json:"chain_path,omitempty"`
	SessionID          string   `json:"session_id,omitempty"`
	RunID              string   `json:"run_id,omitempty"`
	CheckpointsChecked int      `json:"checkpoints_checked,omitempty"`
	LinkageErrors      []string `json:"linkage_errors,omitempty"`
	CheckpointErrors   []string `json:"checkpoint_errors,omitempty"`
	Error              string   `json:"error,omitempty"`
}

type artifactVerifyProfile string

const (
	verifyProfileStandard artifactVerifyProfile = "standard"
	verifyProfileStrict   artifactVerifyProfile = "strict"
)

func runVerify(arguments []string) int {
	if len(arguments) > 0 && arguments[0] == "chain" {
		return runVerifyChain(arguments[1:])
	}
	if len(arguments) > 0 && arguments[0] == "session-chain" {
		return runVerifySessionChain(arguments[1:])
	}
	return runVerifyRunpack(arguments)
}

func runVerifyRunpack(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify runpack integrity offline: file hashes, manifest digest, and optional signatures.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
		"profile":         true,
	})
	flagSet := flag.NewFlagSet("verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var profile string
	var requireSignature bool
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.StringVar(&profile, "profile", string(verifyProfileStandard), "verify profile: standard|strict")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid signatures")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 1 {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: "expected run_id or path"}, exitInvalidInput)
	}
	resolvedProfile, err := parseArtifactVerifyProfile(profile)
	if err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if resolvedProfile == verifyProfileStrict {
		requireSignature = true
	}

	keyConfig := sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	}
	if resolvedProfile == verifyProfileStrict && !hasAnyKeySource(keyConfig) {
		return writeVerifyOutput(jsonOutput, verifyOutput{
			OK:    false,
			Error: "strict verify profile requires --public-key/--public-key-env or private key source",
		}, exitInvalidInput)
	}
	publicKey, err := loadOptionalVerifyKey(keyConfig)
	if err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	output, err := verifyRunpackArtifact(remaining[0], requireSignature, publicKey)
	if err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	exitCode := exitOK
	if !output.OK {
		exitCode = exitVerifyFailed
	}
	return writeVerifyOutput(jsonOutput, output, exitCode)
}

func runVerifyChain(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify runpack, trace, and evidence pack artifacts together to produce one deterministic integrity verdict.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"run":             true,
		"trace":           true,
		"pack":            true,
		"profile":         true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("verify-chain", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var runPath string
	var tracePath string
	var packPath string
	var profile string
	var requireSignature bool
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&runPath, "run", "", "run_id or runpack path")
	flagSet.StringVar(&tracePath, "trace", "", "path to trace record")
	flagSet.StringVar(&packPath, "pack", "", "path to evidence pack zip")
	flagSet.StringVar(&profile, "profile", string(verifyProfileStandard), "verify profile: standard|strict")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid signatures for runpack/pack")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printVerifyChainUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(runPath) == "" {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: "--run is required"}, exitInvalidInput)
	}
	resolvedProfile, err := parseArtifactVerifyProfile(profile)
	if err != nil {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if resolvedProfile == verifyProfileStrict {
		requireSignature = true
	}

	keyConfig := sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	}
	if resolvedProfile == verifyProfileStrict && !hasAnyKeySource(keyConfig) {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{
			OK:    false,
			Error: "strict verify profile requires --public-key/--public-key-env or private key source",
		}, exitInvalidInput)
	}
	publicKey, err := loadOptionalVerifyKey(keyConfig)
	if err != nil {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	runOutput, err := verifyRunpackArtifact(runPath, requireSignature, publicKey)
	if err != nil {
		return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	output := verifyChainOutput{
		OK:  runOutput.OK,
		Run: runOutput,
	}

	if strings.TrimSpace(tracePath) != "" {
		traceOut, traceErr := verifyTraceArtifact(tracePath, publicKey)
		if traceErr != nil {
			return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Run: runOutput, Error: traceErr.Error()}, exitCodeForError(traceErr, exitInvalidInput))
		}
		output.Trace = &traceOut
		output.OK = output.OK && traceOut.OK
	}

	if strings.TrimSpace(packPath) != "" {
		packOut, packErr := verifyPackArtifact(packPath, requireSignature, publicKey)
		if packErr != nil {
			return writeVerifyChainOutput(jsonOutput, verifyChainOutput{OK: false, Run: runOutput, Trace: output.Trace, Error: packErr.Error()}, exitCodeForError(packErr, exitInvalidInput))
		}
		output.Pack = &packOut
		output.OK = output.OK && packOut.OK
	}

	exitCode := exitOK
	if !output.OK {
		exitCode = exitVerifyFailed
	}
	return writeVerifyChainOutput(jsonOutput, output, exitCode)
}

func runVerifySessionChain(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify session checkpoint chains, including per-checkpoint runpack integrity/signatures and prev_checkpoint_digest linkage continuity.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"chain":           true,
		"profile":         true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("verify-session-chain", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var chainPath string
	var profile string
	var requireSignature bool
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&chainPath, "chain", "", "path to session chain json")
	flagSet.StringVar(&profile, "profile", string(verifyProfileStandard), "verify profile: standard|strict")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid signatures for all checkpoints")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printVerifySessionChainUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if strings.TrimSpace(chainPath) == "" && len(remaining) == 1 {
		chainPath = remaining[0]
		remaining = nil
	}
	if strings.TrimSpace(chainPath) == "" || len(remaining) > 0 {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{OK: false, Error: "expected --chain <session_chain.json>"}, exitInvalidInput)
	}

	resolvedProfile, err := parseArtifactVerifyProfile(profile)
	if err != nil {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if resolvedProfile == verifyProfileStrict {
		requireSignature = true
	}

	keyConfig := sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	}
	if resolvedProfile == verifyProfileStrict && !hasAnyKeySource(keyConfig) {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{
			OK:    false,
			Error: "strict verify profile requires --public-key/--public-key-env or private key source",
		}, exitInvalidInput)
	}
	publicKey, err := loadOptionalVerifyKey(keyConfig)
	if err != nil {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	verifyResult, err := runpack.VerifySessionChain(chainPath, runpack.SessionChainVerifyOptions{
		RequireSignature: requireSignature,
		PublicKey:        publicKey,
	})
	if err != nil {
		return writeVerifySessionChainOutput(jsonOutput, verifySessionChainOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	output := verifySessionChainOutput{
		OK:                 len(verifyResult.LinkageErrors) == 0 && len(verifyResult.CheckpointErrors) == 0,
		ChainPath:          chainPath,
		SessionID:          verifyResult.SessionID,
		RunID:              verifyResult.RunID,
		CheckpointsChecked: verifyResult.CheckpointsChecked,
		LinkageErrors:      verifyResult.LinkageErrors,
		CheckpointErrors:   verifyResult.CheckpointErrors,
	}
	exitCode := exitOK
	if !output.OK {
		exitCode = exitVerifyFailed
	}
	return writeVerifySessionChainOutput(jsonOutput, output, exitCode)
}

func loadOptionalVerifyKey(cfg sign.KeyConfig) (ed25519.PublicKey, error) {
	if !hasAnyKeySource(cfg) {
		return nil, nil
	}
	return sign.LoadVerifyKey(cfg)
}

func verifyRunpackArtifact(runValue string, requireSignature bool, publicKey ed25519.PublicKey) (verifyOutput, error) {
	runpackPath, err := resolveRunpackPath(runValue)
	if err != nil {
		return verifyOutput{}, err
	}
	result, err := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{
		PublicKey:        publicKey,
		RequireSignature: requireSignature,
	})
	if err != nil {
		return verifyOutput{}, err
	}
	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0
	if requireSignature {
		ok = ok && result.SignatureStatus == "verified"
	} else {
		ok = ok && result.SignatureStatus != "failed"
	}
	return verifyOutput{
		OK:              ok,
		Path:            runpackPath,
		RunID:           result.RunID,
		ManifestDigest:  result.ManifestDigest,
		FilesChecked:    result.FilesChecked,
		MissingFiles:    result.MissingFiles,
		HashMismatches:  result.HashMismatches,
		SignatureStatus: result.SignatureStatus,
		SignatureErrors: result.SignatureErrors,
		SignaturesTotal: result.SignaturesTotal,
		SignaturesValid: result.SignaturesValid,
	}, nil
}

func verifyTraceArtifact(path string, publicKey ed25519.PublicKey) (traceVerifyOutput, error) {
	tracePath := strings.TrimSpace(path)
	if tracePath == "" {
		return traceVerifyOutput{}, fmt.Errorf("trace path is required")
	}
	if len(publicKey) == 0 {
		return traceVerifyOutput{}, fmt.Errorf("trace verification requires --public-key/--public-key-env or private key source")
	}
	record, err := gate.ReadTraceRecord(tracePath)
	if err != nil {
		return traceVerifyOutput{}, err
	}
	ok, err := gate.VerifyTraceRecordSignature(record, publicKey)
	if err != nil {
		return traceVerifyOutput{
			OK:              false,
			Path:            tracePath,
			TraceID:         record.TraceID,
			Verdict:         record.Verdict,
			SignatureStatus: "failed",
			Error:           err.Error(),
		}, nil
	}
	status := "failed"
	if ok {
		status = "verified"
	}
	keyID := ""
	if record.Signature != nil {
		keyID = record.Signature.KeyID
	}
	return traceVerifyOutput{
		OK:              ok,
		Path:            tracePath,
		TraceID:         record.TraceID,
		Verdict:         record.Verdict,
		SignatureStatus: status,
		KeyID:           keyID,
	}, nil
}

func verifyPackArtifact(path string, requireSignature bool, publicKey ed25519.PublicKey) (guardVerifyOutput, error) {
	result, err := guard.VerifyPackWithOptions(path, guard.VerifyOptions{
		PublicKey:        publicKey,
		RequireSignature: requireSignature,
	})
	if err != nil {
		return guardVerifyOutput{}, err
	}
	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0
	if requireSignature {
		ok = ok && result.SignatureStatus == "verified"
	} else {
		ok = ok && result.SignatureStatus != "failed"
	}
	return guardVerifyOutput{
		OK:              ok,
		Path:            path,
		PackID:          result.PackID,
		RunID:           result.RunID,
		FilesChecked:    result.FilesChecked,
		MissingFiles:    result.MissingFiles,
		HashMismatches:  result.HashMismatches,
		SignatureStatus: result.SignatureStatus,
		SignatureErrors: result.SignatureErrors,
		SignaturesTotal: result.SignaturesTotal,
		SignaturesValid: result.SignaturesValid,
	}, nil
}

func hasAnyKeySource(cfg sign.KeyConfig) bool {
	return cfg.PrivateKeyPath != "" ||
		cfg.PublicKeyPath != "" ||
		cfg.PrivateKeyEnv != "" ||
		cfg.PublicKeyEnv != ""
}

func parseArtifactVerifyProfile(value string) (artifactVerifyProfile, error) {
	profile := strings.ToLower(strings.TrimSpace(value))
	if profile == "" {
		return verifyProfileStandard, nil
	}
	switch artifactVerifyProfile(profile) {
	case verifyProfileStandard, verifyProfileStrict:
		return artifactVerifyProfile(profile), nil
	default:
		return "", fmt.Errorf("unsupported --profile value %q (expected standard or strict)", value)
	}
}

func writeVerifyOutput(jsonOutput bool, output verifyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}

	if output.OK {
		fmt.Printf("verify ok: %s\n", output.Path)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("verify error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("verify failed: %s\n", output.Path)
	if len(output.MissingFiles) > 0 {
		fmt.Printf("missing files: %s\n", strings.Join(output.MissingFiles, ", "))
	}
	if len(output.HashMismatches) > 0 {
		fmt.Printf("hash mismatches: %d\n", len(output.HashMismatches))
	}
	if output.SignatureStatus != "" {
		fmt.Printf("signature status: %s\n", output.SignatureStatus)
	}
	if len(output.SignatureErrors) > 0 {
		fmt.Printf("signature errors: %s\n", strings.Join(output.SignatureErrors, "; "))
	}
	return exitCode
}

func writeVerifyChainOutput(jsonOutput bool, output verifyChainOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Println("verify chain: ok")
		fmt.Printf("runpack: %s\n", output.Run.Path)
		if output.Trace != nil {
			fmt.Printf("trace: %s (%s)\n", output.Trace.Path, output.Trace.SignatureStatus)
		}
		if output.Pack != nil {
			fmt.Printf("pack: %s (%s)\n", output.Pack.Path, output.Pack.SignatureStatus)
		}
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("verify chain error: %s\n", output.Error)
		return exitCode
	}
	fmt.Println("verify chain failed")
	if output.Run.Path != "" {
		fmt.Printf("runpack: %s\n", output.Run.Path)
	}
	if output.Trace != nil {
		fmt.Printf("trace: %s (%s)\n", output.Trace.Path, output.Trace.SignatureStatus)
	}
	if output.Pack != nil {
		fmt.Printf("pack: %s (%s)\n", output.Pack.Path, output.Pack.SignatureStatus)
	}
	return exitCode
}

func writeVerifySessionChainOutput(jsonOutput bool, output verifySessionChainOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("verify session-chain: ok (%s)\n", output.ChainPath)
		fmt.Printf("session=%s run=%s checkpoints=%d\n", output.SessionID, output.RunID, output.CheckpointsChecked)
		return exitCode
	}
	if output.Error != "" {
		fmt.Printf("verify session-chain error: %s\n", output.Error)
		return exitCode
	}
	fmt.Printf("verify session-chain failed: %s\n", output.ChainPath)
	if len(output.LinkageErrors) > 0 {
		fmt.Printf("linkage errors: %s\n", strings.Join(output.LinkageErrors, "; "))
	}
	if len(output.CheckpointErrors) > 0 {
		fmt.Printf("checkpoint errors: %s\n", strings.Join(output.CheckpointErrors, "; "))
	}
	return exitCode
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait approve --intent-digest <sha256> --policy-digest <sha256> --ttl <duration> --scope <csv> --approver <identity> --reason-code <code> [--json] [--explain]")
	fmt.Println("  gait delegate mint --delegator <identity> --delegate <identity> --scope <csv> --ttl <duration> [--scope-class <value>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--json] [--explain]")
	fmt.Println("  gait delegate verify --token <token.json> [--delegator <identity>] [--delegate <identity>] [--scope <csv>] [--intent-digest <sha256>] [--policy-digest <sha256>] [--json] [--explain]")
	fmt.Println("  gait demo [--json] [--explain]")
	fmt.Println("  gait doctor [--production-readiness] [--json] [--explain]")
	fmt.Println("  gait doctor adoption --from <events.jsonl> [--json] [--explain]")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--profile standard|oss-prod] [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--credential-broker off|stub|env|command] [--json] [--explain]")
	fmt.Println("  gait policy init <baseline-lowrisk|baseline-mediumrisk|baseline-highrisk> [--out gait.policy.yaml] [--force] [--json] [--explain]")
	fmt.Println("  gait policy validate <policy.yaml> [--json] [--explain]")
	fmt.Println("  gait policy fmt <policy.yaml> [--write] [--json] [--explain]")
	fmt.Println("  gait policy simulate --policy <candidate.yaml> --baseline <baseline.yaml> --fixtures <csv files/dirs> [--json] [--explain]")
	fmt.Println("  gait policy test <policy.yaml> <intent_fixture.json> [--json] [--explain]")
	fmt.Println("  gait keys init [--out-dir gait-out/keys] [--prefix gait] [--force] [--json] [--explain]")
	fmt.Println("  gait keys rotate [--out-dir gait-out/keys] [--prefix gait] [--json] [--explain]")
	fmt.Println("  gait keys verify [--private-key <path>|--private-key-env <VAR>] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait trace verify <path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait regress init --from <run_id|path> [--json] [--explain]")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--explain]")
	fmt.Println("  gait run record --input <run_record.json> [--json] [--explain]")
	fmt.Println("  gait run inspect --from <run_id|path> [--json] [--explain]")
	fmt.Println("  gait run replay <run_id|path> [--json] [--real-tools --unsafe-real-tools --allow-tools <csv> --unsafe-real-tools-env <VAR>] [--explain]")
	fmt.Println("  gait run diff <left> <right> [--json] [--explain]")
	fmt.Println("  gait run reduce --from <run_id|path> [--predicate missing_result|non_ok_status] [--json] [--explain]")
	fmt.Println("  gait report top --runs <csv|run_id|dir> [--traces <csv|dir>] [--limit <n>] [--json] [--explain]")
	fmt.Println("  gait scout snapshot [--roots <csv>] [--policy <csv>] [--json] [--explain]")
	fmt.Println("  gait scout diff <left_snapshot.json> <right_snapshot.json> [--json] [--explain]")
	fmt.Println("  gait guard pack --run <run_id|path> [--json] [--explain]")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--profile standard|strict] [--json] [--explain]")
	fmt.Println("  gait guard retain [--root <dir>] [--trace-ttl <duration>] [--pack-ttl <duration>] [--dry-run] [--json] [--explain]")
	fmt.Println("  gait guard encrypt --in <artifact> [--out <artifact.gaitenc>] [--json] [--explain]")
	fmt.Println("  gait guard decrypt --in <artifact.gaitenc> [--out <artifact>] [--json] [--explain]")
	fmt.Println("  gait incident pack --from <run_id|path> [--window <duration>] [--json] [--explain]")
	fmt.Println("  gait registry install --source <path|url> --allow-host <csv> [--json] [--explain]")
	fmt.Println("  gait registry list [--cache-dir <path>] [--json] [--explain]")
	fmt.Println("  gait registry verify --path <registry_pack.json> [--cache-dir <path>] [--json] [--explain]")
	fmt.Println("  gait migrate <artifact_path|run_id> [--out <path>] [--json] [--explain]")
	fmt.Println("  gait mcp proxy --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--json] [--explain]")
	fmt.Println("  gait mcp bridge --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--json] [--explain]")
	fmt.Println("  gait verify <run_id|path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify chain --run <run_id|path> [--trace <trace.json>] [--pack <evidence_pack.zip>] [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait verify session-chain --chain <session_chain.json> [--profile standard|strict] [--require-signature] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait ui [--listen 127.0.0.1:7980] [--open-browser=true|false] [--allow-non-loopback] [--json] [--explain]")
	fmt.Println("  gait version")
}

func printVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait verify <run_id|path> [--json] [--profile standard|strict] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify <run_id|path> [--json] [--profile standard|strict] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify chain --run <run_id|path> [--trace <trace.json>] [--pack <evidence_pack.zip>] [--profile standard|strict] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify chain --run <run_id|path> [--trace <trace.json>] [--pack <evidence_pack.zip>] [--profile standard|strict] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify session-chain --chain <session_chain.json> [--json] [--profile standard|strict] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify session-chain --chain <session_chain.json> [--json] [--profile standard|strict] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
}

func printVerifyChainUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait verify chain --run <run_id|path> [--trace <trace.json>] [--pack <evidence_pack.zip>] [--profile standard|strict] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait verify chain --run <run_id|path> [--trace <trace.json>] [--pack <evidence_pack.zip>] [--profile standard|strict] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--json] [--explain]")
}

func printVerifySessionChainUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait verify session-chain --chain <session_chain.json> [--profile standard|strict] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--json] [--explain]")
	fmt.Println("  gait verify session-chain --chain <session_chain.json> [--profile standard|strict] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--json] [--explain]")
}
