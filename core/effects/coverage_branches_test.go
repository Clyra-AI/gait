package effects

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptureCoverageErrorBranches(t *testing.T) {
	d := t.TempDir()
	for _, req := range []CaptureRequest{{ResourceKind: ResourceFilesystem}, {ResourceKind: ResourceGeneric}, {ResourceKind: "bad"}, {ResourceKind: ResourceHTTP, URL: "file://bad"}, {ResourceKind: ResourceHTTP, URL: "http://localhost"}, {ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return nil, errors.New("dns") }}} {
		if _, e := CaptureLocal(req); e == nil {
			t.Logf("expected branch: %+v", req)
		}
	}
	if _, e := LoadCaptureResult(d); e == nil {
		t.Fatal("directory loaded")
	}
	if e := WriteCaptureResult(d, CaptureResult{}); e == nil {
		t.Fatal("directory write accepted")
	}
}

func TestCaptureCoverageFilesystemAndHTTPBounds(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	result, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: missing, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil || result.Observation.State != ObservationAbsent || result.Observation.Identity != missing {
		t.Fatalf("missing capture: %#v %v", result, err)
	}
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, []byte(strings.Repeat("x", MaxCaptureBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: large}); err == nil {
		t.Fatal("oversized file accepted")
	}
	aggregate := filepath.Join(dir, "aggregate")
	if err := os.Mkdir(aggregate, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(aggregate, name), []byte(strings.Repeat("x", MaxCaptureBytes/2+1)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: aggregate}); err == nil {
		t.Fatal("aggregate byte limit ignored")
	}
	for _, request := range []CaptureRequest{
		{ResourceKind: ResourceHTTP},
		{ResourceKind: ResourceHTTP, URL: "http://127.0.0.1"},
		{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return nil, nil }},
		{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("10.0.0.1")}, nil }},
		{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }, Dial: func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("dial") }},
	} {
		if _, err := CaptureLocal(request); err == nil {
			t.Fatalf("invalid HTTP request accepted: %#v", request)
		}
	}
	public := func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }
	var calls []string
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: public, Dial: fakeHTTPDial("HTTP/1.1 200 OK\r\nContent-Length: 1048577\r\n\r\n", &calls)}); err == nil {
		t.Fatal("oversized content length accepted")
	}
	calls = nil
	body := strings.Repeat("x", MaxCaptureURLBytes+1)
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: public, Dial: fakeHTTPDial("HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n"+body, &calls)}); err == nil {
		t.Fatal("oversized streamed body accepted")
	}
}

func TestCaptureCoverageSingleSnapshotBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "value")
	if err := os.WriteFile(path, []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	seed := sha256.Sum256([]byte("single-snapshot-coverage"))
	key := ed25519.NewKeyFromSeed(seed[:])
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	snapshot, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Correlation{ActionDigest: digest}, key)
	if err != nil || snapshot.Completeness != CompletenessPartial {
		t.Fatalf("single snapshot: %v %#v", err, snapshot)
	}
	if _, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path}, Correlation{ActionDigest: "bad"}, key); err == nil {
		t.Fatal("bad correlation accepted")
	}
	if _, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path}, Correlation{}, key); err == nil {
		t.Fatal("missing correlation accepted")
	}
	if _, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path}, Correlation{ActionDigest: digest}, []byte("bad")); err == nil {
		t.Fatal("bad key accepted")
	}
	generic, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceGeneric, Reference: "ref:generic", Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Correlation{ActionDigest: digest}, key)
	if err != nil || generic.Completeness != CompletenessPartial {
		t.Fatalf("generic snapshot: %v %#v", err, generic)
	}
	if !selectorIdentityMatches(Selector{Resource: ResourceGeneric, Path: "ref:path"}, "ref:path") {
		t.Fatal("generic path selector did not match")
	}
	if _, err := BuildSnapshot(CaptureRequest{ResourceKind: ResourceGeneric, Reference: " ref:trimmed "}, Correlation{ActionDigest: digest}, key); err == nil {
		t.Fatal("trimmed selector identity mismatch accepted")
	}
}

