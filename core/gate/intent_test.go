package gate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
)

func TestIntentDigestEquivalentFixtures(t *testing.T) {
	left := mustReadIntentFixture(t, "intent_equivalent_a.json")
	right := mustReadIntentFixture(t, "intent_equivalent_b.json")

	leftDigest, err := IntentDigest(left)
	if err != nil {
		t.Fatalf("digest left intent: %v", err)
	}
	rightDigest, err := IntentDigest(right)
	if err != nil {
		t.Fatalf("digest right intent: %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("expected equal digests for equivalent intents: left=%s right=%s", leftDigest, rightDigest)
	}

	leftBytes, err := NormalizedIntentBytes(left)
	if err != nil {
		t.Fatalf("normalize left intent: %v", err)
	}
	rightBytes, err := NormalizedIntentBytes(right)
	if err != nil {
		t.Fatalf("normalize right intent: %v", err)
	}
	if !bytes.Equal(leftBytes, rightBytes) {
		t.Fatalf("expected identical normalized intent bytes")
	}
}

func TestIntentDigestDifferentIntent(t *testing.T) {
	left := mustReadIntentFixture(t, "intent_equivalent_a.json")
	right := mustReadIntentFixture(t, "intent_different.json")

	leftDigest, err := IntentDigest(left)
	if err != nil {
		t.Fatalf("digest left intent: %v", err)
	}
	rightDigest, err := IntentDigest(right)
	if err != nil {
		t.Fatalf("digest right intent: %v", err)
	}
	if leftDigest == rightDigest {
		t.Fatalf("expected distinct digests for non-equivalent intents")
	}
}

func TestNormalizeIntentPopulatesDigestsAndDefaults(t *testing.T) {
	intent := schemagate.IntentRequest{
		ToolName: " tool.write ",
		Args: map[string]any{
			" path ": " /tmp/out.txt ",
			"options": map[string]any{
				" mode ": " append ",
			},
		},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt"},
			{Kind: " path ", Value: " /tmp/out.txt "},
			{Kind: "host", Value: " api.internal "},
		},
		ArgProvenance: []schemagate.IntentArgProvenance{
			{ArgPath: "args.path", Source: " user "},
			{ArgPath: "args.path", Source: "user"},
			{ArgPath: "args.options", Source: "external", IntegrityDigest: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		},
		Context: schemagate.IntentContext{
			Identity:               " alice ",
			Workspace:              `C:\repo\gait`,
			RiskClass:              " HIGH ",
			SessionID:              " s1 ",
			RequestID:              " req-1 ",
			EnvironmentFingerprint: " env:prod-us-east-1 ",
			CredentialScopes: []string{
				" tools.read ",
				"tools.write",
				"tools.read",
			},
			AuthContext: map[string]any{
				"provider": " oidc ",
				"claims": map[string]any{
					"tenant": " acme ",
				},
			},
		},
	}

	normalized, err := NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize intent: %v", err)
	}

	if normalized.SchemaID != intentRequestSchemaID {
		t.Fatalf("unexpected schema_id: %s", normalized.SchemaID)
	}
	if normalized.SchemaVersion != intentRequestSchemaV1 {
		t.Fatalf("unexpected schema_version: %s", normalized.SchemaVersion)
	}
	if len(normalized.ArgsDigest) != 64 || len(normalized.IntentDigest) != 64 {
		t.Fatalf("expected 64-char digests, got args=%q intent=%q", normalized.ArgsDigest, normalized.IntentDigest)
	}
	if normalized.ToolName != "tool.write" {
		t.Fatalf("unexpected normalized tool name: %s", normalized.ToolName)
	}
	if normalized.Context.Identity != "alice" || normalized.Context.Workspace != "C:/repo/gait" || normalized.Context.RiskClass != "high" {
		t.Fatalf("unexpected normalized context: %#v", normalized.Context)
	}
	if normalized.Context.SessionID != "s1" || normalized.Context.RequestID != "req-1" {
		t.Fatalf("unexpected normalized context ids: %#v", normalized.Context)
	}
	if normalized.Context.EnvironmentFingerprint != "env:prod-us-east-1" {
		t.Fatalf("unexpected normalized environment fingerprint: %#v", normalized.Context)
	}
	if len(normalized.Context.CredentialScopes) != 2 || normalized.Context.CredentialScopes[0] != "tools.read" || normalized.Context.CredentialScopes[1] != "tools.write" {
		t.Fatalf("unexpected normalized credential scopes: %#v", normalized.Context.CredentialScopes)
	}
	if normalized.Context.AuthContext == nil {
		t.Fatalf("expected normalized auth_context")
	}
	if normalized.Context.AuthContext["provider"] != "oidc" {
		t.Fatalf("unexpected normalized auth_context provider: %#v", normalized.Context.AuthContext)
	}
	claims, ok := normalized.Context.AuthContext["claims"].(map[string]any)
	if !ok || claims["tenant"] != "acme" {
		t.Fatalf("unexpected normalized auth_context claims: %#v", normalized.Context.AuthContext["claims"])
	}
	if len(normalized.Targets) != 2 {
		t.Fatalf("expected de-duplicated targets, got %d", len(normalized.Targets))
	}
	if normalized.Targets[0].Kind != "host" || normalized.Targets[1].Kind != "path" {
		t.Fatalf("expected sorted targets, got %#v", normalized.Targets)
	}
	if normalized.Targets[0].EndpointClass != "net.http" || normalized.Targets[0].EndpointDomain != "api.internal" {
		t.Fatalf("expected inferred host endpoint metadata, got %#v", normalized.Targets[0])
	}
	if normalized.Targets[1].EndpointClass != "fs.write" {
		t.Fatalf("expected inferred path endpoint class from tool hint, got %#v", normalized.Targets[1])
	}
	if len(normalized.ArgProvenance) != 2 {
		t.Fatalf("expected de-duplicated provenance entries, got %d", len(normalized.ArgProvenance))
	}
	if normalized.ArgProvenance[0].IntegrityDigest != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("expected lowercased integrity digest, got %#v", normalized.ArgProvenance[0])
	}
}

