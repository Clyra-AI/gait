package actioncontract

// This file contains the runtime-facing, pre-execution portion of the Action
// Contract.  It deliberately does not execute tools, issue credentials, or
// assert that an effect occurred.  Wrkr's proposed artifact remains the
// source of intended outcome data; this model gives Gait a small, versioned
// and deterministic projection for boundary/readiness decisions.

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	RuntimeActionSchemaID      = "https://gait.dev/schemas/v1/runtime-action.schema.json"
	RuntimeActionSchemaVersion = "1"
	RuntimeReadinessSchemaID   = "https://gait.dev/schemas/v1/runtime-readiness.schema.json"
	RuntimeLifecycleSchemaID   = "https://gait.dev/schemas/v1/runtime-lifecycle-record.schema.json"
	RuntimeLifecycleVersion    = "1"
	ProofCompatibilityVersion  = "0.6.1"
	CorrelationProfileVersion  = "1.0"
)

// These values intentionally include the classes emitted by Wrkr's released
// action-contract fixtures.  The additional values are runtime-only
// classifications and do not change Wrkr's report semantics.
const (
	ActionClassRead             = "read"
	ActionClassWrite            = "write"
	ActionClassDeploy           = "deploy"
	ActionClassDelete           = "delete"
	ActionClassExecute          = "execute"
	ActionClassEgress           = "egress"
	ActionClassCredentialAccess = "credential_access"
	ActionClassRelease          = "release"
	ActionClassResource         = "resource"
)

var runtimeActionClasses = map[string]struct{}{
	ActionClassRead: {}, ActionClassWrite: {}, ActionClassDeploy: {},
	ActionClassDelete: {}, ActionClassExecute: {}, ActionClassEgress: {},
	ActionClassCredentialAccess: {}, ActionClassRelease: {}, ActionClassResource: {},
}

var runtimeCompositionRoles = map[string]struct{}{
	"source": {}, "transform": {}, "sink": {}, "internal_sink": {},
	"external_sink": {}, "privileged_sink": {}, "destructive_sink": {},
}

var runtimeDataClasses = map[string]struct{}{
	"public": {}, "internal": {}, "confidential": {}, "sensitive": {},
	"pii": {}, "phi": {}, "secret": {}, "credential": {}, "code": {},
	"unknown": {}, "unclassified": {},
}

var runtimeTrustClasses = map[string]struct{}{
	"unknown": {}, "untrusted": {}, "external": {}, "internal": {},
	"trusted": {}, "privileged": {}, "production": {},
}

var runtimeTransitionClasses = map[string]struct{}{
	"none": {}, "read": {}, "write": {}, "delete": {}, "execute": {},
	"deploy": {}, "egress": {}, "credential": {}, "delegate": {},
	"resource_lifecycle": {}, "approval": {}, "control": {},
}

var runtimeOutcomeClasses = map[string]struct{}{
	"unknown": {}, "none": {}, "read": {}, "write": {}, "execute": {},
	"data_egress": {}, "network_egress": {}, "production_deploy": {},
	"production_mutation": {}, "release_publish": {}, "resource_change": {},
	"external_egress": {}, "release": {},
}

var runtimeResourceActions = map[string]struct{}{
	"none": {}, "create": {}, "acquire": {}, "reserve": {}, "update": {},
	"delete": {}, "release": {}, "cleanup": {}, "expire": {}, "revoke": {},
	"rotate": {}, "snapshot": {}, "restore": {}, "attach": {}, "detach": {},
	"observe": {},
}

// RuntimeVocabularies exposes the fixed compatibility vocabularies in sorted
// order for schema/fixture generators. Callers receive fresh slices.
func RuntimeActionClassVocabulary() []string      { return vocabulary(runtimeActionClasses) }
func RuntimeCompositionRoleVocabulary() []string  { return vocabulary(runtimeCompositionRoles) }
func RuntimeDataClassVocabulary() []string        { return vocabulary(runtimeDataClasses) }
func RuntimeTargetTrustClassVocabulary() []string { return vocabulary(runtimeTrustClasses) }
func RuntimeTransitionClassVocabulary() []string  { return vocabulary(runtimeTransitionClasses) }
func RuntimeOutcomeClassVocabulary() []string     { return vocabulary(runtimeOutcomeClasses) }
func RuntimeResourceActionVocabulary() []string   { return vocabulary(runtimeResourceActions) }

