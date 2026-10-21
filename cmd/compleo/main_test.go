package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells/bash"
	"github.com/nesneros/compleo/internal/shells/zsh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithNoArgsAndNotPipedShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRunPanicsOnNilArgs(t *testing.T) {
	assert.Panics(t, func() {
		_ = run(nil, false, strings.NewReader(""), &bytes.Buffer{}, "")
	})
}

func TestRunWithHelpFlagSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--help"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

// _carapace is carapace's hidden subcommand; it must succeed without piped stdin.
func TestRunCarapaceCallbackSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	// args: dispatch, shell, typed name (ignored by carapace), word being completed (empty = list all).
	err := run([]string{"_carapace", "zsh", "compleo", ""}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())
}

func TestRunWithVersionCommandPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"version"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Equal(t, "dev\n", stdout.String())
}

func TestRunWithOldVersionFlagFails(t *testing.T) {
	err := run([]string{"--version"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunWithHelpAsArgumentFails(t *testing.T) {
	err := run([]string{"help"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunWithValidateOnValidSpecFromStdinSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"validate"}, true, strings.NewReader("name: mytool\n"), &stdout, "")
	require.NoError(t, err)
	assert.Equal(t, "ok\n", stdout.String())
}

func TestRunWithValidateOnInvalidSpecFromStdinFails(t *testing.T) {
	err := run([]string{"validate"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunWithValidateHelpSucceedsEvenWhenNotPiped(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"validate", "--help"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRunWithValidateRequiresPipedStdin(t *testing.T) {
	err := run([]string{"validate"}, false, strings.NewReader("name: mytool\n"), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

// compleo's own --version must never shadow the wrapped script's own --version flag.
func TestRunParseVersionFlagIsWrappedScriptsOwn(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --version: mytool's own version flag, not compleo's
`)
	var stdout bytes.Buffer
	err := run([]string{"parse", "--", "--version"}, true, specReader, &stdout, "")
	require.NoError(t, err)
	assert.NotEqual(t, "dev\n", stdout.String())
}

// A wrapped-script argv error is encoded into the output (print to stderr + exit 2), not
// returned as a Go error: eval alone should stop the script, no caller exit-code check needed.
func TestRunParseWithUnknownFlagEmitsSelfExitingError(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: mytool\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "exit 2")
}

func TestRunParseErrorIncludesScriptName(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: greet\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "greet:")
}

// --name fills in a missing name: without needing an unquoted, escape-requiring heredoc.
func TestRunParseNameFlagFillsInMissingSpecName(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--name", "greet", "--", "--bogus"}, true, strings.NewReader("flags:\n  --loud: shout\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "greet:")
}

func TestRunParseNameFlagMatchingDeclaredNameSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--name", "greet", "--", "--bogus"}, true, strings.NewReader("name: greet\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "greet:")
}

// The matching logic itself is spec.ParseYAML's (internal/spec/parse_test.go); this just
// confirms --name is wired through to it.
func TestRunParseNameFlagMismatchWithDeclaredNameFails(t *testing.T) {
	err := run([]string{"parse", "--name", "other-name", "--", "world"}, true, strings.NewReader("name: greet\n"), &bytes.Buffer{}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec declares name: "greet"`)
	assert.Contains(t, err.Error(), `"other-name" was given`)
}

// Regression test: cobra used to write its own "Error: ...\nUsage:..." straight to the
// real os.Stderr for the wrapped command, duplicating the error already in stdout.
func TestRunParseErrorDoesNotDuplicateOntoRealStderr(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --times=: {description: how many times, type: int}
`)
	var runErr error
	stderr := captureRealStderr(t, func() {
		runErr = run([]string{"parse", "--", "--times", "x"}, true, specReader, &bytes.Buffer{}, "")
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

func TestRunWritesNothingToRealStderrOnSuccess(t *testing.T) {
	specReader := strings.NewReader("name: mytool\nflags:\n  --count=: how many\n")
	var stdout bytes.Buffer

	stderr := captureRealStderr(t, func() {
		require.NoError(t, run([]string{"parse", "--", "--count", "3"}, true, specReader, &stdout, ""))
	})

	assert.Empty(t, stderr)
	assert.NotEmpty(t, stdout.String())
}

// "--" is not actually required: it's only needed to disambiguate a wrapped script's own
// flag from one of compleo's own "parse" flags (--name). With no collision, it's optional.
func TestRunParseWithoutDashWorksWhenNoCollision(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "world"}, true, strings.NewReader("name: mytool\npositionals:\n  - name: who\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "compleo[who]='world'")
}

// A flag compleo's own "parse" doesn't recognize errors regardless of "--" or position.
func TestRunParseUnrecognizedCompleoFlagFails(t *testing.T) {
	err := run([]string{"parse", "--not-a-real-compleo-flag", "world"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

// Regression test: a wrapped script with zero of its own args binds the variadic "args"
// positional to nil, not an empty slice, which used to panic in parse.Parse.
func TestRunParseWithNoWrappedArgsDoesNotPanic(t *testing.T) {
	err := run([]string{"parse"}, true, strings.NewReader("name: mytool\n"), &bytes.Buffer{}, "")
	assert.NoError(t, err)
}

func TestRunParseRequiresPipedStdin(t *testing.T) {
	err := run([]string{"parse", "--", "--count", "3"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunUpRequiresPipedStdin(t *testing.T) {
	err := run([]string{"up"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunUpEmitsReinvocationSnippet(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"up"}, true, strings.NewReader("flags:\n  --loud: shout\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `compleo parse --name="${0:t}" -- "$@"`)
}

func TestRunParseEmitsZshScript(t *testing.T) {
	specReader := strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`)
	var stdout bytes.Buffer

	err := run([]string{"parse", "--", "--count", "3"}, true, specReader, &stdout, "")
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

	err := run([]string{"parse", "--", "--help"}, true, specReader, &stdout, "")
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "A tool.")
	assert.Contains(t, stdout.String(), "exit 0")
}

// --name must also correct the --help/"Usage:" synopsis, not just error text — spec.ParseYAML
// applies it before building cmd, so cmd.Use already reflects it from the start.
func TestRunParseNameFlagCorrectsUsageSynopsis(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"parse", "--name", "greet", "--", "--help"}, true, strings.NewReader("flags:\n  --loud: shout\n"), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Usage:\n  greet [flags]")
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

	require.NoError(t, run([]string{"parse", "--", "--help"}, true, specReader, &stdout, ""))

	cmd := exec.Command("zsh", "-fc", stdout.String())
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "A tool.")
}

// Proves eval alone is enough to stop the script on a bad arg, no "|| exit $?" needed:
// an unreachable statement after the eval must not run, and the real exit code must be 2.
func TestRunParseErrorOutputStopsScriptWhenEvaldUnderRealZsh(t *testing.T) {
	var stdout bytes.Buffer
	require.NoError(t, run([]string{"parse", "--", "--bogus"}, true, strings.NewReader("name: greet\n"), &stdout, ""))

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
	err := run([]string{"parse", "--"}, true, strings.NewReader("not: [valid"), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunWithActivateExplicitZshPrintsFpathWiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	var stdout bytes.Buffer
	err := run([]string{"activate", "zsh"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "fpath+=("+zsh.Em.Quote(dir)+")")
}

func TestRunWithActivateBashPrintsSourceLoop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	var stdout bytes.Buffer
	err := run([]string{"activate", "bash"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "for f in "+bash.Em.Quote(dir)+"/*; do")
	assert.NotContains(t, stdout.String(), "fpath+=")
}

func TestRunWithActivateBashWritesBashCompletion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	err := run([]string{"activate", "bash"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "_compleo"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "complete")
	assert.NotContains(t, string(content), "#compdef compleo")
}

func TestRunWithActivateRejectsUnknownShell(t *testing.T) {
	err := run([]string{"activate", "nosuch"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nosuch")
}

func TestRunWithActivateWritesOwnCompletionFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	err := run([]string{"activate", "zsh"}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "_compleo"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "#compdef compleo")
}

func TestRunWithActivateTwiceDoesNotRewriteUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	require.NoError(t, run([]string{"activate", "zsh"}, false, strings.NewReader(""), &bytes.Buffer{}, ""))
	path := filepath.Join(dir, "_compleo")
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, run([]string{"activate", "zsh"}, false, strings.NewReader(""), &bytes.Buffer{}, ""))
	after, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestWriteCompletionFileSkipsRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, cmdutil.WriteCompletionFile(dir, "_mytool", "content-a"))
	path := filepath.Join(dir, "_mytool")
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, cmdutil.WriteCompletionFile(dir, "_mytool", "content-a"))
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	require.NoError(t, cmdutil.WriteCompletionFile(dir, "_mytool", "content-b"))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content-b", string(content))
}

func TestRunParseGenerateModeWritesWrappedScriptsCompletionFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	err := run([]string{"parse"}, true, strings.NewReader("name: greet\nflags:\n  --loud: shout\n"), &bytes.Buffer{}, "greet")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "_greet"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "#compdef greet")
	assert.Contains(t, string(content), "local -x "+cmdutil.EnvSpecFile+"="+zsh.Em.Quote(filepath.Join(dir, ".greet.spec.yaml")))

	specContent, err := os.ReadFile(filepath.Join(dir, ".greet.spec.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "name: greet\nflags:\n  --loud: shout\n", string(specContent))
}

func TestRunParseGenerateModeReportsSuccessAsSelfExitingZsh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	var stdout bytes.Buffer
	err := run([]string{"parse"}, true, strings.NewReader("name: greet\n"), &stdout, "greet")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "exit 0")
	assert.Contains(t, stdout.String(), "To activate completion for greet in this shell, run:")
}

func TestRunParseGenerateModeDoesNotRequireWrappedArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, dir)

	// Unlike a normal parse, generate mode doesn't parse "$@" at all, so a script run with
	// _COMPLEO_GENERATE_NAME set (no args of its own) must not fail or need "--".
	err := run([]string{"parse"}, true, strings.NewReader("name: greet\npositionals:\n  - name: who\n    required: true\n"), &bytes.Buffer{}, "greet")
	assert.NoError(t, err)
}

