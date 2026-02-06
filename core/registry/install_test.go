package registry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidahmann/gait/core/jcs"
	schemaregistry "github.com/davidahmann/gait/core/schema/v1/registry"
	"github.com/davidahmann/gait/core/sign"
)

func TestInstallRemoteWithSignatureAndPin(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		PackName:        "baseline-highrisk",
		PackVersion:     "1.1.0",
		RiskClass:       "high",
		UseCase:         "tool-gating",
		Compatibility:   []string{"gait>=1.0.0"},
		Artifacts: []schemaregistry.PackArtifact{{
			Path:   "policy.yaml",
			SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}
	signableDigest, err := signableManifestDigest(manifest)
	if err != nil {
		t.Fatalf("signable manifest digest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, signableDigest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	cacheDir := filepath.Join(t.TempDir(), "cache")
	result, err := Install(context.Background(), InstallOptions{
		Source:            server.URL,
		CacheDir:          cacheDir,
		PublicKey:         keyPair.Public,
		AllowHosts:        []string{"127.0.0.1"},
		PinDigest:         "sha256:" + signableDigest,
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if result.PackName != manifest.PackName {
		t.Fatalf("unexpected pack name: %s", result.PackName)
	}
	if result.Digest != signableDigest {
		t.Fatalf("unexpected digest: %s", result.Digest)
	}
	if _, err := os.Stat(result.MetadataPath); err != nil {
		t.Fatalf("metadata path: %v", err)
	}
	if _, err := os.Stat(result.PinPath); err != nil {
		t.Fatalf("pin path: %v", err)
	}
	metadataInfo, err := os.Stat(result.MetadataPath)
	if err != nil {
		t.Fatalf("stat metadata mode: %v", err)
	}
	if runtime.GOOS == "windows" {
		if metadataInfo.Mode().Perm()&0o600 != 0o600 {
			t.Fatalf("expected metadata owner read/write bits on windows got %#o", metadataInfo.Mode().Perm())
		}
	} else if metadataInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected metadata mode 0600 got %#o", metadataInfo.Mode().Perm())
	}
	pinInfo, err := os.Stat(result.PinPath)
	if err != nil {
		t.Fatalf("stat pin mode: %v", err)
	}
	if runtime.GOOS == "windows" {
		if pinInfo.Mode().Perm()&0o600 != 0o600 {
			t.Fatalf("expected pin owner read/write bits on windows got %#o", pinInfo.Mode().Perm())
		}
	} else if pinInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected pin mode 0600 got %#o", pinInfo.Mode().Perm())
	}
}

func TestInstallRemoteRequiresAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := Install(context.Background(), InstallOptions{
		Source:            server.URL,
		AllowInsecureHTTP: true,
	})
	if err == nil {
		t.Fatalf("expected allowlist error")
	}
}

func TestRegistryHelperBranches(t *testing.T) {
	if hosts := normalizeAllowHosts([]string{" EXAMPLE.com,example.com", "api.example.com"}); strings.Join(hosts, ",") != "api.example.com,example.com" {
		t.Fatalf("normalizeAllowHosts mismatch: %#v", hosts)
	}
	if !isRemoteSource("https://example.com/pack.json") || !isRemoteSource("http://example.com/pack.json") {
		t.Fatalf("isRemoteSource should detect http/https")
	}
	if isRemoteSource("/tmp/pack.json") {
		t.Fatalf("isRemoteSource should not detect local path")
	}
	if err := enforceAllowHost("https://example.com/pack.json", []string{"example.com"}); err != nil {
		t.Fatalf("enforceAllowHost expected pass: %v", err)
	}
	if err := enforceAllowHost("://bad-url", []string{"example.com"}); err == nil {
		t.Fatalf("expected parse URL error")
	}
	if err := enforceAllowHost("https://example.com/pack.json", nil); err == nil {
		t.Fatalf("expected missing allow-host error")
	}
	if err := enforceAllowHost("https://example.com/pack.json", []string{"other.com"}); err == nil {
		t.Fatalf("expected allowlist mismatch error")
	}

	if _, err := parseRegistryManifest([]byte(`{}`)); err == nil {
		t.Fatalf("expected parseRegistryManifest schema error")
	}
	validManifest := []byte(`{"schema_id":"gait.registry.pack","schema_version":"1.0.0","created_at":"2026-01-01T00:00:00Z","producer_version":"0.0.0-dev","pack_name":"p","pack_version":"v","artifacts":[{"path":"a","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`)
	if _, err := parseRegistryManifest(validManifest); err != nil {
		t.Fatalf("parseRegistryManifest valid: %v", err)
	}

	if err := enforcePin("", "abc"); err != nil {
		t.Fatalf("enforcePin empty should pass: %v", err)
	}
	if err := enforcePin("sha256:abc", "abc"); err != nil {
		t.Fatalf("enforcePin matching digest should pass: %v", err)
	}
	if err := enforcePin("sha256:def", "abc"); err == nil {
		t.Fatalf("expected pin mismatch")
	}

	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	sig, err := sign.SignDigestHex(keyPair.Private, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	signatures := []schemaregistry.SignatureRef{{
		Alg:          sig.Alg,
		KeyID:        sig.KeyID,
		Sig:          sig.Sig,
		SignedDigest: sig.SignedDigest,
	}}
	if err := verifySignatures(signatures, keyPair.Public, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("verifySignatures valid: %v", err)
	}
	if err := verifySignatures(nil, keyPair.Public, strings.Repeat("a", 64)); err == nil {
		t.Fatalf("expected verifySignatures missing signatures error")
	}
	if err := verifySignatures(signatures, ed25519.PublicKey{}, strings.Repeat("a", 64)); err == nil {
		t.Fatalf("expected verifySignatures missing key error")
	}
	if err := verifySignatures(signatures, keyPair.Public, strings.Repeat("b", 64)); err == nil {
		t.Fatalf("expected verifySignatures digest mismatch error")
	}

	expectedCacheDir := filepath.Clean("/tmp/x")
	if cacheDir, err := resolveCacheDir(" /tmp/x "); err != nil || cacheDir != expectedCacheDir {
		t.Fatalf("resolveCacheDir explicit mismatch: got=%s expected=%s err=%v", cacheDir, expectedCacheDir, err)
	}
	if cacheDir, err := resolveCacheDir(""); err != nil || !strings.Contains(filepath.ToSlash(cacheDir), ".gait/registry") {
		t.Fatalf("resolveCacheDir default mismatch: %s err=%v", cacheDir, err)
	}

	if data, err := ioReadAllLimit(strings.NewReader("hello"), 8); err != nil || string(data) != "hello" {
		t.Fatalf("ioReadAllLimit small read mismatch: %q err=%v", string(data), err)
	}
	if _, err := ioReadAllLimit(strings.NewReader("toolong"), 3); err == nil {
		t.Fatalf("expected ioReadAllLimit payload too large error")
	}

	clientErr := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})}
	if _, err := fetchSource(context.Background(), "https://example.com/x", clientErr, 1, time.Millisecond); err == nil {
		t.Fatalf("expected fetchSource transport error")
	}
	clientStatus := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("bad")),
		}, nil
	})}
	if _, err := fetchSource(context.Background(), "https://example.com/x", clientStatus, 1, time.Millisecond); err == nil {
		t.Fatalf("expected fetchSource non-200 status error")
	}
	clientLarge := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 6*1024*1024))),
		}, nil
	})}
	if _, err := fetchSource(context.Background(), "https://example.com/x", clientLarge, 1, time.Millisecond); err == nil {
		t.Fatalf("expected fetchSource large payload error")
	}
}

