package scout

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

func TestSnapshotCoverageAndDiff(t *testing.T) {
	workDir := t.TempDir()
	pythonPath := filepath.Join(workDir, "agent.py")
	pythonSource := `
from langchain.tools import tool
from agents import function_tool

@tool
def delete_user():
    return "ok"

@function_tool
def list_orders():
    return "ok"
`
	if err := os.WriteFile(pythonPath, []byte(pythonSource), 0o600); err != nil {
		t.Fatalf("write python source: %v", err)
	}
	mcpPath := filepath.Join(workDir, "mcp.json")
	mcpSource := `{"mcpServers":{"database":{"command":"db"},"email":{"command":"mail"}}}`
	if err := os.WriteFile(mcpPath, []byte(mcpSource), 0o600); err != nil {
		t.Fatalf("write mcp config: %v", err)
	}

	provider := DefaultProvider{}
	snapshot, err := provider.Snapshot(context.Background(), SnapshotRequest{Roots: []string{workDir}})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.SchemaID != "gait.scout.inventory_snapshot" {
		t.Fatalf("unexpected schema id: %s", snapshot.SchemaID)
	}
	if len(snapshot.Items) < 4 {
		t.Fatalf("expected at least 4 inventory items, got %d", len(snapshot.Items))
	}

	policyPath := filepath.Join(workDir, "policy.yaml")
	policySource := `default_verdict: block
rules:
  - name: allow-delete
    priority: 1
    effect: allow
    match:
      tool_names: [delete_user]
`
	if err := os.WriteFile(policyPath, []byte(policySource), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	coverage, err := BuildCoverage(snapshot, []string{policyPath})
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if coverage.DiscoveredTools < 3 {
		t.Fatalf("expected discovered tools >= 3 got %d", coverage.DiscoveredTools)
	}
	if coverage.GatedTools != 1 {
		t.Fatalf("expected gated tools 1 got %d", coverage.GatedTools)
	}

	left := snapshot
	right := snapshot
	right.SnapshotID = "snap_other"
	if len(right.Items) == 0 {
		t.Fatalf("expected inventory items")
	}
	right.Items = append(right.Items[:0:0], right.Items...)
	right.Items[0].Locator = right.Items[0].Locator + ":99"
	diff := DiffSnapshots(left, right)
	if diff.ChangedCount != 1 {
		t.Fatalf("expected 1 changed item, got %d", diff.ChangedCount)
	}
}

func TestSnapshotHelpersAndErrorBranches(t *testing.T) {
	if got := normalizeRoots(nil); len(got) != 1 || got[0] != "." {
		t.Fatalf("normalizeRoots default: %#v", got)
	}
	if got := normalizeRoots([]string{" ./b ", "", "./a", "./a"}); strings.Join(got, ",") != "a,b" {
		t.Fatalf("normalizeRoots dedupe/sort: %#v", got)
	}
	if got := normalizePatterns([]string{"", "  *.py", "a"}); strings.Join(got, ",") != "*.py,a" {
		t.Fatalf("normalizePatterns: %#v", got)
	}
	if !matchesAnyPattern("src/tool.py", []string{"src/*.py"}) {
		t.Fatalf("expected glob pattern to match")
	}
	if !matchesAnyPattern("src/tool.py", []string{"src/"}) {
		t.Fatalf("expected contains pattern to match")
	}
	if matchesAnyPattern("src/tool.py", []string{"*.go"}) {
		t.Fatalf("unexpected pattern match")
	}
	if classifyRisk("delete_user") != "critical" {
		t.Fatalf("expected critical risk")
	}
	if classifyRisk("write_user") != "high" {
		t.Fatalf("expected high risk")
	}
	if classifyRisk("read_user") != "low" {
		t.Fatalf("expected low risk")
	}
	if riskScore("critical") <= riskScore("high") {
		t.Fatalf("expected critical > high risk score")
	}
	if normalizeIdentifier("!!!") == "" {
		t.Fatalf("normalizeIdentifier fallback should not be empty")
	}

	merged := mergeTags([]string{"a", "b"}, []string{"b", "c"})
	if strings.Join(merged, ",") != "a,b,c" {
		t.Fatalf("mergeTags mismatch: %#v", merged)
	}

	items := map[string]schemascout.InventoryItem{
		"tool:one": {
			ID:        "tool:one",
			Kind:      "tool",
			Name:      "one",
			Locator:   "z.py",
			RiskLevel: "low",
			Tags:      []string{"x"},
		},
	}
	addItems(items, []schemascout.InventoryItem{{
		ID:        "tool:one",
		Kind:      "tool",
		Name:      "one",
		Locator:   "a.py",
		RiskLevel: "critical",
		Tags:      []string{"y"},
	}})
	item := items["tool:one"]
	if item.Locator != "a.py" || item.RiskLevel != "critical" || strings.Join(item.Tags, ",") != "x,y" {
		t.Fatalf("addItems merge mismatch: %#v", item)
	}

	deduped := dedupeItems([]schemascout.InventoryItem{
		{ID: "a", Kind: "tool", Name: "A", Locator: "b.py", RiskLevel: "low", Tags: []string{"x"}},
		{ID: "a", Kind: "tool", Name: "A", Locator: "a.py", RiskLevel: "high", Tags: []string{"y"}},
		{ID: "b", Kind: "tool", Name: "B", Locator: "c.py", RiskLevel: "low"},
	})
	if len(deduped) != 2 {
		t.Fatalf("dedupeItems expected 2 got %d", len(deduped))
	}
	if deduped[0].ID != "a" || deduped[0].Locator != "a.py" {
		t.Fatalf("dedupeItems ordering/merge mismatch: %#v", deduped)
	}

	_, err := computeSnapshotID("ws", deduped)
	if err != nil {
		t.Fatalf("computeSnapshotID: %v", err)
	}

	workDir := t.TempDir()
	notDir := filepath.Join(workDir, "file.txt")
	if err := os.WriteFile(notDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := walkRoot(notDir, nil, nil, map[string]schemascout.InventoryItem{}); err == nil {
		t.Fatalf("walkRoot expected non-directory error")
	}
}

func TestPythonAndMCPParsers(t *testing.T) {
	workDir := t.TempDir()
	pythonFallbackPath := filepath.Join(workDir, "simple.py")
	if err := os.WriteFile(pythonFallbackPath, []byte("print('hello')\n"), 0o600); err != nil {
		t.Fatalf("write fallback python file: %v", err)
	}
	items, err := scanPythonFile(pythonFallbackPath)
	if err != nil {
		t.Fatalf("scanPythonFile fallback: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no tool items for plain python file, got %d", len(items))
	}

	invalidJSON := filepath.Join(workDir, "config.json")
	if err := os.WriteFile(invalidJSON, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}
	mcpItems, err := scanMCPConfig(invalidJSON)
	if err != nil {
		t.Fatalf("scanMCPConfig non-mcp invalid should ignore parse errors: %v", err)
	}
	if len(mcpItems) != 0 {
		t.Fatalf("expected no mcp items, got %d", len(mcpItems))
	}

	invalidMCP := filepath.Join(workDir, "mcp_invalid.json")
	if err := os.WriteFile(invalidMCP, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid mcp json: %v", err)
	}
	if _, err := scanMCPConfig(invalidMCP); err == nil {
		t.Fatalf("expected parse error for invalid mcp config")
	}

	anyMapPayload := map[any]any{
		"mcpServers": map[string]any{
			"database": map[string]any{"command": "db"},
		},
		"nested": []any{
			map[string]any{"servers": map[string]any{"queue": map[string]any{"command": "q"}}},
		},
	}
	names := map[string]struct{}{}
	collectMCPServerNames(anyMapPayload, names)
	if len(names) < 2 {
		t.Fatalf("expected collected server names, got %d", len(names))
	}
}

func TestSnapshotDiffAddedAndRemoved(t *testing.T) {
	left := schemascout.InventorySnapshot{
		SnapshotID: "snap_left",
		Items: []schemascout.InventoryItem{
			{ID: "tool:one", Kind: "tool", Name: "one", Locator: "one.py"},
			{ID: "tool:two", Kind: "tool", Name: "two", Locator: "two.py"},
		},
	}
	right := schemascout.InventorySnapshot{
		SnapshotID: "snap_right",
		Items: []schemascout.InventoryItem{
			{ID: "tool:two", Kind: "tool", Name: "two", Locator: "two.py"},
			{ID: "tool:three", Kind: "tool", Name: "three", Locator: "three.py"},
		},
	}
	diff := DiffSnapshots(left, right)
	if diff.AddedCount != 1 || diff.RemovedCount != 1 {
		t.Fatalf("expected added=1 removed=1 got added=%d removed=%d", diff.AddedCount, diff.RemovedCount)
	}
	if diff.Added[0].ID != "tool:three" || diff.Removed[0].ID != "tool:one" {
		t.Fatalf("unexpected diff added/removed entries: %#v", diff)
	}
}
