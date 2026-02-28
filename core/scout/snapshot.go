package scout

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
	jcs "github.com/Clyra-AI/proof/canon"
	"github.com/goccy/go-yaml"
)

type SnapshotOptions struct {
	ProducerVersion string
	Now             time.Time
}

type DefaultProvider struct {
	Options SnapshotOptions
}

func (provider DefaultProvider) Snapshot(_ context.Context, req SnapshotRequest) (schemascout.InventorySnapshot, error) {
	roots := normalizeRoots(req.Roots)
	includePatterns := normalizePatterns(req.Include)
	excludePatterns := normalizePatterns(req.Exclude)

	items := make(map[string]schemascout.InventoryItem)
	for _, root := range roots {
		if err := walkRoot(root, includePatterns, excludePatterns, items); err != nil {
			return schemascout.InventorySnapshot{}, err
		}
	}

	snapshotItems := make([]schemascout.InventoryItem, 0, len(items))
	for _, item := range items {
		snapshotItems = append(snapshotItems, item)
	}
	sortInventoryItems(snapshotItems)

	workspace := roots[0]
	if len(roots) > 1 {
		workspace = strings.Join(roots, ",")
	}

	createdAt := provider.Options.Now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(provider.Options.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	snapshotID, err := computeSnapshotID(workspace, snapshotItems)
	if err != nil {
		return schemascout.InventorySnapshot{}, fmt.Errorf("compute snapshot id: %w", err)
	}
	snapshotItems = attachInventoryRelationships(snapshotID, snapshotItems)

	return schemascout.InventorySnapshot{
		SchemaID:        "gait.scout.inventory_snapshot",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: producerVersion,
		SnapshotID:      snapshotID,
		Workspace:       workspace,
		Items:           snapshotItems,
	}, nil
}

func normalizeRoots(roots []string) []string {
	if len(roots) == 0 {
		return []string{"."}
	}
	normalized := make([]string, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		clean := filepath.Clean(trimmed)
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		normalized = append(normalized, clean)
	}
	if len(normalized) == 0 {
		return []string{"."}
	}
	sort.Strings(normalized)
	return normalized
}

func normalizePatterns(patterns []string) []string {
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func walkRoot(
	root string,
	includePatterns []string,
	excludePatterns []string,
	items map[string]schemascout.InventoryItem,
) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat root %s: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root is not a directory: %s", root)
	}

	return filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			name := strings.ToLower(dirEntry.Name())
			if name == ".git" || name == ".beads" || name == "node_modules" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		normalizedPath := filepath.ToSlash(path)
		if matchesAnyPattern(normalizedPath, excludePatterns) {
			return nil
		}
		if len(includePatterns) > 0 && !matchesAnyPattern(normalizedPath, includePatterns) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(normalizedPath))
		switch ext {
		case ".py":
			pythonItems, err := scanPythonFile(normalizedPath)
			if err != nil {
				return err
			}
			addItems(items, pythonItems)
		case ".json", ".yaml", ".yml":
			mcpItems, err := scanMCPConfig(normalizedPath)
			if err != nil {
				return err
			}
			addItems(items, mcpItems)
		}
		return nil
	})
}

func matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func addItems(target map[string]schemascout.InventoryItem, values []schemascout.InventoryItem) {
	for _, value := range values {
		existing, exists := target[value.ID]
		if !exists {
			target[value.ID] = value
			continue
		}
		if value.Locator < existing.Locator {
			existing.Locator = value.Locator
		}
		if riskScore(value.RiskLevel) > riskScore(existing.RiskLevel) {
			existing.RiskLevel = value.RiskLevel
		}
		existing.Tags = mergeTags(existing.Tags, value.Tags)
		target[value.ID] = existing
	}
}

