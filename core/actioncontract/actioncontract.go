// Package actioncontract is Gait's explicit boundary for consuming Wrkr
// report-only proposed Action Contract artifacts.  A proposal is evidence,
// never an authorization.  Activation is a separate, signed object that can
// be handed to an execution boundary by an operator or authority.
package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// schemaAssets is package-owned and immutable at runtime. Validation never
// searches the caller's working directory for schemas.
//
//go:embed schemaassets/*.json
var schemaAssets embed.FS

const (
	ProposedSchemaID         = "https://wrkr.dev/schemas/v1/proposed-action-contract-artifact.schema.json"
	ProposedContractSchemaID = "https://wrkr.dev/schemas/v1/proposed-action-contract-v3.schema.json"
	ProposedSchemaVersion    = "1"
	ProposedContractVersion  = "3"
	ProposedProducer         = "wrkr"

	ActivatedSchemaID        = "https://gait.dev/schemas/v1/activated-action-contract-artifact.schema.json"
	ActivatedSchemaVersion   = "1"
	ActivatedContractVersion = "1"
	ActivatedProducer        = "gait"
)

const (
	ReasonMalformedArtifact            = "artifact_malformed"
	ReasonUnsupportedArtifactSchema    = "artifact_schema_unsupported"
	ReasonUnsupportedContractSchema    = "contract_schema_unsupported"
	ReasonUnsupportedProducer          = "producer_unsupported"
	ReasonReportOnlyRequired           = "report_only_required"
	ReasonMissingContractID            = "contract_id_missing"
	ReasonMissingFamilyID              = "contract_family_id_missing"
	ReasonMissingCompositionRef        = "composition_ref_missing"
	ReasonMissingSourceRefs            = "source_refs_missing"
	ReasonMissingEvidenceRefs          = "evidence_refs_missing"
	ReasonRevisionInvalid              = "revision_invalid"
	ReasonRevisionIdentityMismatch     = "revision_identity_mismatch"
	ReasonContractIdentityMismatch     = "contract_identity_mismatch"
	ReasonArtifactIdentityMismatch     = "artifact_identity_mismatch"
	ReasonDigestMismatch               = "canonical_digest_mismatch"
	ReasonContractDigestMismatch       = "contract_digest_mismatch"
	ReasonUnsupportedConstraint        = "constraint_unsupported"
	ReasonStaleProposal                = "proposal_stale"
	ReasonSupersededProposal           = "proposal_superseded"
	ReasonContradictoryProposal        = "proposal_contradictory"
	ReasonActivationModeUnsupported    = "activation_mode_unsupported"
	ReasonPolicyDigestMissing          = "policy_digest_missing"
	ReasonPrincipalMissing             = "activating_principal_missing"
	ReasonAuthorityRefsMissing         = "authority_refs_missing"
	ReasonTargetMissing                = "target_missing"
	ReasonEnvironmentMissing           = "environment_missing"
	ReasonValidityInvalid              = "validity_invalid"
	ReasonRevisionReactivationRequired = "revision_reactivation_required"
	ReasonSelectionRequired            = "explicit_selection_required"
	ReasonAmbiguousSelection           = "ambiguous_selection"
	ReasonAuthorizationRequired        = "authorization_required"
	ReasonSchemaValidationFailed       = "schema_validation_failed"
	ReasonSigningKeyRequired           = "signing_key_required"
	ReasonDevelopmentSigningForbidden  = "development_signing_forbidden"
	ReasonDevelopmentSigningUnverified = "development_signing_unverified"
	ReasonSelectionEvidenceRequired    = "selection_evidence_required"
	ReasonSelectionMismatch            = "selection_mismatch"
	ReasonSelectionNotCurrent          = "selection_not_current"
	ReasonSelectionAmbiguous           = "selection_ambiguous"
	ReasonBindingMismatch              = "proposal_binding_mismatch"
	ReasonEvaluationTimeInvalid        = "evaluation_time_invalid"
	ReasonActivationNotYetValid        = "activation_not_yet_valid"
	ReasonActivationExpired            = "activation_expired"
)

