package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/davidahmann/gait/core/runpack"
	"github.com/davidahmann/gait/core/sign"
)

const (
	exitOK                = 0
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

func runVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify runpack integrity offline: file hashes, manifest digest, and optional signatures.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var jsonOutput bool
	var requireSignature bool
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var helpFlag bool

	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&requireSignature, "require-signature", false, "require valid signatures")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if helpFlag {
		printVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if len(remaining) != 1 {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: "expected run_id or path"}, exitInvalidInput)
	}

	runpackPath, err := resolveRunpackPath(remaining[0])
	if err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	var publicKey ed25519.PublicKey
	keyConfig := sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	}
	if hasAnyKeySource(keyConfig) {
		publicKey, err = sign.LoadVerifyKey(keyConfig)
		if err != nil {
			return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
		}
	}

	result, err := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{
		PublicKey:        publicKey,
		RequireSignature: requireSignature,
	})
	if err != nil {
		return writeVerifyOutput(jsonOutput, verifyOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}

	ok := len(result.MissingFiles) == 0 && len(result.HashMismatches) == 0
	if requireSignature {
		ok = ok && result.SignatureStatus == "verified"
	} else {
		ok = ok && result.SignatureStatus != "failed"
	}

	output := verifyOutput{
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
	}

	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeVerifyOutput(jsonOutput, output, exitCode)
}

func hasAnyKeySource(cfg sign.KeyConfig) bool {
	return cfg.PrivateKeyPath != "" ||
		cfg.PublicKeyPath != "" ||
		cfg.PrivateKeyEnv != "" ||
		cfg.PublicKeyEnv != ""
}

func writeVerifyOutput(jsonOutput bool, output verifyOutput, exitCode int) int {
	if jsonOutput {
		encoded, err := json.Marshal(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode output"}`)
			return exitInvalidInput
		}
		fmt.Println(string(encoded))
		return exitCode
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

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait approve --intent-digest <sha256> --policy-digest <sha256> --ttl <duration> --scope <csv> --approver <identity> --reason-code <code> [--json] [--explain]")
	fmt.Println("  gait demo [--explain]")
	fmt.Println("  gait doctor [--json] [--explain]")
	fmt.Println("  gait gate eval --policy <policy.yaml> --intent <intent.json> [--simulate] [--approval-token <token.json>] [--approval-token-chain <csv>] [--credential-broker off|stub|env|command] [--json] [--explain]")
	fmt.Println("  gait policy test <policy.yaml> <intent_fixture.json> [--json] [--explain]")
	fmt.Println("  gait trace verify <path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait regress init --from <run_id|path> [--json] [--explain]")
	fmt.Println("  gait regress run [--config gait.yaml] [--output regress_result.json] [--junit junit.xml] [--json] [--explain]")
	fmt.Println("  gait run record --input <run_record.json> [--json] [--explain]")
	fmt.Println("  gait run replay <run_id|path> [--json] [--real-tools --unsafe-real-tools --allow-tools <csv> --unsafe-real-tools-env <VAR>] [--explain]")
	fmt.Println("  gait run diff <left> <right> [--json] [--explain]")
	fmt.Println("  gait run reduce --from <run_id|path> [--predicate missing_result|non_ok_status] [--json] [--explain]")
	fmt.Println("  gait scout snapshot [--roots <csv>] [--policy <csv>] [--json] [--explain]")
	fmt.Println("  gait scout diff <left_snapshot.json> <right_snapshot.json> [--json] [--explain]")
	fmt.Println("  gait guard pack --run <run_id|path> [--json] [--explain]")
	fmt.Println("  gait guard verify <evidence_pack.zip> [--json] [--explain]")
	fmt.Println("  gait guard retain [--root <dir>] [--trace-ttl <duration>] [--pack-ttl <duration>] [--dry-run] [--json] [--explain]")
	fmt.Println("  gait guard encrypt --in <artifact> [--out <artifact.gaitenc>] [--json] [--explain]")
	fmt.Println("  gait guard decrypt --in <artifact.gaitenc> [--out <artifact>] [--json] [--explain]")
	fmt.Println("  gait incident pack --from <run_id|path> [--window <duration>] [--json] [--explain]")
	fmt.Println("  gait registry install --source <path|url> --allow-host <csv> [--json] [--explain]")
	fmt.Println("  gait migrate <artifact_path|run_id> [--out <path>] [--json] [--explain]")
	fmt.Println("  gait mcp proxy --policy <policy.yaml> --call <tool_call.json|-> [--adapter mcp|openai|anthropic|langchain] [--json] [--explain]")
	fmt.Println("  gait verify <run_id|path> [--json] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait version")
}

func printVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait verify <run_id|path> [--json] [--require-signature] [--public-key <path>] [--public-key-env <VAR>] [--explain]")
	fmt.Println("  gait verify <run_id|path> [--json] [--require-signature] [--private-key <path>] [--private-key-env <VAR>] [--explain]")
}