var (
	definitionRegexp       = regexp.MustCompile(`(?m)^\s*def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	langchainToolRegexp    = regexp.MustCompile(`(?m)^\s*@tool(?:\s*\(|\s*$)`)
	openAIFunctionToolExpr = regexp.MustCompile(`(?m)^\s*@function_tool(?:\s*\(|\s*$)`)
	toolNameArgRegexp      = regexp.MustCompile(`(?m)(?:name|tool_name)\s*=\s*['"]([A-Za-z0-9._:-]+)['"]`)
)

func scanPythonFile(path string) ([]schemascout.InventoryItem, error) {
	// #nosec G304 -- scanning user workspace files is intentional.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read python file %s: %w", path, err)
	}
	text := string(content)
	definitions := definitionRegexp.FindAllStringSubmatch(text, -1)
	definitionNames := make([]string, 0, len(definitions))
	for _, match := range definitions {
		if len(match) > 1 {
			definitionNames = append(definitionNames, strings.TrimSpace(match[1]))
		}
	}
	if len(definitionNames) == 0 {
		definitionNames = []string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	}
	sort.Strings(definitionNames)

	items := make([]schemascout.InventoryItem, 0)
	if langchainToolRegexp.Match(content) || strings.Contains(text, "StructuredTool.from_function") || strings.Contains(text, "Tool(") {
		for _, definitionName := range definitionNames {
			items = append(items, makeToolItem(path, definitionName, "framework:langchain"))
		}
	}

	openAIDetected := openAIFunctionToolExpr.Match(content) ||
		strings.Contains(text, "from agents import") ||
		strings.Contains(text, "openai.agents") ||
		strings.Contains(text, "tools=[")
	if openAIDetected {
		names := make([]string, 0, len(definitionNames))
		nameMatches := toolNameArgRegexp.FindAllStringSubmatch(text, -1)
		for _, match := range nameMatches {
			if len(match) > 1 {
				names = append(names, strings.TrimSpace(match[1]))
			}
		}
		names = append(names, definitionNames...)
		names = uniqueSorted(names)
		for _, name := range names {
			items = append(items, makeToolItem(path, name, "framework:openai_agents"))
		}
	}

	return dedupeItems(items), nil
}

func makeToolItem(locator, name, tag string) schemascout.InventoryItem {
	normalizedName := normalizeIdentifier(name)
	return schemascout.InventoryItem{
		ID:        "tool:" + tag + ":" + normalizedName,
		Kind:      "tool",
		Name:      name,
		Locator:   locator,
		RiskLevel: classifyRisk(name),
		Tags:      []string{tag},
	}
}

func scanMCPConfig(path string) ([]schemascout.InventoryItem, error) {
	base := strings.ToLower(filepath.Base(path))
	looksLikeMCP := strings.Contains(base, "mcp")
	// #nosec G304 -- scanning user workspace files is intentional.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var payload any
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &payload); err != nil {
			if looksLikeMCP {
				return nil, fmt.Errorf("parse mcp yaml %s: %w", path, err)
			}
			return nil, nil
		}
	default:
		if err := json.Unmarshal(content, &payload); err != nil {
			if looksLikeMCP {
				return nil, fmt.Errorf("parse mcp json %s: %w", path, err)
			}
			return nil, nil
		}
	}

	serverNames := map[string]struct{}{}
	collectMCPServerNames(payload, serverNames)
	if !looksLikeMCP && len(serverNames) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(serverNames))
	for name := range serverNames {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]schemascout.InventoryItem, 0, len(names))
	for _, name := range names {
		normalizedName := normalizeIdentifier(name)
		items = append(items, schemascout.InventoryItem{
			ID:        "mcp_server:" + normalizedName,
			Kind:      "mcp_server",
			Name:      name,
			Locator:   path,
			RiskLevel: classifyRisk(name),
			Tags:      []string{"mcp"},
		})
	}
	return items, nil
}

func collectMCPServerNames(value any, names map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if lowerKey == "mcpservers" || lowerKey == "servers" {
				if asMap, ok := nested.(map[string]any); ok {
					for serverName := range asMap {
						trimmed := strings.TrimSpace(serverName)
						if trimmed != "" {
							names[trimmed] = struct{}{}
						}
					}
				}
			}
			collectMCPServerNames(nested, names)
		}
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[fmt.Sprintf("%v", key)] = nested
		}
		collectMCPServerNames(normalized, names)
	case []any:
		for _, nested := range typed {
			collectMCPServerNames(nested, names)
		}
	}
}

func computeSnapshotID(workspace string, items []schemascout.InventoryItem) (string, error) {
	payload := struct {
		Workspace string                      `json:"workspace"`
		Items     []schemascout.InventoryItem `json:"items"`
	}{
		Workspace: workspace,
		Items:     items,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest, err := jcs.DigestJCS(encoded)
	if err != nil {
		return "", err
	}
	return "snap_" + digest[:12], nil
}

func classifyRisk(name string) string {
	lower := strings.ToLower(name)
	criticalTokens := []string{"delete", "drop", "destroy", "wire", "payment", "transfer"}
	for _, token := range criticalTokens {
		if strings.Contains(lower, token) {
			return "critical"
		}
	}
	highTokens := []string{"write", "update", "export", "send", "email", "remove", "overwrite"}
	for _, token := range highTokens {
		if strings.Contains(lower, token) {
			return "high"
		}
	}
	return "low"
}

func riskScore(level string) int {
	switch strings.ToLower(level) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func mergeTags(left []string, right []string) []string {
	merged := append(append([]string{}, left...), right...)
	return uniqueSorted(merged)
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
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
	sort.Strings(out)
	return out
}

func dedupeItems(items []schemascout.InventoryItem) []schemascout.InventoryItem {
	if len(items) == 0 {
		return nil
	}
	index := map[string]schemascout.InventoryItem{}
	for _, item := range items {
		existing, exists := index[item.ID]
		if !exists {
			index[item.ID] = item
			continue
		}
		if item.Locator < existing.Locator {
			existing.Locator = item.Locator
		}
		existing.Tags = mergeTags(existing.Tags, item.Tags)
		if riskScore(item.RiskLevel) > riskScore(existing.RiskLevel) {
			existing.RiskLevel = item.RiskLevel
		}
		index[item.ID] = existing
	}
	out := make([]schemascout.InventoryItem, 0, len(index))
	for _, item := range index {
		out = append(out, item)
	}
	sortInventoryItems(out)
	return out
}

func sortInventoryItems(items []schemascout.InventoryItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Locator < items[j].Locator
	})
}

