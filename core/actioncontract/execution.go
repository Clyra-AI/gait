package actioncontract

// This file contains the bounded, reference-first evidence emitted at an
// execution boundary.  Gait records and verifies claims; it never executes a
// tool and these types intentionally contain no raw tool payloads or secrets.

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	ExecutionEvidenceSchemaID            = "https://gait.dev/schemas/v1/action-contract/execution-evidence.schema.json"
	EffectEventSchemaID                  = "https://gait.dev/schemas/v1/action-contract/effect-event.schema.json"
	ContainmentEvidenceSchemaID          = "https://gait.dev/schemas/v1/action-contract/containment-evidence.schema.json"
	CompensationEvidenceSchemaID         = "https://gait.dev/schemas/v1/action-contract/compensation-evidence.schema.json"
	ExecutionEvidenceSchemaVersion       = "1"
	EvidenceProducer                     = "gait"
	MaxEvidenceBytes               int64 = 4 << 20
)

type EvidenceBinding struct {
	ContractFamilyID string                                   `json:"contract_family_id"`
	Revision         int                                      `json:"revision"`
	ContractRef      proof.RelationshipRef                    `json:"contract_ref"`
	ActivationRef    proof.RelationshipRef                    `json:"activation_ref"`
	RuntimeActionRef proof.RelationshipRef                    `json:"runtime_action_ref"`
	ReadinessRef     proof.RelationshipRef                    `json:"readiness_ref"`
	DecisionRef      proof.RelationshipRef                    `json:"decision_ref"`
	PolicyRef        proof.RelationshipRef                    `json:"policy_ref"`
	TargetRef        proof.RelationshipRef                    `json:"target_ref"`
	EnvironmentRef   proof.RelationshipRef                    `json:"environment_ref"`
	ProofRefs        []proof.RelationshipRef                  `json:"proof_refs"`
	CausalRefs       []proof.RelationshipRef                  `json:"causal_refs"`
	Correlation      proof.ControlContainmentTelemetryProfile `json:"correlation"`
}

type EvidenceProvenance struct {
	Producer  string              `json:"producer"`
	Writer    string              `json:"writer"`
	Verifier  string              `json:"verifier"`
	PublicKey string              `json:"public_key"`
	Signature proofsign.Signature `json:"signature"`
}

type ExecutionEvidence struct {
	SchemaID               string                `json:"schema_id"`
	SchemaVersion          string                `json:"schema_version"`
	EvidenceID             string                `json:"evidence_id"`
	Binding                EvidenceBinding       `json:"binding"`
	EventRef               proof.RelationshipRef `json:"event_ref"`
	OccurredAt             string                `json:"occurred_at"`
	FreshUntil             string                `json:"fresh_until"`
	Outcome                string                `json:"outcome"`
	ReasonCode             string                `json:"reason_code"`
	CompensationRequired   bool                  `json:"compensation_required"`
	Provenance             EvidenceProvenance    `json:"provenance"`
	CanonicalContentDigest string                `json:"canonical_content_digest"`
}

type EffectEvent struct {
	SchemaID               string                `json:"schema_id"`
	SchemaVersion          string                `json:"schema_version"`
	EvidenceID             string                `json:"evidence_id"`
	Binding                EvidenceBinding       `json:"binding"`
	EventRef               proof.RelationshipRef `json:"event_ref"`
	ExecutionRef           proof.RelationshipRef `json:"execution_ref"`
	EffectRef              proof.RelationshipRef `json:"effect_ref"`
	OccurredAt             string                `json:"occurred_at"`
	FreshUntil             string                `json:"fresh_until"`
	Outcome                string                `json:"outcome"`
	ReasonCode             string                `json:"reason_code"`
	Provenance             EvidenceProvenance    `json:"provenance"`
	CanonicalContentDigest string                `json:"canonical_content_digest"`
}

