package scenarios

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateScenarioFixtures(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	repoRoot, err := findRepoRoot(cwd)
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	scenarioRoot := filepath.Join(repoRoot, scenarioRootRelativePath)

	entries, err := os.ReadDir(scenarioRoot)
	if err != nil {
		t.Fatalf("read scenario root: %v", err)
	}

	seenScenarios := map[string]struct{}{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scenarioName := entry.Name()
		scenarioPath := filepath.Join(scenarioRoot, scenarioName)
		seenScenarios[scenarioName] = struct{}{}

		requiredFiles, known := requiredScenarioMinimumFiles[scenarioName]
		if !known {
			t.Fatalf("unexpected scenario directory: %s", scenarioName)
		}
		for _, required := range requiredFiles {
			filePath := filepath.Join(scenarioPath, required)
			info, statErr := os.Stat(filePath)
			if statErr != nil {
				t.Fatalf("missing required file %s: %v", filePath, statErr)
			}
			if info.IsDir() {
				t.Fatalf("required file is a directory: %s", filePath)
			}
		}

		walkErr := filepath.WalkDir(scenarioPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			return validateFixtureFileSyntax(path)
		})
		if walkErr != nil {
			t.Fatalf("validate fixture syntax for %s: %v", scenarioName, walkErr)
		}
	}

	if len(seenScenarios) != len(requiredScenarioMinimumFiles) {
		known := make([]string, 0, len(requiredScenarioMinimumFiles))
		for name := range requiredScenarioMinimumFiles {
			known = append(known, name)
		}
		sort.Strings(known)

		seen := make([]string, 0, len(seenScenarios))
		for name := range seenScenarios {
			seen = append(seen, name)
		}
		sort.Strings(seen)
		t.Fatalf("scenario count mismatch: expected=%d got=%d expected_names=%v got_names=%v", len(requiredScenarioMinimumFiles), len(seenScenarios), known, seen)
	}

	t.Logf("validated %d scenarios under %s", len(seenScenarios), scenarioRoot)
}

func validateFixtureFileSyntax(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var parsed any
		if err := yaml.Unmarshal(payload, &parsed); err != nil {
			return err
		}
	case ".json":
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var parsed any
		if err := json.Unmarshal(payload, &parsed); err != nil {
			return err
		}
	case ".jsonl":
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = file.Close()
		}()

		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}
			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	return nil
}
