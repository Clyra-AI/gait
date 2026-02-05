package jcs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalizeJSON(t *testing.T) {
	in := []byte(`{ "b":2, "a":1 }`)
	want := `{"a":1,"b":2}`
	out, err := CanonicalizeJSON(in)
	if err != nil {
		t.Fatalf("canonicalize error: %v", err)
	}
	if string(out) != want {
		t.Fatalf("unexpected canonical form: %s", string(out))
	}
}

func TestDigestJCSStable(t *testing.T) {
	a := []byte(`{"a":1,"b":2}`)
	b := []byte(`{ "b":2, "a":1 }`)

	da, err := DigestJCS(a)
	if err != nil {
		t.Fatalf("digest error: %v", err)
	}
	db, err := DigestJCS(b)
	if err != nil {
		t.Fatalf("digest error: %v", err)
	}
	if da != db {
		t.Fatalf("expected same digest for equivalent JSON")
	}
}

func TestCanonicalizeJSONInvalid(t *testing.T) {
	_, err := CanonicalizeJSON([]byte(`{`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestCanonicalizeJSONFixtures(t *testing.T) {
	cases := []struct {
		input     string
		canonical string
	}{
		{"rfc_sample1.json", "rfc_sample1_canonical.json"},
		{"rfc_sample2.json", "rfc_sample2_canonical.json"},
	}
	root := repoRoot(t)
	for _, c := range cases {
		inPath := filepath.Join(root, "core", "jcs", "testdata", c.input)
		outPath := filepath.Join(root, "core", "jcs", "testdata", c.canonical)
		input, err := os.ReadFile(inPath)
		if err != nil {
			t.Fatalf("read input fixture: %v", err)
		}
		want, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read canonical fixture: %v", err)
		}
		got, err := CanonicalizeJSON(input)
		if err != nil {
			t.Fatalf("canonicalize fixture: %v", err)
		}
		if string(bytesTrimSpace(got)) != string(bytesTrimSpace(want)) {
			t.Fatalf("unexpected canonical form for %s: %s", c.input, string(got))
		}
	}
}

func TestDigestJCSInvalid(t *testing.T) {
	_, err := DigestJCS([]byte(`{`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON digest")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	dir := filepath.Dir(filename)
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}

func bytesTrimSpace(b []byte) []byte {
	start := 0
	end := len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}
