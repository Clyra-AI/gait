package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	proofsign "github.com/Clyra-AI/proof/signing"
)

const (
	rootDir          = "testdata/action-contract-gate/v1"
	seedPhrase       = "gait-action-contract-gate-fixture-v1"
	baseCommit       = "eb4c599a5c1a24dbb270c39a5a513d78f253506d"
	approvalSchema   = "https://gait.dev/schemas/v1/gate/approval_token.schema.json"
	delegationSchema = "https://gait.dev/schemas/v1/gate/delegation_token.schema.json"
)

type manifest struct {
	FixtureVersion         string      `json:"fixture_version"`
	FixtureTestOnly        bool        `json:"fixture_test_only"`
	Quarantine             bool        `json:"quarantine"`
	Authoritative          bool        `json:"authoritative"`
	BaseCommit             string      `json:"base_commit"`
	GeneratorSHA256        string      `json:"generator_sha256"`
	ApprovalSchemaSHA256   string      `json:"approval_schema_sha256"`
	DelegationSchemaSHA256 string      `json:"delegation_schema_sha256"`
	PublicKeySHA256        string      `json:"public_key_sha256"`
	Files                  []fileEntry `json:"files"`
}
type fileEntry struct {
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
	ExpectedCode string `json:"expected_code,omitempty"`
	Signed       bool   `json:"signed"`
}

