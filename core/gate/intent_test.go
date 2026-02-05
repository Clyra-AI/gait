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
			Identity:  " alice ",
			Workspace: `C:\repo\gait`,
			RiskClass: " HIGH ",
			SessionID: " s1 ",
			RequestID: " req-1 ",
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
	if len(normalized.Targets) != 2 {
		t.Fatalf("expected de-duplicated targets, got %d", len(normalized.Targets))
	}
	if normalized.Targets[0].Kind != "host" || normalized.Targets[1].Kind != "path" {
		t.Fatalf("expected sorted targets, got %#v", normalized.Targets)
	}
	if len(normalized.ArgProvenance) != 2 {
		t.Fatalf("expected de-duplicated provenance entries, got %d", len(normalized.ArgProvenance))
	}
	if normalized.ArgProvenance[0].IntegrityDigest != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("expected lowercased integrity digest, got %#v", normalized.ArgProvenance[0])
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
	if targets, err := normalizeTargets(nil); err != nil || len(targets) != 0 {
		t.Fatalf("expected empty targets, got targets=%#v err=%v", targets, err)
	}
	if _, err := normalizeTargets([]schemagate.IntentTarget{{Kind: "path", Value: ""}}); err == nil {
		t.Fatalf("expected target with empty value to fail")
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
