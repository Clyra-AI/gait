package pack

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemapack "github.com/Clyra-AI/gait/core/schema/v1/pack"
	"github.com/Clyra-AI/gait/core/zipx"
	jcs "github.com/Clyra-AI/proof/canon"
)

type BuildAuthorizationOptions struct {
	AuthorizationPath string
	OutputPath        string
	ProducerVersion   string
	SigningPrivateKey ed25519.PrivateKey
}

func BuildAuthorizationPack(options BuildAuthorizationOptions) (BuildResult, error) {
	authorizationPath := strings.TrimSpace(options.AuthorizationPath)
	if authorizationPath == "" {
		return BuildResult{}, fmt.Errorf("authorization bundle path is required")
	}
	// #nosec G304 -- explicit local file input.
	payload, err := os.ReadFile(authorizationPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("read authorization bundle: %w", err)
	}
	bundle, err := normalizeAuthorizationBundle(payload, filepath.Dir(authorizationPath), strings.TrimSpace(options.ProducerVersion))
	if err != nil {
		return BuildResult{}, err
	}
	payloadBytes, err := canonicalJSON(bundle)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode authorization bundle: %w", err)
	}
	files, err := authorizationBundleFiles(bundle, filepath.Dir(authorizationPath))
	if err != nil {
		return BuildResult{}, err
	}
	files = append(files, zipx.File{Path: "authorization_bundle.json", Data: payloadBytes, Mode: 0o644})

	return buildPackWithFiles(buildPackOptions{
		PackType:          string(BuildTypeAuthorization),
		SourceRef:         bundle.TraceID,
		OutputPath:        options.OutputPath,
		ProducerVersion:   bundle.ProducerVersion,
		SigningPrivateKey: options.SigningPrivateKey,
		Files:             files,
		OutputDirFallback: filepath.Dir(authorizationPath),
	})
}

func normalizeAuthorizationBundle(payload []byte, baseDir string, producerVersion string) (schemagate.AuthorizationBundle, error) {
	var bundle schemagate.AuthorizationBundle
	if err := decodeStrictJSON(payload, &bundle); err != nil {
		return schemagate.AuthorizationBundle{}, fmt.Errorf("parse authorization bundle: %w", err)
	}
	if strings.TrimSpace(bundle.SchemaID) == "" {
		bundle.SchemaID = "gait.gate.authorization_bundle"
	}
	if bundle.SchemaID != "gait.gate.authorization_bundle" {
		return schemagate.AuthorizationBundle{}, fmt.Errorf("authorization bundle schema_id must be gait.gate.authorization_bundle")
	}
	if strings.TrimSpace(bundle.SchemaVersion) == "" {
		bundle.SchemaVersion = "1.0.0"
	}
	if bundle.SchemaVersion != "1.0.0" {
		return schemagate.AuthorizationBundle{}, fmt.Errorf("authorization bundle schema_version must be 1.0.0")
	}
	if bundle.CreatedAt.IsZero() {
		bundle.CreatedAt = deterministicTimestamp
	} else {
		bundle.CreatedAt = bundle.CreatedAt.UTC()
	}
	if strings.TrimSpace(bundle.ProducerVersion) == "" {
		if strings.TrimSpace(producerVersion) != "" {
			bundle.ProducerVersion = strings.TrimSpace(producerVersion)
		} else {
			bundle.ProducerVersion = "0.0.0-dev"
		}
	}
	bundle.TraceID = strings.TrimSpace(bundle.TraceID)
	bundle.PolicyDigest = strings.ToLower(strings.TrimSpace(bundle.PolicyDigest))
	bundle.IntentDigest = strings.ToLower(strings.TrimSpace(bundle.IntentDigest))
	if bundle.TraceID == "" || !isSHA256Hex(bundle.PolicyDigest) || !isSHA256Hex(bundle.IntentDigest) {
		return schemagate.AuthorizationBundle{}, fmt.Errorf("authorization bundle requires trace_id, policy_digest, and intent_digest")
	}
	if bundle.OutcomeStatus != "" {
		switch bundle.OutcomeStatus {
		case "allow", "block", "dry_run", "require_approval":
		default:
			return schemagate.AuthorizationBundle{}, fmt.Errorf("authorization bundle outcome_status must be allow|block|dry_run|require_approval")
		}
	}

	type link struct {
		path   *string
		digest *string
	}
	links := []link{
		{&bundle.TracePath, &bundle.TraceDigest},
		{&bundle.ApprovalAuditPath, &bundle.ApprovalAuditDigest},
		{&bundle.CredentialEvidencePath, &bundle.CredentialEvidenceDigest},
		{&bundle.DelegationAuditPath, &bundle.DelegationAuditDigest},
		{&bundle.ContextEvidencePath, &bundle.ContextEvidenceDigest},
		{&bundle.OutcomePath, &bundle.OutcomeDigest},
	}
	for _, current := range links {
		normalizedPath, digest, err := normalizeAuthorizationLink(*current.path, *current.digest, baseDir)
		if err != nil {
			return schemagate.AuthorizationBundle{}, err
		}
		*current.path = normalizedPath
		*current.digest = digest
	}
	return bundle, nil
}

