package actioncontract

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"sync"
)

var lifecycleJournalMu sync.Mutex

// AppendLifecycleRecord appends one signed lifecycle transition to an
// existing JSONL journal. It intentionally does not create a parallel store;
// callers may point it at the runpack/session journal they already retain.
func AppendLifecycleRecord(path string, record LifecycleRecord) error {
	if path == "" {
		return errors.New("lifecycle journal path required")
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	lifecycleJournalMu.Lock()
	defer lifecycleJournalMu.Unlock()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600) // #nosec G304 -- explicit operator-selected journal path.
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err = f.Write(append(raw, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func ReadLifecycleJournal(path string) ([]LifecycleRecord, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- explicit operator-selected journal path.
	if err != nil {
		return nil, err
	}
	return ParseLifecycleJSONL(raw)
}

func VerifyLifecycleJournal(path string, publicKey ed25519.PublicKey) error {
	records, err := ReadLifecycleJournal(path)
	if err != nil {
		return err
	}
	return VerifyLifecycleRecords(records, publicKey)
}

// VerifyLifecycleRecords authenticates and structurally reduces an already
// loaded lifecycle prefix. Callers that append records under a different
// boundary signer can verify each homogeneous prefix with its own key.
func VerifyLifecycleRecords(records []LifecycleRecord, publicKey ed25519.PublicKey) error {
	seen := map[string]struct{}{}
	for _, record := range records {
		if _, exists := seen[record.RecordID]; exists {
			return errors.New("duplicate lifecycle record")
		}
		seen[record.RecordID] = struct{}{}
		valid, verifyErr := VerifyLifecycleRecord(record, publicKey)
		if verifyErr != nil || !valid {
			if verifyErr != nil {
				return verifyErr
			}
			return errors.New("lifecycle signature invalid")
		}
	}
	if _, err := ReduceLifecycleChecked(records); err != nil {
		return err
	}
	return nil
}

func ParseLifecycleJSONL(raw []byte) ([]LifecycleRecord, error) {
	lines := splitJSONLines(raw)
	out := make([]LifecycleRecord, 0, len(lines))
	for _, line := range lines {
		var record LifecycleRecord
		if err := DecodeStrictRuntimeJSON(line, &record); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

func splitJSONLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				lines = append(lines, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
