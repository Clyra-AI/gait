package actioncontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxRuntimeInputBytes int64 = 4 << 20

// DecodeStrictRuntimeJSON rejects duplicate keys, trailing values, unknown
// fields, and oversized payloads before any runtime decision is made.
func DecodeStrictRuntimeJSON(raw []byte, target any) error {
	if int64(len(raw)) > MaxRuntimeInputBytes {
		return errors.New("runtime input exceeds size limit")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return errors.New("runtime input is malformed")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("runtime input is malformed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("runtime input is malformed")
	}
	return nil
}

func validateRuntimeSchema(raw []byte, schemaID string) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	return validateSchema(raw, schemaID, "")
}

// ReadRuntimeInput provides a descriptor-bound, bounded, no-follow read for
// explicit CLI JSON paths. It compares two descriptor reads and the final
// pathname identity so replacement/mutation races fail closed.
func ReadRuntimeInput(path string) ([]byte, error) {
	abs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil || strings.TrimSpace(path) == "" {
		return nil, errors.New("runtime input is unreadable")
	}
	if err := rejectRuntimeAncestors(abs); err != nil {
		return nil, err
	}
	initial, err := os.Lstat(abs)
	if err != nil || initial.Mode()&os.ModeSymlink != 0 || !initial.Mode().IsRegular() {
		return nil, errors.New("runtime input is not a regular file")
	}
	if initial.Size() > MaxRuntimeInputBytes {
		return nil, errors.New("runtime input exceeds size limit")
	}
	file, err := os.Open(abs) // #nosec G304 -- explicit operator-selected runtime input.
	if err != nil {
		return nil, errors.New("runtime input is unreadable")
	}
	defer func() { _ = file.Close() }()
	descriptor, err := file.Stat()
	if err != nil || !descriptor.Mode().IsRegular() || descriptor.Size() > MaxRuntimeInputBytes {
		return nil, errors.New("runtime input is not a stable regular file")
	}
	first, err := io.ReadAll(io.LimitReader(file, MaxRuntimeInputBytes+1))
	if err != nil || int64(len(first)) > MaxRuntimeInputBytes {
		return nil, errors.New("runtime input is unreadable")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("runtime input is unreadable")
	}
	second, err := io.ReadAll(io.LimitReader(file, MaxRuntimeInputBytes+1))
	if err != nil || !bytes.Equal(first, second) {
		return nil, errors.New("runtime input changed during read")
	}
	final, err := os.Lstat(abs)
	if err != nil || final.Mode()&os.ModeSymlink != 0 || !final.Mode().IsRegular() || !os.SameFile(descriptor, final) {
		return nil, errors.New("runtime input changed during read")
	}
	return first, nil
}

func rejectRuntimeAncestors(path string) error {
	current := filepath.Dir(path)
	for {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("runtime input path is unsafe")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}
