package runpack

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/davidahmann/gait/core/fsx"
	"github.com/davidahmann/gait/core/jcs"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

const (
	sessionJournalSchemaID      = "gait.runpack.session_journal"
	sessionJournalSchemaVersion = "1.0.0"
	sessionEventSchemaID        = "gait.runpack.session_event"
	sessionEventSchemaVersion   = "1.0.0"
	sessionChainSchemaID        = "gait.runpack.session_chain"
	sessionChainSchemaVersion   = "1.0.0"
	sessionCheckpointSchemaID   = "gait.runpack.session_checkpoint"
	sessionCheckpointSchemaV1   = "1.0.0"
	sessionStateSchemaID        = "gait.runpack.session_state"
	sessionStateSchemaVersion   = "1.0.0"
	sessionLockTimeout          = 2 * time.Second
	sessionLockRetry            = 50 * time.Millisecond
	sessionLockStaleAfter       = 5 * time.Minute
)

var sessionProcessLockRegistry sync.Map

type SessionStartOptions struct {
	SessionID       string
	RunID           string
	ProducerVersion string
	Now             time.Time
}

type SessionAppendOptions struct {
	CreatedAt       time.Time
	ProducerVersion string
	IntentID        string
	ToolName        string
	IntentDigest    string
	PolicyDigest    string
	TraceID         string
	TracePath       string
	Verdict         string
	ReasonCodes     []string
	Violations      []string
}

type SessionStatus struct {
	SessionID       string    `json:"session_id"`
	RunID           string    `json:"run_id"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at"`
	EventCount      int       `json:"event_count"`
	CheckpointCount int       `json:"checkpoint_count"`
	LastSequence    int64     `json:"last_sequence"`
}

type SessionCheckpointOptions struct {
	Now             time.Time
	ProducerVersion string
	SignKey         ed25519.PrivateKey
}

type SessionCheckpointResult struct {
	Checkpoint schemarunpack.SessionCheckpoint `json:"checkpoint"`
	Chain      schemarunpack.SessionChain      `json:"chain"`
}

type SessionCompactionOptions struct {
	Now             time.Time
	ProducerVersion string
	OutputPath      string
	DryRun          bool
}

type SessionCompactionResult struct {
	JournalPath            string `json:"journal_path"`
	OutputPath             string `json:"output_path"`
	DryRun                 bool   `json:"dry_run"`
	Compacted              bool   `json:"compacted"`
	EventsBefore           int    `json:"events_before"`
	EventsAfter            int    `json:"events_after"`
	Checkpoints            int    `json:"checkpoints"`
	LastCheckpointSequence int64  `json:"last_checkpoint_sequence"`
	BytesBefore            int64  `json:"bytes_before"`
	BytesAfter             int64  `json:"bytes_after"`
}

type SessionChainVerifyOptions struct {
	RequireSignature bool
	PublicKey        ed25519.PublicKey
}

type SessionChainVerifyResult struct {
	SessionID          string   `json:"session_id"`
	RunID              string   `json:"run_id"`
	CheckpointsChecked int      `json:"checkpoints_checked"`
	LinkageErrors      []string `json:"linkage_errors,omitempty"`
	CheckpointErrors   []string `json:"checkpoint_errors,omitempty"`
}

type SessionChainDiffSummary struct {
	SessionIDLeft      string `json:"session_id_left"`
	SessionIDRight     string `json:"session_id_right"`
	CheckpointCountL   int    `json:"checkpoint_count_left"`
	CheckpointCountR   int    `json:"checkpoint_count_right"`
	ChangedIndexes     []int  `json:"changed_indexes,omitempty"`
	LeftOnlyIndexes    []int  `json:"left_only_indexes,omitempty"`
	RightOnlyIndexes   []int  `json:"right_only_indexes,omitempty"`
	ChangedCheckpoints bool   `json:"changed_checkpoints"`
}

type sessionJournalRecord struct {
	RecordType string                           `json:"record_type"`
	Header     *schemarunpack.SessionJournal    `json:"header,omitempty"`
	Event      *schemarunpack.SessionEvent      `json:"event,omitempty"`
	Checkpoint *schemarunpack.SessionCheckpoint `json:"checkpoint,omitempty"`
}

type sessionState struct {
	SchemaID               string    `json:"schema_id"`
	SchemaVersion          string    `json:"schema_version"`
	UpdatedAt              time.Time `json:"updated_at"`
	ProducerVersion        string    `json:"producer_version"`
	SessionID              string    `json:"session_id"`
	RunID                  string    `json:"run_id"`
	CreatedAt              time.Time `json:"created_at"`
	StartedAt              time.Time `json:"started_at"`
	EventCount             int       `json:"event_count"`
	LastSequence           int64     `json:"last_sequence"`
	CheckpointCount        int       `json:"checkpoint_count"`
	LastCheckpointIndex    int       `json:"last_checkpoint_index"`
	LastCheckpointSequence int64     `json:"last_checkpoint_sequence"`
	LastCheckpointDigest   string    `json:"last_checkpoint_digest,omitempty"`
	JournalSizeBytes       int64     `json:"journal_size_bytes"`
}

type sessionLockConfig struct {
	Profile    string
	Timeout    time.Duration
	Retry      time.Duration
	StaleAfter time.Duration
}

type SessionLockContentionError struct {
	LockPath string
	Waited   time.Duration
	Attempts int
	Timeout  time.Duration
	Retry    time.Duration
	Profile  string
}

func (err SessionLockContentionError) Error() string {
	return fmt.Sprintf(
		"session state contention: lock timeout (path=%s waited=%s attempts=%d timeout=%s retry=%s profile=%s)",
		err.LockPath,
		err.Waited.Truncate(time.Millisecond),
		err.Attempts,
		err.Timeout,
		err.Retry,
		err.Profile,
	)
}

