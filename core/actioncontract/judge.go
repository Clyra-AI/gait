package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	proof "github.com/Clyra-AI/proof"
	proofcanon "github.com/Clyra-AI/proof/canon"
	proofsign "github.com/Clyra-AI/proof/signing"
	"sort"
	"strings"
)

const AdvisoryReportSchemaID = "https://gait.dev/schemas/v1/action-contract/advisory-evaluator-report.schema.json"

type AdvisoryInput struct {
	ActionID          string                  `json:"action_id"`
	Claims            []string                `json:"claims,omitempty"`
	EvidenceRefs      []proof.RelationshipRef `json:"evidence_refs,omitempty"`
	ContractDigest    string                  `json:"contract_digest,omitempty"`
	CorrelationDigest string                  `json:"correlation_digest,omitempty"`
}
type AdvisoryProvenance struct {
	SourceProduct string              `json:"source_product"`
	Mode          string              `json:"mode"`
	Provider      string              `json:"provider"`
	PublicKey     string              `json:"public_key"`
	Signature     proofsign.Signature `json:"signature"`
}
type AdvisoryReport struct {
	SchemaID               string                  `json:"schema_id"`
	SchemaVersion          string                  `json:"schema_version"`
	ProviderName           string                  `json:"provider_name"`
	ProviderVersion        string                  `json:"provider_version"`
	Status                 string                  `json:"status"`
	ReasonCodes            []string                `json:"reason_codes,omitempty"`
	AdvisoryOnly           bool                    `json:"advisory_only"`
	ActionID               string                  `json:"action_id"`
	ContractDigest         string                  `json:"contract_digest,omitempty"`
	CorrelationDigest      string                  `json:"correlation_digest,omitempty"`
	EvidenceRefs           []proof.RelationshipRef `json:"evidence_refs,omitempty"`
	Findings               []string                `json:"findings,omitempty"`
	Provenance             AdvisoryProvenance      `json:"provenance"`
	CanonicalContentDigest string                  `json:"canonical_content_digest"`
}

func normalizeAdvisory(r *AdvisoryReport) error {
	if r.Status != "pass" && r.Status != "review" && r.Status != "inconclusive" {
		return errors.New("advisory status invalid")
	}
	if r.ProviderName == "" || r.ProviderVersion == "" || r.ActionID == "" {
		return errors.New("advisory metadata missing")
	}
	for _, d := range []string{r.ContractDigest, r.CorrelationDigest} {
		if d != "" && !validCanonicalDigest(d) {
			return errors.New("advisory binding digest invalid")
		}
	}
	if hasDuplicateStrings(r.Findings) || hasDuplicateStrings(r.ReasonCodes) {
		return errors.New("advisory duplicate findings/reasons")
	}
	sort.Strings(r.Findings)
	sort.Strings(r.ReasonCodes)
	sort.Slice(r.EvidenceRefs, func(i, j int) bool {
		return r.EvidenceRefs[i].Kind+"|"+r.EvidenceRefs[i].ID < r.EvidenceRefs[j].Kind+"|"+r.EvidenceRefs[j].ID
	})
	for i := 1; i < len(r.EvidenceRefs); i++ {
		if r.EvidenceRefs[i].Kind == r.EvidenceRefs[i-1].Kind && r.EvidenceRefs[i].ID == r.EvidenceRefs[i-1].ID {
			return errors.New("advisory evidence duplicate")
		}
	}
	return r.VerifyEvidenceRefs()
}

type AdvisoryEvaluator interface {
	Evaluate(AdvisoryInput) (AdvisoryReport, error)
}
type OfflineAdvisoryEvaluator struct{}

