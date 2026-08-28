package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Release-blocking interoperability coverage: Wrkr's sole-argument consumer
// contract must remain sufficient for every released proposal fixture.
func TestActionContractConsumerNineReleasedReceipts(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-interop", "v1", "expected")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, readErr := filepath.Glob(filepath.Join(root, entry.Name(), "pac-*.json"))
		if readErr != nil || len(files) != 1 {
			continue
		}
		count++
		output := captureStdout(t, func() {
			if code := runActionContractConsume([]string{files[0]}); code != exitOK {
				t.Fatalf("%s: consume exit=%d", entry.Name(), code)
			}
		})
		var receipt actionContractReceipt
		if err := json.Unmarshal([]byte(output), &receipt); err != nil {
			t.Fatalf("%s receipt: %v", entry.Name(), err)
		}
		if receipt.Status != "pass" || !receipt.SemanticResult.ProposalValid || receipt.SemanticResult.ExecutionClaim || receipt.SemanticResult.EffectClaim {
			t.Fatalf("%s receipt contract mismatch: %+v", entry.Name(), receipt)
		}
	}
	if count != 9 {
		t.Fatalf("expected nine released proposal fixtures, got %d", count)
	}
}
