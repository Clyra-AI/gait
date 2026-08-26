package actioncontract

// Replaceable advisory providers. Providers are intentionally outside the
// authorization path: their output is bounded, schema-checked evidence only.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type AdvisoryMode string

const (
	AdvisoryModeOff      AdvisoryMode = "off"
	AdvisoryModeAdvisory AdvisoryMode = "advisory"
	AdvisoryModeRequired AdvisoryMode = "required"
)

const MaxAdvisoryProviderBytes int64 = 1 << 20

type CommandAdvisoryEvaluator struct {
	Command string
	Args    []string
	Env     []string
	Timeout time.Duration
}

func (e CommandAdvisoryEvaluator) Evaluate(in AdvisoryInput) (AdvisoryReport, error) {
	if strings.TrimSpace(e.Command) == "" {
		return AdvisoryReport{}, errors.New("advisory command is required")
	}
	timeout := e.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return AdvisoryReport{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.Command, e.Args...) // #nosec G204 -- command is explicit operator configuration.
	if e.Env != nil {
		cmd.Env = e.Env
	}
	cmd.Stdin = bytes.NewReader(payload)
	var output bytes.Buffer
	cmd.Stdout = &limitedBuffer{Buffer: &output, Limit: MaxAdvisoryProviderBytes}
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return AdvisoryReport{}, errors.New("advisory provider timeout")
		}
		return AdvisoryReport{}, fmt.Errorf("advisory provider failed: %w", err)
	}
	return parseProviderReport(output.Bytes(), "command")
}

type OllamaAdvisoryEvaluator struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
	Client   *http.Client
}

func (e OllamaAdvisoryEvaluator) Evaluate(in AdvisoryInput) (AdvisoryReport, error) {
	if strings.TrimSpace(e.Endpoint) == "" || strings.TrimSpace(e.Model) == "" {
		return AdvisoryReport{}, errors.New("ollama endpoint and model are required")
	}
	timeout := e.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	promptRaw, err := json.Marshal(in)
	if err != nil {
		return AdvisoryReport{}, err
	}
	payload, err := json.Marshal(map[string]any{"model": e.Model, "stream": false, "prompt": string(promptRaw)})
	if err != nil {
		return AdvisoryReport{}, err
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequest(http.MethodPost, e.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return AdvisoryReport{}, err
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req = req.WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req) // #nosec G704 -- endpoint is explicit operator configuration; no model pull is performed.
	if err != nil {
		return AdvisoryReport{}, fmt.Errorf("ollama provider request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxAdvisoryProviderBytes+1))
	if err != nil || int64(len(raw)) > MaxAdvisoryProviderBytes {
		return AdvisoryReport{}, errors.New("ollama provider response exceeds size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AdvisoryReport{}, fmt.Errorf("ollama provider returned HTTP %d", resp.StatusCode)
	}
	return parseProviderReport(raw, "ollama")
}

func EvaluateAdvisory(mode AdvisoryMode, evaluator AdvisoryEvaluator, input AdvisoryInput) (AdvisoryReport, error) {
	switch mode {
	case "", AdvisoryModeOff:
		return AdvisoryReport{}, nil
	case AdvisoryModeAdvisory, AdvisoryModeRequired:
		if evaluator == nil {
			if mode == AdvisoryModeRequired {
				return AdvisoryReport{}, errors.New("required advisory provider is unavailable")
			}
			return AdvisoryReport{}, nil
		}
		report, err := evaluator.Evaluate(input)
		if err != nil {
			if mode == AdvisoryModeRequired {
				return AdvisoryReport{}, err
			}
			return AdvisoryReport{}, nil
		}
		if err := normalizeAdvisory(&report); err != nil {
			return AdvisoryReport{}, err
		}
		if !report.AdvisoryOnly {
			return AdvisoryReport{}, errors.New("advisory provider cannot claim authority")
		}
		return report, nil
	default:
		return AdvisoryReport{}, errors.New("advisory mode must be off, advisory, or required")
	}
}

func parseProviderReport(raw []byte, provider string) (AdvisoryReport, error) {
	var report AdvisoryReport
	if err := DecodeStrictRuntimeJSON(raw, &report); err != nil {
		// Ollama commonly wraps generated JSON in a response string. Decode it
		// only; never ask Ollama to install or pull a model.
		var envelope struct {
			Response string `json:"response"`
		}
		if json.Unmarshal(raw, &envelope) != nil || strings.TrimSpace(envelope.Response) == "" || DecodeStrictRuntimeJSON([]byte(envelope.Response), &report) != nil {
			return AdvisoryReport{}, errors.New("advisory provider response is malformed")
		}
	}
	if report.ProviderName == "" {
		report.ProviderName = provider
	}
	if report.ProviderVersion == "" {
		report.ProviderVersion = "external"
	}
	if !report.AdvisoryOnly {
		return AdvisoryReport{}, errors.New("advisory provider cannot claim authority")
	}
	if err := normalizeAdvisory(&report); err != nil {
		return AdvisoryReport{}, fmt.Errorf("advisory provider schema invalid: %w", err)
	}
	return report, nil
}

type limitedBuffer struct {
	*bytes.Buffer
	Limit int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > b.Limit {
		return 0, errors.New("advisory provider output exceeds size limit")
	}
	return b.Buffer.Write(p)
}