func normalizeAuthorizationLink(pathValue string, digestValue string, baseDir string) (string, string, error) {
	trimmedPath := strings.TrimSpace(pathValue)
	trimmedDigest := strings.ToLower(strings.TrimSpace(digestValue))
	if trimmedPath == "" {
		if trimmedDigest != "" {
			return "", "", fmt.Errorf("authorization bundle digest requires corresponding path")
		}
		return "", "", nil
	}
	cleanedPath := canonicalAuthorizationRelativePath(trimmedPath)
	if cleanedPath == "" || path.IsAbs(cleanedPath) || cleanedPath == "." || cleanedPath == ".." || strings.HasPrefix(cleanedPath, "../") {
		return "", "", fmt.Errorf("authorization evidence path must be local relative: %s", trimmedPath)
	}
	resolvedPath := filepath.Join(baseDir, filepath.FromSlash(cleanedPath))
	// #nosec G304 -- explicit local evidence path input.
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", "", fmt.Errorf("read authorization evidence %s: %w", trimmedPath, err)
	}
	computedDigest, err := digestAuthorizationEvidence(cleanedPath, content)
	if err != nil {
		return "", "", err
	}
	if trimmedDigest != "" && trimmedDigest != computedDigest {
		return "", "", fmt.Errorf("authorization evidence digest mismatch for %s", trimmedPath)
	}
	return cleanedPath, computedDigest, nil
}

func authorizationBundleFiles(bundle schemagate.AuthorizationBundle, baseDir string) ([]zipx.File, error) {
	files := []zipx.File{}
	paths := []string{
		bundle.TracePath,
		bundle.ApprovalAuditPath,
		bundle.CredentialEvidencePath,
		bundle.DelegationAuditPath,
		bundle.ContextEvidencePath,
		bundle.OutcomePath,
	}
	seen := map[string]struct{}{}
	for _, relativeName := range paths {
		if strings.TrimSpace(relativeName) == "" {
			continue
		}
		if _, exists := seen[relativeName]; exists {
			continue
		}
		seen[relativeName] = struct{}{}
		resolvedPath := filepath.Join(baseDir, relativeName)
		// #nosec G304 -- explicit local evidence path input.
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("read authorization evidence %s: %w", relativeName, err)
		}
		files = append(files, zipx.File{Path: authorizationEvidenceArchivePath(relativeName), Data: content, Mode: 0o644})
	}
	return files, nil
}

