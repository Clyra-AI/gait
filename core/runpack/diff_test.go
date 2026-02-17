package runpack

import (
	"bytes"
	"testing"
	"time"

	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
	"github.com/Clyra-AI/gait/core/zipx"
)

func TestDiffRunpacksNoChange(t *testing.T) {
	left := writeTestRunpack(t, "run_diff", buildIntents("intent_1"), buildResults("intent_1"))
	right := writeTestRunpack(t, "run_diff", buildIntents("intent_1"), buildResults("intent_1"))

	result, err := DiffRunpacks(left, right, DiffPrivacyFull)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if result.Summary.ManifestChanged || result.Summary.IntentsChanged || result.Summary.ResultsChanged {
		t.Fatalf("expected no changes")
	}
}

func TestDiffRunpacksIntentChangedMetadata(t *testing.T) {
	leftIntents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"foo": "bar"},
		},
	}
	rightIntents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"foo": "baz"},
		},
	}
	left := writeTestRunpack(t, "run_left", leftIntents, buildResults("intent_1"))
	right := writeTestRunpack(t, "run_right", rightIntents, buildResults("intent_1"))

	result, err := DiffRunpacks(left, right, DiffPrivacyMetadata)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if result.Summary.IntentsChanged {
		t.Fatalf("expected intents unchanged in metadata mode")
	}
}

func TestDiffRunpacksIntentChangedFull(t *testing.T) {
	leftIntents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"foo": "bar"},
		},
	}
	rightIntents := []schemarunpack.IntentRecord{
		{
			IntentID:   "intent_1",
			ToolName:   "tool.demo",
			ArgsDigest: "2222222222222222222222222222222222222222222222222222222222222222",
			Args:       map[string]any{"foo": "baz"},
		},
	}
	left := writeTestRunpack(t, "run_left", leftIntents, buildResults("intent_1"))
	right := writeTestRunpack(t, "run_right", rightIntents, buildResults("intent_1"))

	result, err := DiffRunpacks(left, right, DiffPrivacyFull)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if !result.Summary.IntentsChanged {
		t.Fatalf("expected intents changed in full mode")
	}
}

func TestDiffRunpacksResultsChangedMetadata(t *testing.T) {
	leftResults := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
			Result:       map[string]any{"ok": true},
		},
	}
	rightResults := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
			Result:       map[string]any{"ok": false},
		},
	}
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), leftResults)
	right := writeTestRunpack(t, "run_right", buildIntents("intent_1"), rightResults)

	result, err := DiffRunpacks(left, right, DiffPrivacyMetadata)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if result.Summary.ResultsChanged {
		t.Fatalf("expected results unchanged in metadata mode")
	}
}

func TestDiffRunpacksResultsChangedFull(t *testing.T) {
	leftResults := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
			Result:       map[string]any{"ok": true},
		},
	}
	rightResults := []schemarunpack.ResultRecord{
		{
			IntentID:     "intent_1",
			Status:       "ok",
			ResultDigest: "3333333333333333333333333333333333333333333333333333333333333333",
			Result:       map[string]any{"ok": false},
		},
	}
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), leftResults)
	right := writeTestRunpack(t, "run_right", buildIntents("intent_1"), rightResults)

	result, err := DiffRunpacks(left, right, DiffPrivacyFull)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if !result.Summary.ResultsChanged {
		t.Fatalf("expected results changed in full mode")
	}
}

func TestDiffRunpacksRefsChangedMetadata(t *testing.T) {
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), buildResults("intent_1"))
	right := writeTestRunpack(t, "run_right", buildIntents("intent_1"), buildResults("intent_1"))

	// mutate refs by creating a different runpack with extra receipt
	rightPack, err := ReadRunpack(right)
	if err != nil {
		t.Fatalf("read runpack: %v", err)
	}
	rightPack.Refs.Receipts = append(rightPack.Refs.Receipts, schemarunpack.RefReceipt{
		RefID:         "ref_extra",
		SourceType:    "demo",
		SourceLocator: "extra",
		QueryDigest:   "4444444444444444444444444444444444444444444444444444444444444444",
		ContentDigest: "5555555555555555555555555555555555555555555555555555555555555555",
		RetrievedAt:   time.Date(2026, time.February, 5, 0, 0, 0, 0, time.UTC),
		RedactionMode: "reference",
	})
	tmpPath := writeTempZip(t, buildCustomRunpackZip(t, rightPack))

	result, err := DiffRunpacks(left, tmpPath, DiffPrivacyMetadata)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if !result.Summary.RefsChanged {
		t.Fatalf("expected refs changed in metadata mode")
	}
}

