package effects

// Bounded local effect capture. Capture is observation only: it never executes
// a tool, follows redirects, or treats an observation as authorization.
import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MaxCaptureBytes    = 1 << 20
	MaxCaptureFiles    = 1024
	MaxCaptureURLBytes = 1 << 20
)

var effectDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func validCanonicalDigest(v string) bool { return effectDigestPattern.MatchString(v) }

type CaptureRequest struct {
	ResourceKind     string
	Path             string
	URL              string
	Reference        string
	Now              time.Time
	Client           *http.Client
	AllowUnsafeLocal bool
	Resolve          func(string) ([]net.IP, error)
	Dial             func(context.Context, string, string) (net.Conn, error)
}

type CaptureResult struct {
	Observation Observation `json:"observation"`
	Complete    bool        `json:"complete"`
	Reason      string      `json:"reason,omitempty"`
}

func WriteCaptureResult(path string, r CaptureResult) error {
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return writeAtomicExclusive(path, raw)
}
func WriteSnapshot(path string, s Snapshot) error {
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	return writeAtomicExclusive(path, raw)
}
func LoadCaptureResult(path string) (CaptureResult, error) {
	raw, e := readSelectedFile(path)
	if e != nil {
		return CaptureResult{}, e
	}
	var r CaptureResult
	if e = decodeStrict(raw, &r); e != nil {
		return r, e
	}
	if len(raw) > maxEffectInputBytes {
		return r, errors.New("capture result too large")
	}
	if r.Observation.ObservedAt == "" || r.Observation.Identity == "" {
		return r, errors.New("capture result invalid")
	}
	return r, nil
}

// Capture observes a filesystem path, an HTTP URL, or a generic reference.
// Generic capture is deliberately reference-only and does not invoke commands.
func CaptureLocal(req CaptureRequest) (CaptureResult, error) {
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	kind := strings.ToLower(strings.TrimSpace(req.ResourceKind))
	switch kind {
	case ResourceFilesystem:
		return captureFilesystem(req, now)
	case ResourceHTTP:
		return captureHTTP(req, now)
	case ResourceGeneric:
		ref := strings.TrimSpace(req.Reference)
		if ref == "" {
			return CaptureResult{}, errors.New("generic capture requires --reference")
		}
		return CaptureResult{Observation: Observation{State: ObservationUnknown, Identity: ref, ObservedAt: now.Format(time.RFC3339Nano)}, Complete: false, Reason: "generic_reference_only"}, nil
	default:
		return CaptureResult{}, fmt.Errorf("unsupported capture resource kind %q", req.ResourceKind)
	}
}

