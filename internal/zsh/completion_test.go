package zsh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carapace-sh/carapace/pkg/uid"
	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionGeneratesZshSnippet(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`), "")
	require.NoError(t, err)

	snippet, err := Completion(cmd)
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef mytool")
	assert.Contains(t, snippet, "compdef _mytool_completion mytool")
	assert.True(t, strings.HasSuffix(snippet, "\n"))
}

// Same spec must produce byte-identical output: write-only-if-changed callers depend on this.
func TestCompletionIsDeterministic(t *testing.T) {
	yaml := `
name: mytool
flags:
  --count=: how many times
  --name=: a name
  --verbose: be verbose
commands:
  - name: sub
    flags:
      --tag=: a tag
`
	cmd1, _, err := spec.ParseYAML(strings.NewReader(yaml), "")
	require.NoError(t, err)
	snippet1, err := Completion(cmd1)
	require.NoError(t, err)

	cmd2, _, err := spec.ParseYAML(strings.NewReader(yaml), "")
	require.NoError(t, err)
	snippet2, err := Completion(cmd2)
	require.NoError(t, err)

	assert.Equal(t, snippet1, snippet2)
}

func TestWrappedCompletionEmbedsSpecFilePathAndLeavesCarapacesInvocationUntouched(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader("name: greet\nflags:\n  --loud: shout\n"), "")
	require.NoError(t, err)

	snippet, err := WrappedCompletion(cmd, "greet", "/some/dir/.greet.spec.yaml")
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef greet")
	assert.Contains(t, snippet, "local -x "+cmdutil.EnvSpecFile+"="+Quote("/some/dir/.greet.spec.yaml"))
	// Unlike the old re-invoke-the-script design, a Tab press now dispatches straight to
	// compleo itself — carapace's own default invocation is left exactly as generated.
	assert.Contains(t, snippet, "xargs "+uid.Executable()+" _carapace zsh")
}

func TestWrappedCompletionIsSyntacticallyValidZsh(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader("name: greet\nflags:\n  --loud: shout\n"), "")
	require.NoError(t, err)
	snippet, err := WrappedCompletion(cmd, "greet", "/some/dir/.greet.spec.yaml")
	require.NoError(t, err)

	require.NoError(t, exec.Command("zsh", "-n", "-c", snippet).Run())
}

// #compdef registration must use the given name, not whatever a spec's own (possibly
// stale) name: declares, or Tab-completion silently never triggers.
func TestWrappedCompletionUsesGivenNameNotCmdName(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader("name: carapace-bin-macros\nflags:\n  --loud: shout\n"), "")
	require.NoError(t, err)
	require.Equal(t, "carapace-bin-macros", cmd.Name())

	snippet, err := WrappedCompletion(cmd, "carapace_bin_macros", "/some/dir/.carapace_bin_macros.spec.yaml")
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef carapace_bin_macros")
	assert.Contains(t, snippet, "function _carapace_bin_macros_completion")
	assert.Contains(t, snippet, "compdef _carapace_bin_macros_completion carapace_bin_macros")
}

// The embedded path must round-trip exactly (spaces/quotes/$ included) through being
// spliced in as a single-quoted string. EnvSpecFile is function-local, so a fake
// "xargs" captures it to a file instead of it being readable after the function returns.
func TestWrappedCompletionSpecFilePathRoundTripsThroughRealZsh(t *testing.T) {
	specPath := "/some dir/with '$pecial `chars/.greet.spec.yaml"
	cmd, _, err := spec.ParseYAML(strings.NewReader("name: greet\nflags:\n  --loud: shout\n"), "")
	require.NoError(t, err)

	snippet, err := WrappedCompletion(cmd, "greet", specPath)
	require.NoError(t, err)

	captureFile := filepath.Join(t.TempDir(), "captured")
	script := fmt.Sprintf(`
_describe() { :; }
_message() { :; }
compstate=()
zstyle() { :; }
xargs() { print -rn -- "$%s" > %s; }
%s
words=(greet "")
CURRENT=2
curcontext=":completion::complete:greet::"
_greet_completion
`, cmdutil.EnvSpecFile, Quote(captureFile), snippet)

	_, err = exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)

	captured, err := os.ReadFile(captureFile)
	require.NoError(t, err)
	assert.Equal(t, specPath, string(captured))
}
