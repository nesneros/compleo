// Package cmdutil provides utilities for compleo command execution.
package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// CompletionDir is where "activate" wires fpath to, and where generated completions live.
func CompletionDir() (string, error) {
	if dir := os.Getenv("COMPLEO_COMPLETION_DIR"); dir != "" {
		return dir, nil
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining completion directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "compleo", "completions"), nil
}

// WriteCompletionFile skips the write when content is unchanged, so activate (which may run
// on every shell startup) doesn't bump the dir's mtime and force a full compinit rescan.
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

// IsPiped reports whether f has data piped in, vs. an interactive terminal.
func IsPiped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
