package generate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/zsh"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Setenv(cmdutil.EnvCompletionShell, "zsh")
	shells.SetEvalEmitter(zsh.Em)
	os.Exit(m.Run())
}

// A spec without a name: at all (legal now) gets one embedded into its companion file.
func TestGenerateInsertsNameWhenSpecDoesNotDeclareOne(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)

	rawSpec := "flags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)
	require.Empty(t, sp.Name)

	require.NoError(t, Generate(cmd, sp, rawSpec, "real-name", &bytes.Buffer{}))

	content, err := os.ReadFile(filepath.Join(completionDir, "_real-name"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "#compdef real-name")
	assert.Contains(t, string(content), "function _real-name_completion")
	assert.Contains(t, string(content), "local -x "+cmdutil.EnvSpecFile+"="+zsh.Em.Quote(filepath.Join(completionDir, ".real-name.spec.yaml")))

	specContent, err := os.ReadFile(filepath.Join(completionDir, ".real-name.spec.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "name: 'real-name'\n"+rawSpec, string(specContent))
}

// Regression test: examples/greet's own pattern (--name="${0:t}", no name: in the spec text)
// used to leave the companion file without a name: at all — sp.Name was already non-empty
// from --name, even though rawSpec's own text had none.
func TestGenerateInsertsNameSuppliedOnlyViaNameFlag(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)

	rawSpec := "flags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "greet")
	require.NoError(t, err)
	require.Equal(t, "greet", sp.Name)

	require.NoError(t, Generate(cmd, sp, rawSpec, "", &bytes.Buffer{}))

	specContent, err := os.ReadFile(filepath.Join(completionDir, ".greet.spec.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "name: 'greet'\n"+rawSpec, string(specContent))
}

func TestGenerateInsertsScriptShellFromShebang(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	scriptPath := filepath.Join(t.TempDir(), "greet")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/usr/bin/env zsh\n"), 0o755))
	t.Setenv(cmdutil.EnvScriptFile, scriptPath)

	rawSpec := "name: greet\nflags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	require.NoError(t, Generate(cmd, sp, rawSpec, "", &bytes.Buffer{}))

	specContent, err := os.ReadFile(filepath.Join(completionDir, ".greet.spec.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "scriptShell: 'zsh'\n"+rawSpec, string(specContent))

	_, parsed, err := spec.ParseYAML(string(specContent), "")
	require.NoError(t, err)
	assert.Equal(t, "zsh", parsed.ScriptShell)
}

func TestGenerateFailsWhenScriptShellDisagreesWithShebang(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	scriptPath := filepath.Join(t.TempDir(), "greet")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/zsh\n"), 0o755))
	t.Setenv(cmdutil.EnvScriptFile, scriptPath)

	rawSpec := "name: greet\nscriptShell: bash\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	err = Generate(cmd, sp, rawSpec, "", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `scriptShell: "bash"`)
	assert.Contains(t, err.Error(), "zsh")

	_, statErr := os.Stat(filepath.Join(completionDir, ".greet.spec.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestGenerateForPassesScriptFileToChild(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-wrapped")
	require.NoError(t, os.WriteFile(scriptPath, fmt.Appendf(nil, "#!/bin/sh\n# calls compleo\necho \"script=$%s\"\n", cmdutil.EnvScriptFile), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	require.NoError(t, GenerateFor("fake-wrapped", "", &stdout))
	assert.Equal(t, "script="+scriptPath+"\n", stdout.String())
}

// A declared name: that disagrees with the real, invokable name is an error, not overridden.
func TestGenerateFailsWhenSpecNameDisagreesWithInvokedName(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)

	rawSpec := "name: wrong-name\nflags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	err = Generate(cmd, sp, rawSpec, "real-name", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec declares name: "wrong-name"`)
	assert.Contains(t, err.Error(), `"real-name" was invoked`)

	_, statErr := os.Stat(filepath.Join(completionDir, "_wrong-name"))
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(filepath.Join(completionDir, "_real-name"))
	assert.True(t, os.IsNotExist(statErr))
}

// Checks GenerateFor's own wiring (PATH resolution, env var, stdout passthrough), not
// the full parse/generate pipeline.
func TestGenerateForPassesCompletionShell(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-wrapped")
	require.NoError(t, os.WriteFile(scriptPath, fmt.Appendf(nil, "#!/bin/sh\n# calls compleo\necho \"shell=$%s\"\n", cmdutil.EnvCompletionShell), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	require.NoError(t, GenerateFor("fake-wrapped", "bash", &stdout))
	assert.Equal(t, "shell=bash\n", stdout.String())
}

func TestGenerateForSetsGenerateNameEnvVarOnTheResolvedExecutable(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-wrapped")
	require.NoError(t, os.WriteFile(scriptPath, fmt.Appendf(nil, "#!/bin/sh\n# calls compleo\necho \"name=$%s\"\n", cmdutil.EnvGenerateName), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	err := GenerateFor("fake-wrapped", "", &stdout)
	require.NoError(t, err)
	assert.Equal(t, "name=fake-wrapped\n", stdout.String())
}

// Regression test: exec.LookPath resolves a path directly, without consulting $PATH at all,
// when name contains a "/" — but EnvGenerateName used to be set to that whole path
// string, which then mismatched the script's own "${0:t}"-derived bare name and failed.
func TestGenerateForPathArgumentUsesBasenameNotFullPath(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-wrapped")
	require.NoError(t, os.WriteFile(scriptPath, fmt.Appendf(nil, "#!/bin/sh\n# calls compleo\necho \"name=$%s\"\n", cmdutil.EnvGenerateName), 0o755))
	t.Setenv("PATH", "") // scriptPath is given as a path below; $PATH must not matter at all.

	var stdout bytes.Buffer
	err := GenerateFor(scriptPath, "", &stdout)
	require.NoError(t, err)
	assert.Equal(t, "name=fake-wrapped\n", stdout.String())
}

func TestGenerateForFailsForUnknownExecutable(t *testing.T) {
	err := GenerateFor("definitely-not-a-real-command-xyz", "", &bytes.Buffer{})
	assert.Error(t, err)
}

func TestGenerateWarnsWhenMacroBinaryIsNotOnPath(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	path := os.Getenv("PATH")
	t.Setenv("PATH", "")

	rawSpec := "" +
		"name: demo\n" +
		"persistentflags:\n" +
		"  --remote=: {completion: [\"$missinga.hosts\"]}\n" +
		"commands:\n" +
		"  - name: sub\n" +
		"    flags:\n" +
		"      --color=: {completion: [\"$missingb.color ||| $nospace\"]}\n" +
		"      --again=: {completion: [\"$missinga.again(1)\"]}\n" +
		"    positionals:\n" +
		"      - name: file\n" +
		"        completion: [\"$files\"]\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, Generate(cmd, sp, rawSpec, "", &out))

	_, statErr := os.Stat(filepath.Join(completionDir, "_demo"))
	require.NoError(t, statErr)

	t.Setenv("PATH", path)
	stderr := evalGenerated(t, out.String())
	assert.Equal(t, 1, strings.Count(stderr, "warning:"))
	assert.Contains(t, stderr, "warning: not on PATH, so these completion macros will not work: $missinga.*, $missingb.*")
	assert.NotContains(t, stderr, "$files")
	assert.Contains(t, stderr, "To activate completion for demo in this shell, run:")
}

func TestGenerateDoesNotWarnWhenMacroBinaryIsOnPath(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "onpath"), []byte("#!/bin/sh\n"), 0o755))
	path := os.Getenv("PATH")
	t.Setenv("PATH", binDir)

	rawSpec := "name: demo\nflags:\n  --remote=: {completion: [\"$onpath.hosts\"]}\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, Generate(cmd, sp, rawSpec, "", &out))

	t.Setenv("PATH", path)
	stderr := evalGenerated(t, out.String())
	assert.NotContains(t, stderr, "warning:")
	assert.Contains(t, stderr, "To activate completion for demo in this shell, run:")
}

func TestGenerateDoesNotWarnForCoreOrOwnMacros(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	path := os.Getenv("PATH")
	t.Setenv("PATH", "")

	exe, err := os.Executable()
	require.NoError(t, err)
	self := filepath.Base(exe)

	rawSpec := "name: demo\nflags:\n" +
		"  --file=: {completion: [\"$files\"]}\n" +
		"  --ext=: {completion: [\"$files([.go])\"]}\n" +
		"  --script=: {completion: [\"$" + self + ".scripts\"]}\n" +
		"  --old=: {completion: [\"$_deprecated.thing\"]}\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, Generate(cmd, sp, rawSpec, "", &out))

	t.Setenv("PATH", path)
	stderr := evalGenerated(t, out.String())
	assert.NotContains(t, stderr, "warning:")
}

func evalGenerated(t *testing.T, snippet string) string {
	t.Helper()
	cmd := exec.Command("zsh", "-fc", snippet)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	return stderr.String()
}

// Regression test: "generate" used to run whatever exec.LookPath resolved with no check at
// all — happily executing any binary on PATH with EnvGenerateName set, compleo-wrapped or not.
func TestGenerateForRefusesExecutableThatIsNotCompleoWrapped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not-wrapped"), []byte("#!/bin/sh\necho hi\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := GenerateFor("not-wrapped", "", &bytes.Buffer{})
	assert.Error(t, err)
}
