package runpack

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Clyra-AI/gait/core/contextproof"
	"github.com/Clyra-AI/gait/core/jcs"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

type DiffPrivacy string

const (
	DiffPrivacyFull     DiffPrivacy = "full"
	DiffPrivacyMetadata DiffPrivacy = "metadata"
)

type DiffSummary struct {
	RunIDLeft                  string   `json:"run_id_left"`
	RunIDRight                 string   `json:"run_id_right"`
	ManifestChanged            bool     `json:"manifest_changed"`
	FilesChanged               []string `json:"files_changed,omitempty"`
	IntentsChanged             bool     `json:"intents_changed"`
	ResultsChanged             bool     `json:"results_changed"`
	RefsChanged                bool     `json:"refs_changed"`
	LeftOnlyIntents            []string `json:"left_only_intents,omitempty"`
	RightOnlyIntents           []string `json:"right_only_intents,omitempty"`
	LeftOnlyResults            []string `json:"left_only_results,omitempty"`
	RightOnlyResults           []string `json:"right_only_results,omitempty"`
	ContextDriftClassification string   `json:"context_drift_classification,omitempty"`
	ContextRuntimeOnlyChanges  bool     `json:"context_runtime_only_changes,omitempty"`
}

type DiffResult struct {
	Privacy DiffPrivacy `json:"privacy"`
	Summary DiffSummary `json:"summary"`
}

func DiffRunpacks(leftPath, rightPath string, privacy DiffPrivacy) (DiffResult, error) {
	if privacy == "" {
		privacy = DiffPrivacyFull
	}
	if privacy != DiffPrivacyFull && privacy != DiffPrivacyMetadata {
		return DiffResult{}, fmt.Errorf("unsupported privacy mode: %s", privacy)
	}
	left, err := ReadRunpack(leftPath)
	if err != nil {
		return DiffResult{}, err
	}
	right, err := ReadRunpack(rightPath)
	if err != nil {
		return DiffResult{}, err
	}

	leftManifest := normalizeManifest(left.Manifest)
	rightManifest := normalizeManifest(right.Manifest)

	leftManifestJSON, err := json.Marshal(leftManifest)
	if err != nil {
		return DiffResult{}, err
	}
	rightManifestJSON, err := json.Marshal(rightManifest)
	if err != nil {
		return DiffResult{}, err
	}

	leftManifestDigest, err := digestNormalized(leftManifestJSON)
	if err != nil {
		return DiffResult{}, err
	}
	rightManifestDigest, err := digestNormalized(rightManifestJSON)
	if err != nil {
		return DiffResult{}, err
	}

	filesChanged := diffManifestFiles(left.Manifest.Files, right.Manifest.Files)

	intentsChanged, leftOnlyIntents, rightOnlyIntents, err := diffRecords(left.Intents, right.Intents, privacy)
	if err != nil {
		return DiffResult{}, err
	}
	resultsChanged, leftOnlyResults, rightOnlyResults, err := diffRecords(left.Results, right.Results, privacy)
	if err != nil {
		return DiffResult{}, err
	}
	refsChanged := false
	contextDriftClassification := ""
	contextRuntimeOnly := false
	leftRefs := normalizeRefs(left.Refs)
	rightRefs := normalizeRefs(right.Refs)
	if privacy == DiffPrivacyFull {
		refsChanged, err = jsonDiff(leftRefs, rightRefs)
		if err != nil {
			return DiffResult{}, err
		}
		classification, changed, runtimeOnly, classifyErr := contextproof.ClassifyRefsDrift(left.Refs, right.Refs)
		if classifyErr != nil {
			return DiffResult{}, classifyErr
		}
		contextDriftClassification = classification
		if changed {
			refsChanged = true
		}
		contextRuntimeOnly = runtimeOnly
	} else {
		refsChanged = len(leftRefs.Receipts) != len(rightRefs.Receipts)
		contextDriftClassification = "none"
	}

	summary := DiffSummary{
		RunIDLeft:                  left.Run.RunID,
		RunIDRight:                 right.Run.RunID,
		ManifestChanged:            leftManifestDigest != rightManifestDigest,
		FilesChanged:               filesChanged,
		IntentsChanged:             intentsChanged,
		ResultsChanged:             resultsChanged,
		RefsChanged:                refsChanged,
		LeftOnlyIntents:            leftOnlyIntents,
		RightOnlyIntents:           rightOnlyIntents,
		LeftOnlyResults:            leftOnlyResults,
		RightOnlyResults:           rightOnlyResults,
		ContextDriftClassification: contextDriftClassification,
		ContextRuntimeOnlyChanges:  contextRuntimeOnly,
	}

	return DiffResult{
		Privacy: privacy,
		Summary: summary,
	}, nil
}