func verifyAuthorizationPayloadContracts(bundle *openedZip, manifest schemapack.Manifest) error {
	payloadFile, ok := bundle.Files["authorization_bundle.json"]
	if !ok {
		return fmt.Errorf("missing authorization_bundle.json")
	}
	payloadBytes, err := readZipFile(payloadFile)
	if err != nil {
		return fmt.Errorf("read authorization_bundle.json: %w", err)
	}
	var payload schemagate.AuthorizationBundle
	if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
		return fmt.Errorf("parse authorization_bundle.json: %w", err)
	}
	if strings.TrimSpace(payload.SchemaID) != "gait.gate.authorization_bundle" {
		return fmt.Errorf("authorization bundle schema_id must be gait.gate.authorization_bundle")
	}
	if strings.TrimSpace(payload.SchemaVersion) != "1.0.0" {
		return fmt.Errorf("authorization bundle schema_version must be 1.0.0")
	}
	if payload.CreatedAt.IsZero() || strings.TrimSpace(payload.TraceID) == "" || !isSHA256Hex(payload.PolicyDigest) || !isSHA256Hex(payload.IntentDigest) {
		return fmt.Errorf("authorization bundle requires created_at, trace_id, policy_digest, and intent_digest")
	}
	if strings.TrimSpace(payload.OutcomeStatus) != "" {
		switch payload.OutcomeStatus {
		case "allow", "block", "dry_run", "require_approval":
		default:
			return fmt.Errorf("authorization bundle outcome_status must be allow|block|dry_run|require_approval")
		}
	}
	for _, current := range []struct {
		path   string
		digest string
	}{
		{payload.TracePath, payload.TraceDigest},
		{payload.ApprovalAuditPath, payload.ApprovalAuditDigest},
		{payload.CredentialEvidencePath, payload.CredentialEvidenceDigest},
		{payload.DelegationAuditPath, payload.DelegationAuditDigest},
		{payload.ContextEvidencePath, payload.ContextEvidenceDigest},
		{payload.OutcomePath, payload.OutcomeDigest},
	} {
		if strings.TrimSpace(current.path) == "" {
			continue
		}
		internalPath := authorizationEvidenceArchivePath(current.path)
		file, ok := bundle.Files[internalPath]
		if !ok {
			return fmt.Errorf("missing %s", internalPath)
		}
		content, err := readZipFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", internalPath, err)
		}
		actualDigest, err := digestAuthorizationEvidence(current.path, content)
		if err != nil {
			return err
		}
		if actualDigest != strings.ToLower(strings.TrimSpace(current.digest)) {
			return fmt.Errorf("%s digest mismatch", internalPath)
		}
	}
	if strings.TrimSpace(payload.TracePath) == "" || strings.TrimSpace(payload.TraceDigest) == "" {
		return fmt.Errorf("authorization bundle trace_path and trace_digest are required")
	}
	if strings.TrimSpace(payload.OutcomeStatus) != "" && (strings.TrimSpace(payload.OutcomePath) == "" || strings.TrimSpace(payload.OutcomeDigest) == "") {
		return fmt.Errorf("authorization bundle outcome status requires outcome_path and outcome_digest")
	}
	_ = manifest
	return nil
}

func authorizationEvidenceArchivePath(relativeName string) string {
	return path.Join("evidence", canonicalAuthorizationRelativePath(relativeName))
}

func canonicalAuthorizationRelativePath(relativeName string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(relativeName), "\\", "/")
	if normalized == "" {
		return ""
	}
	return path.Clean(normalized)
}

func digestAuthorizationEvidence(name string, payload []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	switch ext {
	case ".json":
		digest, err := jcs.DigestJCS(payload)
		if err != nil {
			return "", fmt.Errorf("digest authorization json evidence %s: %w", name, err)
		}
		return digest, nil
	default:
		return sha256Hex(payload), nil
	}
}

func countAuthorizationLinkedEvidence(bundle schemagate.AuthorizationBundle) int {
	count := 0
	for _, value := range []string{
		bundle.TracePath,
		bundle.ApprovalAuditPath,
		bundle.CredentialEvidencePath,
		bundle.DelegationAuditPath,
		bundle.ContextEvidencePath,
		bundle.OutcomePath,
	} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func countAuthorizationRequiredEvidence(bundle schemagate.AuthorizationBundle) int {
	count := 0
	if strings.TrimSpace(bundle.TracePath) != "" && strings.TrimSpace(bundle.TraceDigest) != "" {
		count++
	}
	if strings.TrimSpace(bundle.OutcomePath) != "" && strings.TrimSpace(bundle.OutcomeDigest) != "" {
		count++
	}
	return count
}
