package actioncontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewActionContractSchemaAssetsParity(t *testing.T) {
	names := []string{"advisory-evaluator-report.schema.json", "chain-policy.schema.json", "chain-state.schema.json", "chain-decision.schema.json", "circuit-breaker-input.schema.json", "circuit-breaker-decision.schema.json", "control-event-evidence.schema.json", "lifecycle-receipt.schema.json"}
	for _, n := range names {
		checked, e := os.ReadFile(filepath.Join("..", "..", "schemas", "v1", "action-contract", n))
		if e != nil {
			t.Fatal(e)
		}
		embedded, e := schemaAssets.ReadFile("schemaassets/" + n)
		if e != nil {
			t.Fatal(e)
		}
		if string(checked) != string(embedded) {
			t.Fatalf("schema drift: %s", n)
		}
	}
}
