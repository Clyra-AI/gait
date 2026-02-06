package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RetentionOptions struct {
	RootPath        string
	TraceTTL        time.Duration
	PackTTL         time.Duration
	DryRun          bool
	ReportOutput    string
	Now             time.Time
	ProducerVersion string
}

type RetentionResult struct {
	SchemaID        string               `json:"schema_id"`
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	ProducerVersion string               `json:"producer_version"`
	RootPath        string               `json:"root_path"`
	DryRun          bool                 `json:"dry_run"`
	TraceTTLSeconds int64                `json:"trace_ttl_seconds"`
	PackTTLSeconds  int64                `json:"pack_ttl_seconds"`
	ScannedFiles    int                  `json:"scanned_files"`
	DeletedFiles    []RetentionFileEvent `json:"deleted_files"`
	KeptFiles       []RetentionFileEvent `json:"kept_files"`
}

type RetentionFileEvent struct {
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	ModifiedAt time.Time `json:"modified_at"`
	AgeSeconds int64     `json:"age_seconds"`
	Action     string    `json:"action"`
}

func ApplyRetention(options RetentionOptions) (RetentionResult, error) {
	rootPath := strings.TrimSpace(options.RootPath)
	if rootPath == "" {
		rootPath = "."
	}
	rootPath = filepath.Clean(rootPath)

	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(options.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	candidates, err := collectRetentionCandidates(rootPath)
	if err != nil {
		return RetentionResult{}, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].path < candidates[j].path
	})

	result := RetentionResult{
		SchemaID:        "gait.guard.retention_report",
		SchemaVersion:   "1.0.0",
		CreatedAt:       now,
		ProducerVersion: producerVersion,
		RootPath:        rootPath,
		DryRun:          options.DryRun,
		TraceTTLSeconds: int64(options.TraceTTL.Seconds()),
		PackTTLSeconds:  int64(options.PackTTL.Seconds()),
		ScannedFiles:    len(candidates),
		DeletedFiles:    make([]RetentionFileEvent, 0),
		KeptFiles:       make([]RetentionFileEvent, 0),
	}

	for _, candidate := range candidates {
		ttl := options.PackTTL
		if candidate.kind == "trace" {
			ttl = options.TraceTTL
		}
		age := now.Sub(candidate.modifiedAt)
		if age < 0 {
			age = 0
		}

		event := RetentionFileEvent{
			Path:       candidate.path,
			Kind:       candidate.kind,
			ModifiedAt: candidate.modifiedAt.UTC(),
			AgeSeconds: int64(age.Seconds()),
			Action:     "kept",
		}
		if ttl > 0 && age > ttl {
			event.Action = "deleted"
			if !options.DryRun {
				if err := os.Remove(candidate.path); err != nil {
					return RetentionResult{}, fmt.Errorf("delete retained file %s: %w", candidate.path, err)
				}
			}
			result.DeletedFiles = append(result.DeletedFiles, event)
			continue
		}
		result.KeptFiles = append(result.KeptFiles, event)
	}

	if strings.TrimSpace(options.ReportOutput) != "" {
		reportPayload, err := marshalCanonicalJSON(result)
		if err != nil {
			return RetentionResult{}, fmt.Errorf("encode retention report: %w", err)
		}
		reportPath := strings.TrimSpace(options.ReportOutput)
		if err := os.MkdirAll(filepath.Dir(reportPath), 0o750); err != nil {
			return RetentionResult{}, fmt.Errorf("create retention report directory: %w", err)
		}
		if err := os.WriteFile(reportPath, reportPayload, 0o600); err != nil {
			return RetentionResult{}, fmt.Errorf("write retention report: %w", err)
		}
	}

	return result, nil
}

type retentionCandidate struct {
	path       string
	kind       string
	modifiedAt time.Time
}

func collectRetentionCandidates(rootPath string) ([]retentionCandidate, error) {
	candidates := make([]retentionCandidate, 0)
	err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		kind := classifyRetentionFile(path)
		if kind == "" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		candidates = append(candidates, retentionCandidate{
			path:       path,
			kind:       kind,
			modifiedAt: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan retention root: %w", err)
	}
	return candidates, nil
}

func classifyRetentionFile(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasPrefix(base, "trace_") && strings.HasSuffix(base, ".json"):
		return "trace"
	case strings.HasPrefix(base, "evidence_pack_") && strings.HasSuffix(base, ".zip"):
		return "pack"
	case strings.HasPrefix(base, "incident_pack_") && strings.HasSuffix(base, ".zip"):
		return "pack"
	case strings.HasPrefix(base, "evidence_pack_") && strings.HasSuffix(base, ".gaitenc"):
		return "pack"
	case strings.HasPrefix(base, "incident_pack_") && strings.HasSuffix(base, ".gaitenc"):
		return "pack"
	default:
		return ""
	}
}
