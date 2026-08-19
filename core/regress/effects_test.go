package regress

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Clyra-AI/gait/core/effects"
)

func TestEffectContractGraderMapsPureStatusesToFailClosedRegressResults(t *testing.T) {
	dir := t.TempDir()
	var err error
	snapshot := effects.Snapshot{SchemaID: effects.SnapshotSchemaID, SchemaVersion: effects.SchemaVersion, SnapshotID: "snapshot:regress", ResourceKind: effects.ResourceFilesystem, Selector: effects.Selector{Resource: "filesystem.path", Path: "/tmp/example"}, Before: effects.Observation{State: effects.ObservationAbsent, ObservedAt: "2026-08-19T00:00:00Z"}, After: effects.Observation{State: effects.ObservationPresent, Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ObservedAt: "2026-08-19T00:00:01Z"}, Collector: effects.Collector{Name: "fixture", Version: "1", Mode: "deterministic"}, Capture: effects.Capture{Mode: "reference", CapturedAt: "2026-08-19T00:00:02Z"}, Redaction: effects.Redaction{Mode: "reference_only"}, Correlation: effects.Correlation{ActionDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ProofRefs: []string{"proof:regress"}}, EvidenceRefs: []string{"ref:regress"}, Completeness: effects.CompletenessComplete, Enforcement: effects.EnforcementVerified}
	snapshot.Correlation.ProofRefs = []string{"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	seed := sha256.Sum256([]byte("gait-effects-regress-test-key-v1"))
	snapshot, err = snapshot.Sign(ed25519.NewKeyFromSeed(seed[:]), "collector_signed")
	if err != nil {
		t.Fatal(err)
	}
	contract := effects.Contract{SchemaID: effects.ContractSchemaID, SchemaVersion: effects.SchemaVersion, ContractID: "contract:regress", Name: "filesystem created", Predicates: []effects.Predicate{{ID: "created", Kind: effects.PredicateExpect, Field: "after.state", Operator: "equals", Expected: effects.ObservationPresent}}}
	snapshotPath := filepath.Join(dir, "snapshot.json")
	contractPath := filepath.Join(dir, "contract.json")
	publicPath := filepath.Join(dir, "collector.pub")
	if writeErr := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(seedKey(seed).Public().(ed25519.PublicKey))), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	write := func(path string, value any) {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, raw, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	write(snapshotPath, snapshot)
	write(contractPath, contract)
	grader := effectContractGrader{}
	result, err := grader.Grade(FixtureContext{Fixture: fixtureSpec{EffectSnapshotPath: snapshotPath, EffectContractPath: contractPath, EffectPublicKeyPath: publicPath}})
	if err != nil || result.Status != regressStatusPass {
		t.Fatalf("expected effect grader pass: result=%+v err=%v", result, err)
	}
	snapshot.Completeness = effects.CompletenessPartial
	snapshot, err = snapshot.Sign(ed25519.NewKeyFromSeed(seed[:]), "collector_signed")
	if err != nil {
		t.Fatal(err)
	}
	write(snapshotPath, snapshot)
	result, err = grader.Grade(FixtureContext{Fixture: fixtureSpec{EffectSnapshotPath: snapshotPath, EffectContractPath: contractPath, EffectPublicKeyPath: publicPath}})
	if err != nil || result.Status != regressStatusFail || !containsReason(result.ReasonCodes, effects.ReasonPredicateInconclusive) {
		t.Fatalf("inconclusive effect must fail closed: result=%+v err=%v", result, err)
	}
}

func seedKey(seed [32]byte) ed25519.PrivateKey { return ed25519.NewKeyFromSeed(seed[:]) }

func containsReason(reasons []string, wanted string) bool {
	for _, reason := range reasons {
		if reason == wanted {
			return true
		}
	}
	return false
}

func TestResolveOptionalEffectPathsRejectEscape(t *testing.T) {
	if path, err := resolveOptionalFixturePath("effects/snapshot.json", "/fixtures/demo"); err != nil || path != "/fixtures/demo/effects/snapshot.json" {
		t.Fatalf("valid relative effect path: path=%q err=%v", path, err)
	}
	if _, err := resolveOptionalFixturePath("../../outside.json", "/fixtures/demo"); err == nil {
		t.Fatal("effect path escape accepted")
	}
	if _, err := resolveOptionalFixturePath("/absolute.json", "/fixtures/demo"); err == nil {
		t.Fatal("absolute effect path accepted")
	}
}