type ContainmentEvidence struct {
	SchemaID               string                `json:"schema_id"`
	SchemaVersion          string                `json:"schema_version"`
	EvidenceID             string                `json:"evidence_id"`
	Binding                EvidenceBinding       `json:"binding"`
	EventRef               proof.RelationshipRef `json:"event_ref"`
	ExecutionRef           proof.RelationshipRef `json:"execution_ref"`
	EffectRef              proof.RelationshipRef `json:"effect_ref"`
	ContainmentRef         proof.RelationshipRef `json:"containment_ref"`
	OccurredAt             string                `json:"occurred_at"`
	FreshUntil             string                `json:"fresh_until"`
	Outcome                string                `json:"outcome"`
	ReasonCode             string                `json:"reason_code"`
	Provenance             EvidenceProvenance    `json:"provenance"`
	CanonicalContentDigest string                `json:"canonical_content_digest"`
}

type CompensationEvidence struct {
	SchemaID               string                `json:"schema_id"`
	SchemaVersion          string                `json:"schema_version"`
	EvidenceID             string                `json:"evidence_id"`
	Binding                EvidenceBinding       `json:"binding"`
	EventRef               proof.RelationshipRef `json:"event_ref"`
	RequirementRef         proof.RelationshipRef `json:"requirement_ref"`
	ExecutionRef           proof.RelationshipRef `json:"execution_ref"`
	OccurredAt             string                `json:"occurred_at"`
	FreshUntil             string                `json:"fresh_until"`
	Outcome                string                `json:"outcome"`
	ReasonCode             string                `json:"reason_code"`
	Provenance             EvidenceProvenance    `json:"provenance"`
	CanonicalContentDigest string                `json:"canonical_content_digest"`
}

func digestBoundRef(ref proof.RelationshipRef) bool {
	return strings.TrimSpace(ref.Kind) != "" && strings.TrimSpace(ref.ID) != "" && validSHA256Digest(ref.Digest)
}

func (b EvidenceBinding) Validate() error {
	if strings.TrimSpace(b.ContractFamilyID) == "" || b.Revision < 1 {
		return errors.New("evidence binding family/revision required")
	}
	if len(b.ProofRefs) == 0 || len(b.CausalRefs) == 0 {
		return errors.New("evidence binding proof/causal references required")
	}
	refs := []proof.RelationshipRef{b.ContractRef, b.ActivationRef, b.RuntimeActionRef, b.ReadinessRef, b.DecisionRef, b.PolicyRef, b.TargetRef, b.EnvironmentRef}
	for _, ref := range refs {
		if !digestBoundRef(ref) {
			return errors.New("evidence binding reference must be digest-bound")
		}
	}
	if b.ContractRef.Kind != "action_contract" || b.ContractRef.SchemaID != ProposedContractSchemaID || b.ContractRef.SchemaVersion != ProposedContractVersion || b.ContractRef.SourceProduct != "wrkr" {
		return errors.New("evidence binding Wrkr contract reference invalid")
	}
	if b.ActivationRef.Kind != "activated_action_contract" || b.ActivationRef.SchemaID != ActivatedSchemaID || b.ActivationRef.SchemaVersion != ActivatedSchemaVersion || b.ActivationRef.SourceProduct != ActivatedProducer {
		return errors.New("evidence binding Gait activation reference invalid")
	}
	if b.RuntimeActionRef.Kind != "runtime_action" || b.RuntimeActionRef.SchemaID != RuntimeActionSchemaID || b.RuntimeActionRef.SchemaVersion != RuntimeActionSchemaVersion || b.RuntimeActionRef.SourceProduct != EvidenceProducer {
		return errors.New("evidence binding runtime action reference invalid")
	}
	if b.ReadinessRef.Kind != "readiness" || b.ReadinessRef.SchemaID != RuntimeReadinessSchemaID || b.ReadinessRef.SchemaVersion != RuntimeActionSchemaVersion || b.ReadinessRef.SourceProduct != EvidenceProducer {
		return errors.New("evidence binding readiness reference invalid")
	}
	if b.DecisionRef.Kind != "decision" || b.DecisionRef.SchemaID != RuntimeReadinessSchemaID || b.DecisionRef.SchemaVersion != RuntimeActionSchemaVersion || b.DecisionRef.SourceProduct != EvidenceProducer {
		return errors.New("evidence binding decision reference invalid")
	}
	for _, ref := range append(append([]proof.RelationshipRef{}, b.ProofRefs...), b.CausalRefs...) {
		if !digestBoundRef(ref) {
			return errors.New("evidence binding proof/causal reference must be digest-bound")
		}
	}
	if b.Correlation.BindingMode != proof.BindingModeDigestBound || b.Correlation.ContractRef == nil || b.Correlation.ContentDigest != b.ContractRef.Digest || !sameLifecycleRefIdentity(b.Correlation.ContractRef, &b.ContractRef) {
		return errors.New("evidence binding correlation must be digest-bound")
	}
	if err := b.Correlation.Validate(); err != nil {
		return errors.New("evidence binding correlation invalid")
	}
	return nil
}

