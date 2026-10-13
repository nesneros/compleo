package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLooksWrappedAcceptsShebangScriptMentioningCompleo(t *testing.T) {
	path := writeFile(t, "#!/bin/sh\neval \"$(compleo parse -- \"$@\")\"\n")
	assert.True(t, looksWrapped(path))
}

func TestLooksWrappedRejectsScriptWithoutShebang(t *testing.T) {
	path := writeFile(t, "compleo parse -- \"$@\"\n")
	assert.False(t, looksWrapped(path))
}

func TestLooksWrappedRejectsShebangScriptNotMentioningCompleo(t *testing.T) {
	path := writeFile(t, "#!/bin/sh\necho hi\n")
	assert.False(t, looksWrapped(path))
}

func TestLooksWrappedRejectsMissingFile(t *testing.T) {
	assert.False(t, looksWrapped(filepath.Join(t.TempDir(), "does-not-exist")))
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

// TestCompleoScriptsMacroCompletesOnlyWrappedExecutablesOnPath drives $_.scripts (the macro
// registered as $compleo.scripts) end-to-end through a real cobra _carapace dispatch — the
// same path "compleo generate <TAB>" uses. $_.scripts is the raw internal form: $compleo.scripts
// only resolves to it when the running binary is actually named "compleo" (confirmed manually
// against the real built binary), which isn't true for a go test binary.
func TestCompleoScriptsMacroCompletesOnlyWrappedExecutablesOnPath(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755))
	}
	write("my-wrapped-tool", "#!/bin/sh\neval \"$(compleo parse -- \"$@\")\"\n")
	write("my-plain-script", "#!/bin/sh\necho hi\n")
	write("my-compleo-mention-binary", "not a script, just mentions compleo in its bytes")
	t.Setenv("PATH", dir)

	cmd, _, err := ParseYAML(strings.NewReader(`
name: mytool
positionals:
  - name: target
completion:
  positional:
    - ["$_.scripts"]
`))
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd.SetArgs([]string{"_carapace", "zsh", "mytool", ""})
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "my-wrapped-tool")
	assert.NotContains(t, buf.String(), "my-plain-script")
	assert.NotContains(t, buf.String(), "my-compleo-mention-binary")
}
