package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func TestPrimitiveFixturesParseInGoConsumers(t *testing.T) {
	root := repoRoot(t)

	intentRaw := mustReadFixture(t, root, "gate_intent_request_valid.json")
	var intent schemagate.IntentRequest
	if err := json.Unmarshal(intentRaw, &intent); err != nil {
		t.Fatalf("unmarshal intent fixture: %v", err)
	}
	normalizedIntent, err := gate.NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize intent fixture: %v", err)
	}
	if normalizedIntent.IntentDigest == "" {
		t.Fatalf("expected intent digest to be computed")
	}

	gateResultRaw := mustReadFixture(t, root, "gate_result_valid.json")
	var gateResult schemagate.GateResult
	if err := json.Unmarshal(gateResultRaw, &gateResult); err != nil {
		t.Fatalf("unmarshal gate result fixture: %v", err)
	}
	if gateResult.Verdict == "" {
		t.Fatalf("expected gate verdict")
	}

	traceRaw := mustReadFixture(t, root, "gate_trace_record_valid.json")
	var traceRecord schemagate.TraceRecord
	if err := json.Unmarshal(traceRaw, &traceRecord); err != nil {
		t.Fatalf("unmarshal trace fixture: %v", err)
	}
	tracePath := filepath.Join(t.TempDir(), "trace_fixture.json")
	if err := gate.WriteTraceRecord(tracePath, traceRecord); err != nil {
		t.Fatalf("write trace fixture: %v", err)
	}
	reloadedTrace, err := gate.ReadTraceRecord(tracePath)
	if err != nil {
		t.Fatalf("read trace fixture: %v", err)
	}
	if reloadedTrace.TraceID == "" {
		t.Fatalf("expected trace_id")
	}

	manifestRaw := mustReadFixture(t, root, "manifest_valid.json")
	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest fixture: %v", err)
	}
	if manifest.ManifestDigest == "" {
		t.Fatalf("expected manifest digest")
	}
}

func mustReadFixture(t *testing.T, root string, filename string) []byte {
	t.Helper()
	path := filepath.Join(root, "core", "schema", "testdata", filename)
	// #nosec G304 -- path is static fixture under repository root.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", filename, err)
	}
	return raw
}