// StartSession creates a new append-only session journal or returns the existing session status.
func StartSession(path string, opts SessionStartOptions) (SessionStatus, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return SessionStatus{}, err
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	runID := strings.TrimSpace(opts.RunID)
	if sessionID == "" {
		return SessionStatus{}, fmt.Errorf("session_id is required")
	}
	if runID == "" {
		return SessionStatus{}, fmt.Errorf("run_id is required")
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	producerVersion := strings.TrimSpace(opts.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}

	if err := withSessionLock(normalizedPath, func() error {
		if filepath.IsLocal(normalizedPath) {
			if _, statErr := os.Stat(normalizedPath); statErr == nil {
				journal, readErr := ReadSessionJournal(normalizedPath)
				if readErr != nil {
					return readErr
				}
				if journal.SessionID != sessionID || journal.RunID != runID {
					return fmt.Errorf("session journal already initialized with different session/run")
				}
				_, rebuildErr := loadOrRebuildSessionState(normalizedPath)
				return rebuildErr
			}
		} else if strings.HasPrefix(normalizedPath, string(filepath.Separator)) {
			if _, statErr := os.Stat(normalizedPath); statErr == nil {
				journal, readErr := ReadSessionJournal(normalizedPath)
				if readErr != nil {
					return readErr
				}
				if journal.SessionID != sessionID || journal.RunID != runID {
					return fmt.Errorf("session journal already initialized with different session/run")
				}
				_, rebuildErr := loadOrRebuildSessionState(normalizedPath)
				return rebuildErr
			}
		} else if volume := filepath.VolumeName(normalizedPath); volume != "" && strings.HasPrefix(normalizedPath, volume+string(filepath.Separator)) {
			if _, statErr := os.Stat(normalizedPath); statErr == nil {
				journal, readErr := ReadSessionJournal(normalizedPath)
				if readErr != nil {
					return readErr
				}
				if journal.SessionID != sessionID || journal.RunID != runID {
					return fmt.Errorf("session journal already initialized with different session/run")
				}
				_, rebuildErr := loadOrRebuildSessionState(normalizedPath)
				return rebuildErr
			}
		} else {
			return fmt.Errorf("session journal path must be local relative or absolute")
		}
		header := schemarunpack.SessionJournal{
			SchemaID:        sessionJournalSchemaID,
			SchemaVersion:   sessionJournalSchemaVersion,
			CreatedAt:       now,
			ProducerVersion: producerVersion,
			SessionID:       sessionID,
			RunID:           runID,
			StartedAt:       now,
			Events:          []schemarunpack.SessionEvent{},
		}
		record := sessionJournalRecord{
			RecordType: "header",
			Header:     &header,
		}
		if err := appendJournalRecord(normalizedPath, record); err != nil {
			return err
		}
		state := sessionState{
			SchemaID:               sessionStateSchemaID,
			SchemaVersion:          sessionStateSchemaVersion,
			UpdatedAt:              now,
			ProducerVersion:        producerVersion,
			SessionID:              sessionID,
			RunID:                  runID,
			CreatedAt:              now,
			StartedAt:              now,
			EventCount:             0,
			LastSequence:           0,
			CheckpointCount:        0,
			LastCheckpointIndex:    0,
			LastCheckpointSequence: 0,
			LastCheckpointDigest:   "",
		}
		if info, statErr := statLocalOrAbsolutePath(normalizedPath); statErr == nil {
			state.JournalSizeBytes = info.Size()
		}
		return writeSessionState(sessionStatePath(normalizedPath), state)
	}); err != nil {
		return SessionStatus{}, err
	}
	return GetSessionStatus(normalizedPath)
}

// AppendSessionEvent appends one deterministic event entry to the journal.
func AppendSessionEvent(path string, opts SessionAppendOptions) (schemarunpack.SessionEvent, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return schemarunpack.SessionEvent{}, err
	}
	lockConfig := resolveSessionLockConfig()
	var appended schemarunpack.SessionEvent
	err = withSessionLock(normalizedPath, func() error {
		state, stateErr := loadOrRebuildSessionState(normalizedPath)
		if stateErr != nil {
			return stateErr
		}
		now := opts.CreatedAt.UTC()
		if now.IsZero() {
			now = time.Now().UTC()
		}
		producerVersion := strings.TrimSpace(opts.ProducerVersion)
		if producerVersion == "" {
			producerVersion = state.ProducerVersion
		}
		if producerVersion == "" {
			producerVersion = "0.0.0-dev"
		}
		sequence := state.LastSequence + 1
		appended = schemarunpack.SessionEvent{
			SchemaID:        sessionEventSchemaID,
			SchemaVersion:   sessionEventSchemaVersion,
			CreatedAt:       now,
			ProducerVersion: producerVersion,
			SessionID:       state.SessionID,
			RunID:           state.RunID,
			Sequence:        sequence,
			IntentID:        strings.TrimSpace(opts.IntentID),
			ToolName:        strings.TrimSpace(opts.ToolName),
			IntentDigest:    strings.ToLower(strings.TrimSpace(opts.IntentDigest)),
			PolicyDigest:    strings.ToLower(strings.TrimSpace(opts.PolicyDigest)),
			TraceID:         strings.TrimSpace(opts.TraceID),
			TracePath:       strings.TrimSpace(opts.TracePath),
			Verdict:         strings.TrimSpace(opts.Verdict),
			ReasonCodes:     uniqueSortedStrings(opts.ReasonCodes),
			Violations:      uniqueSortedStrings(opts.Violations),
		}
		record := sessionJournalRecord{
			RecordType: "event",
			Event:      &appended,
		}
		// Swarm sessions optimize for sustained throughput and update state
		// incrementally, while checkpoints preserve durable progress.
		syncJournal := lockConfig.Profile != "swarm"
		if err := appendJournalRecordWithSync(normalizedPath, record, syncJournal); err != nil {
			return err
		}
		state.UpdatedAt = now
		state.ProducerVersion = producerVersion
		state.LastSequence = sequence
		state.EventCount++
		if info, statErr := statLocalOrAbsolutePath(normalizedPath); statErr == nil {
			state.JournalSizeBytes = info.Size()
		}
		return writeSessionState(sessionStatePath(normalizedPath), state)
	})
	if err != nil {
		return schemarunpack.SessionEvent{}, err
	}
	return appended, nil
}

func GetSessionStatus(path string) (SessionStatus, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return SessionStatus{}, err
	}
	var status SessionStatus
	if err := withSessionLock(normalizedPath, func() error {
		state, stateErr := loadOrRebuildSessionState(normalizedPath)
		if stateErr != nil {
			return stateErr
		}
		status = SessionStatus{
			SessionID:       state.SessionID,
			RunID:           state.RunID,
			CreatedAt:       state.CreatedAt.UTC(),
			StartedAt:       state.StartedAt.UTC(),
			EventCount:      state.EventCount,
			CheckpointCount: state.CheckpointCount,
			LastSequence:    state.LastSequence,
		}
		return nil
	}); err != nil {
		return SessionStatus{}, err
	}
	return status, nil
}

