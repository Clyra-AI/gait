package gate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	schemagate "github.com/davidahmann/gait/core/schema/v1/gate"
	"github.com/davidahmann/gait/core/sign"
)

type EmitTraceOptions struct {
	ProducerVersion   string
	CorrelationID     string
	ApprovalTokenRef  string
	LatencyMS         float64
	SigningPrivateKey ed25519.PrivateKey
	TracePath         string
}

type EmitTraceResult struct {
	Trace        schemagate.TraceRecord
	TracePath    string
	PolicyDigest string
	IntentDigest string
}

func EmitSignedTrace(policy Policy, intent schemagate.IntentRequest, gateResult schemagate.GateResult, opts EmitTraceOptions) (EmitTraceResult, error) {
	if len(opts.SigningPrivateKey) == 0 {
		return EmitTraceResult{}, fmt.Errorf("signing private key is required")
	}
	normalizedIntent, err := NormalizeIntent(intent)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("normalize intent for trace: %w", err)
	}
	policyDigest, err := PolicyDigest(policy)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("digest policy for trace: %w", err)
	}
	if normalizedIntent.IntentDigest == "" {
		return EmitTraceResult{}, fmt.Errorf("intent digest missing after normalization")
	}
	verdict := strings.TrimSpace(gateResult.Verdict)
	if verdict == "" {
		return EmitTraceResult{}, fmt.Errorf("gate result verdict is required for trace emission")
	}

	createdAt := gateResult.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = normalizedIntent.CreatedAt.UTC()
	}
	if createdAt.IsZero() {
		createdAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = strings.TrimSpace(gateResult.ProducerVersion)
	}
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	trace := schemagate.TraceRecord{
		SchemaID:         "gait.gate.trace",
		SchemaVersion:    "1.0.0",
		CreatedAt:        createdAt,
		ProducerVersion:  producerVersion,
		TraceID:          computeTraceID(policyDigest, normalizedIntent.IntentDigest, verdict),
		CorrelationID:    strings.TrimSpace(opts.CorrelationID),
		ToolName:         normalizedIntent.ToolName,
		ArgsDigest:       normalizedIntent.ArgsDigest,
		IntentDigest:     normalizedIntent.IntentDigest,
		PolicyDigest:     policyDigest,
		Verdict:          verdict,
		Violations:       uniqueSorted(gateResult.Violations),
		LatencyMS:        clampLatency(opts.LatencyMS),
		ApprovalTokenRef: strings.TrimSpace(opts.ApprovalTokenRef),
		SkillProvenance:  normalizedIntent.SkillProvenance,
	}

	signable := trace
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("marshal signable trace: %w", err)
	}
	signature, err := sign.SignTraceRecordJSON(opts.SigningPrivateKey, signableRaw)
	if err != nil {
		return EmitTraceResult{}, fmt.Errorf("sign trace record: %w", err)
	}
	trace.Signature = &schemagate.Signature{
		Alg:          signature.Alg,
		KeyID:        signature.KeyID,
		Sig:          signature.Sig,
		SignedDigest: signature.SignedDigest,
	}

	tracePath := strings.TrimSpace(opts.TracePath)
	if tracePath == "" {
		tracePath = fmt.Sprintf("trace_%s.json", trace.TraceID)
	}
	if err := WriteTraceRecord(tracePath, trace); err != nil {
		return EmitTraceResult{}, err
	}
	return EmitTraceResult{
		Trace:        trace,
		TracePath:    tracePath,
		PolicyDigest: policyDigest,
		IntentDigest: normalizedIntent.IntentDigest,
	}, nil
}

func WriteTraceRecord(path string, trace schemagate.TraceRecord) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create trace directory: %w", err)
		}
	}
	encoded, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trace record: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write trace record: %w", err)
	}
	return nil
}

func ReadTraceRecord(path string) (schemagate.TraceRecord, error) {
	// #nosec G304 -- trace path is explicit local user input.
	content, err := os.ReadFile(path)
	if err != nil {
		return schemagate.TraceRecord{}, fmt.Errorf("read trace record: %w", err)
	}
	var trace schemagate.TraceRecord
	if err := json.Unmarshal(content, &trace); err != nil {
		return schemagate.TraceRecord{}, fmt.Errorf("parse trace record: %w", err)
	}
	return trace, nil
}

func VerifyTraceRecordSignature(trace schemagate.TraceRecord, publicKey ed25519.PublicKey) (bool, error) {
	if trace.Signature == nil {
		return false, fmt.Errorf("trace signature missing")
	}
	signature := sign.Signature{
		Alg:          trace.Signature.Alg,
		KeyID:        trace.Signature.KeyID,
		Sig:          trace.Signature.Sig,
		SignedDigest: trace.Signature.SignedDigest,
	}
	signable := trace
	signable.Signature = nil
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return false, fmt.Errorf("marshal signable trace: %w", err)
	}
	return sign.VerifyTraceRecordJSON(publicKey, signature, signableRaw)
}

func computeTraceID(policyDigest, intentDigest, verdict string) string {
	sum := sha256.Sum256([]byte(policyDigest + ":" + intentDigest + ":" + verdict))
	return hex.EncodeToString(sum[:12])
}

func clampLatency(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
