package actioncontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exportTestRecords(t *testing.T) []LifecycleRecord {
	_, _, _, _, r, _, _ := loadConformanceFixture(t, "successful-execution-effect-containment")
	for i := range r {
		r[i].Correlation.TraceID = "0123456789abcdef0123456789abcdef"
		r[i].Correlation.SpanID = "0123456789abcdef"
		if r[i].OccurredAt == "" {
			r[i].OccurredAt = "2026-07-20T00:00:00Z"
		}
	}
	return r
}
func TestLifecycleExportSafeDeterministicOutput(t *testing.T) {
	r := exportTestRecords(t)
	d := t.TempDir()
	a := filepath.Join(d, "a.jsonl")
	b := filepath.Join(d, "b.jsonl")
	if e := ExportLifecycleOTel(a, r, "v1"); e != nil {
		t.Fatal(e)
	}
	rev := append([]LifecycleRecord{}, r...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if e := ExportLifecycleOTel(b, rev, "v1"); e != nil {
		t.Fatal(e)
	}
	x, _ := os.ReadFile(a)
	y, _ := os.ReadFile(b)
	if string(x) != string(y) {
		t.Fatal("export not deterministic")
	}
	if strings.Contains(string(x), "execution") && strings.Contains(string(x), "payload") {
		t.Fatal("nested payload exported")
	}
	if e := ExportLifecycleOTelWithOptions(filepath.Join(d, "small"), r, LifecycleOTelExportOptions{SourceVersion: "v1", MaxBytes: 1}); e == nil {
		t.Fatal("size limit ignored")
	}
}
func TestLifecycleExportRejectsUnsafeInputs(t *testing.T) {
	r := exportTestRecords(t)
	d := t.TempDir()
	if e := ExportLifecycleOTel(filepath.Join(d, "missing", "x"), r, "v1"); e == nil {
		t.Fatal("missing parent accepted")
	}
	if e := ExportLifecycleOTel(filepath.Join(d, "x"), r, ""); e == nil {
		t.Fatal("missing source version accepted")
	}
	r[0].Correlation.TraceID = "BAD"
	if e := ExportLifecycleOTel(filepath.Join(d, "bad"), r, "v1"); e == nil {
		t.Fatal("bad trace accepted")
	}
}
