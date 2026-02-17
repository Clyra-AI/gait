package scout

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/gate"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

type CoverageReport struct {
	SnapshotID             string   `json:"snapshot_id"`
	DiscoveredTools        int      `json:"discovered_tools"`
	GatedTools             int      `json:"gated_tools"`
	HighRiskUngatedTools   int      `json:"high_risk_ungated_tools"`
	CoveragePercent        float64  `json:"coverage_percent"`
	PolicyDigests          []string `json:"policy_digests,omitempty"`
	DiscoveredToolNames    []string `json:"discovered_tool_names,omitempty"`
	GatedToolNames         []string `json:"gated_tool_names,omitempty"`
	HighRiskUngatedToolIDs []string `json:"high_risk_ungated_tool_ids,omitempty"`
}

func BuildCoverage(snapshot schemascout.InventorySnapshot, policyPaths []string) (CoverageReport, error) {
	discoveredToolNames := map[string]struct{}{}
	highRiskUngated := map[string]struct{}{}
	highRiskItems := map[string]schemascout.InventoryItem{}
	for _, item := range snapshot.Items {
		if !isToolItem(item) {
			continue
		}
		discoveredToolNames[strings.ToLower(strings.TrimSpace(item.Name))] = struct{}{}
		if item.RiskLevel == "high" || item.RiskLevel == "critical" {
			highRiskItems[item.ID] = item
		}
	}

	gatedToolNames := map[string]struct{}{}
	policyDigests := make([]string, 0, len(policyPaths))
	for _, policyPath := range normalizePolicyPaths(policyPaths) {
		policy, err := gate.LoadPolicyFile(policyPath)
		if err != nil {
			return CoverageReport{}, fmt.Errorf("load policy %s: %w", policyPath, err)
		}
		digest, err := gate.PolicyDigest(policy)
		if err != nil {
			return CoverageReport{}, fmt.Errorf("digest policy %s: %w", policyPath, err)
		}
		policyDigests = append(policyDigests, digest)
		for _, rule := range policy.Rules {
			for _, toolName := range rule.Match.ToolNames {
				trimmed := strings.ToLower(strings.TrimSpace(toolName))
				if trimmed == "" {
					continue
				}
				gatedToolNames[trimmed] = struct{}{}
			}
		}
	}
	sort.Strings(policyDigests)

	for itemID, item := range highRiskItems {
		normalizedName := strings.ToLower(strings.TrimSpace(item.Name))
		if _, ok := gatedToolNames[normalizedName]; !ok {
			highRiskUngated[itemID] = struct{}{}
		}
	}

	discoveredCount := len(discoveredToolNames)
	gatedCount := 0
	for toolName := range discoveredToolNames {
		if _, ok := gatedToolNames[toolName]; ok {
			gatedCount++
		}
	}

	coveragePercent := 100.0
	if discoveredCount > 0 {
		coveragePercent = roundToTwoDecimals((float64(gatedCount) / float64(discoveredCount)) * 100)
	}

	report := CoverageReport{
		SnapshotID:             snapshot.SnapshotID,
		DiscoveredTools:        discoveredCount,
		GatedTools:             gatedCount,
		HighRiskUngatedTools:   len(highRiskUngated),
		CoveragePercent:        coveragePercent,
		PolicyDigests:          policyDigests,
		DiscoveredToolNames:    mapKeysSorted(discoveredToolNames),
		GatedToolNames:         mapKeysSorted(gatedToolNames),
		HighRiskUngatedToolIDs: mapKeysSorted(highRiskUngated),
	}
	return report, nil
}

func normalizePolicyPaths(policyPaths []string) []string {
	normalized := make([]string, 0, len(policyPaths))
	for _, policyPath := range policyPaths {
		for _, segment := range strings.Split(policyPath, ",") {
			trimmed := strings.TrimSpace(segment)
			if trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
	}
	return uniqueSorted(normalized)
}

func isToolItem(item schemascout.InventoryItem) bool {
	switch strings.ToLower(strings.TrimSpace(item.Kind)) {
	case "tool", "mcp_server":
		return true
	default:
		return false
	}
}

func mapKeysSorted(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func roundToTwoDecimals(value float64) float64 {
	if value < 0 {
		return float64(int(value*100-0.5)) / 100
	}
	return float64(int(value*100+0.5)) / 100
}
