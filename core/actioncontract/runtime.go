package actioncontract

// This file contains the runtime-facing, pre-execution portion of the Action
// Contract.  It deliberately does not execute tools, issue credentials, or
// assert that an effect occurred.  Wrkr's proposed artifact remains the
// source of intended outcome data; this model gives Gait a small, versioned
// and deterministic projection for boundary/readiness decisions.

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
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
	RuntimeActionSchemaID              = "https://gait.dev/schemas/v1/runtime-action.schema.json"
	RuntimeActionSchemaVersion         = "1"
	RuntimeClassificationInputSchemaID = "https://gait.dev/schemas/v1/runtime-classification-input.schema.json"
	RuntimeClassificationInputVersion  = "1"
	RuntimeReadinessSchemaID           = "https://gait.dev/schemas/v1/runtime-readiness.schema.json"
	RuntimeLifecycleSchemaID           = "https://gait.dev/schemas/v1/runtime-lifecycle-record.schema.json"
	RuntimeLifecycleVersion            = "1"
	ProofCompatibilityVersion          = "0.6.1"
	CorrelationProfileVersion          = "1.0"
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
	ActionClassCredentialAccess = "credential_access" // #nosec G101 -- fixed classification vocabulary, not a credential.
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
	SchemaID                 string               `json:"schema_id"`
	SchemaVersion            string               `json:"schema_version"`
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

func ParseClassificationInput(raw []byte) (ClassificationInput, error) {
	var input ClassificationInput
	if err := DecodeStrictRuntimeJSON(raw, &input); err != nil {
		return ClassificationInput{}, err
	}
	if err := validateRuntimeSchema(raw, RuntimeClassificationInputSchemaID); err != nil {
		return ClassificationInput{}, errors.New("classification input schema invalid")
	}
	return input, nil
}

// ParseRuntimeAction parses the documented runtime-action artifact emitted as
// the classification.action object. It is distinct from ClassificationInput,
// which is the heuristic/raw-input surface used with --input.
func ParseRuntimeAction(raw []byte) (RuntimeAction, error) {
	if err := validateRuntimeSchema(raw, RuntimeActionSchemaID); err != nil {
		return RuntimeAction{}, errors.New("runtime action schema invalid")
	}
	var action RuntimeAction
	if err := DecodeStrictRuntimeJSON(raw, &action); err != nil {
		return RuntimeAction{}, err
	}
	if reasons := ValidateRuntimeAction(action); len(reasons) > 0 {
		return RuntimeAction{}, fmt.Errorf("runtime action schema invalid: %s", strings.Join(reasons, ","))
	}
	return action, nil
}

func ParseReadinessInput(raw []byte) (ReadinessInput, error) {
	var input ReadinessInput
	if err := DecodeStrictRuntimeJSON(raw, &input); err != nil {
		return ReadinessInput{}, err
	}
	return input, nil
}

// Compatibility aliases keep the public vocabulary explicit for callers that
// refer to the three projections independently.
type ActionClassification = RuntimeAction
type BoundaryClassification = RuntimeBoundary
type OutcomeClassification = ObservedEffect

// ClassifyRuntimeAction deterministically normalizes a runtime action.  It
// does not inspect the network, run validators, or infer observed effects.
func ClassifyRuntimeAction(input ClassificationInput) ClassificationResult {
	explicitReasons := validateExplicitClassificationInput(input)
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
	reasons := append(explicitReasons, ValidateRuntimeAction(action)...)
	if stagesExceeded {
		reasons = append(reasons, "stages_limit_exceeded")
	}
	action.ClassificationReasons = sortedUnique(append(action.ClassificationReasons, reasons...))
	return ClassificationResult{Action: action, Valid: len(reasons) == 0, ReasonCodes: reasons}
}

