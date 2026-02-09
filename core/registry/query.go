package registry

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type InstalledPack struct {
	PackName     string `json:"pack_name"`
	PackVersion  string `json:"pack_version"`
	Digest       string `json:"digest"`
	MetadataPath string `json:"metadata_path"`
	PinPath      string `json:"pin_path,omitempty"`
	PinDigest    string `json:"pin_digest,omitempty"`
	PinVerified  bool   `json:"pin_verified"`
}

type ListOptions struct {
	CacheDir string
}

type VerifyOptions struct {
	MetadataPath       string
	CacheDir           string
	PublicKey          ed25519.PublicKey
	PublisherAllowlist []string
}

type VerifyResult struct {
	PackName          string `json:"pack_name"`
	PackVersion       string `json:"pack_version"`
	Digest            string `json:"digest"`
	MetadataPath      string `json:"metadata_path"`
	PinPath           string `json:"pin_path,omitempty"`
	PinDigest         string `json:"pin_digest,omitempty"`
	PinPresent        bool   `json:"pin_present"`
	PinVerified       bool   `json:"pin_verified"`
	SignatureVerified bool   `json:"signature_verified"`
	SignatureError    string `json:"signature_error,omitempty"`
	Publisher         string `json:"publisher,omitempty"`
	Source            string `json:"source,omitempty"`
	PublisherAllowed  bool   `json:"publisher_allowed,omitempty"`
}

func List(options ListOptions) ([]InstalledPack, error) {
	cacheDir, err := resolveCacheDir(options.CacheDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledPack{}, nil
		}
		return nil, fmt.Errorf("read cache dir: %w", err)
	}

	pinPathByPack := map[string]string{}
	pinDigestByPack := map[string]string{}
	if err := loadPins(cacheDir, pinPathByPack, pinDigestByPack); err != nil {
		return nil, err
	}

	packs := make([]InstalledPack, 0)
	for _, packEntry := range entries {
		if !packEntry.IsDir() || packEntry.Name() == "pins" {
			continue
		}
		packName := packEntry.Name()
		packDir := filepath.Join(cacheDir, packName)
		versionEntries, readErr := os.ReadDir(packDir)
		if readErr != nil {
			return nil, fmt.Errorf("read pack versions: %w", readErr)
		}
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}
			versionDir := filepath.Join(packDir, versionEntry.Name())
			digestEntries, digestReadErr := os.ReadDir(versionDir)
			if digestReadErr != nil {
				return nil, fmt.Errorf("read pack digests: %w", digestReadErr)
			}
			for _, digestEntry := range digestEntries {
				if !digestEntry.IsDir() {
					continue
				}
				metadataPath := filepath.Join(versionDir, digestEntry.Name(), "registry_pack.json")
				// #nosec G304 -- path is discovered from local cache structure.
				raw, readMetaErr := os.ReadFile(metadataPath)
				if readMetaErr != nil {
					continue
				}
				manifest, parseErr := parseRegistryManifest(raw)
				if parseErr != nil {
					continue
				}
				digest, _, digestErr := digestSignableManifest(manifest)
				if digestErr != nil {
					continue
				}
				pinDigest := pinDigestByPack[manifest.PackName]
				pinPath := pinPathByPack[manifest.PackName]
				packs = append(packs, InstalledPack{
					PackName:     manifest.PackName,
					PackVersion:  manifest.PackVersion,
					Digest:       digest,
					MetadataPath: metadataPath,
					PinPath:      pinPath,
					PinDigest:    pinDigest,
					PinVerified:  pinDigest != "" && normalizeDigest(pinDigest) == normalizeDigest(digest),
				})
			}
		}
	}

	sort.Slice(packs, func(i, j int) bool {
		if packs[i].PackName != packs[j].PackName {
			return packs[i].PackName < packs[j].PackName
		}
		if packs[i].PackVersion != packs[j].PackVersion {
			return packs[i].PackVersion < packs[j].PackVersion
		}
		return packs[i].Digest < packs[j].Digest
	})
	return packs, nil
}

