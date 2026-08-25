package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Clyra-AI/gait/core/actioncontract"
)

func writeCircuitInput(t *testing.T, path string) {
	t.Helper()
	raw, err := json.Marshal(actioncontract.CircuitBreakerInput{SchemaID: actioncontract.CircuitInputSchemaID, SchemaVersion: "1", Chain: actioncontract.ChainDecision{SchemaID: actioncontract.ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: actioncontract.ChainState{SchemaID: actioncontract.ChainStateSchemaID, SchemaVersion: "1", StepIDs: []string{}, Classes: []string{}, Targets: []string{}}}, EffectStatus: "pass", EffectAuthoritative: true, ContainmentStatus: "completed", StopStatus: "", RevocationStatus: ""})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestActionContractCircuitCLIContracts(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "input.json")
	output := filepath.Join(root, "decision.json")
	writeCircuitInput(t, input)
	if code := runActionContractCircuit([]string{"evaluate", "--input", input, "--out", output, "--json"}); code != exitOK {
		t.Fatalf("allow exit=%d", code)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	if code := runActionContractCircuit([]string{"evaluate", input}); code != exitInvalidInput {
		t.Fatalf("positional exit=%d", code)
	}
	if code := runActionContractCircuit([]string{"evaluate", "--help"}); code != exitOK {
		t.Fatalf("help exit=%d", code)
	}
}
