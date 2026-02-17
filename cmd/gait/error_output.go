package main

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net"
	"strings"

	coreerrors "github.com/Clyra-AI/gait/core/errors"
)

func writeJSONOutput(output any, exitCode int) int {
	encoded, err := marshalOutputWithErrorEnvelope(output, exitCode)
	if err != nil {
		fmt.Println(`{"ok":false,"error":"failed to encode output","error_code":"encode_failed","error_category":"internal_failure","retryable":false}`)
		return exitInvalidInput
	}
	fmt.Println(string(encoded))
	return exitCode
}

func marshalOutputWithErrorEnvelope(output any, exitCode int) ([]byte, error) {
	encoded, err := marshalJSON(output)
	if err != nil {
		return nil, err
	}
	result, err := unmarshalJSONToMap(encoded)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(asString(result["correlation_id"])) == "" {
		if correlationID := currentCorrelationID(); correlationID != "" {
			result["correlation_id"] = correlationID
		}
	}
	errorText := strings.TrimSpace(asString(result["error"]))
	if errorText == "" {
		return marshalJSON(result)
	}
	if strings.TrimSpace(asString(result["error_code"])) == "" {
		result["error_code"] = defaultErrorCode(exitCode)
	}
	if strings.TrimSpace(asString(result["error_category"])) == "" {
		category := defaultErrorCategory(exitCode)
		result["error_category"] = string(category)
	}
	if _, exists := result["retryable"]; !exists {
		category := coreerrors.Category(asString(result["error_category"]))
		result["retryable"] = defaultRetryable(category)
	}
	if strings.TrimSpace(asString(result["hint"])) == "" {
		result["hint"] = defaultHint(exitCode)
	}
	return marshalJSON(result)
}

func exitCodeForError(err error, fallbackExit int) int {
	if err == nil {
		return exitOK
	}
	switch coreerrors.CategoryOf(err) {
	case coreerrors.CategoryInvalidInput:
		return exitInvalidInput
	case coreerrors.CategoryVerification:
		return exitVerifyFailed
	case coreerrors.CategoryPolicyBlocked:
		return exitPolicyBlocked
	case coreerrors.CategoryApprovalRequired:
		return exitApprovalRequired
	case coreerrors.CategoryDependencyMissing:
		return exitMissingDependency
	case coreerrors.CategoryIOFailure, coreerrors.CategoryStateContention, coreerrors.CategoryNetworkTransient, coreerrors.CategoryNetworkPermanent, coreerrors.CategoryInternalFailure:
		return exitInternalFailure
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return exitInternalFailure
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "lock") || strings.Contains(msg, "contention") || strings.Contains(msg, "timeout") {
		return exitInternalFailure
	}
	return fallbackExit
}

func defaultErrorCategory(exitCode int) coreerrors.Category {
	switch exitCode {
	case exitInvalidInput:
		return coreerrors.CategoryInvalidInput
	case exitVerifyFailed:
		return coreerrors.CategoryVerification
	case exitPolicyBlocked:
		return coreerrors.CategoryPolicyBlocked
	case exitApprovalRequired:
		return coreerrors.CategoryApprovalRequired
	case exitMissingDependency:
		return coreerrors.CategoryDependencyMissing
	default:
		return coreerrors.CategoryInternalFailure
	}
}

func defaultErrorCode(exitCode int) string {
	switch exitCode {
	case exitInvalidInput:
		return "invalid_input"
	case exitVerifyFailed:
		return "verification_failed"
	case exitPolicyBlocked:
		return "policy_blocked"
	case exitApprovalRequired:
		return "approval_required"
	case exitMissingDependency:
		return "dependency_missing"
	case exitUnsafeReplay:
		return "unsafe_operation"
	case exitRegressFailed:
		return "regress_failed"
	default:
		return "internal_failure"
	}
}

func defaultHint(exitCode int) string {
	switch exitCode {
	case exitInvalidInput:
		return "check command usage and input schema"
	case exitVerifyFailed:
		return "re-run verify after checking artifact integrity"
	case exitPolicyBlocked:
		return "inspect reason_codes and adjust policy or intent"
	case exitApprovalRequired:
		return "provide a valid approval token and retry"
	case exitMissingDependency:
		return "install or configure the missing dependency and retry"
	case exitUnsafeReplay:
		return "pass explicit unsafe flags only for approved replay paths"
	default:
		return "retry after checking local environment and logs"
	}
}

func defaultRetryable(category coreerrors.Category) bool {
	return category == coreerrors.CategoryNetworkTransient || category == coreerrors.CategoryStateContention
}

func marshalJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}

func unmarshalJSONToMap(payload []byte) (map[string]any, error) {
	output := map[string]any{}
	if err := json.Unmarshal(payload, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}
