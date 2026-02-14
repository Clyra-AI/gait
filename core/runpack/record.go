package runpack

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/davidahmann/gait/core/contextproof"
	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/jcs"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
	"github.com/davidahmann/gait/core/sign"
	"github.com/davidahmann/gait/core/zipx"
)

type RecordOptions struct {
	Run         schemarunpack.Run
	Intents     []schemarunpack.IntentRecord
	Results     []schemarunpack.ResultRecord
	Refs        schemarunpack.Refs
	CaptureMode string
	SignKey     ed25519.PrivateKey
}

type RecordResult struct {
	RunID    string
	Manifest schemarunpack.Manifest
	ZipBytes []byte
}

func RecordRun(options RecordOptions) (RecordResult, error) {
	if options.Run.RunID == "" {
		return RecordResult{}, fmt.Errorf("run_id is required")
	}

	run := options.Run
	applyRunDefaults(&run)

	intents := applyIntentDefaults(options.Intents, run)
	results := applyResultDefaults(options.Results, run)
	refs, err := applyRefsDefaults(options.Refs, run)
	if err != nil {
		return RecordResult{}, fmt.Errorf("normalize refs: %w", err)
	}

	captureMode := options.CaptureMode
	if captureMode == "" {
		captureMode = "reference"
	}

	runBytes, err := canonicalJSON(run)
	if err != nil {
		return RecordResult{}, fmt.Errorf("encode run.json: %w", err)
	}
	intentsBytes, err := canonicalJSONL(intents)
	if err != nil {
		return RecordResult{}, fmt.Errorf("encode intents.jsonl: %w", err)
	}
	resultsBytes, err := canonicalJSONL(results)
	if err != nil {
		return RecordResult{}, fmt.Errorf("encode results.jsonl: %w", err)
	}
	refsBytes, err := canonicalJSON(refs)
	if err != nil {
		return RecordResult{}, fmt.Errorf("encode refs.json: %w", err)
	}

	manifest := schemarunpack.Manifest{
		SchemaID:        "gait.runpack.manifest",
		SchemaVersion:   "1.0.0",
		CreatedAt:       run.CreatedAt,
		ProducerVersion: run.ProducerVersion,
		RunID:           run.RunID,
		CaptureMode:     captureMode,
		Files: []schemarunpack.ManifestFile{
			{Path: "run.json", SHA256: sha256Hex(runBytes)},
			{Path: "intents.jsonl", SHA256: sha256Hex(intentsBytes)},
			{Path: "results.jsonl", SHA256: sha256Hex(resultsBytes)},
			{Path: "refs.json", SHA256: sha256Hex(refsBytes)},
		},
	}
	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].Path < manifest.Files[j].Path
	})
	manifestDigest, err := computeManifestDigest(manifest)
	if err != nil {
		return RecordResult{}, fmt.Errorf("compute manifest digest: %w", err)
	}
	manifest.ManifestDigest = manifestDigest

	if len(options.SignKey) > 0 {
		rawManifest, err := json.Marshal(manifest)
		if err != nil {
			return RecordResult{}, fmt.Errorf("marshal manifest: %w", err)
		}
		signableBytes, err := signableManifestBytes(rawManifest)
		if err != nil {
			return RecordResult{}, fmt.Errorf("prepare manifest signature: %w", err)
		}
		sig, err := sign.SignManifestJSON(options.SignKey, signableBytes)
		if err != nil {
			return RecordResult{}, fmt.Errorf("sign manifest: %w", err)
		}
		manifest.Signatures = []schemarunpack.Signature{{
			Alg:          sig.Alg,
			KeyID:        sig.KeyID,
			Sig:          sig.Sig,
			SignedDigest: sig.SignedDigest,
		}}
	}

	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		return RecordResult{}, fmt.Errorf("encode manifest.json: %w", err)
	}

	files := []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
		{Path: "run.json", Data: runBytes, Mode: 0o644},
		{Path: "intents.jsonl", Data: intentsBytes, Mode: 0o644},
		{Path: "results.jsonl", Data: resultsBytes, Mode: 0o644},
		{Path: "refs.json", Data: refsBytes, Mode: 0o644},
	}
	var buffer bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buffer, files); err != nil {
		return RecordResult{}, fmt.Errorf("write runpack zip: %w", err)
	}

	return RecordResult{
		RunID:    run.RunID,
		Manifest: manifest,
		ZipBytes: buffer.Bytes(),
	}, nil
}

