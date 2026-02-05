package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveRunpackPath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing run_id or path")
	}
	if looksLikePath(input) {
		if fileExists(input) {
			return input, nil
		}
		return "", fmt.Errorf("runpack not found: %s", input)
	}

	runpackName := fmt.Sprintf("runpack_%s.zip", input)
	candidates := []string{
		filepath.Join(".", "gait-out", runpackName),
	}
	homeDir, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(homeDir, ".gait", "runpacks", runpackName))
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("runpack not found for run_id: %s", input)
}

func looksLikePath(input string) bool {
	if strings.Contains(input, string(os.PathSeparator)) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(input), ".zip")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