func EmitSessionCheckpoint(journalPath string, outRunpackPath string, opts SessionCheckpointOptions) (SessionCheckpointResult, error) {
	normalizedPath, err := normalizeOutputPath(journalPath)
	if err != nil {
		return SessionCheckpointResult{}, err
	}
	runpackPath, err := normalizeOutputPath(outRunpackPath)
	if err != nil {
		return SessionCheckpointResult{}, err
	}
	var result SessionCheckpointResult
	err = withSessionLock(normalizedPath, func() error {
		journal, readErr := ReadSessionJournal(normalizedPath)
		if readErr != nil {
			return readErr
		}
		lastCheckpointSeq := int64(0)
		prevCheckpointDigest := ""
		nextCheckpointIdx := len(journal.Checkpoints) + 1
		if len(journal.Checkpoints) > 0 {
			last := journal.Checkpoints[len(journal.Checkpoints)-1]
			lastCheckpointSeq = last.SequenceEnd
			prevCheckpointDigest = last.CheckpointDigest
		}
		newEvents := make([]schemarunpack.SessionEvent, 0)
		for _, event := range journal.Events {
			if event.Sequence > lastCheckpointSeq {
				newEvents = append(newEvents, event)
			}
		}
		if len(newEvents) == 0 {
			return fmt.Errorf("no new session events available for checkpoint")
		}
		sequenceStart := newEvents[0].Sequence
		sequenceEnd := newEvents[len(newEvents)-1].Sequence
		createdAt := opts.Now.UTC()
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		producerVersion := strings.TrimSpace(opts.ProducerVersion)
		if producerVersion == "" {
			producerVersion = journal.ProducerVersion
		}
		if producerVersion == "" {
			producerVersion = "0.0.0-dev"
		}
		checkpointRunID := fmt.Sprintf("%s_cp_%04d", journal.RunID, nextCheckpointIdx)

		intents := make([]schemarunpack.IntentRecord, 0, len(newEvents))
		results := make([]schemarunpack.ResultRecord, 0, len(newEvents))
		timeline := make([]schemarunpack.TimelineEvt, 0, len(newEvents)*2)
		for i, event := range newEvents {
			intentID := strings.TrimSpace(event.IntentID)
			if intentID == "" {
				intentID = fmt.Sprintf("intent_%d", i+1)
			}
			intentArgs := map[string]any{
				"session_id": journal.SessionID,
				"sequence":   event.Sequence,
				"trace_id":   event.TraceID,
				"trace_path": event.TracePath,
			}
			intents = append(intents, schemarunpack.IntentRecord{
				SchemaID:        "gait.runpack.intent",
				SchemaVersion:   "1.0.0",
				CreatedAt:       event.CreatedAt.UTC(),
				ProducerVersion: producerVersion,
				RunID:           checkpointRunID,
				IntentID:        intentID,
				ToolName:        event.ToolName,
				ArgsDigest:      event.IntentDigest,
				Args:            intentArgs,
			})
			resultPayload := map[string]any{
				"verdict":      event.Verdict,
				"reason_codes": event.ReasonCodes,
				"violations":   event.Violations,
				"trace_id":     event.TraceID,
				"trace_path":   event.TracePath,
			}
			resultDigest, digestErr := digestObject(resultPayload)
			if digestErr != nil {
				return digestErr
			}
			status := "ok"
			if event.Verdict == "block" || event.Verdict == "require_approval" {
				status = "error"
			}
			results = append(results, schemarunpack.ResultRecord{
				SchemaID:        "gait.runpack.result",
				SchemaVersion:   "1.0.0",
				CreatedAt:       event.CreatedAt.UTC(),
				ProducerVersion: producerVersion,
				RunID:           checkpointRunID,
				IntentID:        intentID,
				Status:          status,
				ResultDigest:    resultDigest,
				Result:          resultPayload,
			})
			timeline = append(timeline, schemarunpack.TimelineEvt{
				Event: "session_event",
				TS:    event.CreatedAt.UTC(),
				Ref:   event.TraceID,
			})
		}
		timeline = append(timeline, schemarunpack.TimelineEvt{
			Event: "session_checkpoint_emitted",
			TS:    createdAt,
			Ref:   fmt.Sprintf("checkpoint:%d", nextCheckpointIdx),
		})

		recordRes, writeErr := WriteRunpack(runpackPath, RecordOptions{
			Run: schemarunpack.Run{
				SchemaID:        "gait.runpack.run",
				SchemaVersion:   "1.0.0",
				CreatedAt:       createdAt,
				ProducerVersion: producerVersion,
				RunID:           checkpointRunID,
				Timeline:        timeline,
			},
			Intents:     intents,
			Results:     results,
			Refs:        schemarunpack.Refs{RunID: checkpointRunID},
			CaptureMode: "reference",
			SignKey:     opts.SignKey,
		})
		if writeErr != nil {
			return writeErr
		}
		checkpointDigest := computeCheckpointDigest(recordRes.Manifest.ManifestDigest, prevCheckpointDigest, nextCheckpointIdx, sequenceStart, sequenceEnd)
		checkpoint := schemarunpack.SessionCheckpoint{
			SchemaID:             sessionCheckpointSchemaID,
			SchemaVersion:        sessionCheckpointSchemaV1,
			CreatedAt:            createdAt,
			ProducerVersion:      producerVersion,
			SessionID:            journal.SessionID,
			RunID:                journal.RunID,
			CheckpointIndex:      nextCheckpointIdx,
			SequenceStart:        sequenceStart,
			SequenceEnd:          sequenceEnd,
			RunpackPath:          runpackPath,
			ManifestDigest:       recordRes.Manifest.ManifestDigest,
			PrevCheckpointDigest: prevCheckpointDigest,
			CheckpointDigest:     checkpointDigest,
		}
		appendErr := appendJournalRecord(normalizedPath, sessionJournalRecord{
			RecordType: "checkpoint",
			Checkpoint: &checkpoint,
		})
		if appendErr != nil {
			return appendErr
		}

		updatedJournal := journal
		updatedJournal.Checkpoints = append(updatedJournal.Checkpoints, checkpoint)
		state, stateErr := loadOrRebuildSessionState(normalizedPath)
		if stateErr != nil {
			return stateErr
		}
		state.UpdatedAt = createdAt
		state.ProducerVersion = producerVersion
		state.CheckpointCount = checkpoint.CheckpointIndex
		state.LastCheckpointIndex = checkpoint.CheckpointIndex
		state.LastCheckpointSequence = checkpoint.SequenceEnd
		state.LastCheckpointDigest = checkpoint.CheckpointDigest
		if info, statErr := statLocalOrAbsolutePath(normalizedPath); statErr == nil {
			state.JournalSizeBytes = info.Size()
		}
		if writeErr := writeSessionState(sessionStatePath(normalizedPath), state); writeErr != nil {
			return writeErr
		}
		result = SessionCheckpointResult{
			Checkpoint: checkpoint,
			Chain:      journalToSessionChain(updatedJournal),
		}
		return nil
	})
	if err != nil {
		return SessionCheckpointResult{}, err
	}
	return result, nil
}