func (b EvidenceBinding) sameIdentity(other EvidenceBinding) bool {
	// Causal references intentionally advance from one lifecycle event to the
	// next. Every other binding field, including Proof refs and the correlation
	// profile, is immutable for one authoritative lineage.
	left, right := b, other
	left.CausalRefs, right.CausalRefs = nil, nil
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftDigest, leftErr := proofcanon.DigestJCS(leftRaw)
	rightDigest, rightErr := proofcanon.DigestJCS(rightRaw)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func (b EvidenceBinding) RelationshipRefs() []proof.RelationshipRef {
	out := []proof.RelationshipRef{b.ContractRef, b.ActivationRef, b.RuntimeActionRef, b.ReadinessRef, b.DecisionRef, b.PolicyRef, b.TargetRef, b.EnvironmentRef}
	return append(out, append([]proof.RelationshipRef{}, b.ProofRefs...)...)
}

func hasExactRef(refs []proof.RelationshipRef, expected proof.RelationshipRef) bool {
	for _, ref := range refs {
		if ref.Kind == expected.Kind && ref.ID == expected.ID && ref.Digest == expected.Digest && ref.SchemaID == expected.SchemaID && ref.SchemaVersion == expected.SchemaVersion && ref.SourceProduct == expected.SourceProduct && validSHA256Digest(ref.Digest) {
			return true
		}
	}
	return false
}

func exactRef(actual, expected proof.RelationshipRef) bool {
	return hasExactRef([]proof.RelationshipRef{actual}, expected)
}

func validExecutionEvidenceRef(ref proof.RelationshipRef) bool {
	return digestBoundRef(ref) && ref.Kind == "execution" && ref.SchemaID == ExecutionEvidenceSchemaID && ref.SchemaVersion == ExecutionEvidenceSchemaVersion && ref.SourceProduct == EvidenceProducer
}

func validEvidenceTime(value string) bool {
	t, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && !t.IsZero() && t.Location() != nil
}

func validateProvenance(p EvidenceProvenance) error {
	if p.Producer != EvidenceProducer || strings.TrimSpace(p.Writer) == "" || strings.TrimSpace(p.Verifier) == "" || strings.TrimSpace(p.PublicKey) == "" {
		return errors.New("evidence provenance incomplete")
	}
	if p.Signature.Alg != proofsign.AlgEd25519 || p.Signature.SignedDigest == "" {
		return errors.New("evidence provenance signature missing")
	}
	return nil
}

func validateEvidenceCommon(schema, id, version, occurred, fresh, outcome string, binding EvidenceBinding, p EvidenceProvenance, digest string) error {
	if schema == "" || version != ExecutionEvidenceSchemaVersion || strings.TrimSpace(id) == "" || strings.TrimSpace(outcome) == "" {
		return errors.New("evidence identity/outcome incomplete")
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	if !validEvidenceTime(occurred) || !validEvidenceTime(fresh) {
		return errors.New("evidence timestamps invalid")
	}
	if t, _ := time.Parse(time.RFC3339Nano, fresh); t.Before(mustParseTime(occurred)) {
		return errors.New("evidence freshness window invalid")
	}
	if err := validateProvenance(p); err != nil {
		return err
	}
	if !validSHA256Digest(digest) {
		return errors.New("evidence canonical digest invalid")
	}
	return nil
}

func mustParseTime(value string) time.Time { t, _ := time.Parse(time.RFC3339Nano, value); return t }

func canonicalEvidence(value any) (string, error) {
	var copyValue any
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, &copyValue); err != nil {
		return "", err
	}
	object, ok := copyValue.(map[string]any)
	if !ok {
		return "", errors.New("evidence must be object")
	}
	delete(object, "canonical_content_digest")
	delete(object, "evidence_id")
	if provenance, ok := object["provenance"].(map[string]any); ok {
		delete(provenance, "signature")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	digest, err := proofcanon.DigestJCS(canonical)
	if err != nil {
		return "", err
	}
	return "sha256:" + strings.TrimPrefix(digest, "sha256:"), nil
}

func evidenceRef(kind, id, digest, schema string) proof.RelationshipRef {
	return proof.RelationshipRef{Kind: kind, ID: id, Digest: digest, SchemaID: schema, SchemaVersion: ExecutionEvidenceSchemaVersion, SourceProduct: EvidenceProducer}
}

func signEvidence(value any, private ed25519.PrivateKey) (string, proofsign.Signature, error) {
	if len(private) != ed25519.PrivateKeySize {
		return "", proofsign.Signature{}, errors.New("evidence signing key invalid")
	}
	digest, err := canonicalEvidence(value)
	if err != nil {
		return "", proofsign.Signature{}, err
	}
	sig, err := proofsign.SignDigestHex(private, strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return "", proofsign.Signature{}, err
	}
	return digest, sig, nil
}

func VerifyExecutionEvidence(item ExecutionEvidence, public ed25519.PublicKey) (bool, error) {
	return verifyEvidence(item.SchemaID, item.CanonicalContentDigest, item.Provenance, item, public, func() error { return validateExecutionEvidence(item) })
}
func VerifyEffectEvent(item EffectEvent, public ed25519.PublicKey) (bool, error) {
	return verifyEvidence(item.SchemaID, item.CanonicalContentDigest, item.Provenance, item, public, func() error { return validateEffectEvent(item) })
}
func VerifyContainmentEvidence(item ContainmentEvidence, public ed25519.PublicKey) (bool, error) {
	return verifyEvidence(item.SchemaID, item.CanonicalContentDigest, item.Provenance, item, public, func() error { return validateContainmentEvidence(item) })
}
func VerifyCompensationEvidence(item CompensationEvidence, public ed25519.PublicKey) (bool, error) {
	return verifyEvidence(item.SchemaID, item.CanonicalContentDigest, item.Provenance, item, public, func() error { return validateCompensationEvidence(item) })
}

func verifyEvidence(schema, digest string, p EvidenceProvenance, value any, public ed25519.PublicKey, validate func() error) (bool, error) {
	if err := validate(); err != nil {
		return false, err
	}
	if schema == "" || len(public) != ed25519.PublicKeySize {
		return false, errors.New("evidence verification key invalid")
	}
	declared, err := base64.StdEncoding.DecodeString(p.PublicKey)
	if err != nil || len(declared) != ed25519.PublicKeySize || string(declared) != string(public) {
		return false, errors.New("evidence provenance public key mismatch")
	}
	computed, err := canonicalEvidence(value)
	if err != nil || computed != digest || p.Signature.SignedDigest != strings.TrimPrefix(digest, "sha256:") {
		return false, errors.New("evidence digest mismatch")
	}
	ok, err := proofsign.VerifyDigestHex(public, p.Signature)
	if err != nil || !ok {
		return false, errors.New("evidence signature invalid")
	}
	return true, nil
}

func validateExecutionEvidence(item ExecutionEvidence) error {
	if strings.TrimSpace(item.ReasonCode) == "" {
		return errors.New("execution reason code required")
	}
	if item.SchemaID != ExecutionEvidenceSchemaID || item.Outcome != "started" && item.Outcome != "succeeded" && item.Outcome != "failed" && item.Outcome != "blocked" || !digestBoundRef(item.EventRef) {
		return errors.New("execution evidence shape invalid")
	}
	if err := validateEvidenceCommon(ExecutionEvidenceSchemaID, item.EvidenceID, item.SchemaVersion, item.OccurredAt, item.FreshUntil, item.Outcome, item.Binding, item.Provenance, item.CanonicalContentDigest); err != nil {
		return err
	}
	if err := validateEvidenceIdentity("gait-exec-", item.EvidenceID, item.CanonicalContentDigest); err != nil {
		return err
	}
	return validateTypedEvidenceSchema(item, ExecutionEvidenceSchemaID)
}
func validateEffectEvent(item EffectEvent) error {
	if strings.TrimSpace(item.ReasonCode) == "" {
		return errors.New("effect reason code required")
	}
	if err := validateEvidenceCommon(EffectEventSchemaID, item.EvidenceID, item.SchemaVersion, item.OccurredAt, item.FreshUntil, item.Outcome, item.Binding, item.Provenance, item.CanonicalContentDigest); err != nil {
		return err
	}
	if err := validateEvidenceIdentity("gait-effect-", item.EvidenceID, item.CanonicalContentDigest); err != nil {
		return err
	}
	if item.SchemaID != EffectEventSchemaID || (item.Outcome != "recorded" && item.Outcome != "validated") || !digestBoundRef(item.EventRef) {
		return errors.New("effect event shape invalid")
	}
	if !validExecutionEvidenceRef(item.ExecutionRef) || !digestBoundRef(item.EffectRef) {
		return errors.New("effect execution/effect references required")
	}
	return validateTypedEvidenceSchema(item, EffectEventSchemaID)
}
func validateContainmentEvidence(item ContainmentEvidence) error {
	if strings.TrimSpace(item.ReasonCode) == "" {
		return errors.New("containment reason code required")
	}
	if err := validateEvidenceCommon(ContainmentEvidenceSchemaID, item.EvidenceID, item.SchemaVersion, item.OccurredAt, item.FreshUntil, item.Outcome, item.Binding, item.Provenance, item.CanonicalContentDigest); err != nil {
		return err
	}
	if err := validateEvidenceIdentity("gait-containment-", item.EvidenceID, item.CanonicalContentDigest); err != nil {
		return err
	}
	if item.SchemaID != ContainmentEvidenceSchemaID || (item.Outcome != "requested" && item.Outcome != "completed" && item.Outcome != "partial" && item.Outcome != "unresolved") || !digestBoundRef(item.EventRef) {
		return errors.New("containment evidence shape invalid")
	}
	if !validExecutionEvidenceRef(item.ExecutionRef) || !digestBoundRef(item.ContainmentRef) {
		return errors.New("containment execution/scope references required")
	}
	return validateTypedEvidenceSchema(item, ContainmentEvidenceSchemaID)
}
func validateCompensationEvidence(item CompensationEvidence) error {
	if strings.TrimSpace(item.ReasonCode) == "" {
		return errors.New("compensation reason code required")
	}
	if err := validateEvidenceCommon(CompensationEvidenceSchemaID, item.EvidenceID, item.SchemaVersion, item.OccurredAt, item.FreshUntil, item.Outcome, item.Binding, item.Provenance, item.CanonicalContentDigest); err != nil {
		return err
	}
	if err := validateEvidenceIdentity("gait-compensation-", item.EvidenceID, item.CanonicalContentDigest); err != nil {
		return err
	}
	if item.SchemaID != CompensationEvidenceSchemaID || (item.Outcome != "required" && item.Outcome != "started" && item.Outcome != "completed") || !digestBoundRef(item.EventRef) {
		return errors.New("compensation evidence shape invalid")
	}
	if !validExecutionEvidenceRef(item.ExecutionRef) || !digestBoundRef(item.RequirementRef) {
		return errors.New("compensation execution/requirement references required")
	}
	return validateTypedEvidenceSchema(item, CompensationEvidenceSchemaID)
}

func validateEvidenceIdentity(prefix, id, digest string) error {
	hexDigest := strings.TrimPrefix(digest, "sha256:")
	if !validSHA256Digest(digest) || len(hexDigest) < 16 || id != prefix+hexDigest[:16] {
		return errors.New("evidence identity does not match canonical digest")
	}
	return nil
}

func validateTypedEvidenceSchema(value any, schemaID string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return errors.New("evidence schema validation failed")
	}
	if err := validateRuntimeSchema(raw, schemaID); err != nil {
		return fmt.Errorf("evidence schema validation failed: %w", err)
	}
	return nil
}

func NewExecutionEvidence(item ExecutionEvidence, private ed25519.PrivateKey) (ExecutionEvidence, error) {
	item.SchemaID, item.SchemaVersion = ExecutionEvidenceSchemaID, ExecutionEvidenceSchemaVersion
	item.Provenance.Producer, item.Provenance.Writer, item.Provenance.Verifier = EvidenceProducer, "gait-boundary", "gait-verifier"
	if item.ReasonCode == "" {
		item.ReasonCode = item.Outcome
	}
	if len(private) != ed25519.PrivateKeySize {
		return ExecutionEvidence{}, errors.New("evidence signing key invalid")
	}
	item.Provenance.PublicKey = base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	digest, sig, err := signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return ExecutionEvidence{}, err
	}
	item.EvidenceID = "gait-exec-" + strings.TrimPrefix(digest, "sha256:")[:16]
	digest, sig, err = signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return ExecutionEvidence{}, err
	}
	if err := validateExecutionEvidence(item); err != nil {
		return ExecutionEvidence{}, err
	}
	return item, nil
}
func NewEffectEvent(item EffectEvent, private ed25519.PrivateKey) (EffectEvent, error) {
	item.SchemaID, item.SchemaVersion = EffectEventSchemaID, ExecutionEvidenceSchemaVersion
	item.Provenance.Producer, item.Provenance.Writer, item.Provenance.Verifier = EvidenceProducer, "gait-boundary", "gait-verifier"
	if item.ReasonCode == "" {
		item.ReasonCode = item.Outcome
	}
	if len(private) != ed25519.PrivateKeySize {
		return EffectEvent{}, errors.New("evidence signing key invalid")
	}
	item.Provenance.PublicKey = base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	digest, sig, err := signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return EffectEvent{}, err
	}
	item.EvidenceID = "gait-effect-" + strings.TrimPrefix(digest, "sha256:")[:16]
	digest, sig, err = signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return EffectEvent{}, err
	}
	if err := validateEffectEvent(item); err != nil {
		return EffectEvent{}, err
	}
	return item, nil
}
func NewContainmentEvidence(item ContainmentEvidence, private ed25519.PrivateKey) (ContainmentEvidence, error) {
	item.SchemaID, item.SchemaVersion = ContainmentEvidenceSchemaID, ExecutionEvidenceSchemaVersion
	item.Provenance.Producer, item.Provenance.Writer, item.Provenance.Verifier = EvidenceProducer, "gait-boundary", "gait-verifier"
	if item.ReasonCode == "" {
		item.ReasonCode = item.Outcome
	}
	if len(private) != ed25519.PrivateKeySize {
		return ContainmentEvidence{}, errors.New("evidence signing key invalid")
	}
	item.Provenance.PublicKey = base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	digest, sig, err := signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return ContainmentEvidence{}, err
	}
	item.EvidenceID = "gait-containment-" + strings.TrimPrefix(digest, "sha256:")[:16]
	digest, sig, err = signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return ContainmentEvidence{}, err
	}
	if err := validateContainmentEvidence(item); err != nil {
		return ContainmentEvidence{}, err
	}
	return item, nil
}
func NewCompensationEvidence(item CompensationEvidence, private ed25519.PrivateKey) (CompensationEvidence, error) {
	item.SchemaID, item.SchemaVersion = CompensationEvidenceSchemaID, ExecutionEvidenceSchemaVersion
	item.Provenance.Producer, item.Provenance.Writer, item.Provenance.Verifier = EvidenceProducer, "gait-boundary", "gait-verifier"
	if item.ReasonCode == "" {
		item.ReasonCode = item.Outcome
	}
	if len(private) != ed25519.PrivateKeySize {
		return CompensationEvidence{}, errors.New("evidence signing key invalid")
	}
	item.Provenance.PublicKey = base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	digest, sig, err := signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return CompensationEvidence{}, err
	}
	item.EvidenceID = "gait-compensation-" + strings.TrimPrefix(digest, "sha256:")[:16]
	digest, sig, err = signEvidence(item, private)
	item.CanonicalContentDigest, item.Provenance.Signature = digest, sig
	if err != nil {
		return CompensationEvidence{}, err
	}
	if err := validateCompensationEvidence(item); err != nil {
		return CompensationEvidence{}, err
	}
	return item, nil
}

