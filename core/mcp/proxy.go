package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

const (
	defaultIdentity  = "mcp-proxy"
	defaultWorkspace = "."
	defaultRiskClass = "high"
)

type EvalResult struct {
	Call    ToolCall
	Intent  schemagate.IntentRequest
	Outcome gate.EvalOutcome
}

type IntentOptions struct {
	RequireExplicitContext bool
}

func EvaluateToolCall(policy gate.Policy, call ToolCall, opts gate.EvalOptions) (EvalResult, error) {
	return EvaluateToolCallWithIntentOptions(policy, call, opts, IntentOptions{})
}

func EvaluateToolCallWithIntentOptions(policy gate.Policy, call ToolCall, opts gate.EvalOptions, intentOpts IntentOptions) (EvalResult, error) {
	intent, err := ToIntentRequestWithOptions(call, intentOpts)
	if err != nil {
		return EvalResult{}, err
	}
	normalizedIntent, err := gate.NormalizeIntent(intent)
	if err != nil {
		return EvalResult{}, err
	}
	outcome, err := gate.EvaluatePolicyDetailed(policy, normalizedIntent, opts)
	if err != nil {
		return EvalResult{}, err
	}
	return EvalResult{
		Call:    call,
		Intent:  normalizedIntent,
		Outcome: outcome,
	}, nil
}

func ToIntentRequest(call ToolCall) (schemagate.IntentRequest, error) {
	return ToIntentRequestWithOptions(call, IntentOptions{})
}

func ToIntentRequestWithOptions(call ToolCall, opts IntentOptions) (schemagate.IntentRequest, error) {
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return schemagate.IntentRequest{}, fmt.Errorf("tool call name is required")
	}

	targets := call.Targets
	if len(targets) == 0 && strings.TrimSpace(call.Target) != "" {
		targets = []Target{inferLegacyTarget(call.Target)}
	}

	intentTargets := make([]schemagate.IntentTarget, 0, len(targets))
	for _, target := range targets {
		intentTargets = append(intentTargets, schemagate.IntentTarget{
			Kind:        strings.TrimSpace(target.Kind),
			Value:       strings.TrimSpace(target.Value),
			Operation:   strings.TrimSpace(target.Operation),
			Sensitivity: strings.TrimSpace(target.Sensitivity),
		})
	}

	provenance := make([]schemagate.IntentArgProvenance, 0, len(call.ArgProvenance))
	for _, entry := range call.ArgProvenance {
		provenance = append(provenance, schemagate.IntentArgProvenance{
			ArgPath:         strings.TrimSpace(entry.ArgPath),
			Source:          strings.TrimSpace(entry.Source),
			SourceRef:       strings.TrimSpace(entry.SourceRef),
			IntegrityDigest: strings.TrimSpace(entry.IntegrityDigest),
		})
	}

	args := call.Args
	if args == nil {
		args = map[string]any{}
	}

	createdAt := call.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	identity := defaultString(call.Context.Identity, defaultIdentity)
	workspace := defaultString(call.Context.Workspace, defaultWorkspace)
	riskClass := defaultString(call.Context.RiskClass, defaultRiskClass)
	if opts.RequireExplicitContext {
		if strings.TrimSpace(call.Context.Identity) == "" {
			return schemagate.IntentRequest{}, fmt.Errorf("context.identity is required in strict profile")
		}
		if strings.TrimSpace(call.Context.Workspace) == "" {
			return schemagate.IntentRequest{}, fmt.Errorf("context.workspace is required in strict profile")
		}
		if strings.TrimSpace(call.Context.SessionID) == "" {
			return schemagate.IntentRequest{}, fmt.Errorf("context.session_id is required in strict profile")
		}
		identity = strings.TrimSpace(call.Context.Identity)
		workspace = strings.TrimSpace(call.Context.Workspace)
		if strings.TrimSpace(call.Context.RiskClass) != "" {
			riskClass = strings.TrimSpace(call.Context.RiskClass)
		}
	}

	var delegation *schemagate.IntentDelegation
	if call.Delegation != nil {
		chain := make([]schemagate.DelegationLink, 0, len(call.Delegation.Chain))
		for _, link := range call.Delegation.Chain {
			chain = append(chain, schemagate.DelegationLink{
				DelegatorIdentity: strings.TrimSpace(link.DelegatorIdentity),
				DelegateIdentity:  strings.TrimSpace(link.DelegateIdentity),
				ScopeClass:        strings.TrimSpace(link.ScopeClass),
				TokenRef:          strings.TrimSpace(link.TokenRef),
			})
		}
		delegation = &schemagate.IntentDelegation{
			RequesterIdentity: strings.TrimSpace(call.Delegation.RequesterIdentity),
			ScopeClass:        strings.TrimSpace(call.Delegation.ScopeClass),
			TokenRefs:         append([]string{}, call.Delegation.TokenRefs...),
			Chain:             chain,
		}
	}

	return schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-dev",
		ToolName:        name,
		Args:            args,
		Targets:         intentTargets,
		ArgProvenance:   provenance,
		Delegation:      delegation,
		Context: schemagate.IntentContext{
			Identity:               identity,
			Workspace:              workspace,
			RiskClass:              riskClass,
			SessionID:              strings.TrimSpace(call.Context.SessionID),
			RequestID:              strings.TrimSpace(call.Context.RequestID),
			AuthContext:            call.Context.AuthContext,
			CredentialScopes:       append([]string{}, call.Context.CredentialScopes...),
			EnvironmentFingerprint: strings.TrimSpace(call.Context.EnvironmentFingerprint),
		},
	}, nil
}

