package integration

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/jcs"
	"github.com/Clyra-AI/gait/core/policytest"
	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/sign"
	"github.com/Clyra-AI/gait/internal/testutil"
)

func TestRecordVerifyReplayDiffFlow(t *testing.T) {
	workDir := t.TempDir()
	runpackPath := filepath.Join(workDir, "runpack_run_integration.zip")

	run, intents, results, refs := integrationRunpackFixture(t)
	recordResult, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}

	verifyResult, err := runpack.VerifyZip(runpackPath, runpack.VerifyOptions{
		RequireSignature: false,
	})
	if err != nil {
		t.Fatalf("verify runpack: %v", err)
	}
	if len(verifyResult.MissingFiles) > 0 || len(verifyResult.HashMismatches) > 0 {
		t.Fatalf("verify reported issues: missing=%v mismatches=%v", verifyResult.MissingFiles, verifyResult.HashMismatches)
	}

	replayResult, err := runpack.ReplayStub(runpackPath)
	if err != nil {
		t.Fatalf("replay stub: %v", err)
	}
	if len(replayResult.MissingResults) > 0 {
		t.Fatalf("replay missing results: %v", replayResult.MissingResults)
	}

	diffResult, err := runpack.DiffRunpacks(runpackPath, runpackPath, runpack.DiffPrivacy("full"))
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if diffResult.Summary.ManifestChanged || diffResult.Summary.IntentsChanged || diffResult.Summary.ResultsChanged || diffResult.Summary.RefsChanged {
		t.Fatalf("expected no diff changes: %#v", diffResult.Summary)
	}

	golden := map[string]any{
		"run_id":                  verifyResult.RunID,
		"manifest_digest":         recordResult.Manifest.ManifestDigest,
		"verify_signature_status": verifyResult.SignatureStatus,
		"files_checked":           verifyResult.FilesChecked,
		"replay_mode":             string(replayResult.Mode),
		"replay_steps":            len(replayResult.Steps),
		"diff_summary":            diffResult.Summary,
	}
	testutil.AssertGoldenJSON(t, "internal/integration/testdata/record_verify_replay_diff.golden.json", golden)
}

func TestGateEvalTraceVerifyFlow(t *testing.T) {
	workDir := t.TempDir()
	privateKey := mustGeneratePrivateKey(t)

	policy, err := gate.ParsePolicyYAML([]byte(`
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: allow
rules:
  - name: block-external-host
    priority: 10
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
      target_values: [api.external.com]
    reason_codes: [blocked_external]
    violations: [external_target]
`))
	if err != nil {
		t.Fatalf("parse policy yaml: %v", err)
	}

	intent := integrationIntentFixture()
	result, err := gate.EvaluatePolicy(policy, intent, gate.EvalOptions{
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}
	if result.Verdict != "block" {
		t.Fatalf("unexpected verdict: %s", result.Verdict)
	}

	tracePath := filepath.Join(workDir, "trace.json")
	traceResult, err := gate.EmitSignedTrace(policy, intent, result, gate.EmitTraceOptions{
		ProducerVersion:   "0.0.0-test",
		SigningPrivateKey: privateKey,
		TracePath:         tracePath,
		LatencyMS:         3.25,
	})
	if err != nil {
		t.Fatalf("emit trace: %v", err)
	}

	traceRecord, err := gate.ReadTraceRecord(traceResult.TracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	ok, err := gate.VerifyTraceRecordSignature(traceRecord, privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("verify trace signature: %v", err)
	}
	if !ok {
		t.Fatalf("expected trace signature verification to succeed")
	}

	golden := map[string]any{
		"verdict":       result.Verdict,
		"reason_codes":  result.ReasonCodes,
		"violations":    result.Violations,
		"trace_id":      traceResult.Trace.TraceID,
		"policy_digest": traceResult.PolicyDigest,
		"intent_digest": traceResult.IntentDigest,
	}
	testutil.AssertGoldenJSON(t, "internal/integration/testdata/gate_eval_trace_verify.golden.json", golden)
}

func TestPolicyTestExitCodeMapping(t *testing.T) {
	intent := integrationIntentFixture()
	testCases := []struct {
		name           string
		defaultVerdict string
		expectedCode   int
	}{
		{name: "allow", defaultVerdict: "allow", expectedCode: 0},
		{name: "block", defaultVerdict: "block", expectedCode: 3},
		{name: "require_approval", defaultVerdict: "require_approval", expectedCode: 4},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy, err := gate.ParsePolicyYAML([]byte("default_verdict: " + testCase.defaultVerdict + "\n"))
			if err != nil {
				t.Fatalf("parse policy: %v", err)
			}

			runResult, err := policytest.Run(policytest.RunOptions{
				Policy:          policy,
				Intent:          intent,
				ProducerVersion: "0.0.0-test",
			})
			if err != nil {
				t.Fatalf("policy test run: %v", err)
			}

			actualCode := policyVerdictExitCode(runResult.Result.Verdict)
			if actualCode != testCase.expectedCode {
				t.Fatalf("unexpected exit code for verdict %q: got=%d want=%d", runResult.Result.Verdict, actualCode, testCase.expectedCode)
			}
		})
	}
}

