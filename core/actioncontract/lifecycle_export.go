package actioncontract

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Clyra-AI/gait/core/fsx"
	proof "github.com/Clyra-AI/proof"
	proofsign "github.com/Clyra-AI/proof/signing"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const LifecycleReceiptSchemaID = "https://gait.dev/schemas/v1/action-contract/lifecycle-receipt.schema.json"

type LifecycleReceiptProvenance struct {
	SourceProduct string              `json:"source_product"`
	Mode          string              `json:"mode"`
	PublicKey     string              `json:"public_key"`
	Signature     proofsign.Signature `json:"signature"`
}
type LifecycleReceipt struct {
	SchemaID               string                                   `json:"schema_id"`
	SchemaVersion          string                                   `json:"schema_version"`
	ReceiptID              string                                   `json:"receipt_id"`
	ContractFamilyID       string                                   `json:"contract_family_id"`
	ContractID             string                                   `json:"contract_id"`
	Revision               int                                      `json:"revision"`
	ArtifactDigests        []string                                 `json:"artifact_digests"`
	ArtifactRefs           []proof.RelationshipRef                  `json:"artifact_refs,omitempty"`
	ObservedAt             string                                   `json:"observed_at,omitempty"`
	FreshUntil             string                                   `json:"fresh_until,omitempty"`
	Correlation            proof.ControlContainmentTelemetryProfile `json:"correlation"`
	Authority              string                                   `json:"authority"`
	Quarantine             bool                                     `json:"quarantine"`
	Redaction              string                                   `json:"redaction"`
	Outcome                string                                   `json:"outcome"`
	ReasonCodes            []string                                 `json:"reason_codes,omitempty"`
	Provenance             LifecycleReceiptProvenance               `json:"provenance"`
	CanonicalContentDigest string                                   `json:"canonical_content_digest"`
}

func normalizeLifecycleReceipt(r *LifecycleReceipt) error {
	if strings.TrimSpace(r.ContractFamilyID) == "" || strings.TrimSpace(r.ContractID) == "" || r.Revision < 1 {
		return errors.New("receipt contract binding invalid")
	}
	if r.Authority != "authoritative" && r.Authority != "non_authoritative" {
		return errors.New("receipt authority invalid")
	}
	if r.Authority == "authoritative" && r.Quarantine {
		return errors.New("authoritative receipt quarantined")
	}
	if r.Redaction != "reference_only" && r.Redaction != "hash_only" {
		return errors.New("receipt redaction invalid")
	}
	if r.Outcome != "unknown" && r.Outcome != "succeeded" && r.Outcome != "failed" && r.Outcome != "blocked" && r.Outcome != "partial" && r.Outcome != "unresolved" && r.Outcome != "out_of_scope" {
		return errors.New("receipt outcome invalid")
	}
	if r.ObservedAt == "" || r.FreshUntil == "" || !validEvidenceTime(r.ObservedAt) || !validEvidenceTime(r.FreshUntil) || !mustParseTime(r.FreshUntil).After(mustParseTime(r.ObservedAt)) {
		return errors.New("receipt freshness invalid")
	}
	if len(r.ArtifactDigests) == 0 {
		return errors.New("receipt artifact digests missing")
	}
	sort.Strings(r.ArtifactDigests)
	sort.Strings(r.ReasonCodes)
	if hasDuplicateStrings(r.ArtifactDigests) || hasDuplicateStrings(r.ReasonCodes) {
		return errors.New("receipt duplicate digest/reason")
	}
	for _, d := range r.ArtifactDigests {
		if !validCanonicalDigest(d) {
			return errors.New("receipt digest invalid")
		}
	}
	for i := range r.ArtifactRefs {
		if !fullControlRef(r.ArtifactRefs[i]) {
			return errors.New("receipt artifact ref invalid")
		}
	}
	sort.Slice(r.ArtifactRefs, func(i, j int) bool {
		a, b := r.ArtifactRefs[i], r.ArtifactRefs[j]
		return a.Kind+"|"+a.ID+"|"+a.Digest+"|"+a.SchemaID+"|"+a.SchemaVersion+"|"+a.SourceProduct < b.Kind+"|"+b.ID+"|"+b.Digest+"|"+b.SchemaID+"|"+b.SchemaVersion+"|"+b.SourceProduct
	})
	if len(r.ArtifactRefs) > 0 {
		if len(r.ArtifactRefs) != len(r.ArtifactDigests) {
			return errors.New("receipt artifact set mismatch")
		}
		identityCounts := map[string]int{}
		refDigestCounts := map[string]int{}
		seenDigest := map[string]bool{}
		for _, ref := range r.ArtifactRefs {
			key := ref.Kind + "|" + ref.ID + "|" + ref.SchemaID + "|" + ref.SchemaVersion + "|" + ref.SourceProduct
			if identityCounts[key] > 0 || seenDigest[ref.Digest] {
				return errors.New("receipt artifact ref duplicate")
			}
			identityCounts[key]++
			refDigestCounts[ref.Digest]++
			seenDigest[ref.Digest] = true
		}
		digestCounts := map[string]int{}
		for _, d := range r.ArtifactDigests {
			digestCounts[d]++
		}
		for d, n := range digestCounts {
			if refDigestCounts[d] != n {
				return errors.New("receipt artifact set mismatch")
			}
		}
	}
	if err := r.Correlation.Validate(); err != nil {
		return errors.New("receipt correlation invalid")
	}
	if r.Correlation.ContractRef == nil || r.Correlation.ContractRef.ID != r.ContractID || r.Correlation.BindingMode != proof.BindingModeDigestBound {
		return errors.New("receipt correlation contract mismatch")
	}
	found := false
	for _, d := range r.ArtifactDigests {
		if d == r.Correlation.ContractRef.Digest {
			found = true
		}
	}
	if !found {
		return errors.New("receipt correlation digest missing")
	}
	return nil
}

