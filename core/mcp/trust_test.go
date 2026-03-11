package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/registry"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluateServerTrustTrusted(t *testing.T) {
	workDir := t.TempDir()
	snapshotPath := filepath.Join(workDir, "trust_snapshot.json")
	writeTrustSnapshot(t, snapshotPath, TrustSnapshot{
		SchemaID:        mcpTrustSnapshotSchemaID,
		SchemaVersion:   mcpTrustSnapshotSchemaVersion,
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Entries: []TrustSnapshotEntry{{
			ServerID:   "github",
			ServerName: "GitHub",
			Publisher:  "acme",
			Source:     "registry",
			Status:     "trusted",
			UpdatedAt:  time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
			Score:      0.95,
			RegistryVerification: &registry.VerifyResult{
				SignatureVerified: true,
				PublisherAllowed:  true,
			},
		}},
	})

	decision := EvaluateServerTrust(gate.MCPTrustPolicy{
		Enabled:             true,
		SnapshotPath:        snapshotPath,
		Action:              "block",
		RequiredRiskClasses: []string{"high"},
		MinScore:            0.8,
		PublisherAllowlist:  []string{"acme"},
		RequireRegistry:     true,
	}, &ServerInfo{ServerID: "github"}, "high", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if decision == nil || decision.Status != "trusted" || decision.Enforced {
		t.Fatalf("expected trusted non-enforced decision, got %#v", decision)
	}
}

