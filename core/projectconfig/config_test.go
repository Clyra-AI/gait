package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllowMissing(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "missing.yaml")

	configuration, err := Load(path, true)
	if err != nil {
		t.Fatalf("Load allow missing: %v", err)
	}
	if configuration.Gate.Policy != "" {
		t.Fatalf("expected empty configuration, got policy %q", configuration.Gate.Policy)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "missing.yaml")

	if _, err := Load(path, false); err == nil {
		t.Fatal("expected missing required config error")
	}
}

func TestLoadParsesAndNormalizes(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "config.yaml")
	content := []byte(`
gate:
  policy: " examples/policy/base_high_risk.yaml "
  profile: " oss-prod "
  key_mode: " prod "
  private_key: " examples/scenarios/keys/approval_private.key "
  credential_broker: " stub "
  wrkr_inventory_path: " ./.gait/wrkr_inventory.json "
mcp_serve:
  enabled: true
  listen: " 0.0.0.0:8787 "
  auth_mode: " TOKEN "
  auth_token_env: " GAIT_TOKEN "
  max_request_bytes: 1048576
  http_verdict_status: " STRICT "
retention:
  trace_ttl: " 168h "
  session_ttl: " 336h "
  export_ttl: " 168h "
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configuration, err := Load(path, false)
	if err != nil {
		t.Fatalf("Load parse: %v", err)
	}
	if configuration.Gate.Policy != "examples/policy/base_high_risk.yaml" {
		t.Fatalf("unexpected policy %q", configuration.Gate.Policy)
	}
	if configuration.Gate.Profile != "oss-prod" {
		t.Fatalf("unexpected profile %q", configuration.Gate.Profile)
	}
	if configuration.Gate.KeyMode != "prod" {
		t.Fatalf("unexpected key_mode %q", configuration.Gate.KeyMode)
	}
	if configuration.Gate.CredentialBroker != "stub" {
		t.Fatalf("unexpected credential_broker %q", configuration.Gate.CredentialBroker)
	}
	if configuration.Gate.WrkrInventoryPath != "./.gait/wrkr_inventory.json" {
		t.Fatalf("unexpected wrkr_inventory_path %q", configuration.Gate.WrkrInventoryPath)
	}
	if !configuration.MCPServe.Enabled {
		t.Fatalf("expected mcp_serve enabled=true")
	}
	if configuration.MCPServe.Listen != "0.0.0.0:8787" {
		t.Fatalf("unexpected mcp_serve.listen %q", configuration.MCPServe.Listen)
	}
	if configuration.MCPServe.AuthMode != "token" {
		t.Fatalf("unexpected mcp_serve.auth_mode %q", configuration.MCPServe.AuthMode)
	}
	if configuration.MCPServe.HTTPVerdictStatus != "strict" {
		t.Fatalf("unexpected mcp_serve.http_verdict_status %q", configuration.MCPServe.HTTPVerdictStatus)
	}
	if configuration.Retention.TraceTTL != "168h" || configuration.Retention.SessionTTL != "336h" || configuration.Retention.ExportTTL != "168h" {
		t.Fatalf("unexpected retention defaults: %#v", configuration.Retention)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, "config.yaml")
	if err := os.WriteFile(path, []byte("gate: [\n"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	if _, err := Load(path, false); err == nil {
		t.Fatal("expected parse error for invalid yaml")
	}
}
