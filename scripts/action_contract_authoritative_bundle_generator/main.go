// Command action_contract_authoritative_bundle_generator creates and verifies
// the release-owner Action Contract evidence bundle. Unlike the checked-in
// conformance fixtures, every signature is made with a fresh release-time key.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	sourceProposal      = "testdata/action-contract-interop/v1/expected/compensation/pac-4b7f1402784256ce.json"
	sourceActivation    = "testdata/action-contract-interop/v1/expected/compensation/activated-action-contract.json"
	sourceSelection     = "testdata/action-contract-interop/v1/expected/fixture-manifest.json"
	sourceAction        = "testdata/action-contract-evidence/v1/runtime-action.json"
	sourceReadiness     = "testdata/action-contract-evidence/v1/runtime-readiness.json"
	sourceLifecycle     = "testdata/action-contract-evidence/v1/compensation-required-started-completed/lifecycle.json"
	workflowIdentity    = "github.com/Clyra-AI/gait/.github/workflows/release.yml"
	releaseOwner        = "gait-release-owner"
	maxBundleEntryBytes = 16 << 20
)

type digestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type manifest struct {
	SchemaID          string        `json:"schema_id"`
	SchemaVersion     string        `json:"schema_version"`
	ReleaseTag        string        `json:"release_tag"`
	PeeledCommit      string        `json:"peeled_commit"`
	Authoritative     bool          `json:"authoritative"`
	FixtureOnly       bool          `json:"fixture_only"`
	DevelopmentSign   bool          `json:"development_signing"`
	Quarantine        bool          `json:"quarantine"`
	Generator         string        `json:"generator"`
	Workflow          string        `json:"workflow"`
	GeneratorVersion  string        `json:"generator_version"`
	Signing           signingInfo   `json:"signing"`
	Artifacts         []digestEntry `json:"artifacts"`
	ReferencedSchemas []digestEntry `json:"referenced_schemas"`
	Scenarios         []scenario    `json:"scenarios"`
}

type signingInfo struct {
	Algorithm     string `json:"algorithm"`
	Authority     string `json:"authority"`
	KeyOrigin     string `json:"key_origin"`
	PublicKeyPath string `json:"public_key_path"`
	PublicKeySHA  string `json:"public_key_sha256"`
	KeyID         string `json:"key_id"`
}

type scenario struct {
	ID                    string `json:"id"`
	LifecyclePath         string `json:"lifecycle_path"`
	LifecycleSHA256       string `json:"lifecycle_sha256"`
	ExpectedAuthoritative bool   `json:"expected_authoritative"`
	ExpectedQuarantine    bool   `json:"expected_quarantine"`
}

type lifecyclePack struct {
	Records []actioncontract.LifecycleRecord `json:"records"`
}

