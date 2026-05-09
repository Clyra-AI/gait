package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestRunKillSwitchAddListDisableExpire(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	statePath := filepath.Join(workDir, "kill_switch_state.json")

	addRaw := captureStdout(t, func() {
		if code := runKillSwitch([]string{
			"add",
			"--state", statePath,
			"--agent-id", "agent.exec",
			"--tool-name", "tool.exec",
			"--reason", "break glass",
			"--actor", "secops",
			"--json",
		}); code != exitOK {
			t.Fatalf("runKillSwitch add expected %d got %d", exitOK, code)
		}
	})
	var addOut killSwitchOutput
	if err := json.Unmarshal([]byte(addRaw), &addOut); err != nil {
		t.Fatalf("decode add output: %v", err)
	}
	if addOut.Entry == nil || addOut.Entry.EntryID == "" {
		t.Fatalf("expected created entry in add output: %#v", addOut)
	}
	entryID := addOut.Entry.EntryID

	listRaw := captureStdout(t, func() {
		if code := runKillSwitch([]string{"list", "--state", statePath, "--json"}); code != exitOK {
			t.Fatalf("runKillSwitch list expected %d got %d", exitOK, code)
		}
	})
	var listOut killSwitchOutput
	if err := json.Unmarshal([]byte(listRaw), &listOut); err != nil {
		t.Fatalf("decode list output: %v", err)
	}
	if len(listOut.Entries) != 1 || listOut.Entries[0].EntryID != entryID {
		t.Fatalf("unexpected list output: %#v", listOut)
	}

	if code := runKillSwitch([]string{"disable", "--state", statePath, "--entry-id", entryID, "--json"}); code != exitOK {
		t.Fatalf("runKillSwitch disable expected %d got %d", exitOK, code)
	}
	state, err := gate.LoadKillSwitchState(statePath)
	if err != nil {
		t.Fatalf("load state after disable: %v", err)
	}
	if len(state.Entries) != 1 || state.Entries[0].Enabled {
		t.Fatalf("expected disabled entry, got %#v", state)
	}

	if code := runKillSwitch([]string{"expire", "--state", statePath, "--entry-id", entryID, "--expires-at", "2026-05-09T15:00:00Z", "--json"}); code != exitOK {
		t.Fatalf("runKillSwitch expire expected %d got %d", exitOK, code)
	}
	state, err = gate.LoadKillSwitchState(statePath)
	if err != nil {
		t.Fatalf("load state after expire: %v", err)
	}
	if state.Entries[0].ExpiresAt.UTC() != time.Date(2026, time.May, 9, 15, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected expiry after expire command: %#v", state.Entries[0])
	}
}

func TestRunGateEvalKillSwitchBlocksAndJournals(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: allow-exec",
		"    effect: allow",
		"    match:",
		"      tool_names: [tool.exec]",
	}, "\n")+"\n")
	intent := schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "test",
		ToolName:        "tool.exec",
		Args:            map[string]any{"path": "/tmp/work/run.sh"},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/work/run.sh", Operation: "execute", EndpointClass: "proc.exec"},
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			AgentID:   "agent.exec",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}
	intentPath := filepath.Join(workDir, "intent.json")
	rawIntent, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatalf("marshal intent: %v", err)
	}
	mustWriteFile(t, intentPath, string(rawIntent)+"\n")
	statePath := filepath.Join(workDir, "kill_switch_state.json")
	state := gate.NewKillSwitchState(time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC), "test")
	state.Entries = []schemagate.KillSwitchEntry{
		{
			EntryID:   "agent-stop",
			Enabled:   true,
			AgentID:   "agent.exec",
			Reason:    "incident",
			Actor:     "secops",
			CreatedAt: time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	if err := gate.WriteKillSwitchState(statePath, state); err != nil {
		t.Fatalf("write state: %v", err)
	}

	raw := captureStdout(t, func() {
		if code := runGateEval([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--kill-switch-state", statePath,
			"--json",
		}); code != exitPolicyBlocked {
			t.Fatalf("runGateEval kill switch expected %d got %d", exitPolicyBlocked, code)
		}
	})
	var output gateEvalOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode gate output: %v", err)
	}
	if output.KillSwitch == nil || output.KillSwitch.Status != "active" {
		t.Fatalf("unexpected kill switch output: %#v", output)
	}
	if !containsString(output.ReasonCodes, "kill_switch_agent_id_active") {
		t.Fatalf("expected kill_switch_agent_id_active in %#v", output.ReasonCodes)
	}
	journalPayload, err := os.ReadFile(gate.KillSwitchJournalPath(statePath))
	if err != nil {
		t.Fatalf("read kill switch journal: %v", err)
	}
	if !strings.Contains(string(journalPayload), "agent-stop") {
		t.Fatalf("expected journal to include matched entry id: %s", string(journalPayload))
	}
}

func TestRunGateEvalKillSwitchUnavailableFailClosedInOSSProd(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)
	privateKeyPath := filepath.Join(workDir, "trace_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, strings.Join([]string{
		"default_verdict: allow",
		"rules:",
		"  - name: allow-exec",
		"    effect: allow",
		"    require_broker_credential: true",
		"    broker_reference: exec",
		"    broker_scopes: [execute]",
		"    match:",
		"      tool_names: [tool.exec]",
	}, "\n")+"\n")
	intentPath := filepath.Join(workDir, "intent.json")
	writeIntentFixture(t, intentPath, "tool.exec")

	raw := captureStdout(t, func() {
		if code := runGateEval([]string{
			"--policy", policyPath,
			"--intent", intentPath,
			"--profile", "oss-prod",
			"--key-mode", "prod",
			"--private-key", privateKeyPath,
			"--kill-switch-state", filepath.Join(workDir, "missing.json"),
			"--credential-broker", "stub",
			"--json",
		}); code != exitPolicyBlocked {
			t.Fatalf("runGateEval kill-switch unavailable expected %d got %d", exitPolicyBlocked, code)
		}
	})
	var output gateEvalOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode gate output: %v", err)
	}
	if output.KillSwitch == nil || output.KillSwitch.Status != "unavailable" {
		t.Fatalf("expected unavailable kill switch output, got %#v", output)
	}
	if !containsString(output.ReasonCodes, "kill_switch_state_unavailable") {
		t.Fatalf("expected kill_switch_state_unavailable in %#v", output.ReasonCodes)
	}
}

func TestRunKillSwitchUsageAndTextOutput(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	if code := runKillSwitch(nil); code != exitInvalidInput {
		t.Fatalf("runKillSwitch without args expected %d got %d", exitInvalidInput, code)
	}
	if code := runKillSwitch([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runKillSwitch unknown subcommand expected %d got %d", exitInvalidInput, code)
	}

	text := captureStdout(t, func() {
		if code := writeKillSwitchOutput(false, killSwitchOutput{
			OK:     true,
			Action: "list",
			State: &schemagate.KillSwitchState{
				Entries: []schemagate.KillSwitchEntry{{EntryID: "one"}},
			},
		}, exitOK); code != exitOK {
			t.Fatalf("writeKillSwitchOutput text expected %d got %d", exitOK, code)
		}
	})
	if !strings.Contains(text, "list: 1 entries") {
		t.Fatalf("unexpected text kill-switch output: %q", text)
	}
}
