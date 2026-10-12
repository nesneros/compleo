package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithNoArgsAndNotPipedFails(t *testing.T) {
	err := run(nil, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithHelpFlagSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--help"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRunPassthroughWithUnknownFlagFails(t *testing.T) {
	err := run([]string{"--bogus"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithVersionFlagPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--version"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "dev\n", stdout.String())
}

func TestRunWithValidateOnValidSpecFromStdinSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--validate"}, true, strings.NewReader("name: mytool\n"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", stdout.String())
}

func TestRunWithValidateOnInvalidSpecFromStdinFails(t *testing.T) {
	err := run([]string{"--validate"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunPassthroughRequiresPipedStdin(t *testing.T) {
	err := run([]string{"--count", "3"}, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunPassthroughEmitsZshScript(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	err := run([]string{"--count", "3"}, true, specReader, &stdout)
	require.NoError(t, err)

	assert.NotEmpty(t, stdout.String())
}

func TestRunPassthroughWithInvalidSpecFails(t *testing.T) {
	err := run(nil, true, strings.NewReader("not: [valid"), &bytes.Buffer{})
	assert.Error(t, err)
}
