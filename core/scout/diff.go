package scout

import (
	"encoding/json"
	"sort"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

type SnapshotDiff struct {
	LeftSnapshotID  string       `json:"left_snapshot_id"`
	RightSnapshotID string       `json:"right_snapshot_id"`
	Added           []DiffItem   `json:"added,omitempty"`
	Removed         []DiffItem   `json:"removed,omitempty"`
	Changed         []DiffChange `json:"changed,omitempty"`
	AddedCount      int          `json:"added_count"`
	RemovedCount    int          `json:"removed_count"`
	ChangedCount    int          `json:"changed_count"`
}

type DiffItem struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Locator   string `json:"locator"`
	RiskLevel string `json:"risk_level,omitempty"`
}

type DiffChange struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	BeforeLocator   string `json:"before_locator"`
	AfterLocator    string `json:"after_locator"`
	BeforeRiskLevel string `json:"before_risk_level,omitempty"`
	AfterRiskLevel  string `json:"after_risk_level,omitempty"`
}

func DiffSnapshots(left schemascout.InventorySnapshot, right schemascout.InventorySnapshot) SnapshotDiff {
	leftIndex := map[string]schemascout.InventoryItem{}
	rightIndex := map[string]schemascout.InventoryItem{}
	for _, item := range left.Items {
		leftIndex[item.ID] = item
	}
	for _, item := range right.Items {
		rightIndex[item.ID] = item
	}

	added := make([]DiffItem, 0)
	removed := make([]DiffItem, 0)
	changed := make([]DiffChange, 0)

	for id, rightItem := range rightIndex {
		leftItem, exists := leftIndex[id]
		if !exists {
			added = append(added, toDiffItem(rightItem))
			continue
		}
		if inventoryItemEquivalent(leftItem, rightItem) {
			continue
		}
		changed = append(changed, DiffChange{
			ID:              id,
			Kind:            rightItem.Kind,
			Name:            rightItem.Name,
			BeforeLocator:   leftItem.Locator,
			AfterLocator:    rightItem.Locator,
			BeforeRiskLevel: leftItem.RiskLevel,
			AfterRiskLevel:  rightItem.RiskLevel,
		})
	}
	for id, leftItem := range leftIndex {
		if _, exists := rightIndex[id]; !exists {
			removed = append(removed, toDiffItem(leftItem))
		}
	}

	sort.Slice(added, func(i, j int) bool {
		return added[i].ID < added[j].ID
	})
	sort.Slice(removed, func(i, j int) bool {
		return removed[i].ID < removed[j].ID
	})
	sort.Slice(changed, func(i, j int) bool {
		return changed[i].ID < changed[j].ID
	})

	return SnapshotDiff{
		LeftSnapshotID:  left.SnapshotID,
		RightSnapshotID: right.SnapshotID,
		Added:           added,
		Removed:         removed,
		Changed:         changed,
		AddedCount:      len(added),
		RemovedCount:    len(removed),
		ChangedCount:    len(changed),
	}
}

func inventoryItemEquivalent(left schemascout.InventoryItem, right schemascout.InventoryItem) bool {
	if left.Kind != right.Kind || left.Name != right.Name || left.Locator != right.Locator || left.RiskLevel != right.RiskLevel {
		return false
	}
	leftTags, _ := json.Marshal(uniqueSorted(left.Tags))
	rightTags, _ := json.Marshal(uniqueSorted(right.Tags))
	return string(leftTags) == string(rightTags)
}

func toDiffItem(item schemascout.InventoryItem) DiffItem {
	return DiffItem{
		ID:        item.ID,
		Kind:      item.Kind,
		Name:      item.Name,
		Locator:   item.Locator,
		RiskLevel: item.RiskLevel,
	}
}
