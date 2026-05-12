package pack

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemapack "github.com/Clyra-AI/gait/core/schema/v1/pack"
)

func TestBuildAuthorizationPackVerifyInspectAndDeterminism(t *testing.T) {
	workDir := t.TempDir()
	specPath := writeAuthorizationFixture(t, workDir, "allow")

	firstPath := filepath.Join(workDir, "authorization_first.zip")
	secondPath := filepath.Join(workDir, "authorization_second.zip")

	first, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        firstPath,
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build first authorization pack: %v", err)
	}
	if first.Manifest.PackType != string(BuildTypeAuthorization) {
		t.Fatalf("unexpected authorization pack type: %s", first.Manifest.PackType)
	}
	verifyResult, err := Verify(first.Path, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify authorization pack: %v", err)
	}
	if verifyResult.PackType != string(BuildTypeAuthorization) {
		t.Fatalf("unexpected verify result: %#v", verifyResult)
	}
	inspectResult, err := Inspect(first.Path)
	if err != nil {
		t.Fatalf("inspect authorization pack: %v", err)
	}
	if inspectResult.AuthorizationBundle == nil || inspectResult.AuthorizationMeta == nil {
		t.Fatalf("expected authorization inspect payloads, got %#v", inspectResult)
	}
	if inspectResult.AuthorizationBundle.OutcomeStatus != "allow" || inspectResult.AuthorizationMeta.LinkedEvidenceCount < 3 {
		t.Fatalf("unexpected authorization inspect result: %#v", inspectResult)
	}

	second, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        secondPath,
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build second authorization pack: %v", err)
	}
	firstBytes, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("read first authorization pack: %v", err)
	}
	secondBytes, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatalf("read second authorization pack: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("expected deterministic authorization pack bytes")
	}
}

func TestAuthorizationPackTamperAndMissingEvidenceFailVerify(t *testing.T) {
	workDir := t.TempDir()
	specPath := writeAuthorizationFixture(t, workDir, "block")

	result, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        filepath.Join(workDir, "authorization.zip"),
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build authorization pack: %v", err)
	}

	tamperedPath := filepath.Join(workDir, "authorization_tampered.zip")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, tamperedPath, map[string][]byte{
		filepath.ToSlash(filepath.Join("evidence", "trace.json")): []byte(`{"tampered":true}`),
	}); err != nil {
		t.Fatalf("rewrite tampered authorization pack: %v", err)
	}
	if _, err := Verify(tamperedPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected tampered authorization evidence to fail verification")
	}

	missingPath := filepath.Join(workDir, "authorization_missing.zip")
	if err := rewriteZip(result.Path, missingPath, func(name string, payload []byte) (string, []byte) {
		if name == filepath.ToSlash(filepath.Join("evidence", "trace.json")) {
			return "", nil
		}
		return name, payload
	}, nil); err != nil {
		t.Fatalf("rewrite missing authorization evidence pack: %v", err)
	}
	missingVerify, err := Verify(missingPath, VerifyOptions{})
	if err != nil {
		t.Fatalf("verify missing authorization evidence pack: %v", err)
	}
	if len(missingVerify.MissingFiles) == 0 {
		t.Fatalf("expected missing authorization evidence to be reported, got %#v", missingVerify)
	}
}

