package gate

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/jcs"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
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
	allowedEndpointClasses = map[string]struct{}{
		"fs.read":     {},
		"fs.write":    {},
		"fs.delete":   {},
		"proc.exec":   {},
		"net.http":    {},
		"net.dns":     {},
		"ui.click":    {},
		"ui.type":     {},
		"ui.navigate": {},
		"other":       {},
	}
	hexDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type normalizedIntent struct {
	ToolName        string                           `json:"tool_name"`
	Args            map[string]any                   `json:"args"`
	Targets         []schemagate.IntentTarget        `json:"targets"`
	ArgProvenance   []schemagate.IntentArgProvenance `json:"arg_provenance,omitempty"`
	SkillProvenance *schemagate.SkillProvenance      `json:"skill_provenance,omitempty"`
	Delegation      *schemagate.IntentDelegation     `json:"delegation,omitempty"`
	Context         schemagate.IntentContext         `json:"context"`
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
	output.SkillProvenance = normalized.SkillProvenance
	output.Delegation = normalized.Delegation
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
	toolName := strings.ToLower(strings.TrimSpace(input.ToolName))
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

	targets, err := normalizeTargets(toolName, input.Targets)
	if err != nil {
		return normalizedIntent{}, err
	}
	provenance, err := normalizeArgProvenance(input.ArgProvenance)
	if err != nil {
		return normalizedIntent{}, err
	}
	skillProvenance, err := normalizeSkillProvenance(input.SkillProvenance)
	if err != nil {
		return normalizedIntent{}, err
	}
	context, err := normalizeContext(input.Context)
	if err != nil {
		return normalizedIntent{}, err
	}
	delegation, err := normalizeDelegation(input.Delegation)
	if err != nil {
		return normalizedIntent{}, err
	}

	return normalizedIntent{
		ToolName:        toolName,
		Args:            args,
		Targets:         targets,
		ArgProvenance:   provenance,
		SkillProvenance: skillProvenance,
		Delegation:      delegation,
		Context:         context,
	}, nil
}

func normalizeTargets(toolName string, targets []schemagate.IntentTarget) ([]schemagate.IntentTarget, error) {
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
		endpointClass := strings.ToLower(strings.TrimSpace(target.EndpointClass))
		endpointDomain := strings.ToLower(strings.TrimSpace(target.EndpointDomain))

		if kind == "" || value == "" {
			return nil, fmt.Errorf("targets require kind and value")
		}
		if _, ok := allowedTargetKinds[kind]; !ok {
			return nil, fmt.Errorf("unsupported target kind: %s", kind)
		}
		if endpointClass == "" {
			endpointClass = inferEndpointClass(kind, operation, toolName)
		}
		if _, ok := allowedEndpointClasses[endpointClass]; !ok {
			return nil, fmt.Errorf("unsupported endpoint class: %s", endpointClass)
		}
		if endpointDomain == "" {
			endpointDomain = inferEndpointDomain(kind, value)
		}
		destructive := target.Destructive || inferDestructive(endpointClass, operation)

		normalized := schemagate.IntentTarget{
			Kind:           kind,
			Value:          value,
			Operation:      operation,
			Sensitivity:    sensitivity,
			EndpointClass:  endpointClass,
			EndpointDomain: endpointDomain,
			Destructive:    destructive,
		}
		destructiveKey := "0"
		if destructive {
			destructiveKey = "1"
		}
		key := strings.Join([]string{
			normalized.Kind,
			normalized.Value,
			normalized.Operation,
			normalized.Sensitivity,
			normalized.EndpointClass,
			normalized.EndpointDomain,
			destructiveKey,
		}, "\x00")
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
		if out[i].Sensitivity != out[j].Sensitivity {
			return out[i].Sensitivity < out[j].Sensitivity
		}
		if out[i].EndpointClass != out[j].EndpointClass {
			return out[i].EndpointClass < out[j].EndpointClass
		}
		if out[i].EndpointDomain != out[j].EndpointDomain {
			return out[i].EndpointDomain < out[j].EndpointDomain
		}
		if out[i].Destructive != out[j].Destructive {
			return !out[i].Destructive && out[j].Destructive
		}
		return false
	})
	return out, nil
}

