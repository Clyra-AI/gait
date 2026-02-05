package gate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/davidahmann/gait/core/jcs"
	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

const (
	intentRequestSchemaID = "gait.gate.intent_request"
	intentRequestSchemaV1 = "1.0.0"
)

var (
	allowedTargetKinds = map[string]struct{}{
		"path":   {},
		"url":    {},
		"host":   {},
		"repo":   {},
		"bucket": {},
		"table":  {},
		"queue":  {},
		"topic":  {},
		"other":  {},
	}
	allowedProvenanceSources = map[string]struct{}{
		"user":        {},
		"tool_output": {},
		"external":    {},
		"system":      {},
	}
	hexDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type normalizedIntent struct {
	ToolName      string                           `json:"tool_name"`
	Args          map[string]any                   `json:"args"`
	Targets       []schemagate.IntentTarget        `json:"targets"`
	ArgProvenance []schemagate.IntentArgProvenance `json:"arg_provenance,omitempty"`
	Context       schemagate.IntentContext         `json:"context"`
}

func NormalizeIntent(input schemagate.IntentRequest) (schemagate.IntentRequest, error) {
	normalized, err := normalizeIntent(input)
	if err != nil {
		return schemagate.IntentRequest{}, err
	}

	argsDigest, err := digestArgs(normalized.Args)
	if err != nil {
		return schemagate.IntentRequest{}, err
	}
	intentDigest, err := digestNormalizedIntent(normalized)
	if err != nil {
		return schemagate.IntentRequest{}, err
	}

	output := input
	if output.SchemaID == "" {
		output.SchemaID = intentRequestSchemaID
	}
	if output.SchemaVersion == "" {
		output.SchemaVersion = intentRequestSchemaV1
	}
	output.ToolName = normalized.ToolName
	output.Args = normalized.Args
	output.ArgsDigest = argsDigest
	output.IntentDigest = intentDigest
	output.Targets = normalized.Targets
	output.ArgProvenance = normalized.ArgProvenance
	output.Context = normalized.Context
	return output, nil
}

func NormalizedIntentBytes(input schemagate.IntentRequest) ([]byte, error) {
	normalized, err := normalizeIntent(input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized intent: %w", err)
	}
	return jcs.CanonicalizeJSON(raw)
}

func IntentDigest(input schemagate.IntentRequest) (string, error) {
	normalized, err := normalizeIntent(input)
	if err != nil {
		return "", err
	}
	return digestNormalizedIntent(normalized)
}

func ArgsDigest(args map[string]any) (string, error) {
	normalizedValue, err := normalizeJSONValue(args)
	if err != nil {
		return "", err
	}
	normalizedArgs, ok := normalizedValue.(map[string]any)
	if !ok {
		return "", fmt.Errorf("args must normalize to object")
	}
	return digestArgs(normalizedArgs)
}

func normalizeIntent(input schemagate.IntentRequest) (normalizedIntent, error) {
	toolName := strings.TrimSpace(input.ToolName)
	if toolName == "" {
		return normalizedIntent{}, fmt.Errorf("tool_name is required")
	}

	normalizedValue, err := normalizeJSONValue(input.Args)
	if err != nil {
		return normalizedIntent{}, fmt.Errorf("normalize args: %w", err)
	}
	args, ok := normalizedValue.(map[string]any)
	if !ok {
		return normalizedIntent{}, fmt.Errorf("args must be a JSON object")
	}

	targets, err := normalizeTargets(input.Targets)
	if err != nil {
		return normalizedIntent{}, err
	}
	provenance, err := normalizeArgProvenance(input.ArgProvenance)
	if err != nil {
		return normalizedIntent{}, err
	}
	context, err := normalizeContext(input.Context)
	if err != nil {
		return normalizedIntent{}, err
	}

	return normalizedIntent{
		ToolName:      toolName,
		Args:          args,
		Targets:       targets,
		ArgProvenance: provenance,
		Context:       context,
	}, nil
}

func normalizeTargets(targets []schemagate.IntentTarget) ([]schemagate.IntentTarget, error) {
	if len(targets) == 0 {
		return []schemagate.IntentTarget{}, nil
	}
	seen := make(map[string]struct{}, len(targets))
	out := make([]schemagate.IntentTarget, 0, len(targets))
	for _, target := range targets {
		kind := strings.ToLower(strings.TrimSpace(target.Kind))
		value := strings.TrimSpace(target.Value)
		operation := strings.ToLower(strings.TrimSpace(target.Operation))
		sensitivity := strings.ToLower(strings.TrimSpace(target.Sensitivity))

		if kind == "" || value == "" {
			return nil, fmt.Errorf("targets require kind and value")
		}
		if _, ok := allowedTargetKinds[kind]; !ok {
			return nil, fmt.Errorf("unsupported target kind: %s", kind)
		}

		normalized := schemagate.IntentTarget{
			Kind:        kind,
			Value:       value,
			Operation:   operation,
			Sensitivity: sensitivity,
		}
		key := strings.Join([]string{normalized.Kind, normalized.Value, normalized.Operation, normalized.Sensitivity}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Value != out[j].Value {
			return out[i].Value < out[j].Value
		}
		if out[i].Operation != out[j].Operation {
			return out[i].Operation < out[j].Operation
		}
		return out[i].Sensitivity < out[j].Sensitivity
	})
	return out, nil
}

func normalizeArgProvenance(provenance []schemagate.IntentArgProvenance) ([]schemagate.IntentArgProvenance, error) {
	if len(provenance) == 0 {
		return []schemagate.IntentArgProvenance{}, nil
	}
	seen := make(map[string]struct{}, len(provenance))
	out := make([]schemagate.IntentArgProvenance, 0, len(provenance))
	for _, entry := range provenance {
		argPath := strings.TrimSpace(entry.ArgPath)
		source := strings.ToLower(strings.TrimSpace(entry.Source))
		sourceRef := strings.TrimSpace(entry.SourceRef)
		integrityDigest := strings.ToLower(strings.TrimSpace(entry.IntegrityDigest))

		if argPath == "" || source == "" {
			return nil, fmt.Errorf("arg provenance requires arg_path and source")
		}
		if _, ok := allowedProvenanceSources[source]; !ok {
			return nil, fmt.Errorf("unsupported provenance source: %s", source)
		}
		if integrityDigest != "" && !hexDigestPattern.MatchString(integrityDigest) {
			return nil, fmt.Errorf("invalid provenance integrity_digest: %s", integrityDigest)
		}

		normalized := schemagate.IntentArgProvenance{
			ArgPath:         argPath,
			Source:          source,
			SourceRef:       sourceRef,
			IntegrityDigest: integrityDigest,
		}
		key := strings.Join([]string{normalized.ArgPath, normalized.Source, normalized.SourceRef, normalized.IntegrityDigest}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ArgPath != out[j].ArgPath {
			return out[i].ArgPath < out[j].ArgPath
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].SourceRef != out[j].SourceRef {
			return out[i].SourceRef < out[j].SourceRef
		}
		return out[i].IntegrityDigest < out[j].IntegrityDigest
	})
	return out, nil
}

func normalizeContext(context schemagate.IntentContext) (schemagate.IntentContext, error) {
	identity := strings.TrimSpace(context.Identity)
	workspace := strings.TrimSpace(context.Workspace)
	riskClass := strings.ToLower(strings.TrimSpace(context.RiskClass))
	if identity == "" {
		return schemagate.IntentContext{}, fmt.Errorf("context.identity is required")
	}
	if workspace == "" {
		return schemagate.IntentContext{}, fmt.Errorf("context.workspace is required")
	}
	if riskClass == "" {
		return schemagate.IntentContext{}, fmt.Errorf("context.risk_class is required")
	}
	return schemagate.IntentContext{
		Identity:  identity,
		Workspace: filepath.ToSlash(strings.ReplaceAll(workspace, `\`, "/")),
		RiskClass: riskClass,
		SessionID: strings.TrimSpace(context.SessionID),
		RequestID: strings.TrimSpace(context.RequestID),
	}, nil
}

func normalizeJSONValue(value any) (any, error) {
	switch typed := value.(type) {
	case nil, bool, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return typed, nil
	case string:
		return strings.TrimSpace(typed), nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				return nil, fmt.Errorf("args contains empty key")
			}
			normalizedValue, err := normalizeJSONValue(nested)
			if err != nil {
				return nil, err
			}
			out[normalizedKey] = normalizedValue
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for index, nested := range typed {
			normalizedValue, err := normalizeJSONValue(nested)
			if err != nil {
				return nil, err
			}
			out[index] = normalizedValue
		}
		return out, nil
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("marshal json value: %w", err)
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("decode json value: %w", err)
		}
		return normalizeJSONValue(decoded)
	}
}

func digestArgs(args map[string]any) (string, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal normalized args: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest args: %w", err)
	}
	return digest, nil
}

func digestNormalizedIntent(intent normalizedIntent) (string, error) {
	raw, err := json.Marshal(intent)
	if err != nil {
		return "", fmt.Errorf("marshal normalized intent: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest normalized intent: %w", err)
	}
	return digest, nil
}
