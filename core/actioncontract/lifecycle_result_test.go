package actioncontract

import (
	"crypto/ed25519"
	"strings"
	"testing"
)

func TestAppendLifecycleResultProducesSignedExecutionTerminalRecords(t *testing.T) {
	proposal, activation, action, readiness, _, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	path := t.TempDir() + "/lifecycle.jsonl"
	private := mustKey(t)
	digest := "sha256:" + strings.Repeat("a", 64)
	records, err := AppendLifecycleResult(LifecycleResultOptions{JournalPath: path, Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, PrivateKey: private, Outcome: "succeeded", TraceDigest: digest, TraceID: "trace-test", ResultDigest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Kind != LifecycleExecutionStarted || records[1].Kind != LifecycleExecutionSucceeded {
		t.Fatalf("records: %#v", records)
	}
	for _, record := range records {
		if valid, verifyErr := VerifyLifecycleRecord(record, private.Public().(ed25519.PublicKey)); verifyErr != nil || !valid {
			t.Fatalf("record signature: valid=%t err=%v", valid, verifyErr)
		}
	}
}

func TestContractEvidenceBindingAndLifecycleEffectCompensation(t *testing.T) {
	proposal, activation, action, readiness, _, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	digest := "sha256:" + strings.Repeat("b", 64)
	binding, err := BuildContractEvidenceBinding(proposal, activation, action, readiness, "trace-real", digest, digest, "")
	if err != nil || binding.ActivationRef.Digest == "" {
		t.Fatalf("binding: %#v %v", binding, err)
	}
	if _, err := AppendLifecycleResult(LifecycleResultOptions{JournalPath: t.TempDir() + "/events.jsonl", Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, PrivateKey: mustKey(t), Outcome: "failed", TraceDigest: digest, TraceID: "trace-real", ResultDigest: digest, EffectOutcome: "validated"}); err == nil {
		t.Fatal("effect on failed execution accepted")
	}
	if _, err := AppendLifecycleResult(LifecycleResultOptions{JournalPath: t.TempDir() + "/events.jsonl", Proposal: proposal, Activation: activation, RuntimeAction: action, Readiness: readiness, PrivateKey: mustKey(t), Outcome: "succeeded", TraceDigest: digest, TraceID: "trace-real", ResultDigest: digest, CompensationOutcome: "bogus"}); err == nil {
		t.Fatal("invalid compensation accepted")
	}
}

func TestAppendLifecycleResultEmitsEffectAndCompensationRecords(t *testing.T) {
	proposal, activation, action, readiness, _, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	path := t.TempDir() + "/events.jsonl"
	digest := "sha256:" + strings.Repeat("c", 64)
	private := mustKey(t)
	records, err := AppendLifecycleResult(LifecycleResultOptions{
		JournalPath:         path,
		Proposal:            proposal,
		Activation:          activation,
		RuntimeAction:       action,
		Readiness:           readiness,
		PrivateKey:          private,
		Outcome:             "succeeded",
		EffectOutcome:       "validated",
		CompensationOutcome: "completed",
		TraceDigest:         digest,
		TraceID:             "trace-effects",
		ResultDigest:        digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 7 || records[2].Kind != LifecycleEffectRecorded || records[3].Kind != LifecycleEffectValidated || records[4].Kind != LifecycleCompensationRequired || records[5].Kind != LifecycleCompensationStarted || records[6].Kind != LifecycleCompensationCompleted {
		t.Fatalf("unexpected lifecycle records: %#v", records)
	}
	if !records[1].Execution.CompensationRequired {
		t.Fatal("terminal execution did not declare requested compensation")
	}
	if !mustParseTime(records[4].OccurredAt).After(mustParseTime(records[3].OccurredAt)) {
		t.Fatalf("compensation requirement did not follow final validated effect: effect=%s compensation=%s", records[3].OccurredAt, records[4].OccurredAt)
	}
	if len(records[1].Execution.Binding.CausalRefs) != 1 || records[1].Execution.Binding.CausalRefs[0].ID != records[0].Execution.EvidenceID {
		t.Fatalf("terminal execution lost started predecessor: %#v", records[1].Execution.Binding.CausalRefs)
	}
	if len(records[2].Effect.Binding.CausalRefs) != 1 || records[2].Effect.Binding.CausalRefs[0].ID != records[1].Execution.EvidenceID {
		t.Fatalf("effect lost terminal predecessor: %#v", records[2].Effect.Binding.CausalRefs)
	}
	if len(records[3].Effect.Binding.CausalRefs) != 1 || records[3].Effect.Binding.CausalRefs[0].ID != records[2].Effect.EvidenceID || !exactRef(records[3].Effect.EffectRef, records[2].Effect.EffectRef) {
		t.Fatalf("validated effect lost recorded predecessor: %#v", records[3].Effect.Binding.CausalRefs)
	}
	if len(records[5].Compensation.Binding.CausalRefs) != 1 || records[5].Compensation.Binding.CausalRefs[0].ID != records[4].Compensation.EvidenceID || len(records[6].Compensation.Binding.CausalRefs) != 1 || records[6].Compensation.Binding.CausalRefs[0].ID != records[5].Compensation.EvidenceID {
		t.Fatalf("compensation sequence lost causal predecessor")
	}
	for _, record := range records {
		if valid, verifyErr := VerifyLifecycleRecord(record, private.Public().(ed25519.PublicKey)); verifyErr != nil || !valid {
			t.Fatalf("record signature: valid=%t err=%v", valid, verifyErr)
		}
	}
}

func TestAppendLifecycleResultSelectsOptionalOutcomeKinds(t *testing.T) {
	proposal, activation, action, readiness, _, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	digest := "sha256:" + strings.Repeat("d", 64)
	for _, tc := range []struct {
		name         string
		effect       string
		compensation string
		wantEffect   LifecycleEventKind
		wantComp     LifecycleEventKind
	}{
		{"recorded", "recorded", "required", LifecycleEffectRecorded, LifecycleCompensationRequired},
		{"started", "recorded", "started", LifecycleEffectRecorded, LifecycleCompensationStarted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records, err := AppendLifecycleResult(LifecycleResultOptions{
				JournalPath:         t.TempDir() + "/events.jsonl",
				Proposal:            proposal,
				Activation:          activation,
				RuntimeAction:       action,
				Readiness:           readiness,
				PrivateKey:          mustKey(t),
				Outcome:             "succeeded",
				EffectOutcome:       tc.effect,
				CompensationOutcome: tc.compensation,
				TraceDigest:         digest,
				TraceID:             "trace-" + tc.name,
				ResultDigest:        digest,
			})
			if err != nil {
				t.Fatal(err)
			}
			wantLen := 4
			if tc.wantComp == LifecycleCompensationStarted {
				wantLen = 5
			}
			if len(records) != wantLen || records[2].Kind != tc.wantEffect || records[len(records)-1].Kind != tc.wantComp {
				t.Fatalf("records=%#v", records)
			}
		})
	}
}

func TestCircuitDigestHelpers(t *testing.T) {
	state := ChainState{SchemaID: ChainStateSchemaID, SchemaVersion: "1", StepCount: 1, StepIDs: []string{"step"}}
	digest, err := DigestChainState(state)
	if err != nil || !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("state digest: %s %v", digest, err)
	}
	input := CircuitBreakerInput{SchemaID: CircuitInputSchemaID, SchemaVersion: "1", Chain: ChainDecision{SchemaID: ChainDecisionSchemaID, SchemaVersion: "1", Allowed: true, State: state}, EffectStatus: "pass", EffectAuthoritative: true, IntentDigest: digest, ChainStateDigest: digest}
	binding, err := CircuitBindingDigest(input)
	if err != nil || !strings.HasPrefix(binding, "sha256:") {
		t.Fatalf("binding digest: %s %v", binding, err)
	}
}
