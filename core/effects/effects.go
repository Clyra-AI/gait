// Package effects models bounded, reference-first effect evidence and grades
// typed effect contracts without executing or mutating external systems.
package effects

import (
	"bytes"
	"crypto/ed25519"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// schemaAssets is package-owned and immutable at runtime.
//
//go:embed schemaassets/*.json
var schemaAssets embed.FS

const (
	SnapshotSchemaID = "https://gait.dev/schemas/v1/effects/effect-snapshot.schema.json"
	ContractSchemaID = "https://gait.dev/schemas/v1/effects/effect-contract.schema.json"
	GradeSchemaID    = "https://gait.dev/schemas/v1/effects/effect-grading-result.schema.json"
	SchemaVersion    = "1.0.0"
)

const (
	ResourcePostgres   = "postgres"
	ResourceFilesystem = "filesystem"
	ResourceHTTP       = "http"
	ResourceGeneric    = "resource"

	ObservationPresent = "present"
	ObservationAbsent  = "absent"
	ObservationUnknown = "unknown"

	CompletenessComplete = "complete"
	CompletenessPartial  = "partial"
	CompletenessUnknown  = "unknown"

	EnforcementVerified     = "verified"
	EnforcementObservedOnly = "observed_only"
	EnforcementPartial      = "partial"
	EnforcementUnknown      = "unknown"

	GradePass         = "pass"
	GradeFail         = "fail"
	GradeInconclusive = "inconclusive"

	PredicateExpect    = "expect"
	PredicateForbid    = "forbid"
	PredicateInvariant = "invariant"
)

const (
	ReasonSchemaUnsupported      = "effect_schema_unsupported"
	ReasonSnapshotIDMissing      = "effect_snapshot_id_missing"
	ReasonResourceKindInvalid    = "effect_resource_kind_invalid"
	ReasonSelectorMissing        = "effect_selector_missing"
	ReasonObservationInvalid     = "effect_observation_invalid"
	ReasonDigestInvalid          = "effect_digest_invalid"
	ReasonDigestMismatch         = "effect_digest_mismatch"
	ReasonCollectorMissing       = "effect_collector_missing"
	ReasonCaptureInvalid         = "effect_capture_invalid"
	ReasonRedactionInvalid       = "effect_redaction_invalid"
	ReasonCompletenessInvalid    = "effect_completeness_invalid"
	ReasonEnforcementInvalid     = "effect_enforcement_invalid"
	ReasonEvidenceMissing        = "effect_evidence_missing"
	ReasonContractIDMissing      = "effect_contract_id_missing"
	ReasonPredicateInvalid       = "effect_predicate_invalid"
	ReasonContractInvalid        = "effect_contract_invalid"
	ReasonSnapshotInvalid        = "effect_snapshot_invalid"
	ReasonContractMismatch       = "effect_contract_mismatch"
	ReasonPredicateFailed        = "effect_predicate_failed"
	ReasonPredicateInconclusive  = "effect_predicate_inconclusive"
	ReasonEvidenceIncomplete     = "effect_evidence_incomplete"
	ReasonObservedOnly           = "effect_observed_only"
	ReasonFieldUnavailable       = "effect_field_unavailable"
	ReasonWhitespaceInvalid      = "effect_whitespace_invalid"
	ReasonSchemaValidationFailed = "effect_schema_validation_failed"
	ReasonProvenanceMissing      = "effect_provenance_missing"
	ReasonProvenanceInvalid      = "effect_provenance_invalid"
	ReasonNonAuthoritative       = "effect_non_authoritative_provenance"
	ReasonTrustedKeyMissing      = "effect_trusted_collector_key_missing"
	ReasonTrustedKeyMismatch     = "effect_trusted_collector_key_mismatch"
	ReasonCorrelationMissing     = "effect_correlation_missing"
	ReasonCorrelationMismatch    = "effect_correlation_mismatch"
	ReasonTemporalOrderInvalid   = "effect_temporal_order_invalid"
)

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)

const maxEffectInputBytes = 8 << 20