var (
	safeArtifactID = regexp.MustCompile(`^paca-[a-f0-9]{16}$`)
	safeContractID = regexp.MustCompile(`^pac-[a-f0-9]{8,64}$`)
	safeFamilyID   = regexp.MustCompile(`^pacf-[a-f0-9]{8,64}$`)
	safeDigest     = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

// ProducerMetadata is the producer declaration carried by a proposal.
type ProducerMetadata struct {
	Name                  string `json:"name"`
	ArtifactSchemaVersion string `json:"artifact_schema_version"`
	ContractSchemaVersion string `json:"contract_schema_version"`
}

type VariantMetadata struct {
	ShareProfile string `json:"share_profile"`
	Redacted     bool   `json:"redacted"`
}

// Artifact is intentionally map-backed for the embedded contract. This lets
// Gait preserve Wrkr's immutable v3 contract without reimplementing Wrkr's
// risk model or dropping future additive fields.
type Artifact struct {
	SchemaID               string           `json:"schema_id"`
	SchemaVersion          string           `json:"schema_version"`
	ArtifactID             string           `json:"artifact_id"`
	ContractID             string           `json:"contract_id"`
	ContractFamilyID       string           `json:"contract_family_id"`
	Revision               int              `json:"revision"`
	Producer               ProducerMetadata `json:"producer"`
	SourceScanRefs         []string         `json:"source_scan_refs"`
	CompositionRefs        []string         `json:"composition_refs"`
	ResolutionKey          string           `json:"resolution_key,omitempty"`
	CreationEvidence       []string         `json:"creation_evidence"`
	CanonicalContentDigest string           `json:"canonical_content_digest"`
	Variant                VariantMetadata  `json:"variant"`
	ReportOnly             bool             `json:"report_only"`
	Contract               map[string]any   `json:"contract"`
}

// ValidationOptions controls time-sensitive checks. A zero Now uses the
// current UTC time; callers that need reproducibility should pass a fixed Now.
type ValidationOptions struct {
	Now                 time.Time
	RequireExplicitPath bool
	ExpectedContractID  string
	ExpectedFamilyID    string
	ExpectedRevision    int
	SchemaRoot          string
}

type SupportedConstraintSummary struct {
	TargetConstraintKeys []string `json:"target_constraint_keys"`
	PreconditionKinds    []string `json:"precondition_kinds"`
	AuthorityKinds       []string `json:"authority_kinds"`
	Unsupported          []string `json:"unsupported,omitempty"`
}

type ValidationResult struct {
	Valid                  bool                       `json:"valid"`
	Reasons                []string                   `json:"reason_codes,omitempty"`
	Artifact               *Artifact                  `json:"artifact,omitempty"`
	CanonicalContentDigest string                     `json:"canonical_content_digest,omitempty"`
	SupportedConstraints   SupportedConstraintSummary `json:"supported_constraints"`
}

// SelectionEvidence is the Gait-owned current-selection record required
// before activation or consumer handoff. It binds one explicit artifact to
// the family/revision currently selected by the caller.
type SelectionEvidence struct {
	ArtifactID             string `json:"artifact_id"`
	ArtifactSHA256         string `json:"artifact_sha256"`
	CanonicalContentDigest string `json:"canonical_content_digest"`
	ContractID             string `json:"contract_id"`
	ContractFamilyID       string `json:"contract_family_id"`
	Revision               int    `json:"revision"`
	Current                bool   `json:"current"`
}

// ValidationError is stable and machine-readable. Error() is intentionally
// compact because CLI JSON exposes Reasons directly.
type ValidationError struct{ Reasons []string }

func (e *ValidationError) Error() string {
	if e == nil || len(e.Reasons) == 0 {
		return "action contract validation failed"
	}
	return "action contract validation failed: " + strings.Join(e.Reasons, ",")
}

func addReason(reasons *[]string, reason string) {
	if strings.TrimSpace(reason) == "" {
		return
	}
	for _, existing := range *reasons {
		if existing == reason {
			return
		}
	}
	*reasons = append(*reasons, reason)
}

func sortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
		if values[i] != "" {
			seen[values[i]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := map[string]struct{}{}
				for decoder.More() {
					keyToken, err := decoder.Token()
					if err != nil {
						return err
					}
					key, ok := keyToken.(string)
					if !ok {
						return errors.New("object key is not a string")
					}
					if _, exists := seen[key]; exists {
						return fmt.Errorf("duplicate JSON object key %q", key)
					}
					seen[key] = struct{}{}
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			case '[':
				for decoder.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			default:
				return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
			}
		default:
			return nil
		}
	}
	if err := walk(); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func validateSchema(raw []byte, schemaFile, schemaRoot string) error {
	_ = schemaRoot // retained in ValidationOptions for source compatibility; assets are package-owned.
	compiler := jsonschema.NewCompiler()
	resources := map[string]string{
		ProposedSchemaID:         "proposed-action-contract-artifact.schema.json",
		ProposedContractSchemaID: "proposed-action-contract-v3.schema.json",
		ActivatedSchemaID:        "activated-action-contract-artifact.schema.json",
	}
	for uri, filename := range resources {
		payload, err := schemaAssets.ReadFile("schemaassets/" + filename)
		if err != nil {
			return err
		}
		if err := compiler.AddResource(uri, bytes.NewReader(payload)); err != nil {
			return err
		}
	}
	compiled, err := compiler.Compile(schemaFile)
	if err != nil {
		return err
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := compiled.Validate(document); err != nil {
		return err
	}
	return nil
}

// ParseArtifact parses one standalone artifact, rejects duplicate keys and
// trailing JSON, and preserves JSON numbers for JCS digest verification.
func ParseArtifact(raw []byte) (Artifact, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	for key := range object {
		if !allowedArtifactFields[key] {
			return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
		}
	}
	var artifact Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&artifact); err != nil {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	if artifact.Contract == nil {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	if rawContract, ok := object["contract"]; !ok {
		return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	} else {
		var contractObject map[string]json.RawMessage
		if err := json.Unmarshal(rawContract, &contractObject); err != nil || contractObject == nil {
			return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
		}
		for key := range contractObject {
			if !allowedContractFields[key] {
				return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
			}
		}
	}
	for _, field := range []string{"producer", "variant"} {
		rawField, ok := object[field]
		if !ok {
			continue
		}
		var fieldObject map[string]json.RawMessage
		if err := json.Unmarshal(rawField, &fieldObject); err != nil || fieldObject == nil {
			return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
		}
		allowed := map[string]bool{"name": true, "artifact_schema_version": true, "contract_schema_version": true}
		if field == "variant" {
			allowed = map[string]bool{"share_profile": true, "redacted": true}
		}
		for key := range fieldObject {
			if !allowed[key] {
				return Artifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
			}
		}
	}
	return artifact, nil
}

var allowedArtifactFields = map[string]bool{
	"schema_id": true, "schema_version": true, "artifact_id": true, "contract_id": true,
	"contract_family_id": true, "revision": true, "producer": true, "source_scan_refs": true,
	"composition_refs": true, "resolution_key": true, "creation_evidence": true,
	"canonical_content_digest": true, "variant": true, "report_only": true, "contract": true,
}

var allowedContractFields = map[string]bool{
	"contract_id": true, "contract_family_id": true, "contract_content_digest": true, "contract_version": true,
	"contract_kind": true, "composition_ref": true, "resolution_key": true, "revision": true,
	"supersedes_ref": true, "lifecycle_observations": true, "allowed_transitions": true,
	"prohibited_transitions": true, "approval_required_transitions": true, "target_constraints": true,
	"required_credential_mode": true, "maximum_delegation_depth": true, "evidence_requirements": true,
	"acceptable_countersigners": true, "expected_outcome_class": true, "compensation_required": true,
	"expires_at": true, "source_digests": true, "authority_requirements": true,
	"authority_readiness_state": true, "preconditions": true, "confirmation_requirement": true,
	"approval_requirement": true, "compensation_requirement": true, "report_only": true,
	"readiness_state": true, "reason_codes": true,
}

// ReadArtifact reads exactly one artifact from path. The path is a caller
// selection; no directory scanning or recommendation discovery is performed.
func ReadArtifact(path string) (Artifact, []byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Artifact{}, nil, &ValidationError{Reasons: []string{ReasonSelectionRequired}}
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- explicit operator-selected artifact path.
	if err != nil {
		return Artifact{}, nil, err
	}
	artifact, err := ParseArtifact(raw)
	return artifact, raw, err
}

func ParseActivatedArtifact(raw []byte) (ActivatedArtifact, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return ActivatedArtifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var artifact ActivatedArtifact
	if err := decoder.Decode(&artifact); err != nil {
		return ActivatedArtifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ActivatedArtifact{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	return artifact, nil
}

func ReadActivatedArtifact(path string) (ActivatedArtifact, []byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ActivatedArtifact{}, nil, &ValidationError{Reasons: []string{ReasonSelectionRequired}}
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- explicit operator-selected artifact path.
	if err != nil {
		return ActivatedArtifact{}, nil, err
	}
	artifact, err := ParseActivatedArtifact(raw)
	return artifact, raw, err
}

// WriteActivatedArtifact writes deterministic bytes through a same-directory
// temporary file. Existing targets are refused unless overwrite is explicit;
// symlink targets and symlinked parent directories are always rejected.
func WriteActivatedArtifact(path string, artifact ActivatedArtifact, overwrite bool) error {
	return writeActivatedArtifact(path, artifact, overwrite, nil)
}

func writeActivatedArtifact(path string, artifact ActivatedArtifact, overwrite bool, beforeInstall func()) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("activation output path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	directory := filepath.Dir(abs)
	resolvedDirectory, err := validateOutputDirectory(directory)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(resolvedDirectory, filepath.Base(abs))
	initialInfo, err := os.Lstat(targetPath)
	existed := err == nil
	var initialDigest [sha256.Size]byte
	if existed {
		info := initialInfo
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("activation output must not be a symlink")
		}
		if !info.Mode().IsRegular() {
			return errors.New("activation output must be a regular file")
		}
		if !overwrite {
			return errors.New("activation output exists; pass --overwrite to replace it")
		}
		initialDigest, err = digestFile(targetPath)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	encoded, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	temporary, err := os.CreateTemp(resolvedDirectory, ".gait-activated-action-contract-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if beforeInstall != nil {
		beforeInstall()
	}
	if existed {
		currentInfo, err := os.Lstat(targetPath)
		if err != nil {
			return fmt.Errorf("activation output changed before overwrite: %w", err)
		}
		if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.Mode().IsRegular() || !os.SameFile(initialInfo, currentInfo) {
			return errors.New("activation output changed before overwrite")
		}
		currentDigest, err := digestFile(targetPath)
		if err != nil {
			return fmt.Errorf("activation output changed before overwrite: %w", err)
		}
		if currentDigest != initialDigest {
			return errors.New("activation output changed before overwrite")
		}
		// #nosec G703 -- temporaryPath is created inside the validated output directory and targetPath is the checked regular file in that same directory.
		if err := os.Rename(temporaryPath, targetPath); err != nil {
			return err
		}
		return nil
	}
	// A hard link is the portable exclusive-install primitive: it succeeds
	// only when the target is still absent and never follows a symlink.
	if err := os.Link(temporaryPath, targetPath); err != nil {
		if os.IsExist(err) {
			return errors.New("activation output appeared during install")
		}
		return err
	}
	// #nosec G703 -- temporaryPath is the file just created by os.CreateTemp in the validated output directory.
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	return nil
}

func digestFile(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path) // #nosec G304 -- path is derived from the validated activation output directory.
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}

func validateOutputDirectory(directory string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return "", err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("activation output directory must not contain symlinks")
		}
		if !info.IsDir() {
			return "", errors.New("activation output directory must be a directory")
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func ValidateArtifactBytes(raw []byte, options ValidationOptions) (Artifact, ValidationResult) {
	artifact, err := ParseArtifact(raw)
	if err != nil {
		return Artifact{}, ValidationResult{Valid: false, Reasons: []string{ReasonMalformedArtifact}}
	}
	if err := validateSchema(raw, ProposedSchemaID, options.SchemaRoot); err != nil {
		return artifact, ValidationResult{Valid: false, Artifact: &artifact, Reasons: []string{ReasonSchemaValidationFailed}}
	}
	result := ValidateArtifact(artifact, options)
	return artifact, result
}

type selectionManifest struct {
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
		ScenarioID             string `json:"scenario_id"`
		ArtifactPath           string `json:"artifact_path"`
		ArtifactSHA256         string `json:"artifact_sha256"`
		ArtifactID             string `json:"artifact_id"`
		CanonicalContentDigest string `json:"canonical_content_digest"`
		ContractID             string `json:"contract_id"`
		ContractFamilyID       string `json:"contract_family_id"`
		Revision               int    `json:"revision"`
		Current                bool   `json:"current"`
	} `json:"scenarios"`
}

func validateSelectionEvidence(selection SelectionEvidence, artifact Artifact, raw []byte) error {
	if !selection.Current {
		return &ValidationError{Reasons: []string{ReasonSelectionNotCurrent}}
	}
	if selection.ArtifactID != artifact.ArtifactID || selection.ArtifactSHA256 != RawDigest(raw) || selection.CanonicalContentDigest != artifact.CanonicalContentDigest || selection.ContractID != artifact.ContractID || selection.ContractFamilyID != artifact.ContractFamilyID || selection.Revision != artifact.Revision {
		return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	return nil
}

func LoadSelectionEvidence(path, artifactPath string, artifact Artifact, raw []byte) (SelectionEvidence, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionEvidenceRequired}}
	}
	selectionRaw, err := os.ReadFile(path) // #nosec G304 -- explicit operator-selected selection evidence.
	if err != nil {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionEvidenceRequired}}
	}
	if err := rejectDuplicateJSONKeys(selectionRaw); err != nil {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	var manifest selectionManifest
	if err := json.Unmarshal(selectionRaw, &manifest); err != nil {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonMalformedArtifact}}
	}
	if manifest.FixtureVersion != "1" || manifest.Producer.Name != ProposedProducer || manifest.Producer.Version != "v1.14.0" || manifest.Schemas.Artifact != ProposedSchemaVersion || manifest.Schemas.Contract != ProposedContractVersion {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	selectionAbs, err := filepath.Abs(path)
	if err != nil {
		return SelectionEvidence{}, err
	}
	if selectionInfo, statErr := os.Lstat(selectionAbs); statErr != nil || selectionInfo.Mode()&os.ModeSymlink != 0 || !selectionInfo.Mode().IsRegular() {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	selectionRoot := filepath.Dir(selectionAbs)
	if rootInfo, statErr := os.Lstat(selectionRoot); statErr != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	selectionRootResolved, err := filepath.EvalSymlinks(selectionRoot)
	if err != nil {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	artifactAbs, err := filepath.Abs(artifactPath)
	if err != nil {
		return SelectionEvidence{}, err
	}
	artifactResolved, err := filepath.EvalSymlinks(artifactAbs)
	if err != nil {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	var selected SelectionEvidence
	found := 0
	maxRevisionByFamily := map[string]int{}
	for _, scenario := range manifest.Scenarios {
		if scenario.Revision > maxRevisionByFamily[scenario.ContractFamilyID] {
			maxRevisionByFamily[scenario.ContractFamilyID] = scenario.Revision
		}
		candidate := filepath.Clean(filepath.Join(selectionRoot, filepath.FromSlash(scenario.ArtifactPath)))
		relative, err := filepath.Rel(selectionRoot, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
		}
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil || candidateAbs != artifactAbs {
			continue
		}
		if err := rejectSelectionSymlinkPath(selectionRoot, candidateAbs); err != nil {
			return SelectionEvidence{}, err
		}
		candidateResolved, err := filepath.EvalSymlinks(candidateAbs)
		if err != nil || candidateResolved != artifactResolved {
			return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
		}
		if !selectionPathWithin(selectionRootResolved, candidateResolved) {
			return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
		}
		found++
		selected = SelectionEvidence{ArtifactID: scenario.ArtifactID, ArtifactSHA256: scenario.ArtifactSHA256, CanonicalContentDigest: scenario.CanonicalContentDigest, ContractID: scenario.ContractID, ContractFamilyID: scenario.ContractFamilyID, Revision: scenario.Revision, Current: scenario.Current}
	}
	if found == 0 {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionEvidenceRequired}}
	}
	if found > 1 {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionAmbiguous}}
	}
	if maxRevisionByFamily[selected.ContractFamilyID] > selected.Revision {
		return SelectionEvidence{}, &ValidationError{Reasons: []string{ReasonSelectionNotCurrent}}
	}
	if err := validateSelectionEvidence(selected, artifact, raw); err != nil {
		return SelectionEvidence{}, err
	}
	return selected, nil
}

func selectionPathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rejectSelectionSymlinkPath(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil || !selectionPathWithin(rootAbs, targetAbs) {
		return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
	}
	current := rootAbs
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &ValidationError{Reasons: []string{ReasonSelectionMismatch}}
		}
	}
	return nil
}

// ValidateArtifact validates an explicit Wrkr v3 proposal and its JCS
// envelope digest. It does not activate or infer authority.
func ValidateArtifact(artifact Artifact, options ValidationOptions) ValidationResult {
	result := ValidationResult{Artifact: &artifact}
	add := func(reason string) { addReason(&result.Reasons, reason) }
	if encoded, err := json.Marshal(artifact); err != nil {
		add(ReasonSchemaValidationFailed)
	} else if err := validateSchema(encoded, ProposedSchemaID, options.SchemaRoot); err != nil {
		add(ReasonSchemaValidationFailed)
	}
	if artifact.SchemaID != ProposedSchemaID || artifact.SchemaVersion != ProposedSchemaVersion {
		add(ReasonUnsupportedArtifactSchema)
	}
	if artifact.Producer.Name != ProposedProducer || artifact.Producer.ArtifactSchemaVersion != ProposedSchemaVersion {
		add(ReasonUnsupportedProducer)
	}
	if artifact.Producer.ContractSchemaVersion != ProposedContractVersion {
		add(ReasonUnsupportedContractSchema)
	}
	if !artifact.ReportOnly {
		add(ReasonReportOnlyRequired)
	}
	if !safeArtifactID.MatchString(strings.TrimSpace(artifact.ArtifactID)) {
		add(ReasonArtifactIdentityMismatch)
	}
	if !safeContractID.MatchString(strings.TrimSpace(artifact.ContractID)) {
		add(ReasonMissingContractID)
	}
	if !safeFamilyID.MatchString(strings.TrimSpace(artifact.ContractFamilyID)) {
		add(ReasonMissingFamilyID)
	}
	if artifact.Revision < 1 {
		add(ReasonRevisionInvalid)
	}
	if len(nonEmpty(artifact.SourceScanRefs)) == 0 {
		add(ReasonMissingSourceRefs)
	}
	if len(nonEmpty(artifact.CompositionRefs)) == 0 {
		add(ReasonMissingCompositionRef)
	}
	if len(nonEmpty(artifact.CreationEvidence)) == 0 {
		add(ReasonMissingEvidenceRefs)
	}
	if artifact.Contract == nil {
		add(ReasonMalformedArtifact)
		result.Reasons = sortedStrings(result.Reasons)
		return finishValidation(result)
	}

	contractID := stringField(artifact.Contract, "contract_id")
	familyID := stringField(artifact.Contract, "contract_family_id")
	contractVersion := stringField(artifact.Contract, "contract_version")
	if contractID != artifact.ContractID || familyID != artifact.ContractFamilyID || intField(artifact.Contract, "revision") != artifact.Revision {
		add(ReasonRevisionIdentityMismatch)
	}
	if contractID == "" || familyID == "" {
		add(ReasonContractIdentityMismatch)
	}
	if contractVersion != ProposedContractVersion || stringField(artifact.Contract, "contract_kind") != "proposed_action_contract" {
		add(ReasonUnsupportedContractSchema)
	}
	if !boolField(artifact.Contract, "report_only") || !artifact.ReportOnly {
		add(ReasonReportOnlyRequired)
	}
	if stringField(artifact.Contract, "composition_ref") == "" {
		add(ReasonMissingCompositionRef)
	} else if !containsString(artifact.CompositionRefs, stringField(artifact.Contract, "composition_ref")) {
		add(ReasonContractIdentityMismatch)
	}
	if topResolution := strings.TrimSpace(artifact.ResolutionKey); topResolution != "" && stringField(artifact.Contract, "resolution_key") != "" && topResolution != stringField(artifact.Contract, "resolution_key") {
		add(ReasonContractIdentityMismatch)
	}
	if artifact.Revision > 1 && stringField(artifact.Contract, "supersedes_ref") == "" {
		add(ReasonRevisionReactivationRequired)
	}

	result.SupportedConstraints = summarizeConstraints(artifact.Contract)
	for _, unsupported := range result.SupportedConstraints.Unsupported {
		add(ReasonUnsupportedConstraint + ":" + unsupported)
	}

	if digest, err := canonicalContentDigest(artifact); err != nil {
		add(ReasonMalformedArtifact)
	} else {
		result.CanonicalContentDigest = digest
		if digest != strings.TrimSpace(artifact.CanonicalContentDigest) {
			add(ReasonDigestMismatch)
		}
	}
	if embedded := stringField(artifact.Contract, "contract_content_digest"); embedded != "" && !safeDigest.MatchString(embedded) {
		add(ReasonContractDigestMismatch)
	} else if embedded != "" && proposedContractContentDigest(artifact.Contract) != embedded {
		add(ReasonContractDigestMismatch)
	}
	if familyID != "" && proposedContractFamilyID(artifact.Contract) != familyID {
		add(ReasonContractIdentityMismatch)
	}
	if contractID != "" && proposedContractID(artifact.Contract) != contractID {
		add(ReasonContractIdentityMismatch)
	}
	if options.ExpectedContractID != "" && strings.TrimSpace(options.ExpectedContractID) != artifact.ContractID {
		add(ReasonContractIdentityMismatch)
	}
	if options.ExpectedFamilyID != "" && strings.TrimSpace(options.ExpectedFamilyID) != artifact.ContractFamilyID {
		add(ReasonContractIdentityMismatch)
	}
	if options.ExpectedRevision > 0 && options.ExpectedRevision != artifact.Revision {
		add(ReasonRevisionIdentityMismatch)
	}

	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, observation := range objectArray(artifact.Contract, "lifecycle_observations") {
		freshness := stringField(observation, "freshness_state")
		if freshness == "stale" || freshness == "expired" {
			add(ReasonStaleProposal)
		}
		if stringField(observation, "kind") == "supersession" && stringField(observation, "evidence_state") == "contradictory" {
			add(ReasonSupersededProposal)
		}
	}
	if expires := stringField(artifact.Contract, "expires_at"); expires != "" {
		if parsed, err := time.Parse(time.RFC3339, expires); err != nil {
			add(ReasonMalformedArtifact)
		} else if parsed.Before(now) {
			add(ReasonStaleProposal)
		}
	}
	if stringField(artifact.Contract, "readiness_state") == "blocked_by_contradiction" || stringField(artifact.Contract, "authority_readiness_state") == "blocked_by_contradiction" {
		add(ReasonContradictoryProposal)
	}
	result.Reasons = sortedStrings(result.Reasons)
	return finishValidation(result)
}

