package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/sign"
)

type keysInitOutput struct {
	OK             bool   `json:"ok"`
	Prefix         string `json:"prefix,omitempty"`
	KeyID          string `json:"key_id,omitempty"`
	PublicKeyPath  string `json:"public_key_path,omitempty"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	Error          string `json:"error,omitempty"`
}

type keysVerifyOutput struct {
	OK               bool   `json:"ok"`
	KeyID            string `json:"key_id,omitempty"`
	PublicKeySource  string `json:"public_key_source,omitempty"`
	PrivateKeySource string `json:"private_key_source,omitempty"`
	Error            string `json:"error,omitempty"`
}

func runKeys(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Manage local ed25519 signing keys for trace and artifact verification workflows.")
	}
	if len(arguments) == 0 {
		printKeysUsage()
		return exitInvalidInput
	}
	if arguments[0] == "--help" || arguments[0] == "-h" {
		printKeysUsage()
		return exitOK
	}
	switch arguments[0] {
	case "init":
		return runKeysInit(arguments[1:])
	case "rotate":
		return runKeysRotate(arguments[1:])
	case "verify":
		return runKeysVerify(arguments[1:])
	default:
		printKeysUsage()
		return exitInvalidInput
	}
}

func runKeysInit(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Generate a new ed25519 keypair and write base64-encoded key files to disk.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"out-dir": true,
		"prefix":  true,
	})

	flagSet := flag.NewFlagSet("keys-init", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var outDir string
	var prefix string
	var force bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&outDir, "out-dir", filepath.Join("gait-out", "keys"), "directory for generated key files")
	flagSet.StringVar(&prefix, "prefix", "gait", "key file prefix")
	flagSet.BoolVar(&force, "force", false, "overwrite existing key files")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printKeysInitUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	result, err := createSigningKeypair(outDir, prefix, force)
	if err != nil {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeKeysInitOutput(jsonOutput, result, exitOK)
}

func runKeysRotate(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Rotate signing keys by generating a timestamped keypair while keeping previous keys intact.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"out-dir": true,
		"prefix":  true,
	})

	flagSet := flag.NewFlagSet("keys-rotate", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var outDir string
	var prefix string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&outDir, "out-dir", filepath.Join("gait-out", "keys"), "directory for rotated key files")
	flagSet.StringVar(&prefix, "prefix", "gait", "key file prefix")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printKeysRotateUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	rotatedPrefix := fmt.Sprintf("%s_%s", strings.TrimSpace(prefix), time.Now().UTC().Format("20060102T150405Z"))
	result, err := createSigningKeypair(outDir, rotatedPrefix, false)
	if err != nil {
		return writeKeysInitOutput(jsonOutput, keysInitOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeKeysInitOutput(jsonOutput, result, exitOK)
}

func runKeysVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Validate that configured signing key sources decode correctly and that public/private keys match.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"private-key":     true,
		"private-key-env": true,
		"public-key":      true,
		"public-key-env":  true,
	})

	flagSet := flag.NewFlagSet("keys-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var privateKeyPath string
	var privateKeyEnv string
	var publicKeyPath string
	var publicKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeKeysVerifyOutput(jsonOutput, keysVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printKeysVerifyUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeKeysVerifyOutput(jsonOutput, keysVerifyOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	if strings.TrimSpace(privateKeyPath) == "" && strings.TrimSpace(privateKeyEnv) == "" {
		return writeKeysVerifyOutput(jsonOutput, keysVerifyOutput{OK: false, Error: "private key source is required (--private-key or --private-key-env)"}, exitInvalidInput)
	}

	kp, _, err := sign.LoadSigningKey(sign.KeyConfig{
		Mode:           sign.ModeProd,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
	})
	if err != nil {
		return writeKeysVerifyOutput(jsonOutput, keysVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	return writeKeysVerifyOutput(jsonOutput, keysVerifyOutput{
		OK:               true,
		KeyID:            sign.KeyID(kp.Public),
		PrivateKeySource: keySourceLabel(privateKeyPath, privateKeyEnv),
		PublicKeySource:  keySourceLabel(publicKeyPath, publicKeyEnv),
	}, exitOK)
}

func createSigningKeypair(outDir string, prefix string, force bool) (keysInitOutput, error) {
	trimmedOutDir := strings.TrimSpace(outDir)
	if trimmedOutDir == "" {
		return keysInitOutput{}, fmt.Errorf("out-dir must not be empty")
	}
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return keysInitOutput{}, fmt.Errorf("prefix must not be empty")
	}

	if err := os.MkdirAll(trimmedOutDir, 0o750); err != nil {
		return keysInitOutput{}, fmt.Errorf("create keys directory: %w", err)
	}

	privatePath := filepath.Join(trimmedOutDir, trimmedPrefix+"_private.key")
	publicPath := filepath.Join(trimmedOutDir, trimmedPrefix+"_public.key")
	if !force {
		if _, err := os.Stat(privatePath); err == nil {
			return keysInitOutput{}, fmt.Errorf("private key path already exists (use --force): %s", privatePath)
		}
		if _, err := os.Stat(publicPath); err == nil {
			return keysInitOutput{}, fmt.Errorf("public key path already exists (use --force): %s", publicPath)
		}
	}

	kp, err := sign.GenerateKeyPair()
	if err != nil {
		return keysInitOutput{}, fmt.Errorf("generate keypair: %w", err)
	}
	privateEncoded := base64.StdEncoding.EncodeToString(kp.Private)
	publicEncoded := base64.StdEncoding.EncodeToString(kp.Public)

	if err := os.WriteFile(privatePath, []byte(privateEncoded+"\n"), 0o600); err != nil {
		return keysInitOutput{}, fmt.Errorf("write private key: %w", err)
	}
	if err := os.WriteFile(publicPath, []byte(publicEncoded+"\n"), 0o600); err != nil {
		return keysInitOutput{}, fmt.Errorf("write public key: %w", err)
	}

	return keysInitOutput{
		OK:             true,
		Prefix:         trimmedPrefix,
		KeyID:          sign.KeyID(kp.Public),
		PublicKeyPath:  publicPath,
		PrivateKeyPath: privatePath,
	}, nil
}

func keySourceLabel(path string, env string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath != "" {
		return "path:" + trimmedPath
	}
	trimmedEnv := strings.TrimSpace(env)
	if trimmedEnv != "" {
		return "env:" + trimmedEnv
	}
	return "derived"
}

func writeKeysInitOutput(jsonOutput bool, output keysInitOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("keys init ok: key_id=%s public=%s private=%s\n", output.KeyID, output.PublicKeyPath, output.PrivateKeyPath)
		return exitCode
	}
	fmt.Printf("keys init error: %s\n", output.Error)
	return exitCode
}

func writeKeysVerifyOutput(jsonOutput bool, output keysVerifyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("keys verify ok: key_id=%s\n", output.KeyID)
		return exitCode
	}
	fmt.Printf("keys verify error: %s\n", output.Error)
	return exitCode
}

func printKeysUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait keys init [--out-dir gait-out/keys] [--prefix gait] [--force] [--json] [--explain]")
	fmt.Println("  gait keys rotate [--out-dir gait-out/keys] [--prefix gait] [--json] [--explain]")
	fmt.Println("  gait keys verify [--private-key <path>|--private-key-env <VAR>] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
}

func printKeysInitUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait keys init [--out-dir gait-out/keys] [--prefix gait] [--force] [--json] [--explain]")
}

func printKeysRotateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait keys rotate [--out-dir gait-out/keys] [--prefix gait] [--json] [--explain]")
}

func printKeysVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait keys verify [--private-key <path>|--private-key-env <VAR>] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
}