func vocabulary(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// RuntimeAction is a versioned pre-execution classification.  RiskClass is
// intentionally independent of ResourceLifecycleActions: an action can
// reserve/cleanup a resource without changing its risk classification.
type RuntimeAction struct {
	SchemaID                 string               `json:"schema_id"`
	SchemaVersion            string               `json:"schema_version"`
	ActionID                 string               `json:"action_id"`
	ActionClass              string               `json:"action_class"`
	ActionClasses            []string             `json:"action_classes,omitempty"`
	CompositionRole          string               `json:"composition_role"`
	DataClasses              []string             `json:"data_classes,omitempty"`
	TargetTrustClass         string               `json:"target_trust_class"`
	TransitionClass          string               `json:"transition_class"`
	ExpectedOutcomeClass     string               `json:"expected_outcome_class"`
	IntendedOutcomeClass     string               `json:"intended_outcome_class"`
	ResourceLifecycleActions []string             `json:"resource_lifecycle_actions,omitempty"`
	ResourceActions          []string             `json:"resource_actions,omitempty"`
	RiskClass                string               `json:"risk_class,omitempty"`
	TargetRef                string               `json:"target_ref,omitempty"`
	Boundary                 RuntimeBoundary      `json:"boundary"`
	Stages                   []RuntimeActionStage `json:"stages,omitempty"`
	ObservedEffect           *ObservedEffect      `json:"observed_effect,omitempty"`
	InferenceReasons         []string             `json:"inference_reasons,omitempty"`
	ClassificationReasons    []string             `json:"classification_reasons,omitempty"`
}

// RuntimeActionStage is bounded (at most MaxRuntimeStages) to keep the
// representation a hypothesis rather than an unbounded graph/event store.
type RuntimeActionStage struct {
	StageID          string   `json:"stage_id"`
	Role             string   `json:"role"`
	ActionClasses    []string `json:"action_classes,omitempty"`
	DataClasses      []string `json:"data_classes,omitempty"`
	TargetTrustClass string   `json:"target_trust_class"`
	TransitionClass  string   `json:"transition_class"`
	ExpectedOutcome  string   `json:"expected_outcome_class"`
	TargetRef        string   `json:"target_ref,omitempty"`
	BoundaryRefs     []string `json:"boundary_refs,omitempty"`
}

const MaxRuntimeStages = 5

// RuntimeBoundary keeps boundary references separate from evidence refs.
// A boundary ref is a relationship to a trust boundary, not proof that the
// boundary enforced anything.
type RuntimeBoundary struct {
	SourceTrustClass string   `json:"source_trust_class"`
	TargetTrustClass string   `json:"target_trust_class"`
	TransitionClass  string   `json:"transition_class"`
	BoundaryRefs     []string `json:"boundary_refs,omitempty"`
	ProofRefs        []string `json:"proof_refs,omitempty"`
}

type ObservedEffect struct {
	Status      string   `json:"status"` // not_observed|observed|contradictory
	EffectClass string   `json:"effect_class,omitempty"`
	EffectRefs  []string `json:"effect_refs,omitempty"`
	ObservedAt  string   `json:"observed_at,omitempty"`
}

// ClassificationInput is intentionally small.  Supplied values are never
// lowered by inference; inference may only preserve or raise the effective
// action class/control posture.
type ClassificationInput struct {
	ActionID                 string               `json:"action_id"`
	ActionClass              string               `json:"action_class"`
	ActionClasses            []string             `json:"action_classes"`
	CompositionRole          string               `json:"composition_role"`
	DataClasses              []string             `json:"data_classes"`
	TargetTrustClass         string               `json:"target_trust_class"`
	TransitionClass          string               `json:"transition_class"`
	ExpectedOutcomeClass     string               `json:"expected_outcome_class"`
	IntendedOutcomeClass     string               `json:"intended_outcome_class"`
	ResourceLifecycleActions []string             `json:"resource_lifecycle_actions"`
	ResourceActions          []string             `json:"resource_actions"`
	RiskClass                string               `json:"risk_class"`
	TargetRef                string               `json:"target_ref"`
	SourceTrustClass         string               `json:"source_trust_class"`
	BoundaryRefs             []string             `json:"boundary_refs"`
	ProofRefs                []string             `json:"proof_refs"`
	Stages                   []RuntimeActionStage `json:"stages"`
	Hints                    []string             `json:"hints"`
}

type ClassificationResult struct {
	Action      RuntimeAction `json:"action"`
	Valid       bool          `json:"valid"`
	ReasonCodes []string      `json:"reason_codes,omitempty"`
}

// Compatibility aliases keep the public vocabulary explicit for callers that
// refer to the three projections independently.
type ActionClassification = RuntimeAction
type BoundaryClassification = RuntimeBoundary
type OutcomeClassification = ObservedEffect

// ClassifyRuntimeAction deterministically normalizes a runtime action.  It
// does not inspect the network, run validators, or infer observed effects.
func ClassifyRuntimeAction(input ClassificationInput) ClassificationResult {
	action := RuntimeAction{
		SchemaID: RuntimeActionSchemaID, SchemaVersion: RuntimeActionSchemaVersion,
		ActionID: strings.TrimSpace(input.ActionID), ActionClass: normalize(input.ActionClass),
		ActionClasses: sortedUnique(input.ActionClasses), CompositionRole: normalize(input.CompositionRole),
		DataClasses: sortedUnique(input.DataClasses), TargetTrustClass: normalize(input.TargetTrustClass),
		TransitionClass: normalize(input.TransitionClass), ExpectedOutcomeClass: normalize(input.ExpectedOutcomeClass),
		IntendedOutcomeClass: normalize(input.IntendedOutcomeClass), ResourceLifecycleActions: sortedUnique(input.ResourceLifecycleActions),
		ResourceActions: sortedUnique(input.ResourceActions),
		RiskClass:       normalize(input.RiskClass), TargetRef: strings.TrimSpace(input.TargetRef),
		Boundary: RuntimeBoundary{SourceTrustClass: normalize(input.SourceTrustClass), TargetTrustClass: normalize(input.TargetTrustClass), TransitionClass: normalize(input.TransitionClass), BoundaryRefs: sortedUnique(input.BoundaryRefs), ProofRefs: sortedUnique(input.ProofRefs)},
		Stages:   append([]RuntimeActionStage(nil), input.Stages...),
	}
	if action.ActionClass != "" {
		action.ActionClasses = append(action.ActionClasses, action.ActionClass)
	}
	for _, hint := range input.Hints {
		inferred := inferActionClass(hint)
		if inferred != "" {
			action.ActionClasses = append(action.ActionClasses, inferred)
			action.InferenceReasons = append(action.InferenceReasons, "hint:"+normalize(hint)+"->"+inferred)
		}
		if inferredTrust := inferTargetTrustClass(hint); inferredTrust != "" {
			action.TargetTrustClass = strongestTargetTrustClass([]string{action.TargetTrustClass, inferredTrust})
			action.InferenceReasons = append(action.InferenceReasons, "hint:"+normalize(hint)+"->"+inferredTrust)
		}
	}
	action.ActionClasses = normalizeActionClasses(action.ActionClasses)
	if action.ActionClass == "" {
		action.ActionClass = strongestActionClass(action.ActionClasses)
	} else {
		// Never lower an explicitly supplied class when inference finds a
		// stronger one.  This is a monotonic risk floor, not a risk engine.
		action.ActionClass = strongestActionClass(append(action.ActionClasses, action.ActionClass))
	}
	action.TransitionClass = strongestTransitionClass([]string{action.TransitionClass, transitionForAction(action.ActionClass)})
	action.ExpectedOutcomeClass = strongestOutcomeClass([]string{action.ExpectedOutcomeClass, inferOutcome(action.ActionClass)})
	if action.IntendedOutcomeClass == "" {
		action.IntendedOutcomeClass = action.ExpectedOutcomeClass
	}
	if action.CompositionRole == "" && len(action.Stages) > 0 {
		action.CompositionRole = normalize(action.Stages[0].Role)
	}
	if len(action.Stages) > 0 {
		roles := []string{action.CompositionRole}
		for _, stage := range action.Stages {
			roles = append(roles, stage.Role)
		}
		action.CompositionRole = strongestCompositionRole(roles)
	}
	if action.TargetTrustClass == "" {
		action.TargetTrustClass = "unknown"
	}
	action.Boundary.TargetTrustClass = action.TargetTrustClass
	action.Boundary.TransitionClass = action.TransitionClass
	stagesExceeded := len(action.Stages) > MaxRuntimeStages
	if stagesExceeded {
		action.Stages = action.Stages[:MaxRuntimeStages]
		action.ClassificationReasons = append(action.ClassificationReasons, "stages:bounded")
	}
	for i := range action.Stages {
		action.Stages[i] = normalizeStage(action.Stages[i])
	}
	reasons := ValidateRuntimeAction(action)
	if stagesExceeded {
		reasons = append(reasons, "stages_limit_exceeded")
	}
	action.ClassificationReasons = sortedUnique(append(action.ClassificationReasons, reasons...))
	return ClassificationResult{Action: action, Valid: len(reasons) == 0, ReasonCodes: reasons}
}

// ClassifyAction is the concise library entry point used by adapters.
func ClassifyAction(input ClassificationInput) ClassificationResult {
	return ClassifyRuntimeAction(input)
}

// ClassifyArtifact projects the immutable Wrkr proposal without mutating it.
func ClassifyArtifact(artifact Artifact) ClassificationResult {
	contract := artifact.Contract
	input := ClassificationInput{
		ActionID:             artifact.ContractID,
		CompositionRole:      "source",
		ExpectedOutcomeClass: stringField(contract, "expected_outcome_class"),
		IntendedOutcomeClass: stringField(contract, "expected_outcome_class"),
		TargetRef:            stringField(contract, "composition_ref"),
		Hints:                []string{stringField(contract, "expected_outcome_class"), stringField(contract, "composition_ref")},
	}
	for _, item := range objectArray(contract, "target_constraints") {
		key, value := stringField(item, "key"), stringField(item, "value")
		switch key {
		case "target_class":
			if value == "production_impacting" {
				input.TargetTrustClass = "production"
			}
		case "environment":
			if value == "production" {
				input.TargetTrustClass = "production"
			}
		}
	}
	for _, item := range objectArray(contract, "allowed_transitions") {
		input.Hints = append(input.Hints, stringField(item, "to_role"), stringField(item, "reason"))
	}
	// Preserve the proposal's bounded transition topology as runtime stages.
	// The stage list is a projection only: it does not assert that any stage
	// executed or that a transition was permitted at runtime.
	seenStages := map[string]struct{}{}
	for _, transitionKind := range []string{"allowed_transitions", "prohibited_transitions", "approval_required_transitions"} {
		for _, item := range objectArray(contract, transitionKind) {
			for _, prefix := range []string{"from", "to"} {
				stageID := stringField(item, prefix+"_stage_id")
				if stageID == "" {
					continue
				}
				if _, seen := seenStages[stageID]; seen {
					continue
				}
				seenStages[stageID] = struct{}{}
				role := stringField(item, prefix+"_role")
				if role == "" {
					role = "transform"
				}
				input.Stages = append(input.Stages, RuntimeActionStage{StageID: stageID, Role: role, TargetTrustClass: input.TargetTrustClass, ExpectedOutcome: input.ExpectedOutcomeClass})
			}
		}
	}
	return ClassifyRuntimeAction(input)
}

func ValidateRuntimeAction(action RuntimeAction) []string {
	reasons := []string{}
	if action.SchemaID != RuntimeActionSchemaID || action.SchemaVersion != RuntimeActionSchemaVersion {
		reasons = append(reasons, "schema_unsupported")
	}
	if strings.TrimSpace(action.ActionID) == "" {
		reasons = append(reasons, "action_id_missing")
	}
	if _, ok := runtimeActionClasses[normalize(action.ActionClass)]; !ok {
		reasons = append(reasons, "action_class_unsupported")
	}
	if _, ok := runtimeCompositionRoles[normalize(action.CompositionRole)]; !ok {
		reasons = append(reasons, "composition_role_unsupported")
	}
	if _, ok := runtimeTrustClasses[normalize(action.TargetTrustClass)]; !ok {
		reasons = append(reasons, "target_trust_class_unsupported")
	}
	if _, ok := runtimeTransitionClasses[normalize(action.TransitionClass)]; !ok {
		reasons = append(reasons, "transition_class_unsupported")
	}
	if _, ok := runtimeOutcomeClasses[normalize(action.ExpectedOutcomeClass)]; !ok {
		reasons = append(reasons, "expected_outcome_class_unsupported")
	}
	for _, value := range action.ActionClasses {
		if _, ok := runtimeActionClasses[normalize(value)]; !ok {
			reasons = append(reasons, "action_class_unsupported:"+normalize(value))
		}
	}
	for _, value := range action.DataClasses {
		if _, ok := runtimeDataClasses[normalize(value)]; !ok {
			reasons = append(reasons, "data_class_unsupported:"+normalize(value))
		}
	}
	for _, value := range action.ResourceLifecycleActions {
		if _, ok := runtimeResourceActions[normalize(value)]; !ok {
			reasons = append(reasons, "resource_lifecycle_action_unsupported:"+normalize(value))
		}
	}
	for _, value := range action.ResourceActions {
		if _, ok := runtimeResourceActions[normalize(value)]; !ok {
			reasons = append(reasons, "resource_action_unsupported:"+normalize(value))
		}
	}
	if len(action.Stages) > MaxRuntimeStages {
		reasons = append(reasons, "stages_limit_exceeded")
	}
	for i, stage := range action.Stages {
		if strings.TrimSpace(stage.StageID) == "" {
			reasons = append(reasons, fmt.Sprintf("stage_%d_id_missing", i))
		}
		if _, ok := runtimeCompositionRoles[normalize(stage.Role)]; !ok {
			reasons = append(reasons, fmt.Sprintf("stage_%d_role_unsupported", i))
		}
		if _, ok := runtimeTrustClasses[normalize(stage.TargetTrustClass)]; !ok {
			reasons = append(reasons, fmt.Sprintf("stage_%d_target_trust_class_unsupported", i))
		}
		if _, ok := runtimeTransitionClasses[normalize(stage.TransitionClass)]; !ok {
			reasons = append(reasons, fmt.Sprintf("stage_%d_transition_class_unsupported", i))
		}
		if _, ok := runtimeOutcomeClasses[normalize(stage.ExpectedOutcome)]; !ok {
			reasons = append(reasons, fmt.Sprintf("stage_%d_expected_outcome_class_unsupported", i))
		}
	}
	return sortedUnique(reasons)
}

func normalizeStage(stage RuntimeActionStage) RuntimeActionStage {
	stage.StageID = strings.TrimSpace(stage.StageID)
	stage.Role = normalize(stage.Role)
	stage.ActionClasses = normalizeActionClasses(stage.ActionClasses)
	stage.DataClasses = sortedUnique(stage.DataClasses)
	stage.TargetTrustClass = normalize(stage.TargetTrustClass)
	if stage.TargetTrustClass == "" {
		stage.TargetTrustClass = "unknown"
	}
	stage.TransitionClass = normalize(stage.TransitionClass)
	stage.TransitionClass = strongestTransitionClass([]string{stage.TransitionClass, transitionForAction(strongestActionClass(stage.ActionClasses))})
	stage.ExpectedOutcome = normalize(stage.ExpectedOutcome)
	stage.ExpectedOutcome = strongestOutcomeClass([]string{stage.ExpectedOutcome, inferOutcome(strongestActionClass(stage.ActionClasses))})
	stage.BoundaryRefs = sortedUnique(stage.BoundaryRefs)
	return stage
}

func inferActionClass(value string) string {
	v := normalize(value)
	switch {
	case strings.Contains(v, "credential") || strings.Contains(v, "secret") || strings.Contains(v, "token"):
		return ActionClassCredentialAccess
	case strings.Contains(v, "deploy"):
		return ActionClassDeploy
	case strings.Contains(v, "release") || strings.Contains(v, "publish"):
		return ActionClassRelease
	case strings.Contains(v, "delete") || strings.Contains(v, "destroy") || strings.Contains(v, "remove"):
		return ActionClassDelete
	case strings.Contains(v, "egress") || strings.Contains(v, "network") || strings.Contains(v, "http") || strings.Contains(v, "send"):
		return ActionClassEgress
	case strings.Contains(v, "write") || strings.Contains(v, "create") || strings.Contains(v, "update") || strings.Contains(v, "mutat"):
		return ActionClassWrite
	case strings.Contains(v, "execute") || strings.Contains(v, "exec") || strings.Contains(v, "run"):
		return ActionClassExecute
	case strings.Contains(v, "read") || strings.Contains(v, "get") || strings.Contains(v, "list") || strings.Contains(v, "fetch"):
		return ActionClassRead
	default:
		return ""
	}
}

func inferTargetTrustClass(value string) string {
	v := normalize(value)
	switch {
	case strings.Contains(v, "production") || strings.Contains(v, "prod"):
		return "production"
	case strings.Contains(v, "privileged") || strings.Contains(v, "admin"):
		return "privileged"
	case strings.Contains(v, "external") || strings.Contains(v, "internet") || strings.Contains(v, "network"):
		return "external"
	case strings.Contains(v, "internal"):
		return "internal"
	default:
		return ""
	}
}

var actionRank = map[string]int{ActionClassRead: 1, ActionClassExecute: 2, ActionClassCredentialAccess: 3, ActionClassWrite: 4, ActionClassEgress: 5, ActionClassRelease: 6, ActionClassDeploy: 7, ActionClassDelete: 8, ActionClassResource: 2}

func strongestActionClass(values []string) string {
	best := ""
	rank := 0
	for _, value := range values {
		value = normalize(value)
		if r := actionRank[value]; r > rank || (r == rank && value != "" && value < best) {
			best, rank = value, r
		}
	}
	return best
}
func transitionForAction(action string) string {
	switch action {
	case ActionClassRead:
		return "read"
	case ActionClassWrite:
		return "write"
	case ActionClassDelete:
		return "delete"
	case ActionClassExecute:
		return "execute"
	case ActionClassDeploy, ActionClassRelease:
		return "deploy"
	case ActionClassEgress:
		return "egress"
	case ActionClassCredentialAccess:
		return "credential"
	default:
		return "none"
	}
}

var transitionRank = map[string]int{"none": 0, "read": 1, "execute": 2, "credential": 3, "write": 4, "egress": 5, "delegate": 5, "deploy": 6, "delete": 7, "resource_lifecycle": 3, "approval": 3, "control": 3}

func strongestTransitionClass(values []string) string { return strongestValue(values, transitionRank) }

var outcomeRank = map[string]int{"unknown": 0, "none": 0, "read": 1, "execute": 2, "write": 3, "resource_change": 3, "release": 4, "release_publish": 5, "network_egress": 5, "external_egress": 5, "data_egress": 6, "production_mutation": 7, "production_deploy": 8}

func strongestOutcomeClass(values []string) string { return strongestValue(values, outcomeRank) }

var targetTrustRank = map[string]int{"unknown": 0, "untrusted": 1, "external": 1, "internal": 2, "trusted": 3, "privileged": 4, "production": 5}

func strongestTargetTrustClass(values []string) string {
	return strongestValue(values, targetTrustRank)
}

var compositionRoleRank = map[string]int{"source": 1, "transform": 2, "sink": 3, "internal_sink": 3, "external_sink": 4, "privileged_sink": 5, "destructive_sink": 6}

func strongestCompositionRole(values []string) string {
	return strongestValue(values, compositionRoleRank)
}

func strongestValue(values []string, ranks map[string]int) string {
	best := ""
	bestRank := -1
	for _, value := range values {
		value = normalize(value)
		rank, ok := ranks[value]
		if !ok {
			continue
		}
		if rank > bestRank || (rank == bestRank && value != "" && value < best) {
			best, bestRank = value, rank
		}
	}
	return best
}
func inferOutcome(action string) string {
	switch action {
	case ActionClassEgress:
		return "network_egress"
	case ActionClassDeploy:
		return "production_deploy"
	case ActionClassRelease:
		return "release_publish"
	case ActionClassWrite, ActionClassDelete:
		return "production_mutation"
	case ActionClassRead:
		return "read"
	case ActionClassExecute:
		return "execute"
	default:
		return "unknown"
	}
}

// ReadinessStatus is per requirement.  A required inconclusive result is
// never promoted to satisfied.
type ReadinessStatus string

const (
	ReadinessSatisfied    ReadinessStatus = "satisfied"
	ReadinessUnsatisfied  ReadinessStatus = "unsatisfied"
	ReadinessInconclusive ReadinessStatus = "inconclusive"
	ReadinessNotRequired  ReadinessStatus = "not_required"
)

type ControlMode string

const (
	ControlModeEnforced     ControlMode = "enforced"
	ControlModeObserved     ControlMode = "observed"
	ControlModeSelfAttested ControlMode = "self_attested"
	ControlModeUnknown      ControlMode = "unknown"
)

type ReadinessPrecondition struct {
	RequirementID       string          `json:"requirement_id"`
	Kind                string          `json:"kind"`
	Required            bool            `json:"required"`
	RequiredConstraint  string          `json:"required_constraint,omitempty"`
	ContractRef         string          `json:"contract_ref,omitempty"`
	ObservedValue       string          `json:"observed_value,omitempty"`
	ObservedResult      string          `json:"observed_result,omitempty"`
	Producer            string          `json:"producer,omitempty"`
	AcceptableProducers []string        `json:"acceptable_producers,omitempty"`
	EvidenceState       string          `json:"evidence_state,omitempty"`
	FreshnessState      string          `json:"freshness_state,omitempty"`
	EvidenceRefs        []string        `json:"evidence_refs,omitempty"`
	BoundaryRefs        []string        `json:"boundary_refs,omitempty"`
	ControlMode         ControlMode     `json:"control_mode"`
	Status              ReadinessStatus `json:"status"`
	ReasonCodes         []string        `json:"reason_codes,omitempty"`
}

type ReadinessInput struct {
	ContractID           string                  `json:"contract_id,omitempty"`
	Preconditions        []ReadinessPrecondition `json:"preconditions"`
	TrustedValidatorRefs []string                `json:"trusted_validator_refs,omitempty"`
	PolicyDigest         string                  `json:"policy_digest,omitempty"`
	Now                  time.Time               `json:"-"`
}

type ReadinessResult struct {
	SchemaID      string                  `json:"schema_id"`
	SchemaVersion string                  `json:"schema_version"`
	ContractID    string                  `json:"contract_id,omitempty"`
	Ready         bool                    `json:"ready"`
	Status        ReadinessStatus         `json:"status"`
	Preconditions []ReadinessPrecondition `json:"preconditions"`
	ReasonCodes   []string                `json:"reason_codes,omitempty"`
}

// EvaluateReadiness applies only policy-named trusted validator references.
// Wrkr declarations and judge/self-attestation labels are intentionally not
// trusted by default.
func EvaluateReadiness(input ReadinessInput) ReadinessResult {
	trusted := make(map[string]struct{}, len(input.TrustedValidatorRefs))
	for _, ref := range input.TrustedValidatorRefs {
		trusted[normalize(ref)] = struct{}{}
	}
	out := ReadinessResult{SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, ContractID: strings.TrimSpace(input.ContractID), Preconditions: make([]ReadinessPrecondition, len(input.Preconditions))}
	anyUnsatisfied, anyInconclusive, requiredCount := false, false, 0
	for i, item := range input.Preconditions {
		item.RequirementID = strings.TrimSpace(item.RequirementID)
		item.Kind = normalize(item.Kind)
		item.Producer = normalize(item.Producer)
		item.ControlMode = ControlMode(normalize(string(item.ControlMode)))
		item.EvidenceState = normalize(item.EvidenceState)
		item.FreshnessState = normalize(item.FreshnessState)
		item.EvidenceRefs = sortedUnique(item.EvidenceRefs)
		item.BoundaryRefs = sortedUnique(item.BoundaryRefs)
		item.AcceptableProducers = sortedUnique(item.AcceptableProducers)
		item.ReasonCodes = nil
		if !item.Required {
			item.Status = ReadinessNotRequired
			out.Preconditions[i] = item
			continue
		}
		requiredCount++
		item.Status = ReadinessInconclusive
		if item.ControlMode == "" {
			item.ControlMode = ControlModeUnknown
		}
		if item.ControlMode != ControlModeEnforced && item.ControlMode != ControlModeObserved && item.ControlMode != ControlModeSelfAttested && item.ControlMode != ControlModeUnknown {
			item.ControlMode = ControlModeUnknown
		}
		if item.ControlMode == ControlModeSelfAttested || item.ControlMode == ControlModeUnknown {
			item.ReasonCodes = append(item.ReasonCodes, "validator:untrusted_control_mode")
		}
		if item.Producer == "" {
			item.ReasonCodes = append(item.ReasonCodes, "validator:missing")
		} else if _, ok := trusted[item.Producer]; !ok {
			item.ReasonCodes = append(item.ReasonCodes, "validator:not_policy_named")
		}
		if len(item.AcceptableProducers) > 0 && item.Producer != "" && !containsStringValue(item.AcceptableProducers, item.Producer) {
			item.ReasonCodes = append(item.ReasonCodes, "validator:producer_not_acceptable")
		}
		if item.FreshnessState == "stale" || item.FreshnessState == "expired" {
			item.ReasonCodes = append(item.ReasonCodes, "freshness:not_fresh")
		}
		if item.FreshnessState == "unknown" || item.FreshnessState == "" {
			item.ReasonCodes = append(item.ReasonCodes, "freshness:unknown")
		}
		if isFalseResult(item.ObservedResult) {
			item.Status = ReadinessUnsatisfied
			item.ReasonCodes = append(item.ReasonCodes, "result:unsatisfied")
		}
		if item.Kind == "validation_contract" || item.Kind == "effect_contract" {
			if item.ContractRef == "" {
				item.ReasonCodes = append(item.ReasonCodes, "contract:ref_missing")
			}
		}
		if (item.ControlMode == ControlModeEnforced || item.ControlMode == ControlModeObserved) && len(item.BoundaryRefs) == 0 && strings.Contains(item.Kind, "boundary") {
			item.ReasonCodes = append(item.ReasonCodes, "boundary:reference_missing")
		}
		if item.Kind == "policy_digest" {
			supplied := strings.TrimSpace(input.PolicyDigest)
			if supplied == "" {
				item.ReasonCodes = append(item.ReasonCodes, "policy_digest:missing")
			} else if item.ObservedValue != "" && strings.TrimSpace(item.ObservedValue) != supplied {
				item.Status = ReadinessUnsatisfied
				item.ReasonCodes = append(item.ReasonCodes, "policy_digest:mismatch")
			}
		}
		trustedProducer := item.Producer != ""
		if _, ok := trusted[item.Producer]; !ok {
			trustedProducer = false
		}
		fresh := item.FreshnessState == "fresh" || item.FreshnessState == "current"
		verifiedEvidence := item.EvidenceState == "verified" && strings.TrimSpace(item.ObservedResult) == ""
		matchingConstraint := strings.TrimSpace(item.RequiredConstraint) != "" && strings.EqualFold(strings.TrimSpace(item.RequiredConstraint), strings.TrimSpace(item.ObservedResult))
		if item.Status != ReadinessUnsatisfied && trustedProducer && fresh && (item.ControlMode == ControlModeEnforced || item.ControlMode == ControlModeObserved) && (isTrueResult(item.ObservedResult) || verifiedEvidence || matchingConstraint) {
			item.Status = ReadinessSatisfied
		}
		if item.Status == ReadinessSatisfied && len(item.ReasonCodes) > 0 {
			item.Status = ReadinessInconclusive
		}
		if item.Status == ReadinessUnsatisfied {
			anyUnsatisfied = true
		} else if item.Status != ReadinessSatisfied {
			anyInconclusive = true
		}
		item.ReasonCodes = sortedUnique(item.ReasonCodes)
		out.Preconditions[i] = item
	}
	if requiredCount == 0 {
		out.Status = ReadinessNotRequired
		out.Ready = true
	} else if anyUnsatisfied {
		out.Status = ReadinessUnsatisfied
	} else if anyInconclusive {
		out.Status = ReadinessInconclusive
	} else {
		out.Status = ReadinessSatisfied
		out.Ready = true
	}
	for _, item := range out.Preconditions {
		if item.Status != ReadinessSatisfied && item.Status != ReadinessNotRequired {
			out.ReasonCodes = append(out.ReasonCodes, "precondition:"+string(item.Status)+":"+item.Kind)
		}
	}
	out.ReasonCodes = sortedUnique(out.ReasonCodes)
	return out
}

// EvaluateContractReadiness is the stable name for callers evaluating a
// typed contract projection.
func EvaluateContractReadiness(input ReadinessInput) ReadinessResult { return EvaluateReadiness(input) }

func ReadinessFromArtifact(artifact Artifact, options ReadinessInput) ReadinessResult {
	return ReadinessFromContract(artifact.Contract, options)
}

func ReadinessFromContract(contract map[string]any, options ReadinessInput) ReadinessResult {
	if contract == nil {
		return ReadinessResult{SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, Ready: false, Status: ReadinessInconclusive, ReasonCodes: []string{"contract:missing"}, Preconditions: []ReadinessPrecondition{}}
	}
	options.ContractID = stringField(contract, "contract_id")
	options.Preconditions = nil
	for _, raw := range objectArray(contract, "preconditions") {
		contractRef := stringField(raw, "contract_ref")
		if contractRef == "" {
			contractRef = stringField(raw, "validation_contract_ref")
		}
		if contractRef == "" {
			contractRef = stringField(raw, "effect_contract_ref")
		}
		item := ReadinessPrecondition{RequirementID: stringField(raw, "requirement_id"), Kind: stringField(raw, "kind"), Required: true, RequiredConstraint: stringField(raw, "required_constraint"), ContractRef: contractRef, ObservedValue: stringField(raw, "observed_value"), ObservedResult: stringField(raw, "observed_result"), Producer: stringField(raw, "producer"), AcceptableProducers: stringArray(raw, "acceptable_producers"), EvidenceState: stringField(raw, "evidence_state"), FreshnessState: stringField(raw, "freshness_state"), EvidenceRefs: stringArray(raw, "evidence_refs"), BoundaryRefs: stringArray(raw, "boundary_refs"), ControlMode: ControlMode(stringField(raw, "control_mode"))}
		if value, ok := raw["required"].(bool); ok {
			item.Required = value
		}
		options.Preconditions = append(options.Preconditions, item)
	}
	return EvaluateReadiness(options)
}

// Lifecycle records are signed, digest-bound state transitions.  ReduceLifecycle
// is a pure reducer over records; it does not persist or append to an event
// store and it preserves the immutable proposal/activation references.
type LifecycleEventKind string

const (
	LifecycleProposalIngested      LifecycleEventKind = "proposal_ingested"
	LifecycleActivationRequested   LifecycleEventKind = "activation_requested"
	LifecycleActivated             LifecycleEventKind = "activated"
	LifecycleRejected              LifecycleEventKind = "rejected"
	LifecycleRevoked               LifecycleEventKind = "revoked"
	LifecycleSuperseded            LifecycleEventKind = "superseded"
	LifecyclePreconditionEvaluated LifecycleEventKind = "precondition_evaluated"
	LifecycleDecisionReady         LifecycleEventKind = "decision_ready"
)

type LifecycleRecord struct {
	SchemaID         string                                   `json:"schema_id"`
	SchemaVersion    string                                   `json:"schema_version"`
	RecordID         string                                   `json:"record_id"`
	Kind             LifecycleEventKind                       `json:"kind"`
	OccurredAt       string                                   `json:"occurred_at"`
	ContractRef      proof.RelationshipRef                    `json:"contract_ref"`
	ProposalRef      *proof.RelationshipRef                   `json:"proposal_ref,omitempty"`
	ActivationRef    *proof.RelationshipRef                   `json:"activation_ref,omitempty"`
	PreconditionRefs []proof.RelationshipRef                  `json:"precondition_refs,omitempty"`
	Decision         *ReadinessResult                         `json:"decision,omitempty"`
	ReasonCodes      []string                                 `json:"reason_codes,omitempty"`
	Correlation      proof.ControlContainmentTelemetryProfile `json:"correlation"`
	ImmutableObject  json.RawMessage                          `json:"immutable_object,omitempty"`
	Signature        proofsign.Signature                      `json:"signature"`
}

type LifecycleRecordOptions struct {
	Kind              LifecycleEventKind
	OccurredAt        time.Time
	ContractRef       proof.RelationshipRef
	ProposalRef       *proof.RelationshipRef
	ActivationRef     *proof.RelationshipRef
	PreconditionRefs  []proof.RelationshipRef
	Decision          *ReadinessResult
	ReasonCodes       []string
	Correlation       proof.ControlContainmentTelemetryProfile
	ImmutableObject   json.RawMessage
	SigningPrivateKey ed25519.PrivateKey
}

func NewLifecycleRecord(options LifecycleRecordOptions) (LifecycleRecord, error) {
	if strings.TrimSpace(string(options.Kind)) == "" {
		return LifecycleRecord{}, errors.New("lifecycle kind is required")
	}
	if strings.TrimSpace(options.ContractRef.Kind) == "" || strings.TrimSpace(options.ContractRef.ID) == "" {
		return LifecycleRecord{}, errors.New("contract reference is required")
	}
	correlation := options.Correlation
	if correlation.ProfileVersion == "" {
		correlation.ProfileVersion = CorrelationProfileVersion
	}
	if correlation.BindingMode == "" {
		correlation.BindingMode = proof.BindingModeIdentifierOnly
		if options.ContractRef.Digest != "" || (options.ProposalRef != nil && options.ProposalRef.Digest != "") || (options.ActivationRef != nil && options.ActivationRef.Digest != "") {
			correlation.BindingMode = proof.BindingModeDigestBound
		}
	}
	if correlation.ContractRef == nil {
		contractRef := options.ContractRef
		correlation.ContractRef = &contractRef
	}
	if correlation.ContentDigest == "" && options.ContractRef.Digest != "" {
		correlation.ContentDigest = options.ContractRef.Digest
	}
	record := LifecycleRecord{SchemaID: RuntimeLifecycleSchemaID, SchemaVersion: RuntimeLifecycleVersion, Kind: options.Kind, OccurredAt: options.OccurredAt.UTC().Format(time.RFC3339Nano), ContractRef: options.ContractRef, ProposalRef: options.ProposalRef, ActivationRef: options.ActivationRef, PreconditionRefs: append([]proof.RelationshipRef(nil), options.PreconditionRefs...), Decision: options.Decision, ReasonCodes: sortedUnique(options.ReasonCodes), Correlation: correlation, ImmutableObject: append([]byte(nil), options.ImmutableObject...)}
	digest, err := lifecycleDigest(record)
	if err != nil {
		return LifecycleRecord{}, err
	}
	record.RecordID = "gait-lr-" + strings.TrimPrefix(digest, "sha256:")[:16]
	if len(options.SigningPrivateKey) == 0 {
		return LifecycleRecord{}, errors.New("lifecycle signing key is required")
	}
	if len(options.SigningPrivateKey) != ed25519.PrivateKeySize {
		return LifecycleRecord{}, errors.New("lifecycle signing key has invalid size")
	}
	signature, err := proofsign.SignDigestHex(options.SigningPrivateKey, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return LifecycleRecord{}, err
	}
	record.Signature = signature
	return record, nil
}

func VerifyLifecycleRecord(record LifecycleRecord, publicKey ed25519.PublicKey) (bool, error) {
	if record.SchemaID != RuntimeLifecycleSchemaID || record.SchemaVersion != RuntimeLifecycleVersion || record.RecordID == "" {
		return false, errors.New("lifecycle schema or identity invalid")
	}
	digest, err := lifecycleDigest(record)
	if err != nil {
		return false, err
	}
	if record.Signature.SignedDigest != strings.TrimPrefix(digest, "sha256:") {
		return false, errors.New("lifecycle digest mismatch")
	}
	valid, verifyErr := proofsign.VerifyDigestHex(publicKey, record.Signature)
	if verifyErr != nil || !valid {
		return false, errors.New("lifecycle signature invalid")
	}
	return record.RecordID == "gait-lr-"+strings.TrimPrefix(digest, "sha256:")[:16], nil
}

type LifecycleSnapshot struct {
	ProposalIngested       bool              `json:"proposal_ingested"`
	ActivationRequested    bool              `json:"activation_requested"`
	Activated              bool              `json:"activated"`
	Rejected               bool              `json:"rejected"`
	Revoked                bool              `json:"revoked"`
	Superseded             bool              `json:"superseded"`
	DecisionReady          bool              `json:"decision_ready"`
	PreconditionsEvaluated int               `json:"preconditions_evaluated"`
	CurrentStatus          string            `json:"current_status"`
	ReasonCodes            []string          `json:"reason_codes,omitempty"`
	Records                []LifecycleRecord `json:"records"`
}

func ReduceLifecycle(records []LifecycleRecord) LifecycleSnapshot {
	ordered := append([]LifecycleRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].OccurredAt != ordered[j].OccurredAt {
			return ordered[i].OccurredAt < ordered[j].OccurredAt
		}
		return ordered[i].RecordID < ordered[j].RecordID
	})
	out := LifecycleSnapshot{Records: ordered}
	for _, record := range ordered {
		switch record.Kind {
		case LifecycleProposalIngested:
			out.ProposalIngested = true
		case LifecycleActivationRequested:
			out.ActivationRequested = true
		case LifecycleActivated:
			out.Activated = true
			out.Rejected = false
		case LifecycleRejected:
			out.Rejected = true
			out.Activated = false
		case LifecycleRevoked:
			out.Revoked = true
			out.Activated = false
		case LifecycleSuperseded:
			out.Superseded = true
			out.Activated = false
		case LifecyclePreconditionEvaluated:
			out.PreconditionsEvaluated += len(record.PreconditionRefs)
		case LifecycleDecisionReady:
			out.DecisionReady = record.Decision == nil || record.Decision.Ready
		}
		out.ReasonCodes = append(out.ReasonCodes, record.ReasonCodes...)
	}
	switch {
	case out.Revoked:
		out.CurrentStatus = "revoked"
	case out.Superseded:
		out.CurrentStatus = "superseded"
	case out.Rejected:
		out.CurrentStatus = "rejected"
	case out.Activated:
		out.CurrentStatus = "activated"
	case out.DecisionReady:
		out.CurrentStatus = "ready"
	case out.ProposalIngested:
		out.CurrentStatus = "ingested"
	default:
		out.CurrentStatus = "unknown"
	}
	out.ReasonCodes = sortedUnique(out.ReasonCodes)
	return out
}

// ReduceLifecycleEvents is a compatibility alias with the event-oriented
// wording used by integrations; both names remain pure reducers.
func ReduceLifecycleEvents(records []LifecycleRecord) LifecycleSnapshot {
	return ReduceLifecycle(records)
}

func lifecycleDigest(record LifecycleRecord) (string, error) {
	copy := record
	copy.RecordID = ""
	copy.Signature = proofsign.Signature{}
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	return proofcanon.DigestJCS(raw)
}

func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
func normalizeActionClasses(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = normalize(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return sortedUnique(out)
}
func containsStringValue(values []string, wanted string) bool {
	for _, value := range values {
		if normalize(value) == normalize(wanted) {
			return true
		}
	}
	return false
}
func isTrueResult(value string) bool {
	switch normalize(value) {
	case "true", "yes", "pass", "passed", "ok", "satisfied", "verified", "enforced":
		return true
	}
	return false
}
func isFalseResult(value string) bool {
	switch normalize(value) {
	case "false", "no", "fail", "failed", "blocked", "unsatisfied", "contradictory":
		return true
	}
	return false
}
