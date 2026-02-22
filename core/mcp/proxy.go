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
	hasScript := call.Script != nil && len(call.Script.Steps) > 0
	if name == "" && !hasScript {
		return schemagate.IntentRequest{}, fmt.Errorf("tool call name is required")
	}

	targets := call.Targets
	if len(targets) == 0 && strings.TrimSpace(call.Target) != "" {
		targets = []Target{inferLegacyTarget(call.Target)}
	}

	intentTargets := make([]schemagate.IntentTarget, 0, len(targets))
	for _, target := range targets {
		hints := normalizeTargetHints(target)
		intentTargets = append(intentTargets, schemagate.IntentTarget{
			Kind:            strings.TrimSpace(target.Kind),
			Value:           strings.TrimSpace(target.Value),
			Operation:       strings.TrimSpace(target.Operation),
			Sensitivity:     strings.TrimSpace(target.Sensitivity),
			DiscoveryMethod: hints.DiscoveryMethod,
			ReadOnlyHint:    hints.ReadOnlyHint,
			DestructiveHint: hints.DestructiveHint,
			IdempotentHint:  hints.IdempotentHint,
			OpenWorldHint:   hints.OpenWorldHint,
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
	var script *schemagate.IntentScript
	if hasScript {
		steps := make([]schemagate.IntentScriptStep, 0, len(call.Script.Steps))
		for index, step := range call.Script.Steps {
			stepName := strings.TrimSpace(step.Name)
			if stepName == "" {
				return schemagate.IntentRequest{}, fmt.Errorf("script.steps[%d].name is required", index)
			}
			stepTargets := make([]schemagate.IntentTarget, 0, len(step.Targets))
			for _, target := range step.Targets {
				hints := normalizeTargetHints(target)
				stepTargets = append(stepTargets, schemagate.IntentTarget{
					Kind:            strings.TrimSpace(target.Kind),
					Value:           strings.TrimSpace(target.Value),
					Operation:       strings.TrimSpace(target.Operation),
					Sensitivity:     strings.TrimSpace(target.Sensitivity),
					DiscoveryMethod: hints.DiscoveryMethod,
					ReadOnlyHint:    hints.ReadOnlyHint,
					DestructiveHint: hints.DestructiveHint,
					IdempotentHint:  hints.IdempotentHint,
					OpenWorldHint:   hints.OpenWorldHint,
				})
			}
			stepProvenance := make([]schemagate.IntentArgProvenance, 0, len(step.ArgProvenance))
			for _, entry := range step.ArgProvenance {
				stepProvenance = append(stepProvenance, schemagate.IntentArgProvenance{
					ArgPath:         strings.TrimSpace(entry.ArgPath),
					Source:          strings.TrimSpace(entry.Source),
					SourceRef:       strings.TrimSpace(entry.SourceRef),
					IntegrityDigest: strings.TrimSpace(entry.IntegrityDigest),
				})
			}
			stepArgs := step.Args
			if stepArgs == nil {
				stepArgs = map[string]any{}
			}
			steps = append(steps, schemagate.IntentScriptStep{
				ToolName:      stepName,
				Args:          stepArgs,
				Targets:       stepTargets,
				ArgProvenance: stepProvenance,
			})
		}
		script = &schemagate.IntentScript{Steps: steps}
		name = "script"
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
	authContext := map[string]any{}
	for key, value := range call.Context.AuthContext {
		authContext[key] = value
	}
	if strings.TrimSpace(call.Context.AuthMode) != "" {
		authContext["auth_mode"] = strings.TrimSpace(call.Context.AuthMode)
	}
	if call.Context.OAuthEvidence != nil {
		authContext["oauth_evidence"] = call.Context.OAuthEvidence
	}

	return schemagate.IntentRequest{
		SchemaID:        "gait.gate.intent_request",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: "0.0.0-dev",
		ToolName:        name,
		Args:            args,
		Script:          script,
		Targets:         intentTargets,
		ArgProvenance:   provenance,
		Delegation:      delegation,
		Context: schemagate.IntentContext{
			Identity:               identity,
			Workspace:              workspace,
			RiskClass:              riskClass,
			SessionID:              strings.TrimSpace(call.Context.SessionID),
			RequestID:              strings.TrimSpace(call.Context.RequestID),
			AuthContext:            authContext,
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
	case "claude_code", "claude-code", "claudecode":
		return decodeClaudeCodeToolCall(payload)
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

func decodeClaudeCodeToolCall(payload []byte) (ToolCall, error) {
	var envelope struct {
		Type          string `json:"type"`
		Name          string `json:"name"`
		Input         any    `json:"input"`
		SessionID     string `json:"session_id"`
		ToolName      string `json:"tool_name"`
		ToolInput     any    `json:"tool_input"`
		HookEventName string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ToolCall{}, fmt.Errorf("parse claude code tool call: %w", err)
	}

	rawName := strings.TrimSpace(envelope.ToolName)
	if rawName == "" {
		rawName = strings.TrimSpace(envelope.Name)
	}
	name := normalizeClaudeCodeToolName(rawName)
	if name == "" {
		return ToolCall{}, fmt.Errorf("claude code tool call name is required")
	}

	rawArgs := envelope.ToolInput
	if rawArgs == nil {
		rawArgs = envelope.Input
	}
	args, err := decodeArguments(rawArgs)
	if err != nil {
		return ToolCall{}, fmt.Errorf("decode claude code tool_input: %w", err)
	}

	call := ToolCall{
		Name: name,
		Args: args,
	}
	if sessionID := strings.TrimSpace(envelope.SessionID); sessionID != "" {
		call.Context.SessionID = sessionID
	}
	if hookEventName := strings.TrimSpace(envelope.HookEventName); hookEventName != "" {
		call.Context.AuthContext = map[string]any{
			"hook_event_name": hookEventName,
			"adapter":         "claude_code",
		}
	}

	call.Targets = inferClaudeCodeTargets(name, args)
	return call, nil
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

var claudeCodeToolNameMap = map[string]string{
	"read":         "tool.read",
	"grep":         "tool.read",
	"glob":         "tool.read",
	"webfetch":     "tool.read",
	"websearch":    "tool.read",
	"write":        "tool.write",
	"edit":         "tool.write",
	"notebookedit": "tool.write",
	"bash":         "tool.exec",
	"task":         "tool.delegate",
}

func normalizeClaudeCodeToolName(rawName string) string {
	trimmed := strings.TrimSpace(rawName)
	if trimmed == "" {
		return ""
	}
	canonical := strings.ToLower(strings.TrimSpace(trimmed))
	canonical = strings.NewReplacer(" ", "", "_", "", "-", "").Replace(canonical)
	if mapped, ok := claudeCodeToolNameMap[canonical]; ok {
		return mapped
	}
	if strings.Contains(trimmed, ".") {
		return strings.ToLower(trimmed)
	}
	normalized := strings.ToLower(strings.TrimSpace(trimmed))
	normalized = strings.NewReplacer(" ", "_", "-", "_").Replace(normalized)
	return "tool." + normalized
}

type targetHints struct {
	DiscoveryMethod string
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
	OpenWorldHint   bool
}

func normalizeTargetHints(target Target) targetHints {
	hints := targetHints{
		DiscoveryMethod: strings.TrimSpace(target.DiscoveryMethod),
		ReadOnlyHint:    target.ReadOnlyHint || target.ReadOnlyHintAlias,
		DestructiveHint: target.DestructiveHint || target.DestructiveHintAlias,
		IdempotentHint:  target.IdempotentHint || target.IdempotentHintAlias,
		OpenWorldHint:   target.OpenWorldHint || target.OpenWorldHintAlias,
	}
	if hints.DiscoveryMethod == "" {
		hints.DiscoveryMethod = strings.TrimSpace(target.DiscoveryMethodAlias)
	}
	if value, ok := annotationString(target.Annotations, "discovery_method", "discoveryMethod"); ok {
		hints.DiscoveryMethod = strings.TrimSpace(value)
	}
	if value, ok := annotationBool(target.Annotations, "read_only_hint", "readOnlyHint"); ok {
		hints.ReadOnlyHint = value
	}
	if value, ok := annotationBool(target.Annotations, "destructive_hint", "destructiveHint"); ok {
		hints.DestructiveHint = value
	}
	if value, ok := annotationBool(target.Annotations, "idempotent_hint", "idempotentHint"); ok {
		hints.IdempotentHint = value
	}
	if value, ok := annotationBool(target.Annotations, "open_world_hint", "openWorldHint"); ok {
		hints.OpenWorldHint = value
	}
	return hints
}

func annotationBool(values map[string]any, keys ...string) (bool, bool) {
	if values == nil {
		return false, false
	}
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			normalized := strings.ToLower(strings.TrimSpace(typed))
			if normalized == "true" {
				return true, true
			}
			if normalized == "false" {
				return false, true
			}
		}
	}
	return false, false
}

func annotationString(values map[string]any, keys ...string) (string, bool) {
	if values == nil {
		return "", false
	}
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed, true
		case fmt.Stringer:
			return typed.String(), true
		}
	}
	return "", false
}

func inferClaudeCodeTargets(toolName string, args map[string]any) []Target {
	switch toolName {
	case "tool.read":
		targets := make([]Target, 0, 2)
		if path := firstNonEmptyString(args, "path", "file_path", "filepath", "file"); path != "" {
			targets = append(targets, Target{
				Kind:      "path",
				Value:     path,
				Operation: "read",
			})
		}
		if url := firstNonEmptyString(args, "url", "uri"); url != "" {
			targets = append(targets, Target{
				Kind:      "url",
				Value:     url,
				Operation: "read",
			})
		}
		return targets
	case "tool.write":
		if path := firstNonEmptyString(args, "path", "file_path", "filepath", "file"); path != "" {
			return []Target{{
				Kind:      "path",
				Value:     path,
				Operation: "write",
			}}
		}
	case "tool.exec":
		if command := firstNonEmptyString(args, "command", "cmd", "script"); command != "" {
			return []Target{{
				Kind:      "other",
				Value:     command,
				Operation: "execute",
			}}
		}
	case "tool.delegate":
		if task := firstNonEmptyString(args, "task", "prompt", "description"); task != "" {
			return []Target{{
				Kind:      "other",
				Value:     task,
				Operation: "delegate",
			}}
		}
	}
	return nil
}

func firstNonEmptyString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
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
