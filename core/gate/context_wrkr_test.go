package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestLoadWrkrInventoryParsesAndNormalizes(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "wrkr_inventory.json")
	mustWriteWrkrInventoryFile(t, path, `{
  "items": [
    {
      "tool_name": " TOOL.Read ",
      "data_class": " PII ",
      "endpoint_class": " FS.Read ",
      "autonomy_level": " Assist "
    },
    {
      "tool_name": "tool.write",
      "data_class": "secret",
      "endpoint_class": "fs.write",
      "autonomy_level": "auto"
    }
  ]
}`)

	inventory, err := LoadWrkrInventory(path)
	if err != nil {
		t.Fatalf("LoadWrkrInventory returned error: %v", err)
	}
	if inventory.Path != path {
		t.Fatalf("unexpected inventory path: %q", inventory.Path)
	}
	readMetadata, ok := inventory.Tools["tool.read"]
	if !ok {
		t.Fatalf("expected normalized tool key tool.read, got %#v", inventory.Tools)
	}
	if readMetadata.DataClass != "pii" || readMetadata.EndpointClass != "fs.read" || readMetadata.AutonomyLevel != "assist" {
		t.Fatalf("unexpected normalized tool.read metadata: %#v", readMetadata)
	}
}

func TestLoadWrkrInventoryRefreshesOnModTimeChange(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "wrkr_inventory.json")
	mustWriteWrkrInventoryFile(t, path, `[{"tool_name":"tool.write","data_class":"pii","endpoint_class":"fs.write","autonomy_level":"assist"}]`)

	initial, err := LoadWrkrInventory(path)
	if err != nil {
		t.Fatalf("initial LoadWrkrInventory returned error: %v", err)
	}
	if initial.Tools["tool.write"].DataClass != "pii" {
		t.Fatalf("unexpected initial data class: %#v", initial.Tools["tool.write"])
	}

	mustWriteWrkrInventoryFile(t, path, `[{"tool_name":"tool.write","data_class":"secret","endpoint_class":"fs.write","autonomy_level":"assist"}]`)
	nextModTime := time.Now().UTC().Add(2 * time.Second)
	if err := os.Chtimes(path, nextModTime, nextModTime); err != nil {
		t.Fatalf("set wrkr inventory modtime: %v", err)
	}

	refreshed, err := LoadWrkrInventory(path)
	if err != nil {
		t.Fatalf("refreshed LoadWrkrInventory returned error: %v", err)
	}
	if refreshed.Tools["tool.write"].DataClass != "secret" {
		t.Fatalf("expected refreshed data class secret, got %#v", refreshed.Tools["tool.write"])
	}
}

func TestLoadWrkrInventoryRejectsInvalidPayload(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "wrkr_inventory.json")
	mustWriteWrkrInventoryFile(t, path, `{not-json}`)

	_, err := LoadWrkrInventory(path)
	if err == nil {
		t.Fatalf("expected invalid wrkr inventory payload to fail")
	}
	if !strings.Contains(err.Error(), "parse wrkr inventory") {
		t.Fatalf("expected parse wrkr inventory error, got: %v", err)
	}
}

func TestLoadWrkrInventoryAcceptsEmptyEnvelope(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "wrkr_inventory.json")
	mustWriteWrkrInventoryFile(t, path, `{"items":[]}`)

	inventory, err := LoadWrkrInventory(path)
	if err != nil {
		t.Fatalf("LoadWrkrInventory returned error for empty envelope: %v", err)
	}
	if len(inventory.Tools) != 0 {
		t.Fatalf("expected no tools from empty envelope, got %#v", inventory.Tools)
	}
}

func TestApplyWrkrContextAddsMetadata(t *testing.T) {
	intent := schemagate.IntentRequest{
		Context: schemagate.IntentContext{},
	}
	applied := ApplyWrkrContext(&intent, "tool.write", map[string]WrkrToolMetadata{
		"tool.write": {
			ToolName:      "tool.write",
			DataClass:     "secret",
			EndpointClass: "fs.write",
			AutonomyLevel: "assist",
		},
	})
	if !applied {
		t.Fatalf("expected wrkr context to be applied")
	}
	if intent.Context.AuthContext == nil {
		t.Fatalf("expected auth_context to be initialized")
	}
	if intent.Context.AuthContext[wrkrContextToolNameKey] != "tool.write" {
		t.Fatalf("unexpected wrkr.tool_name value: %#v", intent.Context.AuthContext)
	}
	if intent.Context.AuthContext[wrkrContextDataClassKey] != "secret" {
		t.Fatalf("unexpected wrkr.data_class value: %#v", intent.Context.AuthContext)
	}
}

func mustWriteWrkrInventoryFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write wrkr inventory file: %v", err)
	}
}
