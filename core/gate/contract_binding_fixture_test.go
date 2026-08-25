package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"
)

func TestContractBoundApprovalAndDelegationFixtures(t *testing.T) {
	seed := sha256.Sum256([]byte("gait-fixed-contract-fixture-key-v1"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tokenDir := t.TempDir()
	a, err := MintApprovalToken(MintApprovalTokenOptions{ApproverIdentity: "owner", ReasonCode: "jit", IntentDigest: digest, PolicyDigest: digest, ContractFamilyID: "pacf-demo", ContractID: "pac-demo", ContractRevision: 1, ProposalDigest: digest, ActivationDigest: digest, Scope: []string{"write"}, TargetScope: []string{"repo:a"}, EnvironmentScope: []string{"prod"}, OutcomeScope: []string{"succeeded"}, EffectScope: []string{"validated"}, ContainmentScope: []string{"completed"}, MaxTargets: 1, MaxOps: 2, TTL: time.Hour, Now: now, SigningPrivateKey: private, TokenPath: filepath.Join(tokenDir, "approval.json")})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateApprovalToken(a.Token, public, ApprovalValidationOptions{Now: now, ExpectedIntentDigest: digest, ExpectedPolicyDigest: digest, ExpectedContractFamilyID: "pacf-demo", ExpectedContractID: "pac-demo", ExpectedContractRevision: 1, ExpectedProposalDigest: digest, ExpectedActivationDigest: digest, ExpectedTargetScope: []string{"repo:a"}, ExpectedEnvironmentScope: []string{"prod"}, ExpectedOutcomeScope: []string{"succeeded"}, ExpectedEffectScope: []string{"validated"}, ExpectedContainmentScope: []string{"completed"}, RequiredScope: []string{"write"}, TargetCount: 1, OperationCount: 1}); err != nil {
		t.Fatalf("exact approval rejected: %v", err)
	}
	if err := ValidateApprovalToken(a.Token, public, ApprovalValidationOptions{Now: now, ExpectedContractID: "other"}); err == nil {
		t.Fatal("mismatched contract accepted")
	}
	d, err := MintDelegationToken(MintDelegationTokenOptions{DelegatorIdentity: "root", DelegateIdentity: "child", Scope: []string{"write"}, ActionClasses: []string{"write"}, TargetScope: []string{"repo:a"}, EnvironmentScope: []string{"prod"}, DataClasses: []string{"internal"}, NetworkDestinations: []string{"api.example"}, MaxOperations: 2, MaxTargets: 1, MaxDescendantDepth: 1, ContractDigest: digest, TTL: time.Hour, Now: now, SigningPrivateKey: private, TokenPath: filepath.Join(tokenDir, "delegation.json")})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelegationToken(d.Token, public, DelegationValidationOptions{Now: now, ExpectedContractDigest: digest, RequiredActionClasses: []string{"write"}, RequiredTargetScope: []string{"repo:a"}, OperationCount: 1, TargetCount: 1, DescendantDepth: 1}); err != nil {
		t.Fatalf("exact delegation rejected: %v", err)
	}
	if err := ValidateDelegationToken(d.Token, public, DelegationValidationOptions{Now: now, OperationCount: 3}); err == nil {
		t.Fatal("expanded operation authority accepted")
	}
}
