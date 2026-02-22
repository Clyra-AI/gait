package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGatewayIngestCommand(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	logPath := filepath.Join(workDir, "mintmcp.log.jsonl")
	mustWriteFile(t, logPath, strings.Join([]string{
		`{"timestamp":"2026-02-20T12:00:00Z","tool_name":"tool.read","verdict":"allow","request_id":"req-a"}`,
		`{"timestamp":"2026-02-20T12:00:01Z","tool_name":"tool.write","verdict":"block","request_id":"req-b","reason_code":"policy_blocked"}`,
	}, "\n"))

	var code int
	raw := captureStdout(t, func() {
		code = runGateway([]string{
			"ingest",
			"--source", "mintmcp",
			"--log-path", logPath,
			"--json",
		})
	})
	if code != exitOK {
		t.Fatalf("runGateway ingest expected %d got %d", exitOK, code)
	}
	var output gatewayOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode gateway output: %v raw=%q", err, raw)
	}
	if !output.OK || output.OutputRecords != 2 {
		t.Fatalf("unexpected gateway ingest output: %#v", output)
	}
	if strings.TrimSpace(output.ProofRecordsOut) == "" {
		t.Fatalf("expected proof_records_out in output")
	}
	// #nosec G304 -- test validates explicit output path from command result.
	proofRaw, err := os.ReadFile(output.ProofRecordsOut)
	if err != nil {
		t.Fatalf("read proof record output: %v", err)
	}
	if !strings.Contains(string(proofRaw), `"record_type":"policy_enforcement"`) {
		t.Fatalf("expected policy_enforcement records in output file: %s", string(proofRaw))
	}
}

func TestRunGatewayValidationAndHelpPaths(t *testing.T) {
	if code := runGateway([]string{}); code != exitInvalidInput {
		t.Fatalf("runGateway missing args expected %d got %d", exitInvalidInput, code)
	}
	if code := runGateway([]string{"unknown"}); code != exitInvalidInput {
		t.Fatalf("runGateway unknown subcommand expected %d got %d", exitInvalidInput, code)
	}
	if code := runGateway([]string{"ingest", "--help"}); code != exitOK {
		t.Fatalf("runGateway help expected %d got %d", exitOK, code)
	}
	if code := runGateway([]string{"ingest", "--source", "kong", "--json"}); code != exitInvalidInput {
		t.Fatalf("runGateway missing log-path expected %d got %d", exitInvalidInput, code)
	}
}
