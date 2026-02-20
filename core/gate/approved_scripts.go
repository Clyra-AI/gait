package gate

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
)

const (
	approvedScriptSchemaID = "gait.gate.approved_script_entry"
	approvedScriptSchemaV1 = "1.0.0"
)

type ApprovedScriptMatch struct {
	Matched   bool
	PatternID string
	Reason    string
}

func NormalizeApprovedScriptEntry(input schemagate.ApprovedScriptEntry) (schemagate.ApprovedScriptEntry, error) {
	output := input
	if strings.TrimSpace(output.SchemaID) == "" {
		output.SchemaID = approvedScriptSchemaID
	}
	if strings.TrimSpace(output.SchemaID) != approvedScriptSchemaID {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("unsupported approved script schema_id: %s", output.SchemaID)
	}
	if strings.TrimSpace(output.SchemaVersion) == "" {
		output.SchemaVersion = approvedScriptSchemaV1
	}
	if strings.TrimSpace(output.SchemaVersion) != approvedScriptSchemaV1 {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("unsupported approved script schema_version: %s", output.SchemaVersion)
	}
	output.PatternID = strings.TrimSpace(output.PatternID)
	output.PolicyDigest = strings.ToLower(strings.TrimSpace(output.PolicyDigest))
	output.ScriptHash = strings.ToLower(strings.TrimSpace(output.ScriptHash))
	output.ApproverIdentity = strings.TrimSpace(output.ApproverIdentity)
	output.ToolSequence = normalizeStringListLower(output.ToolSequence)
	output.Scope = normalizeStringList(output.Scope)
	output.CreatedAt = output.CreatedAt.UTC()
	output.ExpiresAt = output.ExpiresAt.UTC()
	if output.CreatedAt.IsZero() {
		output.CreatedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	if output.PatternID == "" {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("pattern_id is required")
	}
	if !hexDigestPattern.MatchString(output.PolicyDigest) {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("policy_digest must be sha256 hex")
	}
	if !hexDigestPattern.MatchString(output.ScriptHash) {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("script_hash must be sha256 hex")
	}
	if len(output.ToolSequence) == 0 {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("tool_sequence is required")
	}
	if output.ApproverIdentity == "" {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("approver_identity is required")
	}
	if output.ExpiresAt.IsZero() || !output.ExpiresAt.After(output.CreatedAt) {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("expires_at must be after created_at")
	}
	return output, nil
}

func ApprovedScriptDigest(input schemagate.ApprovedScriptEntry) (string, error) {
	normalized, err := NormalizeApprovedScriptEntry(input)
	if err != nil {
		return "", err
	}
	signable := normalized
	signable.Signature = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return "", fmt.Errorf("marshal approved script signable payload: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest approved script payload: %w", err)
	}
	return digest, nil
}

func SignApprovedScriptEntry(input schemagate.ApprovedScriptEntry, privateKey ed25519.PrivateKey) (schemagate.ApprovedScriptEntry, error) {
	if len(privateKey) == 0 {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("signing private key is required")
	}
	normalized, err := NormalizeApprovedScriptEntry(input)
	if err != nil {
		return schemagate.ApprovedScriptEntry{}, err
	}
	signable := normalized
	signable.Signature = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("marshal approved script entry: %w", err)
	}
	signature, err := sign.SignTraceRecordJSON(privateKey, raw)
	if err != nil {
		return schemagate.ApprovedScriptEntry{}, fmt.Errorf("sign approved script entry: %w", err)
	}
	normalized.Signature = &schemagate.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}
	return normalized, nil
}

func VerifyApprovedScriptEntry(input schemagate.ApprovedScriptEntry, publicKey ed25519.PublicKey, now time.Time) error {
	normalized, err := NormalizeApprovedScriptEntry(input)
	if err != nil {
		return err
	}
	if normalized.Signature == nil {
		return fmt.Errorf("signature is required")
	}
	nowUTC := now.UTC()
	if nowUTC.IsZero() {
		nowUTC = time.Now().UTC()
	}
	if !normalized.ExpiresAt.After(nowUTC) {
		return fmt.Errorf("approved script entry is expired")
	}
	signable := normalized
	signable.Signature = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return fmt.Errorf("marshal signable approved script entry: %w", err)
	}
	if len(publicKey) == 0 {
		return fmt.Errorf("verify key is required")
	}
	ok, err := sign.VerifyTraceRecordJSON(publicKey, sign.Signature{
		Alg:          normalized.Signature.Alg,
		KeyID:        normalized.Signature.KeyID,
		Sig:          normalized.Signature.Sig,
		SignedDigest: normalized.Signature.SignedDigest,
	}, raw)
	if err != nil {
		return fmt.Errorf("verify approved script entry signature: %w", err)
	}
	if !ok {
		return fmt.Errorf("approved script signature did not verify")
	}
	return nil
}