func WriteRunpack(path string, options RecordOptions) (RecordResult, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return RecordResult{}, err
	}

	result, err := RecordRun(options)
	if err != nil {
		return RecordResult{}, err
	}

	dir := filepath.Dir(normalizedPath)
	if dir != "." && dir != "" {
		if filepath.IsLocal(dir) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return RecordResult{}, fmt.Errorf("create runpack directory: %w", err)
			}
		} else if strings.HasPrefix(dir, string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return RecordResult{}, fmt.Errorf("create runpack directory: %w", err)
			}
		} else if volume := filepath.VolumeName(dir); volume != "" && strings.HasPrefix(dir, volume+string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return RecordResult{}, fmt.Errorf("create runpack directory: %w", err)
			}
		} else {
			return RecordResult{}, fmt.Errorf("runpack output directory must be local relative or absolute")
		}
	}
	if err := fsx.WriteFileAtomic(normalizedPath, result.ZipBytes, 0o600); err != nil {
		return RecordResult{}, fmt.Errorf("write runpack: %w", err)
	}
	return result, nil
}

func normalizeOutputPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("runpack output path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("runpack output path is required")
	}
	if !filepath.IsAbs(cleaned) {
		segments := strings.Split(filepath.ToSlash(cleaned), "/")
		for _, segment := range segments {
			if segment == ".." {
				return "", fmt.Errorf("relative runpack output path must not traverse parent directories")
			}
		}
	}
	return cleaned, nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jcs.CanonicalizeJSON(raw)
}

func canonicalJSONL[T any](records []T) ([]byte, error) {
	if len(records) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	for _, record := range records {
		line, err := canonicalJSON(record)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func computeManifestDigest(manifest schemarunpack.Manifest) (string, error) {
	manifest.ManifestDigest = ""
	manifest.Signatures = nil
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func applyRunDefaults(run *schemarunpack.Run) {
	if run.SchemaID == "" {
		run.SchemaID = "gait.runpack.run"
	}
	if run.SchemaVersion == "" {
		run.SchemaVersion = "1.0.0"
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	if run.ProducerVersion == "" {
		run.ProducerVersion = "0.0.0-dev"
	}
	if run.Env.OS == "" {
		run.Env.OS = runtime.GOOS
	}
	if run.Env.Arch == "" {
		run.Env.Arch = runtime.GOARCH
	}
	if run.Env.Runtime == "" {
		run.Env.Runtime = "go"
	}
}

func applyIntentDefaults(intents []schemarunpack.IntentRecord, run schemarunpack.Run) []schemarunpack.IntentRecord {
	out := make([]schemarunpack.IntentRecord, len(intents))
	for i, intent := range intents {
		if intent.SchemaID == "" {
			intent.SchemaID = "gait.runpack.intent"
		}
		if intent.SchemaVersion == "" {
			intent.SchemaVersion = "1.0.0"
		}
		if intent.CreatedAt.IsZero() {
			intent.CreatedAt = run.CreatedAt
		}
		if intent.ProducerVersion == "" {
			intent.ProducerVersion = run.ProducerVersion
		}
		if intent.RunID == "" {
			intent.RunID = run.RunID
		}
		out[i] = intent
	}
	return out
}

func applyResultDefaults(results []schemarunpack.ResultRecord, run schemarunpack.Run) []schemarunpack.ResultRecord {
	out := make([]schemarunpack.ResultRecord, len(results))
	for i, result := range results {
		if result.SchemaID == "" {
			result.SchemaID = "gait.runpack.result"
		}
		if result.SchemaVersion == "" {
			result.SchemaVersion = "1.0.0"
		}
		if result.CreatedAt.IsZero() {
			result.CreatedAt = run.CreatedAt
		}
		if result.ProducerVersion == "" {
			result.ProducerVersion = run.ProducerVersion
		}
		if result.RunID == "" {
			result.RunID = run.RunID
		}
		out[i] = result
	}
	return out
}

func applyRefsDefaults(refs schemarunpack.Refs, run schemarunpack.Run) (schemarunpack.Refs, error) {
	if refs.SchemaID == "" {
		refs.SchemaID = "gait.runpack.refs"
	}
	if refs.SchemaVersion == "" {
		refs.SchemaVersion = "1.0.0"
	}
	if refs.CreatedAt.IsZero() {
		refs.CreatedAt = run.CreatedAt
	}
	if refs.ProducerVersion == "" {
		refs.ProducerVersion = run.ProducerVersion
	}
	if refs.RunID == "" {
		refs.RunID = run.RunID
	}
	if refs.Receipts == nil {
		refs.Receipts = []schemarunpack.RefReceipt{}
	}
	normalized, err := contextproof.NormalizeRefs(refs)
	if err != nil {
		return schemarunpack.Refs{}, err
	}
	return normalized, nil
}
