package effects

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildSnapshotPairDeterministicAndPartialSingle(t *testing.T) {
	seed := sha256.Sum256([]byte("capture-test-key"))
	key := ed25519.NewKeyFromSeed(seed[:])
	d1 := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	d2 := "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	corr := Correlation{ActionDigest: d1}
	b := CaptureResult{Complete: true, Observation: Observation{State: ObservationPresent, Identity: "/tmp/x", Digest: d1, ObservedAt: "2026-07-20T00:00:00Z"}}
	a := b
	a.Observation.Digest = d2
	a.Observation.ObservedAt = "2026-07-20T00:00:01Z"
	s, e := BuildSnapshotFromObservations(b, a, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/x"}, corr, key)
	if e != nil || s.Completeness != CompletenessComplete || !ValidateSnapshot(s).Valid {
		t.Fatalf("pair: %v", e)
	}
	s2, e := BuildSnapshotFromObservations(b, a, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/x"}, corr, key)
	if e != nil || s.SnapshotID != s2.SnapshotID {
		t.Fatal("pair nondeterministic")
	}
	single, e := BuildSnapshot(CaptureRequest{ResourceKind: ResourceFilesystem, Path: "/tmp/nonexistent-capture-test"}, corr, key)
	if e == nil && single.Completeness != CompletenessPartial {
		t.Fatalf("single completeness: %+v", single)
	}
}

func TestBuildSnapshotPairRejectsInvalid(t *testing.T) {
	seed := sha256.Sum256([]byte("capture-test-key"))
	key := ed25519.NewKeyFromSeed(seed[:])
	d := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	base := CaptureResult{Complete: true, Observation: Observation{State: ObservationPresent, Identity: "/tmp/x", Digest: d, ObservedAt: "2026-07-20T00:00:01Z"}}
	cases := []CaptureResult{base, {Complete: true, Observation: Observation{State: ObservationUnknown, Identity: "/tmp/x", Digest: d, ObservedAt: "2026-07-20T00:00:02Z"}}, {Complete: true, Observation: Observation{State: ObservationPresent, Identity: "/tmp/y", Digest: d, ObservedAt: "2026-07-20T00:00:02Z"}}}
	for i, c := range cases {
		_, e := BuildSnapshotFromObservations(base, c, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/x"}, Correlation{ActionDigest: d}, key)
		if i == 0 && e != nil {
			t.Fatal(e)
		}
		if i > 0 && e == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func fakeHTTPDial(response string, calls *[]string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		*calls = append(*calls, address)
		a, b := net.Pipe()
		go func() { buf := make([]byte, 4096); _, _ = b.Read(buf); _, _ = b.Write([]byte(response)); _ = b.Close() }()
		return a, nil
	}
}

func TestCaptureHTTPResolverAndDialSafety(t *testing.T) {
	resp := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"
	calls := []string{}
	resolve := func(host string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }
	r, e := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test/x", Resolve: resolve, Dial: fakeHTTPDial(resp, &calls)})
	if e != nil || len(calls) != 1 || !strings.HasPrefix(calls[0], "93.184.216.34:") {
		t.Fatalf("public dial: %+v %v %v", r, e, calls)
	}
	for name, ips := range map[string][]net.IP{"private": {net.ParseIP("10.0.0.1")}, "mixed": {net.ParseIP("93.184.216.34"), net.ParseIP("127.0.0.1")}, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			if _, e := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return ips, nil }, Dial: fakeHTTPDial(resp, &calls)}); e == nil {
				t.Fatal("unsafe answer accepted")
			}
		})
	}
	if _, e := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://localhost", AllowUnsafeLocal: true, Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil }, Dial: fakeHTTPDial(resp, &calls)}); e != nil {
		t.Fatalf("unsafe localhost: %v", e)
	}
}

func TestCaptureFilesystemIsBoundedAndDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	b, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: path, Now: a.ObservationTime()})
	if err != nil {
		t.Fatal(err)
	}
	if a.Observation.Digest != b.Observation.Digest || a.Observation.State != ObservationPresent {
		t.Fatalf("capture mismatch: %+v %+v", a, b)
	}
}

func (o CaptureResult) ObservationTime() (t time.Time) {
	t, _ = time.Parse(time.RFC3339Nano, o.Observation.ObservedAt)
	return
}

func TestCaptureGenericDoesNotExecute(t *testing.T) {
	r, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceGeneric, Reference: "ref:local"})
	if err != nil || r.Complete || r.Observation.State != ObservationUnknown {
		t.Fatalf("unexpected generic capture: %+v %v", r, err)
	}
}
