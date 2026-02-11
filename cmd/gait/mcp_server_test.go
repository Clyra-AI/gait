package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPServeHandlerHealthz(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("healthz status: expected %d got %d", http.StatusOK, recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode healthz response: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("expected ok=true in healthz response")
	}
}

func TestMCPServeHandlerEvaluateOpenAI(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	traceDir := filepath.Join(workDir, "traces")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		TraceDir:       traceDir,
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := map[string]any{
		"adapter": "openai",
		"run_id":  "run_mcp_server_case",
		"call": map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":      "tool.search",
				"arguments": "{\"query\":\"gait\"}",
			},
		},
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(encoded))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode evaluate response: %v", err)
	}
	if !response.OK {
		t.Fatalf("expected ok=true response, got %#v", response)
	}
	if response.Verdict != "allow" {
		t.Fatalf("expected allow verdict, got %q", response.Verdict)
	}
	if response.ExitCode != exitOK {
		t.Fatalf("expected exit code %d got %d", exitOK, response.ExitCode)
	}
	if response.TracePath == "" {
		t.Fatalf("expected trace path")
	}
	if _, err := os.Stat(response.TracePath); err != nil {
		t.Fatalf("expected trace output file: %v", err)
	}
}

func TestMCPServeHandlerEvaluateValidation(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader([]byte(`{"adapter":"openai"}`)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestMCPServeHandlerEvaluateBlockVerdict(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: block\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "openai",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call": {
	    "type": "function",
	    "function": {
	      "name": "tool.delete",
	      "arguments": "{\"path\":\"/tmp/out.txt\"}"
	    }
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode evaluate response: %v", err)
	}
	if response.Verdict != "block" {
		t.Fatalf("expected block verdict, got %q", response.Verdict)
	}
	if response.ExitCode != exitPolicyBlocked {
		t.Fatalf("expected exit code %d got %d", exitPolicyBlocked, response.ExitCode)
	}
}

func TestMCPServeHandlerEvaluateSSE(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "openai",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call": {
	    "type": "function",
	    "function": {
	      "name": "tool.search",
	      "arguments": "{\"query\":\"gait\"}"
	    }
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate/sse", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate sse status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	contentType := recorder.Header().Get("content-type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected sse content-type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: evaluate\n") || !strings.Contains(body, "\ndata: ") {
		t.Fatalf("unexpected sse body: %q", body)
	}
	lines := strings.Split(body, "\n")
	data := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	if data == "" {
		t.Fatalf("missing sse data line")
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		t.Fatalf("decode sse payload: %v", err)
	}
	if !response.OK || response.Verdict != "allow" || response.ExitCode != exitOK {
		t.Fatalf("unexpected sse response payload: %#v", response)
	}
}

func TestMCPServeHandlerEvaluateStream(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "openai",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call": {
	    "type": "function",
	    "function": {
	      "name": "tool.search",
	      "arguments": "{\"query\":\"gait\"}"
	    }
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate/stream", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate stream status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	contentType := recorder.Header().Get("content-type")
	if !strings.HasPrefix(contentType, "application/x-ndjson") {
		t.Fatalf("expected ndjson content-type, got %q", contentType)
	}
	body := strings.TrimSpace(recorder.Body.String())
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("decode ndjson payload: %v body=%q", err, body)
	}
	if !response.OK || response.Verdict != "allow" || response.ExitCode != exitOK {
		t.Fatalf("unexpected stream response payload: %#v", response)
	}
}