func main() {
	update := flag.Bool("update", false, "write deterministic fixture corpus")
	check := flag.Bool("check", false, "check exact bytes and orphans")
	flag.Parse()
	if *update == *check {
		fatal("exactly one of --update or --check is required")
	}
	if err := run(*update); err != nil {
		fatal("%v", err)
	}
}
func fatal(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
func digest(raw []byte) string         { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }
func fixedKey() ed25519.PrivateKey {
	s := sha256.Sum256([]byte(seedPhrase))
	return ed25519.NewKeyFromSeed(s[:])
}
func fixedDigest(ch byte) string { return strings.Repeat(string(ch), 64) }

func run(update bool) error {
	key := fixedKey()
	pub := key.Public().(ed25519.PublicKey)
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	intent, policy, contract := fixedDigest('a'), fixedDigest('b'), fixedDigest('c')
	approval, err := gate.MintApprovalToken(gate.MintApprovalTokenOptions{ProducerVersion: "v1.5.0", ApproverIdentity: "owner@example", ReasonCode: "action_contract", IntentDigest: intent, PolicyDigest: policy, Scope: []string{"write"}, MaxTargets: 2, MaxOps: 4, TTL: time.Hour, Now: now, SigningPrivateKey: key, TokenPath: filepath.Join(os.TempDir(), "gait-approval-unused.json"), ContractFamilyID: "pacf-fixture", ContractID: "pac-fixture", ContractRevision: 1, ProposalDigest: contract, ActivationDigest: contract, TargetScope: []string{"repo:fixture"}, EnvironmentScope: []string{"prod"}, OutcomeScope: []string{"succeeded"}, EffectScope: []string{"validated"}, ContainmentScope: []string{"completed"}})
	if err != nil {
		return err
	}
	root, err := gate.MintDelegationToken(gate.MintDelegationTokenOptions{ProducerVersion: "v1.5.0", DelegatorIdentity: "root@example", DelegateIdentity: "child@example", Scope: []string{"write"}, ActionClasses: []string{"write"}, TargetScope: []string{"repo:fixture"}, EnvironmentScope: []string{"prod"}, DataClasses: []string{"internal"}, NetworkDestinations: []string{"api.example"}, MaxOperations: 4, MaxTargets: 2, MaxDescendantDepth: 2, ContractDigest: contract, IntentDigest: intent, PolicyDigest: policy, OriginAuthorityDigest: fixedDigest('d'), TTL: 2 * time.Hour, Now: now, SigningPrivateKey: key, TokenPath: filepath.Join(os.TempDir(), "gait-delegation-unused.json")})
	if err != nil {
		return fmt.Errorf("mint root delegation: %w", err)
	}
	if err != nil {
		return err
	}
	rootDigest, err := gate.DelegationTokenDigest(root.Token)
	if err != nil {
		return err
	}
	child, err := gate.MintDelegationToken(gate.MintDelegationTokenOptions{ProducerVersion: "v1.5.0", DelegatorIdentity: "child@example", DelegateIdentity: "leaf@example", Scope: []string{"write"}, ActionClasses: []string{"write"}, TargetScope: []string{"repo:fixture"}, EnvironmentScope: []string{"prod"}, DataClasses: []string{"internal"}, NetworkDestinations: []string{"api.example"}, MaxOperations: 2, MaxTargets: 1, MaxDescendantDepth: 1, ContractDigest: contract, IntentDigest: intent, PolicyDigest: policy, OriginAuthorityDigest: root.Token.OriginAuthorityDigest, ParentTokenID: root.Token.TokenID, ParentTokenDigest: rootDigest, Depth: 1, TTL: time.Hour, Now: now, SigningPrivateKey: key, TokenPath: filepath.Join(os.TempDir(), "gait-child-unused.json")})
	if err != nil {
		return err
	}
	files := map[string]any{"approval-exact.json": approval.Token, "approval-expired.json": resignApproval(expireApproval(approval.Token, now), key), "delegation-root.json": root.Token, "delegation-child-tightened.json": child.Token}
	invalid := map[string]struct {
		token schemagate.DelegationToken
		code  string
	}{
		"invalid/action-expansion.json":       {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.ActionClasses = []string{"delete"} }), "delegation_scope_expanded"},
		"invalid/target-expansion.json":       {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.TargetScope = []string{"repo:other"} }), "delegation_scope_expanded"},
		"invalid/environment-expansion.json":  {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.EnvironmentScope = []string{"dev"} }), "delegation_scope_expanded"},
		"invalid/data-expansion.json":         {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.DataClasses = []string{"secret"} }), "delegation_scope_expanded"},
		"invalid/network-expansion.json":      {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.NetworkDestinations = []string{"evil.example"} }), "delegation_scope_expanded"},
		"invalid/max-ops-expansion.json":      {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.MaxOperations = 5 }), "delegation_authority_expanded"},
		"invalid/max-targets-expansion.json":  {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.MaxTargets = 3 }), "delegation_authority_expanded"},
		"invalid/max-depth-expansion.json":    {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.MaxDescendantDepth = 3 }), "delegation_authority_expanded"},
		"invalid/ttl-expansion.json":          {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.ExpiresAt = root.Token.ExpiresAt.Add(time.Minute) }), "delegation_authority_expanded"},
		"invalid/wrong-parent-digest.json":    {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.ParentTokenDigest = "sha256:" + fixedDigest('e') }), "delegation_parent_digest_mismatch"},
		"invalid/wrong-origin-authority.json": {mutateChild(child.Token, func(t *schemagate.DelegationToken) { t.OriginAuthorityDigest = fixedDigest('f') }), "delegation_inherited_binding_mismatch"},
	}
	revoked := root.Token
	revoked.Revoked = true
	files["invalid/revoked-ancestor.json"] = resignDelegation(revoked, key)
	for path, item := range invalid {
		files[path] = resignDelegation(item.token, key)
	}
	codes := map[string]string{"approval-expired.json": gate.ApprovalCodeExpired, "invalid/revoked-ancestor.json": gate.DelegationCodeChainMismatch}
	for path, item := range invalid {
		codes[path] = item.code
	}
	return writeOrCheck(update, files, codes, pub, approval.Token, root.Token, child.Token, rootDigest)
}

