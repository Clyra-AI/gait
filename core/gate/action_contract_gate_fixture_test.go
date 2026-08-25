package gate

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

type gateFixtureManifest struct {
	FixtureVersion  string `json:"fixture_version"`
	FixtureTestOnly bool   `json:"fixture_test_only"`
	Quarantine      bool   `json:"quarantine"`
	Authoritative   bool   `json:"authoritative"`
	Files           []struct {
		Path         string `json:"path"`
		SHA256       string `json:"sha256"`
		ExpectedCode string `json:"expected_code"`
		Signed       bool   `json:"signed"`
	} `json:"files"`
}

func TestActionContractGateFixtureCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "action-contract-gate", "v1")
	manifestRaw, err := os.ReadFile(filepath.Join(root, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest gateFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FixtureVersion != "1" || !manifest.FixtureTestOnly || !manifest.Quarantine || manifest.Authoritative || len(manifest.Files) < 15 {
		t.Fatalf("fixture authority/count drift: %+v", manifest)
	}
	keyRaw, err := os.ReadFile(filepath.Join(root, "fixture-signing-key.public.b64"))
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyRaw)))
	if err != nil {
		t.Fatal(err)
	}
	public := ed25519.PublicKey(keyBytes)
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	var rootToken, childToken schemagate.DelegationToken
	for _, file := range manifest.Files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if file.Path == "approval-exact.json" || file.Path == "approval-expired.json" {
			var token schemagate.ApprovalToken
			if err := json.Unmarshal(raw, &token); err != nil {
				t.Fatal(err)
			}
			err = ValidateApprovalToken(token, public, ApprovalValidationOptions{Now: now, ExpectedContractFamilyID: "pacf-fixture", ExpectedContractID: "pac-fixture", ExpectedContractRevision: 1, ExpectedProposalDigest: strings.Repeat("c", 64), ExpectedActivationDigest: strings.Repeat("c", 64), ExpectedTargetScope: []string{"repo:fixture"}, ExpectedEnvironmentScope: []string{"prod"}, ExpectedOutcomeScope: []string{"succeeded"}, ExpectedEffectScope: []string{"validated"}, ExpectedContainmentScope: []string{"completed"}, RequiredScope: []string{"write"}, TargetCount: 1, OperationCount: 1})
			if file.ExpectedCode == "" && err != nil {
				t.Fatalf("exact approval invalid: %v", err)
			}
			if file.ExpectedCode != "" && !strings.Contains(errorCode(err), file.ExpectedCode) {
				t.Fatalf("approval reason=%v want=%s", err, file.ExpectedCode)
			}
			continue
		}
		var token schemagate.DelegationToken
		if err := json.Unmarshal(raw, &token); err != nil {
			t.Fatal(err)
		}
		if file.Path == "delegation-root.json" {
			rootToken = token
			continue
		}
		if file.Path == "delegation-child-tightened.json" {
			childToken = token
			continue
		}
	}
	if rootToken.TokenID == "" || childToken.TokenID == "" {
		t.Fatal("missing delegation root/child")
	}
	if err := ValidateDelegationToken(rootToken, public, DelegationValidationOptions{Now: now, ExpectedContractDigest: strings.Repeat("c", 64), RequiredActionClasses: []string{"write"}, RequiredTargetScope: []string{"repo:fixture"}, RequiredEnvironmentScope: []string{"prod"}, RequiredDataClasses: []string{"internal"}, RequiredNetworkDestinations: []string{"api.example"}, OperationCount: 1, TargetCount: 1, DescendantDepth: 0}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDelegationNonExpansion(rootToken, childToken); err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		if !strings.HasPrefix(file.Path, "invalid/") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		var token schemagate.DelegationToken
		if err := json.Unmarshal(raw, &token); err != nil {
			t.Fatal(err)
		}
		var validateErr error
		if file.Path == "invalid/revoked-ancestor.json" {
			validateErr = ValidateDelegationToken(token, public, DelegationValidationOptions{Now: now})
		} else {
			validateErr = ValidateDelegationNonExpansion(rootToken, token)
		}
		if !strings.Contains(errorCode(validateErr), file.ExpectedCode) {
			t.Fatalf("%s reason=%v want=%s", file.Path, validateErr, file.ExpectedCode)
		}
	}
}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var approval *ApprovalTokenError
	if errors.As(err, &approval) {
		return approval.Code
	}
	var delegation *DelegationTokenError
	if errors.As(err, &delegation) {
		return delegation.Code
	}
	return err.Error()
}