func inferEndpointClass(kind string, operation string, toolName string) string {
	switch kind {
	case "path":
		switch operation {
		case "read", "get", "list", "open", "cat", "head", "download", "stat":
			return "fs.read"
		case "write", "append", "put", "create", "update", "save", "copy", "move", "rename":
			return "fs.write"
		case "delete", "remove", "rm", "unlink", "rmdir", "drop", "truncate":
			return "fs.delete"
		}
		if hasToolHint(toolName, "read", "list", "fetch", "search") {
			return "fs.read"
		}
		if hasToolHint(toolName, "write", "update", "save", "create") {
			return "fs.write"
		}
		if hasToolHint(toolName, "delete", "remove", "rm", "drop") {
			return "fs.delete"
		}
		return "other"
	case "host", "url":
		switch operation {
		case "dns", "resolve", "lookup":
			return "net.dns"
		default:
			return "net.http"
		}
	case "other":
		switch operation {
		case "exec", "spawn", "run", "shell", "command":
			return "proc.exec"
		}
		if hasToolHint(toolName, "exec", "shell", "command", "spawn", "run") {
			return "proc.exec"
		}
		return "other"
	default:
		return "other"
	}
}

func inferEndpointDomain(kind string, value string) string {
	switch kind {
	case "host":
		host := strings.TrimSpace(value)
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimPrefix(host, "https://")
		if index := strings.Index(host, "/"); index >= 0 {
			host = host[:index]
		}
		if index := strings.Index(host, ":"); index >= 0 {
			host = host[:index]
		}
		return strings.ToLower(strings.TrimSpace(host))
	case "url":
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	default:
		return ""
	}
}

func inferDestructive(endpointClass string, operation string) bool {
	switch endpointClass {
	case "fs.delete", "proc.exec":
		return true
	}
	switch operation {
	case "delete", "remove", "rm", "unlink", "rmdir", "drop", "truncate", "exec", "spawn":
		return true
	default:
		return false
	}
}