func Verify(options VerifyOptions) (VerifyResult, error) {
	metadataPath := strings.TrimSpace(options.MetadataPath)
	if metadataPath == "" {
		return VerifyResult{}, fmt.Errorf("metadata path is required")
	}
	// #nosec G304 -- metadata path is explicit local user input.
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("read metadata file: %w", err)
	}
	manifest, err := parseRegistryManifest(raw)
	if err != nil {
		return VerifyResult{}, err
	}
	digest, _, err := digestSignableManifest(manifest)
	if err != nil {
		return VerifyResult{}, err
	}

	result := VerifyResult{
		PackName:     manifest.PackName,
		PackVersion:  manifest.PackVersion,
		Digest:       digest,
		MetadataPath: metadataPath,
		Publisher:    strings.TrimSpace(manifest.Publisher),
		Source:       strings.TrimSpace(manifest.Source),
	}

	if sigErr := verifySignatures(manifest.Signatures, options.PublicKey, digest); sigErr != nil {
		result.SignatureVerified = false
		result.SignatureError = sigErr.Error()
	} else {
		result.SignatureVerified = true
	}
	if err := enforcePublisherAllowlist(manifest.Publisher, options.PublisherAllowlist); err != nil {
		result.PublisherAllowed = false
		if result.SignatureError == "" {
			result.SignatureError = err.Error()
		}
	} else {
		result.PublisherAllowed = true
	}

	cacheDir := strings.TrimSpace(options.CacheDir)
	if cacheDir == "" {
		if inferredCacheDir, ok := inferCacheDirFromMetadataPath(metadataPath, manifest.PackName, manifest.PackVersion, digest); ok {
			cacheDir = inferredCacheDir
		}
	}
	if cacheDir != "" {
		pinPath := filepath.Join(cacheDir, "pins", manifest.PackName+".pin")
		result.PinPath = pinPath
		// #nosec G304 -- pin path is derived from local cache root.
		pinRaw, pinErr := os.ReadFile(pinPath)
		if pinErr == nil {
			result.PinPresent = true
			result.PinDigest = strings.TrimSpace(string(pinRaw))
			result.PinVerified = normalizeDigest(result.PinDigest) == normalizeDigest(digest)
		} else if !os.IsNotExist(pinErr) {
			return VerifyResult{}, fmt.Errorf("read pin file: %w", pinErr)
		}
	}

	return result, nil
}

func inferCacheDirFromMetadataPath(metadataPath string, packName string, packVersion string, digest string) (string, bool) {
	cleanPath := filepath.Clean(metadataPath)
	if filepath.Base(cleanPath) != "registry_pack.json" {
		return "", false
	}
	digestDir := filepath.Dir(cleanPath)
	if !strings.EqualFold(filepath.Base(digestDir), strings.TrimSpace(digest)) {
		return "", false
	}
	versionDir := filepath.Dir(digestDir)
	if filepath.Base(versionDir) != strings.TrimSpace(packVersion) {
		return "", false
	}
	packDir := filepath.Dir(versionDir)
	if filepath.Base(packDir) != strings.TrimSpace(packName) {
		return "", false
	}
	return filepath.Dir(packDir), true
}

func loadPins(cacheDir string, pinPathByPack map[string]string, pinDigestByPack map[string]string) error {
	pinsDir := filepath.Join(cacheDir, "pins")
	entries, err := os.ReadDir(pinsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read pins dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pin") {
			continue
		}
		packName := strings.TrimSuffix(entry.Name(), ".pin")
		pinPath := filepath.Join(pinsDir, entry.Name())
		// #nosec G304 -- pin paths are discovered from local cache.
		raw, readErr := os.ReadFile(pinPath)
		if readErr != nil {
			continue
		}
		pinPathByPack[packName] = pinPath
		pinDigestByPack[packName] = strings.TrimSpace(string(raw))
	}
	return nil
}

func normalizeDigest(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.TrimPrefix(trimmed, "sha256:")
}