func TestNormalizeIntentEndpointClassification(t *testing.T) {
	intent := schemagate.IntentRequest{
		ToolName: "tool.exec",
		Args:     map[string]any{"command": "rm -rf /tmp/demo"},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/in.txt", Operation: "read"},
			{Kind: "path", Value: "/tmp/out.txt", Operation: "write"},
			{Kind: "path", Value: "/tmp/out.txt", Operation: "delete"},
			{Kind: "host", Value: "api.internal", Operation: "dns"},
			{Kind: "url", Value: "https://example.com/path"},
			{Kind: "other", Value: "shell", Operation: "exec"},
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/tmp/work",
			RiskClass: "high",
		},
	}

	normalized, err := NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize intent: %v", err)
	}
	gotClasses := map[string]bool{}
	destructiveCount := 0
	for _, target := range normalized.Targets {
		gotClasses[target.EndpointClass] = true
		if target.Destructive {
			destructiveCount++
		}
	}
	for _, expected := range []string{"fs.read", "fs.write", "fs.delete", "net.dns", "net.http", "proc.exec"} {
		if !gotClasses[expected] {
			t.Fatalf("missing inferred endpoint class %s in %#v", expected, normalized.Targets)
		}
	}
	if destructiveCount == 0 {
		t.Fatalf("expected at least one destructive endpoint target")
	}
}

func TestNormalizeIntentSkillProvenance(t *testing.T) {
	intent := schemagate.IntentRequest{
		ToolName: "tool.write",
		Args:     map[string]any{"path": "/tmp/out.txt"},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt", Operation: "write"},
		},
		SkillProvenance: &schemagate.SkillProvenance{
			SkillName:      "safe-curl",
			SkillVersion:   "1.0.1",
			Source:         " Registry ",
			Publisher:      " Acme ",
			Digest:         strings.Repeat("A", 64),
			SignatureKeyID: " key-1 ",
		},
		Context: schemagate.IntentContext{
			Identity:  "alice",
			Workspace: "/tmp/work",
			RiskClass: "high",
		},
	}

	normalized, err := NormalizeIntent(intent)
	if err != nil {
		t.Fatalf("normalize intent: %v", err)
	}
	if normalized.SkillProvenance == nil {
		t.Fatalf("expected skill provenance in normalized intent")
	}
	if normalized.SkillProvenance.Source != "registry" || normalized.SkillProvenance.Publisher != "Acme" {
		t.Fatalf("unexpected normalized skill provenance %#v", normalized.SkillProvenance)
	}
	if normalized.SkillProvenance.Digest != strings.Repeat("a", 64) {
		t.Fatalf("expected lowercased skill digest, got %q", normalized.SkillProvenance.Digest)
	}
}