func TestInstallLocalAndErrorBranches(t *testing.T) {
	workDir := t.TempDir()
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		PackName:        "local-pack",
		PackVersion:     "1.0.0",
		Artifacts:       []schemaregistry.PackArtifact{{Path: "policy.yaml", SHA256: strings.Repeat("a", 64)}},
	}
	digest, err := signableManifestDigest(manifest)
	if err != nil {
		t.Fatalf("signableManifestDigest: %v", err)
	}
	sig, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          sig.Alg,
		KeyID:        sig.KeyID,
		Sig:          sig.Sig,
		SignedDigest: sig.SignedDigest,
	}}
	manifestPath := filepath.Join(workDir, "registry_pack.json")
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := Install(context.Background(), InstallOptions{
		Source:    manifestPath,
		CacheDir:  filepath.Join(workDir, "cache"),
		PublicKey: keyPair.Public,
	}); err != nil {
		t.Fatalf("Install local source: %v", err)
	}

	if _, err := Install(context.Background(), InstallOptions{
		Source:    manifestPath,
		CacheDir:  filepath.Join(workDir, "cache2"),
		PublicKey: keyPair.Public,
		PinDigest: "sha256:" + strings.Repeat("b", 64),
	}); err == nil {
		t.Fatalf("expected pin mismatch error")
	}

	if _, err := Install(context.Background(), InstallOptions{
		Source:   " ",
		CacheDir: filepath.Join(workDir, "cache3"),
	}); err == nil {
		t.Fatalf("expected missing source error")
	}
}

