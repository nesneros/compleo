package cmdutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionDirHonorsEnvOverride(t *testing.T) {
	t.Setenv(EnvCompletionDir, "/some/custom/dir")
	defer t.Setenv(EnvCompletionDir, "")

	dir := CompletionDir()
	assert.Equal(t, "/some/custom/dir", dir)
}

func TestCompletionDirHonorsXDGDataHome(t *testing.T) {
	t.Setenv(EnvCompletionDir, "")
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	defer func() {
		t.Setenv(EnvCompletionDir, "")
		t.Setenv("XDG_DATA_HOME", "")
	}()

	dir := CompletionDir()
	assert.Equal(t, "/xdg/data/compleo/completions", dir)
}

func TestCompletionDirDefaultsUnderHome(t *testing.T) {
	t.Setenv(EnvCompletionDir, "")
	t.Setenv("XDG_DATA_HOME", "")
	defer func() {
		t.Setenv(EnvCompletionDir, "")
		t.Setenv("XDG_DATA_HOME", "")
	}()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	dir := CompletionDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "compleo", "completions"), dir)
}

func TestWriteCompletionFileSkipsRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, WriteCompletionFile(dir, "_mytool", "content-a"))
	path := filepath.Join(dir, "_mytool")
	before, err := os.Stat(path)
	require.NoError(t, err)

	// Small delay to ensure different modification times if write occurs
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, WriteCompletionFile(dir, "_mytool", "content-a"))
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	require.NoError(t, WriteCompletionFile(dir, "_mytool", "content-b"))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content-b", string(content))
}

func TestWriteCompletionFileCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "completion")

	require.NoError(t, WriteCompletionFile(dir, "_mytool", "content"))
	content, err := os.ReadFile(filepath.Join(dir, "_mytool"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestIsPiped(t *testing.T) {
	// Create a temporary file (not a terminal)
	f, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer os.Remove(f.Name()) //nolint:errcheck
	defer f.Close()           //nolint:errcheck

	assert.True(t, IsPiped(f))
}

func TestIsPipedTerminal(t *testing.T) {
	// stdin is a terminal (when running tests)
	// This test may behave differently in CI vs local
	// We just verify it doesn't panic
	_ = IsPiped(os.Stdin)
}
