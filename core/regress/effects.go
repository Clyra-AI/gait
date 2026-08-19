package regress

import (
	"github.com/Clyra-AI/gait/core/effects"
	schemaregress "github.com/Clyra-AI/gait/core/schema/v1/regress"
)

type effectContractGrader struct{}

func (effectContractGrader) Name() string        { return "effect_contract" }
func (effectContractGrader) Deterministic() bool { return true }

func (effectContractGrader) Grade(ctx FixtureContext) (schemaregress.GraderResult, error) {
	if ctx.Fixture.EffectSnapshotPath == "" || ctx.Fixture.EffectContractPath == "" {
		return failResult("effect_contract", "effect_evidence_missing", map[string]any{"snapshot": ctx.Fixture.EffectSnapshotPath, "contract": ctx.Fixture.EffectContractPath}), nil
	}
	snapshot, err := effects.LoadSnapshot(ctx.Fixture.EffectSnapshotPath)
	if err != nil {
		return failResult("effect_contract", "effect_snapshot_read_failed", map[string]any{"error": err.Error()}), nil
	}
	contract, err := effects.LoadContract(ctx.Fixture.EffectContractPath)
	if err != nil {
		return failResult("effect_contract", "effect_contract_read_failed", map[string]any{"error": err.Error()}), nil
	}
	result := effects.Grade(snapshot, contract)
	details := map[string]any{"effect_result": result, "snapshot_path": ctx.Fixture.EffectSnapshotPath, "contract_path": ctx.Fixture.EffectContractPath}
	if result.Status == effects.GradePass {
		return schemaregress.GraderResult{Name: "effect_contract", Status: regressStatusPass, ReasonCodes: result.ReasonCodes, Details: details}, nil
	}
	reasons := append([]string(nil), result.ReasonCodes...)
	if result.Status == effects.GradeInconclusive {
		reasons = append(reasons, effects.ReasonPredicateInconclusive)
	} else {
		reasons = append(reasons, effects.ReasonPredicateFailed)
	}
	return schemaregress.GraderResult{Name: "effect_contract", Status: regressStatusFail, ReasonCodes: uniqueSortedStrings(reasons), Details: details}, nil
}
