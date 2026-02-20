package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

type WrkrToolMetadata struct {
	ToolName      string
	DataClass     string
	EndpointClass string
	AutonomyLevel string
}

type WrkrInventory struct {
	Path    string
	ModTime time.Time
	Tools   map[string]WrkrToolMetadata
}

type wrkrInventoryCacheEntry struct {
	modTime   time.Time
	inventory WrkrInventory
}

var wrkrInventoryCache = struct {
	sync.Mutex
	entries map[string]wrkrInventoryCacheEntry
}{
	entries: map[string]wrkrInventoryCacheEntry{},
}

func LoadWrkrInventory(path string) (WrkrInventory, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return WrkrInventory{}, fmt.Errorf("wrkr inventory path is required")
	}
	cleanPath := filepath.Clean(trimmed)
	// #nosec G304 -- explicit local path from CLI.
	info, err := os.Stat(cleanPath)
	if err != nil {
		return WrkrInventory{}, fmt.Errorf("stat wrkr inventory: %w", err)
	}
	modTime := info.ModTime().UTC()

	wrkrInventoryCache.Lock()
	if cached, ok := wrkrInventoryCache.entries[cleanPath]; ok && cached.modTime.Equal(modTime) {
		wrkrInventoryCache.Unlock()
		return cached.inventory, nil
	}
	wrkrInventoryCache.Unlock()

	// #nosec G304 -- explicit local path from CLI.
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return WrkrInventory{}, fmt.Errorf("read wrkr inventory: %w", err)
	}
	tools, err := parseWrkrInventory(content)
	if err != nil {
		return WrkrInventory{}, err
	}
	inventory := WrkrInventory{
		Path:    cleanPath,
		ModTime: modTime,
		Tools:   tools,
	}

	wrkrInventoryCache.Lock()
	wrkrInventoryCache.entries[cleanPath] = wrkrInventoryCacheEntry{
		modTime:   modTime,
		inventory: inventory,
	}
	wrkrInventoryCache.Unlock()

	return inventory, nil
}

func parseWrkrInventory(content []byte) (map[string]WrkrToolMetadata, error) {
	type item struct {
		ToolName      string `json:"tool_name"`
		DataClass     string `json:"data_class"`
		EndpointClass string `json:"endpoint_class"`
		AutonomyLevel string `json:"autonomy_level"`
	}
	type envelope struct {
		Items []item `json:"items"`
	}

	entries := []item{}
	var wrappedRaw map[string]json.RawMessage
	if err := json.Unmarshal(content, &wrappedRaw); err == nil {
		if rawItems, ok := wrappedRaw["items"]; ok {
			var wrapped envelope
			if err := json.Unmarshal(rawItems, &wrapped.Items); err != nil {
				return nil, fmt.Errorf("parse wrkr inventory: %w", err)
			}
			entries = wrapped.Items
		} else {
			if err := json.Unmarshal(content, &entries); err != nil {
				return nil, fmt.Errorf("parse wrkr inventory: %w", err)
			}
		}
	} else {
		if err := json.Unmarshal(content, &entries); err != nil {
			return nil, fmt.Errorf("parse wrkr inventory: %w", err)
		}
	}

	tools := map[string]WrkrToolMetadata{}
	for _, entry := range entries {
		toolName := strings.ToLower(strings.TrimSpace(entry.ToolName))
		if toolName == "" {
			continue
		}
		tools[toolName] = WrkrToolMetadata{
			ToolName:      toolName,
			DataClass:     strings.ToLower(strings.TrimSpace(entry.DataClass)),
			EndpointClass: strings.ToLower(strings.TrimSpace(entry.EndpointClass)),
			AutonomyLevel: strings.ToLower(strings.TrimSpace(entry.AutonomyLevel)),
		}
	}
	return tools, nil
}

func ApplyWrkrContext(intent *schemagate.IntentRequest, toolName string, inventory map[string]WrkrToolMetadata) bool {
	if intent == nil || len(inventory) == 0 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(toolName))
	metadata, ok := inventory[key]
	if !ok {
		return false
	}
	if intent.Context.AuthContext == nil {
		intent.Context.AuthContext = map[string]any{}
	}
	intent.Context.AuthContext[wrkrContextToolNameKey] = metadata.ToolName
	intent.Context.AuthContext[wrkrContextDataClassKey] = metadata.DataClass
	intent.Context.AuthContext[wrkrContextEndpointClassKey] = metadata.EndpointClass
	intent.Context.AuthContext[wrkrContextAutonomyLevelKey] = metadata.AutonomyLevel
	return true
}
