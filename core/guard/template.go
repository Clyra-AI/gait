package guard

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	schemaguard "github.com/davidahmann/gait/core/schema/v1/guard"
)

const (
	defaultTemplateID = "soc2"
	templateSOC2      = "soc2"
	templatePCI       = "pci"
	templateIncident  = "incident_response"
	entryTypeRunpack  = "runpack"
	entryTypeTrace    = "trace"
	entryTypeReport   = "report"
	entryTypeEvidence = "evidence"
)

type controlTemplate struct {
	ID          string
	Title       string
	EntryTypes  []string
	PathPrefix  []string
	PathMatches []string
}

var controlTemplates = map[string][]controlTemplate{
	templateSOC2: {
		{ID: "CC6.6", Title: "Change Management Evidence", EntryTypes: []string{entryTypeTrace, entryTypeEvidence}, PathMatches: []string{"approval_audit_", "credential_evidence_"}},
		{ID: "CC7.2", Title: "Operational Monitoring", EntryTypes: []string{entryTypeReport}, PathMatches: []string{"regress_summary.json", "trace_summary.json"}},
		{ID: "CC8.1", Title: "Incident Evidence Integrity", EntryTypes: []string{entryTypeRunpack, entryTypeEvidence}, PathMatches: []string{"runpack_summary.json", "referenced_runpacks.json"}},
	},
	templatePCI: {
		{ID: "PCI-10", Title: "Audit Trail and Monitoring", EntryTypes: []string{entryTypeTrace, entryTypeReport}, PathMatches: []string{"trace_summary.json"}},
		{ID: "PCI-7", Title: "Access and Approval Controls", EntryTypes: []string{entryTypeEvidence}, PathMatches: []string{"approval_audit_", "credential_evidence_"}},
		{ID: "PCI-12", Title: "Incident Handling Evidence", EntryTypes: []string{entryTypeRunpack, entryTypeReport}, PathMatches: []string{"runpack_summary.json", "regress_summary.json"}},
	},
	templateIncident: {
		{ID: "IR-CHAIN", Title: "Reconstruction Chain", EntryTypes: []string{entryTypeRunpack, entryTypeTrace, entryTypeEvidence}, PathMatches: []string{"runpack_summary.json", "trace_summary.json", "approval_audit_", "credential_evidence_"}},
		{ID: "IR-ROOTCAUSE", Title: "Root Cause and Regression", EntryTypes: []string{entryTypeReport}, PathMatches: []string{"regress_summary.json", "policy_digests.json"}},
	},
}

func normalizeTemplateID(templateID string) string {
	trimmed := strings.ToLower(strings.TrimSpace(templateID))
	if trimmed == "" {
		return defaultTemplateID
	}
	if _, ok := controlTemplates[trimmed]; ok {
		return trimmed
	}
	return templateIncident
}

func buildControlIndex(templateID string, contents []schemaguard.PackEntry) []schemaguard.Control {
	templates, ok := controlTemplates[templateID]
	if !ok {
		templates = controlTemplates[defaultTemplateID]
	}
	controls := make([]schemaguard.Control, 0, len(templates))
	for _, template := range templates {
		matchedPaths := make([]string, 0, len(contents))
		for _, entry := range contents {
			if !matchesControlTemplate(entry, template) {
				continue
			}
			matchedPaths = append(matchedPaths, entry.Path)
		}
		matchedPaths = uniqueSortedStrings(matchedPaths)
		controls = append(controls, schemaguard.Control{
			ControlID:     template.ID,
			Title:         template.Title,
			EvidencePaths: matchedPaths,
		})
	}
	return controls
}

func matchesControlTemplate(entry schemaguard.PackEntry, template controlTemplate) bool {
	path := strings.TrimSpace(entry.Path)
	if path == "" {
		return false
	}
	for _, kind := range template.EntryTypes {
		if entry.Type == kind {
			return true
		}
	}
	for _, prefix := range template.PathPrefix {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, candidate := range template.PathMatches {
		if strings.Contains(path, candidate) {
			return true
		}
	}
	return false
}

func buildEvidencePointers(contents []schemaguard.PackEntry) []schemaguard.Evidence {
	sorted := make([]schemaguard.PackEntry, len(contents))
	copy(sorted, contents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	pointers := make([]schemaguard.Evidence, 0, len(sorted))
	for index, entry := range sorted {
		pointers = append(pointers, schemaguard.Evidence{
			PointerID: fmt.Sprintf("ev_%03d", index+1),
			Path:      entry.Path,
			Type:      entry.Type,
			SHA256:    entry.SHA256,
		})
	}
	return pointers
}

func normalizePackPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be relative and stay within pack root")
	}
	return filepath.ToSlash(cleaned), nil
}

func normalizeIncidentWindow(window *schemaguard.Window) *schemaguard.Window {
	if window == nil {
		return nil
	}
	from := window.From.UTC()
	to := window.To.UTC()
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil
	}
	windowSeconds := window.WindowSeconds
	if windowSeconds < 0 {
		windowSeconds = 0
	}
	return &schemaguard.Window{
		From:            from,
		To:              to,
		WindowSeconds:   windowSeconds,
		SelectionAnchor: strings.TrimSpace(window.SelectionAnchor),
	}
}
