package guard

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

type auditSummaryPDFOptions struct {
	RunID         string
	CaseID        string
	TemplateID    string
	GeneratedAt   time.Time
	EvidenceFiles map[string][]byte
}

func renderAuditSummaryPDF(options auditSummaryPDFOptions) ([]byte, error) {
	generatedAt := options.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	evidencePaths := make([]string, 0, len(options.EvidenceFiles))
	for path := range options.EvidenceFiles {
		evidencePaths = append(evidencePaths, path)
	}
	sort.Strings(evidencePaths)
	if len(evidencePaths) > 8 {
		evidencePaths = evidencePaths[:8]
	}

	lines := []string{
		"Gait Evidence Pack Summary",
		"Run ID: " + strings.TrimSpace(options.RunID),
		"Case ID: " + strings.TrimSpace(options.CaseID),
		"Template: " + strings.TrimSpace(options.TemplateID),
		"Generated: " + generatedAt.Format(time.RFC3339),
		fmt.Sprintf("Evidence Files: %d", len(options.EvidenceFiles)),
	}
	for _, path := range evidencePaths {
		lines = append(lines, "- "+path)
	}
	contentStream := buildPDFTextStream(lines)

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStream), contentStream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	return buildPDF(objects), nil
}

func buildPDFTextStream(lines []string) string {
	var buffer bytes.Buffer
	buffer.WriteString("BT\n/F1 10 Tf\n50 760 Td\n")
	for index, line := range lines {
		if index > 0 {
			buffer.WriteString("0 -14 Td\n")
		}
		buffer.WriteString("(")
		buffer.WriteString(escapePDFString(line))
		buffer.WriteString(") Tj\n")
	}
	buffer.WriteString("ET")
	return buffer.String()
}

func escapePDFString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	value = strings.ReplaceAll(value, ")", `\)`)
	return value
}

func buildPDF(objects []string) []byte {
	var body bytes.Buffer
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	body.WriteString("%PDF-1.4\n")
	for index, object := range objects {
		offsets = append(offsets, body.Len())
		body.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", index+1, object))
	}
	xrefStart := body.Len()
	body.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objects)+1))
	body.WriteString("0000000000 65535 f \n")
	for index := 1; index < len(offsets); index++ {
		body.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[index]))
	}
	body.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart))
	return body.Bytes()
}