func attachInventoryRelationships(snapshotID string, items []schemascout.InventoryItem) []schemascout.InventoryItem {
	normalizedSnapshotID := strings.TrimSpace(snapshotID)
	for index := range items {
		item := items[index]
		relationship := schemacommon.RelationshipEnvelope{
			EntityRefs: normalizeInventoryRelationshipRefs([]schemacommon.RelationshipRef{
				{Kind: "tool", ID: strings.TrimSpace(item.ID)},
				{Kind: "resource", ID: strings.TrimSpace(item.Locator)},
			}),
			Edges: normalizeInventoryRelationshipEdges([]schemacommon.RelationshipEdge{
				{
					Kind: "derived_from",
					From: schemacommon.RelationshipNodeRef{Kind: "tool", ID: strings.TrimSpace(item.ID)},
					To:   schemacommon.RelationshipNodeRef{Kind: "resource", ID: strings.TrimSpace(item.Locator)},
				},
			}),
			ParentRecordID: normalizedSnapshotID,
		}
		if normalizedSnapshotID != "" {
			relationship.ParentRef = &schemacommon.RelationshipNodeRef{Kind: "evidence", ID: normalizedSnapshotID}
		}
		relationship.RelatedEntityIDs = inventoryRelationshipRefIDs(relationship.EntityRefs)
		if relationship.ParentRef == nil && len(relationship.EntityRefs) == 0 && len(relationship.Edges) == 0 {
			item.Relationship = nil
		} else {
			item.Relationship = &relationship
		}
		items[index] = item
	}
	return items
}

func normalizeInventoryRelationshipRefs(refs []schemacommon.RelationshipRef) []schemacommon.RelationshipRef {
	if len(refs) == 0 {
		return nil
	}
	normalized := make([]schemacommon.RelationshipRef, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		kind := strings.ToLower(strings.TrimSpace(ref.Kind))
		id := strings.TrimSpace(ref.ID)
		if kind == "" || id == "" {
			continue
		}
		if kind != "agent" && kind != "tool" && kind != "resource" && kind != "policy" && kind != "run" && kind != "trace" && kind != "delegation" && kind != "evidence" {
			continue
		}
		key := kind + "\x00" + id
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, schemacommon.RelationshipRef{Kind: kind, ID: id})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		return normalized[i].ID < normalized[j].ID
	})
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeInventoryRelationshipEdges(edges []schemacommon.RelationshipEdge) []schemacommon.RelationshipEdge {
	if len(edges) == 0 {
		return nil
	}
	normalized := make([]schemacommon.RelationshipEdge, 0, len(edges))
	seen := map[string]struct{}{}
	for _, edge := range edges {
		kind := strings.ToLower(strings.TrimSpace(edge.Kind))
		fromKind := strings.ToLower(strings.TrimSpace(edge.From.Kind))
		fromID := strings.TrimSpace(edge.From.ID)
		toKind := strings.ToLower(strings.TrimSpace(edge.To.Kind))
		toID := strings.TrimSpace(edge.To.ID)
		if kind == "" || fromKind == "" || fromID == "" || toKind == "" || toID == "" {
			continue
		}
		if kind != "delegates_to" && kind != "calls" && kind != "governed_by" && kind != "targets" && kind != "derived_from" && kind != "emits_evidence" {
			continue
		}
		key := kind + "\x00" + fromKind + "\x00" + fromID + "\x00" + toKind + "\x00" + toID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, schemacommon.RelationshipEdge{
			Kind: kind,
			From: schemacommon.RelationshipNodeRef{Kind: fromKind, ID: fromID},
			To:   schemacommon.RelationshipNodeRef{Kind: toKind, ID: toID},
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		if normalized[i].From.Kind != normalized[j].From.Kind {
			return normalized[i].From.Kind < normalized[j].From.Kind
		}
		if normalized[i].From.ID != normalized[j].From.ID {
			return normalized[i].From.ID < normalized[j].From.ID
		}
		if normalized[i].To.Kind != normalized[j].To.Kind {
			return normalized[i].To.Kind < normalized[j].To.Kind
		}
		return normalized[i].To.ID < normalized[j].To.ID
	})
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func inventoryRelationshipRefIDs(refs []schemacommon.RelationshipRef) []string {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if id := strings.TrimSpace(ref.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueSorted(ids)
}

func normalizeIdentifier(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, char := range lower {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	normalized := strings.Trim(builder.String(), "_")
	if normalized != "" {
		return normalized
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
