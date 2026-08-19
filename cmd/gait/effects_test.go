package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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
	var err error
	snapshot := effects.Snapshot{SchemaID: effects.SnapshotSchemaID, SchemaVersion: effects.SchemaVersion, SnapshotID: "snapshot:cli", ResourceKind: effects.ResourceHTTP, Selector: effects.Selector{Resource: "http.endpoint", URL: "https://example.test/resource"}, Before: effects.Observation{State: effects.ObservationAbsent, ObservedAt: "2026-08-19T00:00:00Z"}, After: effects.Observation{State: effects.ObservationPresent, Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ObservedAt: "2026-08-19T00:00:01Z"}, Collector: effects.Collector{Name: "fixture", Version: "1", Mode: "deterministic"}, Capture: effects.Capture{Mode: "reference", CapturedAt: "2026-08-19T00:00:02Z"}, Redaction: effects.Redaction{Mode: "reference_only"}, Correlation: effects.Correlation{ActionDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ProofRefs: []string{"proof:cli"}}, EvidenceRefs: []string{"ref:cli"}, Completeness: effects.CompletenessComplete, Enforcement: effects.EnforcementVerified}
	snapshot.Correlation.ProofRefs = []string{"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	seed := sha256.Sum256([]byte("gait-effects-cli-test-key-v1"))
	snapshot, err = snapshot.Sign(ed25519.NewKeyFromSeed(seed[:]), "collector_signed")
	if err != nil {
		t.Fatal(err)
	}
	contract := effects.Contract{SchemaID: effects.ContractSchemaID, SchemaVersion: effects.SchemaVersion, ContractID: "contract:cli", Name: "created", Predicates: []effects.Predicate{{ID: "created", Kind: effects.PredicateExpect, Field: "after.state", Operator: "equals", Expected: effects.ObservationPresent}}}
	snapshotPath := filepath.Join(dir, "snapshot.json")
	contractPath := filepath.Join(dir, "contract.json")
	publicPath := filepath.Join(dir, "collector.pub")
	junitPath := filepath.Join(dir, "effects.xml")
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey))), 0o600); err != nil {
		t.Fatal(err)
	}
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
		return runEffectsGrade([]string{"--snapshot", snapshotPath, "--contract", contractPath, "--trusted-collector-key", publicPath, "--expected-action-digest", snapshot.Correlation.ActionDigest, "--junit", junitPath, "--json"})
	})
	if code != exitOK || !strings.Contains(output, `"status":"pass"`) {
		t.Fatalf("effects CLI pass: code=%d output=%s", code, output)
	}
	if raw, readErr := os.ReadFile(junitPath); readErr != nil || !strings.Contains(string(raw), "testsuite") {
		t.Fatalf("JUnit missing: %v %s", readErr, raw)
	}
	output, code = captureEffectsOutput(t, func() int {
		return runEffectsGrade([]string{"--snapshot", filepath.Join(dir, "missing.json"), "--contract", contractPath, "--trusted-collector-key", publicPath, "--expected-action-digest", snapshot.Correlation.ActionDigest, "--json"})
	})
	if code != exitInvalidInput || !strings.Contains(output, `"ok":false`) {
		t.Fatalf("effects CLI invalid input: code=%d output=%s", code, output)
	}
}

func TestEffectsHelpRequiresTrustedKey(t *testing.T) {
	output, code := captureEffectsOutput(t, func() int { return runEffects([]string{"--help"}) })
	if code != exitOK || !strings.Contains(output, "--trusted-collector-key") || !strings.Contains(output, "--expected-action-digest") || strings.Contains(output, "--allow-fixture-test-provenance") {
		t.Fatalf("effects help drift: code=%d output=%s", code, output)
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