func TestNormalizeAuthorizationBundleValidationAndHelpers(t *testing.T) {
	workDir := t.TempDir()
	writeAuthorizationJSON(t, filepath.Join(workDir, "trace.json"), map[string]any{
		"schema_id":        "gait.gate.trace",
		"schema_version":   "1.0.0",
		"created_at":       "2026-05-09T00:00:00Z",
		"producer_version": "test",
		"trace_id":         "trace-auth-demo",
		"tool_name":        "tool.write",
		"args_digest":      stringsRepeat("c", 64),
		"intent_digest":    stringsRepeat("b", 64),
		"policy_digest":    stringsRepeat("a", 64),
		"verdict":          "allow",
	})
	writeAuthorizationJSON(t, filepath.Join(workDir, "outcome_receipt.json"), map[string]any{
		"status":   "allow",
		"executed": true,
	})

	validPayload := []byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"` + stringsRepeat("a", 64) + `",
  "intent_digest":"` + stringsRepeat("b", 64) + `",
  "trace_path":"trace.json",
  "outcome_path":"outcome_receipt.json",
  "outcome_status":"allow"
}`)
	bundle, err := normalizeAuthorizationBundle(validPayload, workDir, "test")
	if err != nil {
		t.Fatalf("normalize authorization bundle: %v", err)
	}
	if bundle.TraceDigest == "" || bundle.OutcomeDigest == "" {
		t.Fatalf("expected computed digests in normalized bundle: %#v", bundle)
	}

	if _, err := normalizeAuthorizationBundle([]byte(`{
  "schema_id":"bad",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"`+stringsRepeat("a", 64)+`",
  "intent_digest":"`+stringsRepeat("b", 64)+`"
}`), workDir, "test"); err == nil {
		t.Fatalf("expected invalid schema error")
	}
	if _, err := normalizeAuthorizationBundle([]byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"`+stringsRepeat("a", 64)+`",
  "intent_digest":"`+stringsRepeat("b", 64)+`",
  "trace_path":"../trace.json"
}`), workDir, "test"); err == nil {
		t.Fatalf("expected relative path traversal rejection")
	}
	if _, err := normalizeAuthorizationBundle([]byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"`+stringsRepeat("a", 64)+`",
  "intent_digest":"`+stringsRepeat("b", 64)+`",
  "trace_path":"trace.json",
  "outcome_path":"outcome_receipt.json",
  "outcome_status":"mystery"
}`), workDir, "test"); err == nil {
		t.Fatalf("expected invalid outcome_status rejection")
	}

	jsonDigest, err := digestAuthorizationEvidence("trace.json", []byte(`{"ok":true}`))
	if err != nil || jsonDigest == "" {
		t.Fatalf("expected json digest, got digest=%q err=%v", jsonDigest, err)
	}
	binaryDigest, err := digestAuthorizationEvidence("trace.bin", []byte("payload"))
	if err != nil || binaryDigest == "" {
		t.Fatalf("expected binary digest, got digest=%q err=%v", binaryDigest, err)
	}
}

func TestBuildAuthorizationPackRejectsMissingInput(t *testing.T) {
	if _, err := BuildAuthorizationPack(BuildAuthorizationOptions{}); err == nil {
		t.Fatalf("expected missing authorization bundle path error")
	}
	if _, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: filepath.Join(t.TempDir(), "missing.json"),
		SigningPrivateKey: ed25519.PrivateKey{},
	}); err == nil {
		t.Fatalf("expected missing file error")
	}
}

func TestNormalizeAuthorizationLinkAndBundleFilesErrors(t *testing.T) {
	workDir := t.TempDir()
	writeAuthorizationJSON(t, filepath.Join(workDir, "trace.json"), map[string]any{"ok": true})

	if _, _, err := normalizeAuthorizationLink("../trace.json", "", workDir); err == nil {
		t.Fatalf("expected traversal rejection")
	}
	if _, _, err := normalizeAuthorizationLink("trace.json", stringsRepeat("a", 64), workDir); err == nil {
		t.Fatalf("expected digest mismatch rejection")
	}

	bundle := schemagate.AuthorizationBundle{
		TracePath:    "trace.json",
		OutcomePath:  "trace.json",
		PolicyDigest: stringsRepeat("a", 64),
		IntentDigest: stringsRepeat("b", 64),
		TraceID:      "trace-auth-demo",
	}
	files, err := authorizationBundleFiles(bundle, workDir)
	if err != nil {
		t.Fatalf("authorizationBundleFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected deduped evidence files, got %#v", files)
	}
}

func TestVerifyAuthorizationPayloadContractsMissingOutcomeReceipt(t *testing.T) {
	workDir := t.TempDir()
	specPath := writeAuthorizationFixture(t, workDir, "allow")
	result, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        filepath.Join(workDir, "authorization.zip"),
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build authorization pack: %v", err)
	}
	mutatedPath := filepath.Join(workDir, "authorization_invalid_outcome.zip")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, mutatedPath, map[string][]byte{
		"authorization_bundle.json": []byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"` + stringsRepeat("a", 64) + `",
  "intent_digest":"` + stringsRepeat("b", 64) + `",
  "trace_path":"trace.json",
  "trace_digest":"` + stringsRepeat("c", 64) + `",
  "outcome_status":"allow"
}`),
	}); err != nil {
		t.Fatalf("rewrite invalid authorization pack: %v", err)
	}
	if _, err := Verify(mutatedPath, VerifyOptions{}); err == nil {
		t.Fatalf("expected missing outcome receipt linkage to fail verification")
	}
}

