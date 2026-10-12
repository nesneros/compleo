package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/zsh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithNoArgsAndNotPipedShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRunPanicsOnNilArgs(t *testing.T) {
	assert.Panics(t, func() {
		_ = run(nil, false, strings.NewReader(""), &bytes.Buffer{})
	})
}

func TestRunWithHelpFlagSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--help"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

// _carapace is carapace's hidden subcommand; it must succeed without piped stdin.
func TestRunCarapaceCallbackSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	// args: dispatch, shell, typed name (ignored by carapace), word being completed (empty = list all).
	err := run([]string{"_carapace", "zsh", "compleo", ""}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())
}

func TestRunWithVersionCommandPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"version"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "dev\n", stdout.String())
}

func TestRunWithOldVersionFlagFails(t *testing.T) {
	err := run([]string{"--version"}, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithHelpAsArgumentFails(t *testing.T) {
	err := run([]string{"help"}, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithValidateOnValidSpecFromStdinSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"validate"}, true, strings.NewReader("name: mytool\n"), &stdout)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", stdout.String())
}

func TestRunWithValidateOnInvalidSpecFromStdinFails(t *testing.T) {
	err := run([]string{"validate"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithValidateHelpSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"validate", "--help"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRunWithCompletionPrintsZshSnippetEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"completion"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "#compdef compleo")
}

// compleo's own --version must never shadow the wrapped script's own --version flag.
func TestRunParseVersionFlagIsWrappedScriptsOwn(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --version: mytool's own version flag, not compleo's
`)
	var stdout bytes.Buffer
	err := run([]string{"parse", "--version"}, true, specReader, &stdout)
	require.NoError(t, err)
	assert.NotEqual(t, "dev\n", stdout.String())
}

func TestRunParseWithUnknownFlagFails(t *testing.T) {
	err := run([]string{"parse", "--bogus"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunParseRequiresPipedStdin(t *testing.T) {
	err := run([]string{"parse", "--count", "3"}, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunParseEmitsZshScript(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	err := run([]string{"parse", "--count", "3"}, true, specReader, &stdout)
	require.NoError(t, err)

	assert.NotEmpty(t, stdout.String())
}

func TestRunParseWithInvalidSpecFails(t *testing.T) {
	err := run([]string{"parse"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunWithActivatePrintsFpathWiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMPLEO_COMPLETION_DIR", dir)

	var stdout bytes.Buffer
	err := run([]string{"activate"}, false, strings.NewReader(""), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "fpath+=("+zsh.Quote(dir)+")")
}

func TestRunWithActivateWritesOwnCompletionFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMPLEO_COMPLETION_DIR", dir)

	err := run([]string{"activate"}, false, strings.NewReader(""), &bytes.Buffer{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "_compleo"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "#compdef compleo")
}

func TestRunWithActivateTwiceDoesNotRewriteUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMPLEO_COMPLETION_DIR", dir)

	require.NoError(t, run([]string{"activate"}, false, strings.NewReader(""), &bytes.Buffer{}))
	path := filepath.Join(dir, "_compleo")
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, run([]string{"activate"}, false, strings.NewReader(""), &bytes.Buffer{}))
	after, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestWriteCompletionFileSkipsRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, writeCompletionFile(dir, "_mytool", "content-a"))
	path := filepath.Join(dir, "_mytool")
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, writeCompletionFile(dir, "_mytool", "content-a"))
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	require.NoError(t, writeCompletionFile(dir, "_mytool", "content-b"))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content-b", string(content))
}

func TestCompletionDirHonorsEnvOverride(t *testing.T) {
	t.Setenv("COMPLEO_COMPLETION_DIR", "/some/custom/dir")

	dir, err := completionDir()
	require.NoError(t, err)
	assert.Equal(t, "/some/custom/dir", dir)
}

func TestCompletionDirHonorsXDGDataHome(t *testing.T) {
	t.Setenv("COMPLEO_COMPLETION_DIR", "")
	t.Setenv("XDG_DATA_HOME", "/xdg/data")

	dir, err := completionDir()
	require.NoError(t, err)
	assert.Equal(t, "/xdg/data/compleo/completions", dir)
}

func TestCompletionDirDefaultsUnderHome(t *testing.T) {
	t.Setenv("COMPLEO_COMPLETION_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")

	dir, err := completionDir()
	require.NoError(t, err)
	assert.Contains(t, dir, filepath.Join(".local", "share", "compleo", "completions"))
}
