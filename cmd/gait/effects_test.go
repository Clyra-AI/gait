package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clyra-AI/gait/core/effects"
)

func TestEffectsGradeCLIJSONAndJUnit(t *testing.T) {
	dir := t.TempDir()
	snapshot := effects.Snapshot{SchemaID: effects.SnapshotSchemaID, SchemaVersion: effects.SchemaVersion, SnapshotID: "snapshot:cli", ResourceKind: effects.ResourceHTTP, Selector: effects.Selector{Resource: "http.endpoint", URL: "https://example.test/resource"}, Before: effects.Observation{State: effects.ObservationAbsent, ObservedAt: "2026-08-19T00:00:00Z"}, After: effects.Observation{State: effects.ObservationPresent, Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ObservedAt: "2026-08-19T00:00:01Z"}, Collector: effects.Collector{Name: "fixture", Version: "1", Mode: "deterministic"}, Capture: effects.Capture{Mode: "reference", CapturedAt: "2026-08-19T00:00:02Z"}, Redaction: effects.Redaction{Mode: "reference_only"}, Completeness: effects.CompletenessComplete, Enforcement: effects.EnforcementVerified}
	digest, err := snapshot.CanonicalDigest()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.CanonicalContentDigest = digest
	contract := effects.Contract{SchemaID: effects.ContractSchemaID, SchemaVersion: effects.SchemaVersion, ContractID: "contract:cli", Name: "created", Predicates: []effects.Predicate{{ID: "created", Kind: effects.PredicateExpect, Field: "after.state", Operator: "equals", Expected: effects.ObservationPresent}}}
	snapshotPath := filepath.Join(dir, "snapshot.json")
	contractPath := filepath.Join(dir, "contract.json")
	junitPath := filepath.Join(dir, "effects.xml")
	writeJSON := func(path string, value any) {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, raw, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	writeJSON(snapshotPath, snapshot)
	writeJSON(contractPath, contract)
	output, code := captureEffectsOutput(t, func() int {
		return runEffectsGrade([]string{"--snapshot", snapshotPath, "--contract", contractPath, "--junit", junitPath, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"status":"pass"`) {
		t.Fatalf("effects CLI pass: code=%d output=%s", code, output)
	}
	if raw, readErr := os.ReadFile(junitPath); readErr != nil || !strings.Contains(string(raw), "testsuite") {
		t.Fatalf("JUnit missing: %v %s", readErr, raw)
	}
	output, code = captureEffectsOutput(t, func() int {
		return runEffectsGrade([]string{"--snapshot", filepath.Join(dir, "missing.json"), "--contract", contractPath, "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, `"ok":false`) {
		t.Fatalf("effects CLI invalid input: code=%d output=%s", code, output)
	}
}

func captureEffectsOutput(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	resultCh := make(chan struct {
		payload []byte
		err     error
	}, 1)
	go func() {
		payload, readErr := io.ReadAll(reader)
		resultCh <- struct {
			payload []byte
			err     error
		}{payload, readErr}
	}()
	code := fn()
	_ = writer.Close()
	os.Stdout = original
	result := <-resultCh
	_ = reader.Close()
	if result.err != nil {
		t.Fatal(result.err)
	}
	return string(result.payload), code
}