func validateExplicitClassificationInput(input ClassificationInput) []string {
	reasons := []string{}
	if value := normalize(input.ActionClass); value != "" {
		if _, ok := runtimeActionClasses[value]; !ok {
			reasons = append(reasons, "action_class_explicit_unsupported:"+value)
		}
	}
	for _, value := range input.ActionClasses {
		value = normalize(value)
		if value != "" {
			if _, ok := runtimeActionClasses[value]; !ok {
				reasons = append(reasons, "action_class_explicit_unsupported:"+value)
			}
		}
	}
	if value := normalize(input.CompositionRole); value != "" {
		if _, ok := runtimeCompositionRoles[value]; !ok {
			reasons = append(reasons, "composition_role_explicit_unsupported:"+value)
		}
	}
	if value := normalize(input.TargetTrustClass); value != "" {
		if _, ok := runtimeTrustClasses[value]; !ok {
			reasons = append(reasons, "target_trust_class_explicit_unsupported:"+value)
		}
	}
	if value := normalize(input.TransitionClass); value != "" {
		if _, ok := runtimeTransitionClasses[value]; !ok {
			reasons = append(reasons, "transition_class_explicit_unsupported:"+value)
		}
	}
	for field, value := range map[string]string{"expected_outcome_class": input.ExpectedOutcomeClass, "intended_outcome_class": input.IntendedOutcomeClass} {
		value = normalize(value)
		if value != "" {
			if _, ok := runtimeOutcomeClasses[value]; !ok {
				reasons = append(reasons, field+"_explicit_unsupported:"+value)
			}
		}
	}
	for _, value := range input.DataClasses {
		value = normalize(value)
		if value != "" {
			if _, ok := runtimeDataClasses[value]; !ok {
				reasons = append(reasons, "data_class_explicit_unsupported:"+value)
			}
		}
	}
	return sortedUnique(reasons)
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
	if raw, err := json.Marshal(action); err != nil || validateRuntimeSchema(raw, RuntimeActionSchemaID) != nil {
		reasons = append(reasons, "schema_validation_failed")
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
	ObservedAt          string          `json:"observed_at,omitempty"`
	MaxAgeSeconds       int64           `json:"max_age_seconds,omitempty"`
	TTLSeconds          int64           `json:"ttl_seconds,omitempty"`
	Environment         string          `json:"environment,omitempty"`
	Target              string          `json:"target,omitempty"`
	SandboxStatus       string          `json:"sandbox_status,omitempty"`
	CredentialMode      string          `json:"credential_mode,omitempty"`
	ResourceStatus      string          `json:"resource_status,omitempty"`
	CompensationStatus  string          `json:"compensation_status,omitempty"`
	EvidenceDigest      string          `json:"evidence_digest,omitempty"`
	ValidatorSignature  string          `json:"validator_signature,omitempty"`
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
	ContractID           string                       `json:"contract_id,omitempty"`
	Preconditions        []ReadinessPrecondition      `json:"preconditions"`
	TrustedValidatorRefs []string                     `json:"trusted_validator_refs,omitempty"`
	PolicyDigest         string                       `json:"policy_digest,omitempty"`
	Now                  time.Time                    `json:"-"`
	TrustedValidatorKeys map[string]ed25519.PublicKey `json:"-"`
}

type ReadinessResult struct {
	SchemaID      string                  `json:"schema_id"`
	SchemaVersion string                  `json:"schema_version"`
	ContractID    string                  `json:"contract_id,omitempty"`
	PolicyDigest  string                  `json:"policy_digest,omitempty"`
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
	out := ReadinessResult{SchemaID: RuntimeReadinessSchemaID, SchemaVersion: RuntimeActionSchemaVersion, ContractID: strings.TrimSpace(input.ContractID), PolicyDigest: strings.TrimSpace(input.PolicyDigest), Preconditions: make([]ReadinessPrecondition, len(input.Preconditions))}
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
		if item.ControlMode == "" {
			item.ControlMode = ControlModeUnknown
		}
		if !item.Required {
			item.Status = ReadinessNotRequired
			out.Preconditions[i] = item
			continue
		}
		requiredCount++
		item.Status = ReadinessInconclusive
		if item.ControlMode != ControlModeEnforced && item.ControlMode != ControlModeObserved && item.ControlMode != ControlModeSelfAttested && item.ControlMode != ControlModeUnknown {
			item.ControlMode = ControlModeUnknown
		}
		if input.Now.IsZero() {
			item.ReasonCodes = append(item.ReasonCodes, "evaluation_time:required")
		}
		if item.RequirementID == "" || item.Kind == "" {
			item.ReasonCodes = append(item.ReasonCodes, "precondition:identity_missing")
		}
		if item.ControlMode == ControlModeSelfAttested || item.ControlMode == ControlModeUnknown {
			item.ReasonCodes = append(item.ReasonCodes, "validator:untrusted_control_mode")
		}
		if item.Producer == "" {
			item.ReasonCodes = append(item.ReasonCodes, "validator:missing")
		} else if forbiddenReadinessProducer(item.Producer) {
			item.ReasonCodes = append(item.ReasonCodes, "validator:declaration_only")
		} else if _, ok := trusted[item.Producer]; !ok {
			item.ReasonCodes = append(item.ReasonCodes, "validator:not_policy_named")
		}
		claimIdentityValid := strings.TrimSpace(input.ContractID) != "" && validSHA256Digest(input.PolicyDigest)
		if strings.TrimSpace(input.ContractID) == "" {
			item.ReasonCodes = append(item.ReasonCodes, "claim:contract_identity_missing")
		}
		if !validSHA256Digest(input.PolicyDigest) {
			item.ReasonCodes = append(item.ReasonCodes, "claim:policy_digest_missing")
		}
		claimDigest, claimDigestErr := CanonicalReadinessClaimDigest(input, item)
		claimDigestMatches := claimDigestErr == nil && strings.TrimSpace(item.EvidenceDigest) == claimDigest
		if item.ValidatorSignature == "" || item.EvidenceDigest == "" {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:authoritative_signature_missing")
		} else if !validSHA256Digest(item.EvidenceDigest) {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:digest_invalid")
		} else if claimDigestErr != nil || !claimDigestMatches {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:claim_digest_mismatch")
		} else if key, ok := input.TrustedValidatorKeys[item.Producer]; !ok || len(key) != ed25519.PublicKeySize {
			item.ReasonCodes = append(item.ReasonCodes, "validator:public_key_missing")
		} else if !verifyEvidenceSignature(key, item.EvidenceDigest, item.ValidatorSignature) {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:signature_invalid")
		}
		if len(item.EvidenceRefs) == 0 {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:refs_missing")
		}
		if item.EvidenceState != "verified" {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:not_verified")
		}
		if item.ObservedResult == "" && item.ObservedValue == "" {
			item.ReasonCodes = append(item.ReasonCodes, "evidence:result_missing")
		}
		if len(item.AcceptableProducers) > 0 && item.Producer != "" && !containsStringValue(item.AcceptableProducers, item.Producer) {
			item.ReasonCodes = append(item.ReasonCodes, "validator:producer_not_acceptable")
		}
		if err := validateFreshness(item, input.Now); err != "" {
			item.ReasonCodes = append(item.ReasonCodes, err)
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
		item.ReasonCodes = append(item.ReasonCodes, validateReadinessKind(item)...)
		if item.Kind == "policy_digest" {
			supplied := strings.TrimSpace(input.PolicyDigest)
			if !validSHA256Digest(supplied) {
				item.ReasonCodes = append(item.ReasonCodes, "policy_digest:missing")
			} else if !validSHA256Digest(strings.TrimSpace(item.ObservedValue)) || strings.TrimSpace(item.ObservedValue) != supplied {
				item.Status = ReadinessUnsatisfied
				item.ReasonCodes = append(item.ReasonCodes, "policy_digest:mismatch")
			}
		}
		trustedProducer := item.Producer != "" && !forbiddenReadinessProducer(item.Producer)
		if _, ok := trusted[item.Producer]; !ok {
			trustedProducer = false
		}
		fresh := item.FreshnessState == "fresh" || item.FreshnessState == "current"
		qualifying := claimIdentityValid && item.EvidenceState == "verified" && len(item.EvidenceRefs) > 0 && item.ValidatorSignature != "" && item.EvidenceDigest != "" && claimDigestMatches && verifyEvidenceSignature(input.TrustedValidatorKeys[item.Producer], item.EvidenceDigest, item.ValidatorSignature)
		matchingConstraint := strings.TrimSpace(item.RequiredConstraint) != "" && strings.EqualFold(strings.TrimSpace(item.RequiredConstraint), strings.TrimSpace(item.ObservedResult))
		if item.Status != ReadinessUnsatisfied && len(item.ReasonCodes) == 0 && trustedProducer && fresh && qualifying && (item.ControlMode == ControlModeEnforced || item.ControlMode == ControlModeObserved) && (isTrueResult(item.ObservedResult) || matchingConstraint) {
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
		// Absence of required evidence is not an authorization. A caller must
		// provide the typed contract projection (or a separately modeled,
		// cryptographically verified no-preconditions attestation) before a
		// readiness result can be authoritative.
		out.Ready = false
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
	if raw, err := json.Marshal(out); err != nil || validateRuntimeSchema(raw, RuntimeReadinessSchemaID) != nil {
		out.Ready = false
		out.Status = ReadinessInconclusive
		out.ReasonCodes = sortedUnique(append(out.ReasonCodes, "schema_validation_failed"))
	}
	return out
}

func forbiddenReadinessProducer(value string) bool {
	switch normalize(value) {
	case "wrkr", "judge", "advisory", "self", "self_attested", "declaration", "declaration_only":
		return true
	default:
		return false
	}
}

func validateFreshness(item ReadinessPrecondition, now time.Time) string {
	if item.ObservedAt == "" {
		return "freshness:timestamp_missing"
	}
	observed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(item.ObservedAt))
	if err != nil {
		return "freshness:timestamp_invalid"
	}
	if now.IsZero() || item.MaxAgeSeconds <= 0 {
		return "freshness:max_age_missing"
	}
	age := now.UTC().Sub(observed.UTC())
	if age < 0 || age > time.Duration(item.MaxAgeSeconds)*time.Second {
		return "freshness:not_fresh"
	}
	if item.FreshnessState != "fresh" && item.FreshnessState != "current" {
		return "freshness:not_fresh"
	}
	return ""
}

func validateReadinessKind(item ReadinessPrecondition) []string {
	reasons := []string{}
	if _, supported := supportedReadinessKinds[item.Kind]; !supported {
		return []string{"precondition:kind_unsupported"}
	}
	switch item.Kind {
	case "environment":
		if strings.TrimSpace(item.Environment) == "" {
			reasons = append(reasons, "environment:missing")
		}
		if reason := requiredReadinessValue(item, item.Environment, "environment"); reason != "" {
			reasons = append(reasons, reason)
		}
	case "target":
		if strings.TrimSpace(item.Target) == "" {
			reasons = append(reasons, "target:missing")
		}
		if reason := requiredReadinessValue(item, item.Target, "target"); reason != "" {
			reasons = append(reasons, reason)
		}
	case "sandbox", "sandbox_control":
		if !containsStringValue([]string{"clean", "isolated"}, item.SandboxStatus) {
			reasons = append(reasons, "sandbox:not_clean")
		}
	case "credential_mode", "credential":
		if !containsStringValue([]string{"ephemeral", "scoped", "standing_or_unknown"}, item.CredentialMode) {
			reasons = append(reasons, "credential:mode_missing")
		}
		if reason := requiredReadinessValue(item, item.CredentialMode, "credential_mode"); reason != "" {
			reasons = append(reasons, reason)
		}
	case "resource_budget":
		if !containsStringValue([]string{"budget_ok", "within_budget"}, item.ResourceStatus) {
			reasons = append(reasons, "resource:budget_invalid")
		}
		if item.TTLSeconds <= 0 {
			reasons = append(reasons, "resource:ttl_missing")
		}
	case "cleanup", "resource_cleanup":
		if !containsStringValue([]string{"clean", "completed"}, item.ResourceStatus) {
			reasons = append(reasons, "resource:cleanup_incomplete")
		}
	case "resource", "resource_lifecycle":
		if !containsStringValue([]string{"clean", "budget_ok", "within_budget", "released", "verified"}, item.ResourceStatus) {
			reasons = append(reasons, "resource:not_clean")
		}
		if item.TTLSeconds <= 0 {
			reasons = append(reasons, "resource:ttl_missing")
		}
		if reason := requiredReadinessValue(item, item.ResourceStatus, "resource"); reason != "" {
			reasons = append(reasons, reason)
		}
	case "resource_ttl":
		if item.TTLSeconds <= 0 {
			reasons = append(reasons, "resource:ttl_missing")
		}
	case "compensation", "compensation_contract":
		if !containsStringValue([]string{"ready", "verified"}, item.CompensationStatus) {
			reasons = append(reasons, "compensation:not_ready")
		}
	}
	return reasons
}

var supportedReadinessKinds = map[string]struct{}{
	"originating_intent": {}, "requester_identity": {}, "business_owner": {},
	"affected_system_owner": {}, "permitted_agent_role": {}, "policy_authority": {},
	"delegation_root": {}, "credential_subject_constraint": {}, "separation_of_duties": {},
	"validation_contract": {}, "effect_contract": {}, "required_check": {}, "producer": {},
	"freshness": {}, "environment": {}, "target": {}, "sandbox": {}, "sandbox_control": {},
	"policy_digest": {}, "credential_mode": {}, "credential": {}, "expected_effect": {},
	"forbidden_effect": {}, "confirmation": {}, "approval": {}, "compensation": {},
	"compensation_contract": {}, "resource": {}, "resource_lifecycle": {}, "resource_budget": {},
	"resource_ttl": {}, "cleanup": {}, "resource_cleanup": {},
}

func requiredReadinessValue(item ReadinessPrecondition, observed, kind string) string {
	constraint := strings.TrimSpace(item.RequiredConstraint)
	if constraint == "" {
		return ""
	}
	parts := strings.SplitN(constraint, ":", 2)
	expected := constraint
	if len(parts) == 2 && normalize(parts[0]) == normalize(kind) {
		expected = strings.TrimSpace(parts[1])
	}
	if expected == "" || normalize(expected) == "declared" || normalize(expected) == "required" || normalize(expected) == "bounded" {
		return ""
	}
	if normalize(observed) != normalize(expected) {
		return kind + ":constraint_mismatch"
	}
	return ""
}

func validSHA256Digest(value string) bool {
	value = strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// CanonicalReadinessClaimDigest binds a precondition to the contract and
// policy identity under which it was evaluated. This prevents a valid
// validator claim from being replayed for another contract or policy. The
// digest and signature fields, along with derived status/reason fields, are
// excluded so the signature binds the semantic claim rather than caller-
// supplied verification metadata or evaluator output.
func CanonicalReadinessClaimDigest(input ReadinessInput, item ReadinessPrecondition) (string, error) {
	if strings.TrimSpace(input.ContractID) == "" {
		return "", errors.New("claim contract identity is required")
	}
	if !validSHA256Digest(input.PolicyDigest) {
		return "", errors.New("claim policy digest is required")
	}
	item = normalizeReadinessClaim(item)
	envelope := struct {
		ContractID   string                `json:"contract_id"`
		PolicyDigest string                `json:"policy_digest"`
		Precondition ReadinessPrecondition `json:"precondition"`
	}{
		ContractID: strings.TrimSpace(input.ContractID), PolicyDigest: strings.TrimSpace(input.PolicyDigest), Precondition: item,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(raw)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(digest, "sha256:") {
		return digest, nil
	}
	return "sha256:" + digest, nil
}

func normalizeReadinessClaim(item ReadinessPrecondition) ReadinessPrecondition {
	item.RequirementID = strings.TrimSpace(item.RequirementID)
	item.Kind = normalize(item.Kind)
	item.RequiredConstraint = normalize(item.RequiredConstraint)
	item.ContractRef = strings.TrimSpace(item.ContractRef)
	item.ObservedValue = strings.TrimSpace(item.ObservedValue)
	item.ObservedResult = normalize(item.ObservedResult)
	item.ObservedAt = strings.TrimSpace(item.ObservedAt)
	item.Environment = strings.TrimSpace(item.Environment)
	item.Target = strings.TrimSpace(item.Target)
	item.SandboxStatus = normalize(item.SandboxStatus)
	item.CredentialMode = normalize(item.CredentialMode)
	item.ResourceStatus = normalize(item.ResourceStatus)
	item.CompensationStatus = normalize(item.CompensationStatus)
	item.Producer = normalize(item.Producer)
	item.AcceptableProducers = normalizeReadinessValues(item.AcceptableProducers)
	item.EvidenceState = normalize(item.EvidenceState)
	item.FreshnessState = normalize(item.FreshnessState)
	item.EvidenceRefs = sortedUnique(item.EvidenceRefs)
	item.BoundaryRefs = sortedUnique(item.BoundaryRefs)
	item.ControlMode = ControlMode(normalize(string(item.ControlMode)))
	if item.ControlMode == "" {
		item.ControlMode = ControlModeUnknown
	}
	item.EvidenceDigest = ""
	item.ValidatorSignature = ""
	item.Status = ""
	item.ReasonCodes = nil
	return item
}

func normalizeReadinessValues(values []string) []string {
	values = append([]string(nil), values...)
	for i := range values {
		values[i] = normalize(values[i])
	}
	return sortedUnique(values)
}

func verifyEvidenceSignature(publicKey ed25519.PublicKey, digest, encodedSignature string) bool {
	if len(publicKey) != ed25519.PublicKeySize || !validSHA256Digest(digest) {
		return false
	}
	rawSignature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSignature))
	if err != nil || len(rawSignature) != ed25519.SignatureSize {
		return false
	}
	digestBytes, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(digest), "sha256:"))
	return err == nil && len(digestBytes) == 32 && ed25519.Verify(publicKey, digestBytes, rawSignature)
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
		options.Preconditions = append(options.Preconditions, readinessPreconditionFromContract(raw, ""))
	}
	for _, raw := range objectArray(contract, "authority_requirements") {
		item := readinessPreconditionFromContract(raw, "authority:"+stringField(raw, "kind"))
		item.Producer = "authority"
		options.Preconditions = append(options.Preconditions, item)
	}
	if credentialMode := stringField(contract, "required_credential_mode"); credentialMode != "" {
		options.Preconditions = append(options.Preconditions, ReadinessPrecondition{RequirementID: "required_credential_mode", Kind: "credential_mode", Required: true, RequiredConstraint: "credential_mode:" + credentialMode})
	}
	for _, field := range []string{"confirmation_requirement", "approval_requirement"} {
		if raw := objectValue(contract, field); raw != nil {
			item := readinessPreconditionFromContract(raw, strings.TrimSuffix(field, "_requirement"))
			options.Preconditions = append(options.Preconditions, item)
		}
	}
	if raw := objectValue(contract, "compensation_requirement"); raw != nil {
		item := readinessPreconditionFromContract(raw, "compensation")
		item.Target = stringField(raw, "target")
		item.CompensationStatus = stringField(raw, "status")
		options.Preconditions = append(options.Preconditions, item)
	}
	return EvaluateReadiness(options)
}

