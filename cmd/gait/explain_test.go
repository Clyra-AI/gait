package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	validate "github.com/Clyra-AI/proof/schema"
)

func TestRunGateEvalExplainJSON(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	repoRoot := repoRootFromPackageDir(t)
	schemaPath := filepath.Join(repoRoot, "schemas", "v1", "gate", "policy_explain.schema.json")

	tests := []struct {
		name         string
		policyLines  []string
		intent       schemagate.IntentRequest
		wantExitCode int
		wantVerdict  string
		wantRule     string
	}{
		{
			name: "allow",
			policyLines: []string{
				"default_verdict: block",
				"rules:",
				"  - name: allow-read",
				"    effect: allow",
				"    match:",
				"      tool_names: [tool.read]",
			},
			intent: schemagate.IntentRequest{
				SchemaID:        "gait.gate.intent_request",
				SchemaVersion:   "1.0.0",
				CreatedAt:       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
				ProducerVersion: "test",
				ToolName:        "tool.read",
				Args:            map[string]any{"path": "/tmp/in.txt"},
				Targets:         []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/in.txt", Operation: "read"}},
				Context:         schemagate.IntentContext{Identity: "alice", Workspace: "/tmp", RiskClass: "low"},
			},
			wantExitCode: exitOK,
			wantVerdict:  "allow",
			wantRule:     "allow-read",
		},
		{
			name: "block",
			policyLines: []string{
				"default_verdict: allow",
				"rules:",
				"  - name: block-write",
				"    effect: block",
				"    match:",
				"      tool_names: [tool.write]",
			},
			intent: schemagate.IntentRequest{
				SchemaID:        "gait.gate.intent_request",
				SchemaVersion:   "1.0.0",
				CreatedAt:       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
				ProducerVersion: "test",
				ToolName:        "tool.write",
				Args:            map[string]any{"path": "/tmp/out.txt"},
				Targets:         []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out.txt", Operation: "write"}},
				Context:         schemagate.IntentContext{Identity: "alice", Workspace: "/tmp", RiskClass: "medium"},
			},
			wantExitCode: exitPolicyBlocked,
			wantVerdict:  "block",
			wantRule:     "block-write",
		},
		{
			name: "require_approval",
			policyLines: []string{
				"default_verdict: allow",
				"rules:",
				"  - name: require-approval-write",
				"    effect: require_approval",
				"    min_approvals: 2",
				"    match:",
				"      tool_names: [tool.write]",
			},
			intent: schemagate.IntentRequest{
				SchemaID:        "gait.gate.intent_request",
				SchemaVersion:   "1.0.0",
				CreatedAt:       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
				ProducerVersion: "test",
				ToolName:        "tool.write",
				Args:            map[string]any{"path": "/tmp/out.txt"},
				Targets:         []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out.txt", Operation: "write"}},
				Context:         schemagate.IntentContext{Identity: "alice", Workspace: "/tmp", RiskClass: "medium"},
			},
			wantExitCode: exitApprovalRequired,
			wantVerdict:  "require_approval",
			wantRule:     "require-approval-write",
		},
		{
			name: "dry_run",
			policyLines: []string{
				"default_verdict: allow",
				"rules:",
				"  - name: allow-delete",
				"    effect: allow",
				"    match:",
				"      tool_names: [tool.delete]",
			},
			intent: schemagate.IntentRequest{
				SchemaID:        "gait.gate.intent_request",
				SchemaVersion:   "1.0.0",
				CreatedAt:       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
				ProducerVersion: "test",
				ToolName:        "tool.delete",
				Args:            map[string]any{"path": "/tmp/out.txt"},
				Targets:         []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out.txt", Operation: "delete"}},
				Context:         schemagate.IntentContext{Identity: "alice", Workspace: "/tmp", RiskClass: "high", Phase: "plan"},
			},
			wantExitCode: exitOK,
			wantVerdict:  "dry_run",
			wantRule:     "allow-delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policyPath := filepath.Join(workDir, test.name+"_policy.yaml")
			mustWriteFile(t, policyPath, strings.Join(test.policyLines, "\n")+"\n")
			intentPath := filepath.Join(workDir, test.name+"_intent.json")
			rawIntent, err := json.MarshalIndent(test.intent, "", "  ")
			if err != nil {
				t.Fatalf("marshal intent: %v", err)
			}
			mustWriteFile(t, intentPath, string(rawIntent)+"\n")

			var code int
			raw := captureStdout(t, func() {
				code = runGateEval([]string{
					"--policy", policyPath,
					"--intent", intentPath,
					"--json",
					"--explain",
				})
			})
			if code != test.wantExitCode {
				t.Fatalf("runGateEval explain expected %d got %d (%s)", test.wantExitCode, code, raw)
			}
			if err := validate.ValidateJSON(schemaPath, []byte(raw)); err != nil {
				t.Fatalf("validate explain schema: %v\n%s", err, raw)
			}
			var explain schemagate.PolicyExplain
			if err := json.Unmarshal([]byte(raw), &explain); err != nil {
				t.Fatalf("decode explain output: %v", err)
			}
			if explain.Verdict != test.wantVerdict || explain.MatchedRule != test.wantRule {
				t.Fatalf("unexpected explain output: %#v", explain)
			}
			if len(explain.MatchedRules) != 1 || explain.MatchedRules[0].Name != test.wantRule {
				t.Fatalf("unexpected matched rules in explain output: %#v", explain.MatchedRules)
			}
		})
	}
}
