package actioncontract

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	proof "github.com/Clyra-AI/proof"
	proofsign "github.com/Clyra-AI/proof/signing"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ControlEventEvidenceSchemaID = "https://gait.dev/schemas/v1/action-contract/control-event-evidence.schema.json"

type ControlEventEvidence struct {
	SchemaID               string                `json:"schema_id"`
	SchemaVersion          string                `json:"schema_version"`
	EvidenceID             string                `json:"evidence_id"`
	Binding                EvidenceBinding       `json:"binding"`
	EventRef               proof.RelationshipRef `json:"event_ref"`
	CausalRef              proof.RelationshipRef `json:"causal_ref"`
	ControlRef             proof.RelationshipRef `json:"control_ref"`
	Command                string                `json:"command"`
	Phase                  string                `json:"phase"`
	BoundaryID             string                `json:"boundary_id,omitempty"`
	ResourceID             string                `json:"resource_id,omitempty"`
	AffectedScope          []string              `json:"affected_scope,omitempty"`
	AdapterIdentity        string                `json:"adapter_identity,omitempty"`
	AdapterAcknowledged    bool                  `json:"adapter_acknowledged"`
	OccurredAt             string                `json:"occurred_at"`
	FreshUntil             string                `json:"fresh_until"`
	ReasonCode             string                `json:"reason_code"`
	Provenance             EvidenceProvenance    `json:"provenance"`
	CanonicalContentDigest string                `json:"canonical_content_digest"`
}