func ReadSessionJournal(path string) (schemarunpack.SessionJournal, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return schemarunpack.SessionJournal{}, err
	}
	var file *os.File
	if filepath.IsLocal(normalizedPath) {
		// #nosec G304 -- journal path is an explicit local path.
		file, err = os.Open(normalizedPath)
	} else if strings.HasPrefix(normalizedPath, string(filepath.Separator)) {
		// #nosec G304 -- journal path is an explicit local path.
		file, err = os.Open(normalizedPath)
	} else if volume := filepath.VolumeName(normalizedPath); volume != "" && strings.HasPrefix(normalizedPath, volume+string(filepath.Separator)) {
		// #nosec G304 -- journal path is an explicit local path.
		file, err = os.Open(normalizedPath)
	} else {
		return schemarunpack.SessionJournal{}, fmt.Errorf("session journal path must be local relative or absolute")
	}
	if err != nil {
		return schemarunpack.SessionJournal{}, fmt.Errorf("open session journal: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 128*1024), 8*1024*1024)
	lineNo := 0
	var journal schemarunpack.SessionJournal
	var haveHeader bool
	expectedSequence := int64(1)
	lastCheckpointIdx := 0
	lastCheckpointDigest := ""
	lastCheckpointSequence := int64(0)

	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var record sessionJournalRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			return schemarunpack.SessionJournal{}, fmt.Errorf("session journal parse line %d: %w", lineNo, err)
		}
		switch record.RecordType {
		case "header":
			if record.Header == nil {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal line %d missing header payload", lineNo)
			}
			if haveHeader {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal contains duplicate header")
			}
			journal = *record.Header
			if journal.SchemaID == "" {
				journal.SchemaID = sessionJournalSchemaID
			}
			if journal.SchemaVersion == "" {
				journal.SchemaVersion = sessionJournalSchemaVersion
			}
			if journal.SessionID == "" || journal.RunID == "" {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal header missing session_id or run_id")
			}
			journal.Events = []schemarunpack.SessionEvent{}
			journal.Checkpoints = []schemarunpack.SessionCheckpoint{}
			haveHeader = true
		case "event":
			if !haveHeader {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal event encountered before header")
			}
			if record.Event == nil {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal line %d missing event payload", lineNo)
			}
			event := *record.Event
			if event.Sequence != expectedSequence {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session event sequence mismatch line %d: got %d want %d", lineNo, event.Sequence, expectedSequence)
			}
			if event.SessionID != journal.SessionID || event.RunID != journal.RunID {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session event identity mismatch at line %d", lineNo)
			}
			journal.Events = append(journal.Events, event)
			expectedSequence++
		case "checkpoint":
			if !haveHeader {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal checkpoint encountered before header")
			}
			if record.Checkpoint == nil {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session journal line %d missing checkpoint payload", lineNo)
			}
			checkpoint := *record.Checkpoint
			if checkpoint.SessionID != journal.SessionID || checkpoint.RunID != journal.RunID {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint identity mismatch at line %d", lineNo)
			}
			if checkpoint.CheckpointIndex != lastCheckpointIdx+1 {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint index mismatch at line %d", lineNo)
			}
			if checkpoint.SequenceStart < 1 || checkpoint.SequenceEnd < checkpoint.SequenceStart {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint sequence range invalid at line %d", lineNo)
			}
			if checkpoint.SequenceStart <= lastCheckpointSequence {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint sequence overlap at line %d", lineNo)
			}
			if checkpoint.CheckpointIndex > 1 && checkpoint.PrevCheckpointDigest != lastCheckpointDigest {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint digest linkage mismatch at line %d", lineNo)
			}
			expectedDigest := computeCheckpointDigest(
				checkpoint.ManifestDigest,
				checkpoint.PrevCheckpointDigest,
				checkpoint.CheckpointIndex,
				checkpoint.SequenceStart,
				checkpoint.SequenceEnd,
			)
			if checkpoint.CheckpointDigest != expectedDigest {
				return schemarunpack.SessionJournal{}, fmt.Errorf("session checkpoint digest invalid at line %d", lineNo)
			}
			journal.Checkpoints = append(journal.Checkpoints, checkpoint)
			lastCheckpointIdx = checkpoint.CheckpointIndex
			lastCheckpointDigest = checkpoint.CheckpointDigest
			lastCheckpointSequence = checkpoint.SequenceEnd
			if expectedSequence <= checkpoint.SequenceEnd {
				expectedSequence = checkpoint.SequenceEnd + 1
			}
		default:
			return schemarunpack.SessionJournal{}, fmt.Errorf("session journal line %d has unsupported record_type %q", lineNo, record.RecordType)
		}
	}
	if err := scanner.Err(); err != nil {
		return schemarunpack.SessionJournal{}, fmt.Errorf("read session journal: %w", err)
	}
	if !haveHeader {
		return schemarunpack.SessionJournal{}, fmt.Errorf("session journal missing header")
	}
	return journal, nil
}

func WriteSessionChain(path string, chain schemarunpack.SessionChain) error {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return err
	}
	normalized := chain
	if normalized.SchemaID == "" {
		normalized.SchemaID = sessionChainSchemaID
	}
	if normalized.SchemaVersion == "" {
		normalized.SchemaVersion = sessionChainSchemaVersion
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now().UTC()
	}
	if normalized.ProducerVersion == "" {
		normalized.ProducerVersion = "0.0.0-dev"
	}
	encoded, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session chain: %w", err)
	}
	encoded = append(encoded, '\n')
	return fsx.WriteFileAtomic(normalizedPath, encoded, 0o600)
}

func ReadSessionChain(path string) (schemarunpack.SessionChain, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return schemarunpack.SessionChain{}, err
	}
	// #nosec G304 -- chain path is explicit local user input.
	content, err := readFileLocalOrAbsolutePath(normalizedPath)
	if err != nil {
		return schemarunpack.SessionChain{}, fmt.Errorf("read session chain: %w", err)
	}
	var chain schemarunpack.SessionChain
	if err := json.Unmarshal(content, &chain); err != nil {
		return schemarunpack.SessionChain{}, fmt.Errorf("parse session chain: %w", err)
	}
	if chain.SchemaID == "" {
		chain.SchemaID = sessionChainSchemaID
	}
	if chain.SchemaVersion == "" {
		chain.SchemaVersion = sessionChainSchemaVersion
	}
	if chain.SchemaID != sessionChainSchemaID || chain.SchemaVersion != sessionChainSchemaVersion {
		return schemarunpack.SessionChain{}, fmt.Errorf("unsupported session chain schema")
	}
	if chain.SessionID == "" || chain.RunID == "" {
		return schemarunpack.SessionChain{}, fmt.Errorf("session chain missing session_id/run_id")
	}
	if len(chain.Checkpoints) == 0 {
		return schemarunpack.SessionChain{}, fmt.Errorf("session chain has no checkpoints")
	}
	return chain, nil
}

func VerifySessionChain(path string, opts SessionChainVerifyOptions) (SessionChainVerifyResult, error) {
	chain, err := ReadSessionChain(path)
	if err != nil {
		return SessionChainVerifyResult{}, err
	}
	linkageErrors := []string{}
	checkpointErrors := []string{}
	lastDigest := ""
	lastIndex := 0
	lastSequenceEnd := int64(0)
	for idx, checkpoint := range chain.Checkpoints {
		if checkpoint.CheckpointIndex != lastIndex+1 {
			linkageErrors = append(linkageErrors, fmt.Sprintf("checkpoint index mismatch at position %d", idx))
		}
		if checkpoint.CheckpointIndex > 1 && checkpoint.PrevCheckpointDigest != lastDigest {
			linkageErrors = append(linkageErrors, fmt.Sprintf("prev_checkpoint_digest mismatch at checkpoint %d", checkpoint.CheckpointIndex))
		}
		if checkpoint.SequenceStart <= lastSequenceEnd {
			linkageErrors = append(linkageErrors, fmt.Sprintf("sequence overlap at checkpoint %d", checkpoint.CheckpointIndex))
		}
		expectedDigest := computeCheckpointDigest(
			checkpoint.ManifestDigest,
			checkpoint.PrevCheckpointDigest,
			checkpoint.CheckpointIndex,
			checkpoint.SequenceStart,
			checkpoint.SequenceEnd,
		)
		if checkpoint.CheckpointDigest != expectedDigest {
			linkageErrors = append(linkageErrors, fmt.Sprintf("checkpoint_digest mismatch at checkpoint %d", checkpoint.CheckpointIndex))
		}
		verifyRes, verifyErr := VerifyZip(checkpoint.RunpackPath, VerifyOptions{
			RequireSignature: opts.RequireSignature,
			PublicKey:        opts.PublicKey,
		})
		if verifyErr != nil {
			checkpointErrors = append(checkpointErrors, fmt.Sprintf("checkpoint %d verify error: %v", checkpoint.CheckpointIndex, verifyErr))
		} else {
			if verifyRes.ManifestDigest != checkpoint.ManifestDigest {
				checkpointErrors = append(checkpointErrors, fmt.Sprintf("checkpoint %d manifest digest mismatch", checkpoint.CheckpointIndex))
			}
			if len(verifyRes.MissingFiles) > 0 || len(verifyRes.HashMismatches) > 0 {
				checkpointErrors = append(checkpointErrors, fmt.Sprintf("checkpoint %d runpack integrity failure", checkpoint.CheckpointIndex))
			}
			if opts.RequireSignature && verifyRes.SignatureStatus != "verified" {
				checkpointErrors = append(checkpointErrors, fmt.Sprintf("checkpoint %d signature verification failed", checkpoint.CheckpointIndex))
			}
		}
		lastDigest = checkpoint.CheckpointDigest
		lastIndex = checkpoint.CheckpointIndex
		lastSequenceEnd = checkpoint.SequenceEnd
	}
	return SessionChainVerifyResult{
		SessionID:          chain.SessionID,
		RunID:              chain.RunID,
		CheckpointsChecked: len(chain.Checkpoints),
		LinkageErrors:      linkageErrors,
		CheckpointErrors:   checkpointErrors,
	}, nil
}

