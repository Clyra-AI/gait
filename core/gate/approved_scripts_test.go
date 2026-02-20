package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	sign "github.com/Clyra-AI/proof/signing"
)

func TestApprovedScriptRegistryRoundTripAndMatch(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	nowUTC := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	entry, err := SignApprovedScriptEntry(schemagate.ApprovedScriptEntry{
		SchemaID:         "gait.gate.approved_script_entry",
		SchemaVersion:    "1.0.0",
		CreatedAt:        nowUTC,
		ProducerVersion:  "test",
		PatternID:        "pattern_demo",
		PolicyDigest:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ScriptHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ToolSequence:     []string{"tool.read", "tool.write"},
		ApproverIdentity: "security-team",
		ExpiresAt:        nowUTC.Add(24 * time.Hour),
	}, keyPair.Private)
	if err != nil {
		t.Fatalf("sign approved script entry: %v", err)
	}
	if err := VerifyApprovedScriptEntry(entry, keyPair.Public, nowUTC); err != nil {
		t.Fatalf("verify approved script entry: %v", err)
	}

	registryPath := filepath.Join(t.TempDir(), "approved_scripts.json")
	if err := WriteApprovedScriptRegistry(registryPath, []schemagate.ApprovedScriptEntry{entry}); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	readEntries, err := ReadApprovedScriptRegistry(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	if len(readEntries) != 1 || readEntries[0].PatternID != "pattern_demo" {
		t.Fatalf("unexpected registry entries: %#v", readEntries)
	}

	intent := baseIntent()
	intent.ToolName = "script"
	intent.Script = &schemagate.IntentScript{
		Steps: []schemagate.IntentScriptStep{
			{ToolName: "tool.read", Args: map[string]any{"path": "/tmp/in.txt"}},
			{ToolName: "tool.write", Args: map[string]any{"path": "/tmp/out.txt"}},
		},
	}
	normalized, err := NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize script intent: %v", err)
	}
	entry.ScriptHash = normalized.ScriptHash
	if err := WriteApprovedScriptRegistry(registryPath, []schemagate.ApprovedScriptEntry{entry}); err != nil {
		t.Fatalf("rewrite registry with matching script hash: %v", err)
	}
	readEntries, err = ReadApprovedScriptRegistry(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	match, err := MatchApprovedScript(normalized, entry.PolicyDigest, readEntries, nowUTC)
	if err != nil {
		t.Fatalf("match approved script: %v", err)
	}
	if !match.Matched || match.PatternID != "pattern_demo" {
		t.Fatalf("expected match for approved script entry, got %#v", match)
	}
}

func TestApprovedScriptDigestIgnoresSignature(t *testing.T) {
	nowUTC := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	entry := schemagate.ApprovedScriptEntry{
		SchemaID:         "gait.gate.approved_script_entry",
		SchemaVersion:    "1.0.0",
		CreatedAt:        nowUTC,
		ProducerVersion:  "test",
		PatternID:        "pattern_digest",
		PolicyDigest:     strings.Repeat("a", 64),
		ScriptHash:       strings.Repeat("b", 64),
		ToolSequence:     []string{"tool.read", "tool.write"},
		ApproverIdentity: "security-team",
		ExpiresAt:        nowUTC.Add(24 * time.Hour),
	}

	firstDigest, err := ApprovedScriptDigest(entry)
	if err != nil {
		t.Fatalf("digest approved script entry: %v", err)
	}
	entry.Signature = &schemagate.Signature{
		Alg:          "ed25519",
		KeyID:        "key-1",
		Sig:          "sig",
		SignedDigest: strings.Repeat("c", 64),
	}
	secondDigest, err := ApprovedScriptDigest(entry)
	if err != nil {
		t.Fatalf("digest approved script entry with signature: %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("expected signature-free digest stability: first=%q second=%q", firstDigest, secondDigest)
	}

	if _, err := ApprovedScriptDigest(schemagate.ApprovedScriptEntry{}); err == nil {
		t.Fatalf("expected digest failure for invalid approved script entry")
	}
}

func TestReadApprovedScriptRegistryVariants(t *testing.T) {
	if _, err := ReadApprovedScriptRegistry(" "); err == nil {
		t.Fatalf("expected error for empty approved script registry path")
	}

	missingPath := filepath.Join(t.TempDir(), "missing.json")
	entries, err := ReadApprovedScriptRegistry(missingPath)
	if err != nil {
		t.Fatalf("read missing approved script registry: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries for missing registry, got %#v", entries)
	}

	emptyPath := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty registry fixture: %v", err)
	}
	entries, err = ReadApprovedScriptRegistry(emptyPath)
	if err != nil {
		t.Fatalf("read empty approved script registry: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries for blank registry file, got %#v", entries)
	}

	emptyEnvelopePath := filepath.Join(t.TempDir(), "empty_envelope.json")
	if err := os.WriteFile(emptyEnvelopePath, []byte(`{"entries":[]}`), 0o600); err != nil {
		t.Fatalf("write empty envelope registry fixture: %v", err)
	}
	entries, err = ReadApprovedScriptRegistry(emptyEnvelopePath)
	if err != nil {
		t.Fatalf("read empty envelope approved script registry: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries for empty envelope registry, got %#v", entries)
	}

	nowUTC := time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC)
	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	legacyJSON := []byte(`[
		{
			"schema_id":"gait.gate.approved_script_entry",
			"schema_version":"1.0.0",
			"created_at":"2026-02-05T00:00:00Z",
			"producer_version":"test",
			"pattern_id":"pattern_legacy",
			"policy_digest":"` + strings.Repeat("a", 64) + `",
			"script_hash":"` + strings.Repeat("b", 64) + `",
			"tool_sequence":["tool.read"],
			"approver_identity":"secops",
			"expires_at":"2026-02-06T00:00:00Z"
		}
	]
`)
	if err := os.WriteFile(legacyPath, legacyJSON, 0o600); err != nil {
		t.Fatalf("write legacy registry fixture: %v", err)
	}
	entries, err = ReadApprovedScriptRegistry(legacyPath)
	if err != nil {
		t.Fatalf("read legacy approved script registry: %v", err)
	}
	if len(entries) != 1 || entries[0].PatternID != "pattern_legacy" || !entries[0].ExpiresAt.After(nowUTC) {
		t.Fatalf("unexpected legacy registry entries: %#v", entries)
	}
}
