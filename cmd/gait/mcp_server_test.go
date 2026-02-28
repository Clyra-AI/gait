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
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
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

func TestMCPServeRelationshipCallActor(t *testing.T) {
	t.Run("matches calls edge for tool", func(t *testing.T) {
		relationship := &schemacommon.RelationshipEnvelope{
			Edges: []schemacommon.RelationshipEdge{
				{
					Kind: "calls",
					From: schemacommon.RelationshipNodeRef{Kind: "agent", ID: "agent.alpha"},
					To:   schemacommon.RelationshipNodeRef{Kind: "tool", ID: "tool.write"},
				},
			},
		}
		got := mcpServeRelationshipCallActor(relationship, "tool.write")
		if got != "agent.alpha" {
			t.Fatalf("expected actor identity from calls edge, got %q", got)
		}
	})

	t.Run("returns empty when tool does not match", func(t *testing.T) {
		relationship := &schemacommon.RelationshipEnvelope{
			Edges: []schemacommon.RelationshipEdge{
				{
					Kind: "calls",
					From: schemacommon.RelationshipNodeRef{Kind: "agent", ID: "agent.alpha"},
					To:   schemacommon.RelationshipNodeRef{Kind: "tool", ID: "tool.read"},
				},
			},
		}
		got := mcpServeRelationshipCallActor(relationship, "tool.write")
		if got != "" {
			t.Fatalf("expected empty actor for non-matching tool, got %q", got)
		}
	})

	t.Run("returns empty when relationship is nil", func(t *testing.T) {
		if got := mcpServeRelationshipCallActor(nil, "tool.write"); got != "" {
			t.Fatalf("expected empty actor with nil relationship, got %q", got)
		}
	})
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

func TestMCPServeHandlerAutoEmitsPackForStateChangingCall(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	packDir := filepath.Join(workDir, "packs")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		TraceDir:       filepath.Join(workDir, "traces"),
		PackDir:        packDir,
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "run_id":"run_mcp_pack_auto",
	  "call":{
	    "name":"tool.write",
	    "args":{"path":"/tmp/out.txt"},
	    "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"write"}],
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-pack-auto"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode evaluate response: %v", err)
	}
	if strings.TrimSpace(response.PackPath) == "" || strings.TrimSpace(response.PackID) == "" {
		t.Fatalf("expected auto-emitted pack metadata, got %#v", response)
	}
	if _, err := os.Stat(response.PackPath); err != nil {
		t.Fatalf("expected emitted pack path to exist: %v", err)
	}
	if code := runPack([]string{"verify", response.PackPath, "--json"}); code != exitOK {
		t.Fatalf("pack verify expected %d got %d", exitOK, code)
	}
}

