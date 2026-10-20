package shells

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellFromShebangDirectPath(t *testing.T) {
	shell, err := shellFromShebang("#!/usr/local/bin/zsh")
	require.NoError(t, err)
	assert.Equal(t, "zsh", shell)
}

func TestShellFromShebangEnv(t *testing.T) {
	shell, err := shellFromShebang("#!/usr/bin/env zsh")
	require.NoError(t, err)
	assert.Equal(t, "zsh", shell)
}

func TestShellFromShebangIgnoresInterpreterArgs(t *testing.T) {
	shell, err := shellFromShebang("#!/bin/bash -e")
	require.NoError(t, err)
	assert.Equal(t, "bash", shell)
}

func TestShellFromShebangAllowsSpaceAfterBang(t *testing.T) {
	shell, err := shellFromShebang("#! /usr/bin/env bash")
	require.NoError(t, err)
	assert.Equal(t, "bash", shell)
}

func TestShellFromShebangTrimsCarriageReturn(t *testing.T) {
	shell, err := shellFromShebang("#!/bin/zsh\r\n")
	require.NoError(t, err)
	assert.Equal(t, "zsh", shell)
}

func TestShellFromShebangRejectsMissingShebang(t *testing.T) {
	_, err := shellFromShebang("echo hi")
	require.Error(t, err)
}

func TestShellFromShebangRejectsEnvWithoutCommand(t *testing.T) {
	_, err := shellFromShebang("#!/usr/bin/env")
	require.Error(t, err)
}

func TestShebangShellReadsFirstLine(t *testing.T) {
	shell, err := ShebangShell(strings.NewReader("#!/bin/zsh\n#!/bin/bash\n"))
	require.NoError(t, err)
	assert.Equal(t, "zsh", shell)
}

func TestShebangShellNoTrailingNewline(t *testing.T) {
	shell, err := ShebangShell(strings.NewReader("#!/usr/bin/env bash"))
	require.NoError(t, err)
	assert.Equal(t, "bash", shell)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestShebangShellAccepts256CharacterLine(t *testing.T) {
	line := "#!/bin/zsh" + strings.Repeat(" ", 256-len("#!/bin/zsh"))
	shell, err := ShebangShell(strings.NewReader(line))
	require.NoError(t, err)
	assert.Equal(t, "zsh", shell)
}

func TestShebangShellRejectsLineLongerThan256(t *testing.T) {
	_, err := ShebangShell(strings.NewReader(strings.Repeat("a", 257)))
	require.Error(t, err)
}

func TestShebangShellReturnsReadError(t *testing.T) {
	_, err := ShebangShell(errReader{})
	require.EqualError(t, err, "boom")
}