func captureFilesystem(req CaptureRequest, now time.Time) (CaptureResult, error) {
	path := filepath.Clean(strings.TrimSpace(req.Path))
	if path == "." || path == "" {
		return CaptureResult{}, errors.New("filesystem capture requires --path")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return CaptureResult{Observation: Observation{State: ObservationAbsent, Identity: path, ObservedAt: now.Format(time.RFC3339Nano)}, Complete: true}, nil
	}
	if err != nil {
		return CaptureResult{}, fmt.Errorf("stat capture path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return CaptureResult{}, errors.New("filesystem capture refuses symlinks")
	}
	h := sha256.New()
	count := int64(0)
	totalBytes := int64(0)
	if info.IsDir() {
		err = filepath.Walk(path, func(p string, i os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if count >= MaxCaptureFiles {
				return errors.New("capture file limit exceeded")
			}
			if i.Mode()&os.ModeSymlink != 0 {
				return errors.New("filesystem capture refuses symlinks")
			}
			if !i.IsDir() && !i.Mode().IsRegular() {
				return errors.New("filesystem capture refuses special files")
			}
			rel, _ := filepath.Rel(path, p)
			h.Write([]byte(rel))
			h.Write([]byte{0})
			if i.IsDir() {
				h.Write([]byte("dir"))
				h.Write([]byte{0})
				count++
				return nil
			}
			if i.Size() > MaxCaptureBytes {
				return errors.New("capture byte limit exceeded")
			}
			totalBytes += i.Size()
			if totalBytes > MaxCaptureBytes {
				return errors.New("capture byte limit exceeded")
			}
			content, readErr := os.ReadFile(p) // #nosec G304 -- p is produced by the bounded, symlink-rejecting filepath.Walk rooted at the explicit capture path.
			if readErr != nil {
				return readErr
			}
			fileDigest := sha256.Sum256(content)
			h.Write([]byte("file:"))
			h.Write(fileDigest[:])
			h.Write([]byte{0})
			count++
			return nil
		})
	} else {
		if info.Size() > MaxCaptureBytes {
			return CaptureResult{}, errors.New("capture byte limit exceeded")
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return CaptureResult{}, fmt.Errorf("open capture path: %w", openErr)
		}
		_, err = io.Copy(h, io.LimitReader(f, MaxCaptureBytes+1))
		_ = f.Close()
		if err != nil {
			return CaptureResult{}, fmt.Errorf("read capture path: %w", err)
		}
		count = 1
	}
	if err != nil {
		return CaptureResult{}, err
	}
	d := "sha256:" + hex.EncodeToString(h.Sum(nil))
	return CaptureResult{Observation: Observation{State: ObservationPresent, Digest: d, Count: &count, Identity: path, ObservedAt: now.Format(time.RFC3339Nano)}, Complete: true}, nil
}

func captureHTTP(req CaptureRequest, now time.Time) (CaptureResult, error) {
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return CaptureResult{}, errors.New("http capture requires --url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return CaptureResult{}, errors.New("http capture requires an absolute http or https URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return CaptureResult{}, errors.New("http capture rejects credentials and fragments")
	}
	if !req.AllowUnsafeLocal {
		host := parsed.Hostname()
		var ips []net.IP
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else if req.Resolve != nil {
			var re error
			ips, re = req.Resolve(host)
			if re != nil {
				return CaptureResult{}, re
			}
		} else {
			var re error
			ips, re = net.LookupIP(host)
			if re != nil {
				return CaptureResult{}, re
			}
		}
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
				return CaptureResult{}, errors.New("http capture rejects local or private address")
			}
		}
		if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()) {
			return CaptureResult{}, errors.New("http capture rejects local or private address")
		}
		if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
			return CaptureResult{}, errors.New("http capture rejects localhost")
		}
	}
	client := req.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	tr := &http.Transport{Proxy: nil, ResponseHeaderTimeout: client.Timeout, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ip := net.ParseIP(host)
		if ip == nil {
			ips, re := resolveCaptureHost(req, host)
			if re != nil {
				return nil, re
			}
			ip = ips[0]
			if !req.AllowUnsafeLocal && !validCaptureIP(ip) {
				return nil, errors.New("http capture DNS private result")
			}
		}
		if ip == nil {
			return nil, errors.New("http capture address invalid")
		}
		target := net.JoinHostPort(ip.String(), port)
		if req.Dial != nil {
			return req.Dial(ctx, network, target)
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, target)
	}}
	client = &http.Client{Transport: tr, Timeout: client.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(rawURL)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("http capture: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.ContentLength > MaxCaptureURLBytes {
		return CaptureResult{}, errors.New("http capture byte limit exceeded")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxCaptureURLBytes+1))
	if err != nil {
		return CaptureResult{}, fmt.Errorf("read http capture: %w", err)
	}
	if len(body) > MaxCaptureURLBytes {
		return CaptureResult{}, errors.New("http capture byte limit exceeded")
	}
	h := sha256.Sum256(body)
	count := int64(1)
	state := ObservationPresent
	if resp.StatusCode >= 400 {
		state = ObservationUnknown
	}
	return CaptureResult{Observation: Observation{State: state, Digest: "sha256:" + hex.EncodeToString(h[:]), Count: &count, Identity: rawURL, ObservedAt: now.Format(time.RFC3339Nano)}, Complete: resp.StatusCode < 400, Reason: fmt.Sprintf("http_status_%d", resp.StatusCode)}, nil
}
func validCaptureIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsMulticast()
}
func resolveCaptureHost(req CaptureRequest, host string) ([]net.IP, error) {
	var ips []net.IP
	var e error
	if req.Resolve != nil {
		ips, e = req.Resolve(host)
	} else {
		ips, e = net.LookupIP(host)
	}
	if e != nil {
		return nil, e
	}
	if len(ips) == 0 {
		return nil, errors.New("http capture DNS empty")
	}
	for _, ip := range ips {
		if !req.AllowUnsafeLocal && !validCaptureIP(ip) {
			return nil, errors.New("http capture DNS private result")
		}
	}
	sort.Slice(ips, func(i, j int) bool { return ips[i].String() < ips[j].String() })
	return ips, nil
}

