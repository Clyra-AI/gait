package effects

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func captureTestKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("capture-additional-test-key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func captureTestPair() (CaptureResult, CaptureResult) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	before := CaptureResult{Complete: true, Observation: Observation{State: ObservationPresent, Identity: "/tmp/effect-capture", Digest: digest, ObservedAt: "2026-08-20T00:00:00Z"}}
	after := before
	after.Observation.ObservedAt = "2026-08-20T00:00:01Z"
	after.Observation.Digest = "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	return before, after
}

func TestCapturePersistenceAndSafeWrites(t *testing.T) {
	dir := t.TempDir()
	result := CaptureResult{Complete: true, Observation: Observation{State: ObservationPresent, Identity: "ref:test", ObservedAt: "2026-08-20T00:00:00Z"}}
	resultPath := filepath.Join(dir, "capture.json")
	if err := WriteCaptureResult(resultPath, result); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCaptureResult(resultPath)
	if err != nil || loaded.Observation.Identity != result.Observation.Identity {
		t.Fatalf("capture round-trip: %#v %v", loaded, err)
	}
	if err := WriteCaptureResult(resultPath, result); err == nil {
		t.Fatal("existing capture output was overwritten")
	}

	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"observation":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaptureResult(badPath); err == nil {
		t.Fatal("invalid capture result accepted")
	}
	if err := os.WriteFile(badPath, []byte(`{"observation":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaptureResult(badPath); err == nil {
		t.Fatal("malformed capture result accepted")
	}

	before, after := captureTestPair()
	snapshot, err := BuildSnapshotFromObservations(before, after, ResourceFilesystem, Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, Correlation{ActionDigest: before.Observation.Digest}, captureTestKey())
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := WriteSnapshot(snapshotPath, snapshot); err != nil {
		t.Fatal(err)
	}
	loadedSnapshot, err := LoadSnapshot(snapshotPath)
	if err != nil || loadedSnapshot.SnapshotID != snapshot.SnapshotID {
		t.Fatalf("snapshot round-trip: %s %v", loadedSnapshot.SnapshotID, err)
	}
	if err := WriteSnapshot(snapshotPath, snapshot); err == nil {
		t.Fatal("existing snapshot output was overwritten")
	}
	if runtime.GOOS != "windows" {
		target := filepath.Join(dir, "target.json")
		if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link.json")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if err := WriteCaptureResult(link, result); err == nil {
			t.Fatal("symlink output accepted")
		}
	}
}

func TestCaptureFilesystemDirectoryAndSymlinkFailures(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "item"), []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: dir, Now: mustCaptureTime("2026-08-20T00:00:00Z")})
	if err != nil || !result.Complete || result.Observation.State != ObservationPresent || result.Observation.Count == nil || *result.Observation.Count < 2 {
		t.Fatalf("directory capture: %#v %v", result, err)
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem}); err == nil {
		t.Fatal("missing filesystem path accepted")
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: "unsupported", Path: dir}); err == nil {
		t.Fatal("unsupported resource accepted")
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(nested, link); err != nil {
			t.Fatal(err)
		}
		if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceFilesystem, Path: link}); err == nil {
			t.Fatal("filesystem symlink accepted")
		}
	}
}

func TestCaptureHTTPResolverStatusAndInputFailures(t *testing.T) {
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test", Resolve: func(string) ([]net.IP, error) { return nil, errors.New("resolver failed") }}); err == nil {
		t.Fatal("resolver failure was ignored")
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://user:pass@example.test"}); err == nil {
		t.Fatal("URL credentials accepted")
	}
	if _, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test/#fragment"}); err == nil {
		t.Fatal("URL fragment accepted")
	}
	calls := []string{}
	result, err := CaptureLocal(CaptureRequest{ResourceKind: ResourceHTTP, URL: "http://example.test/fail", Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }, Dial: fakeHTTPDial("HTTP/1.1 500 Internal Server Error\r\nContent-Length: 3\r\n\r\nbad", &calls)})
	if err != nil || result.Complete || result.Observation.State != ObservationUnknown || result.Reason != "http_status_500" {
		t.Fatalf("HTTP status handling: %#v %v", result, err)
	}
}

func TestBuildSnapshotPairRejectsTemporalAndSelectorFailures(t *testing.T) {
	before, after := captureTestPair()
	key := captureTestKey()
	cases := []struct {
		name   string
		before CaptureResult
		after  CaptureResult
		res    string
		sel    Selector
		key    []byte
	}{
		{name: "incomplete", before: CaptureResult{Observation: before.Observation}, after: after, res: ResourceFilesystem, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, key: key},
		{name: "bad digest", before: before, after: after, res: ResourceFilesystem, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, key: key},
		{name: "after before", before: before, after: func() CaptureResult { v := after; v.Observation.ObservedAt = "2026-08-19T00:00:00Z"; return v }(), res: ResourceFilesystem, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, key: key},
		{name: "selector mismatch", before: before, after: after, res: ResourceFilesystem, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/other"}, key: key},
		{name: "resource mismatch", before: before, after: after, res: ResourceHTTP, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, key: key},
		{name: "key missing", before: before, after: after, res: ResourceFilesystem, sel: Selector{Resource: ResourceFilesystem, Path: "/tmp/effect-capture"}, key: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.before
			if tc.name == "bad digest" {
				b.Observation.Digest = "not-a-digest"
			}
			if _, err := BuildSnapshotFromObservations(b, tc.after, tc.res, tc.sel, Correlation{ActionDigest: before.Observation.Digest}, tc.key); err == nil {
				t.Fatal("invalid observation pair accepted")
			}
		})
	}
}

func mustCaptureTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, raw)
	return t
}
