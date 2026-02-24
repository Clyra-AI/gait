package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLIStopLatencyAndEmergencyStopPreemption(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	jobsRoot := filepath.Join(workDir, "jobs")
	jobID := "job_stop_e2e"
	runJSONCommand(t, workDir, binPath, "job", "submit", "--id", jobID, "--root", jobsRoot, "--json")

	stopStartedAt := time.Now().UTC()
	stopOut := runJSONCommand(t, workDir, binPath, "job", "stop", "--id", jobID, "--root", jobsRoot, "--actor", "secops", "--json")
	var stopResult struct {
		OK  bool `json:"ok"`
		Job struct {
			Status     string    `json:"status"`
			StopReason string    `json:"stop_reason"`
			UpdatedAt  time.Time `json:"updated_at"`
		} `json:"job"`
	}
	if err := json.Unmarshal(stopOut, &stopResult); err != nil {
		t.Fatalf("parse job stop output: %v\n%s", err, string(stopOut))
	}
	if !stopResult.OK || stopResult.Job.Status != "emergency_stopped" || stopResult.Job.StopReason != "emergency_stopped" {
		t.Fatalf("unexpected stop output: %s", string(stopOut))
	}

	stopAckMS := stopResult.Job.UpdatedAt.Sub(stopStartedAt).Milliseconds()
	if stopAckMS < 0 {
		stopAckMS = 0
	}
	if stopAckMS > 15000 {
		t.Fatalf("stop_ack_ms exceeded e2e threshold: got=%d want<=15000", stopAckMS)
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteE2EFile(t, policyPath, "default_verdict: allow")
	callPath := filepath.Join(workDir, "call.json")
	mustWriteE2EFile(t, callPath, `{
  "name":"tool.delete",
  "args":{"path":"/tmp/out.txt"},
  "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"delete","destructive":true}],
  "arg_provenance":[{"arg_path":"$.path","source":"user"}],
  "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","session_id":"sess-stop","job_id":"job_stop_e2e","phase":"apply"}
}`)

	mcpOut := runJSONCommandExpectCode(t, workDir, binPath, 3,
		"mcp", "proxy",
		"--policy", policyPath,
		"--call", callPath,
		"--job-root", jobsRoot,
		"--json",
	)
	var mcpResult struct {
		OK          bool     `json:"ok"`
		Executed    bool     `json:"executed"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
	}
	if err := json.Unmarshal(mcpOut, &mcpResult); err != nil {
		t.Fatalf("parse mcp proxy output: %v\n%s", err, string(mcpOut))
	}
	if !mcpResult.OK || mcpResult.Verdict != "block" || mcpResult.Executed {
		t.Fatalf("unexpected mcp stop-preemption output: %s", string(mcpOut))
	}
	if !containsReasonCode(mcpResult.ReasonCodes, "emergency_stop_preempted") {
		t.Fatalf("expected emergency_stop_preempted reason code, got %#v", mcpResult.ReasonCodes)
	}

	inspectOut := runJSONCommand(t, workDir, binPath, "job", "inspect", "--id", jobID, "--root", jobsRoot, "--json")
	var inspectResult struct {
		OK     bool `json:"ok"`
		Events []struct {
			Type      string    `json:"type"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"events"`
	}
	if err := json.Unmarshal(inspectOut, &inspectResult); err != nil {
		t.Fatalf("parse job inspect output: %v\n%s", err, string(inspectOut))
	}
	if !inspectResult.OK {
		t.Fatalf("job inspect did not return ok=true: %s", string(inspectOut))
	}

	ackIndex := -1
	for index, event := range inspectResult.Events {
		if event.Type == "emergency_stop_acknowledged" {
			ackIndex = index
			break
		}
	}
	if ackIndex < 0 {
		t.Fatalf("expected emergency_stop_acknowledged event in inspect output: %s", string(inspectOut))
	}

	postStopSideEffects := 0
	blockedDispatches := 0
	for _, event := range inspectResult.Events[ackIndex+1:] {
		switch event.Type {
		case "dispatch_blocked":
			blockedDispatches++
		default:
			postStopSideEffects++
		}
	}
	if blockedDispatches == 0 {
		t.Fatalf("expected at least one dispatch_blocked event after stop")
	}
	if postStopSideEffects != 0 {
		t.Fatalf("expected post_stop_side_effects=0, got=%d", postStopSideEffects)
	}
}

func containsReasonCode(reasonCodes []string, expected string) bool {
	for _, reasonCode := range reasonCodes {
		if strings.EqualFold(strings.TrimSpace(reasonCode), expected) {
			return true
		}
	}
	return false
}
