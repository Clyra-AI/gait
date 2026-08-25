package main

import (
	"strings"
	"testing"
)

func TestActionContractHelpListsGovernanceSubcommands(t *testing.T) {
	out, _ := captureEffectsOutput(t, func() int { printActionContractUsage(); return 0 })
	for _, want := range []string{"advisory evaluate|verify", "chain evaluate", "circuit evaluate", "otel"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q: %s", want, out)
		}
	}
}
