package main

import (
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
)

func TestMarshalOutputWithErrorEnvelope(t *testing.T) {
	setCurrentCorrelationID("cid-test")
	t.Cleanup(func() {
		setCurrentCorrelationID("")
	})
	payload := map[string]any{
		"ok":    false,
		"error": "boom",
	}
	encoded, err := marshalOutputWithErrorEnvelope(payload, exitInvalidInput)
	if err != nil {
		t.Fatalf("marshalOutputWithErrorEnvelope error: %v", err)
	}
	result := string(encoded)
	if !strings.Contains(result, `"error_code":"invalid_input"`) {
		t.Fatalf("missing error_code in output: %s", result)
	}
	if !strings.Contains(result, `"error_category":"invalid_input"`) {
		t.Fatalf("missing error_category in output: %s", result)
	}
	if !strings.Contains(result, `"retryable":false`) {
		t.Fatalf("missing retryable in output: %s", result)
	}
	if !strings.Contains(result, `"hint":"check command usage and input schema"`) {
		t.Fatalf("missing hint in output: %s", result)
	}
	if !strings.Contains(result, `"correlation_id":"cid-test"`) {
		t.Fatalf("missing correlation id in output: %s", result)
	}
}

func TestMarshalOutputWithCorrelationForSuccess(t *testing.T) {
	setCurrentCorrelationID("cid-success")
	t.Cleanup(func() {
		setCurrentCorrelationID("")
	})
	payload := map[string]any{"ok": true}
	encoded, err := marshalOutputWithErrorEnvelope(payload, exitOK)
	if err != nil {
		t.Fatalf("marshalOutputWithErrorEnvelope error: %v", err)
	}
	result := string(encoded)
	if !strings.Contains(result, `"correlation_id":"cid-success"`) {
		t.Fatalf("missing correlation_id for success output: %s", result)
	}
}

func TestExitCodeForError(t *testing.T) {
	if got := exitCodeForError(stderrors.New("plain"), exitInvalidInput); got != exitInvalidInput {
		t.Fatalf("expected fallback invalid-input exit, got %d", got)
	}
	wrapped := coreerrors.Wrap(stderrors.New("approval"), coreerrors.CategoryApprovalRequired, "approval_required", "", false)
	if got := exitCodeForError(wrapped, exitInvalidInput); got != exitApprovalRequired {
		t.Fatalf("expected approval-required exit, got %d", got)
	}
	if got := exitCodeForError(stderrors.New("socket timeout"), exitInvalidInput); got != exitInternalFailure {
		t.Fatalf("expected internal failure exit, got %d", got)
	}
}

func TestDefaultErrorMappings(t *testing.T) {
	cases := []struct {
		exitCode int
		category string
		code     string
		hint     string
	}{
		{exitInvalidInput, string(coreerrors.CategoryInvalidInput), "invalid_input", "check command usage and input schema"},
		{exitVerifyFailed, string(coreerrors.CategoryVerification), "verification_failed", "re-run verify after checking artifact integrity"},
		{exitPolicyBlocked, string(coreerrors.CategoryPolicyBlocked), "policy_blocked", "inspect reason_codes and adjust policy or intent"},
		{exitApprovalRequired, string(coreerrors.CategoryApprovalRequired), "approval_required", "provide a valid approval token and retry"},
		{exitMissingDependency, string(coreerrors.CategoryDependencyMissing), "dependency_missing", "install or configure the missing dependency and retry"},
		{exitUnsafeReplay, string(coreerrors.CategoryInternalFailure), "unsafe_operation", "pass explicit unsafe flags only for approved replay paths"},
		{exitRegressFailed, string(coreerrors.CategoryInternalFailure), "regress_failed", "retry after checking local environment and logs"},
		{exitInternalFailure, string(coreerrors.CategoryInternalFailure), "internal_failure", "retry after checking local environment and logs"},
	}
	for _, tc := range cases {
		if got := string(defaultErrorCategory(tc.exitCode)); got != tc.category {
			t.Fatalf("defaultErrorCategory(%d): got %s want %s", tc.exitCode, got, tc.category)
		}
		if got := defaultErrorCode(tc.exitCode); got != tc.code {
			t.Fatalf("defaultErrorCode(%d): got %s want %s", tc.exitCode, got, tc.code)
		}
		if got := defaultHint(tc.exitCode); got != tc.hint {
			t.Fatalf("defaultHint(%d): got %s want %s", tc.exitCode, got, tc.hint)
		}
	}
	if !defaultRetryable(coreerrors.CategoryNetworkTransient) {
		t.Fatalf("network transient category should be retryable")
	}
	if !defaultRetryable(coreerrors.CategoryStateContention) {
		t.Fatalf("state contention category should be retryable")
	}
	if defaultRetryable(coreerrors.CategoryPolicyBlocked) {
		t.Fatalf("policy blocked category should not be retryable")
	}
}

func TestWriteJSONOutputEncodingFailureFallback(t *testing.T) {
	raw := captureStdout(t, func() {
		code := writeJSONOutput(map[string]any{
			"ok":    false,
			"error": "boom",
			"bad":   make(chan int),
		}, exitInvalidInput)
		if code != exitInvalidInput {
			t.Fatalf("writeJSONOutput fallback exit code: got %d want %d", code, exitInvalidInput)
		}
	})
	if !strings.Contains(raw, `"error_code":"encode_failed"`) {
		t.Fatalf("expected encode_failed fallback envelope, got %s", raw)
	}
}

func TestMarshalOutputWithProvidedEnvelopeFields(t *testing.T) {
	payload := map[string]any{
		"ok":             false,
		"error":          "already_enveloped",
		"error_code":     "custom_code",
		"error_category": "custom_category",
		"retryable":      true,
		"hint":           "custom_hint",
	}
	encoded, err := marshalOutputWithErrorEnvelope(payload, exitInternalFailure)
	if err != nil {
		t.Fatalf("marshalOutputWithErrorEnvelope: %v", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode enveloped output: %v", err)
	}
	if decoded["error_code"] != "custom_code" {
		t.Fatalf("expected custom error code to be preserved, got %#v", decoded["error_code"])
	}
	if decoded["error_category"] != "custom_category" {
		t.Fatalf("expected custom error category to be preserved, got %#v", decoded["error_category"])
	}
	if decoded["hint"] != "custom_hint" {
		t.Fatalf("expected custom hint to be preserved, got %#v", decoded["hint"])
	}
	if decoded["retryable"] != true {
		t.Fatalf("expected custom retryable to be preserved, got %#v", decoded["retryable"])
	}
}
