package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCompleoScriptAcceptsShebangScriptMentioningCompleo(t *testing.T) {
	path := writeFile(t, "#!/bin/sh\neval \"$(compleo parse -- \"$@\")\"\n")
	assert.True(t, IsCompleoScript(path))
}

func TestIsCompleoScriptRejectsScriptWithoutShebang(t *testing.T) {
	path := writeFile(t, "compleo parse -- \"$@\"\n")
	assert.False(t, IsCompleoScript(path))
}

func TestIsCompleoScriptRejectsShebangScriptNotMentioningCompleo(t *testing.T) {
	path := writeFile(t, "#!/bin/sh\necho hi\n")
	assert.False(t, IsCompleoScript(path))
}

func TestIsCompleoScriptRejectsMissingFile(t *testing.T) {
	assert.False(t, IsCompleoScript(filepath.Join(t.TempDir(), "does-not-exist")))
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

// Drives $_.scripts (the macro registered as $compleo.scripts — "compleo." only resolves
// when the running binary is actually named compleo, not a go test binary) through a real
// cobra _carapace dispatch, the same path "compleo generate <TAB>" uses.
func TestCompleoScriptsMacroCompletesOnlyWrappedExecutablesOnPath(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755))
	}
	write("my-wrapped-tool", "#!/bin/sh\neval \"$(compleo parse -- \"$@\")\"\n")
	write("my-plain-script", "#!/bin/sh\necho hi\n")
	write("my-compleo-mention-binary", "not a script, just mentions compleo in its bytes")
	t.Setenv("PATH", dir)

	cmd, _, err := ParseYAML(`
name: mytool
positionals:
  - name: target
    completion: ["$_.scripts"]
`, "")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd.SetArgs([]string{"_carapace", "zsh", "mytool", ""})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "my-wrapped-tool")
	assert.NotContains(t, buf.String(), "my-plain-script")
	assert.NotContains(t, buf.String(), "my-compleo-mention-binary")
}

// A command with no positionals must not offer the cwd's files via whatever carapace's own
// cobra-compat fallback does for an undeclared position — it should complete to nothing.
func TestNoPositionalsCompletesToNothingNotCwdFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "suspicious-file"), []byte("x"), 0o644))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(cwd) }()

	cmd, _, err := ParseYAML(`
name: mytool
flags:
  --loud: shout
`, "")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd.SetArgs([]string{"_carapace", "zsh", "mytool", ""})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	assert.NotContains(t, buf.String(), "suspicious-file")
}

// Confirms the merge with carapace's own file completion actually works: $compleo.scripts
// must also offer path candidates, since "compleo generate" now accepts a path too.
func TestCompleoScriptsMacroAlsoCompletesFilePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "some-file"), []byte("x"), 0o644))
	t.Setenv("PATH", "")

	cmd, _, err := ParseYAML(`
name: mytool
positionals:
  - name: target
    completion: ["$_.scripts"]
`, "")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd.SetArgs([]string{"_carapace", "zsh", "mytool", dir + "/"})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "some-file")
}
