package e2e

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	"github.com/Clyra-AI/gait/core/sign"
)

func TestCLIV11ToV15Scenarios(t *testing.T) {
	root := repoRoot(t)
	binPath := buildGaitBinary(t, root)
	workDir := t.TempDir()

	// Scout snapshot + diff.
	agentPath := filepath.Join(workDir, "agent.py")
	mustWriteE2EFile(t, agentPath, `
from langchain.tools import tool

@tool
def list_orders():
    return "ok"
`)
	mcpConfigPath := filepath.Join(workDir, "mcp.json")
	mustWriteE2EFile(t, mcpConfigPath, `{"mcpServers":{"db":{"command":"db"}}}`)

	snapshotAPath := filepath.Join(workDir, "snapshot_a.json")
	snapshotBPath := filepath.Join(workDir, "snapshot_b.json")
	diffPath := filepath.Join(workDir, "snapshot_diff.json")
	scoutAOut := runJSONCommand(t, workDir, binPath, "scout", "snapshot", "--roots", workDir, "--out", snapshotAPath, "--json")
	var scoutA struct {
		OK    bool `json:"ok"`
		Items int  `json:"items"`
	}
	if err := json.Unmarshal(scoutAOut, &scoutA); err != nil {
		t.Fatalf("parse scout snapshot A output: %v\n%s", err, string(scoutAOut))
	}
	if !scoutA.OK || scoutA.Items == 0 {
		t.Fatalf("unexpected scout snapshot A output: %s", string(scoutAOut))
	}

	mustWriteE2EFile(t, agentPath, `
from langchain.tools import tool
from agents import function_tool

@tool
def list_orders():
    return "ok"

@function_tool
def delete_user():
    return "ok"
`)
	scoutBOut := runJSONCommand(t, workDir, binPath, "scout", "snapshot", "--roots", workDir, "--out", snapshotBPath, "--json")
	var scoutB struct {
		OK    bool `json:"ok"`
		Items int  `json:"items"`
	}
	if err := json.Unmarshal(scoutBOut, &scoutB); err != nil {
		t.Fatalf("parse scout snapshot B output: %v\n%s", err, string(scoutBOut))
	}
	if !scoutB.OK || scoutB.Items <= scoutA.Items {
		t.Fatalf("expected snapshot B to discover more items: A=%d B=%d", scoutA.Items, scoutB.Items)
	}

	diffOut := runJSONCommandExpectCode(t, workDir, binPath, 2, "scout", "diff", snapshotAPath, snapshotBPath, "--out", diffPath, "--json")
	var scoutDiff struct {
		OK   bool `json:"ok"`
		Diff struct {
			AddedCount   int `json:"added_count"`
			RemovedCount int `json:"removed_count"`
			ChangedCount int `json:"changed_count"`
		} `json:"diff"`
	}
	if err := json.Unmarshal(diffOut, &scoutDiff); err != nil {
		t.Fatalf("parse scout diff output: %v\n%s", err, string(diffOut))
	}
	if scoutDiff.OK || (scoutDiff.Diff.AddedCount+scoutDiff.Diff.RemovedCount+scoutDiff.Diff.ChangedCount) == 0 {
		t.Fatalf("expected scout diff to report change: %s", string(diffOut))
	}

	// Guard verify + incident pack generation.
	demo := exec.Command(binPath, "demo")
	demo.Dir = workDir
	if out, err := demo.CombinedOutput(); err != nil {
		t.Fatalf("run demo: %v\n%s", err, string(out))
	}
	evidencePackPath := filepath.Join(workDir, "evidence_pack_e2e.zip")
	guardPackOut := runJSONCommand(t, workDir, binPath,
		"guard", "pack",
		"--run", "run_demo",
		"--inventory", snapshotBPath,
		"--out", evidencePackPath,
		"--json",
	)
	var guardPack struct {
		OK       bool   `json:"ok"`
		PackPath string `json:"pack_path"`
	}
	if err := json.Unmarshal(guardPackOut, &guardPack); err != nil {
		t.Fatalf("parse guard pack output: %v\n%s", err, string(guardPackOut))
	}
	if !guardPack.OK || guardPack.PackPath == "" {
		t.Fatalf("unexpected guard pack output: %s", string(guardPackOut))
	}
	guardVerifyOut := runJSONCommand(t, workDir, binPath, "guard", "verify", guardPack.PackPath, "--json")
	var guardVerify struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(guardVerifyOut, &guardVerify); err != nil {
		t.Fatalf("parse guard verify output: %v\n%s", err, string(guardVerifyOut))
	}
	if !guardVerify.OK {
		t.Fatalf("guard verify expected ok: %s", string(guardVerifyOut))
	}

	incidentPackPath := filepath.Join(workDir, "incident_pack_e2e.zip")
	incidentOut := runJSONCommand(t, workDir, binPath,
		"incident", "pack",
		"--from", "run_demo",
		"--window", "1h",
		"--out", incidentPackPath,
		"--json",
	)
	var incidentResult struct {
		OK       bool   `json:"ok"`
		PackPath string `json:"pack_path"`
	}
	if err := json.Unmarshal(incidentOut, &incidentResult); err != nil {
		t.Fatalf("parse incident output: %v\n%s", err, string(incidentOut))
	}
	if !incidentResult.OK || incidentResult.PackPath == "" {
		t.Fatalf("unexpected incident output: %s", string(incidentOut))
	}
	if _, err := os.Stat(incidentResult.PackPath); err != nil {
		t.Fatalf("incident pack path missing: %v", err)
	}

	// Registry install/list/verify.
	registryManifestPath, publicKeyPath, cacheDir := writeSignedRegistryManifest(t, workDir)
	registryInstallOut := runJSONCommand(t, workDir, binPath,
		"registry", "install",
		"--source", registryManifestPath,
		"--cache-dir", cacheDir,
		"--public-key", publicKeyPath,
		"--json",
	)
	var registryInstall struct {
		OK           bool   `json:"ok"`
		MetadataPath string `json:"metadata_path"`
	}
	if err := json.Unmarshal(registryInstallOut, &registryInstall); err != nil {
		t.Fatalf("parse registry install output: %v\n%s", err, string(registryInstallOut))
	}
	if !registryInstall.OK || registryInstall.MetadataPath == "" {
		t.Fatalf("unexpected registry install output: %s", string(registryInstallOut))
	}

	registryListOut := runJSONCommand(t, workDir, binPath, "registry", "list", "--cache-dir", cacheDir, "--json")
	var registryList struct {
		OK    bool `json:"ok"`
		Packs []struct {
			PackName string `json:"pack_name"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(registryListOut, &registryList); err != nil {
		t.Fatalf("parse registry list output: %v\n%s", err, string(registryListOut))
	}
	if !registryList.OK || len(registryList.Packs) == 0 {
		t.Fatalf("unexpected registry list output: %s", string(registryListOut))
	}

	registryVerifyOut := runJSONCommand(t, workDir, binPath,
		"registry", "verify",
		"--path", registryInstall.MetadataPath,
		"--cache-dir", cacheDir,
		"--public-key", publicKeyPath,
		"--json",
	)
	var registryVerify struct {
		OK                bool `json:"ok"`
		PinVerified       bool `json:"pin_verified"`
		SignatureVerified bool `json:"signature_verified"`
	}
	if err := json.Unmarshal(registryVerifyOut, &registryVerify); err != nil {
		t.Fatalf("parse registry verify output: %v\n%s", err, string(registryVerifyOut))
	}
	if !registryVerify.OK || !registryVerify.PinVerified || !registryVerify.SignatureVerified {
		t.Fatalf("unexpected registry verify output: %s", string(registryVerifyOut))
	}

	// MCP bridge/proxy.
	policyPath := filepath.Join(workDir, "mcp_policy.yaml")
	mustWriteE2EFile(t, policyPath, "default_verdict: allow\n")

	mcpCallPath := filepath.Join(workDir, "mcp_call.json")
	mustWriteE2EFile(t, mcpCallPath, `{"name":"tool.write","args":{"path":"/tmp/out.txt"},"targets":[{"kind":"path","value":"/tmp/out.txt"}],"arg_provenance":[{"arg_path":"$.path","source":"user"}],"context":{"identity":"alice","workspace":"/repo/gait","risk_class":"high","run_id":"run_mcp_bridge"}}`)
	bridgeRunpackPath := filepath.Join(workDir, "runpack_mcp_bridge.zip")
	bridgeOut := runJSONCommand(t, workDir, binPath,
		"mcp", "bridge",
		"--policy", policyPath,
		"--call", mcpCallPath,
		"--runpack-out", bridgeRunpackPath,
		"--json",
	)
	var bridgeResult struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(bridgeOut, &bridgeResult); err != nil {
		t.Fatalf("parse mcp bridge output: %v\n%s", err, string(bridgeOut))
	}
	if !bridgeResult.OK || bridgeResult.Verdict != "allow" {
		t.Fatalf("unexpected mcp bridge output: %s", string(bridgeOut))
	}

	openAIPath := filepath.Join(workDir, "mcp_openai_call.json")
	mustWriteE2EFile(t, openAIPath, `{"type":"function","function":{"name":"tool.write","arguments":"{\"path\":\"/tmp/out2.txt\"}"}}`)
	proxyOut := runJSONCommand(t, workDir, binPath,
		"mcp", "proxy",
		"--policy", policyPath,
		"--call", openAIPath,
		"--adapter", "openai",
		"--json",
	)
	var proxyResult struct {
		OK      bool   `json:"ok"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(proxyOut, &proxyResult); err != nil {
		t.Fatalf("parse mcp proxy output: %v\n%s", err, string(proxyOut))
	}
	if !proxyResult.OK || proxyResult.Verdict != "allow" {
		t.Fatalf("unexpected mcp proxy output: %s", string(proxyOut))
	}
}

func runJSONCommand(t *testing.T, workDir string, binPath string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(binPath, arguments...)
	command.Dir = workDir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run command %q failed: %v\n%s", strings.Join(arguments, " "), err, string(output))
	}
	return output
}

func runJSONCommandExpectCode(t *testing.T, workDir string, binPath string, expectedCode int, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(binPath, arguments...)
	command.Dir = workDir
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("run command %q expected exit code %d", strings.Join(arguments, " "), expectedCode)
	}
	if code := commandExitCode(t, err); code != expectedCode {
		t.Fatalf("run command %q exit code mismatch: got=%d want=%d\n%s", strings.Join(arguments, " "), code, expectedCode, string(output))
	}
	return output
}

