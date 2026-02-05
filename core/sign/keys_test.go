package sign

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSigningKeyDev(t *testing.T) {
	kp, warnings, err := LoadSigningKey(KeyConfig{Mode: ModeDev})
	if err != nil {
		t.Fatalf("load signing key: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected dev warning")
	}
	if len(kp.Private) == 0 || len(kp.Public) == 0 {
		t.Fatalf("expected generated keypair")
	}
}

func TestLoadSigningKeyDevWithConfig(t *testing.T) {
	cfg := KeyConfig{Mode: ModeDev, PrivateKeyEnv: "GAIT_PRIVATE_KEY"}
	if _, _, err := LoadSigningKey(cfg); err == nil {
		t.Fatalf("expected error for dev mode with explicit keys")
	}
}

func TestLoadSigningKeyProdMissing(t *testing.T) {
	if _, _, err := LoadSigningKey(KeyConfig{Mode: ModeProd}); err == nil {
		t.Fatalf("expected error for missing prod key")
	}
}

func TestLoadSigningKeyUnsupportedMode(t *testing.T) {
	if _, _, err := LoadSigningKey(KeyConfig{Mode: "nope"}); err == nil {
		t.Fatalf("expected error for unsupported mode")
	}
}

func TestLoadSigningKeyProdEnvMissing(t *testing.T) {
	cfg := KeyConfig{
		Mode:          ModeProd,
		PrivateKeyEnv: "MISSING_PRIVATE_KEY",
	}
	if _, _, err := LoadSigningKey(cfg); err == nil {
		t.Fatalf("expected error for missing private key env")
	}
}

func TestLoadVerifyKeyConflictingSources(t *testing.T) {
	cfg := KeyConfig{
		PublicKeyPath: "path",
		PublicKeyEnv:  "ENV",
	}
	if _, err := LoadVerifyKey(cfg); err == nil {
		t.Fatalf("expected error for conflicting public key sources")
	}
}

func TestReadEnvValueWhitespace(t *testing.T) {
	t.Setenv("GAIT_EMPTY", "   ")
	if value, ok := readEnvValue("GAIT_EMPTY"); ok || value != "" {
		t.Fatalf("expected empty env to be treated as missing")
	}
}

func TestLoadSigningKeyProdEnv(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	t.Setenv("GAIT_PRIVATE_KEY", base64.StdEncoding.EncodeToString(kp.Private))
	t.Setenv("GAIT_PUBLIC_KEY", base64.StdEncoding.EncodeToString(kp.Public))

	cfg := KeyConfig{
		Mode:          ModeProd,
		PrivateKeyEnv: "GAIT_PRIVATE_KEY",
		PublicKeyEnv:  "GAIT_PUBLIC_KEY",
	}
	loaded, warnings, err := LoadSigningKey(cfg)
	if err != nil {
		t.Fatalf("load signing key: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings in prod")
	}
	if !loaded.Private.Equal(kp.Private) || !loaded.Public.Equal(kp.Public) {
		t.Fatalf("loaded keypair mismatch")
	}
}

func TestLoadSigningKeyProdMismatch(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	t.Setenv("GAIT_PRIVATE_KEY", base64.StdEncoding.EncodeToString(kp1.Private))
	t.Setenv("GAIT_PUBLIC_KEY", base64.StdEncoding.EncodeToString(kp2.Public))

	cfg := KeyConfig{
		Mode:          ModeProd,
		PrivateKeyEnv: "GAIT_PRIVATE_KEY",
		PublicKeyEnv:  "GAIT_PUBLIC_KEY",
	}
	if _, _, err := LoadSigningKey(cfg); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestLoadSigningKeyProdPath(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	dir := t.TempDir()
	privPath := filepath.Join(dir, "priv.key")
	pubPath := filepath.Join(dir, "pub.key")
	if err := os.WriteFile(privPath, []byte(base64.StdEncoding.EncodeToString(kp.Private)), 0o600); err != nil {
		t.Fatalf("write priv: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte(base64.StdEncoding.EncodeToString(kp.Public)), 0o600); err != nil {
		t.Fatalf("write pub: %v", err)
	}
	cfg := KeyConfig{
		Mode:           ModeProd,
		PrivateKeyPath: privPath,
		PublicKeyPath:  pubPath,
	}
	loaded, _, err := LoadSigningKey(cfg)
	if err != nil {
		t.Fatalf("load signing key: %v", err)
	}
	if !loaded.Private.Equal(kp.Private) || !loaded.Public.Equal(kp.Public) {
		t.Fatalf("loaded keypair mismatch")
	}
}

func TestLoadVerifyKeyPublicEnv(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	t.Setenv("GAIT_PUBLIC_KEY", base64.StdEncoding.EncodeToString(kp.Public))

	cfg := KeyConfig{PublicKeyEnv: "GAIT_PUBLIC_KEY"}
	pub, err := LoadVerifyKey(cfg)
	if err != nil {
		t.Fatalf("load verify key: %v", err)
	}
	if !pub.Equal(kp.Public) {
		t.Fatalf("public key mismatch")
	}
}

func TestLoadVerifyKeyFromPrivate(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	t.Setenv("GAIT_PRIVATE_KEY", base64.StdEncoding.EncodeToString(kp.Private))

	cfg := KeyConfig{PrivateKeyEnv: "GAIT_PRIVATE_KEY"}
	pub, err := LoadVerifyKey(cfg)
	if err != nil {
		t.Fatalf("load verify key: %v", err)
	}
	if !pub.Equal(kp.Public) {
		t.Fatalf("public key mismatch")
	}
}

func TestLoadVerifyKeyMissing(t *testing.T) {
	if _, err := LoadVerifyKey(KeyConfig{}); err == nil {
		t.Fatalf("expected error for missing verify key")
	}
}

func TestLoadVerifyKeyPublicEnvMissing(t *testing.T) {
	cfg := KeyConfig{PublicKeyEnv: "MISSING_PUBLIC_KEY"}
	if _, err := LoadVerifyKey(cfg); err == nil {
		t.Fatalf("expected error for missing public key env")
	}
}