func TestNormalizeIntentDelegationDigesting(t *testing.T) {
	base := schemagate.IntentRequest{
		ToolName: "tool.write",
		Args:     map[string]any{"path": "/tmp/out.txt"},
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/out.txt", Operation: "write"},
		},
		Delegation: &schemagate.IntentDelegation{
			RequesterIdentity: " agent.specialist ",
			ScopeClass:        " WRITE ",
			TokenRefs:         []string{" token_b ", "token_a", "token_a"},
			Chain: []schemagate.DelegationLink{
				{DelegatorIdentity: "agent.lead", DelegateIdentity: "agent.specialist", ScopeClass: "write", TokenRef: "token_b"},
			},
		},
		Context: schemagate.IntentContext{
			Identity:  "agent.specialist",
			Workspace: "/repo/gait",
			RiskClass: "high",
			SessionID: "sess-1",
		},
	}
	normalizedA, err := NormalizeIntent(base)
	if err != nil {
		t.Fatalf("normalize intent with delegation: %v", err)
	}
	normalizedB, err := NormalizeIntent(base)
	if err != nil {
		t.Fatalf("normalize equivalent intent with delegation: %v", err)
	}
	if normalizedA.IntentDigest != normalizedB.IntentDigest {
		t.Fatalf("expected equivalent delegation payloads to produce same digest")
	}
	if normalizedA.Delegation == nil {
		t.Fatalf("expected delegation in normalized payload")
	}
	if normalizedA.Delegation.ScopeClass != "write" {
		t.Fatalf("expected lowercased delegation scope class, got %q", normalizedA.Delegation.ScopeClass)
	}
	if len(normalizedA.Delegation.TokenRefs) != 2 || normalizedA.Delegation.TokenRefs[0] != "token_a" || normalizedA.Delegation.TokenRefs[1] != "token_b" {
		t.Fatalf("unexpected normalized delegation token refs: %#v", normalizedA.Delegation.TokenRefs)
	}

	modified := base
	modified.Delegation = &schemagate.IntentDelegation{
		RequesterIdentity: "agent.specialist",
		ScopeClass:        "write",
		TokenRefs:         []string{"token_a"},
		Chain: []schemagate.DelegationLink{
			{DelegatorIdentity: "agent.root", DelegateIdentity: "agent.specialist", ScopeClass: "write", TokenRef: "token_a"},
		},
	}
	normalizedModified, err := NormalizeIntent(modified)
	if err != nil {
		t.Fatalf("normalize modified delegation intent: %v", err)
	}
	if normalizedA.IntentDigest == normalizedModified.IntentDigest {
		t.Fatalf("expected delegation change to alter intent digest")
	}
}

func TestNormalizeIntentValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		intent schemagate.IntentRequest
	}{
		{
			name: "missing_tool_name",
			intent: schemagate.IntentRequest{
				ToolName: "",
				Args:     map[string]any{},
				Context:  schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "missing_workspace",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				Context:  schemagate.IntentContext{Identity: "u", RiskClass: "low"},
			},
		},
		{
			name: "invalid_target_kind",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				Targets:  []schemagate.IntentTarget{{Kind: "invalid", Value: "x"}},
				Context:  schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "invalid_provenance_source",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				ArgProvenance: []schemagate.IntentArgProvenance{
					{ArgPath: "args.x", Source: "bad"},
				},
				Context: schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "invalid_provenance_digest",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				ArgProvenance: []schemagate.IntentArgProvenance{
					{ArgPath: "args.x", Source: "external", IntegrityDigest: "short"},
				},
				Context: schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "invalid_skill_provenance_missing_required",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				SkillProvenance: &schemagate.SkillProvenance{
					SkillName: "",
					Source:    "registry",
					Publisher: "acme",
				},
				Context: schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "invalid_skill_provenance_digest",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				SkillProvenance: &schemagate.SkillProvenance{
					SkillName: "safe-curl",
					Source:    "registry",
					Publisher: "acme",
					Digest:    "bad",
				},
				Context: schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
			},
		},
		{
			name: "invalid_auth_context_value",
			intent: schemagate.IntentRequest{
				ToolName: "tool.demo",
				Args:     map[string]any{},
				Context: schemagate.IntentContext{
					Identity:  "u",
					Workspace: "/tmp",
					RiskClass: "low",
					AuthContext: map[string]any{
						"token": func() {},
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NormalizeIntent(testCase.intent); err == nil {
				t.Fatalf("expected normalization error")
			}
		})
	}
}

func TestArgsDigestStableForEquivalentObjects(t *testing.T) {
	first, err := ArgsDigest(map[string]any{
		"a": " hello ",
		"b": map[string]any{
			"z": float64(1),
			"y": []any{" x ", float64(2)},
		},
	})
	if err != nil {
		t.Fatalf("digest first args: %v", err)
	}

	second, err := ArgsDigest(map[string]any{
		"b": map[string]any{
			"y": []any{"x", float64(2)},
			"z": float64(1),
		},
		"a": "hello",
	})
	if err != nil {
		t.Fatalf("digest second args: %v", err)
	}
	if first != second {
		t.Fatalf("expected identical args digests for equivalent objects: first=%s second=%s", first, second)
	}
}

func TestNormalizeIntentErrorPaths(t *testing.T) {
	_, err := NormalizedIntentBytes(schemagate.IntentRequest{
		ToolName: "tool.demo",
		Args:     map[string]any{},
		Context:  schemagate.IntentContext{Identity: "u", RiskClass: "low"},
	})
	if err == nil {
		t.Fatalf("expected normalization error for missing workspace")
	}

	_, err = IntentDigest(schemagate.IntentRequest{
		ToolName: "",
		Args:     map[string]any{},
		Context:  schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
	})
	if err == nil {
		t.Fatalf("expected normalization error for missing tool")
	}
}