func mustWriteE2EFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func writeSignedRegistryManifest(t *testing.T, workDir string) (string, string, string) {
	t.Helper()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	publicKeyPath := filepath.Join(workDir, "registry_public.key")
	if err := os.WriteFile(publicKeyPath, []byte(base64.StdEncoding.EncodeToString(keyPair.Public)), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	manifest := map[string]any{
		"schema_id":        "gait.registry.pack",
		"schema_version":   "1.0.0",
		"created_at":       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"producer_version": "0.0.0-e2e",
		"pack_name":        "baseline-highrisk",
		"pack_version":     "1.1.0",
		"artifacts": []map[string]string{
			{"path": "policy.yaml", "sha256": strings.Repeat("a", 64)},
		},
	}
	signable, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal signable registry manifest: %v", err)
	}
	digest, err := jcs.DigestJCS(signable)
	if err != nil {
		t.Fatalf("digest registry manifest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign registry manifest: %v", err)
	}
	manifest["signatures"] = []map[string]string{{
		"alg":           signature.Alg,
		"key_id":        signature.KeyID,
		"sig":           signature.Sig,
		"signed_digest": signature.SignedDigest,
	}}

	manifestPath := filepath.Join(workDir, "registry_pack.json")
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal signed registry manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatalf("write registry manifest: %v", err)
	}
	return manifestPath, publicKeyPath, filepath.Join(workDir, "registry_cache")
}