func DiffSessionChains(left, right schemarunpack.SessionChain) SessionChainDiffSummary {
	leftByIndex := map[int]schemarunpack.SessionCheckpoint{}
	rightByIndex := map[int]schemarunpack.SessionCheckpoint{}
	leftIndexes := make([]int, 0, len(left.Checkpoints))
	rightIndexes := make([]int, 0, len(right.Checkpoints))
	for _, checkpoint := range left.Checkpoints {
		leftByIndex[checkpoint.CheckpointIndex] = checkpoint
		leftIndexes = append(leftIndexes, checkpoint.CheckpointIndex)
	}
	for _, checkpoint := range right.Checkpoints {
		rightByIndex[checkpoint.CheckpointIndex] = checkpoint
		rightIndexes = append(rightIndexes, checkpoint.CheckpointIndex)
	}
	sort.Ints(leftIndexes)
	sort.Ints(rightIndexes)

	changed := make([]int, 0)
	leftOnly := make([]int, 0)
	rightOnly := make([]int, 0)
	seen := map[int]struct{}{}
	for _, idx := range leftIndexes {
		seen[idx] = struct{}{}
		rightCheckpoint, ok := rightByIndex[idx]
		if !ok {
			leftOnly = append(leftOnly, idx)
			continue
		}
		leftCheckpoint := leftByIndex[idx]
		if leftCheckpoint.CheckpointDigest != rightCheckpoint.CheckpointDigest {
			changed = append(changed, idx)
		}
	}
	for _, idx := range rightIndexes {
		if _, ok := seen[idx]; ok {
			continue
		}
		rightOnly = append(rightOnly, idx)
	}
	return SessionChainDiffSummary{
		SessionIDLeft:      left.SessionID,
		SessionIDRight:     right.SessionID,
		CheckpointCountL:   len(left.Checkpoints),
		CheckpointCountR:   len(right.Checkpoints),
		ChangedIndexes:     changed,
		LeftOnlyIndexes:    leftOnly,
		RightOnlyIndexes:   rightOnly,
		ChangedCheckpoints: len(changed) > 0 || len(leftOnly) > 0 || len(rightOnly) > 0,
	}
}

func journalToSessionChain(journal schemarunpack.SessionJournal) schemarunpack.SessionChain {
	createdAt := journal.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = journal.StartedAt.UTC()
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return schemarunpack.SessionChain{
		SchemaID:        sessionChainSchemaID,
		SchemaVersion:   sessionChainSchemaVersion,
		CreatedAt:       createdAt,
		ProducerVersion: journal.ProducerVersion,
		SessionID:       journal.SessionID,
		RunID:           journal.RunID,
		Checkpoints:     append([]schemarunpack.SessionCheckpoint{}, journal.Checkpoints...),
	}
}

func appendJournalRecord(path string, record sessionJournalRecord) error {
	return appendJournalRecordWithSync(path, record, true)
}

func appendJournalRecordWithSync(path string, record sessionJournalRecord, syncWrites bool) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session journal record: %w", err)
	}
	encoded = append(encoded, '\n')
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if filepath.IsLocal(dir) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create session journal directory: %w", err)
			}
		} else if strings.HasPrefix(dir, string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create session journal directory: %w", err)
			}
		} else if volume := filepath.VolumeName(dir); volume != "" && strings.HasPrefix(dir, volume+string(filepath.Separator)) {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create session journal directory: %w", err)
			}
		} else {
			return fmt.Errorf("session journal directory must be local relative or absolute")
		}
	}
	var file *os.File
	if filepath.IsLocal(path) {
		// #nosec G304 -- session journal path is explicit local user input.
		file, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	} else if strings.HasPrefix(path, string(filepath.Separator)) {
		// #nosec G304 -- session journal path is explicit local user input.
		file, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	} else if volume := filepath.VolumeName(path); volume != "" && strings.HasPrefix(path, volume+string(filepath.Separator)) {
		// #nosec G304 -- session journal path is explicit local user input.
		file, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	} else {
		return fmt.Errorf("session journal path must be local relative or absolute")
	}
	if err != nil {
		return fmt.Errorf("open session journal: %w", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(encoded); err != nil {
		return fmt.Errorf("append session journal: %w", err)
	}
	if syncWrites {
		if err := file.Sync(); err != nil {
			return fmt.Errorf("sync session journal: %w", err)
		}
	}
	return nil
}

func sessionStatePath(journalPath string) string {
	return journalPath + ".state.json"
}

func readSessionState(path string) (sessionState, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return sessionState{}, err
	}
	// #nosec G304 -- session state path is derived from validated local journal path.
	content, err := readFileLocalOrAbsolutePath(normalizedPath)
	if err != nil {
		return sessionState{}, fmt.Errorf("read session state: %w", err)
	}
	var state sessionState
	if err := json.Unmarshal(content, &state); err != nil {
		return sessionState{}, fmt.Errorf("parse session state: %w", err)
	}
	if strings.TrimSpace(state.SchemaID) != sessionStateSchemaID {
		return sessionState{}, fmt.Errorf("invalid session state schema_id")
	}
	if strings.TrimSpace(state.SchemaVersion) != sessionStateSchemaVersion {
		return sessionState{}, fmt.Errorf("invalid session state schema_version")
	}
	if strings.TrimSpace(state.SessionID) == "" || strings.TrimSpace(state.RunID) == "" {
		return sessionState{}, fmt.Errorf("session state missing session_id/run_id")
	}
	if state.EventCount < 0 || state.CheckpointCount < 0 {
		return sessionState{}, fmt.Errorf("session state counters must be non-negative")
	}
	if state.LastSequence < 0 || state.LastCheckpointSequence < 0 || state.JournalSizeBytes < 0 {
		return sessionState{}, fmt.Errorf("session state numeric fields must be non-negative")
	}
	state.UpdatedAt = state.UpdatedAt.UTC()
	state.CreatedAt = state.CreatedAt.UTC()
	state.StartedAt = state.StartedAt.UTC()
	state.ProducerVersion = strings.TrimSpace(state.ProducerVersion)
	if state.ProducerVersion == "" {
		state.ProducerVersion = "0.0.0-dev"
	}
	return state, nil
}

