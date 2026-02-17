package scout

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

func BenchmarkSnapshotTypical(b *testing.B) {
	workDir := b.TempDir()
	mustWriteBenchFile(b, filepath.Join(workDir, "agent.py"), `
from langchain.tools import tool
from agents import function_tool

@tool
def list_orders():
    return "ok"

@function_tool
def delete_user():
    return "ok"
`)
	mustWriteBenchFile(b, filepath.Join(workDir, "mcp.json"), `{"mcpServers":{"db":{"command":"db"},"mail":{"command":"mail"}}}`)

	provider := DefaultProvider{Options: SnapshotOptions{
		ProducerVersion: "0.0.0-bench",
		Now:             time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
	}}
	request := SnapshotRequest{Roots: []string{workDir}}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		snapshot, err := provider.Snapshot(context.Background(), request)
		if err != nil {
			b.Fatalf("snapshot: %v", err)
		}
		if len(snapshot.Items) == 0 {
			b.Fatalf("expected inventory items")
		}
	}
}

func BenchmarkDiffSnapshotsTypical(b *testing.B) {
	left := benchmarkSnapshot("snap_left", map[string]string{
		"list_orders": "agent_a.py",
		"read_user":   "agent_a.py",
	})
	right := benchmarkSnapshot("snap_right", map[string]string{
		"list_orders": "agent_a.py",
		"delete_user": "agent_b.py",
	})

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		diff := DiffSnapshots(left, right)
		if diff.AddedCount == 0 && diff.RemovedCount == 0 && diff.ChangedCount == 0 {
			b.Fatalf("expected non-empty diff")
		}
	}
}

func benchmarkSnapshot(snapshotID string, tools map[string]string) schemascout.InventorySnapshot {
	items := make([]schemascout.InventoryItem, 0, len(tools))
	for name, locator := range tools {
		items = append(items, makeToolItem(locator, name, "framework:langchain"))
	}
	sortInventoryItems(items)
	return schemascout.InventorySnapshot{
		SchemaID:        "gait.scout.inventory_snapshot",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-bench",
		SnapshotID:      snapshotID,
		Workspace:       "/bench/workspace",
		Items:           items,
	}
}

func mustWriteBenchFile(b *testing.B, path string, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("write bench file %s: %v", path, err)
	}
}
