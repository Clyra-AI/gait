package projectconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

const DefaultPath = ".gait/config.yaml"

type Config struct {
	Gate      GateDefaults      `yaml:"gate"`
	MCPServe  MCPServeDefaults  `yaml:"mcp_serve"`
	Retention RetentionDefaults `yaml:"retention"`
}

type GateDefaults struct {
	Policy                 string `yaml:"policy"`
	Profile                string `yaml:"profile"`
	KeyMode                string `yaml:"key_mode"`
	PrivateKey             string `yaml:"private_key"` // #nosec G117 -- config key name documents expected secret input.
	PrivateKeyEnv          string `yaml:"private_key_env"`
	ApprovalPublicKey      string `yaml:"approval_public_key"`
	ApprovalPublicKeyEnv   string `yaml:"approval_public_key_env"`
	ApprovalPrivateKey     string `yaml:"approval_private_key"`
	ApprovalPrivateKeyEnv  string `yaml:"approval_private_key_env"`
	RateLimitState         string `yaml:"rate_limit_state"`
	CredentialBroker       string `yaml:"credential_broker"`
	CredentialEnvPrefix    string `yaml:"credential_env_prefix"`
	CredentialRef          string `yaml:"credential_ref"`
	CredentialScopes       string `yaml:"credential_scopes"`
	CredentialCommand      string `yaml:"credential_command"`
	CredentialCommandArgs  string `yaml:"credential_command_args"`
	CredentialEvidencePath string `yaml:"credential_evidence_path"`
	TracePath              string `yaml:"trace_path"`
	WrkrInventoryPath      string `yaml:"wrkr_inventory_path"`
}

type MCPServeDefaults struct {
	Enabled                  bool   `yaml:"enabled"`
	Listen                   string `yaml:"listen"`
	AuthMode                 string `yaml:"auth_mode"`
	AuthTokenEnv             string `yaml:"auth_token_env"`
	MaxRequestBytes          int64  `yaml:"max_request_bytes"`
	HTTPVerdictStatus        string `yaml:"http_verdict_status"`
	AllowClientArtifactPaths bool   `yaml:"allow_client_artifact_paths"`
}

type RetentionDefaults struct {
	TraceTTL   string `yaml:"trace_ttl"`
	SessionTTL string `yaml:"session_ttl"`
	ExportTTL  string `yaml:"export_ttl"`
}

func Load(path string, allowMissing bool) (Config, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return Config{}, fmt.Errorf("project config path is required")
	}

	// #nosec G304 -- project config path is explicit local user input.
	content, err := os.ReadFile(trimmedPath)
	if err != nil {
		if os.IsNotExist(err) && allowMissing {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read project config: %w", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return Config{}, nil
	}

	var configuration Config
	if err := yaml.Unmarshal(content, &configuration); err != nil {
		return Config{}, fmt.Errorf("parse project config: %w", err)
	}
	configuration.normalize()
	return configuration, nil
}

func (configuration *Config) normalize() {
	configuration.Gate.Policy = strings.TrimSpace(configuration.Gate.Policy)
	configuration.Gate.Profile = strings.TrimSpace(configuration.Gate.Profile)
	configuration.Gate.KeyMode = strings.TrimSpace(configuration.Gate.KeyMode)
	configuration.Gate.PrivateKey = strings.TrimSpace(configuration.Gate.PrivateKey)
	configuration.Gate.PrivateKeyEnv = strings.TrimSpace(configuration.Gate.PrivateKeyEnv)
	configuration.Gate.ApprovalPublicKey = strings.TrimSpace(configuration.Gate.ApprovalPublicKey)
	configuration.Gate.ApprovalPublicKeyEnv = strings.TrimSpace(configuration.Gate.ApprovalPublicKeyEnv)
	configuration.Gate.ApprovalPrivateKey = strings.TrimSpace(configuration.Gate.ApprovalPrivateKey)
	configuration.Gate.ApprovalPrivateKeyEnv = strings.TrimSpace(configuration.Gate.ApprovalPrivateKeyEnv)
	configuration.Gate.RateLimitState = strings.TrimSpace(configuration.Gate.RateLimitState)
	configuration.Gate.CredentialBroker = strings.TrimSpace(configuration.Gate.CredentialBroker)
	configuration.Gate.CredentialEnvPrefix = strings.TrimSpace(configuration.Gate.CredentialEnvPrefix)
	configuration.Gate.CredentialRef = strings.TrimSpace(configuration.Gate.CredentialRef)
	configuration.Gate.CredentialScopes = strings.TrimSpace(configuration.Gate.CredentialScopes)
	configuration.Gate.CredentialCommand = strings.TrimSpace(configuration.Gate.CredentialCommand)
	configuration.Gate.CredentialCommandArgs = strings.TrimSpace(configuration.Gate.CredentialCommandArgs)
	configuration.Gate.CredentialEvidencePath = strings.TrimSpace(configuration.Gate.CredentialEvidencePath)
	configuration.Gate.TracePath = strings.TrimSpace(configuration.Gate.TracePath)
	configuration.Gate.WrkrInventoryPath = strings.TrimSpace(configuration.Gate.WrkrInventoryPath)
	configuration.MCPServe.Listen = strings.TrimSpace(configuration.MCPServe.Listen)
	configuration.MCPServe.AuthMode = strings.ToLower(strings.TrimSpace(configuration.MCPServe.AuthMode))
	configuration.MCPServe.AuthTokenEnv = strings.TrimSpace(configuration.MCPServe.AuthTokenEnv)
	configuration.MCPServe.HTTPVerdictStatus = strings.ToLower(strings.TrimSpace(configuration.MCPServe.HTTPVerdictStatus))
	configuration.Retention.TraceTTL = strings.TrimSpace(configuration.Retention.TraceTTL)
	configuration.Retention.SessionTTL = strings.TrimSpace(configuration.Retention.SessionTTL)
	configuration.Retention.ExportTTL = strings.TrimSpace(configuration.Retention.ExportTTL)
}