func (r LifecycleReceipt) Sign(key ed25519.PrivateKey) (LifecycleReceipt, error) {
	if len(key) != ed25519.PrivateKeySize {
		return r, errors.New("receipt signing key required")
	}
	r.SchemaID = LifecycleReceiptSchemaID
	r.SchemaVersion = "1"
	r.Provenance = LifecycleReceiptProvenance{SourceProduct: "gait", Mode: "signed_local"}
	if err := normalizeLifecycleReceipt(&r); err != nil {
		return r, err
	}
	c := r
	c.ReceiptID = ""
	c.CanonicalContentDigest = ""
	c.Provenance = LifecycleReceiptProvenance{}
	raw, e := json.Marshal(c)
	if e != nil {
		return r, e
	}
	d, e := canonicalDigest(raw)
	if e != nil {
		return r, e
	}
	r.CanonicalContentDigest = d
	r.ReceiptID = "gait-receipt-" + strings.TrimPrefix(d, "sha256:")[:16]
	r.Provenance.PublicKey = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	r.Provenance.Signature, e = proofsign.SignDigestHex(key, strings.TrimPrefix(d, "sha256:"))
	if raw, e := json.Marshal(r); e != nil || validateRuntimeSchema(raw, LifecycleReceiptSchemaID) != nil {
		return r, errors.New("receipt schema invalid")
	}
	return r, e
}
func VerifyLifecycleReceipt(r LifecycleReceipt, key ed25519.PublicKey) error {
	if err := normalizeLifecycleReceipt(&r); err != nil {
		return err
	}
	if raw, e := json.Marshal(r); e != nil || validateRuntimeSchema(raw, LifecycleReceiptSchemaID) != nil {
		return errors.New("receipt schema invalid")
	}
	if r.SchemaID != LifecycleReceiptSchemaID || r.SchemaVersion != "1" || r.Provenance.Mode != "signed_local" || r.Provenance.SourceProduct != "gait" {
		return errors.New("lifecycle_receipt_schema_invalid")
	}
	c := r
	c.ReceiptID = ""
	c.CanonicalContentDigest = ""
	c.Provenance = LifecycleReceiptProvenance{}
	raw, e := json.Marshal(c)
	if e != nil {
		return e
	}
	d, e := canonicalDigest(raw)
	if e != nil || d != r.CanonicalContentDigest {
		return errors.New("lifecycle_receipt_digest_mismatch")
	}
	if len(key) != ed25519.PublicKeySize || r.Provenance.Signature.KeyID != proofsign.KeyID(key) {
		return errors.New("lifecycle_receipt_key_mismatch")
	}
	declared, e := base64.StdEncoding.DecodeString(r.Provenance.PublicKey)
	if e != nil || !bytes.Equal(declared, key) {
		return errors.New("receipt public key mismatch")
	}
	if r.Provenance.Signature.SignedDigest != strings.TrimPrefix(d, "sha256:") {
		return errors.New("receipt signed digest mismatch")
	}
	ok, e := proofsign.VerifyDigestHex(key, r.Provenance.Signature)
	if e != nil || !ok {
		return errors.New("lifecycle_receipt_signature_invalid")
	}
	if r.ReceiptID != "gait-receipt-"+strings.TrimPrefix(r.CanonicalContentDigest, "sha256:")[:16] {
		return errors.New("receipt id mismatch")
	}
	return nil
}

