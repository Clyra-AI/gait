package credential

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	defaultEnvPrefix      = "GAIT_BROKER_TOKEN_"
	defaultCommandTimeout = 5 * time.Second
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

	// #nosec G204 -- command broker execution is an explicit, opt-in local integration boundary.
	cmd := exec.CommandContext(ctx, command, b.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return Response{}, fmt.Errorf("%w: command broker timed out", ErrCredentialUnavailable)
	}
	if err != nil {
		return Response{}, fmt.Errorf("%w: command broker failed: %s", ErrCredentialUnavailable, strings.TrimSpace(string(output)))
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return Response{}, fmt.Errorf("command broker returned empty output")
	}

	response := Response{}
	if json.Unmarshal(output, &response) == nil {
		response.IssuedBy = strings.TrimSpace(response.IssuedBy)
		response.CredentialRef = strings.TrimSpace(response.CredentialRef)
		if response.IssuedBy == "" {
			response.IssuedBy = b.Name()
		}
		if response.CredentialRef == "" {
			return Response{}, fmt.Errorf("command broker returned empty credential_ref")
		}
		return response, nil
	}

	return Response{
		IssuedBy:      b.Name(),
		CredentialRef: trimmed,
	}, nil
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
		return CommandBroker{
			Command: strings.TrimSpace(command),
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
