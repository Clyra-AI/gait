package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	fixtureRootRel       = "testdata/action-contract-interop/v1"
	manifestRel          = "testdata/action-contract-interop/v1/expected/fixture-manifest.json"
	activationName       = "activated-action-contract.json"
	fixtureVersion       = "1"
	producerVersion      = "v1.4.0"
	validFrom            = "2026-07-19T00:00:00Z"
	validUntil           = "2027-07-19T00:00:00Z"
	policyDigest         = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	publicKeyRel         = "testdata/action-contract-interop/v1/fixture-signing-key.public.b64"
	developmentKeyPhrase = "gait-action-contract-activation-development-key-v1"
)

type sourceManifest struct {
	FixtureVersion string `json:"fixture_version"`
	Producer       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Schemas struct {
		Artifact string `json:"artifact"`
		Contract string `json:"contract"`
	} `json:"schemas"`
	Scenarios []struct {
		ScenarioID   string `json:"scenario_id"`
		ArtifactPath string `json:"artifact_path"`
		Current      bool   `json:"current"`
	} `json:"scenarios"`
}

type activationManifest struct {
	FixtureVersion string `json:"fixture_version"`
	Producer       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"producer"`
	Schemas struct {
		Artifact string `json:"artifact"`
		Contract string `json:"contract"`
	} `json:"schemas"`
	Signing struct {
		Mode          string `json:"mode"`
		Development   bool   `json:"development_signing"`
		KeyID         string `json:"key_id"`
		PublicKeyPath string `json:"public_key_path"`
		Derivation    string `json:"derivation"`
	} `json:"signing"`
	Scenarios []activationScenario `json:"scenarios"`
}

type activationScenario struct {
	ScenarioID              string   `json:"scenario_id"`
	ProposalPath            string   `json:"proposal_path"`
	ProposalSHA256          string   `json:"proposal_sha256"`
	CanonicalContentDigest  string   `json:"canonical_content_digest"`
	ProposalArtifactID      string   `json:"proposal_artifact_id"`
	ContractID              string   `json:"contract_id"`
	ContractFamilyID        string   `json:"contract_family_id"`
	Revision                int      `json:"revision"`
	Current                 bool     `json:"current"`
	ActivationPath          string   `json:"activation_path"`
	ActivationSHA256        string   `json:"activation_sha256"`
	ActivationArtifactID    string   `json:"activation_artifact_id"`
	ActivationSchemaVersion string   `json:"activation_schema_version"`
	ContractSchemaVersion   string   `json:"contract_schema_version"`
	DevelopmentSigning      bool     `json:"development_signing"`
	ActivationStatus        string   `json:"activation_status"`
	ActivationReasonCodes   []string `json:"activation_reason_codes,omitempty"`
}

func main() {
	check := flag.Bool("check", false, "verify generated activation fixtures byte-for-byte")
	update := flag.Bool("update", false, "write generated activation fixtures")
	rootFlag := flag.String("repo-root", ".", "repository root")
	flag.Parse()
	if *check == *update {
		fatal("exactly one of --check or --update is required")
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fatal("resolve repo root: %v", err)
	}
	if err := run(root, *check); err != nil {
		fatal("%v", err)
	}
}

func run(root string, check bool) error {
	manifestRaw, err := os.ReadFile(filepath.Join(root, manifestRel)) // #nosec G304 -- root is the explicitly selected repository and manifest path is fixed.
	if err != nil {
		return err
	}
	var source sourceManifest
	if err := json.Unmarshal(manifestRaw, &source); err != nil {
		return fmt.Errorf("decode source manifest: %w", err)
	}
	if source.FixtureVersion != fixtureVersion || source.Producer.Name != "wrkr" || source.Producer.Version != "v1.14.0" || source.Schemas.Artifact != "1" || source.Schemas.Contract != "3" {
		return errors.New("source manifest is not the released Wrkr v1.14.0 fixture contract")
	}
	seed := sha256.Sum256([]byte(developmentKeyPhrase))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := actioncontract.DevelopmentPublicKey()
	if !bytes.Equal(privateKey.Public().(ed25519.PublicKey), publicKey) {
		return errors.New("fixture key does not match Gait deterministic development key")
	}
	publicRaw, err := os.ReadFile(filepath.Join(root, publicKeyRel)) // #nosec G304 -- root is the explicitly selected repository and public-key path is fixed.
	if err != nil || strings.TrimSpace(string(publicRaw)) != base64.StdEncoding.EncodeToString(publicKey) {
		return errors.New("fixture public key provenance does not match Gait development key")
	}

	tempRoot, err := os.MkdirTemp("", "gait-action-contract-fixtures-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempRoot) }()
	generatedRoot := filepath.Join(tempRoot, "expected")
	if err := os.MkdirAll(generatedRoot, 0o755); err != nil { // #nosec G301 -- temporary generator tree contains public fixture bytes only.
		return err
	}
	result := activationManifest{FixtureVersion: fixtureVersion}
	result.Producer.Name = actioncontract.ActivatedProducer
	result.Producer.Version = producerVersion
	result.Schemas.Artifact = actioncontract.ActivatedSchemaVersion
	result.Schemas.Contract = actioncontract.ActivatedContractVersion
	result.Signing.Mode = "fixture_only_deterministic_development_key"
	result.Signing.Development = true
	result.Signing.KeyID = proofsign.KeyID(publicKey)
	result.Signing.PublicKeyPath = publicKeyRel
	result.Signing.Derivation = "sha256(" + developmentKeyPhrase + ") as Ed25519 seed; never used by production/default activation"

	scenarios := append([]struct {
		ScenarioID   string `json:"scenario_id"`
		ArtifactPath string `json:"artifact_path"`
		Current      bool   `json:"current"`
	}{}, source.Scenarios...)
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ScenarioID < scenarios[j].ScenarioID })
	seenScenarioIDs := make(map[string]struct{}, len(scenarios))
	for _, scenario := range scenarios {
		if _, seen := seenScenarioIDs[scenario.ScenarioID]; seen {
			return fmt.Errorf("duplicate fixture scenario id: %s", scenario.ScenarioID)
		}
		seenScenarioIDs[scenario.ScenarioID] = struct{}{}
		activationRel, err := activationRelativePath(scenario.ScenarioID)
		if err != nil {
			return err
		}
		proposalRoot := filepath.Join(root, fixtureRootRel, "expected")
		proposalPath, err := safeFixturePath(proposalRoot, scenario.ArtifactPath)
		if err != nil {
			return fmt.Errorf("unsafe proposal path for %s: %w", scenario.ScenarioID, err)
		}
		if err := rejectSymlinkAncestors(proposalRoot, proposalPath); err != nil {
			return fmt.Errorf("unsafe proposal path for %s: %w", scenario.ScenarioID, err)
		}
		artifact, raw, err := actioncontract.ReadArtifact(proposalPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", scenario.ScenarioID, err)
		}
		proposalValidation := actioncontract.ValidateArtifact(artifact, actioncontract.ValidationOptions{Now: fixedEvaluationTime()})
		if !proposalValidation.Valid {
			result.Scenarios = append(result.Scenarios, activationScenario{
				ScenarioID: scenario.ScenarioID, ProposalPath: filepath.ToSlash(filepath.Join("expected", scenario.ArtifactPath)),
				ProposalSHA256: actioncontract.RawDigest(raw), CanonicalContentDigest: artifact.CanonicalContentDigest,
				ProposalArtifactID: artifact.ArtifactID, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID,
				Revision: artifact.Revision, Current: scenario.Current, ActivationStatus: "not_activated",
				ActivationReasonCodes: proposalValidation.Reasons,
			})
			continue
		}
		selection, err := actioncontract.LoadSelectionEvidence(filepath.Join(root, manifestRel), proposalPath, artifact, raw)
		if err != nil {
			return fmt.Errorf("load selection %s: %w", scenario.ScenarioID, err)
		}
		activated, _, err := actioncontract.Activate(artifact, actioncontract.ActivationOptions{
			PolicyDigest: policyDigest, ActivatingPrincipal: "principal:gait-fixture",
			AuthorityRefs: []string{"authority:gait-fixture"}, Target: "target:fixture",
			Environment: "test", Mode: actioncontract.ActivationContextOnly,
			ValidFrom: validFrom, ValidUntil: validUntil, AllowDevelopmentSigning: true,
			Selection: &selection, EvaluationTime: fixedEvaluationTime(),
		})
		if err != nil {
			return fmt.Errorf("activate %s: %w", scenario.ScenarioID, err)
		}
		if !activated.DevelopmentSigning || activated.Signature.KeyID != proofsign.KeyID(publicKey) {
			return fmt.Errorf("activation %s is not marked with the fixture signing provenance", scenario.ScenarioID)
		}
		if valid, err := actioncontract.VerifyActivationWithOptions(activated, publicKey, actioncontract.VerificationOptions{AllowDevelopmentSigning: true, Proposal: &artifact, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}); err != nil || !valid {
			return fmt.Errorf("verify generated activation %s: %v", scenario.ScenarioID, err)
		}
		encoded, err := json.MarshalIndent(activated, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		activationPath, err := safeFixturePath(generatedRoot, activationRel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(activationPath), 0o755); err != nil { // #nosec G301 -- generated fixture directories contain public compatibility bytes only.
			return err
		}
		if err := os.WriteFile(activationPath, encoded, 0o644); err != nil { // #nosec G306 -- generated activation fixture is intentionally public test data.
			return err
		}
		result.Scenarios = append(result.Scenarios, activationScenario{
			ScenarioID: scenario.ScenarioID, ProposalPath: filepath.ToSlash(filepath.Join("expected", scenario.ArtifactPath)),
			ProposalSHA256: actioncontract.RawDigest(raw), CanonicalContentDigest: artifact.CanonicalContentDigest,
			ProposalArtifactID: artifact.ArtifactID, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID,
			Revision: artifact.Revision, Current: scenario.Current, ActivationPath: filepath.ToSlash(filepath.Join("expected", activationRel)),
			ActivationSHA256: actioncontract.RawDigest(encoded), ActivationArtifactID: activated.ArtifactID,
			ActivationSchemaVersion: activated.SchemaVersion, ContractSchemaVersion: activated.Proposal.ContractSchemaVersion,
			DevelopmentSigning: activated.DevelopmentSigning, ActivationStatus: "activated",
		})
	}
	manifestEncoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	manifestEncoded = append(manifestEncoded, '\n')
	if err := os.WriteFile(filepath.Join(generatedRoot, "activation-fixture-manifest.json"), manifestEncoded, 0o644); err != nil { // #nosec G306 -- generated manifest is intentionally public test data.
		return err
	}

	destination := filepath.Join(root, fixtureRootRel, "expected")
	if err := reconcileActivationFiles(destination, result, check); err != nil {
		return err
	}
	for _, scenario := range result.Scenarios {
		if scenario.ActivationStatus != "activated" {
			continue
		}
		activationRel, err := activationRelativePath(scenario.ScenarioID)
		if err != nil {
			return err
		}
		generatedPath, err := safeFixturePath(generatedRoot, activationRel)
		if err != nil {
			return err
		}
		destinationPath, err := safeFixturePath(destination, activationRel)
		if err != nil {
			return err
		}
		if err := rejectSymlinkAncestors(destination, destinationPath); err != nil {
			return err
		}
		if err := compareOrCopy(generatedPath, destinationPath, check); err != nil {
			return err
		}
	}
	manifestPath, err := safeFixturePath(destination, "activation-fixture-manifest.json")
	if err != nil {
		return err
	}
	if err := rejectSymlinkAncestors(destination, manifestPath); err != nil {
		return err
	}
	return compareOrCopy(filepath.Join(generatedRoot, "activation-fixture-manifest.json"), manifestPath, check)
}

func compareOrCopy(source, destination string, check bool) error {
	generated, err := os.ReadFile(source) // #nosec G304 -- source is generated inside the private temporary tree.
	if err != nil {
		return err
	}
	info, statErr := os.Lstat(destination) // #nosec G304 -- destination is fixed below the selected repository fixture root.
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if statErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("refusing non-regular fixture destination: %s", destination)
	}
	existing, readErr := os.ReadFile(destination) // #nosec G304 -- destination is fixed below the selected repository fixture root.
	if check {
		if readErr != nil || !bytes.Equal(generated, existing) {
			return fmt.Errorf("activation fixture stale: %s", destination)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { // #nosec G301 -- fixture directories contain public compatibility bytes only.
		return err
	}
	return os.WriteFile(destination, generated, 0o644) // #nosec G306 -- generated fixture is intentionally public test data.
}

func activationRelativePath(scenarioID string) (string, error) {
	if strings.TrimSpace(scenarioID) == "" || scenarioID == "." || scenarioID == ".." || strings.ContainsAny(scenarioID, `/\\`) {
		return "", fmt.Errorf("unsafe activation scenario id: %q", scenarioID)
	}
	return filepath.Join(scenarioID, activationName), nil
}

func safeFixturePath(root, relative string) (string, error) {
	if strings.ContainsRune(relative, 0) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes fixture root: %q", relative)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes fixture root: %q", relative)
	}
	return pathAbs, nil
}

func rejectSymlinkAncestors(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes fixture root: %s", target)
	}
	current := rootAbs
	if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink fixture root: %s", root)
	}
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in fixture path: %s", current)
		}
	}
	return nil
}

func reconcileActivationFiles(expectedRoot string, manifest activationManifest, check bool) error {
	expected := make(map[string]struct{})
	for _, scenario := range manifest.Scenarios {
		if scenario.ActivationStatus != "activated" {
			continue
		}
		rel, err := activationRelativePath(scenario.ScenarioID)
		if err != nil {
			return err
		}
		if _, err := safeFixturePath(expectedRoot, rel); err != nil {
			return err
		}
		expected[filepath.Clean(rel)] = struct{}{}
	}
	actual, err := collectActivationFiles(expectedRoot)
	if err != nil {
		return err
	}
	for _, rel := range actual {
		if _, ok := expected[filepath.Clean(rel)]; ok {
			continue
		}
		path, err := safeFixturePath(expectedRoot, rel)
		if err != nil {
			return err
		}
		if check {
			return fmt.Errorf("obsolete activation fixture: %s", filepath.ToSlash(rel))
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-regular activation fixture: %s", path)
		}
		if err := os.Remove(path); err != nil { // #nosec G304 -- path is a regular activation file below the pinned fixture root.
			return err
		}
	}
	return nil
}

func collectActivationFiles(expectedRoot string) ([]string, error) {
	rootAbs, err := filepath.Abs(expectedRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(rootAbs)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("invalid activation fixture root: %s", rootAbs)
	}
	var paths []string
	err = filepath.WalkDir(rootAbs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == rootAbs {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in activation fixture root: %s", path)
		}
		if entry.Name() != activationName {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("refusing non-regular activation fixture: %s", path)
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func fixedEvaluationTime() (t time.Time) {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
