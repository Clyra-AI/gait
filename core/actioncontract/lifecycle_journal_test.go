package actioncontract

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLifecycleJournalAppendVerifyTamperDuplicateAndConcurrent(t *testing.T) {
	private := DevelopmentPrivateKey()
	ref := strictControlRef("action_contract", "pac-journal")
	first, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, OccurredAt: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), ContractRef: ref, Revision: 1, ProposalRef: &ref, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "lifecycle.jsonl")
	if err := AppendLifecycleRecord(path, first); err != nil {
		t.Fatal(err)
	}
	read, err := ReadLifecycleJournal(path)
	if err != nil || len(read) != 1 || read[0].RecordID != first.RecordID {
		t.Fatalf("read journal: %#v %v", read, err)
	}
	if err := VerifyLifecycleJournal(path, private.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	raw = []byte(strings.Replace(string(raw), `"proposal_ingested"`, `"rejected"`, 1))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLifecycleJournal(path, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("tampered lifecycle journal verified")
	}
	if err := os.WriteFile(path, append(raw, raw...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLifecycleJournal(path, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("duplicate lifecycle journal verified")
	}

	path = filepath.Join(t.TempDir(), "concurrent.jsonl")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, recordErr := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleProposalIngested, OccurredAt: time.Date(2026, 8, 26, 0, 0, i, 0, time.UTC), ContractRef: ref, Revision: 1, ProposalRef: &ref, SigningPrivateKey: private})
			if recordErr == nil {
				_ = AppendLifecycleRecord(path, record)
			}
		}()
	}
	wg.Wait()
	if records, err := ReadLifecycleJournal(path); err != nil || len(records) != 8 {
		t.Fatalf("concurrent journal: %d %v", len(records), err)
	}
}

func TestVerifyLifecycleJournalRejectsSignedIllegalTransition(t *testing.T) {
	private := DevelopmentPrivateKey()
	ref := strictControlRef("action_contract", "pac-illegal-transition")
	record, err := NewLifecycleRecord(LifecycleRecordOptions{Kind: LifecycleActivationRequested, OccurredAt: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), ContractRef: ref, Revision: 1, ProposalRef: &ref, SigningPrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "illegal.jsonl")
	if err := AppendLifecycleRecord(path, record); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLifecycleJournal(path, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("signed activation without proposal was accepted")
	}
}