func writeSessionState(path string, state sessionState) error {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return err
	}
	canonical := state
	canonical.SchemaID = sessionStateSchemaID
	canonical.SchemaVersion = sessionStateSchemaVersion
	canonical.UpdatedAt = canonical.UpdatedAt.UTC()
	if canonical.UpdatedAt.IsZero() {
		canonical.UpdatedAt = time.Now().UTC()
	}
	canonical.CreatedAt = canonical.CreatedAt.UTC()
	canonical.StartedAt = canonical.StartedAt.UTC()
	canonical.ProducerVersion = strings.TrimSpace(canonical.ProducerVersion)
	if canonical.ProducerVersion == "" {
		canonical.ProducerVersion = "0.0.0-dev"
	}
	canonical.SessionID = strings.TrimSpace(canonical.SessionID)
	canonical.RunID = strings.TrimSpace(canonical.RunID)
	if canonical.SessionID == "" || canonical.RunID == "" {
		return fmt.Errorf("session state missing session_id/run_id")
	}
	if canonical.EventCount < 0 || canonical.CheckpointCount < 0 || canonical.LastSequence < 0 || canonical.LastCheckpointSequence < 0 || canonical.JournalSizeBytes < 0 {
		return fmt.Errorf("session state contains negative counters")
	}
	raw, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}
	raw = append(raw, '\n')
	return fsx.WriteFileAtomic(normalizedPath, raw, 0o600)
}

func buildSessionStateFromJournal(path string, journal schemarunpack.SessionJournal, now time.Time) (sessionState, error) {
	updatedAt := now.UTC()
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	lastSequence := int64(0)
	if len(journal.Events) > 0 {
		lastSequence = journal.Events[len(journal.Events)-1].Sequence
	}
	lastCheckpointIndex := 0
	lastCheckpointSequence := int64(0)
	lastCheckpointDigest := ""
	if len(journal.Checkpoints) > 0 {
		last := journal.Checkpoints[len(journal.Checkpoints)-1]
		lastCheckpointIndex = last.CheckpointIndex
		lastCheckpointSequence = last.SequenceEnd
		lastCheckpointDigest = strings.TrimSpace(last.CheckpointDigest)
	}
	sizeBytes := int64(0)
	if info, err := statLocalOrAbsolutePath(path); err == nil {
		sizeBytes = info.Size()
	}
	producerVersion := strings.TrimSpace(journal.ProducerVersion)
	if producerVersion == "" {
		producerVersion = "0.0.0-dev"
	}
	createdAt := journal.CreatedAt.UTC()
	startedAt := journal.StartedAt.UTC()
	if createdAt.IsZero() {
		createdAt = startedAt
	}
	if createdAt.IsZero() {
		createdAt = updatedAt
	}
	if startedAt.IsZero() {
		startedAt = createdAt
	}
	return sessionState{
		SchemaID:               sessionStateSchemaID,
		SchemaVersion:          sessionStateSchemaVersion,
		UpdatedAt:              updatedAt,
		ProducerVersion:        producerVersion,
		SessionID:              journal.SessionID,
		RunID:                  journal.RunID,
		CreatedAt:              createdAt,
		StartedAt:              startedAt,
		EventCount:             len(journal.Events),
		LastSequence:           lastSequence,
		CheckpointCount:        len(journal.Checkpoints),
		LastCheckpointIndex:    lastCheckpointIndex,
		LastCheckpointSequence: lastCheckpointSequence,
		LastCheckpointDigest:   lastCheckpointDigest,
		JournalSizeBytes:       sizeBytes,
	}, nil
}

func loadOrRebuildSessionState(journalPath string) (sessionState, error) {
	statePath := sessionStatePath(journalPath)
	currentSize := int64(-1)
	if info, err := statLocalOrAbsolutePath(journalPath); err == nil {
		currentSize = info.Size()
	}
	state, err := readSessionState(statePath)
	if err == nil {
		if currentSize >= 0 && state.JournalSizeBytes == currentSize {
			return state, nil
		}
	}
	journal, journalErr := ReadSessionJournal(journalPath)
	if journalErr != nil {
		return sessionState{}, journalErr
	}
	rebuilt, buildErr := buildSessionStateFromJournal(journalPath, journal, time.Now().UTC())
	if buildErr != nil {
		return sessionState{}, buildErr
	}
	if writeErr := writeSessionState(statePath, rebuilt); writeErr != nil {
		return sessionState{}, writeErr
	}
	return rebuilt, nil
}

func digestObject(value map[string]any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return jcs.DigestJCS(raw)
}

func computeCheckpointDigest(manifestDigest string, prevCheckpointDigest string, checkpointIndex int, sequenceStart int64, sequenceEnd int64) string {
	raw := fmt.Sprintf("%s:%s:%d:%d:%d", manifestDigest, prevCheckpointDigest, checkpointIndex, sequenceStart, sequenceEnd)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func statLocalOrAbsolutePath(path string) (os.FileInfo, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return nil, err
	}
	if filepath.IsLocal(normalizedPath) {
		return os.Stat(normalizedPath)
	}
	if strings.HasPrefix(normalizedPath, string(filepath.Separator)) {
		return os.Stat(normalizedPath)
	}
	if volume := filepath.VolumeName(normalizedPath); volume != "" && strings.HasPrefix(normalizedPath, volume+string(filepath.Separator)) {
		return os.Stat(normalizedPath)
	}
	return nil, fmt.Errorf("path must be local relative or absolute")
}

func readFileLocalOrAbsolutePath(path string) ([]byte, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return nil, err
	}
	if filepath.IsLocal(normalizedPath) {
		// #nosec G304 -- path is normalized and constrained to local relative or absolute.
		return os.ReadFile(normalizedPath)
	}
	if strings.HasPrefix(normalizedPath, string(filepath.Separator)) {
		// #nosec G304 -- path is normalized and constrained to local relative or absolute.
		return os.ReadFile(normalizedPath)
	}
	if volume := filepath.VolumeName(normalizedPath); volume != "" && strings.HasPrefix(normalizedPath, volume+string(filepath.Separator)) {
		// #nosec G304 -- path is normalized and constrained to local relative or absolute.
		return os.ReadFile(normalizedPath)
	}
	return nil, fmt.Errorf("path must be local relative or absolute")
}

