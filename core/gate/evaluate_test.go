package gate

import "testing"

func TestEvaluateAliasMatchesEvaluatePolicy(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-tool
    effect: allow
    match:
      tool_names: [tool.demo]
`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	intent := baseIntent()
	intent.ToolName = "tool.demo"

	aliased, err := Evaluate(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate alias: %v", err)
	}
	regular, err := EvaluatePolicy(policy, intent, EvalOptions{ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("evaluate policy: %v", err)
	}

	if aliased.Verdict != regular.Verdict {
		t.Fatalf("evaluate alias mismatch: aliased=%#v regular=%#v", aliased, regular)
	}
}