func TestEvaluateServerTrustFailClosedStates(t *testing.T) {
	workDir := t.TempDir()
	snapshotPath := filepath.Join(workDir, "trust_snapshot.json")
	writeTrustSnapshot(t, snapshotPath, TrustSnapshot{
		SchemaID:        mcpTrustSnapshotSchemaID,
		SchemaVersion:   mcpTrustSnapshotSchemaVersion,
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Entries: []TrustSnapshotEntry{
			{
				ServerID:  "stale-server",
				Status:    "trusted",
				UpdatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
				Score:     0.9,
			},
			{
				ServerID:  "low-score",
				Status:    "trusted",
				UpdatedAt: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
				Score:     0.2,
			},
		},
	})

	policy := gate.MCPTrustPolicy{
		Enabled:             true,
		SnapshotPath:        snapshotPath,
		Action:              "block",
		RequiredRiskClasses: []string{"high"},
		MinScore:            0.8,
		MaxAge:              "24h",
	}
	stale := EvaluateServerTrust(policy, &ServerInfo{ServerID: "stale-server"}, "high", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if stale == nil || stale.Status != "stale" || !stale.Enforced {
		t.Fatalf("expected stale enforced decision, got %#v", stale)
	}
	lowScore := EvaluateServerTrust(policy, &ServerInfo{ServerID: "low-score"}, "high", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if lowScore == nil || lowScore.Status != "below_threshold" || !lowScore.Enforced {
		t.Fatalf("expected below-threshold enforced decision, got %#v", lowScore)
	}
}

func TestApplyTrustPolicyOverridesAllowVerdict(t *testing.T) {
	workDir := t.TempDir()
	snapshotPath := filepath.Join(workDir, "trust_snapshot.json")
	writeTrustSnapshot(t, snapshotPath, TrustSnapshot{
		SchemaID:        mcpTrustSnapshotSchemaID,
		SchemaVersion:   mcpTrustSnapshotSchemaVersion,
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Entries:         []TrustSnapshotEntry{},
	})

	outcome := ApplyTrustPolicy(gate.MCPTrustPolicy{
		Enabled:             true,
		SnapshotPath:        snapshotPath,
		Action:              "require_approval",
		RequiredRiskClasses: []string{"high"},
	}, ToolCall{Server: &ServerInfo{ServerID: "missing"}, Context: CallContext{RiskClass: "high"}}, gate.EvalOutcome{
		Result: gateResultAllow(),
	}, time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if outcome.Result.Verdict != "require_approval" {
		t.Fatalf("expected trust policy to require approval, got %#v", outcome.Result)
	}
	if outcome.MCPTrust == nil || outcome.MCPTrust.Status != "unknown" {
		t.Fatalf("expected trust decision on outcome, got %#v", outcome.MCPTrust)
	}
}

func TestEvaluateServerTrustMissingSnapshotAndPublisherMismatch(t *testing.T) {
	missing := EvaluateServerTrust(gate.MCPTrustPolicy{
		Enabled:             true,
		SnapshotPath:        filepath.Join(t.TempDir(), "missing.json"),
		Action:              "block",
		RequiredRiskClasses: []string{"high"},
	}, &ServerInfo{ServerID: "github"}, "high", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if missing == nil || missing.Status != "missing" || !missing.Enforced {
		t.Fatalf("expected missing snapshot to fail closed, got %#v", missing)
	}

	workDir := t.TempDir()
	snapshotPath := filepath.Join(workDir, "trust_snapshot.json")
	writeTrustSnapshot(t, snapshotPath, TrustSnapshot{
		SchemaID:        mcpTrustSnapshotSchemaID,
		SchemaVersion:   mcpTrustSnapshotSchemaVersion,
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Entries: []TrustSnapshotEntry{{
			ServerID:  "github",
			Publisher: "other",
			Status:    "trusted",
			UpdatedAt: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
			Score:     0.9,
		}},
	})
	mismatch := EvaluateServerTrust(gate.MCPTrustPolicy{
		Enabled:             true,
		SnapshotPath:        snapshotPath,
		Action:              "block",
		RequiredRiskClasses: []string{"high"},
		PublisherAllowlist:  []string{"acme"},
	}, &ServerInfo{ServerID: "github"}, "high", time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC))
	if mismatch == nil || mismatch.Status != "untrusted" {
		t.Fatalf("expected publisher mismatch to be untrusted, got %#v", mismatch)
	}
}

func TestLoadTrustSnapshotValidationAndHelpers(t *testing.T) {
	workDir := t.TempDir()
	invalidPath := filepath.Join(workDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"schema_id":"wrong","entries":[]}`), 0o600); err != nil {
		t.Fatalf("write invalid snapshot: %v", err)
	}
	if _, err := LoadTrustSnapshot(invalidPath); err == nil {
		t.Fatalf("expected invalid schema id to fail")
	}

	snapshotPath := filepath.Join(workDir, "named.json")
	writeTrustSnapshot(t, snapshotPath, TrustSnapshot{
		SchemaID:        mcpTrustSnapshotSchemaID,
		SchemaVersion:   mcpTrustSnapshotSchemaVersion,
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Entries: []TrustSnapshotEntry{{
			ServerName: "Named Server",
			Status:     "trusted",
			UpdatedAt:  time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		}},
	})
	snapshot, err := LoadTrustSnapshot(snapshotPath)
	if err != nil {
		t.Fatalf("load valid snapshot: %v", err)
	}
	if _, ok := findTrustSnapshotEntry(snapshot.Entries, "named server"); !ok {
		t.Fatalf("expected lookup by normalized server name to succeed")
	}
	if got := classifyTrustStatus([]string{"mcp_trust_registry_unverified"}); got != "invalid" {
		t.Fatalf("unexpected classifyTrustStatus result: %s", got)
	}
	if got := moreRestrictiveVerdict("require_approval", "block"); got != "block" {
		t.Fatalf("expected block to be more restrictive, got %s", got)
	}
	if got := mcpTrustMaxAgeSeconds("2h"); got != 7200 {
		t.Fatalf("unexpected max age seconds: %d", got)
	}
}

func gateResultAllow() schemagate.GateResult {
	return schemagate.GateResult{
		SchemaID:        "gait.gate.result",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Verdict:         "allow",
		ReasonCodes:     []string{"matched_rule"},
		Violations:      []string{},
	}
}

func writeTrustSnapshot(t *testing.T, path string, snapshot TrustSnapshot) {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal trust snapshot: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write trust snapshot: %v", err)
	}
}