func finishValidation(result ValidationResult) ValidationResult {
	result.Valid = len(result.Reasons) == 0
	return result
}

func canonicalContentDigest(artifact Artifact) (string, error) {
	// The Wrkr envelope's canonical content domain excludes only the derived
	// artifact ID and digest. Every immutable proposal/ref/variant field stays
	// in the digest projection.
	payload := map[string]any{
		"schema_id": artifact.SchemaID, "schema_version": artifact.SchemaVersion,
		"contract_id": artifact.ContractID, "contract_family_id": artifact.ContractFamilyID,
		"revision": artifact.Revision, "producer": artifact.Producer,
		"source_scan_refs": artifact.SourceScanRefs, "composition_refs": artifact.CompositionRefs,
		"creation_evidence": artifact.CreationEvidence, "variant": artifact.Variant,
		"report_only": artifact.ReportOnly, "contract": artifact.Contract,
	}
	if strings.TrimSpace(artifact.ResolutionKey) != "" {
		payload["resolution_key"] = artifact.ResolutionKey
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(encoded)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	return digest, nil
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(wanted) {
			return true
		}
	}
	return false
}

var supportedConstraintKeys = map[string]bool{"composition_id": true, "environment": true, "outcome_class": true, "target_class": true, "target_identity": true}
var supportedPreconditionKinds = map[string]bool{"validation_contract": true, "effect_contract": true, "required_check": true, "producer": true, "freshness": true, "environment": true, "target": true, "sandbox": true, "policy_digest": true, "credential_mode": true, "expected_effect": true, "forbidden_effect": true}
var supportedAuthorityKinds = map[string]bool{"originating_intent": true, "requester_identity": true, "business_owner": true, "affected_system_owner": true, "permitted_agent_role": true, "policy_authority": true, "delegation_root": true, "credential_subject_constraint": true, "separation_of_duties": true}

