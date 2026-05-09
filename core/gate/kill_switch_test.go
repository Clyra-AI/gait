package gate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestMatchKillSwitchSelectors(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	state := NewKillSwitchState(now, "test")
	state.Entries = []schemagate.KillSwitchEntry{
		{EntryID: "agent", Enabled: true, AgentID: "agent.exec", CreatedAt: now},
		{EntryID: "identity", Enabled: true, Identity: "alice", CreatedAt: now},
		{EntryID: "tool", Enabled: true, ToolName: "tool.exec", CreatedAt: now},
		{EntryID: "target", Enabled: true, TargetKind: "host", TargetValue: "api.internal", CreatedAt: now},
		{EntryID: "path", Enabled: true, PathPrefixes: []string{"/tmp/work"}, CreatedAt: now},
		{EntryID: "workspace", Enabled: true, WorkspacePrefixes: []string{"/repo"}, CreatedAt: now},
		{EntryID: "env", Enabled: true, Environment: "prod", CreatedAt: now},
	}
	intent := baseIntent()
	intent.ToolName = "tool.exec"
	intent.Context.AgentID = "agent.exec"
	intent.Context.Identity = "alice"
	intent.Context.Environment = "prod"
	intent.Context.Workspace = "/repo/service"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "host", Value: "api.internal", EndpointClass: "net.http"},
		{Kind: "path", Value: "/tmp/work/script.sh", EndpointClass: "proc.exec"},
	}

	decision := MatchKillSwitch(state, intent, now)
	if decision == nil || decision.Status != "active" {
		t.Fatalf("expected active kill switch decision, got %#v", decision)
	}
	for _, reason := range []string{
		"kill_switch_active",
		"kill_switch_agent_id_active",
		"kill_switch_identity_active",
		"kill_switch_tool_name_active",
		"kill_switch_target_active",
		"kill_switch_path_active",
		"kill_switch_workspace_active",
		"kill_switch_environment_active",
	} {
		if !contains(decision.ReasonCodes, reason) {
			t.Fatalf("expected reason %q in %#v", reason, decision.ReasonCodes)
		}
	}
}

func TestMatchKillSwitchIgnoresDisabledAndExpiredEntries(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	state := NewKillSwitchState(now, "test")
	state.Entries = []schemagate.KillSwitchEntry{
		{EntryID: "disabled", Enabled: false, ToolName: "tool.exec", CreatedAt: now},
		{EntryID: "expired", Enabled: true, ToolName: "tool.exec", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-1 * time.Hour)},
	}
	intent := baseIntent()
	intent.ToolName = "tool.exec"
	decision := MatchKillSwitch(state, intent, now)
	if decision == nil || decision.Status != "inactive" {
		t.Fatalf("expected inactive decision, got %#v", decision)
	}
}

func TestLoadAndWriteKillSwitchState(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	state := NewKillSwitchState(now, "test")
	entry, err := NewKillSwitchEntry(now, schemagate.KillSwitchEntry{
		AgentID:   "agent.exec",
		ToolName:  "tool.exec",
		Reason:    "break glass",
		Actor:     "secops",
		ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("new kill switch entry: %v", err)
	}
	state.Entries = []schemagate.KillSwitchEntry{entry}
	statePath := filepath.Join(t.TempDir(), "kill_switch_state.json")
	if err := WriteKillSwitchState(statePath, state); err != nil {
		t.Fatalf("write kill switch state: %v", err)
	}
	loaded, err := LoadKillSwitchState(statePath)
	if err != nil {
		t.Fatalf("load kill switch state: %v", err)
	}
	if loaded.SchemaID != killSwitchStateSchemaID || len(loaded.Entries) != 1 {
		t.Fatalf("unexpected loaded kill switch state: %#v", loaded)
	}
}

func TestEvaluatePolicyDetailedKillSwitchStateUnavailableFailClosed(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: allow-exec
    effect: allow
    match:
      tool_names: [tool.exec]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	intent := baseIntent()
	intent.ToolName = "tool.exec"
	intent.Context.RiskClass = "high"
	outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{
		ProducerVersion:        "test",
		RequireKillSwitchState: true,
		KillSwitchStateError:   errors.New("missing"),
	})
	if evalErr != nil {
		t.Fatalf("evaluate with unavailable kill switch state: %v", evalErr)
	}
	if outcome.Result.Verdict != "block" || !contains(outcome.Result.ReasonCodes, "kill_switch_state_unavailable") {
		t.Fatalf("unexpected unavailable-state outcome: %#v", outcome)
	}
	if outcome.KillSwitch == nil || outcome.KillSwitch.Status != "unavailable" {
		t.Fatalf("expected unavailable kill switch decision, got %#v", outcome.KillSwitch)
	}
}

func TestAppendKillSwitchJournal(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "kill_switch_journal.jsonl")
	if err := AppendKillSwitchJournal(journalPath, KillSwitchJournalRecord{
		CreatedAt:       time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		Source:          "gate_eval",
		TraceID:         "trace-1",
		ReasonCodes:     []string{"kill_switch_active"},
		MatchedEntryIDs: []string{"entry-1"},
	}); err != nil {
		t.Fatalf("append kill switch journal: %v", err)
	}
	payload, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if !strings.Contains(string(payload), "trace-1") {
		t.Fatalf("expected trace id in journal payload: %s", string(payload))
	}
}

func TestKillSwitchHelpers(t *testing.T) {
	if path := KillSwitchJournalPath("/tmp/state.json"); path != filepath.Join("/tmp", "kill_switch_journal.jsonl") {
		t.Fatalf("unexpected journal path: %s", path)
	}
	prefixes := normalizeKillSwitchPrefixes([]string{" /tmp/work ", "/tmp/work", "/var/tmp/cache "})
	if len(prefixes) != 2 || prefixes[0] != "/tmp/work" || prefixes[1] != "/var/tmp/cache" {
		t.Fatalf("unexpected normalized prefixes: %#v", prefixes)
	}

	scriptIntent := baseIntent()
	scriptIntent.Script = &schemagate.IntentScript{
		Steps: []schemagate.IntentScriptStep{
			{ToolName: "tool.exec", Targets: []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/work/run.sh", EndpointClass: "proc.exec"}}},
		},
	}
	if len(allIntentTargets(scriptIntent)) != 1 {
		t.Fatalf("expected script targets to flatten, got %#v", allIntentTargets(scriptIntent))
	}

	outcome := EvalOutcome{
		Result:         schemagate.GateResult{Verdict: "allow", ReasonCodes: []string{"matched_rule_allow"}},
		PreparedIntent: baseIntent(),
	}
	state := NewKillSwitchState(time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC), "test")
	state.Entries = []schemagate.KillSwitchEntry{{EntryID: "identity-stop", Enabled: true, Identity: "alice", CreatedAt: time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)}}
	applied := applyKillSwitchOutcome(outcome, EvalOptions{
		KillSwitchState: &state,
		EvaluationTime:  time.Date(2026, time.May, 9, 12, 5, 0, 0, time.UTC),
	})
	if applied.Result.Verdict != "block" || applied.KillSwitch == nil || applied.KillSwitch.Status != "active" {
		t.Fatalf("unexpected applied kill switch outcome: %#v", applied)
	}
}
