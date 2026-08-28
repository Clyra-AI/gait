package actioncontract

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type captureRoundTripper struct {
	body   string
	status int
}

func (r *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, _ := io.ReadAll(req.Body)
	r.body = string(raw)
	return &http.Response{StatusCode: r.status, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
}

func TestOTLPExporterDefaultRedactionDedupeAndFailure(t *testing.T) {
	if result := ExportOTLP(OTLPExporter{}, map[string]string{"secret": "do-not-export"}); result.Attempted {
		t.Fatal("default-off exporter attempted transport")
	}
	roundTripper := &captureRoundTripper{status: 200}
	result := ExportOTLP(OTLPExporter{Endpoint: "http://collector.invalid", Client: &http.Client{Transport: roundTripper}}, map[string]string{"secret": "do-not-export"})
	if !result.Sent || strings.Contains(roundTripper.body, "do-not-export") {
		t.Fatalf("redacted export: %#v body=%s", result, roundTripper.body)
	}
	ref := strictControlRef("action_contract", "pac-otlp")
	record, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, OccurredAt: testTime(), ContractRef: ref, Revision: 1, ProposalRef: &ref, SigningPrivateKey: DevelopmentPrivateKey()})
	if err != nil {
		t.Fatal(err)
	}
	roundTripper = &captureRoundTripper{status: 500}
	result = ExportLifecycleOTLP(OTLPExporter{Endpoint: "http://collector.invalid", Client: &http.Client{Transport: roundTripper}}, []LifecycleRecord{record, record})
	if result.Sent || !result.Deduped || result.Error == "" {
		t.Fatalf("dedupe/failure: %#v", result)
	}
	roundTripper = &captureRoundTripper{status: 200}
	if result = ExportLifecycleOTLP(OTLPExporter{Endpoint: "http://collector.invalid", Client: &http.Client{Transport: roundTripper}}, []LifecycleRecord{record}); !result.Sent {
		t.Fatalf("lifecycle export: %#v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(roundTripper.body), &payload); err != nil {
		t.Fatal(err)
	}
	resource := payload["resourceSpans"].([]any)[0].(map[string]any)
	scope := resource["scopeSpans"].([]any)[0].(map[string]any)
	span := scope["spans"].([]any)[0].(map[string]any)
	if _, ok := span["startTimeUnixNano"].(string); !ok {
		t.Fatalf("OTLP span timestamp is not protobuf JSON integer: %#v", span)
	}
	if _, ok := span["attributes"].([]any); !ok {
		t.Fatalf("OTLP span attributes are not repeated key/value messages: %#v", span)
	}
}

func testTime() (t time.Time) { return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC) }
