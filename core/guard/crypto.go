package guard

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type EncryptOptions struct {
	InputPath       string
	OutputPath      string
	KeyEnv          string
	KeyCommand      string
	KeyCommandArgs  []string
	ProducerVersion string
}

type EncryptResult struct {
	Path      string               `json:"path"`
	Artifact  EncryptedArtifact    `json:"artifact"`
	KeySource EncryptedArtifactKey `json:"key_source"`
}

type DecryptOptions struct {
	InputPath      string
	OutputPath     string
	KeyEnv         string
	KeyCommand     string
	KeyCommandArgs []string
}

type DecryptResult struct {
	Path        string `json:"path"`
	PlainSHA256 string `json:"plain_sha256"`
	PlainSize   int    `json:"plain_size"`
}

type EncryptedArtifact struct {
	SchemaID        string               `json:"schema_id"`
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	ProducerVersion string               `json:"producer_version"`
	Algorithm       string               `json:"algorithm"`
	KeySource       EncryptedArtifactKey `json:"key_source"`
	Nonce           string               `json:"nonce"`
	Ciphertext      string               `json:"ciphertext"`
	PlainSHA256     string               `json:"plain_sha256"`
	PlainSize       int                  `json:"plain_size"`
}

type EncryptedArtifactKey struct {
	Mode    string `json:"mode"`
	Ref     string `json:"ref,omitempty"`
	Command string `json:"command,omitempty"`
}

func EncryptArtifact(options EncryptOptions) (EncryptResult, error) {
	plainPath := strings.TrimSpace(options.InputPath)
	if plainPath == "" {
		return EncryptResult{}, fmt.Errorf("input path is required")
	}
	// #nosec G304 -- input path is explicit local user input.
	plainData, err := os.ReadFile(plainPath)
	if err != nil {
		return EncryptResult{}, fmt.Errorf("read input artifact: %w", err)
	}
	key, keySource, err := resolveEncryptionKey(options.KeyEnv, options.KeyCommand, options.KeyCommandArgs)
	if err != nil {
		return EncryptResult{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptResult{}, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptResult{}, fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptResult{}, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plainData, nil)

	createdAt := time.Now().UTC()
	producerVersion := strings.TrimSpace(options.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	plainSHA := sha256.Sum256(plainData)
	artifact := EncryptedArtifact{
		SchemaID:        "gait.guard.encrypted_artifact",
		SchemaVersion:   "1.0.0",
		CreatedAt:       createdAt,
		ProducerVersion: producerVersion,
		Algorithm:       "aes-256-gcm",
		KeySource:       keySource,
		Nonce:           base64.StdEncoding.EncodeToString(nonce),
		Ciphertext:      base64.StdEncoding.EncodeToString(ciphertext),
		PlainSHA256:     fmt.Sprintf("%x", plainSHA[:]),
		PlainSize:       len(plainData),
	}
	encoded, err := marshalCanonicalJSON(artifact)
	if err != nil {
		return EncryptResult{}, fmt.Errorf("encode encrypted artifact: %w", err)
	}

	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		outputPath = plainPath + ".gaitenc"
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return EncryptResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, encoded, 0o600); err != nil {
		return EncryptResult{}, fmt.Errorf("write encrypted artifact: %w", err)
	}
	return EncryptResult{
		Path:      outputPath,
		Artifact:  artifact,
		KeySource: keySource,
	}, nil
}

func DecryptArtifact(options DecryptOptions) (DecryptResult, error) {
	inputPath := strings.TrimSpace(options.InputPath)
	if inputPath == "" {
		return DecryptResult{}, fmt.Errorf("input path is required")
	}
	// #nosec G304 -- input path is explicit local user input.
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("read encrypted artifact: %w", err)
	}
	var artifact EncryptedArtifact
	if err := jsonUnmarshal(raw, &artifact); err != nil {
		return DecryptResult{}, fmt.Errorf("parse encrypted artifact: %w", err)
	}
	key, _, err := resolveEncryptionKey(options.KeyEnv, options.KeyCommand, options.KeyCommandArgs)
	if err != nil {
		return DecryptResult{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("create gcm: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(artifact.Nonce)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(artifact.Ciphertext)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("decode ciphertext: %w", err)
	}
	plainData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("decrypt artifact: %w", err)
	}
	plainSHA := sha256.Sum256(plainData)
	plainSHAHex := fmt.Sprintf("%x", plainSHA[:])
	if artifact.PlainSHA256 != "" && !strings.EqualFold(artifact.PlainSHA256, plainSHAHex) {
		return DecryptResult{}, fmt.Errorf("decrypted payload digest mismatch")
	}

	outputPath := strings.TrimSpace(options.OutputPath)
	if outputPath == "" {
		outputPath = strings.TrimSuffix(inputPath, ".gaitenc")
		if outputPath == inputPath {
			outputPath = inputPath + ".plain"
		}
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return DecryptResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, plainData, 0o600); err != nil {
		return DecryptResult{}, fmt.Errorf("write decrypted artifact: %w", err)
	}
	return DecryptResult{
		Path:        outputPath,
		PlainSHA256: plainSHAHex,
		PlainSize:   len(plainData),
	}, nil
}

func resolveEncryptionKey(envName string, command string, args []string) ([]byte, EncryptedArtifactKey, error) {
	trimmedEnv := strings.TrimSpace(envName)
	if trimmedEnv != "" {
		raw := strings.TrimSpace(os.Getenv(trimmedEnv))
		if raw == "" {
			return nil, EncryptedArtifactKey{}, fmt.Errorf("encryption key env var is empty: %s", trimmedEnv)
		}
		key, err := decodeEncryptionKey(raw)
		if err != nil {
			return nil, EncryptedArtifactKey{}, fmt.Errorf("decode encryption key from env: %w", err)
		}
		return key, EncryptedArtifactKey{Mode: "env", Ref: trimmedEnv}, nil
	}

	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand != "" {
		commandArgs := make([]string, 0, len(args))
		for _, arg := range args {
			trimmed := strings.TrimSpace(arg)
			if trimmed == "" {
				continue
			}
			commandArgs = append(commandArgs, trimmed)
		}
		// #nosec G204 -- command hook is explicit operator configuration.
		cmd := exec.Command(trimmedCommand, commandArgs...)
		out, err := cmd.Output()
		if err != nil {
			return nil, EncryptedArtifactKey{}, fmt.Errorf("run encryption key command: %w", err)
		}
		key, err := decodeEncryptionKey(strings.TrimSpace(string(out)))
		if err != nil {
			return nil, EncryptedArtifactKey{}, fmt.Errorf("decode encryption key from command: %w", err)
		}
		return key, EncryptedArtifactKey{Mode: "command", Command: trimmedCommand}, nil
	}
	return nil, EncryptedArtifactKey{}, fmt.Errorf("missing key source: provide --key-env or --key-command")
}

func decodeEncryptionKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("expected 32-byte key, got %d", len(key))
	}
	return key, nil
}

func jsonUnmarshal(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
