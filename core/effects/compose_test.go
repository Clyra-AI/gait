package effects

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestComposeRunnerDependencyAndScope(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: compose, Project: "bad/project", DockerBinary: "missing-docker"}); got.ReasonCodes[0] != "compose_project_invalid" {
		t.Fatalf("project scope: %#v", got)
	}
	if got := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: compose, Project: "gait_test", DockerBinary: "missing-docker"}); got.ReasonCodes[0] != "compose_dependency_missing" {
		t.Fatalf("dependency: %#v", got)
	}
	if err := ValidateComposeOptions(ComposeRunOptions{ComposeFile: compose, Project: "gait_test", WorkingDir: root}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateComposeOptions(ComposeRunOptions{ComposeFile: filepath.Join(root, "..", "outside.yaml"), Project: "gait_test"}); err != nil {
		t.Fatal(err)
	}
	if ComposeProjectValid("bad/project") || !ComposeProjectValid("gait_test") || ComposeDependencyAvailable("definitely-missing-docker") {
		t.Fatal("compose helper validation drift")
	}
	if _, err := MarshalComposeCollectorResult(CaptureResult{Complete: true}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	writer := &limitedComposeBuffer{Buffer: &buf, Limit: 4}
	if _, err := writer.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("12345")); err == nil {
		t.Fatal("collector writer exceeded bound")
	}
}

func TestComposeRunnerResolvesRelativeFileAgainstWorkingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed fake compose binary is not portable")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	docker := filepath.Join(root, "docker-fake")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	result := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: "compose.yaml", Project: "gait_relative", WorkingDir: root, DockerBinary: docker, MountedPath: root})
	if result.Status != "pass" || result.ComposeFile != filepath.Join(root, "compose.yaml") {
		t.Fatalf("relative compose file was not resolved once: %#v", result)
	}
}

func TestComposeRunnerOptionAndExecutionFailureBranches(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, options := range []ComposeRunOptions{{ComposeFile: compose, Project: "bad/project"}, {ComposeFile: "", Project: "valid"}, {ComposeFile: filepath.Join(root, "missing"), Project: "valid"}, {ComposeFile: compose, Project: "valid", WorkingDir: root, MountedPath: filepath.Join(root, "..", "outside")}} {
		result := RunCompose(context.Background(), options)
		if result.Status == "pass" {
			t.Fatalf("invalid options passed: %#v", options)
		}
	}
}

func TestComposePostgresCollectorCommandStrictBounded(t *testing.T) {
	t.Setenv("GAIT_COMPOSE_HELPER", "1")
	result := CaptureResult{Observation: Observation{Identity: "postgres:orders", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: ObservationPresent}, Complete: true}
	raw, _ := json.Marshal(result)
	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.json")
	_ = os.WriteFile(validPath, raw, 0o600)
	t.Setenv("GAIT_COMPOSE_PATH", validPath)
	got, err := runComposeCollector(context.Background(), []string{os.Args[0], "-test.run=TestComposeCollectorHelperProcess"}, "")
	if err != nil || got.Observation.Identity != result.Observation.Identity {
		t.Fatalf("collector JSON: %#v %v", got, err)
	}
	badPath := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badPath, []byte("not-json"), 0o600)
	t.Setenv("GAIT_COMPOSE_PATH", badPath)
	if _, err := runComposeCollector(context.Background(), []string{os.Args[0], "-test.run=TestComposeCollectorHelperProcess"}, ""); err == nil {
		t.Fatal("malformed collector accepted")
	}
	short, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	t.Setenv("GAIT_COMPOSE_SLEEP", "1")
	if _, err := runComposeCollector(short, []string{os.Args[0], "-test.run=TestComposeCollectorSleepProcess"}, ""); err == nil {
		t.Fatal("timed-out collector accepted")
	}
	oversized := strings.Repeat("x", maxComposeCollectorBytes+1)
	overPath := filepath.Join(dir, "oversized.json")
	_ = os.WriteFile(overPath, []byte(oversized), 0o600)
	t.Setenv("GAIT_COMPOSE_PATH", overPath)
	if _, err := runComposeCollector(context.Background(), []string{os.Args[0], "-test.run=TestComposeCollectorHelperProcess"}, ""); err == nil {
		t.Fatal("oversized collector accepted")
	}
}

func TestComposeCollectorHelperProcess(t *testing.T) {
	if os.Getenv("GAIT_COMPOSE_HELPER") != "1" {
		t.Skip("helper process")
	}
	raw, _ := os.ReadFile(os.Getenv("GAIT_COMPOSE_PATH"))
	_, _ = fmt.Print(string(raw))
	os.Exit(0)
}
func TestComposeCollectorSleepProcess(t *testing.T) {
	if os.Getenv("GAIT_COMPOSE_SLEEP") != "1" {
		t.Skip("helper process")
	}
	select {}
}

func TestComposeRunnerPairedFakePostgresAndProxy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed fake compose binary is not portable")
	}
	root := t.TempDir()
	compose := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	docker := filepath.Join(root, "docker-fake")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	count := 0
	capture := func(req CaptureRequest) (CaptureResult, error) {
		count++
		return CaptureResult{Observation: Observation{Identity: req.Path + req.URL + req.Reference, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: ObservationPresent}, Complete: true}, nil
	}
	result := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: compose, Project: "gait_test", WorkingDir: root, DockerBinary: docker, Service: "postgres", ProxyURL: "http://proxy.local/observe", PostgresReference: "public.orders", Capture: capture, Run: []string{"true"}})
	if result.Status != "pass" || len(result.Before) != 2 || len(result.After) != 2 || count != 4 {
		t.Fatalf("paired observations: %#v", result)
	}
	filesystemOnly := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: compose, Project: "gait_filesystem_only", WorkingDir: root, DockerBinary: docker, Service: "postgres", MountedPath: root})
	if filesystemOnly.Status != "pass" || len(filesystemOnly.Before) != 1 || len(filesystemOnly.After) != 1 {
		t.Fatalf("default postgres collector should be optional: %#v", filesystemOnly)
	}
	collectorCalls := 0
	customService := RunCompose(context.Background(), ComposeRunOptions{ComposeFile: compose, Project: "gait_custom_service", WorkingDir: root, DockerBinary: docker, Service: "db", PostgresCollector: func(context.Context) (CaptureResult, error) {
		collectorCalls++
		return CaptureResult{Observation: Observation{Identity: "db:orders", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: ObservationPresent}, Complete: true}, nil
	}})
	if customService.Status != "pass" || collectorCalls != 2 || len(customService.Before) != 1 || len(customService.After) != 1 {
		t.Fatalf("custom service collector was not observed in pairs: %#v calls=%d", customService, collectorCalls)
	}
}
