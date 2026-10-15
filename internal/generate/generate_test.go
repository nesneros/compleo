package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A spec without a name: at all (legal now) gets one embedded into its companion file.
func TestGenerateInsertsNameWhenSpecDoesNotDeclareOne(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	t.Setenv(cmdutil.EnvGenerateName, "real-name")

	rawSpec := "flags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)
	require.Empty(t, sp.Name)

	require.NoError(t, Generate(cmd, sp, rawSpec, &bytes.Buffer{}))

	content, err := os.ReadFile(filepath.Join(completionDir, "_real-name"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "#compdef real-name")
	assert.Contains(t, string(content), "function _real-name_completion")
	assert.Contains(t, string(content), "local -x "+cmdutil.EnvSpecFile+"="+zsh.Quote(filepath.Join(completionDir, ".real-name.spec.yaml")))

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

	require.NoError(t, Generate(cmd, sp, rawSpec, &bytes.Buffer{}))

	specContent, err := os.ReadFile(filepath.Join(completionDir, ".greet.spec.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "name: 'greet'\n"+rawSpec, string(specContent))
}

// A declared name: that disagrees with the real, invokable name is an error, not overridden.
func TestGenerateFailsWhenSpecNameDisagreesWithInvokedName(t *testing.T) {
	completionDir := t.TempDir()
	t.Setenv(cmdutil.EnvCompletionDir, completionDir)
	t.Setenv(cmdutil.EnvGenerateName, "real-name")

	rawSpec := "name: wrong-name\nflags:\n  --loud: shout\n"
	cmd, sp, err := spec.ParseYAML(rawSpec, "")
	require.NoError(t, err)

	err = Generate(cmd, sp, rawSpec, &bytes.Buffer{})
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
func TestGenerateForSetsGenerateNameEnvVarOnTheResolvedExecutable(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-wrapped")
	require.NoError(t, os.WriteFile(scriptPath, fmt.Appendf(nil, "#!/bin/sh\n# calls compleo\necho \"name=$%s\"\n", cmdutil.EnvGenerateName), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	err := GenerateFor("fake-wrapped", &stdout)
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
	err := GenerateFor(scriptPath, &stdout)
	require.NoError(t, err)
	assert.Equal(t, "name=fake-wrapped\n", stdout.String())
}

func TestGenerateForFailsForUnknownExecutable(t *testing.T) {
	err := GenerateFor("definitely-not-a-real-command-xyz", &bytes.Buffer{})
	assert.Error(t, err)
}

// Regression test: "generate" used to run whatever exec.LookPath resolved with no check at
// all — happily executing any binary on PATH with EnvGenerateName set, compleo-wrapped or not.
func TestGenerateForRefusesExecutableThatIsNotCompleoWrapped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not-wrapped"), []byte("#!/bin/sh\necho hi\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := GenerateFor("not-wrapped", &bytes.Buffer{})
	assert.Error(t, err)
}
