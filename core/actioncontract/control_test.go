package actioncontract

import (
	"crypto/ed25519"
	"crypto/sha256"
	proof "github.com/Clyra-AI/proof"
	"strings"
	"testing"
)

func strictControlRef(kind, id string) proof.RelationshipRef {
	r := executionTestRef(kind, id)
	r.SchemaID = "https://gait.dev/test"
	r.SchemaVersion = "1"
	r.SourceProduct = "gait"
	return r
}

func TestControlEventEvidenceRoundTrip(t *testing.T) {
	seed := sha256.Sum256([]byte("gait-action-contract-activation-development-key-v1"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	binding := executionTestBinding()
	causal := strictControlRef("execution", "exec")
	controlRef := strictControlRef("control", "ctrl")
	for _, tc := range []struct {
		command, phase string
		ack            bool
	}{{"stop", "requested", false}, {"stop", "acknowledged", true}, {"external_revocation", "attempted", false}, {"external_revocation", "acknowledged", true}, {"capability_invalidation", "invalidated", true}, {"descendant_invalidation", "invalidated", true}} {
		t.Run(tc.command+"_"+tc.phase, func(t *testing.T) {
			item, err := NewControlEventEvidence(ControlEventEvidence{EvidenceID: "control-" + tc.command + "-" + tc.phase, Binding: binding, EventRef: strictControlRef("event", "evt"), CausalRef: causal, ControlRef: controlRef, Command: tc.command, Phase: tc.phase, BoundaryID: "boundary", ResourceID: "resource", AffectedScope: []string{"scope"}, AdapterIdentity: "adapter", AdapterAcknowledged: tc.ack, OccurredAt: "2026-07-20T00:00:00Z", FreshUntil: "2026-07-20T01:00:00Z", ReasonCode: "test"}, private)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(item.CanonicalContentDigest, "sha256:") {
				t.Fatal("missing digest")
			}
			if ok, err := VerifyControlEventEvidence(item, public); err != nil || !ok {
				t.Fatalf("verify: %v %v", ok, err)
			}
		})
	}
}

func TestControlEventEvidenceRejectsTamperAndFreshness(t *testing.T) {
	seed := sha256.Sum256([]byte("gait-action-contract-activation-development-key-v1"))
	private := ed25519.NewKeyFromSeed(seed[:])
	item, err := NewControlEventEvidence(ControlEventEvidence{EvidenceID: "control", Binding: executionTestBinding(), EventRef: strictControlRef("event", "evt"), CausalRef: strictControlRef("execution", "exec"), ControlRef: strictControlRef("control", "ctrl"), Command: "stop", Phase: "acknowledged", BoundaryID: "b", ResourceID: "r", AffectedScope: []string{"s"}, AdapterIdentity: "a", AdapterAcknowledged: true, OccurredAt: "2026-07-20T00:00:00Z", FreshUntil: "2026-07-20T01:00:00Z", ReasonCode: "test"}, private)
	if err != nil {
		t.Fatal(err)
	}
	item.ReasonCode = "tampered"
	if ok, _ := VerifyControlEventEvidence(item, private.Public().(ed25519.PublicKey)); ok {
		t.Fatal("tampered control verified")
	}
	item.FreshUntil = "2026-07-19T00:00:00Z"
	if _, err := NewControlEventEvidence(item, private); err == nil {
		t.Fatal("stale control accepted")
	}
}