func TestInstallRemoteRetryAndFallbackBranches(t *testing.T) {
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		PackName:        "retry-pack",
		PackVersion:     "1.0.0",
		Artifacts:       []schemaregistry.PackArtifact{{Path: "policy.yaml", SHA256: strings.Repeat("a", 64)}},
	}
	digest, err := signableManifestDigest(manifest)
	if err != nil {
		t.Fatalf("signable digest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	t.Run("transient retries recover", func(t *testing.T) {
		requests := 0
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if requests < 3 {
				return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad"))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload))}, nil
		})}
		cacheDir := filepath.Join(t.TempDir(), "cache")
		result, installErr := Install(context.Background(), InstallOptions{
			Source:            "http://example.com/pack.json",
			CacheDir:          cacheDir,
			PublicKey:         keyPair.Public,
			AllowHosts:        []string{"example.com"},
			PinDigest:         "sha256:" + digest,
			HTTPClient:        client,
			RetryMaxAttempts:  3,
			RetryBaseDelay:    time.Millisecond,
			AllowInsecureHTTP: true,
		})
		if installErr != nil {
			t.Fatalf("install with retry: %v", installErr)
		}
		if requests != 3 {
			t.Fatalf("expected 3 requests, got %d", requests)
		}
		if result.FallbackUsed {
			t.Fatalf("unexpected fallback usage")
		}
	})

	t.Run("permanent status fails without retries", func(t *testing.T) {
		requests := 0
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("forbidden"))}, nil
		})}
		_, installErr := Install(context.Background(), InstallOptions{
			Source:            "http://example.com/pack.json",
			CacheDir:          filepath.Join(t.TempDir(), "cache"),
			PublicKey:         keyPair.Public,
			AllowHosts:        []string{"example.com"},
			PinDigest:         "sha256:" + digest,
			HTTPClient:        client,
			RetryMaxAttempts:  5,
			RetryBaseDelay:    time.Millisecond,
			AllowInsecureHTTP: true,
		})
		if installErr == nil {
			t.Fatalf("expected permanent failure")
		}
		if requests != 1 {
			t.Fatalf("expected single request for permanent error, got %d", requests)
		}
	})

	t.Run("cached fallback is explicit and pinned", func(t *testing.T) {
		cacheDir := filepath.Join(t.TempDir(), "cache")
		cachedPath := filepath.Join(cacheDir, manifest.PackName, manifest.PackVersion, digest, "registry_pack.json")
		if err := os.MkdirAll(filepath.Dir(cachedPath), 0o750); err != nil {
			t.Fatalf("mkdir cached path: %v", err)
		}
		if err := os.WriteFile(cachedPath, payload, 0o600); err != nil {
			t.Fatalf("write cached manifest: %v", err)
		}
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})}
		result, installErr := Install(context.Background(), InstallOptions{
			Source:              "http://example.com/pack.json",
			CacheDir:            cacheDir,
			PublicKey:           keyPair.Public,
			AllowHosts:          []string{"example.com"},
			PinDigest:           "sha256:" + digest,
			HTTPClient:          client,
			RetryMaxAttempts:    2,
			RetryBaseDelay:      time.Millisecond,
			AllowInsecureHTTP:   true,
			AllowCachedFallback: true,
		})
		if installErr != nil {
			t.Fatalf("install with cached fallback: %v", installErr)
		}
		if !result.FallbackUsed {
			t.Fatalf("expected fallback usage")
		}
		if result.FallbackPath == "" || result.FallbackPath != cachedPath {
			t.Fatalf("unexpected fallback path: %s", result.FallbackPath)
		}
	})

	t.Run("http blocked by default", func(t *testing.T) {
		_, installErr := Install(context.Background(), InstallOptions{
			Source:     "http://example.com/pack.json",
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
			PublicKey:  keyPair.Public,
			AllowHosts: []string{"example.com"},
			PinDigest:  "sha256:" + digest,
		})
		if installErr == nil {
			t.Fatalf("expected https enforcement error")
		}
		if !strings.Contains(strings.ToLower(installErr.Error()), "https") {
			t.Fatalf("expected https error, got %v", installErr)
		}
	})
}

