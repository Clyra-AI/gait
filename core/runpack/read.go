package runpack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

type Runpack struct {
	Manifest schemarunpack.Manifest
	Run      schemarunpack.Run
	Intents  []schemarunpack.IntentRecord
	Results  []schemarunpack.ResultRecord
	Refs     schemarunpack.Refs
}

func ReadRunpack(path string) (Runpack, error) {
	verifyResult, err := VerifyZip(path, VerifyOptions{RequireSignature: false})
	if err != nil {
		return Runpack{}, err
	}
	if len(verifyResult.MissingFiles) > 0 {
		return Runpack{}, fmt.Errorf("missing runpack files: %s", strings.Join(verifyResult.MissingFiles, ", "))
	}
	if len(verifyResult.HashMismatches) > 0 {
		return Runpack{}, fmt.Errorf("runpack hash mismatch")
	}

	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return Runpack{}, fmt.Errorf("open zip: %w", err)
	}
	defer func() {
		_ = zipReader.Close()
	}()

	files := make(map[string]*zip.File, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		files[zipFile.Name] = zipFile
	}

	manifestFile := files["manifest.json"]
	if manifestFile == nil {
		return Runpack{}, fmt.Errorf("missing manifest.json")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read manifest: %w", err)
	}
	runFile := files["run.json"]
	if runFile == nil {
		return Runpack{}, fmt.Errorf("missing run.json")
	}
	runBytes, err := readZipFile(runFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read run: %w", err)
	}
	intentsFile := files["intents.jsonl"]
	if intentsFile == nil {
		return Runpack{}, fmt.Errorf("missing intents.jsonl")
	}
	intentsBytes, err := readZipFile(intentsFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read intents: %w", err)
	}
	resultsFile := files["results.jsonl"]
	if resultsFile == nil {
		return Runpack{}, fmt.Errorf("missing results.jsonl")
	}
	resultsBytes, err := readZipFile(resultsFile)
	if err != nil {
		return Runpack{}, fmt.Errorf("read results: %w", err)
	}
	refsFile := files["refs.json"]
	if refsFile == nil {
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

// no additional helpers