func hasToolHint(toolName string, hints ...string) bool {
	normalizedName := path.Base(strings.ToLower(strings.TrimSpace(toolName)))
	for _, hint := range hints {
		if strings.Contains(normalizedName, hint) {
			return true
		}
	}
	return false
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

func normalizeSkillProvenance(input *schemagate.SkillProvenance) (*schemagate.SkillProvenance, error) {
	if input == nil {
		return nil, nil
	}
	skillName := strings.TrimSpace(input.SkillName)
	source := strings.ToLower(strings.TrimSpace(input.Source))
	publisher := strings.TrimSpace(input.Publisher)
	if skillName == "" || source == "" || publisher == "" {
		return nil, fmt.Errorf("skill provenance requires skill_name, source, and publisher")
	}
	digest := strings.ToLower(strings.TrimSpace(input.Digest))
	if digest != "" && !hexDigestPattern.MatchString(digest) {
		return nil, fmt.Errorf("invalid skill provenance digest: %s", digest)
	}
	output := &schemagate.SkillProvenance{
		SkillName:      skillName,
		SkillVersion:   strings.TrimSpace(input.SkillVersion),
		Source:         source,
		Publisher:      publisher,
		Digest:         digest,
		SignatureKeyID: strings.TrimSpace(input.SignatureKeyID),
	}
	return output, nil
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

	authContext, err := normalizeContextAuth(context.AuthContext)
	if err != nil {
		return schemagate.IntentContext{}, err
	}
	credentialScopes := normalizeCredentialScopes(context.CredentialScopes)
	environmentFingerprint := strings.TrimSpace(context.EnvironmentFingerprint)
	contextSetDigest := strings.ToLower(strings.TrimSpace(context.ContextSetDigest))
	if contextSetDigest != "" && !hexDigestPattern.MatchString(contextSetDigest) {
		return schemagate.IntentContext{}, fmt.Errorf("context.context_set_digest must be sha256 hex")
	}
	contextEvidenceMode := strings.ToLower(strings.TrimSpace(context.ContextEvidenceMode))
	if contextEvidenceMode != "" && contextEvidenceMode != "best_effort" && contextEvidenceMode != "required" {
		return schemagate.IntentContext{}, fmt.Errorf("context.context_evidence_mode must be best_effort or required")
	}
	contextRefs := normalizeContextRefs(context.ContextRefs)

	return schemagate.IntentContext{
		Identity:               identity,
		Workspace:              filepath.ToSlash(strings.ReplaceAll(workspace, `\`, "/")),
		RiskClass:              riskClass,
		SessionID:              strings.TrimSpace(context.SessionID),
		RequestID:              strings.TrimSpace(context.RequestID),
		AuthContext:            authContext,
		CredentialScopes:       credentialScopes,
		EnvironmentFingerprint: environmentFingerprint,
		ContextSetDigest:       contextSetDigest,
		ContextEvidenceMode:    contextEvidenceMode,
		ContextRefs:            contextRefs,
	}, nil
}

func normalizeContextAuth(authContext map[string]any) (map[string]any, error) {
	if len(authContext) == 0 {
		return nil, nil
	}
	normalizedValue, err := normalizeJSONValue(authContext)
	if err != nil {
		return nil, fmt.Errorf("normalize context.auth_context: %w", err)
	}
	normalizedMap, ok := normalizedValue.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("context.auth_context must be a JSON object")
	}
	if len(normalizedMap) == 0 {
		return nil, nil
	}
	return normalizedMap, nil
}

func normalizeCredentialScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeContextRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(refs))
	normalized := make([]string, 0, len(refs))
	for _, raw := range refs {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeDelegation(input *schemagate.IntentDelegation) (*schemagate.IntentDelegation, error) {
	if input == nil {
		return nil, nil
	}
	requesterIdentity := strings.TrimSpace(input.RequesterIdentity)
	if requesterIdentity == "" {
		return nil, fmt.Errorf("delegation.requester_identity is required")
	}
	scopeClass := strings.ToLower(strings.TrimSpace(input.ScopeClass))
	tokenRefs := normalizeDelegationTokenRefs(input.TokenRefs)

	normalizedChain := make([]schemagate.DelegationLink, 0, len(input.Chain))
	for i, link := range input.Chain {
		delegatorIdentity := strings.TrimSpace(link.DelegatorIdentity)
		delegateIdentity := strings.TrimSpace(link.DelegateIdentity)
		if delegatorIdentity == "" || delegateIdentity == "" {
			return nil, fmt.Errorf("delegation.chain[%d] requires delegator_identity and delegate_identity", i)
		}
		issuedAt := link.IssuedAt.UTC()
		expiresAt := link.ExpiresAt.UTC()
		if !issuedAt.IsZero() && !expiresAt.IsZero() && !expiresAt.After(issuedAt) {
			return nil, fmt.Errorf("delegation.chain[%d] expires_at must be after issued_at", i)
		}
		normalizedChain = append(normalizedChain, schemagate.DelegationLink{
			DelegatorIdentity: delegatorIdentity,
			DelegateIdentity:  delegateIdentity,
			ScopeClass:        strings.ToLower(strings.TrimSpace(link.ScopeClass)),
			TokenRef:          strings.TrimSpace(link.TokenRef),
			IssuedAt:          issuedAt,
			ExpiresAt:         expiresAt,
		})
	}

	issuedAt := input.IssuedAt.UTC()
	expiresAt := input.ExpiresAt.UTC()
	if !issuedAt.IsZero() && !expiresAt.IsZero() && !expiresAt.After(issuedAt) {
		return nil, fmt.Errorf("delegation.expires_at must be after issued_at")
	}

	return &schemagate.IntentDelegation{
		RequesterIdentity: requesterIdentity,
		ScopeClass:        scopeClass,
		TokenRefs:         tokenRefs,
		Chain:             normalizedChain,
		IssuedAt:          issuedAt,
		ExpiresAt:         expiresAt,
	}, nil
}

func normalizeDelegationTokenRefs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
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
