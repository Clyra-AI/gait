package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/fsx"
	schemaregistry "github.com/Clyra-AI/gait/core/schema/v1/registry"
	jcs "github.com/Clyra-AI/proof/canon"
	sign "github.com/Clyra-AI/proof/signing"
)

type InstallOptions struct {
	Source              string
	CacheDir            string
	PublicKey           ed25519.PublicKey
	AllowHosts          []string
	PublisherAllowlist  []string
	PinDigest           string
	HTTPClient          *http.Client
	RetryMaxAttempts    int
	RetryBaseDelay      time.Duration
	AllowInsecureHTTP   bool
	AllowCachedFallback bool
}

type InstallResult struct {
	Source       string                      `json:"source"`
	PackName     string                      `json:"pack_name"`
	PackVersion  string                      `json:"pack_version"`
	Digest       string                      `json:"digest"`
	MetadataPath string                      `json:"metadata_path"`
	PinPath      string                      `json:"pin_path"`
	FallbackUsed bool                        `json:"fallback_used,omitempty"`
	FallbackPath string                      `json:"fallback_path,omitempty"`
	Manifest     schemaregistry.RegistryPack `json:"manifest"`
}

type fetchStatusError struct {
	statusCode int
}

func (e fetchStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d", e.statusCode)
}

func (e fetchStatusError) StatusCode() int {
	return e.statusCode
}

func Install(ctx context.Context, options InstallOptions) (InstallResult, error) {
	source := strings.TrimSpace(options.Source)
	if source == "" {
		return InstallResult{}, fmt.Errorf("source is required")
	}
	allowHosts := normalizeAllowHosts(options.AllowHosts)
	remoteSource := isRemoteSource(source)
	if remoteSource {
		if err := enforceRemoteScheme(source, options.AllowInsecureHTTP); err != nil {
			return InstallResult{}, err
		}
		if err := enforceAllowHost(source, allowHosts); err != nil {
			return InstallResult{}, err
		}
	}
	cacheDir, err := resolveCacheDir(options.CacheDir)
	if err != nil {
		return InstallResult{}, err
	}

	rawManifest, err := fetchSource(ctx, source, options.HTTPClient, options.RetryMaxAttempts, options.RetryBaseDelay)
	if err != nil {
		if !remoteSource || !options.AllowCachedFallback {
			return InstallResult{}, err
		}
		rawManifest, fallbackPath, fallbackErr := loadCachedFallback(cacheDir, options.PinDigest)
		if fallbackErr != nil {
			return InstallResult{}, fmt.Errorf("remote fetch failed: %w; cached fallback failed: %v", err, fallbackErr)
		}
		fallbackResult, installErr := installManifest(cacheDir, source, rawManifest, options, true, fallbackPath)
		if installErr != nil {
			return InstallResult{}, installErr
		}
		return fallbackResult, nil
	}
	return installManifest(cacheDir, source, rawManifest, options, false, "")
}

func installManifest(cacheDir string, source string, rawManifest []byte, options InstallOptions, fallbackUsed bool, fallbackPath string) (InstallResult, error) {
	manifest, err := parseRegistryManifest(rawManifest)
	if err != nil {
		return InstallResult{}, err
	}
	if err := enforcePublisherAllowlist(manifest.Publisher, options.PublisherAllowlist); err != nil {
		return InstallResult{}, err
	}
	signableDigest, _, err := digestSignableManifest(manifest)
	if err != nil {
		return InstallResult{}, err
	}
	metadataBytes, err := canonicalManifest(manifest)
	if err != nil {
		return InstallResult{}, err
	}
	if err := enforcePin(options.PinDigest, signableDigest); err != nil {
		return InstallResult{}, err
	}
	if err := verifySignatures(manifest.Signatures, options.PublicKey, signableDigest); err != nil {
		return InstallResult{}, err
	}
	metadataPath := filepath.Join(
		cacheDir,
		manifest.PackName,
		manifest.PackVersion,
		signableDigest,
		"registry_pack.json",
	)
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o750); err != nil {
		return InstallResult{}, fmt.Errorf("mkdir metadata dir: %w", err)
	}
	if err := fsx.WriteFileAtomic(metadataPath, metadataBytes, 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("write registry metadata: %w", err)
	}

	pinsDir := filepath.Join(cacheDir, "pins")
	if err := os.MkdirAll(pinsDir, 0o750); err != nil {
		return InstallResult{}, fmt.Errorf("mkdir pins dir: %w", err)
	}
	pinPath := filepath.Join(pinsDir, manifest.PackName+".pin")
	if err := fsx.WriteFileAtomic(pinPath, []byte("sha256:"+signableDigest+"\n"), 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("write pin file: %w", err)
	}

	return InstallResult{
		Source:       source,
		PackName:     manifest.PackName,
		PackVersion:  manifest.PackVersion,
		Digest:       signableDigest,
		MetadataPath: metadataPath,
		PinPath:      pinPath,
		FallbackUsed: fallbackUsed,
		FallbackPath: fallbackPath,
		Manifest:     manifest,
	}, nil
}

