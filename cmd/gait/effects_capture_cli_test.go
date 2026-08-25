package main

import "testing"

func TestEffectsCaptureRequiresObservationPair(t *testing.T) {
	if c := runEffects([]string{"capture", "--resource", "filesystem", "--out", "x", "--private-key", "k", "--action-digest", "sha256:x"}); c != exitInvalidInput {
		t.Fatalf("expected invalid input, got %d", c)
	}
}
