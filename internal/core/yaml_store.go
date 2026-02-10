package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// maxYAMLFileSize is the maximum size of a vendor.yml or vendor.lock file (1 MB).
// SEC-020: Prevents memory exhaustion from maliciously crafted or accidentally
// oversized files. A config with 500 vendors and detailed mappings is well under
// 100 KB, so 1 MB is generous.
const maxYAMLFileSize = 1 << 20 // 1 MB

// YAMLStore provides generic YAML file I/O operations.
// YAMLStore consolidates duplicate code between ConfigStore and LockStore
// using Go 1.18+ generics.
type YAMLStore[T any] struct {
	rootDir      string
	filename     string
	allowMissing bool // If true, missing file returns zero value instead of error
}

// NewYAMLStore creates a new YAML store for type T.
//
// Parameters:
//   - rootDir: Directory containing the YAML file
//   - filename: Name of the YAML file (e.g., "vendor.yml", "vendor.lock")
//   - allowMissing: If true, Load() returns zero value for missing files instead of error
func NewYAMLStore[T any](rootDir, filename string, allowMissing bool) *YAMLStore[T] {
	return &YAMLStore[T]{
		rootDir:      rootDir,
		filename:     filename,
		allowMissing: allowMissing,
	}
}

// Path returns the full file path
func (s *YAMLStore[T]) Path() string {
	return filepath.Join(s.rootDir, s.filename)
}

// Load reads and unmarshals the YAML file into type T.
// SEC-020: Rejects files larger than maxYAMLFileSize (1 MB) to prevent
// memory exhaustion from oversized config files.
func (s *YAMLStore[T]) Load() (T, error) {
	var result T

	// SEC-020: Check file size before reading to prevent memory exhaustion
	info, err := os.Stat(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && s.allowMissing {
			return result, nil
		}
		return result, err
	}
	if info.Size() > maxYAMLFileSize {
		return result, fmt.Errorf("%s exceeds maximum size (%d bytes > %d byte limit)", s.filename, info.Size(), maxYAMLFileSize)
	}

	data, err := os.ReadFile(s.Path())
	if err != nil {
		// Handle missing file based on allowMissing setting
		if errors.Is(err, os.ErrNotExist) && s.allowMissing {
			return result, nil // Return zero value
		}
		return result, err
	}

	if err := yaml.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("invalid %s: %w", s.filename, err)
	}

	return result, nil
}

// Save marshals and writes type T to the YAML file
func (s *YAMLStore[T]) Save(data T) error {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", s.filename, err)
	}

	if err := os.WriteFile(s.Path(), bytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", s.filename, err)
	}

	return nil
}
