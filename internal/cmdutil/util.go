// Package cmdutil provides utilities for compleo command execution.
package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// CompletionDir returns the directory completion files are read from and written to:
// EnvCompletionDir if set, else "$XDG_DATA_HOME/compleo/completions" (defaulting
// XDG_DATA_HOME to "~/.local/share"). It panics if the home directory can't be determined.
func CompletionDir() string {
	if dir := os.Getenv(EnvCompletionDir); dir != "" {
		return dir
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			Panic(fmt.Sprintf("determining completion directory: %s", err))
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "compleo", "completions")
}

// WriteCompletionFile writes content to name in dir, creating dir if needed. It leaves the
// file's mtime untouched when content already matches what's on disk.
func WriteCompletionFile(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating completion directory: %w", err)
	}

	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