type Selector struct {
	Resource string            `json:"resource"`
	Scope    string            `json:"scope,omitempty"`
	Name     string            `json:"name,omitempty"`
	Path     string            `json:"path,omitempty"`
	URL      string            `json:"url,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type Observation struct {
	State        string   `json:"state"`
	Digest       string   `json:"digest,omitempty"`
	Count        *int64   `json:"count,omitempty"`
	Identity     string   `json:"identity,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	TTLSeconds   *int64   `json:"ttl_seconds,omitempty"`
	ObservedAt   string   `json:"observed_at"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type Collector struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Mode    string `json:"mode"`
}

type Capture struct {
	Mode       string `json:"mode"`
	SourceRef  string `json:"source_ref,omitempty"`
	CapturedAt string `json:"captured_at"`
}

type Redaction struct {
	Mode   string   `json:"mode"`
	Fields []string `json:"fields,omitempty"`
}

type Correlation struct {
	ActionDigest     string   `json:"action_digest,omitempty"`
	ActivationDigest string   `json:"activation_digest,omitempty"`
	ProofDigest      string   `json:"proof_digest,omitempty"`
	RunID            string   `json:"run_id,omitempty"`
	LifecycleID      string   `json:"lifecycle_id,omitempty"`
	ProofRefs        []string `json:"proof_refs,omitempty"`
}

type Provenance struct {
	Mode      string              `json:"mode"`
	PublicKey string              `json:"public_key"`
	Signature proofsign.Signature `json:"signature"`
}

type Snapshot struct {
	SchemaID               string      `json:"schema_id"`
	SchemaVersion          string      `json:"schema_version"`
	SnapshotID             string      `json:"snapshot_id"`
	ResourceKind           string      `json:"resource_kind"`
	Selector               Selector    `json:"selector"`
	Before                 Observation `json:"before"`
	After                  Observation `json:"after"`
	Collector              Collector   `json:"collector"`
	Capture                Capture     `json:"capture"`
	Redaction              Redaction   `json:"redaction"`
	Correlation            Correlation `json:"correlation"`
	Provenance             Provenance  `json:"provenance"`
	Completeness           string      `json:"completeness"`
	Enforcement            string      `json:"enforcement"`
	EvidenceRefs           []string    `json:"evidence_refs,omitempty"`
	CanonicalContentDigest string      `json:"canonical_content_digest"`
}

type Predicate struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Field    string `json:"field"`
	Operator string `json:"operator,omitempty"`
	Expected any    `json:"expected,omitempty"`
}

type Contract struct {
	SchemaID      string      `json:"schema_id"`
	SchemaVersion string      `json:"schema_version"`
	ContractID    string      `json:"contract_id"`
	Name          string      `json:"name"`
	Predicates    []Predicate `json:"predicates"`
}

type ValidationResult struct {
	Valid       bool     `json:"valid"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

type PredicateEvaluation struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Field       string   `json:"field"`
	Status      string   `json:"status"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Observed    any      `json:"observed,omitempty"`
	Expected    any      `json:"expected,omitempty"`
}

type GradeResult struct {
	SchemaID       string                `json:"schema_id"`
	SchemaVersion  string                `json:"schema_version"`
	ContractID     string                `json:"contract_id"`
	SnapshotID     string                `json:"snapshot_id"`
	Status         string                `json:"status"`
	EvidenceStatus string                `json:"evidence_status"`
	ReasonCodes    []string              `json:"reason_codes"`
	Evaluations    []PredicateEvaluation `json:"evaluations"`
}

type GradeOptions struct {
	TrustedCollectorPublicKey ed25519.PublicKey
	// AllowFixtureTestProvenance is reserved for the committed fixture generator
	// and package tests; the production CLI does not expose this bypass.
	AllowFixtureTestProvenance bool
	ExpectedCorrelation        *CorrelationExpectation
}

// CorrelationExpectation is the caller-owned identity binding for an effect
// grade. At least one digest must be supplied before a pass is authoritative.
type CorrelationExpectation struct {
	ActionDigest     string
	ActivationDigest string
	ProofDigest      string
}

func (s Snapshot) CanonicalDigest() (string, error) {
	payload := s
	payload.SnapshotID = ""
	payload.CanonicalContentDigest = ""
	payload.EvidenceRefs = sortedStrings(payload.EvidenceRefs)
	payload.Before.EvidenceRefs = sortedStrings(payload.Before.EvidenceRefs)
	payload.After.EvidenceRefs = sortedStrings(payload.After.EvidenceRefs)
	payload.Redaction.Fields = sortedStrings(payload.Redaction.Fields)
	payload.Provenance.Signature = proofsign.Signature{}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	return digest, nil
}

func ValidateSnapshot(s Snapshot) ValidationResult {
	reasons := []string{}
	if raw, err := json.Marshal(s); err != nil || validateEmbeddedSchema(raw, SnapshotSchemaID) != nil {
		reasons = append(reasons, ReasonSchemaValidationFailed)
	}
	if s.SchemaID != SnapshotSchemaID || s.SchemaVersion != SchemaVersion {
		reasons = append(reasons, ReasonSchemaUnsupported)
	}
	if !idPattern.MatchString(strings.TrimSpace(s.SnapshotID)) {
		reasons = append(reasons, ReasonSnapshotIDMissing)
	}
	for _, value := range []string{s.SnapshotID, s.ResourceKind, s.Selector.Resource, s.Collector.Name, s.Collector.Version, s.Collector.Mode, s.Capture.Mode, s.Completeness, s.Enforcement} {
		if value != strings.TrimSpace(value) {
			reasons = append(reasons, ReasonWhitespaceInvalid)
		}
	}
	switch s.ResourceKind {
	case ResourcePostgres, ResourceFilesystem, ResourceHTTP, ResourceGeneric:
	default:
		reasons = append(reasons, ReasonResourceKindInvalid)
	}
	if strings.TrimSpace(s.Selector.Resource) == "" {
		reasons = append(reasons, ReasonSelectorMissing)
	}
	if !hasCorrelationDigest(s.Correlation) {
		reasons = append(reasons, ReasonEvidenceMissing)
	}
	for _, digest := range []string{s.Correlation.ActionDigest, s.Correlation.ActivationDigest, s.Correlation.ProofDigest} {
		if digest != "" && !digestPattern.MatchString(digest) {
			reasons = append(reasons, ReasonDigestInvalid)
		}
	}
	for _, proofRef := range s.Correlation.ProofRefs {
		if !digestPattern.MatchString(proofRef) {
			reasons = append(reasons, ReasonDigestInvalid)
		}
	}
	validateObservation(&reasons, s.Before)
	validateObservation(&reasons, s.After)
	if strings.TrimSpace(s.Collector.Name) == "" || strings.TrimSpace(s.Collector.Version) == "" || strings.TrimSpace(s.Collector.Mode) == "" {
		reasons = append(reasons, ReasonCollectorMissing)
	}
	var beforeAt, afterAt, capturedAt time.Time
	var beforeTimeOK, afterTimeOK, capturedTimeOK bool
	if strings.TrimSpace(s.Capture.CapturedAt) == "" || strings.TrimSpace(s.Capture.Mode) == "" || (s.Capture.Mode != "reference" && s.Capture.Mode != "redacted" && s.Capture.Mode != "full") {
		reasons = append(reasons, ReasonCaptureInvalid)
	} else if parsed, err := time.Parse(time.RFC3339, s.Capture.CapturedAt); err != nil {
		reasons = append(reasons, ReasonCaptureInvalid)
	} else {
		capturedAt = parsed
		capturedTimeOK = true
	}
	if s.Redaction.Mode != "none" && s.Redaction.Mode != "reference_only" && s.Redaction.Mode != "redacted" {
		reasons = append(reasons, ReasonRedactionInvalid)
	}
	if s.Completeness != CompletenessComplete && s.Completeness != CompletenessPartial && s.Completeness != CompletenessUnknown {
		reasons = append(reasons, ReasonCompletenessInvalid)
	}
	if s.Enforcement != EnforcementVerified && s.Enforcement != EnforcementObservedOnly && s.Enforcement != EnforcementPartial && s.Enforcement != EnforcementUnknown {
		reasons = append(reasons, ReasonEnforcementInvalid)
	}
	if len(s.EvidenceRefs) == 0 || hasEmptyEffectRef(s.EvidenceRefs) {
		reasons = append(reasons, ReasonEvidenceMissing)
	}
	if s.Provenance.Mode != "collector_signed" && s.Provenance.Mode != "fixture_test_only" {
		reasons = append(reasons, ReasonProvenanceMissing)
	} else if err := s.VerifyProvenance(); err != nil {
		reasons = append(reasons, ReasonProvenanceInvalid)
	}
	if !digestPattern.MatchString(s.CanonicalContentDigest) {
		reasons = append(reasons, ReasonDigestInvalid)
	} else if digest, err := s.CanonicalDigest(); err != nil {
		reasons = append(reasons, ReasonDigestInvalid)
	} else if digest != s.CanonicalContentDigest {
		reasons = append(reasons, ReasonDigestMismatch)
	}
	beforeAt, beforeTimeOK = parseEffectTime(s.Before.ObservedAt)
	afterAt, afterTimeOK = parseEffectTime(s.After.ObservedAt)
	if beforeTimeOK && afterTimeOK && capturedTimeOK {
		if beforeAt.After(afterAt) || afterAt.After(capturedAt) {
			reasons = append(reasons, ReasonTemporalOrderInvalid)
		}
	}
	reasons = sortedReasons(reasons)
	return ValidationResult{Valid: len(reasons) == 0, ReasonCodes: reasons}
}

func validateObservation(reasons *[]string, o Observation) {
	if o.State != ObservationPresent && o.State != ObservationAbsent && o.State != ObservationUnknown {
		*reasons = append(*reasons, ReasonObservationInvalid)
	}
	if o.Digest != "" && !digestPattern.MatchString(o.Digest) {
		*reasons = append(*reasons, ReasonDigestInvalid)
	}
	if (o.Count != nil && *o.Count < 0) || (o.TTLSeconds != nil && *o.TTLSeconds < 0) {
		*reasons = append(*reasons, ReasonObservationInvalid)
	}
	if strings.TrimSpace(o.ObservedAt) == "" {
		*reasons = append(*reasons, ReasonObservationInvalid)
	} else if _, err := time.Parse(time.RFC3339, o.ObservedAt); err != nil {
		*reasons = append(*reasons, ReasonObservationInvalid)
	}
}

func parseEffectTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func hasEmptyEffectRef(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func hasCorrelationDigest(c Correlation) bool {
	return c.ActionDigest != "" || c.ActivationDigest != "" || c.ProofDigest != "" || len(c.ProofRefs) > 0
}

func ValidateContract(c Contract) ValidationResult {
	reasons := []string{}
	if raw, err := json.Marshal(c); err != nil || validateEmbeddedSchema(raw, ContractSchemaID) != nil {
		reasons = append(reasons, ReasonSchemaValidationFailed)
	}
	if c.SchemaID != ContractSchemaID || c.SchemaVersion != SchemaVersion {
		reasons = append(reasons, ReasonSchemaUnsupported)
	}
	if !idPattern.MatchString(strings.TrimSpace(c.ContractID)) {
		reasons = append(reasons, ReasonContractIDMissing)
	}
	if c.ContractID != strings.TrimSpace(c.ContractID) || c.Name != strings.TrimSpace(c.Name) {
		reasons = append(reasons, ReasonWhitespaceInvalid)
	}
	if len(c.Predicates) == 0 {
		reasons = append(reasons, ReasonPredicateInvalid)
	}
	seen := map[string]bool{}
	for _, p := range c.Predicates {
		if !idPattern.MatchString(strings.TrimSpace(p.ID)) || seen[p.ID] {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		seen[p.ID] = true
		if p.ID != strings.TrimSpace(p.ID) || p.Kind != strings.TrimSpace(p.Kind) || p.Field != strings.TrimSpace(p.Field) || p.Operator != strings.TrimSpace(p.Operator) {
			reasons = append(reasons, ReasonWhitespaceInvalid)
		}
		if p.Kind != PredicateExpect && p.Kind != PredicateForbid && p.Kind != PredicateInvariant {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if strings.TrimSpace(p.Field) == "" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if p.Kind != PredicateInvariant && strings.TrimSpace(p.Operator) == "" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if p.Kind != PredicateInvariant && p.Operator == "unchanged" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if p.Operator != "" && !supportedOperator(p.Operator) {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if p.Kind == PredicateInvariant && p.Operator != "" && p.Operator != "unchanged" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
	}
	reasons = sortedReasons(reasons)
	return ValidationResult{Valid: len(reasons) == 0, ReasonCodes: reasons}
}

func Grade(snapshot Snapshot, contract Contract) GradeResult {
	return GradeWithOptions(snapshot, contract, GradeOptions{})
}

func GradeWithOptions(snapshot Snapshot, contract Contract, options GradeOptions) GradeResult {
	snapshotValidation := ValidateSnapshot(snapshot)
	contractValidation := ValidateContract(contract)
	evidenceStatus := snapshot.Enforcement
	if evidenceStatus != EnforcementVerified && evidenceStatus != EnforcementObservedOnly && evidenceStatus != EnforcementPartial && evidenceStatus != EnforcementUnknown {
		evidenceStatus = EnforcementUnknown
	}
	result := GradeResult{SchemaID: GradeSchemaID, SchemaVersion: SchemaVersion, ContractID: contract.ContractID, SnapshotID: snapshot.SnapshotID, EvidenceStatus: evidenceStatus, ReasonCodes: []string{}, Evaluations: []PredicateEvaluation{}}
	if !snapshotValidation.Valid {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonSnapshotInvalid)
		result.ReasonCodes = append(result.ReasonCodes, snapshotValidation.ReasonCodes...)
		return finishGrade(result)
	}
	if !contractValidation.Valid {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonContractInvalid)
		result.ReasonCodes = append(result.ReasonCodes, contractValidation.ReasonCodes...)
		return finishGrade(result)
	}
	if len(options.TrustedCollectorPublicKey) != ed25519.PublicKeySize {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonTrustedKeyMissing)
		return finishGrade(result)
	}
	if err := snapshot.VerifyProvenanceAgainst(options.TrustedCollectorPublicKey); err != nil {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonTrustedKeyMismatch)
		return finishGrade(result)
	}
	if reasons := verifyExpectedCorrelation(snapshot.Correlation, options.ExpectedCorrelation); len(reasons) > 0 {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, reasons...)
		return finishGrade(result)
	}
	if snapshot.Completeness != CompletenessComplete || snapshot.Enforcement == EnforcementPartial || snapshot.Enforcement == EnforcementUnknown {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonEvidenceIncomplete)
		return finishGrade(result)
	}
	if snapshot.Enforcement == EnforcementObservedOnly {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonObservedOnly)
		return finishGrade(result)
	}
	if snapshot.Provenance.Mode == "fixture_test_only" && !options.AllowFixtureTestProvenance {
		result.Status = GradeInconclusive
		result.ReasonCodes = append(result.ReasonCodes, ReasonNonAuthoritative)
		return finishGrade(result)
	}
	predicates := append([]Predicate(nil), contract.Predicates...)
	sort.Slice(predicates, func(i, j int) bool { return predicates[i].ID < predicates[j].ID })
	for _, predicate := range predicates {
		evaluation := evaluatePredicate(snapshot, predicate)
		result.Evaluations = append(result.Evaluations, evaluation)
		if evaluation.Status == GradeFail {
			result.ReasonCodes = append(result.ReasonCodes, ReasonPredicateFailed)
		}
		if evaluation.Status == GradeInconclusive {
			result.ReasonCodes = append(result.ReasonCodes, ReasonPredicateInconclusive)
		}
	}
	if hasEvaluationStatus(result.Evaluations, GradeFail) {
		result.Status = GradeFail
	} else if hasEvaluationStatus(result.Evaluations, GradeInconclusive) {
		result.Status = GradeInconclusive
	} else {
		result.Status = GradePass
	}
	return finishGrade(result)
}

func verifyExpectedCorrelation(actual Correlation, expected *CorrelationExpectation) []string {
	if expected == nil || (expected.ActionDigest == "" && expected.ActivationDigest == "" && expected.ProofDigest == "") {
		return []string{ReasonCorrelationMissing}
	}
	reasons := []string{}
	for _, digest := range []string{expected.ActionDigest, expected.ActivationDigest, expected.ProofDigest} {
		if digest != "" && !digestPattern.MatchString(digest) {
			reasons = append(reasons, ReasonDigestInvalid)
		}
	}
	if (expected.ActionDigest != "" && actual.ActionDigest != expected.ActionDigest) ||
		(expected.ActivationDigest != "" && actual.ActivationDigest != expected.ActivationDigest) ||
		(expected.ProofDigest != "" && actual.ProofDigest != expected.ProofDigest) {
		reasons = append(reasons, ReasonCorrelationMismatch)
	}
	return sortedReasons(reasons)
}

func evaluatePredicate(snapshot Snapshot, predicate Predicate) PredicateEvaluation {
	evaluation := PredicateEvaluation{ID: predicate.ID, Kind: predicate.Kind, Field: predicate.Field, Status: GradePass, Expected: predicate.Expected}
	if predicate.Kind == PredicateInvariant {
		before, beforeOK := fieldValue(snapshot, "before."+predicate.Field)
		after, afterOK := fieldValue(snapshot, "after."+predicate.Field)
		if !beforeOK || !afterOK {
			return inconclusiveEvaluation(evaluation, ReasonFieldUnavailable)
		}
		evaluation.Observed = map[string]any{"before": before, "after": after}
		if !valuesEqual(before, after) {
			return failEvaluation(evaluation)
		}
		return evaluation
	}
	observed, ok := fieldValue(snapshot, predicate.Field)
	if !ok {
		return inconclusiveEvaluation(evaluation, ReasonFieldUnavailable)
	}
	evaluation.Observed = observed
	matched, supported := matches(observed, predicate.Operator, predicate.Expected)
	if !supported {
		return inconclusiveEvaluation(evaluation, ReasonPredicateInvalid)
	}
	if predicate.Kind == PredicateForbid {
		if matched {
			return failEvaluation(evaluation)
		}
		return evaluation
	}
	if !matched {
		return failEvaluation(evaluation)
	}
	return evaluation
}

func fieldValue(snapshot Snapshot, field string) (any, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) == 2 && (parts[0] == "before" || parts[0] == "after") {
		observation := snapshot.Before
		if parts[0] == "after" {
			observation = snapshot.After
		}
		switch parts[1] {
		case "state":
			if observation.State == ObservationUnknown {
				return nil, false
			}
			return observation.State, true
		}
		if observation.State != ObservationPresent {
			return nil, false
		}
		switch parts[1] {
		case "digest":
			return nonEmptyValue(observation.Digest)
		case "count":
			if observation.Count == nil {
				return nil, false
			}
			return *observation.Count, true
		case "identity":
			return nonEmptyValue(observation.Identity)
		case "owner":
			return nonEmptyValue(observation.Owner)
		case "ttl_seconds":
			if observation.TTLSeconds == nil {
				return nil, false
			}
			return *observation.TTLSeconds, true
		}
		return nil, false
	}
	switch field {
	case "resource_kind":
		return snapshot.ResourceKind, true
	case "completeness":
		return snapshot.Completeness, true
	case "enforcement":
		return snapshot.Enforcement, true
	case "selector.resource":
		return snapshot.Selector.Resource, true
	case "selector.scope":
		return nonEmptyValue(snapshot.Selector.Scope)
	case "selector.name":
		return nonEmptyValue(snapshot.Selector.Name)
	}
	return nil, false
}

func nonEmptyValue(value string) (any, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, false
	}
	return value, true
}

func matches(observed any, operator string, expected any) (bool, bool) {
	switch operator {
	case "exists":
		return observed != nil, true
	case "equals":
		return valuesEqual(observed, expected), true
	case "not_equals":
		return !valuesEqual(observed, expected), true
	case "contains":
		left, ok := observed.(string)
		right, rightOK := expected.(string)
		return ok && rightOK && strings.Contains(left, right), ok && rightOK
	case "gte", "lte":
		left, leftOK := numberValue(observed)
		right, rightOK := numberValue(expected)
		if !leftOK || !rightOK {
			return false, false
		}
		if operator == "gte" {
			return left >= right, true
		}
		return left <= right, true
	default:
		return false, false
	}
}

func supportedOperator(operator string) bool {
	switch operator {
	case "equals", "not_equals", "contains", "exists", "gte", "lte", "unchanged":
		return true
	default:
		return false
	}
}

func validateEmbeddedSchema(raw []byte, schemaID string) error {
	filename := "effect_snapshot.schema.json"
	if schemaID == ContractSchemaID {
		filename = "effect_contract.schema.json"
	}
	payload, err := schemaAssets.ReadFile("schemaassets/" + filename)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaID, bytes.NewReader(payload)); err != nil {
		return err
	}
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		return err
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	return schema.Validate(document)
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case int64:
		return float64(number), true
	case int:
		return float64(number), true
	case float64:
		return number, !math.IsNaN(number)
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func valuesEqual(left, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftRaw) == string(rightRaw)
}

func finishGrade(result GradeResult) GradeResult {
	result.ReasonCodes = sortedReasons(result.ReasonCodes)
	if result.ReasonCodes == nil {
		result.ReasonCodes = []string{}
	}
	if result.Evaluations == nil {
		result.Evaluations = []PredicateEvaluation{}
	}
	return result
}

func hasEvaluationStatus(evaluations []PredicateEvaluation, status string) bool {
	for _, evaluation := range evaluations {
		if evaluation.Status == status {
			return true
		}
	}
	return false
}

func failEvaluation(evaluation PredicateEvaluation) PredicateEvaluation {
	evaluation.Status = GradeFail
	evaluation.ReasonCodes = []string{ReasonPredicateFailed}
	return evaluation
}

func inconclusiveEvaluation(evaluation PredicateEvaluation, reason string) PredicateEvaluation {
	evaluation.Status = GradeInconclusive
	evaluation.ReasonCodes = []string{reason}
	return evaluation
}

func sortedReasons(reasons []string) []string {
	result := append([]string(nil), reasons...)
	sort.Strings(result)
	unique := result[:0]
	for _, reason := range result {
		if len(unique) == 0 || unique[len(unique)-1] != reason {
			unique = append(unique, reason)
		}
	}
	return unique
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

type junitReport struct {
	XMLName   xml.Name    `xml:"testsuite"`
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	TestCases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

func WriteJUnit(path string, result GradeResult) error {
	report := junitReport{Name: "effects", Tests: len(result.Evaluations)}
	if len(result.Evaluations) == 0 {
		report.Tests = 1
		if result.Status != GradePass {
			report.Failures = 1
			report.TestCases = append(report.TestCases, junitCase{Name: "effect_contract", ClassName: "gait.effects", Failure: &junitFailure{Message: strings.Join(result.ReasonCodes, ","), Type: result.Status, Body: "effect contract did not produce an authoritative pass"}})
			return writeJUnitFile(path, report)
		}
		report.TestCases = append(report.TestCases, junitCase{Name: "effect_contract", ClassName: "gait.effects"})
		return writeJUnitFile(path, report)
	}
	for _, evaluation := range result.Evaluations {
		caseResult := junitCase{Name: evaluation.ID, ClassName: "gait.effects"}
		if evaluation.Status != GradePass {
			report.Failures++
			caseResult.Failure = &junitFailure{Message: strings.Join(evaluation.ReasonCodes, ","), Type: evaluation.Status, Body: "effect predicate did not produce an authoritative pass"}
		}
		report.TestCases = append(report.TestCases, caseResult)
	}
	return writeJUnitFile(path, report)
}

func writeJUnitFile(path string, report junitReport) error {
	raw, err := xml.Marshal(report)
	if err != nil {
		return err
	}
	raw = append([]byte(xml.Header), raw...)
	return writeAtomicExclusive(path, raw)
}

func LoadSnapshot(path string) (Snapshot, error) {
	raw, err := readSelectedFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read effect snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := decodeStrict(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("parse effect snapshot: %w", err)
	}
	if validation := ValidateSnapshot(snapshot); !validation.Valid {
		return Snapshot{}, fmt.Errorf("validate effect snapshot: %s", strings.Join(validation.ReasonCodes, ","))
	}
	return snapshot, nil
}

func LoadContract(path string) (Contract, error) {
	raw, err := readSelectedFile(path)
	if err != nil {
		return Contract{}, fmt.Errorf("read effect contract: %w", err)
	}
	var contract Contract
	if err := decodeStrict(raw, &contract); err != nil {
		return Contract{}, fmt.Errorf("parse effect contract: %w", err)
	}
	if validation := ValidateContract(contract); !validation.Valid {
		return Contract{}, fmt.Errorf("validate effect contract: %s", strings.Join(validation.ReasonCodes, ","))
	}
	return contract, nil
}

func (s Snapshot) Sign(privateKey ed25519.PrivateKey, mode string) (Snapshot, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Snapshot{}, fmt.Errorf("invalid effect signing key length")
	}
	if mode != "collector_signed" && mode != "fixture_test_only" {
		return Snapshot{}, fmt.Errorf("invalid effect provenance mode")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	s.Provenance = Provenance{Mode: mode, PublicKey: base64.StdEncoding.EncodeToString(publicKey)}
	digest, err := s.CanonicalDigest()
	if err != nil {
		return Snapshot{}, err
	}
	s.CanonicalContentDigest = digest
	signature, err := proofsign.SignDigestHex(privateKey, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return Snapshot{}, err
	}
	s.Provenance.Signature = signature
	return s, nil
}

func (s Snapshot) VerifyProvenance() error {
	publicBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s.Provenance.PublicKey))
	if err != nil {
		return err
	}
	if len(publicBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid effect public key length")
	}
	return s.VerifyProvenanceAgainst(ed25519.PublicKey(publicBytes))
}

func (s Snapshot) VerifyProvenanceAgainst(publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid trusted effect public key length")
	}
	if s.Provenance.Signature.KeyID != proofsign.KeyID(publicKey) {
		return fmt.Errorf("effect provenance key id mismatch")
	}
	digest, err := s.CanonicalDigest()
	if err != nil {
		return err
	}
	if digest != s.CanonicalContentDigest {
		return fmt.Errorf("effect provenance canonical digest mismatch")
	}
	if strings.TrimPrefix(digest, "sha256:") != s.Provenance.Signature.SignedDigest {
		return fmt.Errorf("effect provenance digest mismatch")
	}
	valid, err := proofsign.VerifyDigestHex(publicKey, s.Provenance.Signature)
	if err != nil || !valid {
		return fmt.Errorf("effect provenance signature invalid")
	}
	return nil
}

func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := readSelectedFile(path)
	if err != nil {
		return nil, fmt.Errorf("read effect public key: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid effect public key")
	}
	return ed25519.PublicKey(decoded), nil
}

func readSelectedFile(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("effect path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := rejectSymlinkAncestors(abs); err != nil {
		return nil, err
	}
	file, err := os.Open(abs) // #nosec G304 -- explicit caller-selected evidence path, with symlink ancestors rejected.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("effect path is not a regular file")
	}
	if info.Size() > maxEffectInputBytes {
		return nil, fmt.Errorf("effect input exceeds %d bytes", maxEffectInputBytes)
	}
	pathInfo, err := os.Lstat(abs)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !os.SameFile(info, pathInfo) {
		return nil, fmt.Errorf("effect path changed during read")
	}
	readOnce := func() ([]byte, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return nil, err
		}
		return io.ReadAll(io.LimitReader(file, maxEffectInputBytes+1))
	}
	first, err := readOnce()
	if err != nil {
		return nil, err
	}
	second, err := readOnce()
	if err != nil {
		return nil, err
	}
	if len(first) > maxEffectInputBytes || len(second) > maxEffectInputBytes {
		return nil, fmt.Errorf("effect input exceeds %d bytes", maxEffectInputBytes)
	}
	if !bytes.Equal(first, second) {
		return nil, fmt.Errorf("effect file changed during read")
	}
	pathInfo, err = os.Lstat(abs)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !os.SameFile(info, pathInfo) {
		return nil, fmt.Errorf("effect path changed during read")
	}
	return first, nil
}

func rejectSymlinkAncestors(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	current := abs
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if runtime.GOOS != "darwin" || (current != "/var" && current != "/tmp") || resolveErr != nil || !strings.HasPrefix(resolved, "/private/") {
				return fmt.Errorf("effect path contains symlink: %s", current)
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func writeAtomicExclusive(path string, raw []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("JUnit path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	if err := rejectSymlinkAncestors(dir); err != nil {
		return err
	}
	if info, statErr := os.Lstat(abs); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("JUnit output is a symlink")
		}
		return fmt.Errorf("JUnit output already exists")
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	temp, err := os.CreateTemp(dir, ".effects-junit-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }() // #nosec G703 -- tempName is generated by os.CreateTemp in the validated output directory.
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Link(tempName, abs); err != nil {
		return err
	}
	if err := os.Remove(tempName); err != nil { // #nosec G703 -- tempName is generated by os.CreateTemp in the validated output directory.
		return err
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON key: %s", key)
			}
			seen[key] = true
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return nil
	}
}
