package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/registry"
	"github.com/davidahmann/gait/core/sign"
)

type registryInstallOutput struct {
	OK           bool   `json:"ok"`
	Source       string `json:"source,omitempty"`
	PackName     string `json:"pack_name,omitempty"`
	PackVersion  string `json:"pack_version,omitempty"`
	Digest       string `json:"digest,omitempty"`
	MetadataPath string `json:"metadata_path,omitempty"`
	PinPath      string `json:"pin_path,omitempty"`
	FallbackUsed bool   `json:"fallback_used,omitempty"`
	FallbackPath string `json:"fallback_path,omitempty"`
	Error        string `json:"error,omitempty"`
}

type registryListOutput struct {
	OK    bool                     `json:"ok"`
	Packs []registry.InstalledPack `json:"packs,omitempty"`
	Error string                   `json:"error,omitempty"`
}

type registryVerifyOutput struct {
	OK                bool   `json:"ok"`
	PackName          string `json:"pack_name,omitempty"`
	PackVersion       string `json:"pack_version,omitempty"`
	Digest            string `json:"digest,omitempty"`
	MetadataPath      string `json:"metadata_path,omitempty"`
	PinPath           string `json:"pin_path,omitempty"`
	PinDigest         string `json:"pin_digest,omitempty"`
	PinPresent        bool   `json:"pin_present,omitempty"`
	PinVerified       bool   `json:"pin_verified,omitempty"`
	SignatureVerified bool   `json:"signature_verified,omitempty"`
	SignatureError    string `json:"signature_error,omitempty"`
	Error             string `json:"error,omitempty"`
}

func runRegistry(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Install, list, and verify signed registry policy packs with allowlisted hosts, cache pinning, and offline checks.")
	}
	if len(arguments) == 0 {
		printRegistryUsage()
		return exitInvalidInput
	}
	switch arguments[0] {
	case "install":
		return runRegistryInstall(arguments[1:])
	case "list":
		return runRegistryList(arguments[1:])
	case "verify":
		return runRegistryVerify(arguments[1:])
	default:
		printRegistryUsage()
		return exitInvalidInput
	}
}

