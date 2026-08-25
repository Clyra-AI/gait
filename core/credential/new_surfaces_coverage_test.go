package credential

import (
	"testing"
	"time"
)

type coverageBroker struct {
	response Response
	err      error
}

func (b coverageBroker) Name() string                    { return "coverage" }
func (b coverageBroker) Issue(Request) (Response, error) { return b.response, b.err }
func TestCoverageRequestBindingNormalization(t *testing.T) {
	base := Request{ToolName: "  AWS  ", Identity: " id ", ContractFamilyID: "family", ContractID: "contract", ContractRevision: 1, ProposalDigest: "sha256:" + repeatHex(), ActivationDigest: "sha256:" + repeatHex(), PolicyDigest: "sha256:" + repeatHex(), ExpectedOutcome: "succeeded", EffectScope: []string{"write", "WRITE"}, ContainmentScope: []string{"done"}}
	if _, e := normalizeRequest(base); e != nil {
		t.Fatal(e)
	}
	for _, mut := range []Request{{ToolName: "x", Identity: "i", ContractID: "only"}, {ToolName: "x", Identity: "i", ContractFamilyID: "f", ContractID: "c", ContractRevision: 1, ProposalDigest: "bad"}} {
		if _, e := normalizeRequest(mut); e == nil {
			t.Fatalf("invalid binding accepted: %+v", mut)
		}
	}
}
func TestCoverageProviderMismatchAndReceiptFallback(t *testing.T) {
	req := Request{ToolName: "x", Identity: "id"}
	payload := []byte(`{"provider":"unknown"}`)
	if _, e := NormalizeProviderReceipt("", payload, req); e == nil {
		t.Fatal("unsupported provider accepted")
	}
	payload = []byte(`{"provider":"aws","assumed_role_arn":"arn:aws:iam::1:role/r","issued_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-01T00:01:00Z"}`)
	r, e := NormalizeProviderReceipt("aws", payload, req)
	if e != nil || r.CredentialRef == "" {
		t.Fatalf("fallback failed: %+v %v", r, e)
	}
}
func TestCoverageIssueRejectsMissingCredentialAndBindsResponse(t *testing.T) {
	_, e := Issue(nil, Request{})
	if e == nil {
		t.Fatal("nil broker accepted")
	}
	b := coverageBroker{response: Response{CredentialRef: "", IssuedAt: time.Now()}}
	if _, e := Issue(b, Request{ToolName: "x", Identity: "id"}); e == nil {
		t.Fatal("empty credential accepted")
	}
}
func repeatHex() string { return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" }
