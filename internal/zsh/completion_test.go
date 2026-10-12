package zsh

import (
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionGeneratesZshSnippet(t *testing.T) {
	cmd, sp, err := spec.ParseYAML(strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`))
	require.NoError(t, err)

	snippet, err := Completion(cmd, sp)
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef mytool")
	assert.Contains(t, snippet, "compdef _mytool_completion mytool")
}

// Rebuilding the cobra tree from the same spec must produce byte-identical output: a
// caller that only rewrites a completion file when its content changed (to avoid
// bumping the directory's mtime and forcing compinit to rescan) depends on this.
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
	cmd1, sp1, err := spec.ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	snippet1, err := Completion(cmd1, sp1)
	require.NoError(t, err)

	cmd2, sp2, err := spec.ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	snippet2, err := Completion(cmd2, sp2)
	require.NoError(t, err)

	assert.Equal(t, snippet1, snippet2)
}