func integrationRunpackFixture(t *testing.T) (schemarunpack.Run, []schemarunpack.IntentRecord, []schemarunpack.ResultRecord, schemarunpack.Refs) {
	t.Helper()
	createdAt := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	runID := "run_integration"
	run := schemarunpack.Run{
		SchemaID:        "gait.runpack.run",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-test",
		RunID:           runID,
		Env: schemarunpack.RunEnv{
			OS:      "test",
			Arch:    "test",
			Runtime: "go",
		},
		Timeline: []schemarunpack.TimelineEvt{
			{Event: "start", TS: createdAt},
			{Event: "finish", TS: createdAt.Add(2 * time.Second)},
		},
	}

	intentArgs := []map[string]any{
		{"query": "offline test"},
		{"url": "https://example.local/resource"},
	}
	toolNames := []string{"tool.search", "tool.fetch"}

	intents := make([]schemarunpack.IntentRecord, len(intentArgs))
	results := make([]schemarunpack.ResultRecord, len(intentArgs))
	receipts := make([]schemarunpack.RefReceipt, len(intentArgs))

	for index := range intentArgs {
		intentID := "intent_" + string(rune('1'+index))
		argsDigest, err := digestObject(intentArgs[index])
		if err != nil {
			t.Fatalf("digest intent args: %v", err)
		}
		resultObj := map[string]any{"ok": true, "message": "result_" + string(rune('1'+index))}
		resultDigest, err := digestObject(resultObj)
		if err != nil {
			t.Fatalf("digest result object: %v", err)
		}

		intents[index] = schemarunpack.IntentRecord{
			SchemaID:        "gait.runpack.intent",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           runID,
			IntentID:        intentID,
			ToolName:        toolNames[index],
			ArgsDigest:      argsDigest,
			Args:            intentArgs[index],
		}
		results[index] = schemarunpack.ResultRecord{
			SchemaID:        "gait.runpack.result",
			SchemaVersion:   "1.0.0",
			CreatedAt:       createdAt,
			ProducerVersion: run.ProducerVersion,
			RunID:           runID,
			IntentID:        intentID,
			Status:          "ok",
			ResultDigest:    resultDigest,
			Result:          resultObj,
		}

		receipts[index] = schemarunpack.RefReceipt{
			RefID:         "ref_" + string(rune('1'+index)),
			SourceType:    "integration-test",
			SourceLocator: toolNames[index],
			QueryDigest:   digestString("query_" + string(rune('1'+index))),
			ContentDigest: digestString("content_" + string(rune('1'+index))),
			RetrievedAt:   createdAt,
			RedactionMode: "reference",
		}
	}

	refs := schemarunpack.Refs{
		SchemaID:        "gait.runpack.refs",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: run.ProducerVersion,
		RunID:           runID,
		Receipts:        receipts,
	}

	return run, intents, results, refs
}

func integrationIntentFixture() schemagate.IntentRequest {
	createdAt := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	return schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-test",
		ToolName:        "tool.write",
		Args: map[string]any{
			"path": "/tmp/out.txt",
		},
		Targets: []schemagate.IntentTarget{
			{Kind: "host", Value: "api.external.com"},
		},
		ArgProvenance: []schemagate.IntentArgProvenance{
			{ArgPath: "$.path", Source: "user"},
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}
}

func mustGeneratePrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return keyPair.Private
}

func policyVerdictExitCode(verdict string) int {
	switch verdict {
	case "block":
		return 3
	case "require_approval":
		return 4
	default:
		return 0
	}
}

func digestObject(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func digestString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
