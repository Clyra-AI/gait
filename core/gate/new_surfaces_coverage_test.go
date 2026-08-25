package gate

import (
	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCoverageSafeTokenPathFailures(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	if e := os.Mkdir(parent, 0700); e != nil {
		t.Fatal(e)
	}
	if e := safeTokenPath(filepath.Join(parent, "new.json")); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(parent, "target")
	if e := os.WriteFile(target, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := safeTokenPath(target); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "link")
	if e := os.Symlink(parent, link); e != nil {
		t.Fatal(e)
	}
	if e := safeTokenPath(filepath.Join(link, "x")); e == nil {
		t.Fatal("symlink parent accepted")
	}
	if e := os.Remove(target); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(target, 0700); e != nil {
		t.Fatal(e)
	}
	if e := safeTokenPath(target); e == nil {
		t.Fatal("directory destination accepted")
	}
}
func TestCoverageReadTokenFileRejectsSymlinkAndMissing(t *testing.T) {
	root := t.TempDir()
	if _, e := readTokenFile(filepath.Join(root, "missing")); e == nil {
		t.Fatal("missing accepted")
	}
	target := filepath.Join(root, "target")
	if e := os.WriteFile(target, []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "link")
	if e := os.Symlink(target, link); e != nil {
		t.Fatal(e)
	}
	if _, e := readTokenFile(link); e == nil {
		t.Fatal("symlink accepted")
	}
}
func TestCoverageWriteApprovalTokenPaths(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "nested", "approval.json")
	if e := WriteApprovalToken(p, schemagate.ApprovalToken{TokenID: "t"}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(p); e != nil {
		t.Fatal(e)
	}
	parent := filepath.Join(root, "file")
	if e := os.WriteFile(parent, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := WriteApprovalToken(filepath.Join(parent, "child.json"), schemagate.ApprovalToken{}); e == nil {
		t.Fatal("file parent accepted")
	}
}
func TestCoverageDelegationNonExpansionRejectsParentMismatch(t *testing.T) {
	parent := schemagate.DelegationToken{DelegatorIdentity: "root", DelegateIdentity: "child", ContractDigest: "sha256:" + strings.Repeat("a", 64), ActionClasses: []string{"write"}, TargetScope: []string{"repo"}, EnvironmentScope: []string{"prod"}, DataClasses: []string{"internal"}, NetworkDestinations: []string{"api"}, MaxOperations: 2, MaxTargets: 1, MaxDescendantDepth: 1, ExpiresAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)}
	child := parent
	child.DelegatorIdentity = "other"
	if e := ValidateDelegationNonExpansion(parent, child); e == nil {
		t.Fatal("mismatched parent accepted")
	}
}
func TestCoverageDelegationNonExpansionReasonBranches(t *testing.T) {
	p := schemagate.DelegationToken{TokenID: "p", DelegatorIdentity: "root", DelegateIdentity: "child", OriginAuthorityDigest: "sha256:" + strings.Repeat("a", 64), ContractDigest: "sha256:" + strings.Repeat("b", 64), IntentDigest: "sha256:" + strings.Repeat("c", 64), PolicyDigest: "sha256:" + strings.Repeat("d", 64), Scope: []string{"write"}, ActionClasses: []string{"write"}, TargetScope: []string{"repo"}, EnvironmentScope: []string{"prod"}, DataClasses: []string{"internal"}, NetworkDestinations: []string{"api"}, MaxOperations: 4, MaxTargets: 2, MaxDescendantDepth: 2, ExpiresAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Depth: 0}
	c := p
	c.DelegatorIdentity = "child"
	c.DelegateIdentity = "leaf"
	c.ParentTokenID = p.TokenID
	c.ParentTokenDigest, _ = DelegationTokenDigest(p)
	c.Depth = 1
	for _, mut := range []func(){func() { p.Revoked = true }, func() { c.ParentTokenID = "bad" }, func() { c.OriginAuthorityDigest = "sha256:" + strings.Repeat("e", 64) }, func() { c.Depth = 3 }, func() { c.ActionClasses = []string{"delete"} }} {
		p2, c2 := p, c
		mut()
		_ = p2
		_ = c2
		if e := ValidateDelegationNonExpansion(p, c); e == nil {
			t.Fatal("invalid delegation accepted")
		}
	}
}

func TestCoverageNormalizeAgentLineageCanonicalizesAndDropsEmpty(t *testing.T) {
	got := normalizeAgentLineage([]schemacommon.AgentLineage{{AgentID: " b ", DelegatedBy: " z "}, {AgentID: "a"}, {AgentID: ""}, {AgentID: " b ", DelegatedBy: " z "}})
	if len(got) != 2 || got[0].AgentID != "a" || got[1].AgentID != "b" {
		t.Fatalf("unexpected lineage: %+v", got)
	}
	if normalizeAgentLineage(nil) != nil {
		t.Fatal("nil lineage should remain nil")
	}
}

func TestCoverageTokenDecodeAndParentDigest(t *testing.T) {
	var v map[string]any
	if e := strictTokenDecode([]byte(`{"x":1}`), &v); e != nil {
		t.Fatal(e)
	}
	if e := strictTokenDecode([]byte(`{"x":1}{}`), &v); e == nil {
		t.Fatal("trailing token accepted")
	}
	if !validParentTokenDigest("sha256:"+strings.Repeat("a", 64)) || !validParentTokenDigest(strings.Repeat("b", 64)) || validParentTokenDigest("bad") {
		t.Fatal("parent digest validation mismatch")
	}
}
func TestCoverageNormalizeApprovalTokenErrorBranches(t *testing.T) {
	base := schemagate.ApprovalToken{SchemaID: "gait.gate.approval_token", SchemaVersion: "1.0.0", TokenID: "t", ApproverIdentity: "a", ReasonCode: "r", IntentDigest: strings.Repeat("a", 64), PolicyDigest: strings.Repeat("b", 64), Scope: []string{"read"}, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), MaxOps: 1, MaxTargets: 1}
	cases := []schemagate.ApprovalToken{{}, {SchemaID: "bad"}, {SchemaID: base.SchemaID, SchemaVersion: "bad"}, {SchemaID: base.SchemaID, SchemaVersion: base.SchemaVersion, TokenID: ""}, {SchemaID: base.SchemaID, SchemaVersion: base.SchemaVersion, TokenID: "t"}, {SchemaID: base.SchemaID, SchemaVersion: base.SchemaVersion, TokenID: "t", ApproverIdentity: "a", ReasonCode: "r", IntentDigest: "bad", PolicyDigest: base.PolicyDigest, Scope: []string{"read"}}}
	for _, v := range cases {
		if _, e := normalizeApprovalToken(v); e == nil {
			t.Fatalf("invalid approval accepted: %+v", v)
		}
	}
	if _, e := normalizeApprovalToken(base); e != nil {
		t.Fatal(e)
	}
}
func TestCoveragePolicyContextBranches(t *testing.T) {
	base := Policy{normalized: true}
	if PolicyRequiresContextEvidence(base) {
		t.Fatal("empty policy requires context")
	}
	for _, p := range []Policy{{normalized: true, FailClosed: FailClosedPolicy{RequiredFields: []string{"context_evidence"}}}, {normalized: true, Rules: []PolicyRule{{RequireContextEvidence: true}}}, {normalized: true, Rules: []PolicyRule{{RequiredContextEvidenceMode: "required"}}}, {normalized: true, Rules: []PolicyRule{{MaxContextAgeSeconds: 1}}}} {
		if !PolicyRequiresContextEvidence(p) {
			t.Fatalf("context requirement missed: %+v", p)
		}
	}
}
func TestCoveragePolicyRequiresContextEvidenceNormalized(t *testing.T) {
	if PolicyRequiresContextEvidence(Policy{normalized: true}) {
		t.Fatal("empty normalized policy requires context")
	}
	if !PolicyRequiresContextEvidence(Policy{normalized: true, FailClosed: FailClosedPolicy{RequiredFields: []string{"context_evidence"}}}) {
		t.Fatal("required field missing")
	}
	for _, rule := range []PolicyRule{{RequireContextEvidence: true}, {RequiredContextEvidenceMode: "required"}, {MaxContextAgeSeconds: 1}} {
		if !PolicyRequiresContextEvidence(Policy{normalized: true, Rules: []PolicyRule{rule}}) {
			t.Fatalf("rule not detected: %+v", rule)
		}
	}
}
func TestCoverageToolAnnotationMatchesAllHints(t *testing.T) {
	r, f, i, o := true, false, true, false
	match := ToolAnnotationMatch{ReadOnlyHint: &r, DestructiveHint: &f, IdempotentHint: &i, OpenWorldHint: &o}
	target := []schemagate.IntentTarget{{ReadOnlyHint: r, DestructiveHint: f, IdempotentHint: i, OpenWorldHint: o}}
	if !toolAnnotationsMatch(match, target) {
		t.Fatal("matching annotations rejected")
	}
	target[0].OpenWorldHint = !o
	if toolAnnotationsMatch(match, target) {
		t.Fatal("mismatching annotation accepted")
	}
}
func TestCoverageToolAnnotationMismatchesEachHint(t *testing.T) {
	for _, idx := range []int{0, 1, 2, 3} {
		r, f, i, o := true, false, true, false
		match := ToolAnnotationMatch{ReadOnlyHint: &r, DestructiveHint: &f, IdempotentHint: &i, OpenWorldHint: &o}
		target := []schemagate.IntentTarget{{ReadOnlyHint: r, DestructiveHint: f, IdempotentHint: i, OpenWorldHint: o}}
		switch idx {
		case 0:
			target[0].ReadOnlyHint = !r
		case 1:
			target[0].DestructiveHint = !f
		case 2:
			target[0].IdempotentHint = !i
		case 3:
			target[0].OpenWorldHint = !o
		}
		if toolAnnotationsMatch(match, target) {
			t.Fatalf("mismatch %d accepted", idx)
		}
	}
}
