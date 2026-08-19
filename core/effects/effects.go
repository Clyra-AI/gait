// Package effects models bounded, reference-first effect evidence and grades
// typed effect contracts without executing or mutating external systems.
package effects

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	proofcanon "github.com/Clyra-AI/proof/canon"
)

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
	ReasonSchemaUnsupported     = "effect_schema_unsupported"
	ReasonSnapshotIDMissing     = "effect_snapshot_id_missing"
	ReasonResourceKindInvalid   = "effect_resource_kind_invalid"
	ReasonSelectorMissing       = "effect_selector_missing"
	ReasonObservationInvalid    = "effect_observation_invalid"
	ReasonDigestInvalid         = "effect_digest_invalid"
	ReasonDigestMismatch        = "effect_digest_mismatch"
	ReasonCollectorMissing      = "effect_collector_missing"
	ReasonCaptureInvalid        = "effect_capture_invalid"
	ReasonRedactionInvalid      = "effect_redaction_invalid"
	ReasonCompletenessInvalid   = "effect_completeness_invalid"
	ReasonEnforcementInvalid    = "effect_enforcement_invalid"
	ReasonEvidenceMissing       = "effect_evidence_missing"
	ReasonContractIDMissing     = "effect_contract_id_missing"
	ReasonPredicateInvalid      = "effect_predicate_invalid"
	ReasonContractInvalid       = "effect_contract_invalid"
	ReasonSnapshotInvalid       = "effect_snapshot_invalid"
	ReasonContractMismatch      = "effect_contract_mismatch"
	ReasonPredicateFailed       = "effect_predicate_failed"
	ReasonPredicateInconclusive = "effect_predicate_inconclusive"
	ReasonEvidenceIncomplete    = "effect_evidence_incomplete"
	ReasonObservedOnly          = "effect_observed_only"
	ReasonFieldUnavailable      = "effect_field_unavailable"
)

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)

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
	Count        int64    `json:"count,omitempty"`
	Identity     string   `json:"identity,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	TTLSeconds   int64    `json:"ttl_seconds,omitempty"`
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

func (s Snapshot) CanonicalDigest() (string, error) {
	payload := s
	payload.SnapshotID = ""
	payload.CanonicalContentDigest = ""
	payload.EvidenceRefs = sortedStrings(payload.EvidenceRefs)
	payload.Before.EvidenceRefs = sortedStrings(payload.Before.EvidenceRefs)
	payload.After.EvidenceRefs = sortedStrings(payload.After.EvidenceRefs)
	payload.Redaction.Fields = sortedStrings(payload.Redaction.Fields)
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
	if s.SchemaID != SnapshotSchemaID || s.SchemaVersion != SchemaVersion {
		reasons = append(reasons, ReasonSchemaUnsupported)
	}
	if !idPattern.MatchString(strings.TrimSpace(s.SnapshotID)) {
		reasons = append(reasons, ReasonSnapshotIDMissing)
	}
	switch s.ResourceKind {
	case ResourcePostgres, ResourceFilesystem, ResourceHTTP, ResourceGeneric:
	default:
		reasons = append(reasons, ReasonResourceKindInvalid)
	}
	if strings.TrimSpace(s.Selector.Resource) == "" {
		reasons = append(reasons, ReasonSelectorMissing)
	}
	validateObservation(&reasons, s.Before)
	validateObservation(&reasons, s.After)
	if strings.TrimSpace(s.Collector.Name) == "" || strings.TrimSpace(s.Collector.Version) == "" || strings.TrimSpace(s.Collector.Mode) == "" {
		reasons = append(reasons, ReasonCollectorMissing)
	}
	if strings.TrimSpace(s.Capture.CapturedAt) == "" || strings.TrimSpace(s.Capture.Mode) == "" || (s.Capture.Mode != "reference" && s.Capture.Mode != "redacted" && s.Capture.Mode != "full") {
		reasons = append(reasons, ReasonCaptureInvalid)
	} else if _, err := time.Parse(time.RFC3339, s.Capture.CapturedAt); err != nil {
		reasons = append(reasons, ReasonCaptureInvalid)
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
	if len(s.EvidenceRefs) == 0 && s.Redaction.Mode == "none" {
		reasons = append(reasons, ReasonEvidenceMissing)
	}
	if !digestPattern.MatchString(s.CanonicalContentDigest) {
		reasons = append(reasons, ReasonDigestInvalid)
	} else if digest, err := s.CanonicalDigest(); err != nil {
		reasons = append(reasons, ReasonDigestInvalid)
	} else if digest != s.CanonicalContentDigest {
		reasons = append(reasons, ReasonDigestMismatch)
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
	if o.Count < 0 || o.TTLSeconds < 0 {
		*reasons = append(*reasons, ReasonObservationInvalid)
	}
	if strings.TrimSpace(o.ObservedAt) == "" {
		*reasons = append(*reasons, ReasonObservationInvalid)
	} else if _, err := time.Parse(time.RFC3339, o.ObservedAt); err != nil {
		*reasons = append(*reasons, ReasonObservationInvalid)
	}
}

func ValidateContract(c Contract) ValidationResult {
	reasons := []string{}
	if c.SchemaID != ContractSchemaID || c.SchemaVersion != SchemaVersion {
		reasons = append(reasons, ReasonSchemaUnsupported)
	}
	if !idPattern.MatchString(strings.TrimSpace(c.ContractID)) {
		reasons = append(reasons, ReasonContractIDMissing)
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
		if p.Kind != PredicateExpect && p.Kind != PredicateForbid && p.Kind != PredicateInvariant {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if strings.TrimSpace(p.Field) == "" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
		if p.Kind != PredicateInvariant && strings.TrimSpace(p.Operator) == "" {
			reasons = append(reasons, ReasonPredicateInvalid)
		}
	}
	reasons = sortedReasons(reasons)
	return ValidationResult{Valid: len(reasons) == 0, ReasonCodes: reasons}
}

func Grade(snapshot Snapshot, contract Contract) GradeResult {
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
			return observation.State, true
		case "digest":
			return nonEmptyValue(observation.Digest)
		case "count":
			return observation.Count, true
		case "identity":
			return nonEmptyValue(observation.Identity)
		case "owner":
			return nonEmptyValue(observation.Owner)
		case "ttl_seconds":
			return observation.TTLSeconds, true
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
	return os.WriteFile(path, raw, 0o600)
}

func LoadSnapshot(path string) (Snapshot, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- caller explicitly selects the local evidence path.
	if err != nil {
		return Snapshot{}, fmt.Errorf("read effect snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("parse effect snapshot: %w", err)
	}
	return snapshot, nil
}

func LoadContract(path string) (Contract, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- caller explicitly selects the local contract path.
	if err != nil {
		return Contract{}, fmt.Errorf("read effect contract: %w", err)
	}
	var contract Contract
	if err := json.Unmarshal(raw, &contract); err != nil {
		return Contract{}, fmt.Errorf("parse effect contract: %w", err)
	}
	return contract, nil
}