func TestAuthorizationPackVerifyRejectsMissingRequiredFields(t *testing.T) {
	workDir := t.TempDir()
	specPath := writeAuthorizationFixture(t, workDir, "allow")
	result, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        filepath.Join(workDir, "authorization.zip"),
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build authorization pack: %v", err)
	}

	missingTracePath := filepath.Join(workDir, "authorization_missing_trace_path.zip")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, missingTracePath, map[string][]byte{
		"authorization_bundle.json": []byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"` + stringsRepeat("a", 64) + `",
  "intent_digest":"` + stringsRepeat("b", 64) + `",
  "outcome_path":"outcome_receipt.json",
  "outcome_digest":"` + stringsRepeat("e", 64) + `",
  "outcome_status":"allow"
}`),
	}); err != nil {
		t.Fatalf("rewrite missing trace path bundle: %v", err)
	}
	if _, err := Verify(missingTracePath, VerifyOptions{}); err == nil {
		t.Fatalf("expected missing trace path/digest to fail verification")
	}

	missingOutcomePath := filepath.Join(workDir, "authorization_missing_outcome_path.zip")
	if err := rewritePackWithMutatedPayloadAndManifest(result.Path, missingOutcomePath, map[string][]byte{
		"authorization_bundle.json": []byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "producer_version":"test",
  "trace_id":"trace-auth-demo",
  "policy_digest":"` + stringsRepeat("a", 64) + `",
  "intent_digest":"` + stringsRepeat("b", 64) + `",
  "trace_path":"trace.json",
  "trace_digest":"` + stringsRepeat("c", 64) + `",
  "outcome_status":"allow"
}`),
	}); err != nil {
		t.Fatalf("rewrite missing outcome path bundle: %v", err)
	}
	if _, err := Verify(missingOutcomePath, VerifyOptions{}); err == nil {
		t.Fatalf("expected missing outcome path/digest to fail verification")
	}
}

func TestVerifyAuthorizationPayloadContractsDirect(t *testing.T) {
	workDir := t.TempDir()
	specPath := writeAuthorizationFixture(t, workDir, "allow")
	result, err := BuildAuthorizationPack(BuildAuthorizationOptions{
		AuthorizationPath: specPath,
		OutputPath:        filepath.Join(workDir, "authorization.zip"),
		ProducerVersion:   "test",
	})
	if err != nil {
		t.Fatalf("build authorization pack: %v", err)
	}

	bundle, manifest := openPackBundleAndManifest(t, result.Path)
	defer func() { _ = bundle.Close() }()
	if err := verifyAuthorizationPayloadContracts(bundle, manifest); err != nil {
		t.Fatalf("verifyAuthorizationPayloadContracts expected success: %v", err)
	}

	delete(bundle.Files, authorizationEvidenceArchivePath("trace.json"))
	if err := verifyAuthorizationPayloadContracts(bundle, manifest); err == nil {
		t.Fatalf("expected missing trace evidence to fail direct contract verification")
	}
}

func TestAuthorizationEvidenceArchivePathCanonicalizesSeparators(t *testing.T) {
	if got := authorizationEvidenceArchivePath("trace.json"); got != "evidence/trace.json" {
		t.Fatalf("authorizationEvidenceArchivePath trace.json = %q", got)
	}
	if got := authorizationEvidenceArchivePath(`nested\trace.json`); got != "evidence/nested/trace.json" {
		t.Fatalf("authorizationEvidenceArchivePath backslash path = %q", got)
	}
}

func TestNormalizeAuthorizationBundleDefaultsProducerVersion(t *testing.T) {
	workDir := t.TempDir()
	writeAuthorizationJSON(t, filepath.Join(workDir, "trace.json"), map[string]any{"ok": true})

	payload := []byte(`{
  "schema_id":"gait.gate.authorization_bundle",
  "schema_version":"1.0.0",
  "created_at":"2026-05-09T00:00:00Z",
  "trace_id":"trace-auth-demo",
  "policy_digest":"` + stringsRepeat("a", 64) + `",
  "intent_digest":"` + stringsRepeat("b", 64) + `",
  "trace_path":"trace.json"
}`)
	bundle, err := normalizeAuthorizationBundle(payload, workDir, "fallback-version")
	if err != nil {
		t.Fatalf("normalize bundle with fallback producer version: %v", err)
	}
	if bundle.ProducerVersion != "fallback-version" {
		t.Fatalf("expected fallback producer version, got %#v", bundle)
	}
}