func VerifyLifecycleReceiptAt(r LifecycleReceipt, key ed25519.PublicKey, at time.Time) error {
	if at.IsZero() {
		return errors.New("receipt evaluation time required")
	}
	if err := VerifyLifecycleReceipt(r, key); err != nil {
		return err
	}
	if r.FreshUntil != "" && !at.Before(mustParseTime(r.FreshUntil)) {
		return errors.New("lifecycle_receipt_stale")
	}
	return nil
}

var otelIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)
var otelTracePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var otelSpanPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

type LifecycleOTelEvent struct {
	Name             string             `json:"name"`
	Time             string             `json:"time"`
	TraceID          string             `json:"trace_id"`
	SpanID           string             `json:"span_id"`
	Kind             LifecycleEventKind `json:"kind"`
	ContractFamilyID string             `json:"contract_family_id,omitempty"`
	ContractID       string             `json:"contract_id,omitempty"`
	Revision         int                `json:"revision,omitempty"`
	Outcome          string             `json:"outcome,omitempty"`
	ReasonCodes      []string           `json:"reason_codes,omitempty"`
	BoundaryID       string             `json:"boundary_id,omitempty"`
	ResourceID       string             `json:"resource_id,omitempty"`
	AffectedScope    []string           `json:"affected_scope,omitempty"`
	BindingMode      string             `json:"binding_mode"`
	Authority        bool               `json:"authority"`
	Quarantine       bool               `json:"quarantine"`
	SourceProduct    string             `json:"source_product"`
	SourceVersion    string             `json:"source_version"`
	RecordID         string             `json:"record_id"`
	ContractDigest   string             `json:"contract_digest,omitempty"`
	ActivationDigest string             `json:"activation_digest,omitempty"`
	PolicyDigest     string             `json:"policy_digest,omitempty"`
	EvidenceDigests  []string           `json:"evidence_digests,omitempty"`
	ControlDigest    string             `json:"control_digest,omitempty"`
}

type LifecycleOTelExportOptions struct {
	SourceVersion string
	Authority     bool
	Quarantine    bool
	MaxBytes      int
}

