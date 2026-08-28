package actioncontract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type advisoryTransport struct {
	body   []byte
	status int
}

func (t advisoryTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: t.status, Body: io.NopCloser(strings.NewReader(string(t.body))), Header: make(http.Header)}, nil
}

type fixedAdvisoryEvaluator struct {
	report AdvisoryReport
	err    error
}

func (f fixedAdvisoryEvaluator) Evaluate(AdvisoryInput) (AdvisoryReport, error) {
	return f.report, f.err
}

func TestEvaluateAdvisoryModesAndAuthorityBoundary(t *testing.T) {
	input := AdvisoryInput{ActionID: "action:test"}
	if report, err := EvaluateAdvisory(AdvisoryModeOff, fixedAdvisoryEvaluator{}, input); err != nil || report.ActionID != "" {
		t.Fatalf("off mode: %#v %v", report, err)
	}
	if _, err := EvaluateAdvisory(AdvisoryModeRequired, nil, input); err == nil {
		t.Fatal("required mode accepted missing provider")
	}
	report := AdvisoryReport{Status: "pass", AdvisoryOnly: false, ProviderName: "fake", ProviderVersion: "1", ActionID: input.ActionID}
	if _, err := EvaluateAdvisory(AdvisoryModeAdvisory, fixedAdvisoryEvaluator{report: report}, input); err == nil {
		t.Fatal("provider authority claim accepted")
	}
}

func TestCommandAdvisoryEvaluatorBoundedAndStrict(t *testing.T) {
	dir := t.TempDir()
	report, _ := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "action:provider"})
	raw, _ := json.Marshal(report)
	path := filepath.Join(dir, "report.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	command := os.Args[0]
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GAIT_ADVISORY_HELPER=") && !strings.HasPrefix(value, "GAIT_ADVISORY_PATH=") {
			env = append(env, value)
		}
	}
	env = append(env, "GAIT_ADVISORY_HELPER=1", "GAIT_ADVISORY_PATH="+path)
	got, err := (CommandAdvisoryEvaluator{Command: command, Args: []string{"-test.run=TestAdvisoryProviderHelperProcess"}, Timeout: time.Second, Env: env}).Evaluate(AdvisoryInput{ActionID: "action:provider"})
	if err != nil || !got.AdvisoryOnly {
		t.Fatalf("command provider: %#v %v", got, err)
	}
	if _, err := (CommandAdvisoryEvaluator{Command: command, Args: []string{filepath.Join(dir, "missing")}, Timeout: time.Millisecond}).Evaluate(AdvisoryInput{ActionID: "action:provider"}); err == nil {
		t.Fatal("missing provider input accepted")
	}
}

func TestCommandAdvisoryEvaluatorFailureAndTimeoutBranches(t *testing.T) {
	input := AdvisoryInput{ActionID: "action:provider-errors"}
	if _, err := (CommandAdvisoryEvaluator{}).Evaluate(input); err == nil {
		t.Fatal("empty advisory command accepted")
	}
	if _, err := (CommandAdvisoryEvaluator{Command: filepath.Join(t.TempDir(), "missing")}).Evaluate(input); err == nil {
		t.Fatal("missing advisory command accepted")
	}
	t.Setenv("GAIT_ADVISORY_SLEEP", "1")
	if _, err := (CommandAdvisoryEvaluator{Command: os.Args[0], Args: []string{"-test.run=TestAdvisoryProviderSleepProcess"}, Timeout: time.Millisecond}).Evaluate(input); err == nil {
		t.Fatal("timed out advisory command accepted")
	}
	badReport := filepath.Join(t.TempDir(), "bad-report.json")
	if err := os.WriteFile(badReport, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := append([]string{}, os.Environ()...)
	env = append(env, "GAIT_ADVISORY_HELPER=1", "GAIT_ADVISORY_PATH="+badReport)
	if _, err := (CommandAdvisoryEvaluator{Command: os.Args[0], Args: []string{"-test.run=TestAdvisoryProviderHelperProcess"}, Env: env}).Evaluate(input); err == nil {
		t.Fatal("malformed advisory report accepted")
	}
}

func TestAdvisoryProviderHelperProcess(t *testing.T) {
	if os.Getenv("GAIT_ADVISORY_HELPER") != "1" {
		t.Skip("helper process")
	}
	raw, _ := os.ReadFile(os.Getenv("GAIT_ADVISORY_PATH"))
	_, _ = fmt.Print(string(raw))
	os.Exit(0)
}

func TestAdvisoryProviderSleepProcess(t *testing.T) {
	if os.Getenv("GAIT_ADVISORY_SLEEP") != "1" {
		t.Skip("helper process")
	}
	select {}
}

func TestOllamaAdvisoryEvaluatorNoPullAndTimeoutBound(t *testing.T) {
	report, _ := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "action:ollama"})
	raw, _ := json.Marshal(report)
	got, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid/api/generate", Model: "local", Client: &http.Client{Transport: advisoryTransport{body: raw, status: 200}}}).Evaluate(AdvisoryInput{ActionID: "action:ollama"})
	if err != nil || !got.AdvisoryOnly {
		t.Fatalf("ollama fake: %#v %v", got, err)
	}
	if _, err := (OllamaAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "x"}); err == nil {
		t.Fatal("ollama missing config accepted")
	}
}