func TestAuthorizationInspectAuthorizationMeta(t *testing.T) {
	payload := schemagate.AuthorizationBundle{
		CreatedAt:         time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
		TraceID:           "trace-auth-demo",
		PolicyDigest:      stringsRepeat("a", 64),
		IntentDigest:      stringsRepeat("b", 64),
		OutcomeStatus:     "block",
		TracePath:         "trace.json",
		ApprovalAuditPath: "approval_audit.json",
	}
	meta := schemapack.AuthorizationPayload{
		SchemaID:              "gait.pack.authorization",
		SchemaVersion:         "1.0.0",
		CreatedAt:             payload.CreatedAt,
		TraceID:               payload.TraceID,
		PolicyDigest:          payload.PolicyDigest,
		IntentDigest:          payload.IntentDigest,
		OutcomeStatus:         payload.OutcomeStatus,
		LinkedEvidenceCount:   countAuthorizationLinkedEvidence(payload),
		RequiredEvidenceCount: countAuthorizationRequiredEvidence(payload),
	}
	if meta.LinkedEvidenceCount != 2 || meta.RequiredEvidenceCount != 0 {
		t.Fatalf("unexpected authorization meta counts: %#v", meta)
	}
}

func writeAuthorizationFixture(t *testing.T, dir string, outcomeStatus string) string {
	t.Helper()
	tracePath := filepath.Join(dir, "trace.json")
	approvalPath := filepath.Join(dir, "approval_audit.json")
	credentialPath := filepath.Join(dir, "credential_evidence.json")
	outcomePath := filepath.Join(dir, "outcome_receipt.json")
	tracePayload := map[string]any{
		"schema_id":        "gait.gate.trace",
		"schema_version":   "1.0.0",
		"trace_id":         "trace-auth-demo",
		"policy_digest":    stringsRepeat("a", 64),
		"intent_digest":    stringsRepeat("b", 64),
		"tool_name":        "tool.write",
		"args_digest":      stringsRepeat("c", 64),
		"verdict":          outcomeStatus,
		"created_at":       time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"producer_version": "test",
	}
	approvalPayload := map[string]any{"approved": outcomeStatus == "allow", "trace_id": "trace-auth-demo"}
	credentialPayload := map[string]any{"credential_ref": "stub:1234", "trace_id": "trace-auth-demo"}
	outcomePayload := map[string]any{"status": outcomeStatus, "executed": outcomeStatus == "allow", "trace_id": "trace-auth-demo"}
	writeAuthorizationJSON(t, tracePath, tracePayload)
	writeAuthorizationJSON(t, approvalPath, approvalPayload)
	writeAuthorizationJSON(t, credentialPath, credentialPayload)
	writeAuthorizationJSON(t, outcomePath, outcomePayload)

	spec := schemagate.AuthorizationBundle{
		SchemaID:               "gait.gate.authorization_bundle",
		SchemaVersion:          "1.0.0",
		CreatedAt:              time.Date(2026, time.May, 9, 0, 0, 0, 0, time.UTC),
		ProducerVersion:        "test",
		TraceID:                "trace-auth-demo",
		PolicyDigest:           stringsRepeat("a", 64),
		IntentDigest:           stringsRepeat("b", 64),
		TracePath:              "trace.json",
		ApprovalAuditPath:      "approval_audit.json",
		CredentialEvidencePath: "credential_evidence.json",
		OutcomePath:            "outcome_receipt.json",
		OutcomeStatus:          outcomeStatus,
		Sandbox:                &schemagate.SandboxDecision{Status: "valid", EvidenceDigest: stringsRepeat("d", 64)},
		KillSwitch:             &schemagate.KillSwitchDecision{Status: "inactive"},
		FreezeWindow:           &schemagate.FreezeWindowDecision{Status: "inactive"},
	}
	specPath := filepath.Join(dir, "authorization_bundle.json")
	raw, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("marshal authorization spec: %v", err)
	}
	if err := os.WriteFile(specPath, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write authorization spec: %v", err)
	}
	return specPath
}

func writeAuthorizationJSON(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal authorization json: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write authorization json: %v", err)
	}
}

func stringsRepeat(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}