func withSessionLock(journalPath string, fn func() error) error {
	lockConfig := resolveSessionLockConfig()
	lockPath := journalPath + ".lock"
	lockDir := filepath.Dir(lockPath)
	if lockDir != "." && lockDir != "" {
		if filepath.IsLocal(lockDir) {
			if err := os.MkdirAll(lockDir, 0o750); err != nil {
				return fmt.Errorf("prepare session lock directory: %w", err)
			}
		} else if strings.HasPrefix(lockDir, string(filepath.Separator)) {
			if err := os.MkdirAll(lockDir, 0o750); err != nil {
				return fmt.Errorf("prepare session lock directory: %w", err)
			}
		} else if volume := filepath.VolumeName(lockDir); volume != "" && strings.HasPrefix(lockDir, volume+string(filepath.Separator)) {
			if err := os.MkdirAll(lockDir, 0o750); err != nil {
				return fmt.Errorf("prepare session lock directory: %w", err)
			}
		} else {
			return fmt.Errorf("session lock directory must be local relative or absolute")
		}
	}

	processLock := getSessionProcessLock(lockPath)
	processLock.Lock()
	defer processLock.Unlock()

	start := time.Now()
	attempts := 0
	for {
		attempts++
		var lockFile *os.File
		var err error
		if filepath.IsLocal(lockPath) {
			// #nosec G304 -- lock path is derived from normalized local journal path.
			lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		} else if strings.HasPrefix(lockPath, string(filepath.Separator)) {
			// #nosec G304 -- lock path is derived from normalized local journal path.
			lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		} else if volume := filepath.VolumeName(lockPath); volume != "" && strings.HasPrefix(lockPath, volume+string(filepath.Separator)) {
			// #nosec G304 -- lock path is derived from normalized local journal path.
			lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		} else {
			return fmt.Errorf("session lock path must be local relative or absolute")
		}
		if err == nil {
			now := time.Now().UTC()
			metadata := map[string]any{
				"schema_id":      "gait.runpack.session_lock",
				"schema_version": "1.0.0",
				"pid":            os.Getpid(),
				"created_at":     now.Format(time.RFC3339),
			}
			if encoded, marshalErr := json.Marshal(metadata); marshalErr == nil {
				_, _ = lockFile.Write(append(encoded, '\n'))
			}
			_ = lockFile.Close()
			defer func() { _ = os.Remove(lockPath) }()
			return fn()
		}
		if !isSessionLockContention(err, lockPath) && !isWindowsAccessDeniedLockError(err) {
			return fmt.Errorf("acquire session lock: %w", err)
		}
		if shouldRecoverStaleSessionLockWithThreshold(lockPath, time.Now().UTC(), lockConfig.StaleAfter) {
			if filepath.IsLocal(lockPath) {
				_ = os.Remove(lockPath)
			} else if strings.HasPrefix(lockPath, string(filepath.Separator)) {
				_ = os.Remove(lockPath)
			} else if volume := filepath.VolumeName(lockPath); volume != "" && strings.HasPrefix(lockPath, volume+string(filepath.Separator)) {
				_ = os.Remove(lockPath)
			} else {
				return fmt.Errorf("session lock path must be local relative or absolute")
			}
			continue
		}
		if time.Since(start) >= lockConfig.Timeout {
			return SessionLockContentionError{
				LockPath: lockPath,
				Waited:   time.Since(start),
				Attempts: attempts,
				Timeout:  lockConfig.Timeout,
				Retry:    lockConfig.Retry,
				Profile:  lockConfig.Profile,
			}
		}
		time.Sleep(lockConfig.Retry)
	}
}

func getSessionProcessLock(lockPath string) *sync.Mutex {
	if existing, ok := sessionProcessLockRegistry.Load(lockPath); ok {
		if lock, ok := existing.(*sync.Mutex); ok {
			return lock
		}
	}
	lock := &sync.Mutex{}
	actual, _ := sessionProcessLockRegistry.LoadOrStore(lockPath, lock)
	typed, ok := actual.(*sync.Mutex)
	if !ok {
		return lock
	}
	return typed
}

func isSessionLockContention(acquireErr error, lockPath string) bool {
	if os.IsExist(acquireErr) {
		return true
	}
	_, statErr := os.Stat(lockPath)
	return statErr == nil || os.IsPermission(statErr)
}

func isWindowsAccessDeniedLockError(acquireErr error) bool {
	if runtime.GOOS != "windows" || acquireErr == nil {
		return false
	}
	return strings.Contains(strings.ToLower(acquireErr.Error()), "access is denied")
}

func shouldRecoverStaleSessionLock(lockPath string, now time.Time) bool {
	return shouldRecoverStaleSessionLockWithThreshold(lockPath, now, sessionLockStaleAfter)
}

func shouldRecoverStaleSessionLockWithThreshold(lockPath string, now time.Time, staleAfter time.Duration) bool {
	var (
		content []byte
		err     error
	)
	if filepath.IsLocal(lockPath) {
		// #nosec G304 -- lock path is derived from validated journal path.
		content, err = os.ReadFile(lockPath)
	} else if strings.HasPrefix(lockPath, string(filepath.Separator)) {
		// #nosec G304 -- lock path is derived from validated journal path.
		content, err = os.ReadFile(lockPath)
	} else if volume := filepath.VolumeName(lockPath); volume != "" && strings.HasPrefix(lockPath, volume+string(filepath.Separator)) {
		// #nosec G304 -- lock path is derived from validated journal path.
		content, err = os.ReadFile(lockPath)
	} else {
		return false
	}
	if err != nil {
		return false
	}
	var metadata struct {
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return false
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(metadata.CreatedAt))
	if err != nil {
		return false
	}
	return now.Sub(createdAt) > staleAfter
}

func resolveSessionLockConfig() sessionLockConfig {
	profile := strings.ToLower(strings.TrimSpace(os.Getenv("GAIT_SESSION_LOCK_PROFILE")))
	timeout := sessionLockTimeout
	retry := sessionLockRetry
	staleAfter := sessionLockStaleAfter
	switch profile {
	case "", "standard":
		profile = "standard"
	case "swarm":
		timeout = 10 * time.Second
		retry = 20 * time.Millisecond
		staleAfter = 10 * time.Minute
	default:
		profile = "standard"
	}
	timeout = parseSessionLockDuration("GAIT_SESSION_LOCK_TIMEOUT", timeout)
	retry = parseSessionLockDuration("GAIT_SESSION_LOCK_RETRY", retry)
	staleAfter = parseSessionLockDuration("GAIT_SESSION_LOCK_STALE_AFTER", staleAfter)
	if timeout <= 0 {
		timeout = sessionLockTimeout
	}
	if retry <= 0 {
		retry = sessionLockRetry
	}
	if staleAfter <= 0 {
		staleAfter = sessionLockStaleAfter
	}
	if retry > timeout {
		retry = timeout
	}
	return sessionLockConfig{
		Profile:    profile,
		Timeout:    timeout,
		Retry:      retry,
		StaleAfter: staleAfter,
	}
}

