package sign

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSignVerifyManifestJSON(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	manifest := loadManifestFixture(t)
	sig, err := SignManifestJSON(kp.Private, manifest)
	if err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	ok, err := VerifyManifestJSON(kp.Public, sig, manifest)
	if err != nil {
		t.Fatalf("verify manifest: %v", err)
	}
	if !ok {
		t.Fatalf("expected manifest signature to verify")
	}

	tampered := bytes.Replace(manifest, []byte("run_demo"), []byte("run_other"), 1)
	if _, err := VerifyManifestJSON(kp.Public, sig, tampered); err == nil {
		t.Fatalf("expected tampered manifest to fail")
	}
}

func TestSignVerifyTraceRecordJSON(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	trace := []byte(`{
		"schema_id":"gait.gate.trace",
		"schema_version":"1.0.0",
		"created_at":"2026-02-05T00:00:00Z",
		"producer_version":"0.0.0-dev",
		"trace_id":"trace_1",
		"tool_name":"tool.demo",
		"args_digest":"2222222222222222222222222222222222222222222222222222222222222222",
		"policy_digest":"3333333333333333333333333333333333333333333333333333333333333333",
		"verdict":"allow"
	}`)
	sig, err := SignTraceRecordJSON(kp.Private, trace)
	if err != nil {
		t.Fatalf("sign trace: %v", err)
	}
	ok, err := VerifyTraceRecordJSON(kp.Public, sig, trace)
	if err != nil {
		t.Fatalf("verify trace: %v", err)
	}
	if !ok {
		t.Fatalf("expected trace signature to verify")
	}
}

func TestSignJSONInvalid(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	if _, err := SignJSON(kp.Private, []byte("{")); err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestVerifyJSONMissingDigest(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	input := []byte(`{"foo":"bar"}`)
	sig := Signature{Alg: AlgEd25519, KeyID: KeyID(kp.Public), Sig: "invalid", SignedDigest: ""}
	if _, err := VerifyJSON(kp.Public, sig, input); err == nil {
		t.Fatalf("expected error for missing signed_digest")
	}
}

func TestVerifyJSONDigestMismatch(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	input := []byte(`{"foo":"bar"}`)
	sig := Signature{
		Alg:          AlgEd25519,
		KeyID:        KeyID(kp.Public),
		Sig:          "invalid",
		SignedDigest: "1111111111111111111111111111111111111111111111111111111111111111",
	}
	if _, err := VerifyJSON(kp.Public, sig, input); err == nil {
		t.Fatalf("expected error for signed_digest mismatch")
	}
}

func loadManifestFixture(t *testing.T) []byte {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	path := filepath.Join(root, "core", "schema", "testdata", "manifest_valid.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest fixture: %v", err)
	}
	return data
}