func normalizeAllowHosts(allowHosts []string) []string {
	out := make([]string, 0, len(allowHosts))
	for _, allowHost := range allowHosts {
		for _, segment := range strings.Split(allowHost, ",") {
			trimmed := strings.ToLower(strings.TrimSpace(segment))
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	dedup := make([]string, 0, len(out))
	for _, value := range out {
		if len(dedup) == 0 || dedup[len(dedup)-1] != value {
			dedup = append(dedup, value)
		}
	}
	return dedup
}

func normalizePublisherAllowlist(allowlist []string) []string {
	out := make([]string, 0, len(allowlist))
	for _, value := range allowlist {
		for _, segment := range strings.Split(value, ",") {
			trimmed := strings.ToLower(strings.TrimSpace(segment))
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	dedup := make([]string, 0, len(out))
	for _, value := range out {
		if len(dedup) == 0 || dedup[len(dedup)-1] != value {
			dedup = append(dedup, value)
		}
	}
	return dedup
}

func enforcePublisherAllowlist(publisher string, allowlist []string) error {
	normalizedAllowlist := normalizePublisherAllowlist(allowlist)
	if len(normalizedAllowlist) == 0 {
		return nil
	}
	normalizedPublisher := strings.ToLower(strings.TrimSpace(publisher))
	if normalizedPublisher == "" {
		return fmt.Errorf("publisher allowlist configured but manifest publisher is empty")
	}
	for _, allowed := range normalizedAllowlist {
		if normalizedPublisher == allowed {
			return nil
		}
	}
	return fmt.Errorf("manifest publisher %s is not in allowlist", normalizedPublisher)
}

func isRemoteSource(source string) bool {
	return strings.HasPrefix(strings.ToLower(source), "https://") ||
		strings.HasPrefix(strings.ToLower(source), "http://")
}

func enforceAllowHost(source string, allowHosts []string) error {
	parsed, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("parse source url: %w", err)
	}
	if len(allowHosts) == 0 {
		return fmt.Errorf("remote source requires --allow-host")
	}
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range allowHosts {
		if host == allowed {
			return nil
		}
	}
	return fmt.Errorf("source host %s is not in allowlist", host)
}

func enforceRemoteScheme(source string, allowInsecureHTTP bool) error {
	parsed, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("parse source url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme == "https" {
		return nil
	}
	if scheme == "http" && allowInsecureHTTP {
		return nil
	}
	return fmt.Errorf("remote source requires https")
}

func fetchSource(ctx context.Context, source string, client *http.Client, maxAttempts int, baseDelay time.Duration) ([]byte, error) {
	if !isRemoteSource(source) {
		// #nosec G304 -- user-supplied local file path is intentional.
		raw, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read source file: %w", err)
		}
		return raw, nil
	}
	return fetchRemoteSourceWithRetry(ctx, source, client, maxAttempts, baseDelay)
}

func fetchRemoteSourceWithRetry(ctx context.Context, source string, client *http.Client, maxAttempts int, baseDelay time.Duration) ([]byte, error) {
	attempts := maxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	delay := baseDelay
	if delay <= 0 {
		delay = 200 * time.Millisecond
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		payload, err := fetchRemoteSourceOnce(ctx, source, client)
		if err == nil {
			return payload, nil
		}
		lastErr = err
		if !isTransientFetchError(err) || attempt == attempts {
			break
		}
		sleepFor := retryDelay(delay, attempt)
		timer := time.NewTimer(sleepFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("download source: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("download source: %w", lastErr)
}

func fetchRemoteSourceOnce(ctx context.Context, source string, client *http.Client) ([]byte, error) {
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("build source request: %w", err)
	}
	// #nosec G704 -- source URL is policy-controlled and validated by caller before fetch.
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fetchStatusError{statusCode: response.StatusCode}
	}
	raw, err := ioReadAllLimit(response.Body, 5*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("read source response: %w", err)
	}
	return raw, nil
}

func isTransientFetchError(err error) bool {
	var statusErr fetchStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode() {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "connection reset") ||
		strings.Contains(errText, "connection refused") ||
		strings.Contains(errText, "unexpected eof")
}

func retryDelay(baseDelay time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}
	backoff := baseDelay << (attempt - 1)
	jitter := time.Duration(attempt) * 25 * time.Millisecond
	if jitter > 100*time.Millisecond {
		jitter = 100 * time.Millisecond
	}
	return backoff + jitter
}

func loadCachedFallback(cacheDir, pinDigest string) ([]byte, string, error) {
	trimmedDigest := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pinDigest)), "sha256:")
	if trimmedDigest == "" {
		return nil, "", fmt.Errorf("cached fallback requires --pin sha256:<digest>")
	}
	pattern := filepath.Join(cacheDir, "*", "*", trimmedDigest, "registry_pack.json")
	candidates, err := filepath.Glob(pattern)
	if err != nil {
		return nil, "", fmt.Errorf("glob cached fallback: %w", err)
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no cached manifest found for digest %s", trimmedDigest)
	}
	sort.Strings(candidates)
	for _, candidate := range candidates {
		// #nosec G304 -- cached fallback candidate path is derived from controlled cache glob.
		content, readErr := os.ReadFile(candidate)
		if readErr == nil {
			return content, candidate, nil
		}
	}
	return nil, "", fmt.Errorf("cached fallback candidates unreadable")
}