func ExportLifecycleOTel(path string, records []LifecycleRecord, sourceVersion string) error {
	return ExportLifecycleOTelWithOptions(path, records, LifecycleOTelExportOptions{SourceVersion: sourceVersion, Quarantine: true})
}
func ExportLifecycleOTelWithOptions(path string, records []LifecycleRecord, opts LifecycleOTelExportOptions) error {
	if path == "" {
		return errors.New("lifecycle_export_disabled")
	}
	if strings.TrimSpace(opts.SourceVersion) == "" {
		return errors.New("lifecycle_export_source_version_required")
	}
	events := make([]LifecycleOTelEvent, 0, len(records))
	for _, r := range records {
		if !otelIDPattern.MatchString(r.ContractRef.ID) || !otelIDPattern.MatchString(r.RecordID) || !otelTracePattern.MatchString(r.Correlation.TraceID) || !otelSpanPattern.MatchString(r.Correlation.SpanID) || !validEvidenceTime(r.OccurredAt) {
			return errors.New("lifecycle_export_identifier_invalid")
		}
		if opts.Authority && opts.Quarantine {
			return errors.New("lifecycle_export_authority_quarantine_conflict")
		}
		if err := r.Correlation.Validate(); err != nil {
			return errors.New("lifecycle_export_correlation_invalid")
		}
		if !fullControlRef(r.ContractRef) {
			return errors.New("lifecycle_export_contract_ref_invalid")
		}
		e := LifecycleOTelEvent{Name: "gait.lifecycle." + string(r.Kind), Time: r.OccurredAt, TraceID: r.Correlation.TraceID, SpanID: r.Correlation.SpanID, Kind: r.Kind, RecordID: r.RecordID, ContractFamilyID: r.ContractFamilyID, ContractID: r.ContractRef.ID, ContractDigest: r.ContractRef.Digest, Revision: r.Revision, ReasonCodes: append([]string{}, r.ReasonCodes...), BindingMode: string(r.Correlation.BindingMode), Authority: opts.Authority, Quarantine: opts.Quarantine, SourceProduct: "gait", SourceVersion: opts.SourceVersion}
		sort.Strings(e.ReasonCodes)
		if r.Execution != nil {
			e.Outcome = r.Execution.Outcome
		}
		if r.Effect != nil {
			e.Outcome = r.Effect.Outcome
		}
		if r.Containment != nil {
			e.Outcome = r.Containment.Outcome
		}
		if r.Control != nil {
			e.Outcome = r.Control.Phase
			e.ReasonCodes = append(e.ReasonCodes, r.Control.ReasonCode)
		}
		if r.ActivationRef != nil {
			e.ActivationDigest = r.ActivationRef.Digest
		}
		if r.Decision != nil {
			e.PolicyDigest = r.Decision.PolicyDigest
		}
		if r.Control != nil {
			e.ControlDigest = r.Control.CanonicalContentDigest
			e.BoundaryID = r.Control.BoundaryID
			e.ResourceID = r.Control.ResourceID
			e.AffectedScope = append(e.AffectedScope, r.Control.AffectedScope...)
		}
		e.AffectedScope = append([]string{}, r.AffectedScope...)
		sort.Strings(e.AffectedScope)
		events = append(events, e)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time+"|"+events[i].Name < events[j].Time+"|"+events[j].Name })
	raw := make([]byte, 0)
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		raw = append(raw, b...)
		raw = append(raw, '\n')
		if opts.MaxBytes > 0 && len(raw) > opts.MaxBytes {
			return errors.New("lifecycle_export_size_limit")
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent := filepath.Dir(abs)
	if err := safeExportAncestors(parent); err != nil {
		return err
	}
	if info, e := os.Lstat(abs); e == nil && !info.Mode().IsRegular() {
		return errors.New("lifecycle_export_destination_invalid")
	}
	if err := fsx.WriteFileAtomic(abs, raw, 0600); err != nil {
		return fmt.Errorf("lifecycle export failed: %w", err)
	}
	return nil
}

func safeExportAncestors(path string) error {
	i, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if i.Mode()&os.ModeSymlink != 0 {
		return errors.New("lifecycle_export_symlink_parent")
	}
	return nil
}