func mutateChild(in schemagate.DelegationToken, mutate func(*schemagate.DelegationToken)) schemagate.DelegationToken {
	out := in
	mutate(&out)
	return out
}
func expireApproval(in schemagate.ApprovalToken, now time.Time) schemagate.ApprovalToken {
	out := in
	out.ExpiresAt = now.Add(-time.Minute)
	return out
}
func resignApproval(token schemagate.ApprovalToken, key ed25519.PrivateKey) schemagate.ApprovalToken {
	token.Signature = nil
	raw, _ := json.Marshal(token)
	sig, _ := proofsign.SignJSON(key, raw)
	token.Signature = &schemagate.Signature{Alg: sig.Alg, KeyID: sig.KeyID, Sig: sig.Sig, SignedDigest: sig.SignedDigest}
	return token
}
func resignDelegation(token schemagate.DelegationToken, key ed25519.PrivateKey) schemagate.DelegationToken {
	token.Signature = nil
	raw, _ := json.Marshal(token)
	sig, _ := proofsign.SignJSON(key, raw)
	token.Signature = &schemagate.Signature{Alg: sig.Alg, KeyID: sig.KeyID, Sig: sig.Sig, SignedDigest: sig.SignedDigest}
	return token
}
func writeOrCheck(update bool, items map[string]any, codes map[string]string, pub ed25519.PublicKey, approval schemagate.ApprovalToken, root, child schemagate.DelegationToken, rootDigest string) error {
	if update {
		if err := os.MkdirAll(rootDir, 0750); err != nil {
			return err
		}
	}
	entries := []fileEntry{}
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, path := range keys {
		raw, err := json.MarshalIndent(items[path], "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		entries = append(entries, fileEntry{Path: path, SHA256: digest(raw), ExpectedCode: codes[path], Signed: true})
		if update {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(rootDir, path)), 0750); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(rootDir, path), raw, 0600); err != nil {
				return err
			}
		} else {
			got, err := os.ReadFile(filepath.Join(rootDir, path))
			if err != nil || string(got) != string(raw) {
				return fmt.Errorf("fixture drift: %s", path)
			}
		}
	}
	pubRaw := []byte(base64.StdEncoding.EncodeToString(pub) + "\n")
	manifest := manifest{FixtureVersion: "1", FixtureTestOnly: true, Quarantine: true, Authoritative: false, BaseCommit: baseCommit, GeneratorSHA256: "", PublicKeySHA256: digest(pubRaw), Files: entries}
	genRaw, _ := os.ReadFile("scripts/action_contract_gate_fixture_generator/main.go")
	manifest.GeneratorSHA256 = digest(genRaw)
	approvalSchemaRaw, _ := os.ReadFile("schemas/v1/gate/approval_token.schema.json")
	delegationSchemaRaw, _ := os.ReadFile("schemas/v1/gate/delegation_token.schema.json")
	manifest.ApprovalSchemaSHA256, manifest.DelegationSchemaSHA256 = digest(approvalSchemaRaw), digest(delegationSchemaRaw)
	if update {
		if err := os.WriteFile(filepath.Join(rootDir, "fixture-signing-key.public.b64"), pubRaw, 0600); err != nil {
			return err
		}
	} else {
		got, err := os.ReadFile(filepath.Join(rootDir, "fixture-signing-key.public.b64"))
		if err != nil || string(got) != string(pubRaw) {
			return fmt.Errorf("public key drift")
		}
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	raw = append(raw, '\n')
	if update {
		if err := os.WriteFile(filepath.Join(rootDir, "fixture-manifest.json"), raw, 0600); err != nil {
			return err
		}
	} else {
		got, err := os.ReadFile(filepath.Join(rootDir, "fixture-manifest.json"))
		if err != nil || string(got) != string(raw) {
			return fmt.Errorf("manifest drift")
		}
	}
	allowed := map[string]bool{"fixture-manifest.json": true, "fixture-signing-key.public.b64": true}
	for path := range items {
		allowed[path] = true
	}
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		if !allowed[filepath.ToSlash(rel)] {
			return fmt.Errorf("fixture orphan: %s", rel)
		}
		return nil
	})
}