func TestInstallLocalConcurrentOperations(t *testing.T) {
	workDir := t.TempDir()
	cacheDir := filepath.Join(workDir, "cache")
	keyPair, err := sign.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	manifest := schemaregistry.RegistryPack{
		SchemaID:        "gait.registry.pack",
		SchemaVersion:   "1.0.0",
		CreatedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ProducerVersion: "0.0.0-dev",
		PackName:        "concurrent-pack",
		PackVersion:     "1.0.0",
		Artifacts: []schemaregistry.PackArtifact{{
			Path:   "policy.yaml",
			SHA256: strings.Repeat("a", 64),
		}},
	}
	digest, err := signableManifestDigest(manifest)
	if err != nil {
		t.Fatalf("signable digest: %v", err)
	}
	signature, err := sign.SignDigestHex(keyPair.Private, digest)
	if err != nil {
		t.Fatalf("sign digest: %v", err)
	}
	manifest.Signatures = []schemaregistry.SignatureRef{{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(workDir, "registry_pack.json")
	if err := os.WriteFile(manifestPath, payload, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	const workers = 8
	var group sync.WaitGroup
	errs := make(chan error, workers)
	results := make(chan InstallResult, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, installErr := Install(context.Background(), InstallOptions{
				Source:    manifestPath,
				CacheDir:  cacheDir,
				PublicKey: keyPair.Public,
				PinDigest: "sha256:" + digest,
			})
			if installErr != nil {
				errs <- installErr
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(errs)
	close(results)

	transientErrors := 0
	for installErr := range errs {
		if installErr != nil {
			if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(installErr.Error()), "access is denied") {
				transientErrors++
				continue
			}
			t.Fatalf("concurrent install error: %v", installErr)
		}
	}
	count := 0
	for result := range results {
		count++
		if result.Digest != digest {
			t.Fatalf("unexpected digest from concurrent install: %s", result.Digest)
		}
		if _, statErr := os.Stat(result.MetadataPath); statErr != nil {
			t.Fatalf("missing metadata path from concurrent install: %v", statErr)
		}
		if _, statErr := os.Stat(result.PinPath); statErr != nil {
			t.Fatalf("missing pin path from concurrent install: %v", statErr)
		}
	}
	if runtime.GOOS == "windows" {
		if count == 0 {
			t.Fatalf("expected at least one successful concurrent install result on windows")
		}
		if count+transientErrors != workers {
			t.Fatalf("expected workers to be accounted for: workers=%d success=%d transient=%d", workers, count, transientErrors)
		}
	} else if count != workers {
		t.Fatalf("expected %d successful install results, got %d", workers, count)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func signableManifestDigest(manifest schemaregistry.RegistryPack) (string, error) {
	signable := manifest
	signable.Signatures = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}