func parseSessionLockDuration(envName string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func sessionChainFromJournalPath(journalPath string) string {
	base := filepath.Base(journalPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		base = "session"
	}
	return filepath.Join(filepath.Dir(journalPath), base+"_chain.json")
}

func SessionCheckpointAndWriteChain(journalPath string, runpackOut string, opts SessionCheckpointOptions) (SessionCheckpointResult, string, error) {
	result, err := EmitSessionCheckpoint(journalPath, runpackOut, opts)
	if err != nil {
		return SessionCheckpointResult{}, "", err
	}
	chainPath := sessionChainFromJournalPath(journalPath)
	if err := WriteSessionChain(chainPath, result.Chain); err != nil {
		return SessionCheckpointResult{}, "", err
	}
	return result, chainPath, nil
}

func CompactSessionJournal(path string, opts SessionCompactionOptions) (SessionCompactionResult, error) {
	normalizedPath, err := normalizeOutputPath(path)
	if err != nil {
		return SessionCompactionResult{}, err
	}
	outputPath := strings.TrimSpace(opts.OutputPath)
	if outputPath == "" {
		outputPath = normalizedPath
	}
	normalizedOutputPath, err := normalizeOutputPath(outputPath)
	if err != nil {
		return SessionCompactionResult{}, err
	}
	var result SessionCompactionResult
	lockErr := withSessionLock(normalizedPath, func() error {
		journal, readErr := ReadSessionJournal(normalizedPath)
		if readErr != nil {
			return readErr
		}
		now := opts.Now.UTC()
		if now.IsZero() {
			now = time.Now().UTC()
		}
		lastCheckpointSequence := int64(0)
		if len(journal.Checkpoints) > 0 {
			lastCheckpointSequence = journal.Checkpoints[len(journal.Checkpoints)-1].SequenceEnd
		}
		retainedEvents := make([]schemarunpack.SessionEvent, 0, len(journal.Events))
		for _, event := range journal.Events {
			if event.Sequence > lastCheckpointSequence {
				retainedEvents = append(retainedEvents, event)
			}
		}

		records := make([]sessionJournalRecord, 0, 1+len(journal.Checkpoints)+len(retainedEvents))
		header := schemarunpack.SessionJournal{
			SchemaID:        sessionJournalSchemaID,
			SchemaVersion:   sessionJournalSchemaVersion,
			CreatedAt:       journal.CreatedAt.UTC(),
			ProducerVersion: journal.ProducerVersion,
			SessionID:       journal.SessionID,
			RunID:           journal.RunID,
			StartedAt:       journal.StartedAt.UTC(),
		}
		records = append(records, sessionJournalRecord{
			RecordType: "header",
			Header:     &header,
		})
		for i := range journal.Checkpoints {
			checkpoint := journal.Checkpoints[i]
			records = append(records, sessionJournalRecord{
				RecordType: "checkpoint",
				Checkpoint: &checkpoint,
			})
		}
		for i := range retainedEvents {
			event := retainedEvents[i]
			records = append(records, sessionJournalRecord{
				RecordType: "event",
				Event:      &event,
			})
		}
		var compacted bytes.Buffer
		for _, record := range records {
			encoded, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				return fmt.Errorf("marshal compacted session journal record: %w", marshalErr)
			}
			compacted.Write(encoded)
			compacted.WriteByte('\n')
		}

		bytesBefore := int64(0)
		if info, statErr := statLocalOrAbsolutePath(normalizedPath); statErr == nil {
			bytesBefore = info.Size()
		}
		bytesAfter := int64(compacted.Len())
		compactedChanged := len(retainedEvents) != len(journal.Events) || normalizedOutputPath != normalizedPath || bytesAfter != bytesBefore

		if !opts.DryRun {
			if compactedChanged {
				if writeErr := fsx.WriteFileAtomic(normalizedOutputPath, compacted.Bytes(), 0o600); writeErr != nil {
					return fmt.Errorf("write compacted session journal: %w", writeErr)
				}
				if normalizedOutputPath == normalizedPath {
					updatedJournal := journal
					updatedJournal.Events = retainedEvents
					updatedJournal.Checkpoints = append([]schemarunpack.SessionCheckpoint{}, journal.Checkpoints...)
					state, buildErr := buildSessionStateFromJournal(normalizedPath, updatedJournal, now)
					if buildErr != nil {
						return buildErr
					}
					producerVersion := strings.TrimSpace(opts.ProducerVersion)
					if producerVersion != "" {
						state.ProducerVersion = producerVersion
					}
					state.UpdatedAt = now
					if writeErr := writeSessionState(sessionStatePath(normalizedPath), state); writeErr != nil {
						return writeErr
					}
				}
			}
		}

		result = SessionCompactionResult{
			JournalPath:            normalizedPath,
			OutputPath:             normalizedOutputPath,
			DryRun:                 opts.DryRun,
			Compacted:              compactedChanged,
			EventsBefore:           len(journal.Events),
			EventsAfter:            len(retainedEvents),
			Checkpoints:            len(journal.Checkpoints),
			LastCheckpointSequence: lastCheckpointSequence,
			BytesBefore:            bytesBefore,
			BytesAfter:             bytesAfter,
		}
		return nil
	})
	if lockErr != nil {
		return SessionCompactionResult{}, lockErr
	}
	return result, nil
}

func ContainsSessionChainPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".json")
}

func ResolveSessionCheckpointRunpack(chainPath string, checkpointRef string) (schemarunpack.SessionCheckpoint, error) {
	chain, err := ReadSessionChain(chainPath)
	if err != nil {
		return schemarunpack.SessionCheckpoint{}, err
	}
	trimmed := strings.TrimSpace(checkpointRef)
	if trimmed == "" || strings.EqualFold(trimmed, "latest") {
		return chain.Checkpoints[len(chain.Checkpoints)-1], nil
	}
	index := -1
	if _, parseErr := fmt.Sscanf(trimmed, "%d", &index); parseErr != nil || index <= 0 {
		return schemarunpack.SessionCheckpoint{}, fmt.Errorf("invalid checkpoint reference: %s", checkpointRef)
	}
	for _, checkpoint := range chain.Checkpoints {
		if checkpoint.CheckpointIndex == index {
			return checkpoint, nil
		}
	}
	return schemarunpack.SessionCheckpoint{}, fmt.Errorf("checkpoint %d not found", index)
}

func sessionChainLooksLike(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".json")
}

func maybeReadSessionChain(path string) (schemarunpack.SessionChain, bool) {
	if !sessionChainLooksLike(path) {
		return schemarunpack.SessionChain{}, false
	}
	chain, err := ReadSessionChain(path)
	if err != nil {
		return schemarunpack.SessionChain{}, false
	}
	return chain, true
}

func CompareRunpackOrSessionChain(leftPath string, rightPath string, privacy DiffPrivacy) (DiffResult, error) {
	leftChain, leftIsChain := maybeReadSessionChain(leftPath)
	rightChain, rightIsChain := maybeReadSessionChain(rightPath)
	if leftIsChain && rightIsChain {
		diff := DiffSessionChains(leftChain, rightChain)
		filesChanged := []string{}
		for _, idx := range diff.ChangedIndexes {
			filesChanged = append(filesChanged, fmt.Sprintf("checkpoint_%d", idx))
		}
		for _, idx := range diff.LeftOnlyIndexes {
			filesChanged = append(filesChanged, fmt.Sprintf("left_only_checkpoint_%d", idx))
		}
		for _, idx := range diff.RightOnlyIndexes {
			filesChanged = append(filesChanged, fmt.Sprintf("right_only_checkpoint_%d", idx))
		}
		slices.Sort(filesChanged)
		return DiffResult{
			Privacy: privacy,
			Summary: DiffSummary{
				RunIDLeft:       leftChain.RunID,
				RunIDRight:      rightChain.RunID,
				ManifestChanged: diff.ChangedCheckpoints,
				FilesChanged:    filesChanged,
				IntentsChanged:  diff.ChangedCheckpoints,
				ResultsChanged:  diff.ChangedCheckpoints,
				RefsChanged:     diff.ChangedCheckpoints,
			},
		}, nil
	}
	return DiffRunpacks(leftPath, rightPath, privacy)
}
