package regress

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/runpack"
	schema "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"time"
)

type LifecycleReceiptMaterializeOptions struct{ SigningPrivateKey ed25519.PrivateKey }

// MaterializeLifecycleReceipt creates a reference-only runpack; no tool is run.
func MaterializeLifecycleReceipt(receipt actioncontract.LifecycleReceipt, opts LifecycleReceiptMaterializeOptions) (runpack.RecordResult, error) {
	if len(opts.SigningPrivateKey) != ed25519.PrivateKeySize {
		return runpack.RecordResult{}, fmt.Errorf("lifecycle_receipt_runpack_key_required")
	}
	t, err := time.Parse(time.RFC3339Nano, receipt.ObservedAt)
	if err != nil {
		return runpack.RecordResult{}, fmt.Errorf("lifecycle_receipt_observed_at_invalid")
	}
	idHash := sha256.Sum256([]byte(receipt.CanonicalContentDigest))
	runID := "run_lifecycle_" + hex.EncodeToString(idHash[:])[:16]
	intentID := "intent_lifecycle_" + hex.EncodeToString(idHash[:])[:16]
	run := schema.Run{SchemaID: "gait.run", SchemaVersion: "1.0.0", CreatedAt: t, ProducerVersion: "gait", RunID: runID, Timeline: []schema.TimelineEvt{{Event: "lifecycle_receipt", TS: t, Ref: receipt.CanonicalContentDigest}}}
	intent := schema.IntentRecord{SchemaID: "gait.runpack.intent", SchemaVersion: "1.0.0", CreatedAt: t, ProducerVersion: "gait", RunID: runID, IntentID: intentID, ToolName: "action_contract.lifecycle", ArgsDigest: receipt.CanonicalContentDigest}
	status := "error"
	if receipt.Outcome == "succeeded" || receipt.Outcome == "pass" {
		status = "ok"
	}
	result := schema.ResultRecord{SchemaID: "gait.runpack.result", SchemaVersion: "1.0.0", CreatedAt: t, ProducerVersion: "gait", RunID: runID, IntentID: intentID, Status: status, ResultDigest: receipt.CanonicalContentDigest, Result: map[string]any{"source_kind": "lifecycle_receipt", "outcome": receipt.Outcome, "authority": receipt.Authority, "quarantine": receipt.Quarantine, "redaction": receipt.Redaction, "reason_codes": receipt.ReasonCodes}}
	refs := schema.Refs{SchemaID: "gait.runpack.refs", SchemaVersion: "1.0.0", CreatedAt: t, ProducerVersion: "gait", RunID: runID, Receipts: make([]schema.RefReceipt, 0, len(receipt.ArtifactDigests))}
	for i, d := range receipt.ArtifactDigests {
		refs.Receipts = append(refs.Receipts, schema.RefReceipt{RefID: fmt.Sprintf("lifecycle:%d", i), SourceType: "lifecycle_receipt", SourceLocator: receipt.ReceiptID, QueryDigest: d, ContentDigest: d, RetrievedAt: t, RedactionMode: receipt.Redaction})
	}
	return runpack.RecordRun(runpack.RecordOptions{Run: run, Intents: []schema.IntentRecord{intent}, Results: []schema.ResultRecord{result}, Refs: refs, CaptureMode: "reference", SignKey: opts.SigningPrivateKey})
}