func (OfflineAdvisoryEvaluator) Evaluate(in AdvisoryInput) (AdvisoryReport, error) {
	if strings.TrimSpace(in.ActionID) == "" {
		return AdvisoryReport{}, errors.New("action_id required")
	}
	findings := []string{}
	for _, c := range in.Claims {
		if strings.TrimSpace(c) != "" {
			findings = append(findings, "claim_present")
		}
	}
	sort.Strings(findings)
	status := "pass"
	if len(findings) > 0 {
		status = "review"
	}
	return AdvisoryReport{SchemaID: AdvisoryReportSchemaID, SchemaVersion: "1", ProviderName: "offline", ProviderVersion: "1", Status: status, ReasonCodes: []string{"advisory_offline_evaluator"}, AdvisoryOnly: true, ActionID: in.ActionID, ContractDigest: in.ContractDigest, CorrelationDigest: in.CorrelationDigest, EvidenceRefs: in.EvidenceRefs, Findings: findings}, nil
}
func canonicalAdvisory(r AdvisoryReport) ([]byte, error) {
	c := r
	c.CanonicalContentDigest = ""
	c.Provenance.Signature = proofsign.Signature{}
	c.Provenance.PublicKey = ""
	raw, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	return proofcanon.CanonicalizeJSON(raw)
}
func (r AdvisoryReport) Sign(key ed25519.PrivateKey) (AdvisoryReport, error) {
	if len(key) != ed25519.PrivateKeySize {
		return AdvisoryReport{}, errors.New("advisory signing key required")
	}
	if !r.AdvisoryOnly {
		return AdvisoryReport{}, errors.New("advisory_only must be true")
	}
	if err := normalizeAdvisory(&r); err != nil {
		return AdvisoryReport{}, err
	}
	r.Provenance = AdvisoryProvenance{SourceProduct: "gait", Mode: "non_authoritative", Provider: r.ProviderName}
	raw, e := canonicalAdvisory(r)
	if e != nil {
		return AdvisoryReport{}, e
	}
	d, e := canonicalDigest(raw)
	if e != nil {
		return AdvisoryReport{}, e
	}
	r.CanonicalContentDigest = d
	r.Provenance.PublicKey = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	r.Provenance.Signature, e = proofsign.SignDigestHex(key, strings.TrimPrefix(d, "sha256:"))
	if r.Provenance.Signature.SignedDigest != strings.TrimPrefix(d, "sha256:") {
		return AdvisoryReport{}, errors.New("advisory signature digest mismatch")
	}
	if raw, e := json.Marshal(r); e != nil || validateRuntimeSchema(raw, AdvisoryReportSchemaID) != nil {
		return AdvisoryReport{}, errors.New("advisory schema invalid")
	}
	return r, e
}
func VerifyAdvisoryReport(r AdvisoryReport, trusted ed25519.PublicKey, expectedContract, expectedCorrelation string) error {
	if r.SchemaID != AdvisoryReportSchemaID || r.SchemaVersion != "1" || !r.AdvisoryOnly {
		return errors.New("advisory schema or authority invalid")
	}
	if err := normalizeAdvisory(&r); err != nil {
		return err
	}
	if r.ProviderName == "" || r.ProviderVersion == "" {
		return errors.New("advisory provider metadata missing")
	}
	if expectedContract != "" && r.ContractDigest != expectedContract {
		return errors.New("advisory contract binding mismatch")
	}
	if expectedCorrelation != "" && r.CorrelationDigest != expectedCorrelation {
		return errors.New("advisory correlation binding mismatch")
	}
	if len(trusted) != ed25519.PublicKeySize {
		return errors.New("trusted advisory key required")
	}
	declared, err := base64.StdEncoding.DecodeString(r.Provenance.PublicKey)
	if err != nil || !bytes.Equal(declared, trusted) {
		return errors.New("advisory public key mismatch")
	}
	if err := r.VerifyEvidenceRefs(); err != nil {
		return err
	}
	raw, e := canonicalAdvisory(r)
	if e != nil {
		return e
	}
	d, e := canonicalDigest(raw)
	if e != nil || d != r.CanonicalContentDigest {
		return errors.New("advisory canonical digest mismatch")
	}
	if r.Provenance.SourceProduct != "gait" || r.Provenance.Mode != "non_authoritative" {
		return errors.New("advisory provenance is authoritative")
	}
	if r.Provenance.Provider != r.ProviderName || r.Provenance.Signature.SignedDigest != strings.TrimPrefix(r.CanonicalContentDigest, "sha256:") {
		return errors.New("advisory provenance mismatch")
	}
	if raw, e := json.Marshal(r); e != nil || validateRuntimeSchema(raw, AdvisoryReportSchemaID) != nil {
		return errors.New("advisory schema invalid")
	}
	if r.Provenance.Signature.KeyID != proofsign.KeyID(trusted) {
		return errors.New("advisory key mismatch")
	}
	ok, err := proofsign.VerifyDigestHex(trusted, r.Provenance.Signature)
	if err != nil || !ok {
		return errors.New("advisory signature invalid")
	}
	return nil
}
func (r AdvisoryReport) VerifyEvidenceRefs() error {
	for _, ref := range r.EvidenceRefs {
		if strings.TrimSpace(ref.Kind) == "" || strings.TrimSpace(ref.ID) == "" || !validCanonicalDigest(ref.Digest) || strings.TrimSpace(ref.SchemaID) == "" || strings.TrimSpace(ref.SchemaVersion) == "" || strings.TrimSpace(ref.SourceProduct) == "" {
			return errors.New("advisory evidence reference must be digest-bound")
		}
	}
	return nil
}
func (r AdvisoryReport) MarshalDeterministic() ([]byte, error) { return json.Marshal(r) }
