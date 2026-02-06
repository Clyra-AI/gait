package credential

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultEnvPrefix              = "GAIT_BROKER_TOKEN_"
	defaultCommandTimeout         = 5 * time.Second
	defaultCommandOutputMaxBytes  = 16 * 1024
	defaultCredentialRefMaxLength = 256
	commandAllowlistEnv           = "GAIT_CREDENTIAL_COMMAND_ALLOWLIST"
)

type StubBroker struct{}

func (StubBroker) Name() string {
	return "stub"
}

func (StubBroker) Issue(request Request) (Response, error) {
	scope := strings.Join(normalizeScope(request.Scope), ",")
	raw := strings.Join([]string{
		request.ToolName,
		request.Identity,
		request.Workspace,
		request.SessionID,
		request.RequestID,
		request.Reference,
		scope,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return Response{
		IssuedBy:      "stub",
		CredentialRef: "stub:" + hex.EncodeToString(sum[:12]),
	}, nil
}

type EnvBroker struct {
	Prefix string
}

func (b EnvBroker) Name() string {
	return "env"
}

func (b EnvBroker) Issue(request Request) (Response, error) {
	prefix := strings.TrimSpace(b.Prefix)
	if prefix == "" {
		prefix = defaultEnvPrefix
	}
	tokenName := request.Reference
	if tokenName == "" {
		tokenName = request.ToolName
	}
	envKey := prefix + normalizeEnvKey(tokenName)
	value := strings.TrimSpace(os.Getenv(envKey))
	if value == "" {
		return Response{}, fmt.Errorf("%w: %s", ErrCredentialUnavailable, envKey)
	}
	sum := sha256.Sum256([]byte(value))
	return Response{
		IssuedBy:      "env",
		CredentialRef: "env:" + envKey + ":" + hex.EncodeToString(sum[:8]),
	}, nil
}

type CommandBroker struct {
	Command string
	Args    []string
	Timeout time.Duration
}

func (b CommandBroker) Name() string {
	return "command"
}

func (b CommandBroker) Issue(request Request) (Response, error) {
	command := strings.TrimSpace(b.Command)
	if command == "" {
		return Response{}, fmt.Errorf("command broker requires command")
	}
	if strings.ContainsAny(command, " \t\r\n") {
		return Response{}, fmt.Errorf("command broker command must not contain whitespace")
	}
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	payload, err := json.Marshal(request)
	if err != nil {
		return Response{}, fmt.Errorf("marshal command broker request: %w", err)
	}
	output, truncated, err := runCommandBroker(ctx, command, b.Args, payload, defaultCommandOutputMaxBytes)
	if ctx.Err() != nil {
		return Response{}, fmt.Errorf("%w: command broker timed out", ErrCredentialUnavailable)
	}
	if truncated {
		return Response{}, fmt.Errorf("%w: command broker output exceeded %d bytes", ErrCredentialUnavailable, defaultCommandOutputMaxBytes)
	}
	if err != nil {
		return Response{}, fmt.Errorf("%w: command broker failed", ErrCredentialUnavailable)
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return Response{}, fmt.Errorf("command broker returned empty output")
	}

	response := Response{}
	if err := json.Unmarshal(output, &response); err != nil {
		return Response{}, fmt.Errorf("%w: command broker must return JSON response", ErrCredentialUnavailable)
	}
	response.IssuedBy = strings.TrimSpace(response.IssuedBy)
	response.CredentialRef = strings.TrimSpace(response.CredentialRef)
	if response.IssuedBy == "" {
		response.IssuedBy = b.Name()
	}
	if response.CredentialRef == "" {
		return Response{}, fmt.Errorf("command broker returned empty credential_ref")
	}
	if len(response.CredentialRef) > defaultCredentialRefMaxLength {
		return Response{}, fmt.Errorf("command broker credential_ref too long")
	}
	if strings.ContainsAny(response.CredentialRef, "\r\n\t") {
		return Response{}, fmt.Errorf("command broker credential_ref contains invalid whitespace")
	}
	return response, nil
}

func ResolveBroker(name string, envPrefix string, command string, commandArgs []string) (Broker, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "off", "none":
		return nil, nil
	case "stub":
		return StubBroker{}, nil
	case "env":
		return EnvBroker{Prefix: strings.TrimSpace(envPrefix)}, nil
	case "command":
		if strings.TrimSpace(command) == "" {
			return nil, fmt.Errorf("command broker requires --credential-command")
		}
		commandPath := strings.TrimSpace(command)
		allowlist := normalizeCommandAllowlist(os.Getenv(commandAllowlistEnv))
		if len(allowlist) > 0 && !isCommandAllowed(commandPath, allowlist) {
			return nil, fmt.Errorf("command broker command is not in allowlist %s", commandAllowlistEnv)
		}
		return CommandBroker{
			Command: commandPath,
			Args:    normalizeCommandArgs(commandArgs),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported credential broker: %s", name)
	}
}

var envSanitizer = regexp.MustCompile(`[^A-Z0-9_]+`)

func normalizeEnvKey(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(value))
	upper = strings.ReplaceAll(upper, ".", "_")
	upper = strings.ReplaceAll(upper, "-", "_")
	upper = envSanitizer.ReplaceAllString(upper, "_")
	upper = strings.Trim(upper, "_")
	if upper == "" {
		return "DEFAULT"
	}
	return upper
}

func normalizeCommandArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

type limitedOutputBuffer struct {
	maxBytes  int
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedOutputBuffer) Write(payload []byte) (int, error) {
	if b.maxBytes <= 0 {
		b.maxBytes = defaultCommandOutputMaxBytes
	}
	remaining := b.maxBytes - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(payload), nil
	}
	if len(payload) > remaining {
		_, _ = b.buffer.Write(payload[:remaining])
		b.truncated = true
		return len(payload), nil
	}
	_, _ = b.buffer.Write(payload)
	return len(payload), nil
}

func runCommandBroker(ctx context.Context, command string, args []string, payload []byte, maxBytes int) ([]byte, bool, error) {
	// #nosec G204 -- command broker execution is an explicit, opt-in local integration boundary.
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = bytes.NewReader(payload)
	buffer := &limitedOutputBuffer{maxBytes: maxBytes}
	cmd.Stdout = io.Writer(buffer)
	cmd.Stderr = io.Writer(buffer)
	err := cmd.Run()
	return buffer.buffer.Bytes(), buffer.truncated, err
}

func normalizeCommandAllowlist(value string) []string {
	entries := strings.Split(value, ",")
	normalized := make([]string, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		trimmed := filepath.Clean(strings.TrimSpace(entry))
		if trimmed == "." || trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func isCommandAllowed(command string, allowlist []string) bool {
	trimmed := filepath.Clean(strings.TrimSpace(command))
	base := filepath.Base(trimmed)
	for _, allowed := range allowlist {
		if trimmed == allowed || base == allowed {
			return true
		}
	}
	return false
}
