package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCheckRejectsAndUpdateRemovesOrphanActivationWithoutDeletingUnrelatedFiles(t *testing.T) {
	root := copyFixtureRepo(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}

	proposal := filepath.Join(root, fixtureRootRel, "expected", "compensation", "pac-4b7f1402784256ce.json")
	unrelated := filepath.Join(root, fixtureRootRel, "expected", "keep.txt")
	if err := os.WriteFile(unrelated, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, fixtureRootRel, "expected", "removed-scenario", activationName)
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(root, true); err == nil {
		t.Fatal("--check accepted an orphan activation fixture")
	}
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan activation was not removed: %v", err)
	}
	if got, err := os.ReadFile(unrelated); err != nil || string(got) != "keep\n" {
		t.Fatalf("unrelated file changed or removed: %q %v", got, err)
	}
	if _, err := os.Stat(proposal); err != nil {
		t.Fatalf("proposal was removed during activation cleanup: %v", err)
	}
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
}

func TestRunUpdateRemovesActivationForRemovedScenario(t *testing.T) {
	root := copyFixtureRepo(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	activation := filepath.Join(root, fixtureRootRel, "expected", "workflow-to-deploy", activationName)
	proposal := filepath.Join(root, fixtureRootRel, "expected", "workflow-to-deploy", "pac-78931e17d6204821.json")
	removeScenario(t, root, "workflow-to-deploy")
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(activation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed scenario activation still exists: %v", err)
	}
	if _, err := os.Stat(proposal); err != nil {
		t.Fatalf("removed scenario proposal was deleted: %v", err)
	}
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
}

func TestRunUpdateRemovesActivationWhenScenarioBecomesRejected(t *testing.T) {
	root := copyFixtureRepo(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	activation := filepath.Join(root, fixtureRootRel, "expected", "compensation", activationName)
	proposal := filepath.Join(root, fixtureRootRel, "expected", "compensation", "pac-4b7f1402784256ce.json")
	if err := makeProposalDigestInvalid(proposal); err != nil {
		t.Fatal(err)
	}
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(activation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected scenario activation still exists: %v", err)
	}
	if _, err := os.Stat(proposal); err != nil {
		t.Fatalf("rejected scenario proposal was deleted: %v", err)
	}
	manifest := readJSON(t, filepath.Join(root, fixtureRootRel, "expected", "activation-fixture-manifest.json"))
	for _, item := range manifest["scenarios"].([]any) {
		scenario := item.(map[string]any)
		if scenario["scenario_id"] == "compensation" && scenario["activation_status"] != "not_activated" {
			t.Fatalf("compensation was not recorded as rejected: %#v", scenario)
		}
	}
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsScenarioPathTraversalAndSymlinkActivation(t *testing.T) {
	root := copyFixtureRepo(t)
	manifestPath := filepath.Join(root, manifestRel)
	manifest := readJSON(t, manifestPath)
	manifest["scenarios"].([]any)[0].(map[string]any)["scenario_id"] = "../escaped"
	writeJSON(t, manifestPath, manifest)
	if err := run(root, false); err == nil {
		t.Fatal("scenario path traversal was accepted")
	}

	root = copyFixtureRepo(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, fixtureRootRel, "expected", "orphan", activationName)
	if err := os.MkdirAll(filepath.Dir(symlink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.txt"), symlink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("must remain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(root, false); err == nil {
		t.Fatal("symlink activation was accepted for cleanup")
	}
	if got, err := os.ReadFile(filepath.Join(root, "outside.txt")); err != nil || string(got) != "must remain\n" {
		t.Fatalf("symlink target changed: %q %v", got, err)
	}
}

func TestFixturePathHelpersAndCopyRejectUnsafeDestinations(t *testing.T) {
	root := t.TempDir()
	expectedRoot := filepath.Join(root, "expected")
	if err := os.MkdirAll(expectedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := safeFixturePath(expectedRoot, "../escape"); err == nil {
		t.Fatal("safeFixturePath accepted traversal")
	}
	if _, err := safeFixturePath(expectedRoot, filepath.Join(root, "absolute")); err == nil {
		t.Fatal("safeFixturePath accepted an absolute path")
	}
	if err := rejectSymlinkAncestors(expectedRoot, filepath.Join(expectedRoot, "missing", "file")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkRoot := filepath.Join(root, "symlink-root")
	if err := os.Symlink(expectedRoot, symlinkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := rejectSymlinkAncestors(symlinkRoot, filepath.Join(symlinkRoot, "file")); err == nil {
		t.Fatal("rejectSymlinkAncestors accepted a symlink root")
	}

	source := filepath.Join(root, "source.json")
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	regularDestination := filepath.Join(expectedRoot, "regular.json")
	if err := compareOrCopy(source, regularDestination, false); err != nil {
		t.Fatal(err)
	}
	if err := compareOrCopy(source, regularDestination, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(expectedRoot, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := compareOrCopy(source, filepath.Join(expectedRoot, "directory"), false); err == nil {
		t.Fatal("compareOrCopy accepted a directory destination")
	}
	stale := filepath.Join(expectedRoot, "stale.json")
	if err := os.WriteFile(stale, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := compareOrCopy(source, stale, true); err == nil {
		t.Fatal("--check accepted stale fixture bytes")
	}
	if err := compareOrCopy(filepath.Join(root, "missing-source.json"), regularDestination, false); err == nil {
		t.Fatal("compareOrCopy accepted a missing source")
	}
	symlinkDestination := filepath.Join(expectedRoot, "symlink.json")
	if err := os.Symlink(outside, symlinkDestination); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := compareOrCopy(source, symlinkDestination, false); err == nil {
		t.Fatal("compareOrCopy accepted a symlink destination")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
		t.Fatalf("outside file changed: %q %v", got, err)
	}
}

func TestCollectActivationFilesRejectsInvalidRootAndNonRegularActivation(t *testing.T) {
	root := t.TempDir()
	if _, err := collectActivationFiles(filepath.Join(root, "missing")); err == nil {
		t.Fatal("collectActivationFiles accepted a missing root")
	}
	fileRoot := filepath.Join(root, "file-root")
	if err := os.WriteFile(fileRoot, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := collectActivationFiles(fileRoot); err == nil {
		t.Fatal("collectActivationFiles accepted a regular-file root")
	}
	fixtureRoot := filepath.Join(root, "fixture-root")
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "scenario"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(fixtureRoot, "scenario", activationName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := collectActivationFiles(fixtureRoot); err == nil {
		t.Fatal("collectActivationFiles accepted a directory activation fixture")
	}
}

func TestRunRejectsInvalidSourceManifestInputs(t *testing.T) {
	if err := run(t.TempDir(), false); err == nil {
		t.Fatal("run accepted a repository without the pinned source manifest")
	}

	root := copyFixtureRepo(t)
	manifestPath := filepath.Join(root, manifestRel)
	manifest := readJSON(t, manifestPath)
	manifest["producer"].(map[string]any)["version"] = "v9.9.9"
	writeJSON(t, manifestPath, manifest)
	if err := run(root, false); err == nil {
		t.Fatal("run accepted an unsupported source producer version")
	}

	root = copyFixtureRepo(t)
	manifestPath = filepath.Join(root, manifestRel)
	manifest = readJSON(t, manifestPath)
	scenarios := manifest["scenarios"].([]any)
	scenarios[1].(map[string]any)["scenario_id"] = scenarios[0].(map[string]any)["scenario_id"]
	writeJSON(t, manifestPath, manifest)
	if err := run(root, false); err == nil {
		t.Fatal("run accepted duplicate source scenario IDs")
	}
}

func copyFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join("..", "..", fixtureRootRel)
	destination := filepath.Join(root, fixtureRootRel)
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("unexpected symlink in source fixture")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func writeJSON(t *testing.T, path string, value map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func removeScenario(t *testing.T, root, scenarioID string) {
	t.Helper()
	path := filepath.Join(root, manifestRel)
	manifest := readJSON(t, path)
	scenarios := manifest["scenarios"].([]any)
	filtered := make([]any, 0, len(scenarios))
	for _, item := range scenarios {
		if item.(map[string]any)["scenario_id"] != scenarioID {
			filtered = append(filtered, item)
		}
	}
	manifest["scenarios"] = filtered
	writeJSON(t, path, manifest)
}

func makeProposalDigestInvalid(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	old := []byte(`"canonical_content_digest": "sha256:`)
	index := bytes.Index(raw, old)
	if index < 0 {
		return errors.New("proposal digest field missing")
	}
	start := index + len(old)
	end := start + 64
	if end > len(raw) {
		return errors.New("proposal digest field truncated")
	}
	copy(raw[start:end], bytes.Repeat([]byte{'0'}, 64))
	return os.WriteFile(path, raw, 0o644)
}