func diffManifestFiles(left, right []schemarunpack.ManifestFile) []string {
	leftSet := make(map[string]struct{}, len(left))
	for _, file := range left {
		leftSet[file.Path] = struct{}{}
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, file := range right {
		rightSet[file.Path] = struct{}{}
	}
	changed := make([]string, 0)
	for path := range leftSet {
		if _, ok := rightSet[path]; !ok {
			changed = append(changed, path)
		}
	}
	for path := range rightSet {
		if _, ok := leftSet[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func diffRecords[T any](left, right []T, privacy DiffPrivacy) (bool, []string, []string, error) {
	leftDigest, err := digestRecords(left, privacy)
	if err != nil {
		return false, nil, nil, err
	}
	rightDigest, err := digestRecords(right, privacy)
	if err != nil {
		return false, nil, nil, err
	}
	leftKeys := make(map[string]struct{}, len(leftDigest.keys))
	for _, key := range leftDigest.keys {
		leftKeys[key] = struct{}{}
	}
	rightKeys := make(map[string]struct{}, len(rightDigest.keys))
	for _, key := range rightDigest.keys {
		rightKeys[key] = struct{}{}
	}
	leftOnly := diffKeys(leftKeys, rightKeys)
	rightOnly := diffKeys(rightKeys, leftKeys)

	changed := len(leftOnly) > 0 || len(rightOnly) > 0
	if !changed {
		for key, leftValue := range leftDigest.byKey {
			if rightValue, ok := rightDigest.byKey[key]; !ok || leftValue != rightValue {
				changed = true
				break
			}
		}
	}
	return changed, leftOnly, rightOnly, nil
}

type digestIndex struct {
	keys  []string
	byKey map[string]string
}

func digestRecords[T any](records []T, privacy DiffPrivacy) (digestIndex, error) {
	index := digestIndex{
		keys:  make([]string, 0, len(records)),
		byKey: make(map[string]string, len(records)),
	}
	for _, record := range records {
		key, err := recordKey(record)
		if err != nil {
			return digestIndex{}, err
		}
		digest, err := digestRecord(record, privacy)
		if err != nil {
			return digestIndex{}, err
		}
		index.keys = append(index.keys, key)
		index.byKey[key] = digest
	}
	sort.Strings(index.keys)
	return index, nil
}

func diffKeys(left, right map[string]struct{}) []string {
	out := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func digestNormalized(raw []byte) (string, error) {
	return jcs.DigestJCS(raw)
}

func jsonDiff(left, right any) (bool, error) {
	leftRaw, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	rightRaw, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	leftDigest, err := digestNormalized(leftRaw)
	if err != nil {
		return false, err
	}
	rightDigest, err := digestNormalized(rightRaw)
	if err != nil {
		return false, err
	}
	return leftDigest != rightDigest, nil
}

func recordKey(record any) (string, error) {
	switch value := record.(type) {
	case schemarunpack.IntentRecord:
		return value.IntentID, nil
	case schemarunpack.ResultRecord:
		return value.IntentID, nil
	default:
		return "", fmt.Errorf("unsupported record type")
	}
}

func digestRecord(record any, privacy DiffPrivacy) (string, error) {
	switch value := record.(type) {
	case schemarunpack.IntentRecord:
		value.RunID = ""
		value.ProducerVersion = ""
		value.CreatedAt = time.Time{}
		if privacy == DiffPrivacyMetadata {
			value.Args = nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return digestNormalized(raw)
	case schemarunpack.ResultRecord:
		value.RunID = ""
		value.ProducerVersion = ""
		value.CreatedAt = time.Time{}
		if privacy == DiffPrivacyMetadata {
			value.Result = nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return digestNormalized(raw)
	default:
		return "", fmt.Errorf("unsupported record type")
	}
}

func normalizeManifest(manifest schemarunpack.Manifest) schemarunpack.Manifest {
	manifest.RunID = ""
	manifest.ProducerVersion = ""
	manifest.CreatedAt = time.Time{}
	manifest.ManifestDigest = ""
	manifest.Signatures = nil
	for i := range manifest.Files {
		manifest.Files[i].SHA256 = ""
	}
	return manifest
}

func normalizeRefs(refs schemarunpack.Refs) schemarunpack.Refs {
	refs.RunID = ""
	refs.ProducerVersion = ""
	refs.CreatedAt = time.Time{}
	return refs
}