// Simulates one Tab-press callback: _COMPLEO_SPEC_FILE is the companion spec file path
// WrappedCompletion embeds; args[0] is the typed command name (carapace's "_carapace zsh
// <name> <words>" convention).
func TestRunCarapaceWithCompleoSpecFileCompletesTheReferencedSpec(t *testing.T) {
	specFile := filepath.Join(t.TempDir(), ".greet.spec.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte("name: greet\nflags:\n  --loud: shout\n"), 0o644))
	t.Setenv(cmdutil.EnvSpecFile, specFile)

	var stdout bytes.Buffer
	err := run([]string{"_carapace", "zsh", "greet", "--l"}, false, strings.NewReader(""), &stdout, "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "loud")
}

func TestRunCarapaceWithInvalidCompleoSpecFileFails(t *testing.T) {
	specFile := filepath.Join(t.TempDir(), ".greet.spec.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte("not: [valid"), 0o644))
	t.Setenv(cmdutil.EnvSpecFile, specFile)

	err := run([]string{"_carapace", "zsh", "greet", ""}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestRunCarapaceWithMissingCompleoSpecFileFails(t *testing.T) {
	t.Setenv(cmdutil.EnvSpecFile, filepath.Join(t.TempDir(), "does-not-exist.yaml"))

	err := run([]string{"_carapace", "zsh", "greet", ""}, false, strings.NewReader(""), &bytes.Buffer{}, "")
	assert.Error(t, err)
}

func TestLogDebugWritesErrorWhenEnvSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	t.Setenv(cmdutil.EnvDebugLog, path)

	logDebug([]string{"_carapace", "zsh", "greet", ""}, errors.New("invalid embedded spec: boom"))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), `args="_carapace zsh greet "`)
	assert.Contains(t, string(content), "invalid embedded spec: boom")
}

func TestLogDebugAppendsAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	t.Setenv(cmdutil.EnvDebugLog, path)

	logDebug([]string{"first"}, errors.New("one"))
	logDebug([]string{"second"}, errors.New("two"))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(content), "\n"))
	assert.Contains(t, string(content), "one")
	assert.Contains(t, string(content), "two")
}

func TestLogDebugDoesNothingWhenEnvUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	t.Setenv(cmdutil.EnvDebugLog, "")

	logDebug([]string{"whatever"}, errors.New("boom"))

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

// Best effort: an unwritable path must never panic or otherwise disrupt the caller, since
// this is a secondary debugging aid, not part of compleo's primary error reporting.
func TestLogDebugIsBestEffortOnBadPath(t *testing.T) {
	t.Setenv(cmdutil.EnvDebugLog, filepath.Join(t.TempDir(), "no-such-dir", "debug.log"))

	assert.NotPanics(t, func() {
		logDebug([]string{"whatever"}, errors.New("boom"))
	})
}

func TestIsPiped(t *testing.T) {
	// Create a temporary file (not a terminal)
	f, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer os.Remove(f.Name()) //nolint:errcheck
	defer f.Close()           //nolint:errcheck

	assert.True(t, isPiped(f))
}

func TestIsPipedTerminal(t *testing.T) {
	// stdin is a terminal (when running tests)
	// This test may behave differently in CI vs local
	// We just verify it doesn't panic
	_ = isPiped(os.Stdin)
}