func ParseExecutionEvidence(raw []byte) (ExecutionEvidence, error) {
	var item ExecutionEvidence
	if err := DecodeStrictRuntimeJSON(raw, &item); err != nil {
		return item, err
	}
	if err := validateExecutionEvidence(item); err != nil {
		return item, err
	}
	return item, nil
}
func ParseEffectEvent(raw []byte) (EffectEvent, error) {
	var item EffectEvent
	if err := DecodeStrictRuntimeJSON(raw, &item); err != nil {
		return item, err
	}
	if err := validateEffectEvent(item); err != nil {
		return item, err
	}
	return item, nil
}
func ParseContainmentEvidence(raw []byte) (ContainmentEvidence, error) {
	var item ContainmentEvidence
	if err := DecodeStrictRuntimeJSON(raw, &item); err != nil {
		return item, err
	}
	if err := validateContainmentEvidence(item); err != nil {
		return item, err
	}
	return item, nil
}
func ParseCompensationEvidence(raw []byte) (CompensationEvidence, error) {
	var item CompensationEvidence
	if err := DecodeStrictRuntimeJSON(raw, &item); err != nil {
		return item, err
	}
	if err := validateCompensationEvidence(item); err != nil {
		return item, err
	}
	return item, nil
}

// WriteEvidenceAtomic and ReadEvidenceFile are bounded, no-follow helpers for
// evidence handoff. The writer anchors all path operations to a verified
// directory descriptor. The reader accepts an already-open descriptor and
// never resolves a path itself.
func WriteEvidenceAtomic(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("evidence path required")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if int64(len(raw)) > MaxEvidenceBytes {
		return errors.New("evidence exceeds size limit")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return errors.New("evidence path invalid")
	}
	if err := rejectRuntimeAncestors(abs); err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	root, err := openVerifiedEvidenceRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	base := filepath.Base(abs)
	if _, err := root.Lstat(base); err == nil {
		return errors.New("evidence destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("evidence destination is not stable")
	}
	tempName, err := randomEvidenceTempName(base)
	if err != nil {
		return err
	}
	tmp, err := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(tempName) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := root.Lstat(base); err == nil {
		return errors.New("evidence destination changed during write")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("evidence destination is not stable")
	}
	if err := root.Link(tempName, base); err != nil {
		return err
	}
	return root.Remove(tempName)
}

func ReadEvidenceFile(file *os.File) ([]byte, error) {
	if file == nil {
		return nil, errors.New("evidence descriptor required")
	}
	descriptor, err := file.Stat()
	if err != nil || !descriptor.Mode().IsRegular() || descriptor.Size() > MaxEvidenceBytes {
		return nil, errors.New("evidence descriptor is not a bounded regular file")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("evidence descriptor is unreadable")
	}
	first, err := io.ReadAll(io.LimitReader(file, MaxEvidenceBytes+1))
	if err != nil || int64(len(first)) > MaxEvidenceBytes {
		return nil, errors.New("evidence descriptor is unreadable")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("evidence descriptor is unreadable")
	}
	second, err := io.ReadAll(io.LimitReader(file, MaxEvidenceBytes+1))
	if err != nil || !bytes.Equal(first, second) {
		return nil, errors.New("evidence descriptor changed during read")
	}
	return first, nil
}

func openVerifiedEvidenceRoot(dir string) (*os.Root, error) {
	initial, err := os.Lstat(dir)
	if err != nil || initial.Mode()&os.ModeSymlink != 0 || !initial.IsDir() {
		return nil, errors.New("evidence directory is unsafe")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, errors.New("evidence directory is unsafe")
	}
	anchored, err := root.Stat(".")
	if err != nil || !os.SameFile(initial, anchored) {
		_ = root.Close()
		return nil, errors.New("evidence directory changed during open")
	}
	return root, nil
}

func randomEvidenceTempName(base string) (string, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", errors.New("evidence temporary name unavailable")
	}
	return "." + base + ".gait-tmp-" + hex.EncodeToString(suffix[:]), nil
}
