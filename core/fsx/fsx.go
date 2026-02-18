package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func WriteFileAtomic(path string, content []byte, mode os.FileMode) error {
	cleanPath := filepath.Clean(path)
	if filepath.IsLocal(cleanPath) {
		// Allowed local relative destination.
	} else if strings.HasPrefix(cleanPath, string(filepath.Separator)) {
		// Allowed absolute destination.
	} else if volume := filepath.VolumeName(cleanPath); volume != "" && strings.HasPrefix(cleanPath, volume+string(filepath.Separator)) {
		// Allowed absolute destination with a drive volume prefix.
	} else {
		return fmt.Errorf("path must be local relative or absolute")
	}

	parent := filepath.Dir(cleanPath)
	base := filepath.Base(cleanPath)

	var (
		tempFile *os.File
		err      error
	)
	if filepath.IsLocal(parent) {
		tempFile, err = createTempFile(parent, base)
	} else if strings.HasPrefix(parent, string(filepath.Separator)) {
		tempFile, err = createTempFile(parent, base)
	} else if volume := filepath.VolumeName(parent); volume != "" && strings.HasPrefix(parent, volume+string(filepath.Separator)) {
		tempFile, err = createTempFile(parent, base)
	} else {
		return fmt.Errorf("parent directory must be local relative or absolute")
	}
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := renameWithWindowsFallback(tempPath, cleanPath, runtime.GOOS); err != nil {
		return err
	}
	cleanup = false

	if filepath.IsLocal(parent) {
		syncDirectory(parent)
	} else if strings.HasPrefix(parent, string(filepath.Separator)) {
		syncDirectory(parent)
	} else if volume := filepath.VolumeName(parent); volume != "" && strings.HasPrefix(parent, volume+string(filepath.Separator)) {
		syncDirectory(parent)
	} else {
		return fmt.Errorf("parent directory must be local relative or absolute")
	}
	return nil
}

func renameWithWindowsFallback(tempPath string, path string, goos string) error {
	if filepath.IsLocal(path) {
		return renameWithValidatedTempPath(tempPath, path, goos)
	}
	if strings.HasPrefix(path, string(filepath.Separator)) {
		return renameWithValidatedTempPath(tempPath, path, goos)
	}
	if volume := filepath.VolumeName(path); volume != "" && strings.HasPrefix(path, volume+string(filepath.Separator)) {
		return renameWithValidatedTempPath(tempPath, path, goos)
	}
	return fmt.Errorf("path must be local relative or absolute")
}

func renameWithValidatedTempPath(tempPath string, path string, goos string) error {
	if filepath.IsLocal(tempPath) {
		return renameWithWindowsSemantics(tempPath, path, goos)
	}
	if strings.HasPrefix(tempPath, string(filepath.Separator)) {
		return renameWithWindowsSemantics(tempPath, path, goos)
	}
	if volume := filepath.VolumeName(tempPath); volume != "" && strings.HasPrefix(tempPath, volume+string(filepath.Separator)) {
		return renameWithWindowsSemantics(tempPath, path, goos)
	}
	return fmt.Errorf("temp path must be local relative or absolute")
}

func renameWithWindowsSemantics(tempPath string, path string, goos string) error {
	// #nosec G703 -- both paths are validated local/absolute before entering this function.
	if err := os.Rename(tempPath, path); err != nil {
		if goos != "windows" {
			return fmt.Errorf("rename temp file: %w", err)
		}
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove destination before rename: %w", removeErr)
		}
		// #nosec G703 -- both paths are validated local/absolute before entering this function.
		if renameErr := os.Rename(tempPath, path); renameErr != nil {
			return fmt.Errorf("rename temp file after remove: %w", renameErr)
		}
	}
	return nil
}

func createTempFile(parent string, base string) (*os.File, error) {
	return os.CreateTemp(parent, "."+base+".tmp-*")
}

func syncDirectory(parent string) {
	// #nosec G304 -- parent directory path is validated local or absolute before use.
	if dirHandle, err := os.Open(parent); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
}
