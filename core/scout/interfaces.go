package scout

import (
	"context"

	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

type SnapshotRequest struct {
	Roots   []string
	Include []string
	Exclude []string
}

type InventoryProvider interface {
	Snapshot(context.Context, SnapshotRequest) (schemascout.InventorySnapshot, error)
}