func TestMCPServeHandlerReadOnlyCallDoesNotAutoEmitPack(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "openai",
		TraceDir:       filepath.Join(workDir, "traces"),
		PackDir:        filepath.Join(workDir, "packs"),
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
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("evaluate status: expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode evaluate response: %v", err)
	}
	if strings.TrimSpace(response.PackPath) != "" {
		t.Fatalf("expected read-only call to skip auto pack emission, got %#v", response)
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

func TestMCPServeHandlerRejectsMissingOAuthEvidenceInOSSProd(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	privateKeyPath := filepath.Join(workDir, "trace_private.key")
	writePrivateKey(t, privateKeyPath)

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		Profile:        "oss-prod",
		TraceDir:       filepath.Join(workDir, "traces"),
		KeyMode:        "prod",
		PrivateKey:     privateKeyPath,
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{
	      "identity":"alice",
	      "workspace":"/repo/gait",
	      "risk_class":"high",
	      "session_id":"sess-1",
	      "auth_mode":"oauth_dcr"
	    }
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
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

func TestMCPServeHandlerSessionJournalAndCheckpoint(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:     policyPath,
		DefaultAdapter: "mcp",
		Profile:        "standard",
		TraceDir:       filepath.Join(workDir, "traces"),
		RunpackDir:     filepath.Join(workDir, "runpacks"),
		SessionDir:     filepath.Join(workDir, "sessions"),
		KeyMode:        "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "session_id":"sess-server-1",
	  "checkpoint_interval":1,
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","session_id":"sess-server-1"}
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
	if !response.OK || response.Verdict != "allow" {
		t.Fatalf("unexpected session evaluate response: %#v", response)
	}
	foundCheckpointWarning := false
	for _, warning := range response.Warnings {
		if strings.Contains(warning, "session_checkpoint=") {
			foundCheckpointWarning = true
			break
		}
	}
	if !foundCheckpointWarning {
		t.Fatalf("expected checkpoint warning in response warnings: %#v", response.Warnings)
	}
}

func TestMCPServeHandlerRejectsClientArtifactPathOverridesByDefault(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          filepath.Join(workDir, "traces"),
		SessionDir:        filepath.Join(workDir, "sessions"),
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "compat",
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "trace_path":"/tmp/forbidden.json",
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestMCPServeHandlerAllowsClientArtifactPathOverridesWhenEnabled(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	tracePath := filepath.Join(workDir, "custom_trace.json")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:               policyPath,
		DefaultAdapter:           "mcp",
		TraceDir:                 filepath.Join(workDir, "traces"),
		SessionDir:               filepath.Join(workDir, "sessions"),
		AllowClientArtifactPaths: true,
		MaxRequestBytes:          1 << 20,
		HTTPVerdictStatus:        "compat",
		KeyMode:                  "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}
	requestPayload := map[string]any{
		"trace_path": tracePath,
		"call": map[string]any{
			"name": "tool.search",
			"args": map[string]any{"query": "gait"},
			"context": map[string]any{
				"identity":   "alice",
				"workspace":  "/repo/gait",
				"session_id": "sess-1",
			},
		},
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(tracePath); err != nil {
		t.Fatalf("expected client trace path to be written: %v", err)
	}
}

func TestMCPServeHandlerRequiresBearerAuthorizationWhenEnabled(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          filepath.Join(workDir, "traces"),
		AuthMode:          "token",
		AuthToken:         "s3cret",
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "compat",
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
}

func TestMCPServeHandlerAcceptsBearerAuthorizationWhenEnabled(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          filepath.Join(workDir, "traces"),
		AuthMode:          "token",
		AuthToken:         "s3cret",
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "compat",
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}

	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	request.Header.Set("authorization", "Bearer s3cret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestMCPServeHandlerRejectsOversizedRequest(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          filepath.Join(workDir, "traces"),
		MaxRequestBytes:   256,
		HTTPVerdictStatus: "compat",
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}
	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"` + strings.Repeat("x", 1024) + `"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected %d got %d body=%s", http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	}
}

func TestMCPServeHandlerStrictVerdictStatusForBlock(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: block\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "openai",
		TraceDir:          filepath.Join(workDir, "traces"),
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "strict",
		KeyMode:           "dev",
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
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected %d got %d body=%s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
	var response mcpServeEvaluateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Executed {
		t.Fatalf("expected executed=false in strict block response")
	}
}

func TestMCPServeHandlerTraceRetentionMaxCount(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	traceDir := filepath.Join(workDir, "traces")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          traceDir,
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "compat",
		TraceMaxCount:     1,
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}
	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
		request.Header.Set("content-type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d expected %d got %d body=%s", i+1, http.StatusOK, recorder.Code, recorder.Body.String())
		}
		time.Sleep(2 * time.Millisecond)
	}
	matches, err := filepath.Glob(filepath.Join(traceDir, "*.json"))
	if err != nil {
		t.Fatalf("glob trace files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 retained trace file, got %d (%v)", len(matches), matches)
	}
}

