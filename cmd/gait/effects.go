package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/effects"
	proofsign "github.com/Clyra-AI/proof/signing"
)

type effectsOutput struct {
	OK     bool                `json:"ok"`
	Result effects.GradeResult `json:"result"`
	JUnit  string              `json:"junit,omitempty"`
	Error  string              `json:"error,omitempty"`
}

func runEffects(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Grade a deterministic effect contract against a bounded before/after effect snapshot without executing external effects.")
	}
	if len(arguments) == 0 || (len(arguments) == 1 && isTopLevelHelp(arguments[0])) {
		printEffectsUsage()
		if len(arguments) == 0 {
			return exitInvalidInput
		}
		return exitOK
	}
	if arguments[0] == "capture" {
		return runEffectsCapture(arguments[1:])
	}
	if arguments[0] == "observe" {
		return runEffectsObserve(arguments[1:])
	}
	if arguments[0] != "grade" {
		printEffectsUsage()
		return exitInvalidInput
	}
	return runEffectsGrade(arguments[1:])
}

func runEffectsObserve(args []string) int {
	f := flag.NewFlagSet("effects-observe", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var kind, path, url, ref, out, at string
	var unsafe, js bool
	f.StringVar(&kind, "resource", "", "resource kind")
	f.StringVar(&path, "path", "", "filesystem path")
	f.StringVar(&url, "url", "", "HTTP URL")
	f.StringVar(&ref, "reference", "", "generic reference")
	f.StringVar(&out, "out", "", "observation output")
	f.StringVar(&at, "observed-at", "", "fixed timestamp")
	f.BoolVar(&unsafe, "allow-unsafe-local", false, "allow local HTTP")
	f.BoolVar(&js, "json", false, "JSON")
	if e := f.Parse(args); e != nil {
		return writeEffectsCaptureOutput(js, effectsCaptureOutput{Error: e.Error()}, exitInvalidInput)
	}
	if kind == "" || out == "" {
		return writeEffectsCaptureOutput(js, effectsCaptureOutput{Error: "--resource and --out required"}, exitInvalidInput)
	}
	var observed time.Time
	var e error
	if at != "" {
		observed, e = time.Parse(time.RFC3339Nano, at)
		if e != nil {
			return writeEffectsCaptureOutput(js, effectsCaptureOutput{Error: "invalid --observed-at"}, exitInvalidInput)
		}
	}
	r, e := effects.CaptureLocal(effects.CaptureRequest{ResourceKind: kind, Path: path, URL: url, Reference: ref, AllowUnsafeLocal: unsafe, Now: observed})
	if e == nil {
		e = effects.WriteCaptureResult(out, r)
	}
	if e != nil {
		return writeEffectsCaptureOutput(js, effectsCaptureOutput{Error: e.Error()}, exitInvalidInput)
	}
	return writeEffectsCaptureOutput(js, effectsCaptureOutput{OK: true, Result: r}, exitOK)
}

type effectsCaptureOutput struct {
	OK       bool                  `json:"ok"`
	Result   effects.CaptureResult `json:"result"`
	Error    string                `json:"error,omitempty"`
	Snapshot *effects.Snapshot     `json:"snapshot,omitempty"`
}

func runEffectsCapture(arguments []string) int {
	flags := flag.NewFlagSet("effects-capture", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var kind, path, url, reference, out, privateKeyPath, actionDigest, activationDigest, proofDigest, beforePath, afterPath string
	var jsonOutput, help bool
	flags.StringVar(&kind, "resource", "", "resource kind: filesystem, http, or resource")
	flags.StringVar(&path, "path", "", "filesystem path")
	flags.StringVar(&url, "url", "", "HTTP URL (bounded, no redirects)")
	flags.StringVar(&reference, "reference", "", "generic reference")
	flags.StringVar(&out, "out", "", "optional JSON output path")
	flags.StringVar(&beforePath, "before-observation", "", "before observation JSON")
	flags.StringVar(&afterPath, "after-observation", "", "after observation JSON")
	flags.StringVar(&privateKeyPath, "private-key", "", "Ed25519 collector private key (base64 file)")
	flags.StringVar(&actionDigest, "action-digest", "", "required action correlation digest")
	flags.StringVar(&activationDigest, "activation-digest", "", "required activation correlation digest")
	flags.StringVar(&proofDigest, "proof-digest", "", "required Proof correlation digest")
	flags.BoolVar(&jsonOutput, "json", false, "emit JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if help {
		fmt.Println("Usage: gait effects capture --resource filesystem|http|resource [--path path|--url url|--reference ref] [--out result.json] [--json]")
		return exitOK
	}
	if len(flags.Args()) > 0 || strings.TrimSpace(kind) == "" {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: "--resource is required; positional arguments are not accepted"}, exitInvalidInput)
	}
	if strings.TrimSpace(privateKeyPath) == "" || strings.TrimSpace(out) == "" || beforePath == "" || afterPath == "" || (actionDigest == "" && activationDigest == "" && proofDigest == "") {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: "--private-key, --out, and a correlation digest are required"}, exitInvalidInput)
	}
	privateKey, err := proofsign.LoadPrivateKeyBase64(privateKeyPath)
	if err != nil {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	before, err := effects.LoadCaptureResult(beforePath)
	if err != nil {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{Error: err.Error()}, exitInvalidInput)
	}
	after, err := effects.LoadCaptureResult(afterPath)
	if err != nil {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{Error: err.Error()}, exitInvalidInput)
	}
	snapshot, err := effects.BuildSnapshotFromObservations(before, after, kind, effects.Selector{Resource: kind, Path: path, URL: url, Name: reference}, effects.Correlation{ActionDigest: actionDigest, ActivationDigest: activationDigest, ProofDigest: proofDigest}, privateKey)
	if err != nil {
		return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	output := effectsCaptureOutput{OK: true, Snapshot: &snapshot}
	{
		if err := effects.WriteSnapshot(out, snapshot); err != nil {
			return writeEffectsCaptureOutput(jsonOutput, effectsCaptureOutput{OK: false, Error: err.Error()}, exitInternalFailure)
		}
	}
	return writeEffectsCaptureOutput(jsonOutput, output, exitOK)
}

