package scenarios

import (
	"fmt"
	"os"
	"path/filepath"
)

const scenarioRootRelativePath = "scenarios/gait"

var requiredScenarioMinimumFiles = map[string][]string{
	"policy-block-destructive":  {"README.md", "policy.yaml", "intents.jsonl", "expected-verdicts.jsonl"},
	"policy-allow-safe-tools":   {"README.md", "policy.yaml", "intents.jsonl", "expected-verdicts.jsonl"},
	"dry-run-no-side-effects":   {"README.md", "policy.yaml", "intent.json", "flags.yaml", "expected.yaml"},
	"concurrent-evaluation-10":  {"README.md", "policy.yaml", "intent.json", "flags.yaml", "expected.yaml"},
	"pack-integrity-round-trip": {"README.md", "expected.yaml"},
	"delegation-chain-depth-3":  {"README.md", "policy.yaml", "intent.json", "flags.yaml", "expected.yaml", "delegation-token-1.json", "delegation-token-2.json", "delegation-token-3.json", "delegation_public.key"},
	"approval-expiry-1s-past":   {"README.md", "policy.yaml", "intent.json", "expected.yaml", "approval-token.json", "approval_public.key"},
	"approval-token-valid":      {"README.md", "policy.yaml", "intent.json", "expected.yaml", "approval-token.json", "approval_public.key"},
}

func findRepoRoot(startDir string) (string, error) {
	current := startDir
	for {
		candidate := filepath.Join(current, "go.mod")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("unable to locate repository root from %s", startDir)
		}
		current = parent
	}
}