// BuildSnapshot turns a capture observation into the existing signed effect
// snapshot envelope. Both observations are intentionally identical: callers
// needing before/after semantics must capture both sides explicitly.
func BuildSnapshot(req CaptureRequest, correlation Correlation, privateKey []byte) (Snapshot, error) {
	result, err := CaptureLocal(req)
	if err != nil {
		return Snapshot{}, err
	}
	if strings.TrimSpace(correlation.ActionDigest) == "" && strings.TrimSpace(correlation.ActivationDigest) == "" && strings.TrimSpace(correlation.ProofDigest) == "" {
		return Snapshot{}, errors.New("capture correlation digest required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Snapshot{}, errors.New("capture signing key required")
	}
	if req.ResourceKind == ResourceFilesystem && strings.TrimSpace(req.Path) == "" || req.ResourceKind == ResourceHTTP && strings.TrimSpace(req.URL) == "" || req.ResourceKind == ResourceGeneric && strings.TrimSpace(req.Reference) == "" {
		return Snapshot{}, errors.New("capture selector identity missing")
	}
	sel := Selector{Resource: req.ResourceKind, Path: req.Path, URL: req.URL, Name: req.Reference}
	if !selectorIdentityMatches(sel, result.Observation.Identity) {
		return Snapshot{}, errors.New("capture selector identity mismatch")
	}
	now := result.Observation.ObservedAt
	seedRaw, _ := json.Marshal(struct {
		Resource    string
		Selector    Selector
		Observation Observation
		Correlation Correlation
	}{req.ResourceKind, sel, result.Observation, correlation})
	seed := sha256.Sum256(seedRaw)
	sid := "effect-snapshot:" + hex.EncodeToString(seed[:])[:16]
	od := observationDigest(result.Observation)
	s := Snapshot{SchemaID: SnapshotSchemaID, SchemaVersion: SchemaVersion, SnapshotID: sid, ResourceKind: req.ResourceKind, Selector: Selector{Resource: req.ResourceKind, Path: req.Path, URL: req.URL, Name: req.Reference}, Before: result.Observation, After: result.Observation, Collector: Collector{Name: "gait-local-capture", Version: "1", Mode: "bounded"}, Capture: Capture{Mode: "reference", SourceRef: result.Observation.Identity, CapturedAt: now}, Redaction: Redaction{Mode: "reference_only"}, Correlation: correlation, Completeness: CompletenessPartial, Enforcement: EnforcementObservedOnly, EvidenceRefs: []string{"capture:" + od}}
	out, err := s.Sign(ed25519.PrivateKey(privateKey), "collector_signed")
	if err != nil {
		return Snapshot{}, err
	}
	if v := ValidateSnapshot(out); !v.Valid {
		return Snapshot{}, errors.New("effect snapshot validation failed")
	}
	return out, nil
}
func selectorIdentityMatches(s Selector, identity string) bool {
	switch s.Resource {
	case ResourceFilesystem:
		return filepath.Clean(s.Path) == filepath.Clean(identity)
	case ResourceHTTP:
		return s.URL == identity
	default:
		return s.Name == identity || s.Path == identity
	}
}
func observationDigest(o Observation) string {
	raw, _ := json.Marshal(o)
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}

func BuildSnapshotFromObservations(before, after CaptureResult, resource string, selector Selector, correlation Correlation, privateKey []byte) (Snapshot, error) {
	if !before.Complete || !after.Complete {
		return Snapshot{}, errors.New("effect observations incomplete")
	}
	if before.Observation.State == ObservationUnknown || after.Observation.State == ObservationUnknown || (before.Observation.State == ObservationPresent && !validCanonicalDigest(before.Observation.Digest)) || (after.Observation.State == ObservationPresent && !validCanonicalDigest(after.Observation.Digest)) {
		return Snapshot{}, errors.New("effect observation invalid")
	}
	if before.Observation.Identity != after.Observation.Identity || selector.Resource != resource {
		return Snapshot{}, errors.New("effect observation identity mismatch")
	}
	if !selectorIdentityMatches(selector, before.Observation.Identity) {
		return Snapshot{}, errors.New("effect selector identity mismatch")
	}
	bt, e := time.Parse(time.RFC3339Nano, before.Observation.ObservedAt)
	if e != nil {
		return Snapshot{}, e
	}
	at, e := time.Parse(time.RFC3339Nano, after.Observation.ObservedAt)
	if e != nil || at.Before(bt) {
		return Snapshot{}, errors.New("effect observation time invalid")
	}
	seedRaw, err := json.Marshal(struct {
		Resource    string
		Selector    Selector
		Before      Observation
		After       Observation
		Correlation Correlation
	}{resource, selector, before.Observation, after.Observation, correlation})
	if err != nil {
		return Snapshot{}, err
	}
	seed := sha256.Sum256(seedRaw)
	s := Snapshot{SchemaID: SnapshotSchemaID, SchemaVersion: SchemaVersion, SnapshotID: "effect-snapshot:" + hex.EncodeToString(seed[:])[:16], ResourceKind: resource, Selector: selector, Before: before.Observation, After: after.Observation, Collector: Collector{Name: "gait-local-capture", Version: "1", Mode: "bounded"}, Capture: Capture{Mode: "reference", SourceRef: after.Observation.Identity, CapturedAt: after.Observation.ObservedAt}, Redaction: Redaction{Mode: "reference_only"}, Correlation: correlation, Completeness: CompletenessComplete, Enforcement: EnforcementObservedOnly, EvidenceRefs: []string{"capture:" + observationDigest(before.Observation), "capture:" + observationDigest(after.Observation)}}
	out, err := s.Sign(ed25519.PrivateKey(privateKey), "collector_signed")
	if err != nil {
		return Snapshot{}, err
	}
	if v := ValidateSnapshot(out); !v.Valid {
		return Snapshot{}, errors.New("effect snapshot validation failed")
	}
	return out, nil
}
