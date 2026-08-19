package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Clyra-AI/gait/core/effects"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const fixtureRoot = "testdata/effects/v1"

type manifest struct {
	FixtureVersion string `json:"fixture_version"`
	Producer       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Schemas struct {
		Snapshot string `json:"snapshot"`
		Contract string `json:"contract"`
		Grade    string `json:"grade"`
	} `json:"schemas"`
	Signing struct {
		Mode             string `json:"mode"`
		Development      bool   `json:"development_signing"`
		NonAuthoritative bool   `json:"non_authoritative"`
		KeyID            string `json:"key_id"`
		PublicKeyPath    string `json:"public_key_path"`
		PublicKeySHA256  string `json:"public_key_sha256"`
		Derivation       string `json:"derivation"`
	} `json:"signing"`
	Files []manifestFile `json:"files"`
}

type manifestFile struct {
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	SchemaID      string `json:"schema_id"`
	SchemaVersion string `json:"schema_version"`
}

func main() {
	check := flag.Bool("check", false, "verify exact fixture bytes")
	update := flag.Bool("update", false, "write exact fixture bytes")
	root := flag.String("repo-root", ".", "repository root")
	flag.Parse()
	if *check == *update {
		fail(errors.New("exactly one of --check or --update is required"))
	}
	if err := run(*root, *update); err != nil {
		fail(err)
	}
}

func run(repoRoot string, update bool) error {
	root := filepath.Join(repoRoot, fixtureRoot)
	snapshot := effects.Snapshot{
		SchemaID: effects.SnapshotSchemaID, SchemaVersion: effects.SchemaVersion, SnapshotID: "effect-snapshot:filesystem-fixture", ResourceKind: effects.ResourceFilesystem,
		Selector:  effects.Selector{Resource: "filesystem.path", Path: "/srv/example/data.json"},
		Before:    effects.Observation{State: effects.ObservationPresent, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Count: int64ptr(1), Identity: "file:data.json", Owner: "owner:fixture", TTLSeconds: int64ptr(3600), ObservedAt: "2026-08-19T00:00:00Z", EvidenceRefs: []string{"ref:before"}},
		After:     effects.Observation{State: effects.ObservationPresent, Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Count: int64ptr(1), Identity: "file:data.json", Owner: "owner:fixture", TTLSeconds: int64ptr(3600), ObservedAt: "2026-08-19T00:00:01Z", EvidenceRefs: []string{"ref:after"}},
		Collector: effects.Collector{Name: "fixture-collector", Version: "1.0.0", Mode: "deterministic"}, Capture: effects.Capture{Mode: "reference", SourceRef: "capture:effects-fixture", CapturedAt: "2026-08-19T00:00:02Z"}, Redaction: effects.Redaction{Mode: "reference_only"}, Correlation: effects.Correlation{ActionDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", ProofDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", LifecycleID: "lifecycle:fixture", ProofRefs: []string{"proof:fixture"}}, EvidenceRefs: []string{"ref:before", "ref:after"}, Completeness: effects.CompletenessComplete, Enforcement: effects.EnforcementVerified,
	}
	snapshot.Correlation.ProofRefs = []string{"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}
	seed := sha256.Sum256([]byte("gait-effects-fixture-producer-key-v1"))
	snapshot, err := snapshot.Sign(ed25519.NewKeyFromSeed(seed[:]), "fixture_test_only")
	if err != nil {
		return err
	}
	contract := effects.Contract{SchemaID: effects.ContractSchemaID, SchemaVersion: effects.SchemaVersion, ContractID: "effect-contract:filesystem-fixture", Name: "filesystem lifecycle fixture", Predicates: []effects.Predicate{{ID: "owner", Kind: effects.PredicateExpect, Field: "after.owner", Operator: "equals", Expected: "owner:fixture"}, {ID: "count", Kind: effects.PredicateInvariant, Field: "count"}, {ID: "identity", Kind: effects.PredicateForbid, Field: "after.identity", Operator: "equals", Expected: "file:deleted"}}}
	trusted := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	grade := effects.GradeWithOptions(snapshot, contract, effects.GradeOptions{TrustedCollectorPublicKey: trusted, AllowFixtureTestProvenance: true})
	if grade.Status != effects.GradePass {
		return fmt.Errorf("generated fixture does not pass: %+v", grade)
	}
	values := map[string]any{"effect_snapshot.json": snapshot, "effect_contract.json": contract, "effect_grading_result.json": grade}
	files := make(map[string][]byte, len(values))
	for name, value := range values {
		raw, marshalErr := json.MarshalIndent(value, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		files[name] = append(raw, '\n')
	}
	manifestValue := manifest{FixtureVersion: "1", Files: []manifestFile{{Path: "effect_snapshot.json", SHA256: rawDigest(files["effect_snapshot.json"]), SchemaID: effects.SnapshotSchemaID, SchemaVersion: effects.SchemaVersion}, {Path: "effect_contract.json", SHA256: rawDigest(files["effect_contract.json"]), SchemaID: effects.ContractSchemaID, SchemaVersion: effects.SchemaVersion}, {Path: "effect_grading_result.json", SHA256: rawDigest(files["effect_grading_result.json"]), SchemaID: effects.GradeSchemaID, SchemaVersion: effects.SchemaVersion}}}
	manifestValue.Producer.Name, manifestValue.Producer.Version = "gait", "v1.5.0"
	manifestValue.Schemas.Snapshot, manifestValue.Schemas.Contract, manifestValue.Schemas.Grade = effects.SchemaVersion, effects.SchemaVersion, effects.SchemaVersion
	publicKey := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	publicKeyRaw := []byte(base64.StdEncoding.EncodeToString(publicKey) + "\n")
	files["fixture_public_key.base64"] = publicKeyRaw
	manifestValue.Files = append(manifestValue.Files, manifestFile{Path: "fixture_public_key.base64", SHA256: rawDigest(publicKeyRaw), SchemaID: "ed25519-public-key", SchemaVersion: "1"})
	manifestValue.Signing.Mode, manifestValue.Signing.Development, manifestValue.Signing.NonAuthoritative = "fixture_test_only", true, true
	manifestValue.Signing.KeyID = proofsign.KeyID(publicKey)
	manifestValue.Signing.PublicKeyPath = filepath.ToSlash(filepath.Join(fixtureRoot, "fixture_public_key.base64"))
	manifestValue.Signing.PublicKeySHA256 = rawDigest(publicKeyRaw)
	manifestValue.Signing.Derivation = "sha256(gait-effects-fixture-producer-key-v1) as Ed25519 seed; never used by production/default grading"
	manifestRaw, err := json.MarshalIndent(manifestValue, "", "  ")
	if err != nil {
		return err
	}
	files["fixture-manifest.json"] = append(manifestRaw, '\n')
	if update {
		if err := os.MkdirAll(root, 0o750); err != nil {
			return err
		}
		for name, raw := range files {
			if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
				return err
			}
		}
		return nil
	}
	for name, expected := range files {
		actual, readErr := os.ReadFile(filepath.Join(root, name)) // #nosec G304 -- name is one of the fixed generator-owned fixture filenames.
		if readErr != nil {
			return readErr
		}
		if string(actual) != string(expected) {
			return fmt.Errorf("fixture drift: %s (sha256 %s != %s)", name, rawDigest(actual), rawDigest(expected))
		}
	}
	return nil
}

func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func int64ptr(value int64) *int64 { return &value }
func fail(err error)              { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