func main() {
	var root, out, tag, commit, workflow string
	var verify string
	var checksums string
	flag.StringVar(&root, "repo-root", ".", "repository root")
	flag.StringVar(&out, "out", "dist/action-contract-authoritative", "output directory")
	flag.StringVar(&tag, "release-tag", "", "release tag, for example v1.7.1")
	flag.StringVar(&commit, "peeled-commit", "", "peeled tag commit SHA")
	flag.StringVar(&workflow, "workflow", workflowIdentity, "release workflow identity")
	flag.StringVar(&verify, "verify", "", "verify an existing bundle instead of generating")
	flag.StringVar(&checksums, "checksums", "", "caller-trusted signed checksums file anchoring this bundle")
	flag.Parse()
	var err error
	if verify != "" {
		err = verifyBundle(verify, tag, commit, checksums)
	} else {
		if strings.TrimSpace(tag) == "" || strings.TrimSpace(commit) == "" {
			err = errors.New("--release-tag and --peeled-commit are required")
		} else {
			err = generateBundle(root, out, tag, commit, workflow)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateBundle(root, out, tag, commit, workflow string) error {
	tag, commit, workflow = strings.TrimSpace(tag), strings.TrimSpace(commit), strings.TrimSpace(workflow)
	if tag == "" || commit == "" || workflow == "" {
		return errors.New("release identity values must not be empty")
	}
	seed := releaseSigningSeed(tag, commit, workflow)
	var err error
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	proposal, proposalRaw, err := actioncontract.ReadArtifact(filepath.Join(root, sourceProposal))
	if err != nil {
		return err
	}
	activationRaw, err := os.ReadFile(filepath.Join(root, sourceActivation))
	if err != nil {
		return err
	}
	sourceActivationArtifact, err := actioncontract.ParseActivatedArtifact(activationRaw)
	if err != nil {
		return err
	}
	selection, err := actioncontract.LoadSelectionEvidence(filepath.Join(root, sourceSelection), filepath.Join(root, sourceProposal), proposal, proposalRaw)
	if err != nil {
		return err
	}
	activation, _, err := actioncontract.Activate(proposal, actioncontract.ActivationOptions{
		PolicyDigest: sourceActivationArtifact.PolicyDigest, ActivatingPrincipal: sourceActivationArtifact.ActivatingPrincipal,
		AuthorityRefs: sourceActivationArtifact.AuthorityRefs, Target: sourceActivationArtifact.Target, Environment: sourceActivationArtifact.Environment,
		Mode: sourceActivationArtifact.ActivationMode, ValidFrom: sourceActivationArtifact.Validity.NotBefore, ValidUntil: sourceActivationArtifact.Validity.NotAfter,
		ExplicitExceptions: sourceActivationArtifact.ExplicitExceptions, SigningPrivateKey: private, Selection: &selection,
		EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return fmt.Errorf("release activation: %w", err)
	}
	if activation.DevelopmentSigning {
		return errors.New("release activation unexpectedly marked development_signing")
	}

	readiness, readinessRaw, err := releaseReadiness(filepath.Join(root, sourceReadiness), proposal.ContractID, activation.PolicyDigest, private, public)
	if err != nil {
		return err
	}
	_, actionRaw, err := readRuntimeAction(filepath.Join(root, sourceAction))
	if err != nil {
		return err
	}
	lifecycleRaw, err := os.ReadFile(filepath.Join(root, sourceLifecycle))
	if err != nil {
		return err
	}
	var sourcePack lifecyclePack
	if err := json.Unmarshal(lifecycleRaw, &sourcePack); err != nil {
		return fmt.Errorf("parse lifecycle source: %w", err)
	}
	proposalRef := relationship("action_contract", proposal.ContractID, proposal.CanonicalContentDigest, actioncontract.ProposedContractSchemaID, actioncontract.ProposedContractVersion, actioncontract.ProposedProducer)
	activationBytes, err := json.Marshal(activation)
	if err != nil {
		return err
	}
	activationFileBytes := append(append([]byte(nil), activationBytes...), '\n')
	// Lifecycle relationship refs bind the canonical signed activation object,
	// matching core's activatedSignableDigest, rather than the transport file
	// bytes (which may include formatting or a trailing newline).
	activationRef := relationship("activated_action_contract", activation.ArtifactID, "sha256:"+strings.TrimPrefix(activation.Signature.SignedDigest, "sha256:"), actioncontract.ActivatedSchemaID, actioncontract.ActivatedSchemaVersion, actioncontract.ActivatedProducer)
	readinessDigest, err := digestJCS(readinessRaw)
	if err != nil {
		return err
	}
	actionDigest, err := digestJCS(actionRaw)
	if err != nil {
		return err
	}
	records, err := resignLifecycle(sourcePack.Records, proposalRef, activationRef, readiness, readinessDigest, actionDigest, private)
	if err != nil {
		return err
	}
	lifecycleBytes, err := json.MarshalIndent(lifecyclePack{Records: records}, "", "  ")
	if err != nil {
		return err
	}
	lifecycleBytes = append(lifecycleBytes, '\n')
	publicBytes := []byte(base64.StdEncoding.EncodeToString(public) + "\n")
	files := map[string][]byte{
		"manifest.json":          nil,
		"public-key.b64":         publicBytes,
		"proposal.json":          proposalRaw,
		"activation.json":        activationFileBytes,
		"runtime-action.json":    actionRaw,
		"runtime-readiness.json": readinessRaw,
		"lifecycle.json":         lifecycleBytes,
	}
	manifestValue := manifest{
		SchemaID: "https://gait.dev/schemas/v1/action-contract/authoritative-evidence-bundle-manifest.schema.json", SchemaVersion: "1",
		ReleaseTag: tag, PeeledCommit: strings.TrimSpace(commit), Authoritative: true, FixtureOnly: false, DevelopmentSign: false, Quarantine: false,
		Generator: "scripts/action_contract_authoritative_bundle_generator", Workflow: workflow, GeneratorVersion: "1",
		Signing:   signingInfo{Algorithm: "ed25519", Authority: releaseOwner, KeyOrigin: "deterministic_release_identity_non_secret_internal_integrity", PublicKeyPath: "public-key.b64", PublicKeySHA: rawDigest(publicBytes), KeyID: proofsign.KeyID(public)},
		Scenarios: []scenario{{ID: "compensation-required-started-completed", LifecyclePath: "lifecycle.json", ExpectedAuthoritative: true, ExpectedQuarantine: false}},
	}
	for path, data := range files {
		if path != "manifest.json" {
			manifestValue.Artifacts = append(manifestValue.Artifacts, digestEntry{Path: path, SHA256: rawDigest(data)})
		}
	}
	schemaDir := filepath.Join(root, "schemas/v1/action-contract")
	schemaPaths, err := filepath.Glob(filepath.Join(schemaDir, "*.schema.json"))
	if err != nil {
		return err
	}
	sort.Strings(schemaPaths)
	for _, schemaPath := range schemaPaths {
		data, readErr := os.ReadFile(schemaPath)
		if readErr != nil {
			return readErr
		}
		name := filepath.ToSlash(filepath.Join("schemas/v1/action-contract", filepath.Base(schemaPath)))
		files[name] = data
		manifestValue.ReferencedSchemas = append(manifestValue.ReferencedSchemas, digestEntry{Path: name, SHA256: rawDigest(data)})
	}
	sort.Slice(manifestValue.Artifacts, func(i, j int) bool { return manifestValue.Artifacts[i].Path < manifestValue.Artifacts[j].Path })
	manifestValue.Scenarios[0].LifecycleSHA256 = rawDigest(lifecycleBytes)
	manifestBytes, err := json.MarshalIndent(manifestValue, "", "  ")
	if err != nil {
		return err
	}
	files["manifest.json"] = append(manifestBytes, '\n')
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	for path, data := range files {
		destination := filepath.Join(out, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(destination, data, 0o600); err != nil {
			return err
		}
	}
	bundlePath := filepath.Join(out, "action-contract-authoritative-evidence-"+tag+".zip")
	if err := writeZip(bundlePath, files); err != nil {
		return err
	}
	return nil
}

func releaseReadiness(path, contractID, policy string, private ed25519.PrivateKey, public ed25519.PublicKey) (actioncontract.ReadinessResult, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return actioncontract.ReadinessResult{}, nil, err
	}
	var source actioncontract.ReadinessResult
	if err := json.Unmarshal(raw, &source); err != nil {
		return source, nil, err
	}
	for i := range source.Preconditions {
		item := &source.Preconditions[i]
		item.Producer = releaseOwner
		item.AcceptableProducers = []string{releaseOwner}
		item.ValidatorSignature, item.EvidenceDigest = "", ""
		item.EvidenceState, item.FreshnessState, item.Status = "verified", "fresh", ""
		item.ReasonCodes = nil
		claim, claimErr := actioncontract.CanonicalReadinessClaimDigest(actioncontract.ReadinessInput{ContractID: contractID, PolicyDigest: policy}, *item)
		if claimErr != nil {
			return source, nil, claimErr
		}
		item.EvidenceDigest = claim
		decoded, decodeErr := hex.DecodeString(strings.TrimPrefix(claim, "sha256:"))
		if decodeErr != nil {
			return source, nil, decodeErr
		}
		item.ValidatorSignature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, decoded))
	}
	result := actioncontract.EvaluateReadiness(actioncontract.ReadinessInput{Now: time.Date(2026, 7, 19, 1, 0, 3, 0, time.UTC), ContractID: contractID, PolicyDigest: policy, TrustedValidatorRefs: []string{releaseOwner}, TrustedValidatorKeys: map[string]ed25519.PublicKey{releaseOwner: public}, Preconditions: source.Preconditions})
	if !result.Ready || result.Status != actioncontract.ReadinessSatisfied {
		return result, nil, fmt.Errorf("release readiness is not satisfied: %v", result.ReasonCodes)
	}
	resultRaw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, nil, err
	}
	return result, append(resultRaw, '\n'), nil
}

func readRuntimeAction(path string) (actioncontract.RuntimeAction, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return actioncontract.RuntimeAction{}, nil, err
	}
	var wrapper struct {
		Action json.RawMessage `json:"action"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return actioncontract.RuntimeAction{}, nil, err
	}
	if len(wrapper.Action) == 0 {
		wrapper.Action = raw
	}
	var action actioncontract.RuntimeAction
	if err := json.Unmarshal(wrapper.Action, &action); err != nil {
		return action, nil, err
	}
	return action, wrapper.Action, nil
}

func resignLifecycle(source []actioncontract.LifecycleRecord, proposal, activation proof.RelationshipRef, readiness actioncontract.ReadinessResult, readinessDigest, actionDigest string, private ed25519.PrivateKey) ([]actioncontract.LifecycleRecord, error) {
	result := make([]actioncontract.LifecycleRecord, 0, len(source))
	digestMap := map[string]proof.RelationshipRef{}
	for _, original := range source {
		record := original
		record.ContractRef, record.ProposalRef = proposal, refPtr(proposal)
		if record.ActivationRef != nil {
			record.ActivationRef = refPtr(activation)
		}
		record.Correlation.ContractRef = refPtr(proposal)
		record.Correlation.ContentDigest = proposal.Digest
		if record.Correlation.EventRef != nil {
			eventRef := proposal
			if record.ActivationRef != nil {
				eventRef = activation
			}
			record.Correlation.EventRef = refPtr(eventRef)
		}
		if record.Correlation.CausalRef != nil {
			record.Correlation.CausalRef = refPtr(proposal)
		}
		record.PreconditionRefs = readinessRefs(readiness)
		if record.Decision != nil {
			copyReadiness := readiness
			record.Decision = &copyReadiness
		}
		record.EvidenceRefs = nil
		if record.Execution != nil {
			item := *record.Execution
			item.Binding = rewriteBinding(item.Binding, proposal, activation, readiness, readinessDigest, actionDigest, digestMap)
			updated, err := actioncontract.NewExecutionEvidence(item, private)
			if err != nil {
				return nil, err
			}
			digestMap[original.Execution.CanonicalContentDigest] = evidenceRef("execution", updated.EvidenceID, updated.CanonicalContentDigest, actioncontract.ExecutionEvidenceSchemaID)
			record.Execution = &updated
		}
		if record.Effect != nil {
			item := *record.Effect
			item.Binding = rewriteBinding(item.Binding, proposal, activation, readiness, readinessDigest, actionDigest, digestMap)
			item.ExecutionRef = rewriteRef(item.ExecutionRef, digestMap)
			updated, err := actioncontract.NewEffectEvent(item, private)
			if err != nil {
				return nil, err
			}
			digestMap[original.Effect.CanonicalContentDigest] = evidenceRef("effect_event", updated.EvidenceID, updated.CanonicalContentDigest, actioncontract.EffectEventSchemaID)
			record.Effect = &updated
		}
		if record.Containment != nil {
			item := *record.Containment
			item.Binding = rewriteBinding(item.Binding, proposal, activation, readiness, readinessDigest, actionDigest, digestMap)
			item.ExecutionRef = rewriteRef(item.ExecutionRef, digestMap)
			item.EffectRef = rewriteRef(item.EffectRef, digestMap)
			updated, err := actioncontract.NewContainmentEvidence(item, private)
			if err != nil {
				return nil, err
			}
			digestMap[original.Containment.CanonicalContentDigest] = evidenceRef("containment", updated.EvidenceID, updated.CanonicalContentDigest, actioncontract.ContainmentEvidenceSchemaID)
			record.Containment = &updated
		}
		if record.Compensation != nil {
			item := *record.Compensation
			item.Binding = rewriteBinding(item.Binding, proposal, activation, readiness, readinessDigest, actionDigest, digestMap)
			item.ExecutionRef = rewriteRef(item.ExecutionRef, digestMap)
			updated, err := actioncontract.NewCompensationEvidence(item, private)
			if err != nil {
				return nil, err
			}
			digestMap[original.Compensation.CanonicalContentDigest] = evidenceRef("compensation", updated.EvidenceID, updated.CanonicalContentDigest, actioncontract.CompensationEvidenceSchemaID)
			record.Compensation = &updated
		}
		opts := actioncontract.LifecycleRecordOptions{Kind: record.Kind, OccurredAt: mustTime(record.OccurredAt), ContractRef: proposal, ContractFamilyID: proposalFamily(record, proposal), Revision: record.Revision, ProposalRef: record.ProposalRef, ActivationRef: record.ActivationRef, PreconditionRefs: record.PreconditionRefs, Decision: record.Decision, Execution: record.Execution, Effect: record.Effect, Containment: record.Containment, Compensation: record.Compensation, Control: record.Control, ReasonCodes: record.ReasonCodes, Correlation: record.Correlation, ImmutableObject: record.ImmutableObject, SigningPrivateKey: private}
		updatedRecord, err := actioncontract.NewLifecycleRecord(opts)
		if err != nil {
			return nil, err
		}
		result = append(result, updatedRecord)
	}
	return result, nil
}

func proposalFamily(record actioncontract.LifecycleRecord, proposal proof.RelationshipRef) string {
	if record.ContractFamilyID != "" {
		return record.ContractFamilyID
	}
	return "pacf-authoritative"
}

func rewriteBinding(binding actioncontract.EvidenceBinding, proposal, activation proof.RelationshipRef, readiness actioncontract.ReadinessResult, readinessDigest, actionDigest string, digestMap map[string]proof.RelationshipRef) actioncontract.EvidenceBinding {
	binding.ContractRef, binding.ActivationRef = proposal, activation
	binding.ReadinessRef.Digest, binding.DecisionRef.Digest = readinessDigest, readinessDigest
	binding.PolicyRef.Digest = readiness.PolicyDigest
	binding.RuntimeActionRef.Digest = actionDigest
	binding.Correlation.ContractRef, binding.Correlation.ContentDigest = refPtr(proposal), proposal.Digest
	for i := range binding.CausalRefs {
		binding.CausalRefs[i] = rewriteRef(binding.CausalRefs[i], digestMap)
	}
	return binding
}

func rewriteRef(ref proof.RelationshipRef, mapping map[string]proof.RelationshipRef) proof.RelationshipRef {
	if updated, ok := mapping[ref.Digest]; ok {
		return updated
	}
	return ref
}
func readinessRefs(readiness actioncontract.ReadinessResult) []proof.RelationshipRef {
	refs := make([]proof.RelationshipRef, 0, len(readiness.Preconditions))
	for _, item := range readiness.Preconditions {
		refs = append(refs, relationship("precondition", item.RequirementID, item.EvidenceDigest, actioncontract.RuntimeReadinessSchemaID, "1", actioncontract.EvidenceProducer))
	}
	return refs
}
func refPtr(ref proof.RelationshipRef) *proof.RelationshipRef { return &ref }
func evidenceRef(kind, id, digest, schema string) proof.RelationshipRef {
	return relationship(kind, id, digest, schema, "1", actioncontract.EvidenceProducer)
}
func relationship(kind, id, digest, schema, version, product string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: digest, SchemaID: schema, SchemaVersion: version, SourceProduct: product}
}
func mustTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func releaseSigningSeed(tag, commit, workflow string) []byte {
	tag, commit, workflow = strings.TrimSpace(tag), strings.TrimSpace(commit), strings.TrimSpace(workflow)
	material := "gait.authoritative.action-contract.release-key.v1\x00" + workflow + "\x00" + tag + "\x00" + commit
	sum := sha256.Sum256([]byte(material))
	return sum[:]
}
func digestJCS(raw []byte) (string, error) {
	d, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	return "sha256:" + strings.TrimPrefix(d, "sha256:"), nil
}

func writeZip(path string, files map[string][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		header := &zip.FileHeader{Name: filepath.ToSlash(path), Method: zip.Deflate}
		header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		header.SetMode(0o600)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := writer.Write(files[path]); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

func verifyBundle(path, expectedTag, expectedCommit, checksumsPath string) error {
	expectedTag, expectedCommit = strings.TrimSpace(expectedTag), strings.TrimSpace(expectedCommit)
	if strings.TrimSpace(checksumsPath) == "" {
		return errors.New("caller-trusted signed checksums anchor is required")
	}
	checksumsRaw, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("read checksums anchor: %w", err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	files := map[string][]byte{}
	seenNames := map[string]struct{}{}
	for _, file := range reader.File {
		if _, exists := seenNames[file.Name]; exists {
			return fmt.Errorf("duplicate bundle entry: %s", file.Name)
		}
		seenNames[file.Name] = struct{}{}
		if file.FileInfo().IsDir() || filepath.IsAbs(file.Name) || strings.Contains(filepath.ToSlash(file.Name), "../") {
			return errors.New("unsafe bundle path")
		}
		if file.UncompressedSize64 > maxBundleEntryBytes {
			return fmt.Errorf("bundle entry exceeds size limit: %s", file.Name)
		}
		handle, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		data, readErr := io.ReadAll(io.LimitReader(handle, maxBundleEntryBytes+1))
		_ = handle.Close()
		if readErr != nil {
			return readErr
		}
		if int64(len(data)) > maxBundleEntryBytes {
			return fmt.Errorf("bundle entry exceeds size limit: %s", file.Name)
		}
		files[file.Name] = data
	}
	var m manifest
	manifestRaw, ok := files["manifest.json"]
	if !ok {
		return errors.New("manifest missing")
	}
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		return err
	}
	checksumLines := strings.Split(string(checksumsRaw), "\n")
	anchoredDigest := func(name, expected string) error {
		for _, line := range checksumLines {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == name && strings.EqualFold(fields[0], strings.TrimPrefix(expected, "sha256:")) {
				return nil
			}
		}
		return fmt.Errorf("trusted checksums anchor missing or mismatched: %s", name)
	}
	if !m.Authoritative || m.FixtureOnly || m.DevelopmentSign || m.Quarantine || m.Signing.Authority != releaseOwner || m.Signing.Algorithm != "ed25519" {
		return errors.New("bundle contains non-authoritative signing markers")
	}
	if m.Signing.KeyOrigin != "deterministic_release_identity_non_secret_internal_integrity" {
		return errors.New("unsupported release key origin")
	}
	if expectedTag != "" && m.ReleaseTag != expectedTag {
		return errors.New("release tag mismatch")
	}
	if expectedCommit != "" && m.PeeledCommit != expectedCommit {
		return errors.New("peeled commit mismatch")
	}
	publicRaw, ok := files[m.Signing.PublicKeyPath]
	if !ok {
		return errors.New("public key missing")
	}
	publicBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(publicRaw)))
	if err != nil || len(publicBytes) != ed25519.PublicKeySize {
		return errors.New("public key invalid")
	}
	if rawDigest(publicRaw) != m.Signing.PublicKeySHA {
		return errors.New("public key digest mismatch")
	}
	for _, entry := range append(append([]digestEntry{}, m.Artifacts...), m.ReferencedSchemas...) {
		data, ok := files[entry.Path]
		if !ok || rawDigest(data) != entry.SHA256 {
			return fmt.Errorf("artifact digest mismatch: %s", entry.Path)
		}
	}
	bundleRaw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	prefix := "action-contract-authoritative/"
	for _, entry := range append(append([]digestEntry{}, m.Artifacts...), m.ReferencedSchemas...) {
		if err := anchoredDigest(prefix+entry.Path, entry.SHA256); err != nil {
			return err
		}
	}
	if err := anchoredDigest(prefix+"action-contract-authoritative-evidence-"+m.ReleaseTag+".zip", rawDigest(bundleRaw)); err != nil {
		return err
	}
	for name, data := range files {
		if bytes.Contains(data, []byte(`"development_signing":true`)) || bytes.Contains(data, []byte(`"quarantine":true`)) || bytes.Contains(data, []byte(`"fixture_only":true`)) {
			return fmt.Errorf("non-authoritative marker in %s", name)
		}
	}
	proposalRaw, ok := files["proposal.json"]
	if !ok {
		return errors.New("proposal missing")
	}
	proposal, err := actioncontract.ParseArtifact(proposalRaw)
	if err != nil {
		return err
	}
	activationRaw, ok := files["activation.json"]
	if !ok {
		return errors.New("activation missing")
	}
	activation, err := actioncontract.ParseActivatedArtifact(activationRaw)
	if err != nil {
		return err
	}
	if activation.DevelopmentSigning {
		return errors.New("development activation in authoritative bundle")
	}
	if valid, err := actioncontract.VerifyActivationWithOptions(activation, ed25519.PublicKey(publicBytes), actioncontract.VerificationOptions{Proposal: &proposal, EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}); err != nil || !valid {
		return fmt.Errorf("activation verification failed: %v", err)
	}
	var pack lifecyclePack
	if err := json.Unmarshal(files["lifecycle.json"], &pack); err != nil {
		return err
	}
	for _, scenario := range m.Scenarios {
		if scenario.LifecyclePath == "" || !scenario.ExpectedAuthoritative || scenario.ExpectedQuarantine {
			return errors.New("scenario authority markers invalid")
		}
		scenarioRaw, exists := files[scenario.LifecyclePath]
		if !exists || rawDigest(scenarioRaw) != scenario.LifecycleSHA256 {
			return fmt.Errorf("scenario lifecycle digest mismatch: %s", scenario.ID)
		}
		var scenarioPack lifecyclePack
		if err := json.Unmarshal(scenarioRaw, &scenarioPack); err != nil || len(scenarioPack.Records) == 0 {
			return fmt.Errorf("scenario lifecycle invalid: %s", scenario.ID)
		}
		if scenario.LifecyclePath == "lifecycle.json" && len(scenarioPack.Records) != len(pack.Records) {
			return errors.New("manifest lifecycle scenario mismatch")
		}
		for _, record := range scenarioPack.Records {
			valid, verifyErr := actioncontract.VerifyLifecycleRecord(record, ed25519.PublicKey(publicBytes))
			if verifyErr != nil || !valid {
				return fmt.Errorf("lifecycle signature invalid: %v", verifyErr)
			}
			if record.Execution != nil {
				if valid, e := actioncontract.VerifyExecutionEvidence(*record.Execution, ed25519.PublicKey(publicBytes)); e != nil || !valid {
					return fmt.Errorf("execution evidence invalid: %v", e)
				}
			}
			if record.Effect != nil {
				if valid, e := actioncontract.VerifyEffectEvent(*record.Effect, ed25519.PublicKey(publicBytes)); e != nil || !valid {
					return fmt.Errorf("effect evidence invalid: %v", e)
				}
			}
			if record.Containment != nil {
				if valid, e := actioncontract.VerifyContainmentEvidence(*record.Containment, ed25519.PublicKey(publicBytes)); e != nil || !valid {
					return fmt.Errorf("containment evidence invalid: %v", e)
				}
			}
			if record.Compensation != nil {
				if valid, e := actioncontract.VerifyCompensationEvidence(*record.Compensation, ed25519.PublicKey(publicBytes)); e != nil || !valid {
					return fmt.Errorf("compensation evidence invalid: %v", e)
				}
			}
		}
	}
	if snapshot, err := actioncontract.ReduceVerifiedLifecycle(pack.Records, ed25519.PublicKey(publicBytes)); err != nil || snapshot.ExecutionStatus != "succeeded" || snapshot.EffectStatus != "validated" || snapshot.ContainmentStatus != "completed" || snapshot.CompensationStatus != "completed" {
		return fmt.Errorf("authoritative lifecycle reduction failed: %v", err)
	}
	var readiness actioncontract.ReadinessResult
	if err := json.Unmarshal(files["runtime-readiness.json"], &readiness); err != nil {
		return err
	}
	var action actioncontract.RuntimeAction
	if err := json.Unmarshal(files["runtime-action.json"], &action); err != nil {
		return err
	}
	conformance := actioncontract.VerifyLifecycleConformance(actioncontract.LifecycleConformanceInput{
		Proposal: proposal, Activation: activation, ActivationPublicKey: ed25519.PublicKey(publicBytes), RuntimeAction: action,
		Readiness: readiness, ReadinessTrustedValidatorRefs: []string{releaseOwner}, ReadinessTrustedValidatorKeys: map[string]ed25519.PublicKey{releaseOwner: ed25519.PublicKey(publicBytes)},
		LifecycleRecords: pack.Records, LifecyclePublicKey: ed25519.PublicKey(publicBytes), EvaluationTime: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		Expectation: actioncontract.LifecycleConformanceExpectation{ExecutionOutcome: "succeeded", EffectOutcome: "validated", ContainmentOutcome: "completed", CompensationOutcome: "completed", RequireComplete: true},
	})
	if !conformance.Valid || !conformance.AuthoritativeSuccess {
		return fmt.Errorf("authoritative conformance failed: %v", conformance.ReasonCodes)
	}
	if len(m.ReferencedSchemas) == 0 || len(m.Artifacts) == 0 || len(m.Scenarios) == 0 {
		return errors.New("manifest completeness failure")
	}
	return nil
}