func TestMCPServeHandlerTraceRetentionMaxAge(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	traceDir := filepath.Join(workDir, "traces")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")
	if err := os.MkdirAll(traceDir, 0o750); err != nil {
		t.Fatalf("mkdir trace dir: %v", err)
	}
	oldTrace := filepath.Join(traceDir, "trace_old.json")
	mustWriteFile(t, oldTrace, "{}\n")
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	if err := os.Chtimes(oldTrace, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old trace: %v", err)
	}
	handler, err := newMCPServeHandler(mcpServeConfig{
		PolicyPath:        policyPath,
		DefaultAdapter:    "mcp",
		TraceDir:          traceDir,
		MaxRequestBytes:   1 << 20,
		HTTPVerdictStatus: "compat",
		TraceMaxAge:       time.Hour,
		KeyMode:           "dev",
	})
	if err != nil {
		t.Fatalf("newMCPServeHandler: %v", err)
	}
	requestBody := []byte(`{
	  "call":{
	    "name":"tool.search",
	    "args":{"query":"gait"},
	    "context":{"identity":"alice","workspace":"/repo/gait","session_id":"sess-1"}
	  }
	}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(requestBody))
	request.Header.Set("content-type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(oldTrace); !os.IsNotExist(err) {
		t.Fatalf("expected old trace to be pruned, stat err=%v", err)
	}
}

func TestParseOptionalDuration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		raw       string
		want      time.Duration
		expectErr bool
	}{
		{name: "empty", raw: "", want: 0},
		{name: "zero", raw: "0", want: 0},
		{name: "positive", raw: "48h", want: 48 * time.Hour},
		{name: "negative", raw: "-1h", expectErr: true},
		{name: "invalid", raw: "not-a-duration", expectErr: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseOptionalDuration(testCase.raw)
			if testCase.expectErr {
				if err == nil {
					t.Fatalf("expected error for raw=%q", testCase.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOptionalDuration(%q): %v", testCase.raw, err)
			}
			if got != testCase.want {
				t.Fatalf("duration mismatch: expected %s got %s", testCase.want, got)
			}
		})
	}
}

func TestMCPServeIsLoopbackListen(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		addr      string
		want      bool
		expectErr bool
	}{
		{name: "localhost", addr: "localhost:8787", want: true},
		{name: "ipv4 loopback", addr: "127.0.0.1:8787", want: true},
		{name: "ipv6 loopback", addr: "[::1]:8787", want: true},
		{name: "non loopback", addr: "0.0.0.0:8787", want: false},
		{name: "hostname", addr: "example.com:8787", want: false},
		{name: "missing", addr: "", expectErr: true},
		{name: "invalid", addr: "bad-listen", expectErr: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := mcpServeIsLoopbackListen(testCase.addr)
			if testCase.expectErr {
				if err == nil {
					t.Fatalf("expected error for addr=%q", testCase.addr)
				}
				return
			}
			if err != nil {
				t.Fatalf("mcpServeIsLoopbackListen(%q): %v", testCase.addr, err)
			}
			if got != testCase.want {
				t.Fatalf("loopback mismatch: expected %v got %v", testCase.want, got)
			}
		})
	}
}

func TestMCPRetentionMatches(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		class    string
		fileName string
		want     bool
	}{
		{name: "trace json", class: "trace", fileName: "trace_abc.json", want: true},
		{name: "trace non-json", class: "trace", fileName: "trace_abc.txt", want: false},
		{name: "runpack zip", class: "runpack", fileName: "runpack_abc.zip", want: true},
		{name: "runpack checkpoint", class: "runpack", fileName: "sess_1_cp_000001.zip", want: true},
		{name: "runpack excludes pack", class: "runpack", fileName: "pack_abc.zip", want: false},
		{name: "runpack generic zip", class: "runpack", fileName: "artifact.zip", want: false},
		{name: "runpack json", class: "runpack", fileName: "runpack_abc.json", want: false},
		{name: "pack zip", class: "pack", fileName: "pack_abc.zip", want: true},
		{name: "pack excludes runpack", class: "pack", fileName: "runpack_abc.zip", want: false},
		{name: "session json", class: "session", fileName: "sess_1.json", want: true},
		{name: "session jsonl", class: "session", fileName: "sess_1.journal.jsonl", want: true},
		{name: "session state", class: "session", fileName: "sess_1.state", want: true},
		{name: "session other", class: "session", fileName: "sess_1.log", want: false},
		{name: "unknown class", class: "unknown", fileName: "any.json", want: false},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := mcpRetentionMatches(testCase.class, testCase.fileName)
			if got != testCase.want {
				t.Fatalf("mcpRetentionMatches(%q,%q): expected %v got %v", testCase.class, testCase.fileName, testCase.want, got)
			}
		})
	}
}

func TestApplyMCPServeRetentionKeepsPackAndRunpackClassesIsolated(t *testing.T) {
	artifactDir := t.TempDir()
	now := time.Now().UTC()

	runpackOld := filepath.Join(artifactDir, "runpack_old.zip")
	runpackNew := filepath.Join(artifactDir, "runpack_new.zip")
	packOld := filepath.Join(artifactDir, "pack_old.zip")
	packNew := filepath.Join(artifactDir, "pack_new.zip")

	mustWriteFile(t, runpackOld, "runpack old")
	mustWriteFile(t, runpackNew, "runpack new")
	mustWriteFile(t, packOld, "pack old")
	mustWriteFile(t, packNew, "pack new")

	if err := os.Chtimes(runpackOld, now.Add(-4*time.Hour), now.Add(-4*time.Hour)); err != nil {
		t.Fatalf("chtimes runpack_old: %v", err)
	}
	if err := os.Chtimes(runpackNew, now.Add(-10*time.Minute), now.Add(-10*time.Minute)); err != nil {
		t.Fatalf("chtimes runpack_new: %v", err)
	}
	if err := os.Chtimes(packOld, now.Add(-3*time.Hour), now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("chtimes pack_old: %v", err)
	}
	if err := os.Chtimes(packNew, now.Add(-20*time.Minute), now.Add(-20*time.Minute)); err != nil {
		t.Fatalf("chtimes pack_new: %v", err)
	}

	warnings := applyMCPServeRetention(mcpServeConfig{
		RunpackDir:      artifactDir,
		PackDir:         artifactDir,
		RunpackMaxAge:   0,
		RunpackMaxCount: 1,
		PackMaxAge:      0,
		PackMaxCount:    1,
	}, now)
	if len(warnings) == 0 {
		t.Fatalf("expected retention warnings for removed files")
	}
	if !containsWarningPrefix(warnings, "retention_runpack_removed_age=") {
		t.Fatalf("expected runpack retention warning, got %#v", warnings)
	}
	if !containsWarningPrefix(warnings, "retention_pack_removed_age=") {
		t.Fatalf("expected pack retention warning, got %#v", warnings)
	}

	if _, err := os.Stat(runpackNew); err != nil {
		t.Fatalf("expected latest runpack to remain: %v", err)
	}
	if _, err := os.Stat(packNew); err != nil {
		t.Fatalf("expected latest pack to remain: %v", err)
	}
	if _, err := os.Stat(runpackOld); !os.IsNotExist(err) {
		t.Fatalf("expected old runpack to be pruned, err=%v", err)
	}
	if _, err := os.Stat(packOld); !os.IsNotExist(err) {
		t.Fatalf("expected old pack to be pruned, err=%v", err)
	}
}

func TestMCPServeVerdictHTTPStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mode    string
		verdict string
		want    int
	}{
		{name: "compat block", mode: "compat", verdict: "block", want: http.StatusOK},
		{name: "strict block", mode: "strict", verdict: "block", want: http.StatusForbidden},
		{name: "strict require approval", mode: "strict", verdict: "require_approval", want: http.StatusConflict},
		{name: "strict dry run", mode: "strict", verdict: "dry_run", want: http.StatusConflict},
		{name: "strict allow", mode: "strict", verdict: "allow", want: http.StatusOK},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := mcpServeVerdictHTTPStatus(
				mcpServeConfig{HTTPVerdictStatus: testCase.mode},
				mcpServeEvaluateResponse{mcpProxyOutput: mcpProxyOutput{Verdict: testCase.verdict}},
			)
			if got != testCase.want {
				t.Fatalf("status mismatch: expected %d got %d", testCase.want, got)
			}
		})
	}
}

func TestRunMCPServeValidationErrors(t *testing.T) {
	workDir := t.TempDir()
	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, "default_verdict: allow\n")

	testCases := []struct {
		name      string
		envKey    string
		envValue  string
		arguments []string
	}{
		{
			name:      "invalid trace max age",
			arguments: []string{"--json", "--policy", policyPath, "--trace-max-age", "bad"},
		},
		{
			name:      "invalid runpack max age",
			arguments: []string{"--json", "--policy", policyPath, "--runpack-max-age", "bad"},
		},
		{
			name:      "invalid session max age",
			arguments: []string{"--json", "--policy", policyPath, "--session-max-age", "bad"},
		},
		{
			name:      "invalid listen",
			arguments: []string{"--json", "--policy", policyPath, "--listen", "bad-listen"},
		},
		{
			name:      "non loopback requires token",
			arguments: []string{"--json", "--policy", policyPath, "--listen", "0.0.0.0:8787"},
		},
		{
			name:      "token mode requires auth token env flag",
			arguments: []string{"--json", "--policy", policyPath, "--listen", "0.0.0.0:8787", "--auth-mode", "token"},
		},
		{
			name:      "token mode requires non empty env",
			arguments: []string{"--json", "--policy", policyPath, "--listen", "0.0.0.0:8787", "--auth-mode", "token", "--auth-token-env", "GAIT_EMPTY_TOKEN"},
		},
		{
			name:      "invalid max request bytes",
			arguments: []string{"--json", "--policy", policyPath, "--max-request-bytes", "0"},
		},
		{
			name:      "invalid http verdict status",
			arguments: []string{"--json", "--policy", policyPath, "--http-verdict-status", "bad"},
		},
		{
			name:      "negative retention count",
			arguments: []string{"--json", "--policy", policyPath, "--trace-max-count", "-1"},
		},
		{
			name:      "invalid auth mode",
			arguments: []string{"--json", "--policy", policyPath, "--auth-mode", "bad"},
		},
	}

	t.Setenv("GAIT_EMPTY_TOKEN", "")
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if testCase.envKey != "" {
				t.Setenv(testCase.envKey, testCase.envValue)
			}
			code := runMCPServe(testCase.arguments)
			if code != exitInvalidInput {
				t.Fatalf("expected exit code %d got %d", exitInvalidInput, code)
			}
		})
	}
}

func containsWarningPrefix(warnings []string, prefix string) bool {
	for _, warning := range warnings {
		if strings.HasPrefix(warning, prefix) {
			return true
		}
	}
	return false
}
