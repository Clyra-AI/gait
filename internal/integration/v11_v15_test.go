package integration

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/gate"
	"github.com/Clyra-AI/gait/core/guard"
	"github.com/Clyra-AI/gait/core/jcs"
	"github.com/Clyra-AI/gait/core/mcp"
	"github.com/Clyra-AI/gait/core/registry"
	"github.com/Clyra-AI/gait/core/runpack"
	schemaregistry "github.com/Clyra-AI/gait/core/schema/v1/registry"
	"github.com/Clyra-AI/gait/core/schema/validate"
	"github.com/Clyra-AI/gait/core/scout"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestV11ToV15CrossModuleSchemaFlow(t *testing.T) {
	root := repoRootIntegration(t)
	workDir := t.TempDir()
	workspaceDir := filepath.Join(workDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	mustWriteIntegrationFile(t, filepath.Join(workspaceDir, "agent.py"), `
from langchain.tools import tool
from agents import function_tool

@tool
def list_orders():
    return "ok"

@function_tool
def delete_user():
    return "ok"
`)
	mustWriteIntegrationFile(t, filepath.Join(workspaceDir, "mcp.json"), `{"mcpServers":{"db":{"command":"db"}}}`)

	snapshotProvider := scout.DefaultProvider{Options: scout.SnapshotOptions{
		ProducerVersion: "0.0.0-test",
		Now:             time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
	}}
	snapshot, err := snapshotProvider.Snapshot(context.Background(), scout.SnapshotRequest{Roots: []string{workspaceDir}})
	if err != nil {
		t.Fatalf("snapshot workspace: %v", err)
	}
	if len(snapshot.Items) == 0 {
		t.Fatalf("expected scout snapshot to discover inventory items")
	}
	snapshotRaw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := validate.ValidateJSON(filepath.Join(root, "schemas", "v1", "scout", "inventory_snapshot.schema.json"), snapshotRaw); err != nil {
		t.Fatalf("validate scout snapshot schema: %v", err)
	}
	snapshotPath := filepath.Join(workDir, "inventory_snapshot.json")
	if err := os.WriteFile(snapshotPath, snapshotRaw, 0o600); err != nil {
		t.Fatalf("write snapshot file: %v", err)
	}

	policyPath := filepath.Join(workDir, "coverage_policy.yaml")
	mustWriteIntegrationFile(t, policyPath, `default_verdict: allow
rules:
  - name: allow-list-orders
    effect: allow
    match:
      tool_names: [list_orders]
`)
	coverage, err := scout.BuildCoverage(snapshot, []string{policyPath})
	if err != nil {
		t.Fatalf("build coverage: %v", err)
	}
	if coverage.DiscoveredTools == 0 {
		t.Fatalf("expected discovered tools in coverage")
	}

	runpackPath := filepath.Join(workDir, "runpack_cross.zip")
	run, intents, results, refs := integrationRunpackFixture(t)
	recordResult, err := runpack.WriteRunpack(runpackPath, runpack.RecordOptions{
		Run:         run,
		Intents:     intents,
		Results:     results,
		Refs:        refs,
		CaptureMode: "reference",
	})
	if err != nil {
		t.Fatalf("write runpack: %v", err)
	}
	if strings.TrimSpace(recordResult.Manifest.ManifestDigest) == "" {
		t.Fatalf("expected runpack manifest digest")
	}

	gatePolicy, err := gate.ParsePolicyYAML([]byte(`
default_verdict: allow
rules:
  - name: block-external-host
    effect: block
    match:
      tool_names: [tool.write]
      target_kinds: [host]
`))
	if err != nil {
		t.Fatalf("parse gate policy: %v", err)
	}
	mcpResult, err := mcp.EvaluateToolCall(gatePolicy, mcp.ToolCall{
		Name: "tool.write",
		Args: map[string]any{
			"path": "/tmp/out.txt",
		},
		Targets: []mcp.Target{{
			Kind:  "path",
			Value: "/tmp/out.txt",
		}},
		ArgProvenance: []mcp.ArgProvenance{{
			ArgPath: "$.path",
			Source:  "user",
		}},
		Context: mcp.CallContext{
			Identity:  "alice",
			Workspace: "/repo/gait",
			RiskClass: "high",
		},
	}, gate.EvalOptions{ProducerVersion: "0.0.0-test"})
	if err != nil {
		t.Fatalf("evaluate mcp tool call: %v", err)
	}
	if mcpResult.Outcome.Result.Verdict != "allow" {
		t.Fatalf("expected allow verdict for path target, got %s", mcpResult.Outcome.Result.Verdict)
	}

	privateKey := mustGeneratePrivateKey(t)
	tracePath := filepath.Join(workDir, "trace_cross.json")
	traceResult, err := gate.EmitSignedTrace(gatePolicy, mcpResult.Intent, mcpResult.Outcome.Result, gate.EmitTraceOptions{
		ProducerVersion:   "0.0.0-test",
		SigningPrivateKey: privateKey,
		TracePath:         tracePath,
	})
	if err != nil {
		t.Fatalf("emit signed trace: %v", err)
	}
	traceRaw, err := os.ReadFile(traceResult.TracePath)
	if err != nil {
		t.Fatalf("read trace output: %v", err)
	}
	if err := validate.ValidateJSON(filepath.Join(root, "schemas", "v1", "gate", "trace_record.schema.json"), traceRaw); err != nil {
		t.Fatalf("validate gate trace schema: %v", err)
	}
	traceRecord, err := gate.ReadTraceRecord(traceResult.TracePath)
	if err != nil {
		t.Fatalf("read trace record: %v", err)
	}
	verified, err := gate.VerifyTraceRecordSignature(traceRecord, privateKey.Public().(ed25519.PublicKey))
	if err != nil || !verified {
		t.Fatalf("verify trace signature: verified=%t err=%v", verified, err)
	}

	evidencePath := filepath.Join(workDir, "evidence_pack_cross.zip")
	buildResult, err := guard.BuildPack(guard.BuildOptions{
		RunpackPath:     runpackPath,
		OutputPath:      evidencePath,
		InventoryPaths:  []string{snapshotPath},
		TracePaths:      []string{tracePath},
		TemplateID:      "soc2",
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("build guard evidence pack: %v", err)
	}
	verifyPackResult, err := guard.VerifyPack(evidencePath)
	if err != nil {
		t.Fatalf("verify guard evidence pack: %v", err)
	}
	if len(verifyPackResult.MissingFiles) > 0 || len(verifyPackResult.HashMismatches) > 0 {
		t.Fatalf("unexpected guard verify issues: missing=%v mismatches=%v", verifyPackResult.MissingFiles, verifyPackResult.HashMismatches)
	}
	manifestRaw, err := json.Marshal(buildResult.Manifest)
	if err != nil {
		t.Fatalf("marshal guard manifest: %v", err)
	}
	if err := validate.ValidateJSON(filepath.Join(root, "schemas", "v1", "guard", "pack_manifest.schema.json"), manifestRaw); err != nil {
		t.Fatalf("validate guard manifest schema: %v", err)
	}

	incidentPath := filepath.Join(workDir, "incident_pack_cross.zip")
	incidentResult, err := guard.BuildIncidentPack(guard.IncidentPackOptions{
		RunpackPath:     runpackPath,
		OutputPath:      incidentPath,
		Window:          2 * time.Hour,
		TemplateID:      "incident_response",
		ProducerVersion: "0.0.0-test",
	})
	if err != nil {
		t.Fatalf("build incident pack: %v", err)
	}
	if incidentResult.BuildResult.Manifest.IncidentWindow == nil {
		t.Fatalf("expected incident window metadata in incident pack")
	}
	incidentVerify, err := guard.VerifyPack(incidentPath)
	if err != nil {
		t.Fatalf("verify incident pack: %v", err)
	}
	if len(incidentVerify.MissingFiles) > 0 || len(incidentVerify.HashMismatches) > 0 {
		t.Fatalf("unexpected incident verify issues: missing=%v mismatches=%v", incidentVerify.MissingFiles, incidentVerify.HashMismatches)
	}

	registryManifestPath, registryPublicKey := writeIntegrationRegistryManifest(t, workDir)
	registryCacheDir := filepath.Join(workDir, "registry_cache")
	installResult, err := registry.Install(context.Background(), registry.InstallOptions{
		Source:    registryManifestPath,
		CacheDir:  registryCacheDir,
		PublicKey: registryPublicKey,
	})
	if err != nil {
		t.Fatalf("install registry pack: %v", err)
	}
	registryRaw, err := os.ReadFile(installResult.MetadataPath)
	if err != nil {
		t.Fatalf("read installed registry metadata: %v", err)
	}
	if err := validate.ValidateJSON(filepath.Join(root, "schemas", "v1", "registry", "registry_pack.schema.json"), registryRaw); err != nil {
		t.Fatalf("validate registry pack schema: %v", err)
	}
	installedPacks, err := registry.List(registry.ListOptions{CacheDir: registryCacheDir})
	if err != nil {
		t.Fatalf("list installed registry packs: %v", err)
	}
	if len(installedPacks) == 0 {
		t.Fatalf("expected at least one installed registry pack")
	}
	verifyRegistryResult, err := registry.Verify(registry.VerifyOptions{
		MetadataPath: installResult.MetadataPath,
		CacheDir:     registryCacheDir,
		PublicKey:    registryPublicKey,
	})
	if err != nil {
		t.Fatalf("verify installed registry pack: %v", err)
	}
	if !verifyRegistryResult.SignatureVerified || !verifyRegistryResult.PinVerified {
		t.Fatalf("expected registry signature and pin verification to pass: %#v", verifyRegistryResult)
	}
}

func writeIntegrationRegistryManifest(t *testing.T, workDir string) (string, ed25519.PublicKey) {
	t.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-test",
		PackName:        "cross-module-pack",
		PackVersion:     "1.0.0",
		RiskClass:       "high",
		UseCase:         "tool-gating",
		Compatibility:   []string{"gait>=1.0.0"},
		Artifacts: []schemaregistry.PackArtifact{{
			Path:   "policy.yaml",
			SHA256: strings.Repeat("a", 64),
		}},
	}
	digest, err := digestRegistrySignableManifest(manifest)
	if err != nil {
		t.Fatalf("digest registry manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign registry digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}

	path := filepath.Join(workDir, "registry_pack_cross.json")
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal registry manifest: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write registry manifest: %v", err)
	}
	return path, keyPair.Public
}

func digestRegistrySignableManifest(manifest schemaregistry.RegistryPack) (string, error) {
	signable := manifest
	signable.Signatures = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func mustWriteIntegrationFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func repoRootIntegration(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