func ReadApprovedScriptRegistry(path string) ([]schemagate.ApprovedScriptEntry, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("approved script registry path is required")
	}
	// #nosec G304 -- explicit local path.
	content, err := os.ReadFile(trimmed)
	if err != nil {
		if os.IsNotExist(err) {
			return []schemagate.ApprovedScriptEntry{}, nil
		}
		return nil, fmt.Errorf("read approved script registry: %w", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return []schemagate.ApprovedScriptEntry{}, nil
	}

	type registryEnvelope struct {
		Entries []schemagate.ApprovedScriptEntry `json:"entries"`
	}
	var envelope registryEnvelope
	if err := json.Unmarshal(content, &envelope); err == nil && len(envelope.Entries) > 0 {
		return normalizeApprovedScriptEntries(envelope.Entries)
	}

	var entries []schemagate.ApprovedScriptEntry
	if err := json.Unmarshal(content, &entries); err != nil {
		return nil, fmt.Errorf("parse approved script registry: %w", err)
	}
	return normalizeApprovedScriptEntries(entries)
}

func WriteApprovedScriptRegistry(path string, entries []schemagate.ApprovedScriptEntry) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("approved script registry path is required")
	}
	normalizedEntries, err := normalizeApprovedScriptEntries(entries)
	if err != nil {
		return err
	}
	sort.Slice(normalizedEntries, func(i, j int) bool {
		if normalizedEntries[i].PatternID != normalizedEntries[j].PatternID {
			return normalizedEntries[i].PatternID < normalizedEntries[j].PatternID
		}
		return normalizedEntries[i].CreatedAt.Before(normalizedEntries[j].CreatedAt)
	})
	encoded, err := json.MarshalIndent(struct {
		Entries []schemagate.ApprovedScriptEntry `json:"entries"`
	}{Entries: normalizedEntries}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approved script registry: %w", err)
	}
	encoded = append(encoded, '\n')
	dir := filepath.Dir(trimmed)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create approved script registry directory: %w", err)
		}
	}
	if err := fsx.WriteFileAtomic(trimmed, encoded, 0o600); err != nil {
		return fmt.Errorf("write approved script registry: %w", err)
	}
	return nil
}

func MatchApprovedScript(intent schemagate.IntentRequest, policyDigest string, entries []schemagate.ApprovedScriptEntry, now time.Time) (ApprovedScriptMatch, error) {
	normalized, err := NormalizeIntent(intent)
	if err != nil {
		return ApprovedScriptMatch{}, err
	}
	if normalized.Script == nil {
		return ApprovedScriptMatch{Matched: false, Reason: "not_script"}, nil
	}
	sequence := make([]string, 0, len(normalized.Script.Steps))
	for _, step := range normalized.Script.Steps {
		sequence = append(sequence, step.ToolName)
	}
	normalizedPolicyDigest := strings.ToLower(strings.TrimSpace(policyDigest))
	nowUTC := now.UTC()
	if nowUTC.IsZero() {
		nowUTC = time.Now().UTC()
	}
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(entry.PolicyDigest)) != normalizedPolicyDigest {
			continue
		}
		if strings.ToLower(strings.TrimSpace(entry.ScriptHash)) != normalized.ScriptHash {
			continue
		}
		if !entry.ExpiresAt.UTC().After(nowUTC) {
			continue
		}
		if !stringSliceEqual(normalizeStringListLower(entry.ToolSequence), sequence) {
			continue
		}
		return ApprovedScriptMatch{
			Matched:   true,
			PatternID: entry.PatternID,
			Reason:    "approved_script_match",
		}, nil
	}
	return ApprovedScriptMatch{
		Matched: false,
		Reason:  "approved_script_not_found",
	}, nil
}

func normalizeApprovedScriptEntries(entries []schemagate.ApprovedScriptEntry) ([]schemagate.ApprovedScriptEntry, error) {
	output := make([]schemagate.ApprovedScriptEntry, 0, len(entries))
	for index, entry := range entries {
		normalized, err := NormalizeApprovedScriptEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("approved script entries[%d]: %w", index, err)
		}
		output = append(output, normalized)
	}
	return output, nil
}

func stringSliceEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