func NewControlEventEvidence(item ControlEventEvidence, key ed25519.PrivateKey) (ControlEventEvidence, error) {
	if len(key) != ed25519.PrivateKeySize {
		return item, errors.New("control signing key invalid")
	}
	item.SchemaID = ControlEventEvidenceSchemaID
	item.SchemaVersion = ExecutionEvidenceSchemaVersion
	if item.Command == "" || item.Phase == "" || item.ReasonCode == "" || !validEvidenceTime(item.OccurredAt) || !validEvidenceTime(item.FreshUntil) {
		return item, errors.New("control evidence fields invalid")
	}
	if !validControlPhase(item.Command, item.Phase) || item.BoundaryID == "" || item.ResourceID == "" || item.AdapterIdentity == "" || len(item.AffectedScope) == 0 {
		return item, errors.New("control metadata invalid")
	}
	for i := range item.AffectedScope {
		item.AffectedScope[i] = strings.TrimSpace(item.AffectedScope[i])
		if item.AffectedScope[i] == "" {
			return item, errors.New("control scope empty")
		}
	}
	sort.Strings(item.AffectedScope)
	if hasDuplicateStrings(item.AffectedScope) {
		return item, errors.New("control scope duplicate")
	}
	item.AffectedScope = append([]string{}, item.AffectedScope...)
	sort.Strings(item.AffectedScope)
	if !fullControlRef(item.EventRef) || !fullControlRef(item.CausalRef) || !fullControlRef(item.ControlRef) {
		return item, errors.New("control ref incomplete")
	}
	if (item.Phase == "acknowledged" || item.Phase == "invalidated") && !item.AdapterAcknowledged {
		return item, errors.New("control adapter acknowledgement required")
	}
	if (item.Phase == "requested" || item.Phase == "attempted") && item.AdapterAcknowledged {
		return item, errors.New("control adapter acknowledgement premature")
	}
	if !mustParseTime(item.FreshUntil).After(mustParseTime(item.OccurredAt)) {
		return item, errors.New("control freshness invalid")
	}
	if err := item.Binding.Validate(); err != nil {
		return item, err
	}
	if !digestBoundRef(item.EventRef) || !digestBoundRef(item.CausalRef) || !digestBoundRef(item.ControlRef) {
		return item, errors.New("control refs must be digest-bound")
	}
	item.Provenance = EvidenceProvenance{Producer: EvidenceProducer, Writer: "gait-control", Verifier: "gait-verifier", PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
	signed, e := signControl(item, key)
	if e != nil {
		return item, e
	}
	if raw, e := json.Marshal(signed); e != nil || validateRuntimeSchema(raw, ControlEventEvidenceSchemaID) != nil {
		return item, errors.New("control schema invalid")
	}
	return signed, nil
}
func signControl(item ControlEventEvidence, key ed25519.PrivateKey) (ControlEventEvidence, error) {
	d, err := canonicalEvidence(item)
	if err != nil {
		return item, err
	}
	item.CanonicalContentDigest = d
	item.EvidenceID = "gait-control-" + strings.TrimPrefix(d, "sha256:")[:16]
	sig, err := proofsign.SignDigestHex(key, strings.TrimPrefix(d, "sha256:"))
	item.Provenance.Signature = sig
	return item, err
}
func ParseControlEventEvidence(raw []byte) (ControlEventEvidence, error) {
	var item ControlEventEvidence
	if err := DecodeStrictRuntimeJSON(raw, &item); err != nil {
		return item, err
	}
	if item.SchemaID != ControlEventEvidenceSchemaID || item.SchemaVersion != ExecutionEvidenceSchemaVersion {
		return item, errors.New("control schema invalid")
	}
	if validateRuntimeSchema(raw, ControlEventEvidenceSchemaID) != nil {
		return item, errors.New("control schema invalid")
	}
	return item, nil
}
func VerifyControlEventEvidence(item ControlEventEvidence, key ed25519.PublicKey) (bool, error) {
	if item.SchemaID != ControlEventEvidenceSchemaID || item.SchemaVersion != ExecutionEvidenceSchemaVersion {
		return false, errors.New("control schema invalid")
	}
	if item.Command == "" || item.Phase == "" || strings.TrimSpace(item.ReasonCode) == "" || !validControlPhase(item.Command, item.Phase) || item.BoundaryID == "" || item.ResourceID == "" || item.AdapterIdentity == "" || len(item.AffectedScope) == 0 || hasDuplicateStrings(item.AffectedScope) || !fullControlRef(item.EventRef) || !fullControlRef(item.CausalRef) || !fullControlRef(item.ControlRef) {
		return false, errors.New("control fields invalid")
	}
	if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(item.CanonicalContentDigest) || item.EvidenceID != "gait-control-"+strings.TrimPrefix(item.CanonicalContentDigest, "sha256:")[:16] {
		return false, errors.New("control evidence id invalid")
	}
	if !validEvidenceTime(item.OccurredAt) || !validEvidenceTime(item.FreshUntil) || !mustParseTime(item.FreshUntil).After(mustParseTime(item.OccurredAt)) {
		return false, errors.New("control timestamp invalid")
	}
	if (item.Phase == "acknowledged" || item.Phase == "invalidated") && !item.AdapterAcknowledged {
		return false, errors.New("control adapter acknowledgement required")
	}
	if (item.Phase == "requested" || item.Phase == "attempted") && item.AdapterAcknowledged {
		return false, errors.New("control adapter acknowledgement premature")
	}
	if hasDuplicateStrings(item.AffectedScope) {
		return false, errors.New("control scope duplicate")
	}
	for _, s := range item.AffectedScope {
		if strings.TrimSpace(s) == "" {
			return false, errors.New("control scope empty")
		}
	}
	if raw, e := json.Marshal(item); e != nil || validateRuntimeSchema(raw, ControlEventEvidenceSchemaID) != nil {
		return false, errors.New("control schema invalid")
	}
	if item.FreshUntil != "" {
		a, _ := time.Parse(time.RFC3339Nano, item.OccurredAt)
		b, _ := time.Parse(time.RFC3339Nano, item.FreshUntil)
		if b.Before(a) {
			return false, errors.New("control freshness invalid")
		}
	}
	if err := item.Binding.Validate(); err != nil {
		return false, err
	}
	if !digestBoundRef(item.EventRef) || !digestBoundRef(item.CausalRef) || !digestBoundRef(item.ControlRef) {
		return false, errors.New("control refs invalid")
	}
	return verifyEvidence(item.SchemaID, item.CanonicalContentDigest, item.Provenance, item, key, func() error { return nil })
}
func VerifyControlEventEvidenceAt(item ControlEventEvidence, key ed25519.PublicKey, at time.Time) (bool, error) {
	ok, e := VerifyControlEventEvidence(item, key)
	if e != nil || !ok {
		return ok, e
	}
	if !at.IsZero() && !at.Before(mustParseTime(item.FreshUntil)) {
		return false, errors.New("control evidence stale")
	}
	return true, nil
}
func validControlPhase(command, phase string) bool {
	switch command {
	case "stop":
		return phase == "requested" || phase == "acknowledged" || phase == "denied" || phase == "failed"
	case "external_revocation":
		return phase == "attempted" || phase == "acknowledged" || phase == "failed"
	case "capability_invalidation", "descendant_invalidation":
		return phase == "invalidated"
	}
	return false
}
func hasDuplicateStrings(v []string) bool {
	for i := 1; i < len(v); i++ {
		if v[i] == v[i-1] {
			return true
		}
	}
	return false
}
func fullControlRef(r proof.RelationshipRef) bool {
	return digestBoundRef(r) && r.SchemaID != "" && r.SchemaVersion != "" && r.SourceProduct != ""
}
