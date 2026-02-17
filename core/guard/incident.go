package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemaguard "github.com/Clyra-AI/gait/core/schema/v1/guard"
	schemaregress "github.com/Clyra-AI/gait/core/schema/v1/regress"
)

type IncidentPackOptions struct {
	RunpackPath     string
	OutputPath      string
	CaseID          string
	Window          time.Duration
	TemplateID      string
	RenderPDF       bool
	ProducerVersion string
}

type IncidentPackResult struct {
	BuildResult             BuildResult `json:"build_result"`
	WindowFrom              time.Time   `json:"window_from"`
	WindowTo                time.Time   `json:"window_to"`
	TraceCount              int         `json:"trace_count"`
	RegressCount            int         `json:"regress_count"`
	ApprovalAuditCount      int         `json:"approval_audit_count"`
	CredentialEvidenceCount int         `json:"credential_evidence_count"`
	PolicyDigests           []string    `json:"policy_digests"`
}

func BuildIncidentPack(options IncidentPackOptions) (IncidentPackResult, error) {
	runpackPath := strings.TrimSpace(options.RunpackPath)
	if runpackPath == "" {
		return IncidentPackResult{}, fmt.Errorf("runpack path is required")
	}
	data, err := runpack.ReadRunpack(runpackPath)
	if err != nil {
		return IncidentPackResult{}, fmt.Errorf("read runpack for incident pack: %w", err)
	}
	window := options.Window
	if window <= 0 {
		window = 24 * time.Hour
	}
	anchor := data.Run.CreatedAt.UTC()
	if anchor.IsZero() {
		anchor = time.Now().UTC()
	}
	windowFrom := anchor.Add(-window)
	windowTo := anchor.Add(window)
	rootDir := filepath.Dir(runpackPath)

	tracePaths, traceIDs, policyDigests, err := collectIncidentTraces(rootDir, windowFrom, windowTo)
	if err != nil {
		return IncidentPackResult{}, err
	}
	regressPaths, err := collectIncidentRegress(rootDir, windowFrom, windowTo)
	if err != nil {
		return IncidentPackResult{}, err
	}
	approvalPaths, err := collectIncidentApprovals(rootDir, traceIDs, windowFrom, windowTo)
	if err != nil {
		return IncidentPackResult{}, err
	}
	credentialPaths, err := collectIncidentCredentials(rootDir, traceIDs, windowFrom, windowTo)
	if err != nil {
		return IncidentPackResult{}, err
	}

	policyDigestPayload, err := marshalCanonicalJSON(map[string]any{
		"run_id":         data.Run.RunID,
		"window_from":    windowFrom,
		"window_to":      windowTo,
		"policy_digests": policyDigests,
	})
	if err != nil {
		return IncidentPackResult{}, fmt.Errorf("encode policy digests: %w", err)
	}

	buildResult, err := BuildPack(BuildOptions{
		RunpackPath:             runpackPath,
		OutputPath:              options.OutputPath,
		CaseID:                  options.CaseID,
		TemplateID:              options.TemplateID,
		TracePaths:              tracePaths,
		RegressPaths:            regressPaths,
		ApprovalAuditPaths:      approvalPaths,
		CredentialEvidencePaths: credentialPaths,
		ExtraEvidenceFiles: map[string][]byte{
			"policy_digests.json": policyDigestPayload,
		},
		RenderPDF: options.RenderPDF,
		IncidentWindow: &schemaguard.Window{
			From:            windowFrom,
			To:              windowTo,
			WindowSeconds:   int64(window.Seconds()),
			SelectionAnchor: data.Run.RunID,
		},
		AutoDiscoverV12: false,
		ProducerVersion: options.ProducerVersion,
	})
	if err != nil {
		return IncidentPackResult{}, err
	}

	return IncidentPackResult{
		BuildResult:             buildResult,
		WindowFrom:              windowFrom,
		WindowTo:                windowTo,
		TraceCount:              len(tracePaths),
		RegressCount:            len(regressPaths),
		ApprovalAuditCount:      len(approvalPaths),
		CredentialEvidenceCount: len(credentialPaths),
		PolicyDigests:           policyDigests,
	}, nil
}