func TestDiffRunpacksInvalidPrivacy(t *testing.T) {
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), buildResults("intent_1"))
	right := writeTestRunpack(t, "run_right", buildIntents("intent_1"), buildResults("intent_1"))

	if _, err := DiffRunpacks(left, right, DiffPrivacy("bad")); err == nil {
		t.Fatalf("expected error for invalid privacy")
	}
}

func TestDiffRunpacksLeftOnlyIntent(t *testing.T) {
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), buildResults("intent_1"))
	right := writeTestRunpack(t, "run_right", buildIntents("intent_2"), buildResults("intent_2"))

	result, err := DiffRunpacks(left, right, DiffPrivacyFull)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if len(result.Summary.LeftOnlyIntents) == 0 || len(result.Summary.RightOnlyIntents) == 0 {
		t.Fatalf("expected intent id differences")
	}
}

func TestDiffRunpacksFilesChanged(t *testing.T) {
	left := writeTestRunpack(t, "run_left", buildIntents("intent_1"), buildResults("intent_1"))
	right := writeTestRunpack(t, "run_left", buildIntents("intent_1"), buildResults("intent_1"))

	rightPack, err := ReadRunpack(right)
	if err != nil {
		t.Fatalf("read runpack: %v", err)
	}
	extra := zipx.File{Path: "extra.json", Data: []byte(`{"ok":true}`), Mode: 0o644}
	tmpPath := writeTempZip(t, buildCustomRunpackZip(t, rightPack, extra))

	result, err := DiffRunpacks(left, tmpPath, DiffPrivacyFull)
	if err != nil {
		t.Fatalf("diff runpacks: %v", err)
	}
	if len(result.Summary.FilesChanged) == 0 {
		t.Fatalf("expected file changes")
	}
}

func buildCustomRunpackZip(t *testing.T, pack Runpack, extraFiles ...zipx.File) []byte {
	t.Helper()
	runBytes, err := canonicalJSON(pack.Run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	intentsBytes, err := canonicalJSONL(pack.Intents)
	if err != nil {
		t.Fatalf("marshal intents: %v", err)
	}
	resultsBytes, err := canonicalJSONL(pack.Results)
	if err != nil {
		t.Fatalf("marshal results: %v", err)
	}
	refsBytes, err := canonicalJSON(pack.Refs)
	if err != nil {
		t.Fatalf("marshal refs: %v", err)
	}
	schemaID := pack.Manifest.SchemaID
	if schemaID == "" {
		schemaID = "gait.runpack.manifest"
	}
	schemaVersion := pack.Manifest.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = "1.0.0"
	}
	captureMode := pack.Manifest.CaptureMode
	if captureMode == "" {
		captureMode = "reference"
	}
	manifest := schemarunpack.Manifest{
		SchemaID:        schemaID,
		SchemaVersion:   schemaVersion,
		CreatedAt:       pack.Run.CreatedAt,
		ProducerVersion: pack.Run.ProducerVersion,
		RunID:           pack.Run.RunID,
		CaptureMode:     captureMode,
		Files: []schemarunpack.ManifestFile{
			{Path: "run.json", SHA256: sha256Hex(runBytes)},
			{Path: "intents.jsonl", SHA256: sha256Hex(intentsBytes)},
			{Path: "results.jsonl", SHA256: sha256Hex(resultsBytes)},
			{Path: "refs.json", SHA256: sha256Hex(refsBytes)},
		},
	}
	for _, extra := range extraFiles {
		manifest.Files = append(manifest.Files, schemarunpack.ManifestFile{
			Path:   extra.Path,
			SHA256: sha256Hex(extra.Data),
		})
	}
	manifestDigest, err := computeManifestDigest(manifest)
	if err != nil {
		t.Fatalf("digest manifest: %v", err)
	}
	manifest.ManifestDigest = manifestDigest
	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	files := []zipx.File{
		{Path: "manifest.json", Data: manifestBytes, Mode: 0o644},
		{Path: "run.json", Data: runBytes, Mode: 0o644},
		{Path: "intents.jsonl", Data: intentsBytes, Mode: 0o644},
		{Path: "results.jsonl", Data: resultsBytes, Mode: 0o644},
		{Path: "refs.json", Data: refsBytes, Mode: 0o644},
	}
	files = append(files, extraFiles...)
	var buf bytes.Buffer
	if err := zipx.WriteDeterministicZip(&buf, files); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	return buf.Bytes()
}