func writeEffectsCaptureOutput(jsonOutput bool, output effectsCaptureOutput, code int) int {
	if jsonOutput {
		raw, err := marshalJSON(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode capture output"}`)
			return exitInternalFailure
		}
		fmt.Println(string(raw))
	} else if output.Error != "" {
		fmt.Printf("effects capture error: %s\n", output.Error)
	} else if output.Snapshot != nil {
		fmt.Printf("effects capture: snapshot=%s completeness=%s\n", output.Snapshot.SnapshotID, output.Snapshot.Completeness)
	} else {
		fmt.Printf("effects capture: state=%s\n", output.Result.Observation.State)
	}
	return code
}

func runEffectsGrade(arguments []string) int {
	flags := flag.NewFlagSet("effects-grade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var snapshotPath, contractPath, junitPath, trustedCollectorKeyPath string
	var expectedActionDigest, expectedActivationDigest, expectedProofDigest string
	var jsonOutput, help bool
	flags.StringVar(&snapshotPath, "snapshot", "", "effect snapshot evidence JSON")
	flags.StringVar(&contractPath, "contract", "", "effect contract JSON")
	flags.StringVar(&junitPath, "junit", "", "optional deterministic JUnit output path")
	flags.StringVar(&trustedCollectorKeyPath, "trusted-collector-key", "", "trusted Ed25519 collector public key path (required for pass)")
	flags.StringVar(&expectedActionDigest, "expected-action-digest", "", "caller-expected action digest")
	flags.StringVar(&expectedActivationDigest, "expected-activation-digest", "", "caller-expected activation digest")
	flags.StringVar(&expectedProofDigest, "expected-proof-digest", "", "caller-expected Proof digest")
	flags.BoolVar(&jsonOutput, "json", false, "emit deterministic JSON output")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(arguments); err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	if help {
		printEffectsGradeUsage()
		return exitOK
	}
	if strings.TrimSpace(snapshotPath) == "" || strings.TrimSpace(contractPath) == "" || strings.TrimSpace(trustedCollectorKeyPath) == "" || (strings.TrimSpace(expectedActionDigest) == "" && strings.TrimSpace(expectedActivationDigest) == "" && strings.TrimSpace(expectedProofDigest) == "") || len(flags.Args()) > 0 {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: "--snapshot, --contract, --trusted-collector-key, and at least one expected correlation digest are required; positional arguments are not accepted"}, exitInvalidInput)
	}
	snapshot, err := effects.LoadSnapshot(snapshotPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	contract, err := effects.LoadContract(contractPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	trustedKey, err := effects.LoadPublicKey(trustedCollectorKeyPath)
	if err != nil {
		return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Error: err.Error()}, exitInvalidInput)
	}
	result := effects.GradeWithOptions(snapshot, contract, effects.GradeOptions{
		TrustedCollectorPublicKey: trustedKey,
		ExpectedCorrelation: &effects.CorrelationExpectation{
			ActionDigest: expectedActionDigest, ActivationDigest: expectedActivationDigest, ProofDigest: expectedProofDigest,
		},
	})
	if strings.TrimSpace(junitPath) != "" {
		if err := effects.WriteJUnit(junitPath, result); err != nil {
			return writeEffectsOutput(jsonOutput, effectsOutput{OK: false, Result: result, Error: err.Error()}, exitInternalFailure)
		}
	}
	code := exitOK
	if result.Status != effects.GradePass {
		code = exitVerifyFailed
	}
	return writeEffectsOutput(jsonOutput, effectsOutput{OK: code == exitOK, Result: result, JUnit: junitPath}, code)
}

func writeEffectsOutput(jsonOutput bool, output effectsOutput, code int) int {
	if jsonOutput {
		encoded, err := marshalJSON(output)
		if err != nil {
			fmt.Println(`{"ok":false,"error":"failed to encode effects output"}`)
			return exitInternalFailure
		}
		fmt.Println(string(encoded))
		return code
	}
	if output.Error != "" {
		fmt.Printf("effects grade error: %s\n", output.Error)
	} else {
		fmt.Printf("effects grade: status=%s\n", output.Result.Status)
	}
	return code
}

func printEffectsUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait effects observe --resource filesystem|http|resource [--path path|--url url|--reference ref] --out observation.json [--observed-at RFC3339] [--allow-unsafe-local] [--json]")
	fmt.Println("  gait effects capture --resource filesystem|http|resource [--path path|--url url|--reference ref] [--out result.json] [--json]")
	fmt.Println("  gait effects grade --snapshot <effect_snapshot.json> --contract <effect_contract.json> --trusted-collector-key <public-key> [--expected-action-digest sha256:<hex>] [--expected-activation-digest sha256:<hex>] [--expected-proof-digest sha256:<hex>] [--junit <junit.xml>] [--json] [--explain]")
}

func printEffectsGradeUsage() {
	fmt.Println("Usage: gait effects grade --snapshot <effect_snapshot.json> --contract <effect_contract.json> --trusted-collector-key <public-key> [--expected-action-digest sha256:<hex>] [--expected-activation-digest sha256:<hex>] [--expected-proof-digest sha256:<hex>] [--junit <junit.xml>] [--json] [--explain]")
}