func summarizeConstraints(contract map[string]any) SupportedConstraintSummary {
	result := SupportedConstraintSummary{}
	for _, item := range objectArray(contract, "target_constraints") {
		key := stringField(item, "key")
		if supportedConstraintKeys[key] {
			result.TargetConstraintKeys = append(result.TargetConstraintKeys, key)
		} else if key != "" {
			result.Unsupported = append(result.Unsupported, "target:"+key)
		}
	}
	for _, item := range objectArray(contract, "preconditions") {
		kind := stringField(item, "kind")
		if supportedPreconditionKinds[kind] {
			result.PreconditionKinds = append(result.PreconditionKinds, kind)
		} else if kind != "" {
			result.Unsupported = append(result.Unsupported, "precondition:"+kind)
		}
	}
	for _, item := range objectArray(contract, "authority_requirements") {
		kind := stringField(item, "kind")
		if supportedAuthorityKinds[kind] {
			result.AuthorityKinds = append(result.AuthorityKinds, kind)
		} else if kind != "" {
			result.Unsupported = append(result.Unsupported, "authority:"+kind)
		}
	}
	result.TargetConstraintKeys = sortedStrings(result.TargetConstraintKeys)
	result.PreconditionKinds = sortedStrings(result.PreconditionKinds)
	result.AuthorityKinds = sortedStrings(result.AuthorityKinds)
	result.Unsupported = sortedStrings(result.Unsupported)
	return result
}

// The following projections mirror Wrkr v3's immutable identity rules. Proof
// remains the owner of canonicalization primitives; this consumer only checks
// the product identity fields emitted by Wrkr.
func proposedContractFamilyID(contract map[string]any) string {
	constraints := make([]string, 0)
	for _, item := range objectArray(contract, "target_constraints") {
		constraints = append(constraints, stringField(item, "key")+"="+stringField(item, "value"))
	}
	raw := strings.Join([]string{
		stringField(contract, "composition_ref"),
		stringField(contract, "required_credential_mode") + "|" + strconv.Itoa(intField(contract, "maximum_delegation_depth")) + "|" + stringField(contract, "expected_outcome_class") + "|" + stringField(contract, "resolution_key"),
		strings.Join(constraints, "|"),
	}, "\x1f")
	return "pacf-" + shortHash(raw)
}

func proposedContractID(contract map[string]any) string {
	digest := proposedContractContentDigest(contract)
	if digest == "" {
		return ""
	}
	return "pac-" + shortHash(proposedContractFamilyID(contract)+"|"+digest+"|"+stringField(contract, "contract_version"))
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:8])
}

