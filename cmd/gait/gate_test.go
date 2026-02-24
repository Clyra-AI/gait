package main

import (
	"testing"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestGateIntentTargetCountAndOperationCount(t *testing.T) {
	intent := schemagate.IntentRequest{
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/fallback"},
		},
		Script: &schemagate.IntentScript{
			Steps: []schemagate.IntentScriptStep{
				{
					ToolName: "tool.read",
					Targets: []schemagate.IntentTarget{
						{Kind: "path", Value: "/tmp/a"},
					},
				},
				{
					ToolName: "tool.write",
					Targets: []schemagate.IntentTarget{
						{Kind: "path", Value: "/tmp/b"},
						{Kind: "path", Value: "/tmp/c"},
					},
				},
			},
		},
	}

	if got := gateIntentTargetCount(intent); got != 3 {
		t.Fatalf("gateIntentTargetCount() = %d, want 3", got)
	}
	if got := gateIntentOperationCount(intent); got != 2 {
		t.Fatalf("gateIntentOperationCount() = %d, want 2", got)
	}
}

func TestGateIntentTargetCountFallsBackWhenScriptTargetsMissing(t *testing.T) {
	intent := schemagate.IntentRequest{
		Targets: []schemagate.IntentTarget{
			{Kind: "path", Value: "/tmp/fallback-a"},
			{Kind: "path", Value: "/tmp/fallback-b"},
		},
		Script: &schemagate.IntentScript{
			Steps: []schemagate.IntentScriptStep{
				{ToolName: "tool.read"},
				{ToolName: "tool.write"},
			},
		},
	}

	if got := gateIntentTargetCount(intent); got != 2 {
		t.Fatalf("gateIntentTargetCount() = %d, want 2", got)
	}
	if got := gateIntentOperationCount(intent); got != 2 {
		t.Fatalf("gateIntentOperationCount() = %d, want 2", got)
	}
}

func TestGateIntentOperationCountDefaultsToOneWithoutTargets(t *testing.T) {
	intent := schemagate.IntentRequest{}
	if got := gateIntentTargetCount(intent); got != 0 {
		t.Fatalf("gateIntentTargetCount() = %d, want 0", got)
	}
	if got := gateIntentOperationCount(intent); got != 1 {
		t.Fatalf("gateIntentOperationCount() = %d, want 1", got)
	}
}