func TestOllamaAdvisoryEvaluatorFailureBranches(t *testing.T) {
	input := AdvisoryInput{ActionID: "action:ollama-errors"}
	if _, err := (OllamaAdvisoryEvaluator{Endpoint: "://bad", Model: "local"}).Evaluate(input); err == nil {
		t.Fatal("invalid Ollama endpoint accepted")
	}
	transportError := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if _, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid", Model: "local", Client: &http.Client{Transport: transportError}}).Evaluate(input); err == nil {
		t.Fatal("Ollama transport failure hidden")
	}
	if _, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid", Model: "local", Client: &http.Client{Transport: advisoryTransport{body: []byte("bad"), status: 500}}}).Evaluate(input); err == nil {
		t.Fatal("Ollama HTTP failure accepted")
	}
	if _, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid", Model: "local", Client: &http.Client{Transport: advisoryTransport{body: []byte(strings.Repeat("x", int(MaxAdvisoryProviderBytes)+1)), status: 200}}}).Evaluate(input); err == nil {
		t.Fatal("oversized Ollama response accepted")
	}
}

func TestOllamaAdvisoryEvaluatorEnforcesTimeoutWithInjectedClient(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	started := time.Now()
	_, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid", Model: "local", Timeout: 10 * time.Millisecond, Client: &http.Client{Transport: transport}}).Evaluate(AdvisoryInput{ActionID: "action:timeout"})
	if err == nil || time.Since(started) > time.Second || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("injected Ollama client ignored evaluator timeout: err=%v elapsed=%s", err, time.Since(started))
	}
}

func TestOllamaPromptUsesStructuredString(t *testing.T) {
	report, err := (OfflineAdvisoryEvaluator{}).Evaluate(AdvisoryInput{ActionID: "action:prompt"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(report)
	transport := &promptCaptureTransport{body: raw}
	if _, err := (OllamaAdvisoryEvaluator{Endpoint: "http://ollama.invalid", Model: "local", Client: &http.Client{Transport: transport}}).Evaluate(AdvisoryInput{ActionID: "action:prompt"}); err != nil {
		t.Fatal(err)
	}
	var request struct {
		Prompt any `json:"prompt"`
	}
	if err := json.Unmarshal(transport.request, &request); err != nil {
		t.Fatal(err)
	}
	if _, ok := request.Prompt.(string); !ok {
		t.Fatalf("Ollama prompt must be a structured JSON string: %#v", request.Prompt)
	}
}

type promptCaptureTransport struct {
	body    []byte
	request []byte
}

func (t *promptCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	t.request, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(t.body))), Header: make(http.Header)}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestEvaluateAdvisoryOptionalFailureModes(t *testing.T) {
	input := AdvisoryInput{ActionID: "action:optional"}
	if _, err := EvaluateAdvisory(AdvisoryModeAdvisory, nil, input); err != nil {
		t.Fatal(err)
	}
	failing := fixedAdvisoryEvaluator{err: errors.New("provider unavailable")}
	if _, err := EvaluateAdvisory(AdvisoryModeAdvisory, failing, input); err != nil {
		t.Fatal(err)
	}
	if _, err := EvaluateAdvisory(AdvisoryModeRequired, failing, input); err == nil {
		t.Fatal("required provider failure hidden")
	}
	wrapped := `{"response":"{\"status\":\"pass\",\"action_id\":\"action:optional\",\"advisory_only\":true}"}`
	report, err := parseProviderReport([]byte(wrapped), "ollama")
	if err != nil || report.ProviderName != "ollama" || report.ProviderVersion != "external" {
		t.Fatalf("wrapped report: %#v %v", report, err)
	}
}

func TestLimitedAdvisoryWriterRejectsOversize(t *testing.T) {
	var output bytes.Buffer
	writer := &limitedBuffer{Buffer: &output, Limit: 2}
	if _, err := writer.Write([]byte("abc")); err == nil {
		t.Fatal("oversized provider output accepted")
	}
	if _, err := writer.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
}