func proposedContractContentDigest(contract map[string]any) string {
	parts := []string{
		"version=" + stringField(contract, "contract_version"), "kind=" + stringField(contract, "contract_kind"),
		"composition=" + stringField(contract, "composition_ref"), "resolution=" + stringField(contract, "resolution_key"),
		"credential_mode=" + stringField(contract, "required_credential_mode"), "delegation_depth=" + strconv.Itoa(intField(contract, "maximum_delegation_depth")),
		"outcome=" + stringField(contract, "expected_outcome_class"), "compensation=" + strconv.FormatBool(boolField(contract, "compensation_required")),
		"expires_at=" + stringField(contract, "expires_at"), "report_only=" + strconv.FormatBool(boolField(contract, "report_only")),
		"readiness=" + stringField(contract, "readiness_state"), "revision=" + strconv.Itoa(intField(contract, "revision")),
		"supersedes=" + stringField(contract, "supersedes_ref"), "authority_readiness=" + stringField(contract, "authority_readiness_state"),
	}
	for _, entry := range []struct{ key, prefix string }{{"allowed_transitions", "allow="}, {"prohibited_transitions", "prohibit="}, {"approval_required_transitions", "approval="}} {
		for _, item := range objectArray(contract, entry.key) {
			parts = append(parts, entry.prefix+transitionKey(item))
		}
	}
	for _, item := range objectArray(contract, "target_constraints") {
		parts = append(parts, "target="+stringField(item, "key")+"="+stringField(item, "value"))
	}
	for _, value := range stringArray(contract, "evidence_requirements") {
		parts = append(parts, "evidence="+value)
	}
	for _, value := range stringArray(contract, "acceptable_countersigners") {
		parts = append(parts, "countersigner="+value)
	}
	for _, value := range stringArray(contract, "source_digests") {
		parts = append(parts, "digest="+value)
	}
	for _, value := range stringArray(contract, "reason_codes") {
		parts = append(parts, "reason="+value)
	}
	for _, item := range objectArray(contract, "authority_requirements") {
		parts = append(parts, "authority="+requirementKey(item))
	}
	for _, item := range objectArray(contract, "preconditions") {
		parts = append(parts, "precondition="+preconditionKey(item))
	}
	if item := firstObject(contract, "confirmation_requirement"); item != nil {
		parts = append(parts, "confirmation="+strings.Join([]string{stringField(item, "mode"), strconv.FormatBool(boolField(item, "required")), stringField(item, "evidence_state"), stringField(item, "freshness_state"), strings.Join(stringArray(item, "evidence_refs"), ","), strings.Join(stringArray(item, "reason_codes"), ",")}, "|"))
	}
	if item := firstObject(contract, "approval_requirement"); item != nil {
		parts = append(parts, "approval="+strings.Join([]string{strconv.FormatBool(boolField(item, "required")), strings.Join(stringArray(item, "approver_roles"), ","), strconv.Itoa(intField(item, "minimum_approvals")), strings.Join(stringArray(item, "separation_of_duties"), ","), stringField(item, "scope_digest"), stringField(item, "validity_window"), strings.Join(stringArray(item, "reapproval_triggers"), ","), stringField(item, "evidence_state"), stringField(item, "freshness_state"), strings.Join(stringArray(item, "evidence_refs"), ","), strings.Join(stringArray(item, "reason_codes"), ",")}, "|"))
	}
	if item := firstObject(contract, "compensation_requirement"); item != nil {
		parts = append(parts, "compensation="+strings.Join([]string{strconv.FormatBool(boolField(item, "required")), stringField(item, "kind"), stringField(item, "procedure_ref"), stringField(item, "target"), stringField(item, "execution_window"), strconv.FormatBool(boolField(item, "verification_required")), strings.Join(stringArray(item, "acceptable_producers"), ","), stringField(item, "evidence_state"), stringField(item, "freshness_state"), strings.Join(stringArray(item, "evidence_refs"), ","), strings.Join(stringArray(item, "reason_codes"), ",")}, "|"))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func transitionKey(item map[string]any) string {
	return strings.Join([]string{stringField(item, "transition_id"), stringField(item, "from_stage_id"), stringField(item, "to_stage_id"), stringField(item, "from_role"), stringField(item, "to_role"), stringField(item, "reason")}, "|")
}
func requirementKey(item map[string]any) string {
	return strings.Join([]string{stringField(item, "requirement_id"), stringField(item, "kind"), stringField(item, "required_constraint"), stringField(item, "observed_value"), stringField(item, "evidence_state"), stringField(item, "freshness_state"), strings.Join(stringArray(item, "evidence_refs"), ","), strings.Join(stringArray(item, "reason_codes"), ",")}, "|")
}
func preconditionKey(item map[string]any) string {
	return strings.Join([]string{stringField(item, "requirement_id"), stringField(item, "kind"), stringField(item, "required_constraint"), stringField(item, "observed_value"), stringField(item, "observed_result"), strings.Join(stringArray(item, "acceptable_producers"), ","), stringField(item, "max_age"), stringField(item, "evidence_state"), stringField(item, "freshness_state"), strings.Join(stringArray(item, "evidence_refs"), ","), strings.Join(stringArray(item, "reason_codes"), ",")}, "|")
}
func stringArray(object map[string]any, key string) []string {
	raw, _ := object[key].([]any)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}
func firstObject(object map[string]any, key string) map[string]any {
	item, _ := object[key].(map[string]any)
	return item
}

func stringField(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return strings.TrimSpace(value)
}
func boolField(object map[string]any, key string) bool { value, _ := object[key].(bool); return value }
func intField(object map[string]any, key string) int {
	switch value := object[key].(type) {
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case int:
		return value
	}
	return 0
}
func objectArray(object map[string]any, key string) []map[string]any {
	raw, ok := object[key].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(raw))
	for _, value := range raw {
		if item, ok := value.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

type ActivationMode string

const (
	ActivationContextOnly  ActivationMode = "context_only"
	ActivationEnforceFloor ActivationMode = "enforce_floor"
	ActivationRequired     ActivationMode = "required"
)

type ActivationOptions struct {
	PolicyDigest            string
	ActivatingPrincipal     string
	AuthorityRefs           []string
	Target                  string
	Environment             string
	Mode                    ActivationMode
	ValidFrom               string
	ValidUntil              string
	ExplicitExceptions      []string
	SigningPrivateKey       ed25519.PrivateKey
	AllowDevelopmentSigning bool
	Selection               *SelectionEvidence
	EvaluationTime          time.Time
}

type ActivationProposalRef struct {
	ArtifactID             string `json:"artifact_id"`
	CanonicalContentDigest string `json:"canonical_content_digest"`
	ContractID             string `json:"contract_id"`
	ContractFamilyID       string `json:"contract_family_id"`
	Revision               int    `json:"revision"`
	SchemaID               string `json:"schema_id"`
	SchemaVersion          string `json:"schema_version"`
	ContractSchemaVersion  string `json:"contract_schema_version"`
}

type Validity struct {
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after,omitempty"`
}

type ActivatedArtifact struct {
	SchemaID            string                `json:"schema_id"`
	SchemaVersion       string                `json:"schema_version"`
	ArtifactID          string                `json:"artifact_id"`
	ContractID          string                `json:"contract_id"`
	ContractFamilyID    string                `json:"contract_family_id"`
	Revision            int                   `json:"revision"`
	Producer            ProducerMetadata      `json:"producer"`
	Proposal            ActivationProposalRef `json:"proposal"`
	PolicyDigest        string                `json:"policy_digest"`
	ActivatingPrincipal string                `json:"activating_principal"`
	AuthorityRefs       []string              `json:"authority_refs"`
	Target              string                `json:"target"`
	Environment         string                `json:"environment"`
	ActivationMode      ActivationMode        `json:"activation_mode"`
	Validity            Validity              `json:"validity"`
	ExplicitExceptions  []string              `json:"explicit_exceptions"`
	ReportOnly          bool                  `json:"report_only"`
	DevelopmentSigning  bool                  `json:"development_signing"`
	Signature           proofsign.Signature   `json:"signature"`
}

type ActivationResult struct {
	Activated  ActivatedArtifact `json:"activated"`
	Validation ValidationResult  `json:"validation"`
}

type VerificationOptions struct {
	AllowDevelopmentSigning bool
	Proposal                *Artifact
	// EvaluationTime controls activation and proposal expiry checks. A zero
	// value preserves the library's historical current-time behavior; CLI and
	// deterministic callers should pass an explicit UTC time.
	EvaluationTime time.Time
}

// Activate validates one proposal and emits a deterministic signed activation
// object. No approval, authority, execution, or effect state is generated.
func Activate(artifact Artifact, options ActivationOptions) (ActivatedArtifact, ValidationResult, error) {
	validation := ValidateArtifact(artifact, ValidationOptions{Now: options.EvaluationTime})
	reasons := append([]string(nil), validation.Reasons...)
	add := func(reason string) { addReason(&reasons, reason) }
	if options.Selection == nil {
		add(ReasonSelectionEvidenceRequired)
	} else if options.Selection.ArtifactID != artifact.ArtifactID || options.Selection.CanonicalContentDigest != artifact.CanonicalContentDigest || options.Selection.ContractID != artifact.ContractID || options.Selection.ContractFamilyID != artifact.ContractFamilyID || options.Selection.Revision != artifact.Revision || !options.Selection.Current {
		add(ReasonSelectionMismatch)
		if !options.Selection.Current {
			add(ReasonSelectionNotCurrent)
		}
	}
	switch options.Mode {
	case ActivationContextOnly, ActivationEnforceFloor, ActivationRequired:
	default:
		add(ReasonActivationModeUnsupported)
	}
	if !safeDigest.MatchString(strings.TrimSpace(options.PolicyDigest)) {
		add(ReasonPolicyDigestMissing)
	}
	if strings.TrimSpace(options.ActivatingPrincipal) == "" {
		add(ReasonPrincipalMissing)
	}
	if len(nonEmpty(options.AuthorityRefs)) == 0 {
		add(ReasonAuthorityRefsMissing)
	}
	if strings.TrimSpace(options.Target) == "" {
		add(ReasonTargetMissing)
	}
	if strings.TrimSpace(options.Environment) == "" {
		add(ReasonEnvironmentMissing)
	}
	if options.AllowDevelopmentSigning && options.Environment != "development" && options.Environment != "test" {
		add(ReasonDevelopmentSigningForbidden)
	}
	validity := Validity{NotBefore: strings.TrimSpace(options.ValidFrom), NotAfter: strings.TrimSpace(options.ValidUntil)}
	var notBefore, notAfter time.Time
	if validity.NotBefore == "" {
		add(ReasonValidityInvalid)
	} else {
		parsed, err := time.Parse(time.RFC3339, validity.NotBefore)
		if err != nil {
			add(ReasonValidityInvalid)
		} else {
			notBefore = parsed
			validity.NotBefore = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if validity.NotAfter != "" {
		parsed, err := time.Parse(time.RFC3339, validity.NotAfter)
		if err != nil {
			add(ReasonValidityInvalid)
		} else {
			notAfter = parsed
			validity.NotAfter = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if !notBefore.IsZero() && !notAfter.IsZero() && !notAfter.After(notBefore) {
		add(ReasonValidityInvalid)
	}
	if options.Mode != ActivationContextOnly && len(validation.Reasons) > 0 {
		add(ReasonAuthorizationRequired)
	}
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return ActivatedArtifact{}, validation, &ValidationError{Reasons: reasons}
	}

	privateKey := options.SigningPrivateKey
	developmentSigning := false
	if len(privateKey) == 0 {
		if !options.AllowDevelopmentSigning {
			return ActivatedArtifact{}, validation, &ValidationError{Reasons: []string{ReasonSigningKeyRequired}}
		}
		privateKey = deterministicDevelopmentKey()
		developmentSigning = true
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return ActivatedArtifact{}, validation, fmt.Errorf("signing private key must be %d bytes", ed25519.PrivateKeySize)
	}
	activated := ActivatedArtifact{SchemaID: ActivatedSchemaID, SchemaVersion: ActivatedSchemaVersion, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID, Revision: artifact.Revision, Producer: ProducerMetadata{Name: ActivatedProducer, ArtifactSchemaVersion: ActivatedSchemaVersion, ContractSchemaVersion: ActivatedContractVersion}, Proposal: ActivationProposalRef{ArtifactID: artifact.ArtifactID, CanonicalContentDigest: artifact.CanonicalContentDigest, ContractID: artifact.ContractID, ContractFamilyID: artifact.ContractFamilyID, Revision: artifact.Revision, SchemaID: artifact.SchemaID, SchemaVersion: artifact.SchemaVersion, ContractSchemaVersion: artifact.Producer.ContractSchemaVersion}, PolicyDigest: strings.TrimSpace(options.PolicyDigest), ActivatingPrincipal: strings.TrimSpace(options.ActivatingPrincipal), AuthorityRefs: sortedStrings(append([]string(nil), options.AuthorityRefs...)), Target: strings.TrimSpace(options.Target), Environment: strings.TrimSpace(options.Environment), ActivationMode: options.Mode, Validity: validity, ExplicitExceptions: sortedStrings(append([]string(nil), options.ExplicitExceptions...)), ReportOnly: false, DevelopmentSigning: developmentSigning}
	digest, err := activatedSignableDigest(activated)
	if err != nil {
		return ActivatedArtifact{}, validation, err
	}
	signature, err := proofsign.SignDigestHex(privateKey, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return ActivatedArtifact{}, validation, err
	}
	activated.Signature = signature
	activated.ArtifactID = "gact-" + strings.TrimPrefix(digest, "sha256:")[:16]
	if encoded, err := json.Marshal(activated); err != nil {
		return ActivatedArtifact{}, validation, err
	} else if err := validateSchema(encoded, ActivatedSchemaID, ""); err != nil {
		return ActivatedArtifact{}, validation, fmt.Errorf("activated artifact schema validation: %w", err)
	}
	return activated, validation, nil
}

func activatedSignableDigest(artifact ActivatedArtifact) (string, error) {
	payload := artifact
	payload.ArtifactID = ""
	payload.Signature = proofsign.Signature{}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return proofcanon.DigestJCS(encoded)
}

func deterministicDevelopmentKey() ed25519.PrivateKey {
	sum := sha256.Sum256([]byte("gait-action-contract-activation-development-key-v1"))
	return ed25519.NewKeyFromSeed(sum[:])
}

// VerifyActivation checks the signed object against a supplied public key and
// the actual bound proposal. A signature alone is not a full activation
// verification result.
func VerifyActivation(artifact ActivatedArtifact, publicKey ed25519.PublicKey, proposal Artifact) (bool, error) {
	return VerifyActivationWithOptions(artifact, publicKey, VerificationOptions{Proposal: &proposal})
}

func VerifyActivationWithOptions(artifact ActivatedArtifact, publicKey ed25519.PublicKey, options VerificationOptions) (bool, error) {
	if options.Proposal == nil {
		return false, &ValidationError{Reasons: []string{ReasonBindingMismatch}}
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return false, &ValidationError{Reasons: []string{ReasonSigningKeyRequired}}
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return false, err
	}
	if err := validateSchema(encoded, ActivatedSchemaID, ""); err != nil {
		return false, &ValidationError{Reasons: []string{ReasonSchemaValidationFailed}}
	}
	if artifact.SchemaID != ActivatedSchemaID || artifact.SchemaVersion != ActivatedSchemaVersion || artifact.Producer.Name != ActivatedProducer {
		return false, &ValidationError{Reasons: []string{ReasonUnsupportedArtifactSchema}}
	}
	if artifact.ReportOnly {
		return false, &ValidationError{Reasons: []string{ReasonReportOnlyRequired}}
	}
	if artifact.DevelopmentSigning && !options.AllowDevelopmentSigning {
		return false, &ValidationError{Reasons: []string{ReasonDevelopmentSigningUnverified}}
	}
	if artifact.Proposal.SchemaID != ProposedSchemaID || artifact.Proposal.SchemaVersion != ProposedSchemaVersion || artifact.Proposal.ContractSchemaVersion != ProposedContractVersion || artifact.Proposal.ArtifactID == "" || artifact.Proposal.CanonicalContentDigest == "" || artifact.Proposal.ContractID != artifact.ContractID || artifact.Proposal.ContractFamilyID != artifact.ContractFamilyID || artifact.Proposal.Revision != artifact.Revision {
		return false, &ValidationError{Reasons: []string{ReasonBindingMismatch}}
	}
	if validation := ValidateArtifact(*options.Proposal, ValidationOptions{Now: options.EvaluationTime}); !validation.Valid {
		return false, &ValidationError{Reasons: []string{ReasonBindingMismatch}}
	}
	if artifact.Proposal.ArtifactID != options.Proposal.ArtifactID || artifact.Proposal.CanonicalContentDigest != options.Proposal.CanonicalContentDigest || artifact.Proposal.ContractID != options.Proposal.ContractID || artifact.Proposal.ContractFamilyID != options.Proposal.ContractFamilyID || artifact.Proposal.Revision != options.Proposal.Revision || artifact.Proposal.ContractSchemaVersion != options.Proposal.Producer.ContractSchemaVersion {
		return false, &ValidationError{Reasons: []string{ReasonBindingMismatch}}
	}
	evaluationTime := options.EvaluationTime
	if evaluationTime.IsZero() {
		evaluationTime = time.Now().UTC()
	}
	notBefore, parseErr := time.Parse(time.RFC3339, artifact.Validity.NotBefore)
	if parseErr != nil {
		return false, &ValidationError{Reasons: []string{ReasonValidityInvalid}}
	}
	if evaluationTime.Before(notBefore) {
		return false, &ValidationError{Reasons: []string{ReasonActivationNotYetValid}}
	}
	if strings.TrimSpace(artifact.Validity.NotAfter) != "" {
		notAfter, parseErr := time.Parse(time.RFC3339, artifact.Validity.NotAfter)
		if parseErr != nil {
			return false, &ValidationError{Reasons: []string{ReasonValidityInvalid}}
		}
		if !evaluationTime.Before(notAfter) {
			return false, &ValidationError{Reasons: []string{ReasonActivationExpired}}
		}
	}
	digest, err := activatedSignableDigest(artifact)
	if err != nil {
		return false, err
	}
	if artifact.Signature.SignedDigest != strings.TrimPrefix(digest, "sha256:") {
		return false, &ValidationError{Reasons: []string{ReasonDigestMismatch}}
	}
	valid, err := proofsign.VerifyDigestHex(publicKey, artifact.Signature)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, &ValidationError{Reasons: []string{ReasonDigestMismatch}}
	}
	wantID := "gact-" + strings.TrimPrefix(digest, "sha256:")[:16]
	if artifact.ArtifactID != wantID {
		return false, &ValidationError{Reasons: []string{ReasonContractIdentityMismatch}}
	}
	return true, nil
}

// RawDigest returns the byte SHA-256 used by conformance receipts.
func RawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// DevelopmentPublicKey exposes the deterministic dev key's public half for
// local verification tests; production callers should use their configured key.
func DevelopmentPublicKey() ed25519.PublicKey {
	return deterministicDevelopmentKey().Public().(ed25519.PublicKey)
}

// EncodePrivateKey is a small helper for test/CLI fixtures and uses Proof's
// base64-compatible representation without making key material part of an
// artifact.
func EncodePrivateKey(privateKey ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(privateKey)
}
