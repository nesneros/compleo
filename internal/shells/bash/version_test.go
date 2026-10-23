package bash

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionSupportedAcceptsExactMinimum(t *testing.T) {
	assert.True(t, VersionSupported(MinMajor, MinMinor))
}

func TestVersionSupportedAcceptsNewerMajor(t *testing.T) {
	assert.True(t, VersionSupported(MinMajor+1, 0))
}

func TestVersionSupportedRejectsOlderMinor(t *testing.T) {
	assert.False(t, VersionSupported(MinMajor, MinMinor-1))
}

func TestVersionSupportedRejectsOlderMajor(t *testing.T) {
	assert.False(t, VersionSupported(MinMajor-1, 99))
}

func TestParseVersionParsesMajorMinor(t *testing.T) {
	major, minor, err := ParseVersion("4 3")
	require.NoError(t, err)
	assert.Equal(t, 4, major)
	assert.Equal(t, 3, minor)
}

func TestParseVersionRejectsMalformedOutput(t *testing.T) {
	_, _, err := ParseVersion("not-a-version")
	assert.Error(t, err)
}

// DetectVersion runs a fake "bash" that ignores its args and always reports a fixed version,
// so this doesn't depend on whatever bash happens to be installed where tests run.
func TestDetectVersionRunsScriptAndParsesItsOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake-bash")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho '4 2'\n"), 0o755)) //nolint:gosec

	major, minor, err := DetectVersion(path)
	require.NoError(t, err)
	assert.Equal(t, 4, major)
	assert.Equal(t, 2, minor)
}

func TestDetectVersionFailsOnMissingBinary(t *testing.T) {
	_, _, err := DetectVersion(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Error(t, err)
}