func collectIncidentTraces(rootDir string, from time.Time, to time.Time) ([]string, map[string]struct{}, []string, error) {
	paths, err := filepath.Glob(filepath.Join(rootDir, "trace_*.json"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("discover traces: %w", err)
	}
	sort.Strings(paths)
	selected := make([]string, 0, len(paths))
	traceIDs := make(map[string]struct{})
	policyDigestSet := make(map[string]struct{})
	for _, path := range paths {
		record, err := readTraceRecord(path)
		if err != nil {
			continue
		}
		createdAt := record.CreatedAt.UTC()
		if createdAt.Before(from) || createdAt.After(to) {
			continue
		}
		selected = append(selected, path)
		traceIDs[record.TraceID] = struct{}{}
		if strings.TrimSpace(record.PolicyDigest) != "" {
			policyDigestSet[record.PolicyDigest] = struct{}{}
		}
	}
	policyDigests := make([]string, 0, len(policyDigestSet))
	for digest := range policyDigestSet {
		policyDigests = append(policyDigests, digest)
	}
	sort.Strings(policyDigests)
	return selected, traceIDs, policyDigests, nil
}

func collectIncidentRegress(rootDir string, from time.Time, to time.Time) ([]string, error) {
	candidates, err := filepath.Glob(filepath.Join(rootDir, "regress*.json"))
	if err != nil {
		return nil, fmt.Errorf("discover regress results: %w", err)
	}
	sort.Strings(candidates)
	selected := make([]string, 0, len(candidates))
	for _, path := range candidates {
		// #nosec G304 -- discovered local artifact path.
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var result schemaregress.RegressResult
		if err := json.Unmarshal(raw, &result); err != nil {
			continue
		}
		createdAt := result.CreatedAt.UTC()
		if createdAt.Before(from) || createdAt.After(to) {
			continue
		}
		selected = append(selected, path)
	}
	return selected, nil
}

func collectIncidentApprovals(rootDir string, traceIDs map[string]struct{}, from time.Time, to time.Time) ([]string, error) {
	candidates, err := filepath.Glob(filepath.Join(rootDir, "approval_audit_*.json"))
	if err != nil {
		return nil, fmt.Errorf("discover approval audits: %w", err)
	}
	sort.Strings(candidates)
	selected := make([]string, 0, len(candidates))
	for _, path := range candidates {
		// #nosec G304 -- discovered local artifact path.
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record schemagate.ApprovalAuditRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			continue
		}
		createdAt := record.CreatedAt.UTC()
		if createdAt.Before(from) || createdAt.After(to) {
			continue
		}
		if len(traceIDs) > 0 {
			if _, ok := traceIDs[record.TraceID]; !ok {
				continue
			}
		}
		selected = append(selected, path)
	}
	return selected, nil
}

func collectIncidentCredentials(rootDir string, traceIDs map[string]struct{}, from time.Time, to time.Time) ([]string, error) {
	candidates, err := filepath.Glob(filepath.Join(rootDir, "credential_evidence_*.json"))
	if err != nil {
		return nil, fmt.Errorf("discover credential evidence: %w", err)
	}
	sort.Strings(candidates)
	selected := make([]string, 0, len(candidates))
	for _, path := range candidates {
		// #nosec G304 -- discovered local artifact path.
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record schemagate.BrokerCredentialRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			continue
		}
		createdAt := record.CreatedAt.UTC()
		if createdAt.Before(from) || createdAt.After(to) {
			continue
		}
		if len(traceIDs) > 0 {
			if _, ok := traceIDs[record.TraceID]; !ok {
				continue
			}
		}
		selected = append(selected, path)
	}
	return selected, nil
}