func runRegistryInstall(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Fetch a registry pack manifest, verify signatures and optional pin digest, then cache and pin locally.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"source":                true,
		"cache-dir":             true,
		"allow-host":            true,
		"pin":                   true,
		"public-key":            true,
		"public-key-env":        true,
		"private-key":           true,
		"private-key-env":       true,
		"retry-max-attempts":    true,
		"retry-base-delay":      true,
		"allow-insecure-http":   true,
		"allow-cached-fallback": true,
	})
	flagSet := flag.NewFlagSet("registry-install", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var source string
	var cacheDir string
	var allowHostsCSV string
	var pinDigest string
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var retryMaxAttempts int
	var retryBaseDelay time.Duration
	var allowInsecureHTTP bool
	var allowCachedFallback bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&source, "source", "", "registry pack source (path or URL)")
	flagSet.StringVar(&cacheDir, "cache-dir", "", "registry cache directory (default ~/.gait/registry)")
	flagSet.StringVar(&allowHostsCSV, "allow-host", "", "comma-separated remote host allowlist")
	flagSet.StringVar(&pinDigest, "pin", "", "expected digest (sha256:<hex>)")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.IntVar(&retryMaxAttempts, "retry-max-attempts", 3, "max remote fetch retry attempts for transient failures")
	flagSet.DurationVar(&retryBaseDelay, "retry-base-delay", 200*time.Millisecond, "base remote fetch retry delay (for example 200ms, 1s)")
	flagSet.BoolVar(&allowInsecureHTTP, "allow-insecure-http", false, "allow insecure http sources (unsafe)")
	flagSet.BoolVar(&allowCachedFallback, "allow-cached-fallback", false, "allow fallback to cached pinned manifest on remote fetch failure")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegistryInstallOutput(jsonOutput, registryInstallOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRegistryInstallUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if source == "" && len(remaining) > 0 {
		source = remaining[0]
		remaining = remaining[1:]
	}
	if source == "" || len(remaining) > 0 {
		return writeRegistryInstallOutput(jsonOutput, registryInstallOutput{OK: false, Error: "expected --source <path|url>"}, exitInvalidInput)
	}

	publicKey, err := sign.LoadVerifyKey(sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeRegistryInstallOutput(jsonOutput, registryInstallOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := registry.Install(context.Background(), registry.InstallOptions{
		Source:              source,
		CacheDir:            cacheDir,
		PublicKey:           publicKey,
		AllowHosts:          parseCSVList(allowHostsCSV),
		PinDigest:           pinDigest,
		RetryMaxAttempts:    retryMaxAttempts,
		RetryBaseDelay:      retryBaseDelay,
		AllowInsecureHTTP:   allowInsecureHTTP,
		AllowCachedFallback: allowCachedFallback,
	})
	if err != nil {
		return writeRegistryInstallOutput(jsonOutput, registryInstallOutput{OK: false, Error: err.Error()}, exitVerifyFailed)
	}
	return writeRegistryInstallOutput(jsonOutput, registryInstallOutput{
		OK:           true,
		Source:       result.Source,
		PackName:     result.PackName,
		PackVersion:  result.PackVersion,
		Digest:       result.Digest,
		MetadataPath: result.MetadataPath,
		PinPath:      result.PinPath,
		FallbackUsed: result.FallbackUsed,
		FallbackPath: result.FallbackPath,
	}, exitOK)
}

func runRegistryList(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("List cached and pinned registry policy packs from the local registry cache.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"cache-dir": true,
	})
	flagSet := flag.NewFlagSet("registry-list", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var cacheDir string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&cacheDir, "cache-dir", "", "registry cache directory (default ~/.gait/registry)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegistryListOutput(jsonOutput, registryListOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRegistryListUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeRegistryListOutput(jsonOutput, registryListOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}

	packs, err := registry.List(registry.ListOptions{CacheDir: cacheDir})
	if err != nil {
		return writeRegistryListOutput(jsonOutput, registryListOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	return writeRegistryListOutput(jsonOutput, registryListOutput{
		OK:    true,
		Packs: packs,
	}, exitOK)
}

func runRegistryVerify(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Verify a cached registry manifest signature and optional local pin digest deterministically.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"path":            true,
		"cache-dir":       true,
		"public-key":      true,
		"public-key-env":  true,
		"private-key":     true,
		"private-key-env": true,
	})
	flagSet := flag.NewFlagSet("registry-verify", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var metadataPath string
	var cacheDir string
	var publicKeyPath string
	var publicKeyEnv string
	var privateKeyPath string
	var privateKeyEnv string
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&metadataPath, "path", "", "path to registry_pack.json")
	flagSet.StringVar(&cacheDir, "cache-dir", "", "registry cache directory for pin verification")
	flagSet.StringVar(&publicKeyPath, "public-key", "", "path to base64 public key")
	flagSet.StringVar(&publicKeyEnv, "public-key-env", "", "env var containing base64 public key")
	flagSet.StringVar(&privateKeyPath, "private-key", "", "path to base64 private key (derive public)")
	flagSet.StringVar(&privateKeyEnv, "private-key-env", "", "env var containing base64 private key (derive public)")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeRegistryVerifyOutput(jsonOutput, registryVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printRegistryVerifyUsage()
		return exitOK
	}
	remaining := flagSet.Args()
	if metadataPath == "" && len(remaining) > 0 {
		metadataPath = remaining[0]
		remaining = remaining[1:]
	}
	if metadataPath == "" || len(remaining) > 0 {
		return writeRegistryVerifyOutput(jsonOutput, registryVerifyOutput{OK: false, Error: "expected --path <registry_pack.json>"}, exitInvalidInput)
	}

	publicKey, err := sign.LoadVerifyKey(sign.KeyConfig{
		PublicKeyPath:  publicKeyPath,
		PublicKeyEnv:   publicKeyEnv,
		PrivateKeyPath: privateKeyPath,
		PrivateKeyEnv:  privateKeyEnv,
	})
	if err != nil {
		return writeRegistryVerifyOutput(jsonOutput, registryVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	result, err := registry.Verify(registry.VerifyOptions{
		MetadataPath: metadataPath,
		CacheDir:     cacheDir,
		PublicKey:    publicKey,
	})
	if err != nil {
		return writeRegistryVerifyOutput(jsonOutput, registryVerifyOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}

	ok := result.SignatureVerified && (!result.PinPresent || result.PinVerified)
	exitCode := exitOK
	if !ok {
		exitCode = exitVerifyFailed
	}
	return writeRegistryVerifyOutput(jsonOutput, registryVerifyOutput{
		OK:                ok,
		PackName:          result.PackName,
		PackVersion:       result.PackVersion,
		Digest:            result.Digest,
		MetadataPath:      result.MetadataPath,
		PinPath:           result.PinPath,
		PinDigest:         result.PinDigest,
		PinPresent:        result.PinPresent,
		PinVerified:       result.PinVerified,
		SignatureVerified: result.SignatureVerified,
		SignatureError:    result.SignatureError,
	}, exitCode)
}

func writeRegistryInstallOutput(jsonOutput bool, output registryInstallOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("registry install ok: %s@%s\n", output.PackName, output.PackVersion)
		fmt.Printf("digest: sha256:%s\n", output.Digest)
		if output.FallbackUsed {
			fmt.Printf("fallback: cached manifest %s\n", output.FallbackPath)
		}
		return exitCode
	}
	fmt.Printf("registry install error: %s\n", output.Error)
	return exitCode
}

func writeRegistryListOutput(jsonOutput bool, output registryListOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("registry list ok: %d packs\n", len(output.Packs))
		for _, pack := range output.Packs {
			fmt.Printf("- %s@%s sha256:%s\n", pack.PackName, pack.PackVersion, pack.Digest)
		}
		return exitCode
	}
	fmt.Printf("registry list error: %s\n", output.Error)
	return exitCode
}

func writeRegistryVerifyOutput(jsonOutput bool, output registryVerifyOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		fmt.Printf("registry verify ok: %s@%s sha256:%s\n", output.PackName, output.PackVersion, output.Digest)
		return exitCode
	}
	if strings.TrimSpace(output.Error) != "" {
		fmt.Printf("registry verify error: %s\n", output.Error)
		return exitCode
	}
	if strings.TrimSpace(output.SignatureError) != "" {
		fmt.Printf("registry verify failed: %s\n", output.SignatureError)
		return exitCode
	}
	fmt.Printf("registry verify failed: pin mismatch for %s\n", output.PackName)
	return exitCode
}

func printRegistryUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait registry install --source <path|url> --allow-host <csv> [--pin sha256:<hex>] [--cache-dir <path>] [--public-key <path>|--public-key-env <VAR>] [--retry-max-attempts <n>] [--retry-base-delay <duration>] [--allow-cached-fallback] [--allow-insecure-http] [--json] [--explain]")
	fmt.Println("  gait registry list [--cache-dir <path>] [--json] [--explain]")
	fmt.Println("  gait registry verify --path <registry_pack.json> [--cache-dir <path>] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
}

func printRegistryInstallUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait registry install --source <path|url> --allow-host <csv> [--pin sha256:<hex>] [--cache-dir <path>] [--public-key <path>|--public-key-env <VAR>] [--retry-max-attempts <n>] [--retry-base-delay <duration>] [--allow-cached-fallback] [--allow-insecure-http] [--json] [--explain]")
}

func printRegistryListUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait registry list [--cache-dir <path>] [--json] [--explain]")
}

func printRegistryVerifyUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait registry verify --path <registry_pack.json> [--cache-dir <path>] [--public-key <path>|--public-key-env <VAR>] [--json] [--explain]")
}
