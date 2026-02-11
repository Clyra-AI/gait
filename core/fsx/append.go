package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	appendLockTimeout    = 30 * time.Second
	appendLockRetry      = 10 * time.Millisecond
	appendLockStaleAfter = 2 * time.Minute
	maxInt               = int(^uint(0) >> 1)
)

// AppendLineLocked appends exactly one line to a file with a cross-process lock.
// The caller provides raw bytes for one record; this function appends a trailing
// newline and fsyncs the file before returning.
func AppendLineLocked(path string, line []byte, mode os.FileMode) error {
	cleanPath, err := validateLocalOrAbsolutePath(path)
	if err != nil {
		return err
	}
	parent := filepath.Dir(cleanPath)
	if parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("create append directory: %w", err)
		}
	}
	payloadCapacity, err := appendPayloadCapacity(len(line))
	if err != nil {
		return err
	}
	payload := make([]byte, 0, payloadCapacity)
	payload = append(payload, line...)
	payload = append(payload, '\n')

	if err := withAppendFileLock(cleanPath, func() error {
		// #nosec G304 -- append path is validated local relative or absolute.
		file, openErr := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, mode)
		if openErr != nil {
			return fmt.Errorf("open append file: %w", openErr)
		}
		defer func() {
			_ = file.Close()
		}()
		if _, writeErr := file.Write(payload); writeErr != nil {
			return fmt.Errorf("append file line: %w", writeErr)
		}
		if syncErr := file.Sync(); syncErr != nil {
			return fmt.Errorf("sync append file: %w", syncErr)
		}
		return nil
	}); err != nil {
		return err
	}

	if parent != "." && parent != "" {
		syncDirectory(parent)
	}
	return nil
}

func appendPayloadCapacity(lineLength int) (int, error) {
	if lineLength < 0 {
		return 0, fmt.Errorf("line length must be >= 0")
	}
	if lineLength >= maxInt {
		return 0, fmt.Errorf("line length exceeds maximum supported size")
	}
	return lineLength + 1, nil
}

func withAppendFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	start := time.Now()
	for {
		// #nosec G304 -- lock path is derived from a validated append path.
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = lockFile.Close()
			defer func() {
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !isAppendLockContention(err, lockPath) {
			return fmt.Errorf("acquire append lock: %w", err)
		}
		if shouldRecoverStaleAppendLock(lockPath, time.Now().UTC()) {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Since(start) >= appendLockTimeout {
			return fmt.Errorf("append lock timeout")
		}
		time.Sleep(appendLockRetry)
	}
}

func isAppendLockContention(acquireErr error, lockPath string) bool {
	if os.IsExist(acquireErr) {
		return true
	}
	_, statErr := os.Stat(lockPath)
	return statErr == nil || os.IsPermission(statErr)
}

func shouldRecoverStaleAppendLock(lockPath string, now time.Time) bool {
	// #nosec G304 -- lock path is derived from a validated append path.
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}
	return now.Sub(info.ModTime().UTC()) > appendLockStaleAfter
}

func validateLocalOrAbsolutePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if filepath.IsLocal(cleanPath) {
		return cleanPath, nil
	}
	if strings.HasPrefix(cleanPath, string(filepath.Separator)) {
		return cleanPath, nil
	}
	if volume := filepath.VolumeName(cleanPath); volume != "" && strings.HasPrefix(cleanPath, volume+string(filepath.Separator)) {
		return cleanPath, nil
	}
	return "", fmt.Errorf("path must be local relative or absolute")
}
