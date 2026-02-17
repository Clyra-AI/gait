package runpack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Clyra-AI/gait/core/contextproof"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

type Runpack struct {
	Manifest schemarunpack.Manifest
	Run      schemarunpack.Run
	Intents  []schemarunpack.IntentRecord
	Results  []schemarunpack.ResultRecord
	Refs     schemarunpack.Refs
}

func ReadRunpack(path string) (Runpack, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return Runpack{}, fmt.Errorf("open zip: %w", err)
	}
	defer func() {
		_ = zipReader.Close()
	}()

	manifestFile, manifestFound := findZipFile(zipReader.File, "manifest.json")
	if !manifestFound {
		return Runpack{}, fmt.Errorf("missing manifest.json")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read manifest: %w", err)
	}
	runFile, runFound := findZipFile(zipReader.File, "run.json")
	if !runFound {
		return Runpack{}, fmt.Errorf("missing run.json")
	}
	runBytes, err := readZipFile(runFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read run: %w", err)
	}
	intentsFile, intentsFound := findZipFile(zipReader.File, "intents.jsonl")
	if !intentsFound {
		return Runpack{}, fmt.Errorf("missing intents.jsonl")
	}
	intentsBytes, err := readZipFile(intentsFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read intents: %w", err)
	}
	resultsFile, resultsFound := findZipFile(zipReader.File, "results.jsonl")
	if !resultsFound {
		return Runpack{}, fmt.Errorf("missing results.jsonl")
	}
	resultsBytes, err := readZipFile(resultsFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read results: %w", err)
	}
	refsFile, refsFound := findZipFile(zipReader.File, "refs.json")
	if !refsFound {
		return Runpack{}, fmt.Errorf("missing refs.json")
	}
	refsBytes, err := readZipFile(refsFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read refs: %w", err)
	}

	var manifest schemarunpack.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Runpack{}, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.SchemaID != "gait.runpack.manifest" {
		return Runpack{}, fmt.Errorf("manifest schema_id must be gait.runpack.manifest")
	}
	if manifest.SchemaVersion != "1.0.0" {
		return Runpack{}, fmt.Errorf("manifest schema_version must be 1.0.0")
	}
	if manifest.RunID == "" {
		return Runpack{}, fmt.Errorf("manifest missing run_id")
	}

	fileHashes := map[string]string{
		"run.json":      sha256Hex(runBytes),
		"intents.jsonl": sha256Hex(intentsBytes),
		"results.jsonl": sha256Hex(resultsBytes),
		"refs.json":     sha256Hex(refsBytes),
	}
	hasRun := false
	hasIntents := false
	hasResults := false
	hasRefs := false
	missingFiles := make([]string, 0, 4)
	hashMismatch := false
	for _, entry := range manifest.Files {
		name := filepath.ToSlash(entry.Path)
		switch name {
		case "run.json":
			hasRun = true
		case "intents.jsonl":
			hasIntents = true
		case "results.jsonl":
			hasResults = true
		case "refs.json":
			hasRefs = true
		}
		actualHash, known := fileHashes[name]
		if !known {
			zipFile, exists := findZipFile(zipReader.File, name)
			if !exists {
				missingFiles = append(missingFiles, name)
				continue
			}
			actualHash, err = hashZipFile(zipFile)
			if err != nil {
				return Runpack{}, fmt.Errorf("hash %s: %w", name, err)
			}
		}
		if !equalHex(actualHash, entry.SHA256) {
			hashMismatch = true
		}
	}
	if !hasRun {
		missingFiles = append(missingFiles, "run.json")
	}
	if !hasIntents {
		missingFiles = append(missingFiles, "intents.jsonl")
	}
	if !hasResults {
		missingFiles = append(missingFiles, "results.jsonl")
	}
	if !hasRefs {
		missingFiles = append(missingFiles, "refs.json")
	}
	computedManifestDigest, err := computeManifestDigest(manifest)
	if err != nil {
		return Runpack{}, fmt.Errorf("compute manifest digest: %w", err)
	}
	if !equalHex(manifest.ManifestDigest, computedManifestDigest) {
		hashMismatch = true
	}
	if len(missingFiles) > 0 {
		sort.Strings(missingFiles)
		return Runpack{}, fmt.Errorf("missing runpack files: %s", strings.Join(missingFiles, ", "))
	}
	if hashMismatch {
		return Runpack{}, fmt.Errorf("runpack hash mismatch")
	}

	var run schemarunpack.Run
	if err := json.Unmarshal(runBytes, &run); err != nil {
		return Runpack{}, fmt.Errorf("parse run: %w", err)
	}
	intents, err := decodeJSONL[schemarunpack.IntentRecord](intentsBytes)
	if err != nil {
		return Runpack{}, fmt.Errorf("parse intents: %w", err)
	}
	results, err := decodeJSONL[schemarunpack.ResultRecord](resultsBytes)
	if err != nil {
		return Runpack{}, fmt.Errorf("parse results: %w", err)
	}
	var refs schemarunpack.Refs
	if err := json.Unmarshal(refsBytes, &refs); err != nil {
		return Runpack{}, fmt.Errorf("parse refs: %w", err)
	}
	refs, err = contextproof.NormalizeRefs(refs)
	if err != nil {
		return Runpack{}, fmt.Errorf("normalize refs: %w", err)
	}
	if refs.ContextEvidenceMode == contextproof.EvidenceModeRequired && strings.TrimSpace(refs.ContextSetDigest) == "" {
		return Runpack{}, fmt.Errorf("context evidence mode required but context_set_digest is missing")
	}

	return Runpack{
		Manifest: manifest,
		Run:      run,
		Intents:  intents,
		Results:  results,
		Refs:     refs,
	}, nil
}

func decodeJSONL[T any](data []byte) ([]T, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var records []T
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		records = append(records, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan jsonl: %w", err)
	}
	return records, nil
}