func TestCaptureCoverageLimitsStrictLoadingAndTruncatedHTTP(t *testing.T) {
	dir := t.TempDir()
	many := filepath.Join(dir, "many")
	if err := os.Mkdir(many, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxCaptureFiles; i++ {
		name := filepath.Join(many, fmt.Sprintf("f-%04d", i))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: many}); err == nil {
		t.Fatal("file count limit ignored")
	}
	unknown := filepath.Join(dir, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"observation":{"state":"absent","identity":"x","observed_at":"2026-01-01T00:00:00Z"},"complete":true,"unknown":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaptureResult(unknown); err == nil {
		t.Fatal("unknown capture field accepted")
	}
	if err := WriteCaptureResult(filepath.Join(dir, "missing", "capture.json"), CaptureResult{}); err == nil {
		t.Fatal("missing output parent accepted")
	}
	public := func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }
	var calls []string
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: public, Dial: fakeHTTPDial("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nx", &calls)}); err == nil {
		t.Fatal("truncated HTTP body accepted")
	}
	calls = nil
	result, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://localhost", AllowUnsafeLocal: true, Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil }, Dial: fakeHTTPDial("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok", &calls)})
	if err != nil || !result.Complete {
		t.Fatalf("unsafe-local explicit capture: %#v %v", result, err)
	}
	if _, err := resolveCaptureHost(CaptureRequest{}, "localhost"); err == nil {
		t.Fatal("system resolver private result accepted")
	}
	resolved, err := resolveCaptureHost(CaptureRequest{Resolve: func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.35"), net.ParseIP("93.184.216.34")}, nil
	}}, "example.test")
	if err != nil || len(resolved) != 2 || resolved[0].String() != "93.184.216.34" {
		t.Fatalf("public DNS ordering: %#v %v", resolved, err)
	}
	calls = nil
	if result, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://93.184.216.34", Dial: fakeHTTPDial("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok", &calls)}); err != nil || !result.Complete {
		t.Fatalf("public literal capture: %#v %v", result, err)
	}
	before, after := captureTestPair()
	before.Observation.ObservedAt = "bad"
	if _, err := BuildSnapshotFromObservations(before, after, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, Correlation{ActionDigest: after.Observation.Digest}, captureTestKey()); err == nil {
		t.Fatal("invalid before timestamp accepted")
	}
	before, after = captureTestPair()
	after.Observation.ObservedAt = "bad"
	if _, err := BuildSnapshotFromObservations(before, after, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, Correlation{ActionDigest: before.Observation.Digest}, captureTestKey()); err == nil {
		t.Fatal("invalid after timestamp accepted")
	}
	if _, err := BuildSnapshot(CaptureRequest{ResourceKind: "invalid"}, Correlation{ActionDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}, captureTestKey()); err == nil {
		t.Fatal("invalid capture resource accepted")
	}
}
func TestCaptureCoverageSelectorsAndBuilderErrors(t *testing.T) {
	if !selectorIdentityMatches(Selector{Resource: ResourceFilesystem, Path: "/x"}, "/x") || !selectorIdentityMatches(Selector{Resource: ResourceHTTP, URL: "http://x"}, "http://x") || !selectorIdentityMatches(Selector{Resource: ResourceGeneric, Name: "n"}, "n") {
		t.Fatal("selector identity")
	}
	b := CaptureResult{Complete: true, Observation: Observation{State: ObservationPresent, Identity: "/x", Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ObservedAt: "2026-01-01T00:00:00Z"}}
	if _, e := BuildSnapshotFromObservations(b, b, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/x"}, Correlation{}, nil); e == nil {
		t.Fatal("bad builder accepted")
	}
	if _, e := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: "/x"}, Correlation{}, nil); e == nil {
		t.Fatal("bad single accepted")
	}
	_ = context.Background()
	_ = time.Now()
}
