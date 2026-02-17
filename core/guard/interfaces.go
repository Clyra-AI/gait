package guard

import (
	"context"

	schemaguard "github.com/Clyra-AI/gait/core/schema/v1/guard"
)

type BuildRequest struct {
	RunID      string
	RunpackZip string
	CaseID     string
	OutputPath string
}

type EvidencePackBuilder interface {
	Build(context.Context, BuildRequest) (schemaguard.PackManifest, error)
}