func DecodeToolCall(adapter string, payload []byte) (ToolCall, error) {
	switch strings.ToLower(strings.TrimSpace(adapter)) {
	case "", "mcp":
		var call ToolCall
		if err := json.Unmarshal(payload, &call); err != nil {
			return ToolCall{}, fmt.Errorf("parse mcp tool call: %w", err)
		}
		return call, nil
	case "openai":
		return decodeOpenAIToolCall(payload)
	case "anthropic":
		return decodeAnthropicToolCall(payload)
	case "langchain":
		return decodeLangChainToolCall(payload)
	default:
		return ToolCall{}, fmt.Errorf("unsupported adapter: %s", adapter)
	}
}

func decodeOpenAIToolCall(payload []byte) (ToolCall, error) {
	var envelope struct {
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ToolCall{}, fmt.Errorf("parse openai tool call: %w", err)
	}
	args, err := decodeArguments(envelope.Function.Arguments)
	if err != nil {
		return ToolCall{}, fmt.Errorf("decode openai arguments: %w", err)
	}
	return ToolCall{
		Name: envelope.Function.Name,
		Args: args,
	}, nil
}

func decodeAnthropicToolCall(payload []byte) (ToolCall, error) {
	var envelope struct {
		Type  string         `json:"type"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ToolCall{}, fmt.Errorf("parse anthropic tool call: %w", err)
	}
	return ToolCall{
		Name: envelope.Name,
		Args: envelope.Input,
	}, nil
}

func decodeLangChainToolCall(payload []byte) (ToolCall, error) {
	var envelope struct {
		Tool      string `json:"tool"`
		ToolInput any    `json:"tool_input"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ToolCall{}, fmt.Errorf("parse langchain tool call: %w", err)
	}
	args, err := decodeArguments(envelope.ToolInput)
	if err != nil {
		return ToolCall{}, fmt.Errorf("decode langchain tool_input: %w", err)
	}
	return ToolCall{
		Name: envelope.Tool,
		Args: args,
	}, nil
}

func decodeArguments(raw any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	switch typed := raw.(type) {
	case map[string]any:
		return typed, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return map[string]any{}, nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(typed), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, err
		}
		var parsed map[string]any
		if err := json.Unmarshal(encoded, &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}
}

func inferLegacyTarget(raw string) Target {
	value := strings.TrimSpace(raw)
	kind := "other"
	switch {
	case strings.Contains(value, "://"):
		kind = "url"
	case strings.Contains(value, "/"):
		kind = "path"
	case value != "":
		kind = "host"
	}
	return Target{
		Kind:  kind,
		Value: value,
	}
}

func defaultString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
