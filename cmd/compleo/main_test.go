package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
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
	err := run([]string{"parse", "--", "--version"}, true, specReader, &stdout)
	require.NoError(t, err)
	assert.NotEqual(t, "dev\n", stdout.String())
}

// A wrapped-script argv error is encoded into the output (print to stderr + exit 2), not
// returned as a Go error: eval alone should stop the script, no caller exit-code check needed.
func TestRunParseWithUnknownFlagEmitsSelfExitingError(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: mytool\n"), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "exit 2")
}

func TestRunParseErrorIncludesScriptName(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: greet\n"), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "greet:")
}

// Regression test: cobra used to write its own "Error: ...\nUsage:..." straight to the
// real os.Stderr for the wrapped command, duplicating the error already in stdout.
func TestRunParseErrorDoesNotDuplicateOntoRealStderr(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --times=: how many times
flagtypes:
  times: int
`)
	var runErr error
	stderr := captureRealStderr(t, func() {
		runErr = run([]string{"parse", "--", "--times", "x"}, true, specReader, &bytes.Buffer{})
	})

	assert.NoError(t, runErr)
	assert.Empty(t, stderr)
}

// captureRealStderr redirects the real os.Stderr for fn's duration and returns what it wrote.
func captureRealStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	fn()

	require.NoError(t, w.Close())
	captured, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(captured)
}

func TestRunDumpWritesSuccessOutputToStderrTooBeforeStdout(t *testing.T) {
	specReader := strings.NewReader("name: mytool\nflags:\n  --count=: how many\n")
	var stdout bytes.Buffer

	stderr := captureRealStderr(t, func() {
		require.NoError(t, run([]string{"parse", "--dump", "--", "--count", "3"}, true, specReader, &stdout))
	})

	assert.Equal(t, stdout.String(), stderr)
}

func TestRunDumpWritesErrorOutputToStderrTooBeforeStdout(t *testing.T) {
	specReader := strings.NewReader("name: mytool\nflags:\n  --times=: how many\nflagtypes:\n  times: int\n")
	var stdout bytes.Buffer

	stderr := captureRealStderr(t, func() {
		require.NoError(t, run([]string{"parse", "--dump", "--", "--times", "x"}, true, specReader, &stdout))
	})

	assert.Equal(t, stdout.String(), stderr)
	assert.Contains(t, stdout.String(), "exit 2")
}

func TestRunWithoutDumpWritesNothingToRealStderrOnSuccess(t *testing.T) {
	specReader := strings.NewReader("name: mytool\nflags:\n  --count=: how many\n")
	var stdout bytes.Buffer

	stderr := captureRealStderr(t, func() {
		require.NoError(t, run([]string{"parse", "--", "--count", "3"}, true, specReader, &stdout))
	})

	assert.Empty(t, stderr)
	assert.NotEmpty(t, stdout.String())
}

// "--" is not actually required: it's only needed to disambiguate a wrapped script's own
// flag from one of compleo's own "parse" flags (--dump). With no collision, it's optional.
func TestRunParseWithoutDashWorksWhenNoCollision(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "world"}, true, strings.NewReader("name: mytool\npositionals:\n  - name: who\n"), &stdout)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "compleo[who]='world'")
}

// A flag compleo's own "parse" doesn't recognize errors regardless of "--" or position.
func TestRunParseUnrecognizedCompleoFlagFails(t *testing.T) {
	err := run([]string{"parse", "--not-a-real-compleo-flag", "world"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{})
	assert.Error(t, err)
}

// Regression test: a wrapped script with zero of its own args binds the variadic "args"
// positional to nil, not an empty slice, which used to panic in parse.Parse.
func TestRunParseWithNoWrappedArgsDoesNotPanic(t *testing.T) {
	err := run([]string{"parse"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{})
	assert.NoError(t, err)
}

func TestRunParseRequiresPipedStdin(t *testing.T) {
	err := run([]string{"parse", "--", "--count", "3"}, false, strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestRunParseEmitsZshScript(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	err := run([]string{"parse", "--", "--count", "3"}, true, specReader, &stdout)
	require.NoError(t, err)

	assert.NotEmpty(t, stdout.String())
}

func TestRunParseWithHelpEmitsHelpAsZsh(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
description: A tool.
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	err := run([]string{"parse", "--", "--help"}, true, specReader, &stdout)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "A tool.")
	assert.Contains(t, stdout.String(), "exit 0")
}

// Regression test for the bug this fixes: cobra used to write the wrapped command's
// --help straight to stdout, corrupting the zsh the wrapped script evals.
func TestRunParseWithHelpOutputEvalsCleanlyUnderRealZsh(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
description: A tool.
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	require.NoError(t, run([]string{"parse", "--", "--help"}, true, specReader, &stdout))

	cmd := exec.Command("zsh", "-fc", stdout.String())
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "A tool.")
}

// Proves eval alone is enough to stop the script on a bad arg, no "|| exit $?" needed:
// an unreachable statement after the eval must not run, and the real exit code must be 2.
func TestRunParseErrorOutputStopsScriptWhenEvaldUnderRealZsh(t *testing.T) {
	var stdout bytes.Buffer
	require.NoError(t, run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: greet\n"), &stdout))

	script := `eval "$SNIPPET"` + "\n" + `print -r -- "unreachable"`
	cmd := exec.Command("zsh", "-fc", script)
	cmd.Env = append(os.Environ(), "SNIPPET="+stdout.String())
	out, err := cmd.Output()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 2, exitErr.ExitCode())
	assert.Empty(t, out)
}

func TestRunParseWithInvalidSpecFails(t *testing.T) {
	err := run([]string{"parse", "--"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{})
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