func readinessPreconditionFromContract(raw map[string]any, defaultKind string) ReadinessPrecondition {
	contractRef := stringField(raw, "contract_ref")
	if contractRef == "" {
		contractRef = stringField(raw, "validation_contract_ref")
	}
	if contractRef == "" {
		contractRef = stringField(raw, "effect_contract_ref")
	}
	item := ReadinessPrecondition{RequirementID: stringField(raw, "requirement_id"), Kind: stringField(raw, "kind"), Required: true, RequiredConstraint: stringField(raw, "required_constraint"), ContractRef: contractRef, ObservedValue: stringField(raw, "observed_value"), ObservedResult: stringField(raw, "observed_result"), ObservedAt: stringField(raw, "observed_at"), MaxAgeSeconds: int64Field(raw, "max_age_seconds"), TTLSeconds: int64Field(raw, "ttl_seconds"), Environment: stringField(raw, "environment"), Target: stringField(raw, "target"), SandboxStatus: stringField(raw, "sandbox_status"), CredentialMode: stringField(raw, "credential_mode"), ResourceStatus: stringField(raw, "resource_status"), CompensationStatus: stringField(raw, "compensation_status"), EvidenceDigest: stringField(raw, "evidence_digest"), ValidatorSignature: stringField(raw, "validator_signature"), Producer: stringField(raw, "producer"), AcceptableProducers: stringArray(raw, "acceptable_producers"), EvidenceState: stringField(raw, "evidence_state"), FreshnessState: stringField(raw, "freshness_state"), EvidenceRefs: stringArray(raw, "evidence_refs"), BoundaryRefs: stringArray(raw, "boundary_refs"), ControlMode: ControlMode(stringField(raw, "control_mode"))}
	if item.Kind == "" {
		item.Kind = defaultKind
	}
	if value, ok := raw["required"].(bool); ok {
		item.Required = value
	}
	return item
}