func TestArgsDigestErrorPaths(t *testing.T) {
	if _, err := ArgsDigest(map[string]any{"": "x"}); err == nil {
		t.Fatalf("expected args digest to fail for empty key")
	}

	if _, err := ArgsDigest(map[string]any{"value": func() {}}); err == nil {
		t.Fatalf("expected args digest to fail for non-marshalable value")
	}
}

func TestNormalizeJSONValueFallback(t *testing.T) {
	type nested struct {
		Name string `json:"name"`
	}
	type sample struct {
		Record nested `json:"record"`
	}
	normalized, err := normalizeJSONValue(sample{Record: nested{Name: " value "}})
	if err != nil {
		t.Fatalf("normalize json value: %v", err)
	}
	valueMap, ok := normalized.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", normalized)
	}
	recordMap, ok := valueMap["record"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map output, got %#v", valueMap["record"])
	}
	if recordMap["name"] != "value" {
		t.Fatalf("expected trimmed nested value, got %#v", recordMap["name"])
	}
}

func TestDigestHelperErrors(t *testing.T) {
	if _, err := digestArgs(map[string]any{"bad": func() {}}); err == nil {
		t.Fatalf("expected digestArgs marshal error")
	}
	if _, err := digestNormalizedIntent(normalizedIntent{
		ToolName: "tool.demo",
		Args:     map[string]any{"bad": func() {}},
		Context:  schemagate.IntentContext{Identity: "u", Workspace: "/tmp", RiskClass: "low"},
	}); err == nil {
		t.Fatalf("expected digestNormalizedIntent marshal error")
	}
}

func TestNormalizeTargetsAndProvenanceErrors(t *testing.T) {
	if targets, err := normalizeTargets("tool.demo", nil); err != nil || len(targets) != 0 {
		t.Fatalf("expected empty targets, got targets=%#v err=%v", targets, err)
	}
	if _, err := normalizeTargets("tool.demo", []schemagate.IntentTarget{{Kind: "path", Value: ""}}); err == nil {
		t.Fatalf("expected target with empty value to fail")
	}
	if _, err := normalizeTargets("tool.demo", []schemagate.IntentTarget{{Kind: "path", Value: "/tmp/out", EndpointClass: "invalid"}}); err == nil {
		t.Fatalf("expected unsupported endpoint class to fail")
	}

	if provenance, err := normalizeArgProvenance(nil); err != nil || len(provenance) != 0 {
		t.Fatalf("expected empty provenance, got entries=%#v err=%v", provenance, err)
	}
	if _, err := normalizeArgProvenance([]schemagate.IntentArgProvenance{{ArgPath: "", Source: "user"}}); err == nil {
		t.Fatalf("expected provenance with empty arg_path to fail")
	}
	if _, err := normalizeArgProvenance([]schemagate.IntentArgProvenance{{
		ArgPath:         "args.x",
		Source:          "external",
		IntegrityDigest: strings.Repeat("z", 64),
	}}); err == nil {
		t.Fatalf("expected invalid integrity digest to fail")
	}

	if _, err := normalizeContext(schemagate.IntentContext{
		Identity:  "u",
		Workspace: "/tmp",
		RiskClass: "",
	}); err == nil {
		t.Fatalf("expected missing risk class to fail")
	}
}

func TestNormalizeContextRefs(t *testing.T) {
	if refs := normalizeContextRefs(nil); refs != nil {
		t.Fatalf("expected nil refs for empty input, got %#v", refs)
	}
	if refs := normalizeContextRefs([]string{" ", "\t"}); refs != nil {
		t.Fatalf("expected nil refs for whitespace-only input, got %#v", refs)
	}

	refs := normalizeContextRefs([]string{" ref-b ", "ref-a", "ref-b", "", "ref-c"})
	expected := []string{"ref-a", "ref-b", "ref-c"}
	if len(refs) != len(expected) {
		t.Fatalf("unexpected refs length: got=%d want=%d refs=%#v", len(refs), len(expected), refs)
	}
	for index, want := range expected {
		if refs[index] != want {
			t.Fatalf("unexpected refs[%d]: got=%q want=%q refs=%#v", index, refs[index], want, refs)
		}
	}
}

func mustReadIntentFixture(t *testing.T, name string) schemagate.IntentRequest {
	t.Helper()
	path := filepath.Join("testdata", name)
	// #nosec G304 -- test fixture names are hardcoded in tests.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var intent schemagate.IntentRequest
	if err := json.Unmarshal(content, &intent); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return intent
}
