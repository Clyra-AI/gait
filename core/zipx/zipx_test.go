package zipx

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
	"time"
)

func TestWriteDeterministicZipStable(t *testing.T) {
	files := []File{
		{Path: "b.txt", Data: []byte("two")},
		{Path: "a.txt", Data: []byte("one")},
	}

	sum1 := zipSum(t, files)
	sum2 := zipSum(t, files)
	if sum1 != sum2 {
		t.Fatalf("expected deterministic zip, got %s vs %s", sum1, sum2)
	}
}

func TestWriteDeterministicZipOrderIndependent(t *testing.T) {
	filesA := []File{
		{Path: "b.txt", Data: []byte("two")},
		{Path: "a.txt", Data: []byte("one")},
	}
	filesB := []File{
		{Path: "a.txt", Data: []byte("one")},
		{Path: "b.txt", Data: []byte("two")},
	}

	sumA := zipSum(t, filesA)
	sumB := zipSum(t, filesB)
	if sumA != sumB {
		t.Fatalf("expected same zip for different input order")
	}
}

func TestWriteDeterministicZipEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDeterministicZip(&buf, nil); err != nil {
		t.Fatalf("expected no error for empty input: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output for empty input")
	}
}

func TestWriteDeterministicZipHeaderFields(t *testing.T) {
	files := []File{
		{Path: "dir/b.txt", Data: []byte("two"), Mode: 0o755},
		{Path: "a.txt", Data: []byte("one"), Mode: 0},
		{Path: "c.txt", Data: []byte("three"), Mode: 0o640},
	}
	var buf bytes.Buffer
	if err := WriteDeterministicZip(&buf, files); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	r := bytes.NewReader(buf.Bytes())
	zr, err := zip.NewReader(r, int64(r.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 3 {
		t.Fatalf("expected 3 files, got %d", len(zr.File))
	}
	// Verify deterministic ordering and normalized paths
	if zr.File[0].Name != "a.txt" || zr.File[1].Name != "c.txt" || zr.File[2].Name != "dir/b.txt" {
		t.Fatalf("unexpected file order or names: %v, %v, %v", zr.File[0].Name, zr.File[1].Name, zr.File[2].Name)
	}
	// Verify mode normalization
	if zr.File[0].Mode().Perm() != 0o644 {
		t.Fatalf("expected 0644 for a.txt, got %v", zr.File[0].Mode().Perm())
	}
	if zr.File[1].Mode().Perm() != 0o644 {
		t.Fatalf("expected 0644 for c.txt, got %v", zr.File[1].Mode().Perm())
	}
	if zr.File[2].Mode().Perm() != 0o755 {
		t.Fatalf("expected 0755 for dir/b.txt, got %v", zr.File[2].Mode().Perm())
	}
	// Verify fixed timestamp
	wantTime, _ := time.Parse(time.RFC3339, fixedTime)
	if !zr.File[0].Modified.Equal(wantTime) || !zr.File[1].Modified.Equal(wantTime) || !zr.File[2].Modified.Equal(wantTime) {
		t.Fatalf("unexpected modified time")
	}
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWriteDeterministicZipWriteError(t *testing.T) {
	files := []File{{Path: "a.txt", Data: []byte("one")}}
	err := WriteDeterministicZip(errWriter{}, files)
	if err == nil {
		t.Fatalf("expected write error")
	}
}

func zipSum(t *testing.T, files []File) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteDeterministicZip(&buf, files); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}