func objectValue(object map[string]any, key string) map[string]any {
	value, ok := object[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
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
	LifecycleExecutionStarted      LifecycleEventKind = "execution_started"
	LifecycleExecutionSucceeded    LifecycleEventKind = "execution_succeeded"
	LifecycleExecutionFailed       LifecycleEventKind = "execution_failed"
	LifecycleExecutionBlocked      LifecycleEventKind = "execution_blocked"
	LifecycleEffectRecorded        LifecycleEventKind = "effect_recorded"
	LifecycleEffectValidated       LifecycleEventKind = "effect_validated"
	LifecycleContainmentRequested  LifecycleEventKind = "containment_requested"
	LifecycleContainmentCompleted  LifecycleEventKind = "containment_completed"
	LifecycleContainmentPartial    LifecycleEventKind = "containment_partial"
	LifecycleContainmentUnresolved LifecycleEventKind = "containment_unresolved"
	LifecycleCompensationRequired  LifecycleEventKind = "compensation_required"
	LifecycleCompensationStarted   LifecycleEventKind = "compensation_started"
	LifecycleCompensationCompleted LifecycleEventKind = "compensation_completed"
)

type LifecycleRecord struct {
	SchemaID         string                                   `json:"schema_id"`
	SchemaVersion    string                                   `json:"schema_version"`
	RecordID         string                                   `json:"record_id"`
	Kind             LifecycleEventKind                       `json:"kind"`
	OccurredAt       string                                   `json:"occurred_at"`
	ContractRef      proof.RelationshipRef                    `json:"contract_ref"`
	ContractFamilyID string                                   `json:"contract_family_id,omitempty"`
	Revision         int                                      `json:"revision"`
	ProposalRef      *proof.RelationshipRef                   `json:"proposal_ref,omitempty"`
	ActivationRef    *proof.RelationshipRef                   `json:"activation_ref,omitempty"`
	PreconditionRefs []proof.RelationshipRef                  `json:"precondition_refs,omitempty"`
	Decision         *ReadinessResult                         `json:"decision,omitempty"`
	EvidenceRefs     []proof.RelationshipRef                  `json:"evidence_refs,omitempty"`
	Execution        *ExecutionEvidence                       `json:"execution,omitempty"`
	Effect           *EffectEvent                             `json:"effect,omitempty"`
	Containment      *ContainmentEvidence                     `json:"containment,omitempty"`
	Compensation     *CompensationEvidence                    `json:"compensation,omitempty"`
	ReasonCodes      []string                                 `json:"reason_codes,omitempty"`
	Correlation      proof.ControlContainmentTelemetryProfile `json:"correlation"`
	ImmutableObject  json.RawMessage                          `json:"immutable_object,omitempty"`
	Signature        proofsign.Signature                      `json:"signature"`
}

type LifecycleRecordOptions struct {
	Kind              LifecycleEventKind
	OccurredAt        time.Time
	ContractRef       proof.RelationshipRef
	ContractFamilyID  string
	Revision          int
	ProposalRef       *proof.RelationshipRef
	ActivationRef     *proof.RelationshipRef
	PreconditionRefs  []proof.RelationshipRef
	Decision          *ReadinessResult
	EvidenceRefs      []proof.RelationshipRef
	Execution         *ExecutionEvidence
	Effect            *EffectEvent
	Containment       *ContainmentEvidence
	Compensation      *CompensationEvidence
	ReasonCodes       []string
	Correlation       proof.ControlContainmentTelemetryProfile
	ImmutableObject   json.RawMessage
	SigningPrivateKey ed25519.PrivateKey
}

func NewLifecycleRecord(options LifecycleRecordOptions) (LifecycleRecord, error) {
	if strings.TrimSpace(string(options.Kind)) == "" {
		return LifecycleRecord{}, errors.New("lifecycle kind is required")
	}
	if !validLifecycleRef(options.ContractRef) {
		return LifecycleRecord{}, errors.New("digest-bound contract reference is required")
	}
	if options.Revision < 1 {
		return LifecycleRecord{}, errors.New("lifecycle revision is required")
	}
	correlation := options.Correlation
	if correlation.ProfileVersion == "" {
		correlation.ProfileVersion = CorrelationProfileVersion
	}
	if correlation.BindingMode == "" {
		correlation.BindingMode = proof.BindingModeDigestBound
	} else if correlation.BindingMode != proof.BindingModeDigestBound {
		return LifecycleRecord{}, errors.New("lifecycle records require digest_bound correlation")
	}
	if correlation.ContractRef == nil {
		contractRef := options.ContractRef
		correlation.ContractRef = &contractRef
	}
	if correlation.ContentDigest == "" && options.ContractRef.Digest != "" {
		correlation.ContentDigest = options.ContractRef.Digest
	}
	decision := options.Decision
	if decision != nil {
		copyDecision := *decision
		if copyDecision.SchemaID == "" {
			copyDecision.SchemaID = RuntimeReadinessSchemaID
		}
		if copyDecision.SchemaVersion == "" {
			copyDecision.SchemaVersion = RuntimeActionSchemaVersion
		}
		if copyDecision.Preconditions == nil {
			copyDecision.Preconditions = []ReadinessPrecondition{}
		}
		decision = &copyDecision
	}
	record := LifecycleRecord{SchemaID: RuntimeLifecycleSchemaID, SchemaVersion: RuntimeLifecycleVersion, Kind: options.Kind, OccurredAt: options.OccurredAt.UTC().Format(time.RFC3339Nano), ContractRef: options.ContractRef, ContractFamilyID: strings.TrimSpace(options.ContractFamilyID), Revision: options.Revision, ProposalRef: options.ProposalRef, ActivationRef: options.ActivationRef, PreconditionRefs: append([]proof.RelationshipRef(nil), options.PreconditionRefs...), Decision: decision, EvidenceRefs: append([]proof.RelationshipRef(nil), options.EvidenceRefs...), Execution: options.Execution, Effect: options.Effect, Containment: options.Containment, Compensation: options.Compensation, ReasonCodes: sortedUnique(options.ReasonCodes), Correlation: correlation, ImmutableObject: append([]byte(nil), options.ImmutableObject...)}
	if record.Execution != nil {
		record.EvidenceRefs = append(record.EvidenceRefs, evidenceRefForExecution(*record.Execution))
	}
	if record.Effect != nil {
		record.EvidenceRefs = append(record.EvidenceRefs, evidenceRefForEffect(*record.Effect))
	}
	if record.Containment != nil {
		record.EvidenceRefs = append(record.EvidenceRefs, evidenceRefForContainment(*record.Containment))
	}
	if record.Compensation != nil {
		record.EvidenceRefs = append(record.EvidenceRefs, evidenceRefForCompensation(*record.Compensation))
	}
	if options.OccurredAt.IsZero() {
		return LifecycleRecord{}, errors.New("lifecycle timestamp is required")
	}
	if err := record.Correlation.Validate(); err != nil {
		return LifecycleRecord{}, fmt.Errorf("lifecycle correlation invalid: %w", err)
	}
	if err := validateLifecycleRefs(record); err != nil {
		return LifecycleRecord{}, err
	}
	if err := validateLifecycleEvent(record); err != nil {
		return LifecycleRecord{}, err
	}
	if err := validateLifecycleEvidence(record); err != nil {
		return LifecycleRecord{}, err
	}
	if len(options.SigningPrivateKey) == 0 {
		return LifecycleRecord{}, errors.New("lifecycle signing key is required")
	}
	if len(options.SigningPrivateKey) != ed25519.PrivateKeySize {
		return LifecycleRecord{}, errors.New("lifecycle signing key has invalid size")
	}
	if err := verifyLifecycleEmbeddedEvidence(record, options.SigningPrivateKey.Public().(ed25519.PublicKey)); err != nil {
		return LifecycleRecord{}, err
	}
	digest, err := lifecycleDigest(record)
	if err != nil {
		return LifecycleRecord{}, err
	}
	record.RecordID = "gait-lr-" + strings.TrimPrefix(digest, "sha256:")[:16]
	signature, err := proofsign.SignDigestHex(options.SigningPrivateKey, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return LifecycleRecord{}, err
	}
	record.Signature = signature
	if raw, err := json.Marshal(record); err != nil {
		return LifecycleRecord{}, errors.New("lifecycle schema validation failed")
	} else if err := validateRuntimeSchema(raw, RuntimeLifecycleSchemaID); err != nil {
		return LifecycleRecord{}, fmt.Errorf("lifecycle schema validation failed: %w", err)
	}
	return record, nil
}

func VerifyLifecycleRecord(record LifecycleRecord, publicKey ed25519.PublicKey) (bool, error) {
	if record.SchemaID != RuntimeLifecycleSchemaID || record.SchemaVersion != RuntimeLifecycleVersion || record.RecordID == "" {
		return false, errors.New("lifecycle schema or identity invalid")
	}
	if err := record.Correlation.Validate(); err != nil {
		return false, fmt.Errorf("lifecycle correlation invalid: %w", err)
	}
	if err := validateLifecycleRefs(record); err != nil {
		return false, err
	}
	if err := validateLifecycleEvent(record); err != nil {
		return false, err
	}
	if err := validateLifecycleEvidence(record); err != nil {
		return false, err
	}
	if raw, err := json.Marshal(record); err != nil {
		return false, errors.New("lifecycle schema validation failed")
	} else if err := validateRuntimeSchema(raw, RuntimeLifecycleSchemaID); err != nil {
		return false, fmt.Errorf("lifecycle schema validation failed: %w", err)
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
	if record.RecordID != "gait-lr-"+strings.TrimPrefix(digest, "sha256:")[:16] {
		return false, nil
	}
	if err := verifyLifecycleEmbeddedEvidence(record, publicKey); err != nil {
		return false, err
	}
	return true, nil
}

// ParseLifecycleRecord is the strict JSON boundary for persisted lifecycle
// records; callers must verify the returned record before reducing it.
func ParseLifecycleRecord(raw []byte) (LifecycleRecord, error) {
	var record LifecycleRecord
	if err := DecodeStrictRuntimeJSON(raw, &record); err != nil {
		return LifecycleRecord{}, err
	}
	if err := validateRuntimeSchema(raw, RuntimeLifecycleSchemaID); err != nil {
		return LifecycleRecord{}, errors.New("lifecycle schema validation failed")
	}
	if err := validateLifecycleEvent(record); err != nil {
		return LifecycleRecord{}, err
	}
	return record, nil
}

func validLifecycleRef(ref proof.RelationshipRef) bool {
	return strings.TrimSpace(ref.Kind) != "" && strings.TrimSpace(ref.ID) != "" && validSHA256Digest(ref.Digest)
}

func validateLifecycleRefs(record LifecycleRecord) error {
	if !validLifecycleRef(record.ContractRef) {
		return errors.New("lifecycle contract reference must be digest-bound")
	}
	if record.Correlation.BindingMode != proof.BindingModeDigestBound || record.Correlation.ContractRef == nil || !validLifecycleRef(*record.Correlation.ContractRef) {
		return errors.New("lifecycle correlation must carry digest-bound contract reference")
	}
	if !sameLifecycleRefIdentity(record.Correlation.ContractRef, &record.ContractRef) {
		return errors.New("lifecycle correlation contract identity mismatch")
	}
	if !validSHA256Digest(record.Correlation.ContentDigest) || record.Correlation.ContentDigest != record.ContractRef.Digest {
		return errors.New("lifecycle correlation content digest mismatch")
	}
	if record.ProposalRef != nil && !validLifecycleRef(*record.ProposalRef) {
		return errors.New("lifecycle proposal reference must be digest-bound")
	}
	if record.ProposalRef != nil && !sameLifecycleRefIdentity(record.ProposalRef, &record.ContractRef) {
		return errors.New("lifecycle proposal reference identity mismatch")
	}
	if record.ActivationRef != nil && !validLifecycleRef(*record.ActivationRef) {
		return errors.New("lifecycle activation reference must be digest-bound")
	}
	if record.ActivationRef != nil && (record.ActivationRef.Kind != "activated_action_contract" || record.ActivationRef.SchemaID != ActivatedSchemaID || record.ActivationRef.SchemaVersion != ActivatedSchemaVersion || record.ActivationRef.SourceProduct != ActivatedProducer) {
		return errors.New("lifecycle activation reference identity mismatch")
	}
	for _, ref := range record.PreconditionRefs {
		if !validLifecycleRef(ref) {
			return errors.New("lifecycle precondition reference must be digest-bound")
		}
	}
	for _, ref := range record.EvidenceRefs {
		if !validLifecycleRef(ref) {
			return errors.New("lifecycle evidence reference must be digest-bound")
		}
	}
	if err := validateLifecycleEvidenceBinding(record); err != nil {
		return err
	}
	if record.Correlation.EventRef != nil {
		expected := record.ActivationRef
		if expected == nil {
			expected = record.ProposalRef
		}
		if expected == nil || !sameLifecycleRefIdentity(record.Correlation.EventRef, expected) {
			return errors.New("lifecycle correlation event reference identity mismatch")
		}
	}
	if record.Correlation.CausalRef != nil {
		if record.ProposalRef == nil || !sameLifecycleRefIdentity(record.Correlation.CausalRef, record.ProposalRef) {
			return errors.New("lifecycle correlation causal reference identity mismatch")
		}
	}
	return nil
}

func sameLifecycleRefIdentity(actual, expected *proof.RelationshipRef) bool {
	if actual == nil || expected == nil || !validLifecycleRef(*actual) {
		return false
	}
	if actual.Kind != expected.Kind || actual.ID != expected.ID || actual.Digest != expected.Digest {
		return false
	}
	if actual.SchemaID != "" && actual.SchemaID != expected.SchemaID {
		return false
	}
	if actual.SchemaVersion != "" && actual.SchemaVersion != expected.SchemaVersion {
		return false
	}
	if actual.SourceProduct != "" && actual.SourceProduct != expected.SourceProduct {
		return false
	}
	return true
}

func validateLifecycleEvent(record LifecycleRecord) error {
	typedEvidenceCount := 0
	if record.Execution != nil {
		typedEvidenceCount++
	}
	if record.Effect != nil {
		typedEvidenceCount++
	}
	if record.Containment != nil {
		typedEvidenceCount++
	}
	if record.Compensation != nil {
		typedEvidenceCount++
	}
	if typedEvidenceCount > 1 {
		return errors.New("lifecycle_typed_evidence_ambiguous")
	}
	switch record.Kind {
	case LifecycleExecutionStarted, LifecycleExecutionSucceeded, LifecycleExecutionFailed, LifecycleExecutionBlocked:
		if record.Execution == nil || typedEvidenceCount != 1 {
			return errors.New("lifecycle_execution_evidence_required")
		}
	case LifecycleEffectRecorded, LifecycleEffectValidated:
		if record.Effect == nil || typedEvidenceCount != 1 {
			return errors.New("lifecycle_effect_evidence_required")
		}
	case LifecycleContainmentRequested, LifecycleContainmentCompleted, LifecycleContainmentPartial, LifecycleContainmentUnresolved:
		if record.Containment == nil || typedEvidenceCount != 1 {
			return errors.New("lifecycle_containment_evidence_required")
		}
	case LifecycleCompensationRequired, LifecycleCompensationStarted, LifecycleCompensationCompleted:
		if record.Compensation == nil || typedEvidenceCount != 1 {
			return errors.New("lifecycle_compensation_evidence_required")
		}
	default:
		if typedEvidenceCount != 0 {
			return errors.New("lifecycle_typed_evidence_kind_mismatch")
		}
	}
	switch record.Kind {
	case LifecycleProposalIngested:
		if record.ProposalRef == nil {
			return errors.New("lifecycle_proposal_ref_required")
		}
	case LifecycleActivationRequested:
		if record.ProposalRef == nil {
			return errors.New("lifecycle_proposal_ref_required")
		}
	case LifecycleActivated:
		if record.ProposalRef == nil || record.ActivationRef == nil {
			return errors.New("lifecycle_activation_refs_required")
		}
	case LifecyclePreconditionEvaluated:
		if len(record.PreconditionRefs) == 0 {
			return errors.New("lifecycle_precondition_refs_required")
		}
	case LifecycleDecisionReady:
		if record.Decision == nil || !record.Decision.Ready {
			return errors.New("lifecycle_decision_not_ready")
		}
		if record.Decision.Status != ReadinessSatisfied {
			return errors.New("lifecycle_decision_status_not_satisfied")
		}
		requiredCount := 0
		for _, precondition := range record.Decision.Preconditions {
			if precondition.Required {
				requiredCount++
				if precondition.Status != ReadinessSatisfied {
					return errors.New("lifecycle_decision_precondition_unsatisfied")
				}
			}
		}
		if requiredCount == 0 {
			return errors.New("lifecycle_decision_required_precondition_missing")
		}
	case LifecycleExecutionStarted:
		if record.Execution == nil || record.Execution.Outcome != "started" {
			return errors.New("lifecycle_execution_started_evidence_required")
		}
	case LifecycleExecutionSucceeded:
		if record.Execution == nil || record.Execution.Outcome != "succeeded" {
			return errors.New("lifecycle_execution_succeeded_evidence_required")
		}
	case LifecycleExecutionFailed:
		if record.Execution == nil || record.Execution.Outcome != "failed" {
			return errors.New("lifecycle_execution_failed_evidence_required")
		}
	case LifecycleExecutionBlocked:
		if record.Execution == nil || record.Execution.Outcome != "blocked" {
			return errors.New("lifecycle_execution_blocked_evidence_required")
		}
	case LifecycleEffectRecorded:
		if record.Effect == nil || record.Effect.Outcome != "recorded" {
			return errors.New("lifecycle_effect_recorded_evidence_required")
		}
	case LifecycleEffectValidated:
		if record.Effect == nil || record.Effect.Outcome != "validated" {
			return errors.New("lifecycle_effect_validated_evidence_required")
		}
	case LifecycleContainmentRequested:
		if record.Containment == nil || record.Containment.Outcome != "requested" {
			return errors.New("lifecycle_containment_requested_evidence_required")
		}
	case LifecycleContainmentCompleted:
		if record.Containment == nil || record.Containment.Outcome != "completed" {
			return errors.New("lifecycle_containment_completed_evidence_required")
		}
	case LifecycleContainmentPartial:
		if record.Containment == nil || record.Containment.Outcome != "partial" {
			return errors.New("lifecycle_containment_partial_evidence_required")
		}
	case LifecycleContainmentUnresolved:
		if record.Containment == nil || record.Containment.Outcome != "unresolved" {
			return errors.New("lifecycle_containment_unresolved_evidence_required")
		}
	case LifecycleCompensationRequired, LifecycleCompensationStarted, LifecycleCompensationCompleted:
		if record.Compensation == nil {
			return errors.New("lifecycle_compensation_evidence_required")
		}
		if record.Kind == LifecycleCompensationRequired && record.Compensation.Outcome != "required" || record.Kind == LifecycleCompensationStarted && record.Compensation.Outcome != "started" || record.Kind == LifecycleCompensationCompleted && record.Compensation.Outcome != "completed" {
			return errors.New("lifecycle_compensation_outcome_mismatch")
		}
	}
	return nil
}

func evidenceRefForExecution(item ExecutionEvidence) proof.RelationshipRef {
	return evidenceRef("execution", item.EvidenceID, item.CanonicalContentDigest, ExecutionEvidenceSchemaID)
}
func evidenceRefForEffect(item EffectEvent) proof.RelationshipRef {
	return evidenceRef("effect_event", item.EvidenceID, item.CanonicalContentDigest, EffectEventSchemaID)
}
func evidenceRefForContainment(item ContainmentEvidence) proof.RelationshipRef {
	return evidenceRef("containment", item.EvidenceID, item.CanonicalContentDigest, ContainmentEvidenceSchemaID)
}
func evidenceRefForCompensation(item CompensationEvidence) proof.RelationshipRef {
	return evidenceRef("compensation", item.EvidenceID, item.CanonicalContentDigest, CompensationEvidenceSchemaID)
}

func validateLifecycleEvidenceBinding(record LifecycleRecord) error {
	var binding *EvidenceBinding
	if record.Execution != nil {
		b := record.Execution.Binding
		binding = &b
	}
	if record.Effect != nil {
		b := record.Effect.Binding
		if binding != nil && !binding.sameIdentity(b) {
			return errors.New("lifecycle evidence binding mismatch")
		}
		binding = &b
	}
	if record.Containment != nil {
		b := record.Containment.Binding
		if binding != nil && !binding.sameIdentity(b) {
			return errors.New("lifecycle evidence binding mismatch")
		}
		binding = &b
	}
	if record.Compensation != nil {
		b := record.Compensation.Binding
		if binding != nil && !binding.sameIdentity(b) {
			return errors.New("lifecycle evidence binding mismatch")
		}
		binding = &b
	}
	if binding == nil {
		return nil
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	if !sameLifecycleRefIdentity(&binding.ContractRef, &record.ContractRef) || binding.Revision != record.Revision {
		return errors.New("lifecycle evidence contract/revision mismatch")
	}
	if record.ContractFamilyID == "" || binding.ContractFamilyID != record.ContractFamilyID {
		return errors.New("lifecycle evidence family mismatch")
	}
	if record.ProposalRef == nil || record.ActivationRef == nil || !sameLifecycleRefIdentity(&binding.ContractRef, record.ProposalRef) || !sameLifecycleRefIdentity(&binding.ActivationRef, record.ActivationRef) {
		return errors.New("lifecycle evidence proposal/activation mismatch")
	}
	expectedRefs := []proof.RelationshipRef{}
	if record.Execution != nil {
		expectedRefs = append(expectedRefs, evidenceRefForExecution(*record.Execution))
	}
	if record.Effect != nil {
		expectedRefs = append(expectedRefs, evidenceRefForEffect(*record.Effect))
	}
	if record.Containment != nil {
		expectedRefs = append(expectedRefs, evidenceRefForContainment(*record.Containment))
	}
	if record.Compensation != nil {
		expectedRefs = append(expectedRefs, evidenceRefForCompensation(*record.Compensation))
	}
	for _, expected := range expectedRefs {
		found := false
		for _, actual := range record.EvidenceRefs {
			if actual.Kind == expected.Kind && actual.ID == expected.ID && actual.Digest == expected.Digest && actual.SchemaID == expected.SchemaID && actual.SchemaVersion == expected.SchemaVersion && actual.SourceProduct == expected.SourceProduct {
				found = true
				break
			}
		}
		if !found {
			return errors.New("lifecycle typed evidence reference missing")
		}
	}
	return nil
}

func validateLifecycleEvidence(record LifecycleRecord) error {
	evidenceOccurredAt, evidenceFreshUntil := "", ""
	if record.Execution != nil {
		if err := validateExecutionEvidence(*record.Execution); err != nil {
			return err
		}
		evidenceOccurredAt, evidenceFreshUntil = record.Execution.OccurredAt, record.Execution.FreshUntil
	}
	if record.Effect != nil {
		if err := validateEffectEvent(*record.Effect); err != nil {
			return err
		}
		evidenceOccurredAt, evidenceFreshUntil = record.Effect.OccurredAt, record.Effect.FreshUntil
	}
	if record.Containment != nil {
		if err := validateContainmentEvidence(*record.Containment); err != nil {
			return err
		}
		evidenceOccurredAt, evidenceFreshUntil = record.Containment.OccurredAt, record.Containment.FreshUntil
	}
	if record.Compensation != nil {
		if err := validateCompensationEvidence(*record.Compensation); err != nil {
			return err
		}
		evidenceOccurredAt, evidenceFreshUntil = record.Compensation.OccurredAt, record.Compensation.FreshUntil
	}
	if evidenceOccurredAt != "" {
		recordTime, recordErr := time.Parse(time.RFC3339Nano, record.OccurredAt)
		evidenceTime, evidenceErr := time.Parse(time.RFC3339Nano, evidenceOccurredAt)
		freshUntil, freshErr := time.Parse(time.RFC3339Nano, evidenceFreshUntil)
		if recordErr != nil || evidenceErr != nil || freshErr != nil || recordTime.Before(evidenceTime) || recordTime.After(freshUntil) {
			return errors.New("lifecycle evidence outside event time window")
		}
	}
	return nil
}

func verifyLifecycleEmbeddedEvidence(record LifecycleRecord, publicKey ed25519.PublicKey) error {
	if record.Execution != nil {
		if ok, err := VerifyExecutionEvidence(*record.Execution, publicKey); err != nil || !ok {
			return errors.New("execution evidence verification failed")
		}
	}
	if record.Effect != nil {
		if ok, err := VerifyEffectEvent(*record.Effect, publicKey); err != nil || !ok {
			return errors.New("effect evidence verification failed")
		}
	}
	if record.Containment != nil {
		if ok, err := VerifyContainmentEvidence(*record.Containment, publicKey); err != nil || !ok {
			return errors.New("containment evidence verification failed")
		}
	}
	if record.Compensation != nil {
		if ok, err := VerifyCompensationEvidence(*record.Compensation, publicKey); err != nil || !ok {
			return errors.New("compensation evidence verification failed")
		}
	}
	return nil
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
	ExecutionStatus        string            `json:"execution_status,omitempty"`
	EffectStatus           string            `json:"effect_status,omitempty"`
	ContainmentStatus      string            `json:"containment_status,omitempty"`
	CompensationStatus     string            `json:"compensation_status,omitempty"`
	CurrentStatus          string            `json:"current_status"`
	ReasonCodes            []string          `json:"reason_codes,omitempty"`
	Records                []LifecycleRecord `json:"records"`
}

func ReduceLifecycle(records []LifecycleRecord) LifecycleSnapshot {
	snapshot, err := ReduceLifecycleChecked(records)
	if err != nil {
		return LifecycleSnapshot{CurrentStatus: "invalid", ReasonCodes: []string{err.Error()}, Records: append([]LifecycleRecord(nil), records...)}
	}
	return snapshot
}

// ReduceLifecycleChecked validates structural identity, timestamps, contract
// isolation, duplicate IDs, and terminal ordering before applying the pure
// reduction. It does not verify signatures; use ReduceVerifiedLifecycle when
// the result is used as authoritative lifecycle state.
func ReduceLifecycleChecked(records []LifecycleRecord) (LifecycleSnapshot, error) {
	ordered := append([]LifecycleRecord(nil), records...)
	seenIDs := map[string]struct{}{}
	contractKey := ""
	revision := -1
	var previousTime time.Time
	previousID := ""
	var evidenceBinding *EvidenceBinding
	for index, record := range ordered {
		if record.RecordID == "" {
			return LifecycleSnapshot{}, errors.New("lifecycle_record_id_missing")
		}
		if _, exists := seenIDs[record.RecordID]; exists {
			return LifecycleSnapshot{}, errors.New("lifecycle_duplicate_record_id")
		}
		seenIDs[record.RecordID] = struct{}{}
		occurredAt, err := time.Parse(time.RFC3339Nano, record.OccurredAt)
		if err != nil {
			return LifecycleSnapshot{}, errors.New("lifecycle_timestamp_invalid")
		}
		if index > 0 && (occurredAt.Before(previousTime) || (occurredAt.Equal(previousTime) && record.RecordID < previousID)) {
			return LifecycleSnapshot{}, errors.New("lifecycle_input_reordered")
		}
		previousTime = occurredAt
		previousID = record.RecordID
		if record.Revision < 1 {
			return LifecycleSnapshot{}, errors.New("lifecycle_revision_invalid")
		}
		if err := record.Correlation.Validate(); err != nil {
			return LifecycleSnapshot{}, errors.New("lifecycle_correlation_invalid")
		}
		if err := validateLifecycleRefs(record); err != nil {
			return LifecycleSnapshot{}, err
		}
		if err := validateLifecycleEvent(record); err != nil {
			return LifecycleSnapshot{}, err
		}
		if err := validateLifecycleEvidence(record); err != nil {
			return LifecycleSnapshot{}, err
		}
		if record.Execution != nil || record.Effect != nil || record.Containment != nil || record.Compensation != nil {
			var current EvidenceBinding
			switch {
			case record.Execution != nil:
				current = record.Execution.Binding
			case record.Effect != nil:
				current = record.Effect.Binding
			case record.Containment != nil:
				current = record.Containment.Binding
			default:
				current = record.Compensation.Binding
			}
			if evidenceBinding == nil {
				evidenceBinding = &current
			} else if !evidenceBinding.sameIdentity(current) {
				return LifecycleSnapshot{}, errors.New("lifecycle_evidence_binding_mismatch")
			}
		}
		key := record.ContractRef.Kind + "|" + record.ContractRef.ID + "|" + record.ContractRef.Digest
		if contractKey == "" {
			contractKey = key
		} else if key != contractKey {
			return LifecycleSnapshot{}, errors.New("lifecycle_contract_mismatch")
		}
		if revision < 0 {
			revision = record.Revision
		} else if record.Revision != revision {
			return LifecycleSnapshot{}, errors.New("lifecycle_revision_mismatch")
		}
	}
	out := LifecycleSnapshot{Records: ordered}
	evaluated := map[string]struct{}{}
	evaluatedByID := map[string]string{}
	terminal := false
	proposalIngested := false
	activationRequested := false
	decisionReady := false
	executionStarted := false
	executionTerminal := false
	executionOutcome := ""
	executionStartedRef := proof.RelationshipRef{}
	executionEvidenceRef := proof.RelationshipRef{}
	effectRecorded := false
	effectValidated := false
	effectArtifactRef := proof.RelationshipRef{}
	containmentRequested := false
	containmentTerminal := false
	containmentRequestRef := proof.RelationshipRef{}
	containmentScopeRef := proof.RelationshipRef{}
	compensationNeeded := false
	compensationRequiredRecorded := false
	compensationStarted := false
	compensationCompleted := false
	compensationRequirementRef := proof.RelationshipRef{}
	compensationEventRef := proof.RelationshipRef{}
	effectEvidenceRef := proof.RelationshipRef{}
	for _, record := range ordered {
		if terminal {
			return LifecycleSnapshot{}, errors.New("lifecycle_terminal_order_invalid")
		}
		switch record.Kind {
		case LifecycleProposalIngested:
			out.ProposalIngested = true
			proposalIngested = true
		case LifecycleActivationRequested:
			if !proposalIngested {
				return LifecycleSnapshot{}, errors.New("lifecycle_activation_without_proposal")
			}
			if out.Rejected {
				return LifecycleSnapshot{}, errors.New("lifecycle_activation_after_rejection")
			}
			out.ActivationRequested = true
			activationRequested = true
		case LifecycleActivated:
			if !proposalIngested || !activationRequested {
				return LifecycleSnapshot{}, errors.New("lifecycle_activation_without_request")
			}
			if out.Rejected {
				return LifecycleSnapshot{}, errors.New("lifecycle_activation_after_rejection")
			}
			if !decisionReady {
				return LifecycleSnapshot{}, errors.New("lifecycle_activation_without_decision")
			}
			out.Activated = true
			out.Rejected = false
		case LifecycleRejected:
			out.Rejected = true
			out.Activated = false
			terminal = true
		case LifecycleRevoked:
			out.Revoked = true
			out.Activated = false
			terminal = true
		case LifecycleSuperseded:
			out.Superseded = true
			out.Activated = false
			terminal = true
		case LifecyclePreconditionEvaluated:
			for _, ref := range record.PreconditionRefs {
				key := ref.Kind + "|" + ref.ID + "|" + ref.Digest
				if _, exists := evaluated[key]; !exists {
					evaluated[key] = struct{}{}
					out.PreconditionsEvaluated++
				}
				if existing, exists := evaluatedByID[ref.ID]; exists && existing != ref.Digest {
					return LifecycleSnapshot{}, errors.New("lifecycle_precondition_binding_conflict")
				}
				evaluatedByID[ref.ID] = ref.Digest
			}
		case LifecycleDecisionReady:
			if !proposalIngested {
				return LifecycleSnapshot{}, errors.New("lifecycle_decision_without_proposal")
			}
			if record.Decision == nil || !record.Decision.Ready {
				return LifecycleSnapshot{}, errors.New("lifecycle_decision_not_ready")
			}
			for _, precondition := range record.Decision.Preconditions {
				if !precondition.Required || precondition.Status == ReadinessNotRequired {
					continue
				}
				if precondition.Status != ReadinessSatisfied {
					return LifecycleSnapshot{}, errors.New("lifecycle_decision_precondition_unsatisfied")
				}
				if precondition.RequirementID == "" || !validSHA256Digest(precondition.EvidenceDigest) {
					return LifecycleSnapshot{}, errors.New("lifecycle_decision_precondition_digest_missing")
				}
				if digest, ok := evaluatedByID[precondition.RequirementID]; !ok || digest != precondition.EvidenceDigest {
					return LifecycleSnapshot{}, errors.New("lifecycle_decision_precondition_not_evaluated")
				}
			}
			out.DecisionReady = true
			decisionReady = true
		case LifecycleExecutionStarted:
			if !decisionReady || !out.Activated {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_before_verified_activation")
			}
			if executionStarted || executionTerminal {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_replayed")
			}
			if record.Execution == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_evidence_missing")
			}
			executionStarted = true
			executionStartedRef = evidenceRefForExecution(*record.Execution)
			executionEvidenceRef = executionStartedRef
			out.ExecutionStatus = "started"
			compensationNeeded = compensationNeeded || record.Execution.CompensationRequired
		case LifecycleExecutionBlocked:
			if !decisionReady || !out.Activated || executionStarted || executionTerminal || record.Execution == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_blocked_order_invalid")
			}
			if record.ActivationRef == nil || !hasExactRef(record.Execution.Binding.CausalRefs, *record.ActivationRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_blocked_predecessor_mismatch")
			}
			executionTerminal = true
			executionOutcome = record.Execution.Outcome
			executionEvidenceRef = evidenceRefForExecution(*record.Execution)
			out.ExecutionStatus = executionOutcome
			compensationNeeded = compensationNeeded || record.Execution.CompensationRequired
		case LifecycleExecutionSucceeded, LifecycleExecutionFailed:
			if !executionStarted || executionTerminal {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_terminal_order_invalid")
			}
			if record.Execution == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_evidence_missing")
			}
			if !hasExactRef(record.Execution.Binding.CausalRefs, executionStartedRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_execution_predecessor_mismatch")
			}
			executionTerminal = true
			executionOutcome = record.Execution.Outcome
			executionEvidenceRef = evidenceRefForExecution(*record.Execution)
			out.ExecutionStatus = executionOutcome
			compensationNeeded = compensationNeeded || record.Execution.CompensationRequired
		case LifecycleEffectRecorded:
			if !executionTerminal || executionOutcome != "succeeded" {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_without_successful_execution")
			}
			if effectRecorded || effectValidated || record.Effect == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_replayed_or_missing")
			}
			if !exactRef(record.Effect.ExecutionRef, executionEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_execution_mismatch")
			}
			if !hasExactRef(record.Effect.Binding.CausalRefs, executionEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_predecessor_mismatch")
			}
			effectRecorded = true
			effectEvidenceRef = evidenceRefForEffect(*record.Effect)
			effectArtifactRef = record.Effect.EffectRef
			out.EffectStatus = "recorded"
		case LifecycleEffectValidated:
			if !effectRecorded || effectValidated || record.Effect == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_validation_order_invalid")
			}
			effectValidated = true
			if !exactRef(record.Effect.ExecutionRef, executionEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_validation_mismatch")
			}
			if !exactRef(record.Effect.EffectRef, effectArtifactRef) || !hasExactRef(record.Effect.Binding.CausalRefs, effectEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_effect_validation_predecessor_mismatch")
			}
			effectEvidenceRef = evidenceRefForEffect(*record.Effect)
			out.EffectStatus = "validated"
		case LifecycleContainmentRequested:
			if !executionStarted || containmentRequested || containmentTerminal || record.Containment == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_containment_request_invalid")
			}
			if !effectValidated {
				return LifecycleSnapshot{}, errors.New("lifecycle_containment_without_validated_effect")
			}
			containmentRequested = true
			if !exactRef(record.Containment.ExecutionRef, executionEvidenceRef) || !exactRef(record.Containment.EffectRef, effectEvidenceRef) || !hasExactRef(record.Containment.Binding.CausalRefs, effectEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_containment_effect_mismatch")
			}
			containmentRequestRef = evidenceRefForContainment(*record.Containment)
			containmentScopeRef = record.Containment.ContainmentRef
			out.ContainmentStatus = "requested"
		case LifecycleContainmentCompleted, LifecycleContainmentPartial, LifecycleContainmentUnresolved:
			if !containmentRequested || containmentTerminal || record.Containment == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_containment_terminal_order_invalid")
			}
			containmentTerminal = true
			if !exactRef(record.Containment.ExecutionRef, executionEvidenceRef) || !exactRef(record.Containment.EffectRef, effectEvidenceRef) || !exactRef(record.Containment.ContainmentRef, containmentScopeRef) || !hasExactRef(record.Containment.Binding.CausalRefs, containmentRequestRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_containment_effect_mismatch")
			}
			out.ContainmentStatus = record.Containment.Outcome
		case LifecycleCompensationRequired:
			if !executionTerminal || !compensationNeeded || compensationRequiredRecorded || record.Compensation == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_requirement_invalid")
			}
			if !exactRef(record.Compensation.ExecutionRef, executionEvidenceRef) || !hasExactRef(record.Compensation.Binding.CausalRefs, executionEvidenceRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_execution_mismatch")
			}
			compensationRequiredRecorded = true
			compensationRequirementRef = record.Compensation.RequirementRef
			compensationEventRef = evidenceRefForCompensation(*record.Compensation)
			out.CompensationStatus = "required"
		case LifecycleCompensationStarted:
			if !compensationRequiredRecorded || compensationStarted || compensationCompleted || record.Compensation == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_start_invalid")
			}
			if !exactRef(record.Compensation.ExecutionRef, executionEvidenceRef) || !exactRef(record.Compensation.RequirementRef, compensationRequirementRef) || !hasExactRef(record.Compensation.Binding.CausalRefs, compensationEventRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_requirement_mismatch")
			}
			compensationStarted = true
			compensationEventRef = evidenceRefForCompensation(*record.Compensation)
			out.CompensationStatus = "started"
		case LifecycleCompensationCompleted:
			if !compensationStarted || compensationCompleted || record.Compensation == nil {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_completion_invalid")
			}
			if !exactRef(record.Compensation.ExecutionRef, executionEvidenceRef) || !exactRef(record.Compensation.RequirementRef, compensationRequirementRef) || !hasExactRef(record.Compensation.Binding.CausalRefs, compensationEventRef) {
				return LifecycleSnapshot{}, errors.New("lifecycle_compensation_completion_mismatch")
			}
			compensationCompleted = true
			out.CompensationStatus = "completed"
		}
		out.ReasonCodes = append(out.ReasonCodes, record.ReasonCodes...)
	}
	if compensationNeeded && !compensationCompleted {
		return LifecycleSnapshot{}, errors.New("lifecycle_required_compensation_missing")
	}
	switch {
	case out.Revoked:
		out.CurrentStatus = "revoked"
	case out.Superseded:
		out.CurrentStatus = "superseded"
	case out.Rejected:
		out.CurrentStatus = "rejected"
	case out.CompensationStatus != "":
		out.CurrentStatus = "compensation_" + out.CompensationStatus
	case out.ContainmentStatus != "":
		out.CurrentStatus = "containment_" + out.ContainmentStatus
	case out.EffectStatus != "":
		out.CurrentStatus = "effect_" + out.EffectStatus
	case out.ExecutionStatus != "":
		out.CurrentStatus = "execution_" + out.ExecutionStatus
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
	return out, nil
}

// ReduceVerifiedLifecycle verifies every record with the trusted public key
// before applying the structural pure reducer. A failed verification is
// authoritative failure and never falls back to structural reduction.
func ReduceVerifiedLifecycle(records []LifecycleRecord, publicKey ed25519.PublicKey) (LifecycleSnapshot, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return LifecycleSnapshot{}, errors.New("lifecycle public key invalid")
	}
	for _, record := range records {
		valid, err := VerifyLifecycleRecord(record, publicKey)
		if err != nil || !valid {
			if err != nil {
				return LifecycleSnapshot{}, fmt.Errorf("lifecycle record verification failed: %w", err)
			}
			return LifecycleSnapshot{}, errors.New("lifecycle record verification failed")
		}
	}
	return ReduceLifecycleChecked(records)
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
func int64Field(object map[string]any, key string) int64 {
	switch value := object[key].(type) {
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	default:
		return 0
	}
}
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