func parseRegistryManifest(raw []byte) (schemaregistry.RegistryPack, error) {
	var manifest schemaregistry.RegistryPack
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return schemaregistry.RegistryPack{}, fmt.Errorf("parse registry manifest: %w", err)
	}
	if manifest.SchemaID != "gait.registry.pack" {
		return schemaregistry.RegistryPack{}, fmt.Errorf("unsupported schema_id %q", manifest.SchemaID)
	}
	if manifest.SchemaVersion != "1.0.0" {
		return schemaregistry.RegistryPack{}, fmt.Errorf("unsupported schema_version %q", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.PackName) == "" {
		return schemaregistry.RegistryPack{}, fmt.Errorf("pack_name is required")
	}
	if strings.TrimSpace(manifest.PackVersion) == "" {
		return schemaregistry.RegistryPack{}, fmt.Errorf("pack_version is required")
	}
	manifest.PackType = strings.ToLower(strings.TrimSpace(manifest.PackType))
	manifest.Publisher = strings.ToLower(strings.TrimSpace(manifest.Publisher))
	manifest.Source = strings.ToLower(strings.TrimSpace(manifest.Source))
	if manifest.PackType == "skill" {
		if manifest.Publisher == "" || manifest.Source == "" {
			return schemaregistry.RegistryPack{}, fmt.Errorf("skill pack requires publisher and source")
		}
	}
	return manifest, nil
}

func digestSignableManifest(manifest schemaregistry.RegistryPack) (string, []byte, error) {
	signable := manifest
	signable.Signatures = nil
	raw, err := json.Marshal(signable)
	if err != nil {
		return "", nil, fmt.Errorf("marshal signable registry manifest: %w", err)
	}
	canonical, err := jcs.CanonicalizeJSON(raw)
	if err != nil {
		return "", nil, fmt.Errorf("canonicalize signable registry manifest: %w", err)
	}
	digest, err := jcs.DigestJCS(canonical)
	if err != nil {
		return "", nil, fmt.Errorf("digest signable registry manifest: %w", err)
	}
	return digest, canonical, nil
}

func canonicalManifest(manifest schemaregistry.RegistryPack) ([]byte, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal registry manifest: %w", err)
	}
	canonical, err := jcs.CanonicalizeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("canonicalize registry manifest: %w", err)
	}
	return canonical, nil
}

func enforcePin(expected string, actual string) error {
	trimmed := strings.TrimSpace(expected)
	if trimmed == "" {
		return nil
	}
	trimmed = strings.TrimPrefix(strings.ToLower(trimmed), "sha256:")
	if trimmed != strings.ToLower(actual) {
		return fmt.Errorf("pin digest mismatch")
	}
	return nil
}

func verifySignatures(signatures []schemaregistry.SignatureRef, publicKey ed25519.PublicKey, digest string) error {
	if len(signatures) == 0 {
		return fmt.Errorf("registry manifest has no signatures")
	}
	if len(publicKey) == 0 {
		return fmt.Errorf("public key is required for signature verification")
	}
	valid := 0
	for _, signatureRef := range signatures {
		signature := sign.Signature{
			Alg:          signatureRef.Alg,
			KeyID:        signatureRef.KeyID,
			Sig:          signatureRef.Sig,
			SignedDigest: signatureRef.SignedDigest,
		}
		if !strings.EqualFold(signature.SignedDigest, digest) {
			continue
		}
		ok, err := sign.VerifyDigestHex(publicKey, signature)
		if err != nil {
			continue
		}
		if ok {
			valid++
		}
	}
	if valid == 0 {
		return fmt.Errorf("no valid signature for manifest digest")
	}
	return nil
}

func resolveCacheDir(cacheDir string) (string, error) {
	trimmed := strings.TrimSpace(cacheDir)
	if trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".gait", "registry"), nil
}

func ioReadAllLimit(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("payload too large")
	}
	return data, nil
}
